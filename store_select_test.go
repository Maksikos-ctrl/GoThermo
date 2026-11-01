package main

import (
	"strings"
	"testing"
)

func TestChooseStore(t *testing.T) {
	useMiniRedis(t)
	if s, err := chooseStore("", nil); err != nil || s != (redisStore{}) {
		t.Fatalf("without a database the default is Redis: %v %v", s, err)
	}
	if s, err := chooseStore(" Redis ", nil); err != nil || s != (redisStore{}) {
		t.Fatalf("redis: %v %v", s, err)
	}
	if _, err := chooseStore("mysql", nil); err == nil {
		t.Error("an unknown store was accepted")
	}
	if _, err := chooseStore("postgres", nil); err == nil {
		t.Error("postgres without a database was accepted")
	}
}

func TestChooseStorePostgresGuards(t *testing.T) {
	useMiniRedis(t)
	pool := migratedTestDatabase(t)

	// with a database and no setting, Postgres is the default; "redis" still wins
	if s, err := chooseStore("", pool); err != nil {
		t.Fatalf("default with a database: %v", err)
	} else if _, ok := s.(pgStore); !ok {
		t.Fatalf("the default with a database must be Postgres, got %T", s)
	}
	if s, err := chooseStore("redis", pool); err != nil || s != (redisStore{}) {
		t.Fatalf("explicit redis: %v %v", s, err)
	}

	// empty Postgres, empty Redis: a fresh install, fine
	if s, err := chooseStore("postgres", pool); err != nil || s == nil {
		t.Fatalf("fresh install: %v", err)
	} else if _, ok := s.(pgStore); !ok {
		t.Fatalf("not a pgStore: %T", s)
	}

	// empty Postgres next to a Redis that has channels: refuse
	if err := redisClient.SAdd(ctx, "channels", "general").Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := chooseStore("postgres", pool); err == nil || !strings.Contains(err.Error(), "migrate") {
		t.Fatalf("must point at the migration, got %v", err)
	}
	if _, err := chooseStore("", pool); err == nil || !strings.Contains(err.Error(), "migrate") {
		t.Fatalf("the default must be guarded too, got %v", err)
	}
	if s, err := chooseStore("redis", pool); err != nil || s != (redisStore{}) {
		t.Fatalf("running on Redis on purpose stays possible: %v %v", s, err)
	}

	// once Postgres has data the leftover Redis does not matter
	mustExec(t, pool, `INSERT INTO channels (id, name, created_by) VALUES ('c1', 'general', 'system')`)
	if _, err := chooseStore("postgres", pool); err != nil {
		t.Fatalf("after migration: %v", err)
	}
}

// With Postgres active the Redis-only cleanup must leave Redis alone.
func TestCompactDeletedMessagesOnlyOnRedis(t *testing.T) {
	useMiniRedis(t)
	pool := migratedTestDatabase(t)
	prev := store
	t.Cleanup(func() { store = prev })

	key := "channel:general:messages"
	if err := redisClient.RPush(ctx, key, "DELETED", `{"id":"m1"}`).Err(); err != nil {
		t.Fatal(err)
	}
	store = pgStore{pool: pool}
	CompactDeletedMessages()
	if n, _ := redisClient.LLen(ctx, key).Result(); n != 2 {
		t.Fatalf("the cleanup touched Redis while Postgres is active: %d entries", n)
	}
	store = redisStore{}
	CompactDeletedMessages()
	if n, _ := redisClient.LLen(ctx, key).Result(); n != 1 {
		t.Fatalf("the cleanup must still work on Redis: %d entries", n)
	}
}
