package main

import (
	"strings"
	"testing"
	"time"
)

func reply(t *testing.T, e calEnv, user, channel, parent, text string) {
	t.Helper()
	time.Sleep(3 * time.Millisecond) // distinct timestamps
	if _, err := e.a.SendReply(e.tk[user], user, channel, parent, text); err != nil {
		t.Fatal(err)
	}
}

func unread(t *testing.T, e calEnv, user, channel string) map[string]int {
	t.Helper()
	m, err := e.a.GetThreadUnread(e.tk[user], channel)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestThreadUnreadLifecycle(t *testing.T) {
	e := setupCalendar(t)
	p := seedParent(t, e, "general", "alice")
	reply(t, e, "bob", "general", p, "old reply")

	// First look at alice: what is already there counts as seen.
	if got := unread(t, e, "alice", "general"); len(got) != 0 {
		t.Fatalf("old replies must not be unread: %v", got)
	}

	reply(t, e, "bob", "general", p, "new 1")
	reply(t, e, "carol", "general", p, "new 2")
	reply(t, e, "alice", "general", p, "my own") // never unread for me
	if got := unread(t, e, "alice", "general"); got[p] != 2 {
		t.Fatalf("want 2 unread, got %v", got)
	}

	// Somebody else has their own count.
	_ = unread(t, e, "bob", "general")
	reply(t, e, "carol", "general", p, "for bob")
	if got := unread(t, e, "bob", "general"); got[p] != 1 {
		t.Fatalf("bob: want 1 unread, got %v", got)
	}

	time.Sleep(3 * time.Millisecond)
	if err := e.a.MarkThreadRead(e.tk["alice"], "general", p); err != nil {
		t.Fatal(err)
	}
	if got := unread(t, e, "alice", "general"); len(got) != 0 {
		t.Fatalf("after reading: %v", got)
	}
	if got := unread(t, e, "bob", "general"); got[p] != 1 {
		t.Fatalf("alice reading must not change bob: %v", got)
	}

	reply(t, e, "bob", "general", p, "again")
	if got := unread(t, e, "alice", "general"); got[p] != 1 {
		t.Fatalf("after one more reply: %v", got)
	}
}

func TestThreadUnreadChecksAccess(t *testing.T) {
	e := setupCalendar(t)
	p := seedParent(t, e, "secret", "alice")
	if _, err := e.a.GetThreadUnread("", "general"); err == nil {
		t.Fatal("no token must be refused")
	}
	if _, err := e.a.GetThreadUnread(e.tk["carol"], "secret"); err == nil {
		t.Fatal("outsider must be refused")
	}
	if err := e.a.MarkThreadRead(e.tk["carol"], "secret", p); err == nil {
		t.Fatal("outsider must not mark read")
	}
	if err := e.a.MarkThreadRead("", "secret", p); err == nil {
		t.Fatal("no token must be refused")
	}
}

func TestBuildThreadExport(t *testing.T) {
	e := setupCalendar(t)
	p := seedParent(t, e, "general", "alice")
	reply(t, e, "bob", "general", p, "first reply")
	reply(t, e, "carol", "general", p, "second reply")

	f, err := e.a.BuildThreadExport(e.tk["alice"], "general", p, "txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(f.Filename, "gothermo-general-thread-") || !strings.HasSuffix(f.Filename, ".txt") {
		t.Fatalf("filename %q", f.Filename)
	}
	i, j, k := strings.Index(f.Content, "parent text"), strings.Index(f.Content, "first reply"), strings.Index(f.Content, "second reply")
	if i < 0 || !(i < j && j < k) {
		t.Fatalf("parent and replies must come in order:\n%s", f.Content)
	}

	for _, format := range []string{"json", "csv"} {
		f, err := e.a.BuildThreadExport(e.tk["alice"], "general", p, format)
		if err != nil || !strings.Contains(f.Content, "second reply") {
			t.Fatalf("%s: %v", format, err)
		}
	}
}

func TestBuildThreadExportRefusals(t *testing.T) {
	e := setupCalendar(t)
	p := seedParent(t, e, "secret", "alice")
	reply(t, e, "bob", "secret", p, "private words")

	if f, err := e.a.BuildThreadExport(e.tk["carol"], "secret", p, "txt"); err == nil || f.Content != "" {
		t.Fatalf("outsider got the thread: %v %q", err, f.Content)
	}
	if _, err := e.a.BuildThreadExport("", "secret", p, "txt"); err == nil {
		t.Fatal("no token must be refused")
	}
	if _, err := e.a.BuildThreadExport(e.tk["alice"], "secret", p, "pdf"); err == nil {
		t.Fatal("bad format must fail")
	}
	if _, err := e.a.BuildThreadExport(e.tk["alice"], "secret", "nope", "txt"); err == nil {
		t.Fatal("unknown thread must fail")
	}
}
