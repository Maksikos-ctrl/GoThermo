package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func reportsInRedis(t *testing.T) []Report {
	t.Helper()
	raw, err := redisClient.LRange(ctx, reportsKey, 0, -1).Result()
	if err != nil {
		t.Fatal(err)
	}
	var out []Report
	for _, s := range raw {
		var r Report
		if err := json.Unmarshal([]byte(s), &r); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func TestReportMessage(t *testing.T) {
	useMiniRedis(t)
	SaveChannel(Channel{Name: "general", Members: []string{}, CreatedBy: "system", CreatedAt: time.Now()})
	SaveChannel(Channel{Name: "dm_alice_bob", Members: []string{"alice", "bob"}, IsPrivate: true, CreatedBy: "system", CreatedAt: time.Now()})
	a := &App{hub: NewHub()}
	go a.hub.Run()
	ta, _ := CreateSession("alice")
	tb, _ := CreateSession("bob")
	tc, _ := CreateSession("carol")

	id, err := a.SendMessage(tb, "bob", strings.Repeat("x", 500), "general")
	if err != nil {
		t.Fatal(err)
	}

	if err := a.ReportMessage("bogus", "alice", "general", id, "spam"); err == nil {
		t.Fatal("anonymous report accepted")
	}
	if err := a.ReportMessage(ta, "alice", "general", id, "because"); err == nil {
		t.Fatal("unknown reason accepted")
	}
	if err := a.ReportMessage(tb, "bob", "general", id, "spam"); err == nil {
		t.Fatal("own message reported")
	}
	if err := a.ReportMessage(ta, "alice", "general", "no-such-id", "spam"); err == nil {
		t.Fatal("missing message accepted")
	}

	if err := a.ReportMessage(ta, "alice", "general", id, "NSFW"); err != nil {
		t.Fatal(err)
	}
	// the same person reporting again changes nothing
	if err := a.ReportMessage(ta, "alice", "general", id, "spam"); err != nil {
		t.Fatal(err)
	}
	got := reportsInRedis(t)
	if len(got) != 1 {
		t.Fatalf("want 1 report, got %d", len(got))
	}
	r := got[0]
	if r.Reporter != "alice" || r.Author != "bob" || r.Reason != "nsfw" || r.MessageID != id {
		t.Fatalf("%+v", r)
	}
	if len([]rune(r.Snippet)) > reportSnippetLen+1 {
		t.Fatalf("snippet not trimmed: %d", len(r.Snippet))
	}

	// a private chat cannot be reported from outside
	pmID, err := a.SendMessage(ta, "alice", "secret", "dm_alice_bob")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.ReportMessage(tc, "carol", "dm_alice_bob", pmID, "spam"); err == nil {
		t.Fatal("outsider reported a private message")
	}
}

func TestReportRateLimit(t *testing.T) {
	useMiniRedis(t)
	SaveChannel(Channel{Name: "general", Members: []string{}, CreatedBy: "system", CreatedAt: time.Now()})
	a := &App{hub: NewHub()}
	go a.hub.Run()
	ta, _ := CreateSession("alice")
	tb, _ := CreateSession("bob")
	var failed bool
	for i := 0; i < maxReportsPerHour+3; i++ {
		id, err := a.SendMessage(tb, "bob", "spam", "general")
		if err != nil {
			t.Fatal(err)
		}
		if err := a.ReportMessage(ta, "alice", "general", id, "spam"); err != nil {
			failed = true
		}
	}
	if !failed {
		t.Fatal("no limit on reports")
	}
	if n := len(reportsInRedis(t)); n != maxReportsPerHour {
		t.Fatalf("stored %d reports, want %d", n, maxReportsPerHour)
	}
}

func TestBannedAccountCannotUseTheApp(t *testing.T) {
	useMiniRedis(t)
	tok, _ := CreateSession("bob")
	if _, ok := GetSessionUsername(tok); !ok {
		t.Fatal("session should work before the ban")
	}
	redisClient.SAdd(ctx, bannedKey, "bob")
	if _, ok := GetSessionUsername(tok); ok {
		t.Fatal("banned user's old session still works")
	}
	if _, err := CreateSession("Bob"); err == nil {
		t.Fatal("banned user got a new session (case must not matter)")
	}
	if _, err := CreateSession("alice"); err != nil {
		t.Fatal(err)
	}
	redisClient.SRem(ctx, bannedKey, "bob")
	if _, ok := GetSessionUsername(tok); !ok {
		t.Fatal("unban should restore the session")
	}
}
