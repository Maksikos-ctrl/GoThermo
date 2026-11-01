package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Where the durable data (accounts, messages, file records) lives. Sessions and
// login limits always live in Redis, so the commands about those never use this.
type dataSource interface {
	Label() string
	Users(ctx context.Context) ([]userRec, error)
	// ReferencedFileIDs: ids of the files attached to messages that still exist.
	ReferencedFileIDs(ctx context.Context) (map[string]bool, error)
	// FileMetas: every file record. An empty Path means the record is broken.
	FileMetas(ctx context.Context) ([]fileMeta, error)
	DeleteFileMetas(ctx context.Context, ids []string) error
}

type fileMeta struct{ ID, Path string }

// chooseSource follows the same rule as the server: an empty mode means
// Postgres when a database url is given, otherwise Redis.
func chooseSource(ctx context.Context, mode, databaseURL string, rdb *redis.Client, redisAddr string) (dataSource, func(), error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" && databaseURL != "" {
		mode = "postgres"
	}
	switch mode {
	case "", "redis":
		return redisSource{rdb: rdb, addr: redisAddr}, func() {}, nil
	case "postgres":
		if databaseURL == "" {
			return nil, nil, fmt.Errorf("the data is in Postgres: give -database-url or set DATABASE_URL")
		}
		pool, err := pgxpool.New(ctx, databaseURL)
		if err != nil {
			return nil, nil, fmt.Errorf("DATABASE_URL: %w", err)
		}
		if err := pool.Ping(ctx); err != nil {
			pool.Close()
			return nil, nil, fmt.Errorf("cannot connect to Postgres: %w", err)
		}
		return pgSource{pool: pool}, pool.Close, nil
	default:
		return nil, nil, fmt.Errorf("-store %q is not known (use redis or postgres)", mode)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ───────────────────────── Redis ─────────────────────────

type redisSource struct {
	rdb  *redis.Client
	addr string
}

func (s redisSource) Label() string { return "Redis " + s.addr }

func (s redisSource) Users(ctx context.Context) ([]userRec, error) {
	keys, err := scan(ctx, s.rdb, "user:*")
	if err != nil {
		return nil, err
	}
	var users []userRec
	for _, key := range keys {
		// Same filter the app uses: account records contain an "@" and are not
		// the :password / :token / :lastread helper keys.
		if !strings.Contains(key, "@") || strings.Contains(key, ":password") ||
			strings.Contains(key, ":token") || strings.Contains(key, ":lastread:") {
			continue
		}
		raw, err := s.rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		var u userRec
		if json.Unmarshal([]byte(raw), &u) == nil && u.Username != "" {
			users = append(users, u)
		}
	}
	return users, nil
}

func (s redisSource) ReferencedFileIDs(ctx context.Context) (map[string]bool, error) {
	keys, err := scan(ctx, s.rdb, "channel:*:messages")
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, key := range keys {
		items, err := s.rdb.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if item == "DELETED" {
				continue
			}
			var m struct {
				IsFile bool   `json:"isFile"`
				FileID string `json:"fileId"`
			}
			if json.Unmarshal([]byte(item), &m) == nil && m.IsFile && m.FileID != "" {
				ids[m.FileID] = true
			}
		}
	}
	return ids, nil
}

func (s redisSource) FileMetas(ctx context.Context) ([]fileMeta, error) {
	keys, err := scan(ctx, s.rdb, "file:*")
	if err != nil {
		return nil, err
	}
	metas := make([]fileMeta, 0, len(keys))
	for _, key := range keys {
		path, _ := s.rdb.HGet(ctx, key, "path").Result()
		metas = append(metas, fileMeta{ID: strings.TrimPrefix(key, "file:"), Path: path})
	}
	return metas, nil
}

func (s redisSource) DeleteFileMetas(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = "file:" + id
	}
	return s.rdb.Del(ctx, keys...).Err()
}

// ───────────────────────── Postgres ─────────────────────────

type pgSource struct{ pool *pgxpool.Pool }

func (pgSource) Label() string { return "Postgres" }

func (s pgSource) Users(ctx context.Context) ([]userRec, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT username, email, coalesce(provider, ''), status, last_seen_at FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []userRec
	for rows.Next() {
		var (
			u        userRec
			lastSeen *time.Time
		)
		if err := rows.Scan(&u.Username, &u.Email, &u.Provider, &u.Status, &lastSeen); err != nil {
			return nil, err
		}
		if lastSeen != nil {
			u.LastSeen = lastSeen.Format(time.RFC3339)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s pgSource) ReferencedFileIDs(ctx context.Context) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx, "SELECT DISTINCT file_id FROM messages WHERE file_id IS NOT NULL AND file_id <> ''")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

func (s pgSource) FileMetas(ctx context.Context) ([]fileMeta, error) {
	rows, err := s.pool.Query(ctx, "SELECT id, path FROM files ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var metas []fileMeta
	for rows.Next() {
		var m fileMeta
		if err := rows.Scan(&m.ID, &m.Path); err != nil {
			return nil, err
		}
		metas = append(metas, m)
	}
	return metas, rows.Err()
}

func (s pgSource) DeleteFileMetas(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, "DELETE FROM files WHERE id = ANY($1)", ids)
	return err
}
