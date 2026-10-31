package main

import (
	"context"
	"testing"
)

// These tests pin down what the schema promises, so a later migration cannot
// quietly break it.

func TestSchemaDeletingAChannelRemovesEverythingInIt(t *testing.T) {
	pool := migratedTestDatabase(t)
	mustExec(t, pool, `INSERT INTO channels(id,name,created_by) VALUES ('c1','general','system'),('c2','other','alice')`)
	mustExec(t, pool, `INSERT INTO channel_members(channel_id,username,position) VALUES ('c1','alice',0)`)
	mustExec(t, pool, `INSERT INTO messages(id,channel_id,author,body) VALUES ('m1','c1','alice','hi'),('m2','c2','bob','keep me')`)
	mustExec(t, pool, `INSERT INTO messages(id,channel_id,thread_id,author,body) VALUES ('r1','c1','m1','bob','a reply')`)
	mustExec(t, pool, `INSERT INTO reactions(message_id,emoji,username) VALUES ('m1','👍','bob'),('m2','👍','bob')`)
	mustExec(t, pool, `INSERT INTO pins(message_id,channel_id,pinned_by) VALUES ('m1','c1','alice'),('m2','c2','bob')`)
	mustExec(t, pool, `INSERT INTO channel_reads VALUES ('alice','c1',now()),('alice','c2',now())`)
	mustExec(t, pool, `INSERT INTO thread_reads VALUES ('alice','m1',now())`)
	mustExec(t, pool, `INSERT INTO events(id,channel_id,title,created_by,start_at,end_at) VALUES ('e1','c1','standup','alice',now(),now()+interval '1 hour')`)
	mustExec(t, pool, `INSERT INTO event_rsvps VALUES ('e1','bob','going')`)
	mustExec(t, pool, `INSERT INTO event_reminders(event_id,username) VALUES ('e1','bob')`)

	mustExec(t, pool, `DELETE FROM channels WHERE id='c1'`)

	for table, want := range map[string]int{
		"messages": 1, "reactions": 1, "pins": 1, "channel_reads": 1,
		"channel_members": 0, "thread_reads": 0, "events": 0, "event_rsvps": 0, "event_reminders": 0,
	} {
		if got := mustCount(t, pool, "SELECT count(*) FROM "+table); got != want {
			t.Errorf("%s: %d rows left after deleting the channel, want %d", table, got, want)
		}
	}
}

func TestSchemaDeletingAMessageRemovesItsRepliesReactionsAndPin(t *testing.T) {
	pool := migratedTestDatabase(t)
	mustExec(t, pool, `INSERT INTO channels(id,name,created_by) VALUES ('c1','general','system')`)
	mustExec(t, pool, `INSERT INTO messages(id,channel_id,author) VALUES ('m1','c1','alice'),('m2','c1','alice')`)
	mustExec(t, pool, `INSERT INTO messages(id,channel_id,thread_id,author) VALUES ('r1','c1','m1','bob')`)
	mustExec(t, pool, `INSERT INTO reactions(message_id,emoji,username) VALUES ('m1','👍','bob')`)
	mustExec(t, pool, `INSERT INTO pins(message_id,channel_id,pinned_by) VALUES ('m1','c1','alice')`)

	mustExec(t, pool, `DELETE FROM messages WHERE id='m1'`)

	if n := mustCount(t, pool, "SELECT count(*) FROM messages"); n != 1 {
		t.Errorf("messages left: %d, want only m2", n)
	}
	for _, table := range []string{"reactions", "pins"} {
		if n := mustCount(t, pool, "SELECT count(*) FROM "+table); n != 0 {
			t.Errorf("%s still has %d rows", table, n)
		}
	}
}

