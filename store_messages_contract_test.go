package main

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

// Contract tests for messages, threads, pins and files. Like the ones in
// store_contract_test.go they run against Redis and Postgres alike.

func mustChannel(t *testing.T, s Store, name string) {
	t.Helper()
	if err := s.SaveChannel(Channel{ID: "id-" + name, Name: name, Members: []string{}, CreatedBy: "system", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("SaveChannel(%s): %v", name, err)
	}
}

func mustAppend(t *testing.T, s Store, m Message) {
	t.Helper()
	if m.Reactions == nil {
		m.Reactions = map[string][]string{}
	}
	if err := s.AppendMessage(m); err != nil {
		t.Fatalf("AppendMessage(%s): %v", m.ID, err)
	}
}

func mustFind(t *testing.T, s Store, channel, id string) *Message {
	t.Helper()
	m, err := s.FindMessage(channel, id)
	if err != nil {
		t.Fatalf("FindMessage(%s): %v", id, err)
	}
	return m
}

func ids(msgs []Message) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.ID
	}
	return out
}

// ───────────────────────── messages ─────────────────────────

func TestStoreMessagesRoundTrip(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		mustChannel(t, s, "general")
		mustChannel(t, s, "random")
		base := time.Now().Add(-time.Hour)

		mustAppend(t, s, Message{ID: "m1", User: "alice", Text: "hello", Channel: "general", Timestamp: base})
		mustAppend(t, s, Message{ID: "m2", User: "bob", Text: "a post", Channel: "general", Timestamp: base.Add(time.Second), IsPost: true})
		mustAppend(t, s, Message{ID: "m3", User: "alice", Text: "pic.png", Channel: "general", Timestamp: base.Add(2 * time.Second),
			IsFile: true, FileID: "f1", FileName: "pic.png", FileSize: 123456, MimeType: "image/png"})
		mustAppend(t, s, Message{ID: "m4", User: "alice", Text: "call ended", Channel: "general", Timestamp: base.Add(3 * time.Second),
			IsCall: true, CallStatus: "ended", CallDuration: 65, CallInitiator: "alice"})
		mustAppend(t, s, Message{ID: "m5", User: "carol", Text: "привет 👋", Channel: "general", Timestamp: base.Add(4 * time.Second)})
		mustAppend(t, s, Message{ID: "x1", User: "bob", Text: "elsewhere", Channel: "random", Timestamp: base})

		got, err := s.RecentMessages("general", 10)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ids(got), []string{"m1", "m2", "m3", "m4", "m5"}) {
			t.Fatalf("order or content wrong: %v", ids(got))
		}
		if got[0].Channel != "general" || got[0].User != "alice" || got[0].Text != "hello" || got[0].IsPost || got[0].IsEdited {
			t.Errorf("m1: %+v", got[0])
		}
		sameInstant(t, "m1 time", got[0].Timestamp, base, time.Millisecond)
		if !got[1].IsPost {
			t.Error("m2 must stay a post")
		}
		f := got[2]
		if !f.IsFile || f.FileID != "f1" || f.FileName != "pic.png" || f.FileSize != 123456 || f.MimeType != "image/png" {
			t.Errorf("file message: %+v", f)
		}
		c := got[3]
		if !c.IsCall || c.CallStatus != "ended" || c.CallDuration != 65 || c.CallInitiator != "alice" {
			t.Errorf("call message: %+v", c)
		}
		if got[4].Text != "привет 👋" {
			t.Errorf("unicode text: %q", got[4].Text)
		}

		last2, _ := s.RecentMessages("general", 2)
		if !reflect.DeepEqual(ids(last2), []string{"m4", "m5"}) {
			t.Fatalf("the newest two, oldest first: %v", ids(last2))
		}
		if other, _ := s.RecentMessages("random", 10); !reflect.DeepEqual(ids(other), []string{"x1"}) {
			t.Fatalf("channels must not mix: %v", ids(other))
		}
		if empty, err := s.RecentMessages("nobody-here", 10); err != nil || len(empty) != 0 {
			t.Fatalf("a channel without messages: %v %v", empty, err)
		}
	})
}

