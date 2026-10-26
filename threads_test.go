package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// A channel message to reply to; returns its id.
func seedParent(t *testing.T, e calEnv, channel, user string) string {
	t.Helper()
	id, err := e.a.SendMessage(e.tk[user], user, "parent text", channel)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func channelMsg(t *testing.T, channel, id string) Message {
	t.Helper()
	m, err := findChannelMessage(channel, id)
	if err != nil {
		t.Fatal(err)
	}
	return *m
}

func TestReplyBasics(t *testing.T) {
	e := setupCalendar(t)
	p := seedParent(t, e, "general", "alice")

	r1, err := e.a.SendReply(e.tk["bob"], "bob", "general", p, "first")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if _, err := e.a.SendReply(e.tk["alice"], "alice", "general", p, "second"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.SendReply(e.tk["bob"], "bob", "general", p, "third"); err != nil {
		t.Fatal(err)
	}

	if r1.ThreadID != p || r1.Channel != "general" {
		t.Errorf("reply fields wrong: %+v", r1)
	}

	th, err := e.a.GetThread(e.tk["carol"], "general", p)
	if err != nil {
		t.Fatal(err)
	}
	if th.Parent.ID != p || len(th.Replies) != 3 {
		t.Fatalf("thread wrong: %+v", th)
	}
	for i, want := range []string{"first", "second", "third"} {
		if th.Replies[i].Text != want {
			t.Errorf("reply %d = %q, want %q (order)", i, th.Replies[i].Text, want)
		}
	}

	// the parent in the channel carries the summary
	par := channelMsg(t, "general", p)
	if par.ReplyCount != 3 {
		t.Errorf("ReplyCount = %d, want 3", par.ReplyCount)
	}
	last, err := time.Parse(time.RFC3339Nano, par.LastReplyAt)
	if err != nil || !last.Equal(th.Replies[2].Timestamp) {
		t.Errorf("LastReplyAt = %q (%v), want the time of the newest reply %v", par.LastReplyAt, err, th.Replies[2].Timestamp)
	}
	if strings.Join(par.ReplyUsers, ",") != "bob,alice" {
		t.Errorf("ReplyUsers = %v, want [bob alice] (distinct, first-reply order)", par.ReplyUsers)
	}
}

func TestRepliesStayOutOfTheChannel(t *testing.T) {
	e := setupCalendar(t)
	p := seedParent(t, e, "general", "alice")
	e.a.SendReply(e.tk["bob"], "bob", "general", p, "hidden from channel")

	msgs, _ := e.a.GetMessages(e.tk["bob"], "general")
	if len(msgs) != 1 {
		t.Fatalf("channel has %d messages, want just the parent", len(msgs))
	}
	page, _ := e.a.GetMessagesPage(e.tk["bob"], "general", "", 50)
	if len(page.Messages) != 1 {
		t.Errorf("page has %d messages, want 1", len(page.Messages))
	}
	counts, _ := e.a.GetUnreadCounts(e.tk["carol"], "carol")
	if counts["general"] != 0 {
		t.Errorf("a reply must not count as an unread channel message, got %d", counts["general"])
	}
}

func TestReplyValidation(t *testing.T) {
	e := setupCalendar(t)
	p := seedParent(t, e, "general", "alice")
	call, _ := e.a.LogCall(e.tk["alice"], "general", "alice", "completed", 5)

	cases := []struct {
		name                         string
		token, user, channel, parent string
		text                         string
	}{
		{"no token", "", "bob", "general", p, "x"},
		{"bad token", "nope", "bob", "general", p, "x"},
		{"someone elses token", e.tk["alice"], "bob", "general", p, "x"},
		{"empty", e.tk["bob"], "bob", "general", p, ""},
		{"blank", e.tk["bob"], "bob", "general", p, "  \n "},
		{"no such parent", e.tk["bob"], "bob", "general", "nope", "x"},
		{"call log", e.tk["bob"], "bob", "general", call.ID, "x"},
	}
	for _, c := range cases {
		if _, err := e.a.SendReply(c.token, c.user, c.channel, c.parent, c.text); err == nil {
			t.Errorf("%s: expected an error", c.name)
		}
	}
	if th, _ := e.a.GetThread(e.tk["alice"], "general", p); len(th.Replies) != 0 {
		t.Errorf("rejected replies were saved: %+v", th.Replies)
	}
	if par := channelMsg(t, "general", p); par.ReplyCount != 0 {
		t.Errorf("rejected replies changed the counter: %d", par.ReplyCount)
	}
}

func TestThreadsRespectPrivateChannels(t *testing.T) {
	e := setupCalendar(t) // "secret" = alice + bob, carol is outside
	p := seedParent(t, e, "secret", "alice")
	if _, err := e.a.SendReply(e.tk["bob"], "bob", "secret", p, "inside"); err != nil {
		t.Fatal(err)
	}

	if _, err := e.a.SendReply(e.tk["carol"], "carol", "secret", p, "intruder"); err == nil {
		t.Error("an outsider replied in a private channel")
	}
	if _, err := e.a.GetThread(e.tk["carol"], "secret", p); err == nil {
		t.Error("an outsider read a private thread")
	}
	r, _ := e.a.GetThread(e.tk["alice"], "secret", p)
	if len(r.Replies) != 1 {
		t.Fatal("setup failed")
	}
	id := r.Replies[0].ID
	if _, err := e.a.EditReply(e.tk["carol"], "secret", p, id, "carol", "x"); err == nil {
		t.Error("an outsider edited a reply")
	}
	if err := e.a.DeleteReply(e.tk["carol"], "secret", p, id, "carol"); err == nil {
		t.Error("an outsider deleted a reply")
	}
	if err := e.a.AddReplyReaction(e.tk["carol"], "secret", p, id, "👍", "carol"); err == nil {
		t.Error("an outsider reacted in a private thread")
	}
}

func TestEditReply(t *testing.T) {
	e := setupCalendar(t)
	p := seedParent(t, e, "general", "alice")
	r, _ := e.a.SendReply(e.tk["bob"], "bob", "general", p, "typo")

	if _, err := e.a.EditReply(e.tk["alice"], "general", p, r.ID, "alice", "hijack"); err == nil {
		t.Error("edited someone else's reply")
	}
	if _, err := e.a.EditReply(e.tk["bob"], "general", p, r.ID, "bob", "  "); err == nil {
		t.Error("blank edit accepted")
	}
	got, err := e.a.EditReply(e.tk["bob"], "general", p, r.ID, "bob", "fixed")
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "fixed" || !got.IsEdited {
		t.Errorf("edit result wrong: %+v", got)
	}
	th, _ := e.a.GetThread(e.tk["bob"], "general", p)
	if th.Replies[0].Text != "fixed" || !th.Replies[0].IsEdited || len(th.Replies) != 1 {
		t.Errorf("edit not saved: %+v", th.Replies)
	}
}

func TestReplyReactionsToggle(t *testing.T) {
	e := setupCalendar(t)
	p := seedParent(t, e, "general", "alice")
	r, _ := e.a.SendReply(e.tk["bob"], "bob", "general", p, "hi")

	get := func() map[string][]string {
		th, _ := e.a.GetThread(e.tk["bob"], "general", p)
		return th.Replies[0].Reactions
	}
	e.a.AddReplyReaction(e.tk["alice"], "general", p, r.ID, "👍", "alice")
	e.a.AddReplyReaction(e.tk["carol"], "general", p, r.ID, "👍", "carol")
	if got := get()["👍"]; len(got) != 2 {
		t.Fatalf("reactions = %v, want 2", got)
	}
	e.a.AddReplyReaction(e.tk["alice"], "general", p, r.ID, "👍", "alice") // toggle off
	if got := get()["👍"]; len(got) != 1 || got[0] != "carol" {
		t.Errorf("after toggle: %v", got)
	}
	e.a.AddReplyReaction(e.tk["carol"], "general", p, r.ID, "👍", "carol")
	if _, still := get()["👍"]; still {
		t.Error("an emoji with no users must be removed")
	}
}

func TestDeleteReplyUpdatesParent(t *testing.T) {
	e := setupCalendar(t)
	p := seedParent(t, e, "general", "alice")
	r1, _ := e.a.SendReply(e.tk["bob"], "bob", "general", p, "one")
	r2, _ := e.a.SendReply(e.tk["carol"], "carol", "general", p, "two")

	if err := e.a.DeleteReply(e.tk["alice"], "general", p, r1.ID, "alice"); err == nil {
		t.Error("deleted someone else's reply")
	}
	if err := e.a.DeleteReply(e.tk["bob"], "general", p, "nope", "bob"); err == nil {
		t.Error("deleting a missing reply succeeded")
	}
	if err := e.a.DeleteReply(e.tk["bob"], "general", p, r1.ID, "bob"); err != nil {
		t.Fatal(err)
	}
	th, _ := e.a.GetThread(e.tk["bob"], "general", p)
	if len(th.Replies) != 1 || th.Replies[0].ID != r2.ID {
		t.Fatalf("wrong reply removed: %+v", th.Replies)
	}
	par := channelMsg(t, "general", p)
	if par.ReplyCount != 1 || strings.Join(par.ReplyUsers, ",") != "carol" {
		t.Errorf("parent summary not refreshed: count=%d users=%v", par.ReplyCount, par.ReplyUsers)
	}

	e.a.DeleteReply(e.tk["carol"], "general", p, r2.ID, "carol")
	par = channelMsg(t, "general", p)
	if par.ReplyCount != 0 || par.LastReplyAt != "" || len(par.ReplyUsers) != 0 {
		t.Errorf("empty thread must clear the summary: %+v", par)
	}
}

func TestDeletingParentRemovesThread(t *testing.T) {
	e := setupCalendar(t)
	p := seedParent(t, e, "general", "alice")
	e.a.SendReply(e.tk["bob"], "bob", "general", p, "x")
	if !miniHas(t, threadKey("general", p)) {
		t.Fatal("setup: thread list missing")
	}
	if err := e.a.DeleteMessage(e.tk["alice"], p, "general", "alice"); err != nil {
		t.Fatal(err)
	}
	if miniHas(t, threadKey("general", p)) {
		t.Error("thread list survived its parent")
	}
	if _, err := e.a.GetThread(e.tk["bob"], "general", p); err == nil {
		t.Error("thread of a deleted message is still readable")
	}
	if _, err := e.a.SendReply(e.tk["bob"], "bob", "general", p, "late"); err == nil {
		t.Error("replied to a deleted message")
	}
}

func TestDeletingChannelRemovesItsThreads(t *testing.T) {
	e := setupCalendar(t)
	e.a.CreateChannel(e.tk["alice"], "temp", "", "alice", false, nil)
	other := seedParent(t, e, "general", "alice")
	e.a.SendReply(e.tk["bob"], "bob", "general", other, "keep me")
	p := seedParent(t, e, "temp", "alice")
	e.a.SendReply(e.tk["bob"], "bob", "temp", p, "x")

	if err := e.a.DeleteChannel(e.tk["alice"], "temp", "alice"); err != nil {
		t.Fatal(err)
	}
	if miniHas(t, threadKey("temp", p)) || miniHas(t, threadIndexKey("temp")) {
		t.Error("thread data of a deleted channel was left behind")
	}
	if !miniHas(t, threadKey("general", other)) {
		t.Error("another channel's thread was removed")
	}
}

func TestThreadSizeLimit(t *testing.T) {
	e := setupCalendar(t)
	p := seedParent(t, e, "general", "alice")
	for i := 0; i < maxRepliesPerThread; i++ {
		raw, _ := json.Marshal(Message{ID: "r", User: "bob", Channel: "general", ThreadID: p})
		redisClient.RPush(ctx, threadKey("general", p), raw)
	}
	if _, err := e.a.SendReply(e.tk["bob"], "bob", "general", p, "one too many"); err == nil {
		t.Error("a full thread accepted another reply")
	}
}

func TestParentEditsKeepThreadSummary(t *testing.T) {
	e := setupCalendar(t)
	p := seedParent(t, e, "general", "alice")
	e.a.SendReply(e.tk["bob"], "bob", "general", p, "x")

	if _, err := e.a.EditMessage(e.tk["alice"], p, "general", "alice", "edited parent"); err != nil {
		t.Fatal(err)
	}
	e.a.AddReaction(e.tk["bob"], p, "👍", "bob", "general")
	par := channelMsg(t, "general", p)
	if par.ReplyCount != 1 || par.Text != "edited parent" || len(par.Reactions["👍"]) != 1 {
		t.Errorf("editing/reacting to the parent lost data: %+v", par)
	}
}

// ───────────────────────── live updates ─────────────────────────

func listen(e calEnv, name string) chan []byte {
	c := &Client{Username: name, Send: make(chan []byte, 32)}
	e.a.hub.mutex.Lock()
	e.a.hub.clients[name] = c
	e.a.hub.mutex.Unlock()
	return c.Send
}

func drain(ch chan []byte) []WSMessage {
	var out []WSMessage
	for {
		select {
		case raw := <-ch:
			var m WSMessage
			if json.Unmarshal(raw, &m) == nil {
				out = append(out, m)
			}
		default:
			return out
		}
	}
}

func countType(msgs []WSMessage, typ string) int {
	n := 0
	for _, m := range msgs {
		if m.Type == typ {
			n++
		}
	}
	return n
}

func TestLiveThreadEvents(t *testing.T) {
	e := setupCalendar(t)
	bob := listen(e, "bob")
	carol := listen(e, "carol")
	p := seedParent(t, e, "general", "alice")
	drain(bob)
	drain(carol)

	r, _ := e.a.SendReply(e.tk["bob"], "bob", "general", p, "live")
	got := drain(carol)
	if countType(got, "thread_reply") != 1 {
		t.Errorf("carol should get one thread_reply, got %v", got)
	}
	if countType(got, "channel_message") != 1 {
		t.Errorf("carol should get the updated parent as channel_message, got %v", got)
	}

	e.a.EditReply(e.tk["bob"], "general", p, r.ID, "bob", "edited")
	if countType(drain(carol), "thread_reply") != 1 {
		t.Error("an edit must be announced as thread_reply (upsert)")
	}
	e.a.DeleteReply(e.tk["bob"], "general", p, r.ID, "bob")
	if countType(drain(carol), "thread_reply_deleted") != 1 {
		t.Error("a delete must be announced as thread_reply_deleted")
	}
}

func TestPrivateThreadEventsOnlyReachMembers(t *testing.T) {
	e := setupCalendar(t) // secret = alice + bob
	alice := listen(e, "alice")
	carol := listen(e, "carol")
	p := seedParent(t, e, "secret", "alice")
	drain(alice)
	drain(carol)

	e.a.SendReply(e.tk["bob"], "bob", "secret", p, "private talk")
	if len(drain(carol)) != 0 {
		t.Error("a private thread event leaked to a non-member")
	}
	if countType(drain(alice), "thread_reply") != 1 {
		t.Error("the member did not get the event")
	}
}

func miniHas(t *testing.T, key string) bool {
	t.Helper()
	n, err := redisClient.Exists(ctx, key).Result()
	if err != nil {
		t.Fatal(err)
	}
	return n > 0
}
