package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Tests that need a real Postgres run only when TEST_DATABASE_URL is set, for
// example:
//
//	docker run --name gt-test -e POSTGRES_PASSWORD=dev -p 54321:5432 -d postgres:16
//	TEST_DATABASE_URL=postgres://postgres:dev@localhost:54321/postgres go test ./...
//
// Every test gets its own brand-new database, dropped afterwards, so tests
// never see each other's data and your real data is never touched.

// newTestDatabase creates an empty database and returns a pool for it.
func newTestDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()

	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("cannot connect to TEST_DATABASE_URL: %v", err)
	}
	name := fmt.Sprintf("gothermo_test_%d_%d", time.Now().UnixNano(), rand.Intn(1000))
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close(ctx)
		t.Fatalf("create database: %v", err)
	}

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		admin.Close(ctx)
	})
	return pool
}

// migratedTestDatabase is newTestDatabase with the real schema applied.
func migratedTestDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := newTestDatabase(t)
	if _, err := runMigrations(context.Background(), pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return pool
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s\n  -> %v", sql, err)
	}
}

func mustCount(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s\n  -> %v", sql, err)
	}
	return n
}

func TestInitPostgresWithoutURLDoesNothing(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	pgPool = nil
	initPostgres()
	if pgPool != nil {
		t.Fatal("pool must stay nil when DATABASE_URL is not set")
	}
}

func TestInitPostgresUnreachableIsNotFatal(t *testing.T) {
	// Nothing listens on port 1: the app must carry on with Redis alone.
	t.Setenv("DATABASE_URL", "postgres://u:p@127.0.0.1:1/db?connect_timeout=1")
	pgPool = nil
	initPostgres()
	if pgPool != nil {
		t.Fatal("pool must stay nil when Postgres cannot be reached")
	}
}

func TestOpenPostgresAppliesSchema(t *testing.T) {
	base := newTestDatabase(t)
	cfg := base.Config()
	pool, err := openPostgresConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if n := mustCount(t, pool, "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name='messages'"); n != 1 {
		t.Fatalf("messages table missing after openPostgres")
	}
}