func TestStoreUpdateMessageAndReactions(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		mustChannel(t, s, "general")
		mustAppend(t, s, Message{ID: "m1", User: "alice", Text: "first", Channel: "general", Timestamp: time.Now()})

		m := mustFind(t, s, "general", "m1")
		m.Text, m.IsEdited = "edited", true
		m.Reactions = map[string][]string{"👍": {"bob", "carol"}, "🎉": {"alice"}}
		if err := s.UpdateMessage("general", *m); err != nil {
			t.Fatal(err)
		}
		got := mustFind(t, s, "general", "m1")
		if got.Text != "edited" || !got.IsEdited || got.User != "alice" {
			t.Fatalf("edit lost: %+v", got)
		}
		if !reflect.DeepEqual(got.Reactions["👍"], []string{"bob", "carol"}) || !reflect.DeepEqual(got.Reactions["🎉"], []string{"alice"}) {
			t.Fatalf("reactions: %v", got.Reactions)
		}

		// carol leaves, dave joins: bob keeps his place, dave goes last
		got.Reactions = map[string][]string{"👍": {"bob", "dave"}, "🎉": {"alice"}}
		s.UpdateMessage("general", *got)
		if g := mustFind(t, s, "general", "m1"); !reflect.DeepEqual(g.Reactions["👍"], []string{"bob", "dave"}) {
			t.Fatalf("reaction order: %v", g.Reactions["👍"])
		}

		// all reactions removed
		got.Reactions = map[string][]string{}
		s.UpdateMessage("general", *got)
		if g := mustFind(t, s, "general", "m1"); len(g.Reactions) != 0 {
			t.Fatalf("reactions should be gone: %v", g.Reactions)
		}

		if err := s.UpdateMessage("general", Message{ID: "nope", Text: "x"}); err == nil {
			t.Error("updating a missing message must fail")
		}
	})
}

func TestStoreDeleteMessage(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		mustChannel(t, s, "general")
		for _, id := range []string{"m1", "m2", "m3"} {
			mustAppend(t, s, Message{ID: id, User: "alice", Text: id, Channel: "general", Timestamp: time.Now()})
		}
		if err := s.DeleteMessage("general", "m2"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.FindMessage("general", "m2"); err == nil {
			t.Fatal("deleted message still found")
		}
		got, _ := s.RecentMessages("general", 10)
		if !reflect.DeepEqual(ids(got), []string{"m1", "m3"}) {
			t.Fatalf("after delete: %v", ids(got))
		}
		if err := s.DeleteMessage("general", "m2"); err == nil {
			t.Error("deleting twice must fail")
		}
		if err := s.DeleteMessage("general", "never"); err == nil {
			t.Error("deleting a missing message must fail")
		}
	})
}

