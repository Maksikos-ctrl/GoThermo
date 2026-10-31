package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgStore keeps users, channels and read marks in Postgres.
type pgStore struct{ pool *pgxpool.Pool }

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ───────────────────────── users ─────────────────────────

const userColumns = "id, username, email, status, last_seen_at, coalesce(provider, ''), coalesce(provider_id, '')"

func scanUser(row pgx.Row) (*User, error) {
	var (
		u        User
		lastSeen *time.Time
	)
	if err := row.Scan(&u.ID, &u.Username, &u.Email, &u.Status, &lastSeen, &u.Provider, &u.ProviderID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errNotFound
		}
		return nil, err
	}
	u.IsOnline = u.Status != "offline"
	if lastSeen != nil {
		u.LastSeen = lastSeen.Format(time.RFC3339)
	}
	return &u, nil
}

// SaveUser inserts or updates by id. It never touches the password.
func (p pgStore) SaveUser(u *User) error {
	status := u.Status
	if status == "" {
		status = "offline"
	}
	var lastSeen any
	if u.LastSeen != "" {
		t, err := time.Parse(time.RFC3339, u.LastSeen)
		if err != nil {
			return fmt.Errorf("last seen %q: %w", u.LastSeen, err)
		}
		lastSeen = t
	}
	_, err := p.pool.Exec(ctx, `
		INSERT INTO users (id, username, email, status, last_seen_at, provider, provider_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			username = EXCLUDED.username, email = EXCLUDED.email, status = EXCLUDED.status,
			last_seen_at = EXCLUDED.last_seen_at,
			provider = EXCLUDED.provider, provider_id = EXCLUDED.provider_id`,
		u.ID, u.Username, u.Email, status, lastSeen, nullIfEmpty(u.Provider), nullIfEmpty(u.ProviderID))
	return err
}

func (p pgStore) GetUserByEmail(email string) (*User, error) {
	return scanUser(p.pool.QueryRow(ctx,
		"SELECT "+userColumns+" FROM users WHERE lower(email) = lower($1) AND provider IS NULL", email))
}

func (p pgStore) ListUsers() ([]User, error) {
	rows, err := p.pool.Query(ctx, "SELECT "+userColumns+" FROM users ORDER BY created_at, username")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

func (p pgStore) SetPasswordHash(email, hash string) error {
	tag, err := p.pool.Exec(ctx,
		"UPDATE users SET password_hash = $2 WHERE lower(email) = lower($1) AND provider IS NULL", email, hash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

func (p pgStore) PasswordHash(email string) (string, error) {
	var hash *string
	err := p.pool.QueryRow(ctx,
		"SELECT password_hash FROM users WHERE lower(email) = lower($1) AND provider IS NULL", email).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && hash == nil) {
		return "", errNotFound
	}
	if err != nil {
		return "", err
	}
	return *hash, nil
}

func (p pgStore) UsernameByProvider(provider, providerID string) (string, bool, error) {
	var username string
	err := p.pool.QueryRow(ctx,
		"SELECT username FROM users WHERE provider = $1 AND provider_id = $2", provider, providerID).Scan(&username)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return username, true, nil
}

// ───────────────────────── channels ─────────────────────────

// SaveChannel creates the channel or replaces it by name. An existing channel
// keeps its id (messages point at it).
func (p pgStore) SaveChannel(c Channel) error {
	if c.Name == "" {
		return fmt.Errorf("channel name cannot be empty")
	}
	created := c.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO channels (id, name, description, is_private, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (name) DO UPDATE SET
			description = EXCLUDED.description, is_private = EXCLUDED.is_private,
			created_by = EXCLUDED.created_by, created_at = EXCLUDED.created_at
		RETURNING id`,
		c.ID, c.Name, c.Description, c.IsPrivate, c.CreatedBy, created).Scan(&id); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, "DELETE FROM channel_members WHERE channel_id = $1", id); err != nil {
		return err
	}
	seen := map[string]bool{}
	pos := 0
	for _, m := range c.Members {
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		if _, err := tx.Exec(ctx,
			"INSERT INTO channel_members (channel_id, username, position) VALUES ($1, $2, $3)", id, m, pos); err != nil {
			return err
		}
		pos++
	}
	return tx.Commit(ctx)
}

const channelSelect = `
	SELECT c.id, c.name, c.description, c.is_private, c.created_by, c.created_at,
	       coalesce(array_agg(m.username ORDER BY m.position) FILTER (WHERE m.username IS NOT NULL), '{}')
	FROM channels c LEFT JOIN channel_members m ON m.channel_id = c.id`

func scanChannel(row pgx.Row) (*Channel, error) {
	var c Channel
	if err := row.Scan(&c.ID, &c.Name, &c.Description, &c.IsPrivate, &c.CreatedBy, &c.CreatedAt, &c.Members); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errNotFound
		}
		return nil, err
	}
	if c.Members == nil {
		c.Members = []string{}
	}
	return &c, nil
}

func (p pgStore) GetChannel(name string) (*Channel, error) {
	return scanChannel(p.pool.QueryRow(ctx, channelSelect+" WHERE c.name = $1 GROUP BY c.id", name))
}

func (p pgStore) ListChannels() ([]Channel, error) {
	rows, err := p.pool.Query(ctx, channelSelect+" GROUP BY c.id ORDER BY c.created_at, c.name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	channels := []Channel{}
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, *c)
	}
	return channels, rows.Err()
}

// DeleteChannel removes the channel; messages, pins, read marks and events go
// with it through ON DELETE CASCADE.
func (p pgStore) DeleteChannel(name string) error {
	_, err := p.pool.Exec(ctx, "DELETE FROM channels WHERE name = $1", name)
	return err
}

// ───────────────────────── read marks ─────────────────────────

func (p pgStore) SaveLastRead(username, channel string) error {
	// The time comes from this program, like message timestamps do, so the two
	// are always compared on the same clock. A channel that does not exist has
	// nothing to mark.
	_, err := p.pool.Exec(ctx, `
		INSERT INTO channel_reads (username, channel_id, last_read_at)
		SELECT $1, id, $3 FROM channels WHERE name = $2
		ON CONFLICT (username, channel_id) DO UPDATE SET last_read_at = EXCLUDED.last_read_at`,
		username, channel, time.Now())
	return err
}

func (p pgStore) LastRead(username, channel string) (time.Time, bool, error) {
	var t time.Time
	err := p.pool.QueryRow(ctx, `
		SELECT r.last_read_at FROM channel_reads r JOIN channels c ON c.id = r.channel_id
		WHERE r.username = $1 AND c.name = $2`, username, channel).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return t, true, nil
}
