package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres is being introduced step by step (see db/migrations). Until the
// data moves over, Redis stays the store of record and Postgres is optional:
// with DATABASE_URL unset nothing changes.
//
//	DATABASE_URL   postgres://user:password@host:5432/dbname
var pgPool *pgxpool.Pool

// openPostgres connects, checks the connection and brings the schema up to date.
func openPostgres(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL: %w", err)
	}
	return openPostgresConfig(ctx, cfg)
}

func openPostgresConfig(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("cannot reach Postgres: %w", err)
	}
	applied, err := runMigrations(ctx, pool)
	if err != nil {
		pool.Close()
		return nil, err
	}
	if len(applied) > 0 {
		log.Printf("🗄️ Applied database migrations: %v", applied)
	}
	return pool, nil
}

// initPostgres is called at startup. A missing or unreachable database is not
// fatal yet: the app still runs on Redis alone.
func initPostgres() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Println("ℹ️ DATABASE_URL is not set: running on Redis only")
		return
	}
	pool, err := openPostgres(context.Background(), url)
	if err != nil {
		log.Printf("⚠️ Postgres is not available (%v): running on Redis only", err)
		return
	}
	pgPool = pool
	fmt.Println("✓ Successfully connected to Postgres")
}