func TestStoreMessagePaging(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		mustChannel(t, s, "general")
		const total = 120
		want := make([]string, total)
		base := time.Now().Add(-time.Hour)
		for i := 0; i < total; i++ {
			want[i] = fmt.Sprintf("m%03d", i)
			mustAppend(t, s, Message{ID: want[i], User: "alice", Text: want[i], Channel: "general", Timestamp: base.Add(time.Duration(i) * time.Second)})
		}

		p, err := s.MessagePage("general", "", 50)
		if err != nil || len(p.Messages) != 50 || !p.HasMore {
			t.Fatalf("newest page: %d %v %v", len(p.Messages), p.HasMore, err)
		}
		if p.Messages[0].ID != want[70] || p.Messages[49].ID != want[119] {
			t.Fatalf("newest window: %s .. %s", p.Messages[0].ID, p.Messages[49].ID)
		}

		var collected []string
		collected = append(collected, ids(p.Messages)...)
		for pages := 0; p.HasMore; pages++ {
			if pages > 10 {
				t.Fatal("never ends")
			}
			p, err = s.MessagePage("general", p.Messages[0].ID, 50)
			if err != nil {
				t.Fatal(err)
			}
			collected = append(ids(p.Messages), collected...)
		}
		if !reflect.DeepEqual(collected, want) {
			t.Fatalf("walking back must deliver every message once, in order (got %d)", len(collected))
		}

		// exactly one page worth: nothing more
		mustChannel(t, s, "small")
		for i := 0; i < 50; i++ {
			mustAppend(t, s, Message{ID: fmt.Sprintf("s%02d", i), User: "a", Text: "x", Channel: "small", Timestamp: base.Add(time.Duration(i) * time.Second)})
		}
		if p, _ := s.MessagePage("small", "", 50); len(p.Messages) != 50 || p.HasMore {
			t.Errorf("50 of 50: %d messages, hasMore=%v", len(p.Messages), p.HasMore)
		}
		mustAppend(t, s, Message{ID: "s50", User: "a", Text: "x", Channel: "small", Timestamp: base.Add(time.Hour)})
		if p, _ := s.MessagePage("small", "", 50); len(p.Messages) != 50 || !p.HasMore {
			t.Errorf("51 messages: %d, hasMore=%v", len(p.Messages), p.HasMore)
		}

		// a cursor that does not exist (or was deleted) gives an empty page, not an error
		if p, err := s.MessagePage("general", "ghost", 50); err != nil || len(p.Messages) != 0 || p.HasMore {
			t.Errorf("unknown cursor: %+v %v", p, err)
		}
		s.DeleteMessage("general", want[60])
		if p, err := s.MessagePage("general", want[60], 50); err != nil || len(p.Messages) != 0 {
			t.Errorf("deleted cursor: %+v %v", p, err)
		}
	})
}

// ───────────────────────── threads ─────────────────────────

