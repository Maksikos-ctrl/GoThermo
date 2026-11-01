package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests need a real Postgres: TEST_DATABASE_URL=postgres://... go test ./...
// Each test gets its own database with the real schema (db/migrations).

func newAdminTestDatabase(t *testing.T) (string, *pgxpool.Pool) {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("cannot connect to TEST_DATABASE_URL: %v", err)
	}
	var b [6]byte
	rand.Read(b[:])
	name := "admin_test_" + hex.EncodeToString(b[:])
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	admin.Close(ctx)

	u, _ := url.Parse(base)
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob("../../db/migrations/*.sql")
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatal("no migrations found")
	}
	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}
	t.Cleanup(func() {
		pool.Close()
		if c, err := pgx.Connect(ctx, base); err == nil {
			c.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
			c.Close(ctx)
		}
	})
	return u.String(), pool
}

func runCLIPostgres(t *testing.T, mr *miniredis.Miniredis, dbURL, stdin string, args ...string) (string, string, int) {
	t.Helper()
	t.Setenv("DATABASE_URL", "")
	t.Setenv("GOTHERMO_STORE", "")
	var out, errOut bytes.Buffer
	all := append([]string{"-addr", mr.Addr(), "-database-url", dbURL}, args...)
	code := run(all, strings.NewReader(stdin), &out, &errOut)
	return out.String(), errOut.String(), code
}

func TestUsersFromPostgres(t *testing.T) {
	mr := miniredis.RunT(t)
	dbURL, pool := newAdminTestDatabase(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, username, email, password_hash, status, last_seen_at) VALUES
			('u1', 'bob', 'bob@x.com', '$2a$secrethash', 'online', '2026-10-01T10:00:00Z');
		INSERT INTO users (id, username, email, status, provider, provider_id) VALUES
			('u2', 'alice', 'alice@gmail.com', 'away', 'google', '123')`); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := runCLIPostgres(t, mr, dbURL, "", "users")
	if code != 0 || !strings.Contains(out, "alice") || !strings.Contains(out, "google") ||
		!strings.Contains(out, "bob") || !strings.Contains(out, "password") || !strings.Contains(out, "2 accounts") {
		t.Fatalf("code=%d err=%s\n%s", code, errOut, out)
	}
	if strings.Contains(out, "secrethash") {
		t.Fatal("password hash leaked")
	}
	if strings.Index(out, "alice") > strings.Index(out, "bob") {
		t.Errorf("not sorted by username:\n%s", out)
	}

	// sessions are still read from Redis
	mr.Set("session:tok1", "bob")
	if out, _, code := runCLIPostgres(t, mr, dbURL, "", "sessions"); code != 0 || !strings.Contains(out, "bob") {
		t.Fatalf("sessions:\n%s", out)
	}
}

func TestUploadsFromPostgres(t *testing.T) {
	mr := miniredis.RunT(t)
	dbURL, pool := newAdminTestDatabase(t)
	ctx := context.Background()

	dir := t.TempDir()
	old := time.Now().Add(-72 * time.Hour)
	write := func(name string) string {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte("data"), 0644)
		os.Chtimes(p, old, old)
		return p
	}
	write(idUsed + "_used.png")
	write(idOrphan + "_orphan.png")
	write(idReply + "_in-a-reply.png")

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO channels (id, name, created_by) VALUES ('c1', 'general', 'system')`)
	exec(`INSERT INTO messages (id, channel_id, author, body, file_id, file_name) VALUES
		('m1', 'c1', 'alice', '', $1, 'used.png'),
		('m2', 'c1', 'alice', 'parent', NULL, NULL)`, idUsed)
	// a message whose file has vanished from disk: reported, its record is kept
	exec(`INSERT INTO messages (id, channel_id, author, body, file_id, file_name)
		VALUES ('m3', 'c1', 'alice', '', 'broken-id', 'lost.png')`)
	exec(`INSERT INTO files (id, path, mime_type, file_name) VALUES ('broken-id', $1, 'image/png', 'lost.png')`,
		filepath.Join(dir, "broken-id_lost.png"))
	exec(`INSERT INTO messages (id, channel_id, thread_id, author, body, file_id, file_name)
		VALUES ('r1', 'c1', 'm2', 'bob', '', $1, 'in-a-reply.png')`, idReply)
	exec(`INSERT INTO files (id, path, mime_type, file_name) VALUES
		($1, $4, 'image/png', 'used.png'),
		($2, $5, 'image/png', 'orphan.png'),
		($3, $6, 'image/png', 'in-a-reply.png'),
		('gone', $7, 'image/png', 'gone.png')`,
		idUsed, idOrphan, idReply,
		filepath.Join(dir, idUsed+"_used.png"), filepath.Join(dir, idOrphan+"_orphan.png"),
		filepath.Join(dir, idReply+"_in-a-reply.png"), filepath.Join(dir, "gone_does_not_exist.png"))

	out, errOut, code := runCLIPostgres(t, mr, dbURL, "", "uploads", "-dir", dir)
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errOut)
	}
	for _, want := range []string{"Data: Postgres", "In use:           2 files", "Orphans:          1 files",
		"Stale metadata:   1 entries", "Broken messages:  1", "Dry run"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}

	out, _, _ = runCLIPostgres(t, mr, dbURL, "y\n", "uploads", "-dir", dir, "-clean")
	if !strings.Contains(out, "Deleted 1 files and 1 metadata") {
		t.Fatalf("clean:\n%s", out)
	}
	exists := func(n string) bool { _, err := os.Stat(filepath.Join(dir, n)); return err == nil }
	if exists(idOrphan+"_orphan.png") || !exists(idUsed+"_used.png") || !exists(idReply+"_in-a-reply.png") {
		t.Fatal("wrong files deleted (a file attached to a thread reply is in use)")
	}
	var left []string
	rows, err := pool.Query(ctx, "SELECT id FROM files ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		rows.Scan(&id)
		left = append(left, id)
	}
	rows.Close()
	want := []string{idUsed, idReply, "broken-id"}
	sort.Strings(want)
	if strings.Join(left, ",") != strings.Join(want, ",") {
		t.Fatalf("file records left: %v, want %v", left, want)
	}
}

func TestChooseSource(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	dbURL, _ := newAdminTestDatabase(t)
	rdb := newRedis(mr.Addr())
	defer rdb.Close()

	label := func(mode, url string) (string, error) {
		src, closeFn, err := chooseSource(ctx, mode, url, rdb, mr.Addr())
		if err != nil {
			return "", err
		}
		defer closeFn()
		return src.Label(), nil
	}
	if l, err := label("", ""); err != nil || !strings.HasPrefix(l, "Redis") {
		t.Errorf("no database url: %q %v", l, err)
	}
	if l, err := label("", dbURL); err != nil || l != "Postgres" {
		t.Errorf("with a database url the default is Postgres: %q %v", l, err)
	}
	if l, err := label("redis", dbURL); err != nil || !strings.HasPrefix(l, "Redis") {
		t.Errorf("redis on purpose: %q %v", l, err)
	}
	if _, err := label("postgres", ""); err == nil {
		t.Error("postgres without a url was accepted")
	}
	if _, err := label("mysql", dbURL); err == nil {
		t.Error("an unknown store was accepted")
	}
	if _, err := label("postgres", "postgres://nobody@127.0.0.1:1/none?connect_timeout=1"); err == nil {
		t.Error("an unreachable database was accepted")
	}
}
