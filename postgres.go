package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
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

// chooseStore picks where the durable data lives. mode is GOTHERMO_STORE:
// empty or "redis" keeps everything in Redis (the default for now), "postgres"
// uses the database. It refuses the one dangerous combination: an empty
// database next to a Redis that still holds the data, where the app would
// start with every user and message missing.
func chooseStore(mode string, pool *pgxpool.Pool) (Store, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "redis":
		return redisStore{}, nil
	case "postgres":
		if pool == nil {
			return nil, fmt.Errorf("GOTHERMO_STORE=postgres needs a reachable DATABASE_URL")
		}
		empty, err := targetIsEmpty(pool)
		if err != nil {
			return nil, fmt.Errorf("cannot look into Postgres: %w", err)
		}
		if empty {
			if n, err := redisClient.SCard(ctx, "channels").Result(); err == nil && n > 0 {
				return nil, fmt.Errorf("Postgres is empty but Redis still has %d channel(s): "+
					"run `gothermo-server migrate` first (or empty Redis if that data is not needed)", n)
			}
		}
		return pgStore{pool: pool}, nil
	default:
		return nil, fmt.Errorf("GOTHERMO_STORE=%q is not known (use redis or postgres)", mode)
	}
}

// initStore is called at startup. A wrong setting stops the program: running
// on the other store than asked would look like lost data.
func initStore() {
	s, err := chooseStore(os.Getenv("GOTHERMO_STORE"), pgPool)
	if err != nil {
		log.Fatalf("❌ %v", err)
	}
	store = s
	if _, ok := s.(pgStore); ok {
		log.Println("🗄️ Durable data lives in Postgres (sessions and limits stay in Redis)")
	} else {
		log.Println("🗄️ Durable data lives in Redis")
	}
}