func TestStoreThreads(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		mustChannel(t, s, "general")
		base := time.Now().Add(-time.Hour)
		mustAppend(t, s, Message{ID: "p1", User: "alice", Text: "parent", Channel: "general", Timestamp: base})
		mustAppend(t, s, Message{ID: "p2", User: "alice", Text: "no replies here", Channel: "general", Timestamp: base})

		authors := []string{"alice", "bob", "alice", "carol", "dave", "erin"}
		for i, a := range authors {
			r := Message{ID: fmt.Sprintf("r%d", i), User: a, Text: fmt.Sprintf("reply %d", i), Channel: "general",
				ThreadID: "p1", Timestamp: base.Add(time.Duration(i+1) * time.Minute), Reactions: map[string][]string{}}
			if err := s.AddReply(r); err != nil {
				t.Fatalf("AddReply: %v", err)
			}
		}

		replies, err := s.Replies("general", "p1")
		if err != nil || len(replies) != 6 || replies[0].ID != "r0" || replies[5].ID != "r5" {
			t.Fatalf("replies in order: %v %v", ids(replies), err)
		}
		if replies[1].ThreadID != "p1" || replies[1].User != "bob" || replies[1].Channel != "general" {
			t.Errorf("reply fields: %+v", replies[1])
		}
		if n, _ := s.ReplyCount("general", "p1"); n != 6 {
			t.Errorf("ReplyCount = %d", n)
		}
		if n, _ := s.ReplyCount("general", "p2"); n != 0 {
			t.Errorf("a thread without replies: %d", n)
		}

		// replies never appear in the channel
		if all, _ := s.RecentMessages("general", 50); !reflect.DeepEqual(ids(all), []string{"p1", "p2"}) {
			t.Fatalf("channel must only hold parents: %v", ids(all))
		}
		if _, err := s.FindMessage("general", "r0"); err == nil {
			t.Error("a reply is not a channel message")
		}

		// the parent's summary
		parent, err := s.RefreshThreadMeta("general", "p1")
		if err != nil {
			t.Fatal(err)
		}
		if parent.ReplyCount != 6 || !reflect.DeepEqual(parent.ReplyUsers, []string{"alice", "bob", "carol", "dave"}) {
			t.Errorf("summary: count=%d users=%v", parent.ReplyCount, parent.ReplyUsers)
		}
		last, err := time.Parse(time.RFC3339Nano, parent.LastReplyAt)
		if err != nil {
			t.Fatalf("LastReplyAt %q: %v", parent.LastReplyAt, err)
		}
		sameInstant(t, "LastReplyAt", last, base.Add(6*time.Minute), time.Millisecond)
		if got := mustFind(t, s, "general", "p1"); got.ReplyCount != 6 {
			t.Errorf("the channel copy of the parent must carry the summary, got %d", got.ReplyCount)
		}
		if p2 := mustFind(t, s, "general", "p2"); p2.ReplyCount != 0 || p2.LastReplyAt != "" || len(p2.ReplyUsers) != 0 {
			t.Errorf("p2 has no thread: %+v", p2)
		}

		// editing a reply and reacting to it
		r := replies[1]
		r.Text, r.IsEdited = "bob edited", true
		r.Reactions = map[string][]string{"👍": {"alice", "carol"}}
		if err := s.UpdateReply("general", "p1", r); err != nil {
			t.Fatal(err)
		}
		replies, _ = s.Replies("general", "p1")
		if replies[1].Text != "bob edited" || !replies[1].IsEdited || !reflect.DeepEqual(replies[1].Reactions["👍"], []string{"alice", "carol"}) {
			t.Fatalf("reply not updated: %+v", replies[1])
		}
		if len(replies) != 6 || replies[1].ID != "r1" {
			t.Fatalf("an update must not move or duplicate the reply: %v", ids(replies))
		}
		if err := s.UpdateReply("general", "p1", Message{ID: "nope"}); err == nil {
			t.Error("updating a missing reply must fail")
		}

		// deleting a reply
		if err := s.DeleteReply("general", "p1", "r5"); err != nil {
			t.Fatal(err)
		}
		if err := s.DeleteReply("general", "p1", "r5"); err == nil {
			t.Error("deleting twice must fail")
		}
		parent, _ = s.RefreshThreadMeta("general", "p1")
		if parent.ReplyCount != 5 {
			t.Errorf("after delete: %d", parent.ReplyCount)
		}

		// editing the parent keeps its summary
		parent.Text, parent.IsEdited = "parent edited", true
		if err := s.UpdateMessage("general", *parent); err != nil {
			t.Fatal(err)
		}
		if got := mustFind(t, s, "general", "p1"); got.Text != "parent edited" || got.ReplyCount != 5 {
			t.Errorf("parent after edit: %q count=%d", got.Text, got.ReplyCount)
		}

		// removing just the replies leaves the parent
		if err := s.DeleteThread("general", "p1"); err != nil {
			t.Fatal(err)
		}
		if n, _ := s.ReplyCount("general", "p1"); n != 0 {
			t.Errorf("thread should be empty, %d left", n)
		}
		mustFind(t, s, "general", "p1")
	})
}

func TestStoreDeletingAParentRemovesItsThread(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		mustChannel(t, s, "general")
		mustAppend(t, s, Message{ID: "p1", User: "alice", Text: "parent", Channel: "general", Timestamp: time.Now()})
		mustAppend(t, s, Message{ID: "p2", User: "alice", Text: "other", Channel: "general", Timestamp: time.Now()})
		s.AddReply(Message{ID: "r1", User: "bob", Text: "x", Channel: "general", ThreadID: "p1", Timestamp: time.Now()})
		s.AddReply(Message{ID: "r2", User: "bob", Text: "y", Channel: "general", ThreadID: "p2", Timestamp: time.Now()})

		if err := s.DeleteMessage("general", "p1"); err != nil {
			t.Fatal(err)
		}
		// (the app also calls DeleteThread; both stores must cope with either order)
		s.DeleteThread("general", "p1")
		if r, _ := s.Replies("general", "p1"); len(r) != 0 {
			t.Errorf("replies of a deleted parent survive: %v", ids(r))
		}
		if r, _ := s.Replies("general", "p2"); len(r) != 1 {
			t.Errorf("another thread must be untouched: %v", ids(r))
		}
	})
}

