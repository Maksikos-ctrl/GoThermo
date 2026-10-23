package main

import (
	"fmt"
	"testing"
	"time"
)

func fillChannel(t *testing.T, channel string, n int) []string {
	t.Helper()
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ids[i] = fmt.Sprintf("m%03d", i)
		if err := SaveMessage(Message{ID: ids[i], User: "alice", Text: ids[i], Channel: channel, Timestamp: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	return ids
}

func TestMessagePaging(t *testing.T) {
	e := setupCalendar(t)
	ids := fillChannel(t, "general", 120)
	tk := e.tk["alice"]

	// newest page
	p, err := e.a.GetMessagesPage(tk, "general", "", 50)
	if err != nil || len(p.Messages) != 50 || !p.HasMore {
		t.Fatalf("first page: %d %v %v", len(p.Messages), p.HasMore, err)
	}
	if p.Messages[0].ID != ids[70] || p.Messages[49].ID != ids[119] {
		t.Errorf("wrong window: %s .. %s", p.Messages[0].ID, p.Messages[49].ID)
	}

	// walk back to the beginning, collecting everything exactly once
	seen := map[string]bool{}
	for _, m := range p.Messages {
		seen[m.ID] = true
	}
	oldest := p.Messages[0].ID
	pages := 1
	for p.HasMore {
		p, err = e.a.GetMessagesPage(tk, "general", oldest, 50)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range p.Messages {
			if seen[m.ID] {
				t.Fatalf("%s delivered twice", m.ID)
			}
			seen[m.ID] = true
		}
		if len(p.Messages) > 0 {
			oldest = p.Messages[0].ID
		}
		pages++
		if pages > 10 {
			t.Fatal("never ends")
		}
	}
	if len(seen) != 120 || pages != 3 {
		t.Errorf("seen %d messages in %d pages", len(seen), pages)
	}
	if oldest != ids[0] {
		t.Errorf("did not reach the first message: %s", oldest)
	}
	// last page is in chronological order
	if p.Messages[0].ID != ids[0] || p.Messages[len(p.Messages)-1].ID != ids[19] {
		t.Errorf("last page: %s .. %s", p.Messages[0].ID, p.Messages[len(p.Messages)-1].ID)
	}
}

func TestMessagePagingEdges(t *testing.T) {
	e := setupCalendar(t)
	tk := e.tk["alice"]

	// empty channel
	if p, err := e.a.GetMessagesPage(tk, "general", "", 50); err != nil || len(p.Messages) != 0 || p.HasMore || p.Messages == nil {
		t.Errorf("empty: %+v %v", p, err)
	}

	ids := fillChannel(t, "general", 10)
	// exactly one page: nothing more
	if p, _ := e.a.GetMessagesPage(tk, "general", "", 10); len(p.Messages) != 10 || p.HasMore {
		t.Errorf("exact fit: %d %v", len(p.Messages), p.HasMore)
	}
	// unknown cursor
	if p, err := e.a.GetMessagesPage(tk, "general", "nope", 5); err != nil || len(p.Messages) != 0 || p.HasMore {
		t.Errorf("unknown cursor: %+v %v", p, err)
	}
	// cursor at the very first message: nothing before it
	if p, _ := e.a.GetMessagesPage(tk, "general", ids[0], 5); len(p.Messages) != 0 || p.HasMore {
		t.Errorf("before first: %+v", p)
	}
	// limit is clamped (0 and absurd values)
	if p, _ := e.a.GetMessagesPage(tk, "general", ids[9], 0); len(p.Messages) != 9 {
		t.Errorf("default limit: %d", len(p.Messages))
	}
	if p, _ := e.a.GetMessagesPage(tk, "general", ids[9], 1_000_000); len(p.Messages) != 9 {
		t.Errorf("huge limit: %d", len(p.Messages))
	}

	// deleted messages are skipped and do not break the cursor
	redisClient.LSet(ctx, "channel:general:messages", 4, "DELETED")
	p, _ := e.a.GetMessagesPage(tk, "general", ids[9], 100)
	if len(p.Messages) != 8 {
		t.Errorf("deleted should be skipped: %d", len(p.Messages))
	}
	for _, m := range p.Messages {
		if m.ID == ids[4] {
			t.Error("deleted message came back")
		}
	}
	// the cursor itself may be right after a deleted one
	if p, _ := e.a.GetMessagesPage(tk, "general", ids[5], 2); len(p.Messages) != 2 || p.Messages[0].ID != ids[2] || p.Messages[1].ID != ids[3] {
		t.Errorf("around deleted: %+v", p.Messages)
	}
}

func TestMessagePagingAccess(t *testing.T) {
	e := setupCalendar(t)
	fillChannel(t, "secret", 5)

	if _, err := e.a.GetMessagesPage("", "secret", "", 10); err == nil {
		t.Error("no token accepted")
	}
	if _, err := e.a.GetMessagesPage("nope", "general", "", 10); err == nil {
		t.Error("bad token accepted")
	}
	if p, err := e.a.GetMessagesPage(e.tk["carol"], "secret", "", 10); err == nil || len(p.Messages) != 0 {
		t.Errorf("outsider read a private channel: %+v %v", p, err)
	}
	if p, err := e.a.GetMessagesPage(e.tk["carol"], "secret", "m004", 10); err == nil || len(p.Messages) != 0 {
		t.Errorf("outsider paged a private channel: %+v %v", p, err)
	}
	if p, err := e.a.GetMessagesPage(e.tk["bob"], "secret", "", 10); err != nil || len(p.Messages) != 5 {
		t.Errorf("member: %+v %v", p, err)
	}
}
