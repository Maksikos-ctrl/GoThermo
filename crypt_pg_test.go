package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Text is sealed on its way into Postgres and opened on its way out; older
// plain rows stay readable; `encrypt` and `encrypt -decrypt` convert them.
func TestMessageTextEncryptedAtRest(t *testing.T) {
	pool := migratedTestDatabase(t)
	s := pgStore{pool: pool}
	mustChannel(t, s, "general")
	base := time.Now().Add(-time.Hour)
	bg := context.Background()

	// written before the key existed
	mustAppend(t, s, Message{ID: "old", User: "alice", Text: "plain old row", Channel: "general", Timestamp: base})

	withKey(t, goodKey())
	mustAppend(t, s, Message{ID: "new", User: "bob", Text: "secret news", Channel: "general", Timestamp: base.Add(time.Second)})
	mustAppend(t, s, Message{ID: "r1", User: "bob", Text: "secret reply", Channel: "general", ThreadID: "new", Timestamp: base.Add(2 * time.Second)})

	stored := func(id string) string {
		var b string
		if err := pool.QueryRow(bg, "SELECT body FROM messages WHERE id = $1", id).Scan(&b); err != nil {
			t.Fatal(err)
		}
		return b
	}
	if stored("old") != "plain old row" {
		t.Fatal("old row must be untouched")
	}
	for _, id := range []string{"new", "r1"} {
		if b := stored(id); !strings.HasPrefix(b, encPrefix) || strings.Contains(b, "secret") {
			t.Fatalf("%s is not sealed in the database: %q", id, b)
		}
	}

	got, err := s.RecentMessages("general", 10)
	if err != nil || len(got) != 2 {
		t.Fatalf("%v %v", got, err)
	}
	if got[0].Text != "plain old row" || got[1].Text != "secret news" {
		t.Fatalf("read back: %q / %q", got[0].Text, got[1].Text)
	}

	// an edit is sealed too
	edited := got[1]
	edited.Text = "secret news, edited"
	edited.IsEdited = true
	if err := s.UpdateMessage("general", edited); err != nil {
		t.Fatal(err)
	}
	if b := stored("new"); strings.Contains(b, "edited") || !strings.HasPrefix(b, encPrefix) {
		t.Fatalf("edit stored in the clear: %q", b)
	}

	// encrypt converts the old row; running again changes nothing
	if n, err := convertMessageText(pool, true, true); err != nil || n != 1 {
		t.Fatalf("dry run: %d %v", n, err)
	}
	if stored("old") != "plain old row" {
		t.Fatal("dry run must not write")
	}
	if n, err := convertMessageText(pool, true, false); err != nil || n != 1 {
		t.Fatalf("encrypt: %d %v", n, err)
	}
	if n, err := convertMessageText(pool, true, false); err != nil || n != 0 {
		t.Fatalf("second run: %d %v", n, err)
	}
	if b := stored("old"); !strings.HasPrefix(b, encPrefix) || strings.Contains(b, "plain") {
		t.Fatalf("old row not sealed: %q", b)
	}

	// and back
	if n, err := convertMessageText(pool, false, false); err != nil || n != 3 {
		t.Fatalf("decrypt: %d %v", n, err)
	}
	if stored("old") != "plain old row" || stored("r1") != "secret reply" {
		t.Fatal("decrypt did not restore the text")
	}
}