func TestStoreThreadUnread(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		mustChannel(t, s, "general")
		mustChannel(t, s, "random")
		mustSaveUser(t, s, User{ID: "u1", Username: "alice", Email: "alice@x.com", Status: "online"})
		mustAppend(t, s, Message{ID: "p1", User: "alice", Text: "parent", Channel: "general", Timestamp: time.Now()})
		mustAppend(t, s, Message{ID: "q1", User: "alice", Text: "parent", Channel: "random", Timestamp: time.Now()})
		reply := func(id, user, channel, parent string) {
			t.Helper()
			time.Sleep(5 * time.Millisecond)
			if err := s.AddReply(Message{ID: id, User: user, Text: id, Channel: channel, ThreadID: parent, Timestamp: time.Now()}); err != nil {
				t.Fatal(err)
			}
		}

		// The first look sets the starting point: older threads count as seen.
		s.AddReply(Message{ID: "old", User: "bob", Text: "old", Channel: "general", ThreadID: "p1", Timestamp: time.Now().Add(-time.Minute)})
		if got, err := s.ThreadUnread("alice", "general"); err != nil || len(got) != 0 {
			t.Fatalf("first look: %v %v", got, err)
		}

		reply("r1", "bob", "general", "p1")
		reply("r2", "alice", "general", "p1") // her own reply never counts
		reply("r3", "carol", "general", "p1")
		reply("x1", "bob", "random", "q1") // another channel
		got, _ := s.ThreadUnread("alice", "general")
		if !reflect.DeepEqual(got, map[string]int{"p1": 2}) {
			t.Fatalf("two unread: %v", got)
		}
		if other, _ := s.ThreadUnread("alice", "random"); !reflect.DeepEqual(other, map[string]int{"q1": 1}) {
			t.Fatalf("other channel: %v", other)
		}

		time.Sleep(5 * time.Millisecond)
		if err := s.MarkThreadRead("alice", "general", "p1"); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.ThreadUnread("alice", "general"); len(got) != 0 {
			t.Fatalf("after opening: %v", got)
		}
		reply("r4", "bob", "general", "p1")
		if got, _ := s.ThreadUnread("alice", "general"); !reflect.DeepEqual(got, map[string]int{"p1": 1}) {
			t.Fatalf("a new reply after opening: %v", got)
		}
		// bob has never been looked at, so his starting point is "now": nothing unread
		if got, _ := s.ThreadUnread("bob", "general"); len(got) != 0 {
			t.Fatalf("bob: %v", got)
		}
	})
}

// ───────────────────────── pins ─────────────────────────

func TestStorePins(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		mustChannel(t, s, "general")
		now := time.Now()
		for _, id := range []string{"a", "b", "c"} {
			mustAppend(t, s, Message{ID: id, User: "alice", Text: "msg " + id, Channel: "general", Timestamp: now})
		}
		if entries, err := s.PinEntries("general"); err != nil || len(entries) != 0 {
			t.Fatalf("no pins yet: %v %v", entries, err)
		}

		pinned := now.Add(time.Minute)
		for i, id := range []string{"a", "b", "c"} {
			if err := s.AddPin("general", pinEntry{MessageID: id, PinnedBy: []string{"alice", "bob", "carol"}[i], PinnedAt: pinned.Add(time.Duration(i) * time.Second)}); err != nil {
				t.Fatal(err)
			}
		}
		entries, _ := s.PinEntries("general")
		if len(entries) != 3 || entries[0].MessageID != "c" || entries[2].MessageID != "a" {
			t.Fatalf("newest pin first: %+v", entries)
		}
		if entries[1].PinnedBy != "bob" {
			t.Errorf("pinned by: %+v", entries[1])
		}
		sameInstant(t, "PinnedAt", entries[1].PinnedAt, pinned.Add(time.Second), time.Millisecond)

		// the pinned message shows its edits and reactions
		b := mustFind(t, s, "general", "b")
		b.Text, b.IsEdited = "b edited", true
		b.Reactions = map[string][]string{"👍": {"carol"}}
		s.UpdateMessage("general", *b)
		pins, err := s.ResolvedPins("general")
		if err != nil || len(pins) != 3 {
			t.Fatalf("resolved: %d %v", len(pins), err)
		}
		if pins[0].Message.ID != "c" || pins[1].Message.Text != "b edited" || !reflect.DeepEqual(pins[1].Message.Reactions["👍"], []string{"carol"}) || pins[1].PinnedBy != "bob" {
			t.Errorf("pins: %+v", pins)
		}

		// unpin
		if removed, err := s.RemovePin("general", "b"); err != nil || !removed {
			t.Fatalf("RemovePin: %v %v", removed, err)
		}
		if removed, _ := s.RemovePin("general", "b"); removed {
			t.Error("there was no pin to remove the second time")
		}

		// a pinned message that gets deleted disappears from the list
		if err := s.DeleteMessage("general", "c"); err != nil {
			t.Fatal(err)
		}
		pins, _ = s.ResolvedPins("general")
		if len(pins) != 1 || pins[0].Message.ID != "a" {
			t.Fatalf("after deleting c: %+v", pins)
		}
		if entries, _ := s.PinEntries("general"); len(entries) != 1 {
			t.Errorf("the pin of a deleted message must not linger: %+v", entries)
		}

		if err := s.DeleteChannelPins("general"); err != nil {
			t.Fatal(err)
		}
		if entries, _ := s.PinEntries("general"); len(entries) != 0 {
			t.Errorf("channel pins not cleared: %+v", entries)
		}
	})
}

