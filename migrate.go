package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Schema migrations.
//
// Every file in db/migrations is named <version>_<name>.sql (0001_init.sql).
// Files are applied once, in version order, each inside its own transaction,
// and recorded in the schema_migrations table together with a checksum. Never
// edit a file that has been applied: add a new one instead. The checksum makes
// an edited file fail loudly instead of silently diverging from the database.

//go:embed db/migrations/*.sql
var embeddedMigrations embed.FS

// migrationLockID is an arbitrary number used with pg_advisory_lock so two
// servers starting at the same moment do not run the same migration twice.
const migrationLockID int64 = 7_362_001

var migrationName = regexp.MustCompile(`^(\d+)_([a-z0-9_]+)\.sql$`)

type migration struct {
	Version  int
	Name     string
	SQL      string
	Checksum string
}

// loadMigrations reads and orders the migration files of dir.
func loadMigrations(fsys fs.FS, dir string) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	var out []migration
	seen := map[int]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := migrationName.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("migration file %q must be named like 0001_name.sql", e.Name())
		}
		version, _ := strconv.Atoi(m[1])
		if version <= 0 {
			return nil, fmt.Errorf("migration file %q: version must be 1 or more", e.Name())
		}
		if other, dup := seen[version]; dup {
			return nil, fmt.Errorf("migration version %d is used by both %q and %q", version, other, e.Name())
		}
		seen[version] = e.Name()
		data, err := fs.ReadFile(fsys, dir+"/"+e.Name())
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		out = append(out, migration{Version: version, Name: m[2], SQL: string(data), Checksum: hex.EncodeToString(sum[:])})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// runMigrations brings the database up to date and returns the versions it
// applied just now (empty when nothing was left to do).
func runMigrations(ctx context.Context, pool *pgxpool.Pool) ([]int, error) {
	return runMigrationsFS(ctx, pool, embeddedMigrations, "db/migrations")
}

func runMigrationsFS(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS, dir string) ([]int, error) {
	migs, err := loadMigrations(fsys, dir)
	if err != nil {
		return nil, err
	}

	// One connection for the whole run: the advisory lock belongs to it.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return nil, fmt.Errorf("migration lock: %w", err)
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockID)

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    integer     PRIMARY KEY,
		name       text        NOT NULL,
		checksum   text        NOT NULL,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return nil, fmt.Errorf("schema_migrations: %w", err)
	}

	rows, err := conn.Query(ctx, "SELECT version, checksum FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	applied := map[int]string{}
	for rows.Next() {
		var v int
		var sum string
		if err := rows.Scan(&v, &sum); err != nil {
			rows.Close()
			return nil, err
		}
		applied[v] = sum
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var done []int
	for _, m := range migs {
		if sum, ok := applied[m.Version]; ok {
			if sum != m.Checksum {
				return done, fmt.Errorf("migration %04d_%s was changed after it was applied; add a new migration instead of editing it", m.Version, m.Name)
			}
			continue
		}
		if err := applyMigration(ctx, conn, m); err != nil {
			return done, fmt.Errorf("migration %04d_%s failed: %w", m.Version, m.Name, err)
		}
		done = append(done, m.Version)
	}
	return done, nil
}

func applyMigration(ctx context.Context, conn *pgxpool.Conn, m migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) // no-op after a successful commit

	// No arguments: pgx sends the whole file as one simple query, so a file
	// may hold many statements.
	if _, err := tx.Exec(ctx, m.SQL, pgx.QueryExecModeSimpleProtocol); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)",
		m.Version, m.Name, m.Checksum); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
