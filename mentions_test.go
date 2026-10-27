package main

import (
	"testing"
	"time"
)

func TestMentionsUserRule(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"@alice", true},
		{"hey @alice", true},
		{"hey @alice, look", true},
		{"@alice!", true},
		{"is that you @alice?", true},
		{"@alice\nnext line", true},
		{"@alicebob", false},
		{"mail@alice", false},
		{"(@alice)", false},
		{"@Alice", false}, // same case rule as the highlight in the app
		{"no mention", false},
		{"", false},
	}
	for _, c := range cases {
		if got := mentionsUser(c.text, "alice"); got != c.want {
			t.Errorf("%q: got %v, want %v", c.text, got, c.want)
		}
	}
	if mentionsUser("@ ", "") || mentionsUser("anything", "") {
		t.Error("an empty username is never mentioned")
	}
	if !mentionsUser("hi @a.b", "a.b") || mentionsUser("hi @axb", "a.b") {
		t.Error("the name must be matched literally, not as a pattern")
	}
}

func TestMentionCounts(t *testing.T) {
	e := setupCalendar(t)
	// alice has opened both channels, so her starting point is set
	_ = SaveLastRead("alice", "general")
	_ = SaveLastRead("alice", "secret")
	time.Sleep(5 * time.Millisecond)

	send := func(user, channel, text string) {
		t.Helper()
		time.Sleep(3 * time.Millisecond)
		if _, err := e.a.SendMessage(e.tk[user], user, text, channel); err != nil {
			t.Fatal(err)
		}
	}
	send("bob", "general", "hey @alice")
	send("bob", "general", "@alice again")
	send("carol", "general", "@alicebob is someone else")
	send("bob", "general", "no mention")
	send("alice", "general", "@alice talking to myself")
	send("bob", "secret", "@alice private hello")

	got, err := e.a.GetMentionCounts(e.tk["alice"], "alice")
	if err != nil {
		t.Fatal(err)
	}
	if got["general"] != 2 || got["secret"] != 1 || len(got) != 2 {
		t.Fatalf("alice: %v", got)
	}

	// carol is not in the private channel and is mentioned nowhere
	_ = SaveLastRead("carol", "general")
	got, _ = e.a.GetMentionCounts(e.tk["carol"], "carol")
	if len(got) != 0 {
		t.Fatalf("carol: %v", got)
	}

	// reading the channel clears it
	time.Sleep(3 * time.Millisecond)
	_ = SaveLastRead("alice", "general")
	got, _ = e.a.GetMentionCounts(e.tk["alice"], "alice")
	if got["general"] != 0 || got["secret"] != 1 {
		t.Fatalf("after reading general: %v", got)
	}
}

func TestMentionCountsNeedsAValidSession(t *testing.T) {
	e := setupCalendar(t)
	if _, err := e.a.GetMentionCounts("", "alice"); err == nil {
		t.Fatal("no token must be refused")
	}
	if _, err := e.a.GetMentionCounts(e.tk["bob"], "alice"); err == nil {
		t.Fatal("someone else's token must be refused")
	}
}

func TestMentionCountsSkipChannelsNeverOpened(t *testing.T) {
	e := setupCalendar(t)
	if _, err := e.a.SendMessage(e.tk["bob"], "bob", "@alice hello", "general"); err != nil {
		t.Fatal(err)
	}
	got, _ := e.a.GetMentionCounts(e.tk["alice"], "alice")
	if len(got) != 0 {
		t.Fatalf("no starting point yet, so nothing to count: %v", got)
	}
}