// ───────────────────────── files ─────────────────────────

func TestStoreFileMeta(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		if _, _, _, err := s.FileMeta("f1"); err == nil {
			t.Fatal("a missing file must be an error")
		}
		if err := s.SaveFileMeta("f1", "/data/uploads/f1.png", "image/png", "photo.png"); err != nil {
			t.Fatal(err)
		}
		path, mime, name, err := s.FileMeta("f1")
		if err != nil || path != "/data/uploads/f1.png" || mime != "image/png" || name != "photo.png" {
			t.Fatalf("got %q %q %q %v", path, mime, name, err)
		}
		s.SaveFileMeta("f1", "/data/uploads/f1b.png", "image/jpeg", "renamed.jpg")
		if path, mime, name, _ := s.FileMeta("f1"); path != "/data/uploads/f1b.png" || mime != "image/jpeg" || name != "renamed.jpg" {
			t.Fatalf("not replaced: %q %q %q", path, mime, name)
		}
		if err := s.DeleteFileMeta("f1"); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := s.FileMeta("f1"); err == nil {
			t.Error("deleted file is still there")
		}
		if err := s.DeleteFileMeta("never"); err != nil {
			t.Errorf("deleting a missing file is not an error: %v", err)
		}
	})
}

// ───────────────────────── deleting a channel ─────────────────────────

func TestStoreDeletingAChannelLeavesNothingBehind(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		mustChannel(t, s, "temp")
		mustAppend(t, s, Message{ID: "p1", User: "alice", Text: "x", Channel: "temp", Timestamp: time.Now()})
		s.AddReply(Message{ID: "r1", User: "bob", Text: "y", Channel: "temp", ThreadID: "p1", Timestamp: time.Now()})
		s.AddPin("temp", pinEntry{MessageID: "p1", PinnedBy: "alice", PinnedAt: time.Now()})
		s.SaveLastRead("alice", "temp")

		if err := s.DeleteChannel("temp"); err != nil {
			t.Fatal(err)
		}
		s.DeleteChannelPins("temp") // the app does this next to DeleteChannel
		mustChannel(t, s, "temp")   // same name again

		if m, _ := s.RecentMessages("temp", 10); len(m) != 0 {
			t.Errorf("messages survived: %v", ids(m))
		}
		if r, _ := s.Replies("temp", "p1"); len(r) != 0 {
			t.Errorf("replies survived: %v", ids(r))
		}
		if e, _ := s.PinEntries("temp"); len(e) != 0 {
			t.Errorf("pins survived: %v", e)
		}
		if _, found, _ := s.LastRead("alice", "temp"); found {
			t.Error("read mark survived")
		}
	})
}
