package main

import (
	"context"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
)

func fakeMigrations(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, sql := range files {
		fsys["m/"+name] = &fstest.MapFile{Data: []byte(sql)}
	}
	return fsys
}

// ── parsing (no database needed) ──

func TestLoadMigrationsSortsByVersion(t *testing.T) {
	migs, err := loadMigrations(fakeMigrations(map[string]string{
		"0010_c.sql": "select 3", "0002_b.sql": "select 2", "0001_a.sql": "select 1",
	}), "m")
	if err != nil {
		t.Fatal(err)
	}
	if len(migs) != 3 || migs[0].Version != 1 || migs[1].Version != 2 || migs[2].Version != 10 {
		t.Fatalf("wrong order: %+v", migs)
	}
	if migs[0].Name != "a" || migs[0].Checksum == "" || migs[0].Checksum == migs[1].Checksum {
		t.Fatalf("name or checksum wrong: %+v", migs[:2])
	}
}

func TestLoadMigrationsRejectsBadNames(t *testing.T) {
	for _, bad := range []string{"init.sql", "1init.sql", "0001-init.sql", "0000_zero.sql", "0001_Init.sql", "0001_init.txt"} {
		if _, err := loadMigrations(fakeMigrations(map[string]string{bad: "select 1"}), "m"); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestLoadMigrationsRejectsDuplicateVersion(t *testing.T) {
	_, err := loadMigrations(fakeMigrations(map[string]string{"0001_a.sql": "select 1", "0001_b.sql": "select 2"}), "m")
	if err == nil || !strings.Contains(err.Error(), "version 1") {
		t.Fatalf("want a duplicate-version error, got %v", err)
	}
}

func TestEmbeddedMigrationsAreValid(t *testing.T) {
	migs, err := loadMigrations(embeddedMigrations, "db/migrations")
	if err != nil {
		t.Fatal(err)
	}
	if len(migs) == 0 || migs[0].Version != 1 {
		t.Fatalf("the project must ship at least migration 1: %+v", migs)
	}
	for i, m := range migs {
		if m.Version != i+1 {
			t.Fatalf("versions must be 1,2,3... without gaps, got %d at position %d", m.Version, i)
		}
	}
}

// ── running (needs Postgres) ──

func TestRunMigrationsAppliesOnceAndIsRepeatable(t *testing.T) {
	pool := newTestDatabase(t)
	ctx := context.Background()

	first, err := runMigrations(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	migs, _ := loadMigrations(embeddedMigrations, "db/migrations")
	if len(first) != len(migs) {
		t.Fatalf("first run applied %v, want %d migrations", first, len(migs))
	}

	second, err := runMigrations(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 0 {
		t.Fatalf("second run must do nothing, applied %v", second)
	}
	if n := mustCount(t, pool, "SELECT count(*) FROM schema_migrations"); n != len(migs) {
		t.Fatalf("schema_migrations has %d rows, want %d", n, len(migs))
	}
}

func TestRunMigrationsAppliesOnlyNewFiles(t *testing.T) {
	pool := newTestDatabase(t)
	ctx := context.Background()

	v1 := map[string]string{"0001_a.sql": "CREATE TABLE a (id int);"}
	if _, err := runMigrationsFS(ctx, pool, fakeMigrations(v1), "m"); err != nil {
		t.Fatal(err)
	}
	v2 := map[string]string{"0001_a.sql": v1["0001_a.sql"], "0002_b.sql": "CREATE TABLE b (id int);"}
	done, err := runMigrationsFS(ctx, pool, fakeMigrations(v2), "m")
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 1 || done[0] != 2 {
		t.Fatalf("only migration 2 should run, got %v", done)
	}
}

func TestRunMigrationsDetectsEditedFile(t *testing.T) {
	pool := newTestDatabase(t)
	ctx := context.Background()

	if _, err := runMigrationsFS(ctx, pool, fakeMigrations(map[string]string{"0001_a.sql": "CREATE TABLE a (id int);"}), "m"); err != nil {
		t.Fatal(err)
	}
	_, err := runMigrationsFS(ctx, pool, fakeMigrations(map[string]string{"0001_a.sql": "CREATE TABLE a (id int, extra int);"}), "m")
	if err == nil || !strings.Contains(err.Error(), "changed after it was applied") {
		t.Fatalf("an edited migration must be refused, got %v", err)
	}
}

func TestFailedMigrationRollsBackCompletely(t *testing.T) {
	pool := newTestDatabase(t)
	ctx := context.Background()

	files := map[string]string{
		"0001_ok.sql":  "CREATE TABLE good (id int);",
		"0002_bad.sql": "CREATE TABLE half (id int); INSERT INTO nowhere VALUES (1);",
	}
	done, err := runMigrationsFS(ctx, pool, fakeMigrations(files), "m")
	if err == nil || !strings.Contains(err.Error(), "0002_bad") {
		t.Fatalf("want the failing migration named in the error, got %v", err)
	}
	if len(done) != 1 || done[0] != 1 {
		t.Fatalf("migration 1 should have been applied before the failure, got %v", done)
	}
	if n := mustCount(t, pool, "SELECT count(*) FROM information_schema.tables WHERE table_name='half'"); n != 0 {
		t.Fatal("the half-applied table must be rolled back")
	}
	if n := mustCount(t, pool, "SELECT count(*) FROM schema_migrations WHERE version=2"); n != 0 {
		t.Fatal("a failed migration must not be recorded")
	}

	// Fix the file name's content and it runs on the next start.
	files["0002_bad.sql"] = "CREATE TABLE half (id int);"
	done, err = runMigrationsFS(ctx, pool, fakeMigrations(files), "m")
	if err != nil || len(done) != 1 || done[0] != 2 {
		t.Fatalf("retry after fixing: done=%v err=%v", done, err)
	}
}

func TestRunMigrationsConcurrentStarts(t *testing.T) {
	pool := newTestDatabase(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := runMigrations(ctx, pool)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent start failed: %v", err)
		}
	}
	migs, _ := loadMigrations(embeddedMigrations, "db/migrations")
	if n := mustCount(t, pool, "SELECT count(*) FROM schema_migrations"); n != len(migs) {
		t.Fatalf("each migration must be recorded exactly once, rows=%d", n)
	}
}
