package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

const (
	idUsed   = "11111111-1111-4111-8111-111111111111"
	idOrphan = "22222222-2222-4222-8222-222222222222"
	idNew    = "33333333-3333-4333-8333-333333333333"
)

func runCLI(t *testing.T, mr *miniredis.Miniredis, stdin string, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(append([]string{"-addr", mr.Addr()}, args...), strings.NewReader(stdin), &out, &errOut)
	return out.String(), errOut.String(), code
}

func TestUsersAndAuth(t *testing.T) {
	mr := miniredis.RunT(t)
	put := func(key string, v any) { b, _ := json.Marshal(v); mr.Set(key, string(b)) }
	put("user:bob@x.com", map[string]any{"username": "bob", "email": "bob@x.com", "status": "online", "lastSeen": "2026-10-01T10:00:00Z"})
	put("user:123@google", map[string]any{"username": "alice", "email": "alice@gmail.com", "provider": "google", "status": "away"})
	mr.Set("user:bob@x.com:password", "$2a$hash")
	mr.Set("user:bob:lastread:general", "2026-01-01T00:00:00Z")

	out, _, code := runCLI(t, mr, "", "users")
	if code != 0 || !strings.Contains(out, "alice") || !strings.Contains(out, "google") ||
		!strings.Contains(out, "bob") || !strings.Contains(out, "password") || !strings.Contains(out, "2 accounts") {
		t.Fatalf("users output:\n%s", out)
	}
	if strings.Contains(out, "$2a$hash") {
		t.Fatal("password hash leaked")
	}
}

func TestLockedUnlock(t *testing.T) {
	mr := miniredis.RunT(t)
	out, _, _ := runCLI(t, mr, "", "locked")
	if !strings.Contains(out, "No failed login attempts") {
		t.Fatalf("empty: %s", out)
	}

	mr.Set("loginfail:bob@x.com", "5")
	mr.SetTTL("loginfail:bob@x.com", 10*time.Minute)
	mr.Set("loginfail:eve@x.com", "2")
	out, _, _ = runCLI(t, mr, "", "locked")
	if !strings.Contains(out, "LOCKED") || !strings.Contains(out, "counting") {
		t.Fatalf("locked:\n%s", out)
	}

	out, _, code := runCLI(t, mr, "", "unlock", " Bob@X.com ")
	if code != 0 || !strings.Contains(out, "unlocked") || mr.Exists("loginfail:bob@x.com") {
		t.Fatalf("unlock: %s", out)
	}
	out, _, _ = runCLI(t, mr, "", "unlock", "nobody@x.com")
	if !strings.Contains(out, "no failed attempts") {
		t.Fatalf("unlock none: %s", out)
	}
	if _, _, code := runCLI(t, mr, "", "unlock"); code == 0 {
		t.Fatal("unlock without email must fail")
	}
}

func TestSessionsRevoke(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.Set("session:aaa", "bob")
	mr.Set("session:bbb", "bob")
	mr.Set("session:ccc", "alice")

	out, _, _ := runCLI(t, mr, "", "sessions")
	if !strings.Contains(out, "3 sessions, 2 users") || strings.Contains(out, "aaa") {
		t.Fatalf("sessions:\n%s", out)
	}
	out, _, _ = runCLI(t, mr, "", "revoke", "bob")
	if !strings.Contains(out, "Revoked 2") || mr.Exists("session:aaa") || mr.Exists("session:bbb") || !mr.Exists("session:ccc") {
		t.Fatalf("revoke: %s", out)
	}
}

func setupUploads(t *testing.T, mr *miniredis.Miniredis) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "uploads")
	os.MkdirAll(dir, 0755)
	old := time.Now().Add(-72 * time.Hour)
	write := func(name string, mod time.Time) string {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte("data"), 0644)
		os.Chtimes(p, mod, mod)
		return p
	}
	write(idUsed+"_used.png", old)
	write(idOrphan+"_orphan.png", old)
	write(idNew+"_fresh.png", time.Now())
	write("notes.txt", old)

	msg, _ := json.Marshal(map[string]any{"id": "m1", "isFile": true, "fileId": idUsed})
	mr.RPush("channel:general:messages", string(msg), "DELETED")
	mr.HSet("file:"+idUsed, "path", filepath.Join(dir, idUsed+"_used.png"))
	mr.HSet("file:"+idOrphan, "path", filepath.Join(dir, idOrphan+"_orphan.png"))
	mr.HSet("file:gone", "path", filepath.Join(dir, "gone_does_not_exist.png"))
	return dir
}

func TestUploadsDryRunAndClean(t *testing.T) {
	mr := miniredis.RunT(t)
	dir := setupUploads(t, mr)

	out, errOut, code := runCLI(t, mr, "", "uploads", "-dir", dir)
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errOut)
	}
	for _, want := range []string{"In use:           1 files", "Orphans:          1 files", "Too new to judge: 1 files", "Unknown names:    1 files", "Stale metadata:   1 entries", "Dry run"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, idOrphan+"_orphan.png")); err != nil {
		t.Fatal("dry run deleted something")
	}

	// answering "n" deletes nothing
	out, _, _ = runCLI(t, mr, "n\n", "uploads", "-dir", dir, "-clean")
	if !strings.Contains(out, "Cancelled") {
		t.Fatalf("expected cancel:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, idOrphan+"_orphan.png")); err != nil {
		t.Fatal("cancel deleted something")
	}

	// confirm
	out, _, _ = runCLI(t, mr, "y\n", "uploads", "-dir", dir, "-clean")
	if !strings.Contains(out, "Deleted 1 files and 1 metadata") {
		t.Fatalf("clean:\n%s", out)
	}
	exists := func(n string) bool { _, err := os.Stat(filepath.Join(dir, n)); return err == nil }
	if exists(idOrphan+"_orphan.png") || !exists(idUsed+"_used.png") || !exists(idNew+"_fresh.png") || !exists("notes.txt") {
		t.Fatal("wrong files deleted")
	}
	if mr.Exists("file:"+idOrphan) || mr.Exists("file:gone") || !mr.Exists("file:"+idUsed) {
		t.Fatal("wrong metadata deleted")
	}
}

func TestUploadsBadDirAndUnknownCommand(t *testing.T) {
	mr := miniredis.RunT(t)
	if _, errOut, code := runCLI(t, mr, "", "uploads", "-dir", "/definitely/not/here"); code == 0 || !strings.Contains(errOut, "not a directory") {
		t.Fatalf("bad dir: code=%d %s", code, errOut)
	}
	if _, _, code := runCLI(t, mr, "", "bogus"); code != 2 {
		t.Fatal("unknown command should exit 2")
	}
	if _, _, code := runCLI(t, mr, ""); code != 2 {
		t.Fatal("no command should exit 2")
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"-addr", "127.0.0.1:1", "users"}, strings.NewReader(""), &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "cannot connect") {
		t.Fatalf("unreachable redis: %d %s", code, errOut.String())
	}
}

func TestFileIDFromName(t *testing.T) {
	if id, ok := fileIDFromName(idUsed + "_a b.png"); !ok || id != idUsed {
		t.Fatal("valid name rejected")
	}
	for _, bad := range []string{"notes.txt", idUsed + ".png", "zzzzzzzz-1111-4111-8111-111111111111_x", idUsed + "_"} {
		if _, ok := fileIDFromName(bad); ok {
			t.Errorf("accepted %q", bad)
		}
	}
}