func TestSchemaRefusesBadData(t *testing.T) {
	pool := migratedTestDatabase(t)
	ctx := context.Background()
	mustExec(t, pool, `INSERT INTO users(id,username,email) VALUES ('u1','alice','Alice@x.com')`)
	mustExec(t, pool, `INSERT INTO users(id,username,email,provider,provider_id) VALUES ('u9','alice-g','alice@x.com','google','sub-1')`)
	mustExec(t, pool, `INSERT INTO channels(id,name,created_by) VALUES ('c1','general','system')`)
	mustExec(t, pool, `INSERT INTO messages(id,channel_id,author) VALUES ('m1','c1','alice')`)
	mustExec(t, pool, `INSERT INTO events(id,channel_id,title,created_by,start_at,end_at) VALUES ('e1','c1','t','alice',now(),now()+interval '1 hour')`)
	mustExec(t, pool, `INSERT INTO reactions(message_id,emoji,username) VALUES ('m1','👍','alice')`)

	bad := map[string]string{
		"same username twice":                 `INSERT INTO users(id,username,email) VALUES ('u2','alice','other@x.com')`,
		"same email in another case":          `INSERT INTO users(id,username,email) VALUES ('u2','alice2','alice@X.COM')`,
		"empty username":                      `INSERT INTO users(id,username,email) VALUES ('u2','','z@x.com')`,
		"same channel name twice":             `INSERT INTO channels(id,name,created_by) VALUES ('c2','general','alice')`,
		"message in a missing channel":        `INSERT INTO messages(id,channel_id,author) VALUES ('m2','nope','alice')`,
		"reply to a missing message":          `INSERT INTO messages(id,channel_id,thread_id,author) VALUES ('m2','c1','nope','alice')`,
		"message that is its own parent":      `INSERT INTO messages(id,channel_id,thread_id,author) VALUES ('m2','c1','m2','alice')`,
		"the same reaction twice":             `INSERT INTO reactions(message_id,emoji,username) VALUES ('m1','👍','alice')`,
		"an empty emoji":                      `INSERT INTO reactions(message_id,emoji,username) VALUES ('m1','','bob')`,
		"the same message pinned twice":       `INSERT INTO pins(message_id,channel_id,pinned_by) VALUES ('m1','c1','a'),('m1','c1','b')`,
		"an event that ends before it starts": `INSERT INTO events(id,channel_id,title,created_by,start_at,end_at) VALUES ('e2','c1','t','a',now(),now()-interval '1 hour')`,
		"an unknown RSVP answer":              `INSERT INTO event_rsvps VALUES ('e1','alice','sure')`,
		"a Google link to a missing user":     `INSERT INTO google_links(sub,username) VALUES ('g1','ghost')`,
	}
	for name, sql := range bad {
		if _, err := pool.Exec(ctx, sql); err == nil {
			t.Errorf("the database accepted %s", name)
		}
	}
}

func TestSchemaKeepsMessagesInInsertionOrder(t *testing.T) {
	pool := migratedTestDatabase(t)
	ctx := context.Background()
	mustExec(t, pool, `INSERT INTO channels(id,name,created_by) VALUES ('c1','general','system')`)
	// Same timestamp on purpose: only seq can tell the order.
	mustExec(t, pool, `INSERT INTO messages(id,channel_id,author,created_at) VALUES
		('zzz','c1','a','2026-01-01'),('aaa','c1','a','2026-01-01'),('mmm','c1','a','2026-01-01')`)

	rows, err := pool.Query(ctx, `SELECT id FROM messages WHERE channel_id='c1' AND thread_id IS NULL ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		got = append(got, id)
	}
	if len(got) != 3 || got[0] != "zzz" || got[1] != "aaa" || got[2] != "mmm" {
		t.Fatalf("order must follow insertion, got %v", got)
	}
}

func TestSchemaSearchFindsSubstringsCaseInsensitively(t *testing.T) {
	pool := migratedTestDatabase(t)
	mustExec(t, pool, `INSERT INTO channels(id,name,created_by) VALUES ('c1','general','system')`)
	mustExec(t, pool, `INSERT INTO messages(id,channel_id,author,body) VALUES
		('m1','c1','a','Deploy finished OK'),('m2','c1','a','lunch?'),('m3','c1','a','Привет, команда')`)

	if n := mustCount(t, pool, `SELECT count(*) FROM messages WHERE body ILIKE '%deploy%'`); n != 1 {
		t.Errorf("latin search: %d", n)
	}
	if n := mustCount(t, pool, `SELECT count(*) FROM messages WHERE body ILIKE '%ПРИВЕТ%'`); n != 1 {
		t.Errorf("cyrillic search: %d", n)
	}
}
