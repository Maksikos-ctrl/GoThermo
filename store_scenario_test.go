package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// One end-to-end walk through the app (send, react, edit, reply, pin, page,
// delete) on every store. It calls the same App methods the frontend calls, so
// it checks the whole chain, not just the storage layer.

type scenarioEnv struct {
	a  *App
	tk map[string]string
}

func setupScenario(t *testing.T, s Store) scenarioEnv {
	t.Helper()
	useMiniRedis(t) // sessions and login limits always live in Redis
	prev := store
	store = s
	t.Cleanup(func() { store = prev })

	hub := NewHub()
	go hub.Run()
	env := scenarioEnv{a: &App{hub: hub}, tk: map[string]string{}}
	for _, name := range []string{"alice", "bob", "carol"} {
		email := name + "@x.com"
		userManager.RegisterUser(name, email)
		deadline := time.Now().Add(2 * time.Second)
		for {
			if _, err := s.GetUserByEmail(email); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("user %s was not saved", name)
			}
			time.Sleep(5 * time.Millisecond)
		}
		token, err := CreateSession(name)
		if err != nil {
			t.Fatal(err)
		}
		env.tk[name] = token
	}
	if err := s.SaveChannel(Channel{ID: "c-general", Name: "general", Members: []string{}, CreatedBy: "system", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveChannel(Channel{ID: "c-secret", Name: "secret", IsPrivate: true, Members: []string{"alice", "bob"}, CreatedBy: "alice", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	return env
}

func mustSend(t *testing.T, e scenarioEnv, channel, user, text string) string {
	t.Helper()
	id, err := e.a.SendMessage(e.tk[user], user, text, channel)
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	return id
}

func TestScenarioMessagesReactionsEditsAndSearch(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		e := setupScenario(t, s)
		m1 := mustSend(t, e, "general", "alice", "hello world")
		m2 := mustSend(t, e, "general", "bob", "second one")

		got, err := e.a.GetMessages(e.tk["bob"], "general")
		if err != nil || !reflect.DeepEqual(ids(got), []string{m1, m2}) {
			t.Fatalf("GetMessages: %v %v", ids(got), err)
		}

		// reactions toggle, and keep the order people reacted in
		a := e.a
		must := func(err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		}
		must(a.AddReaction(e.tk["bob"], m1, "👍", "bob", "general"))
		must(a.AddReaction(e.tk["bob"], m1, "👍", "bob", "general")) // second click removes it
		must(a.AddReaction(e.tk["carol"], m1, "👍", "carol", "general"))
		must(a.AddReaction(e.tk["bob"], m1, "👍", "bob", "general"))
		got, _ = a.GetMessages(e.tk["alice"], "general")
		if !reflect.DeepEqual(got[0].Reactions["👍"], []string{"carol", "bob"}) {
			t.Fatalf("reactions: %v", got[0].Reactions)
		}

		// editing: only the author, and it is marked as edited
		if _, err := a.EditMessage(e.tk["bob"], m1, "general", "bob", "hacked"); err == nil {
			t.Fatal("bob must not edit alice's message")
		}
		edited, err := a.EditMessage(e.tk["alice"], m1, "general", "alice", "hello brave world")
		if err != nil || !edited.IsEdited || edited.Text != "hello brave world" {
			t.Fatalf("EditMessage: %+v %v", edited, err)
		}

		// search finds it, whatever the case
		found, err := a.SearchMessages(e.tk["bob"], "bob", "BRAVE")
		if err != nil || len(found) != 1 || found[0].ID != m1 {
			t.Fatalf("SearchMessages: %v %v", ids(found), err)
		}

		// deleting: only the author
		if err := a.DeleteMessage(e.tk["alice"], m2, "general", "alice"); err == nil {
			t.Fatal("alice must not delete bob's message")
		}
		must(a.DeleteMessage(e.tk["bob"], m2, "general", "bob"))
		got, _ = a.GetMessages(e.tk["alice"], "general")
		if !reflect.DeepEqual(ids(got), []string{m1}) {
			t.Fatalf("after delete: %v", ids(got))
		}
	})
}

func TestScenarioThreads(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		e := setupScenario(t, s)
		a := e.a
		parent := mustSend(t, e, "general", "alice", "let's discuss")
		other := mustSend(t, e, "general", "alice", "unrelated")

		// alice looks at her threads once, so everything before counts as seen
		if u, _ := a.GetThreadUnread(e.tk["alice"], "general"); len(u) != 0 {
			t.Fatalf("nothing unread yet: %v", u)
		}
		time.Sleep(5 * time.Millisecond)

		r1, err := a.SendReply(e.tk["bob"], "bob", "general", parent, "first reply")
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
		r2, err := a.SendReply(e.tk["carol"], "carol", "general", parent, "second reply")
		if err != nil {
			t.Fatal(err)
		}

		th, err := a.GetThread(e.tk["alice"], "general", parent)
		if err != nil || len(th.Replies) != 2 || th.Replies[0].ID != r1.ID || th.Replies[1].ID != r2.ID {
			t.Fatalf("GetThread: %+v %v", th, err)
		}
		if th.Parent.ReplyCount != 2 || !reflect.DeepEqual(th.Parent.ReplyUsers, []string{"bob", "carol"}) {
			t.Errorf("parent summary: %+v", th.Parent)
		}

		// the channel shows the summary and none of the replies
		list, _ := a.GetMessages(e.tk["alice"], "general")
		if !reflect.DeepEqual(ids(list), []string{parent, other}) || list[0].ReplyCount != 2 {
			t.Fatalf("channel view: %v count=%d", ids(list), list[0].ReplyCount)
		}

		if u, _ := a.GetThreadUnread(e.tk["alice"], "general"); !reflect.DeepEqual(u, map[string]int{parent: 2}) {
			t.Fatalf("unread for alice: %v", u)
		}
		if err := a.MarkThreadRead(e.tk["alice"], "general", parent); err != nil {
			t.Fatal(err)
		}
		if u, _ := a.GetThreadUnread(e.tk["alice"], "general"); len(u) != 0 {
			t.Fatalf("after opening: %v", u)
		}

		// edit, react, delete a reply
		if _, err := a.EditReply(e.tk["carol"], "general", parent, r1.ID, "carol", "x"); err == nil {
			t.Fatal("carol must not edit bob's reply")
		}
		er, err := a.EditReply(e.tk["bob"], "general", parent, r1.ID, "bob", "first reply, fixed")
		if err != nil || !er.IsEdited {
			t.Fatalf("EditReply: %+v %v", er, err)
		}
		if err := a.AddReplyReaction(e.tk["alice"], "general", parent, r1.ID, "🎉", "alice"); err != nil {
			t.Fatal(err)
		}
		th, _ = a.GetThread(e.tk["alice"], "general", parent)
		if th.Replies[0].Text != "first reply, fixed" || !reflect.DeepEqual(th.Replies[0].Reactions["🎉"], []string{"alice"}) {
			t.Fatalf("reply after edit and reaction: %+v", th.Replies[0])
		}
		if err := a.DeleteReply(e.tk["bob"], "general", parent, r1.ID, "bob"); err != nil {
			t.Fatal(err)
		}
		th, _ = a.GetThread(e.tk["alice"], "general", parent)
		if len(th.Replies) != 1 || th.Parent.ReplyCount != 1 {
			t.Fatalf("after delete: %d replies, count %d", len(th.Replies), th.Parent.ReplyCount)
		}

		// export of the thread
		if exp, err := a.BuildThreadExport(e.tk["alice"], "general", parent, "txt"); err != nil || exp.Content == "" {
			t.Fatalf("BuildThreadExport: %v", err)
		}

		// deleting the parent takes the thread with it
		if err := a.DeleteMessage(e.tk["alice"], parent, "general", "alice"); err != nil {
			t.Fatal(err)
		}
		if r, _ := s.Replies("general", parent); len(r) != 0 {
			t.Errorf("thread survived its parent: %v", ids(r))
		}
	})
}

func TestScenarioPrivateChannelsStayPrivate(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		e := setupScenario(t, s)
		a := e.a
		m := mustSend(t, e, "secret", "alice", "for bob only")
		reply, err := a.SendReply(e.tk["bob"], "bob", "secret", m, "ok")
		if err != nil {
			t.Fatal(err)
		}

		if got, err := a.GetMessages(e.tk["bob"], "secret"); err != nil || len(got) != 1 {
			t.Fatalf("bob: %v %v", got, err)
		}
		if _, err := a.GetMessages(e.tk["carol"], "secret"); err == nil {
			t.Error("carol read a private channel")
		}
		if _, err := a.GetThread(e.tk["carol"], "secret", m); err == nil {
			t.Error("carol read a private thread")
		}
		if _, err := a.SendReply(e.tk["carol"], "carol", "secret", m, "hi"); err == nil {
			t.Error("carol replied in a private thread")
		}
		if err := a.PinMessage(e.tk["carol"], "secret", m, "carol"); err == nil {
			t.Error("carol pinned in a private channel")
		}
		_ = reply
	})
}

func TestScenarioPins(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		e := setupScenario(t, s)
		a := e.a
		m1 := mustSend(t, e, "general", "alice", "important")
		m2 := mustSend(t, e, "general", "bob", "also important")
		m3 := mustSend(t, e, "general", "carol", "temporary")

		for i, id := range []string{m1, m2, m3} {
			who := []string{"alice", "bob", "carol"}[i]
			if err := a.PinMessage(e.tk[who], "general", id, who); err != nil {
				t.Fatal(err)
			}
			time.Sleep(2 * time.Millisecond)
		}
		// pinning twice changes nothing
		if err := a.PinMessage(e.tk["bob"], "general", m1, "bob"); err != nil {
			t.Fatal(err)
		}
		pins, err := a.GetPinnedMessages(e.tk["carol"], "general")
		if err != nil || len(pins) != 3 || pins[0].Message.ID != m3 || pins[2].Message.ID != m1 {
			t.Fatalf("pins newest first: %v %v", pinIDs(pins), err)
		}
		if pins[2].PinnedBy != "alice" {
			t.Errorf("pinned by: %+v", pins[2])
		}

		if err := a.UnpinMessage(e.tk["bob"], "general", m2, "bob"); err != nil {
			t.Fatal(err)
		}
		// deleting a pinned message unpins it
		if err := a.DeleteMessage(e.tk["carol"], m3, "general", "carol"); err != nil {
			t.Fatal(err)
		}
		pins, _ = a.GetPinnedMessages(e.tk["alice"], "general")
		if len(pins) != 1 || pins[0].Message.ID != m1 {
			t.Fatalf("after unpin and delete: %v", pinIDs(pins))
		}

		// a call log cannot be pinned
		if err := s.AppendMessage(Message{ID: "call1", User: "alice", Text: "call", Channel: "general", Timestamp: time.Now(), IsCall: true, CallStatus: "ended", Reactions: map[string][]string{}}); err != nil {
			t.Fatal(err)
		}
		if err := a.PinMessage(e.tk["alice"], "general", "call1", "alice"); err == nil {
			t.Error("a call log was pinned")
		}
	})
}

func pinIDs(p []PinnedMessage) []string {
	out := make([]string, len(p))
	for i, x := range p {
		out[i] = x.Message.ID
	}
	return out
}

func TestScenarioHistoryPaging(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		e := setupScenario(t, s)
		base := time.Now().Add(-time.Hour)
		want := make([]string, 130)
		for i := range want {
			want[i] = fmt.Sprintf("h%03d", i)
			if err := s.AppendMessage(Message{ID: want[i], User: "alice", Text: want[i], Channel: "general",
				Timestamp: base.Add(time.Duration(i) * time.Second), Reactions: map[string][]string{}}); err != nil {
				t.Fatal(err)
			}
		}
		tk := e.tk["bob"]
		p, err := e.a.GetMessagesPage(tk, "general", "", 50)
		if err != nil || len(p.Messages) != 50 || !p.HasMore || p.Messages[49].ID != want[129] {
			t.Fatalf("newest page: %d %v %v", len(p.Messages), p.HasMore, err)
		}
		all := ids(p.Messages)
		for p.HasMore {
			p, err = e.a.GetMessagesPage(tk, "general", p.Messages[0].ID, 50)
			if err != nil {
				t.Fatal(err)
			}
			all = append(ids(p.Messages), all...)
		}
		if !reflect.DeepEqual(all, want) {
			t.Fatalf("paging must deliver everything once, in order: got %d", len(all))
		}
		// a huge page size is clamped (here there are fewer messages than the limit)
		if p, _ := e.a.GetMessagesPage(tk, "general", "", 100000); len(p.Messages) != len(want) || p.HasMore {
			t.Errorf("big page: %d hasMore=%v", len(p.Messages), p.HasMore)
		}
	})
}

func TestScenarioDeletingAChannelWithAttachment(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		e := setupScenario(t, s)
		if err := s.SaveChannel(Channel{ID: "c-temp", Name: "temp", Members: []string{"alice"}, CreatedBy: "alice", CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
		wd, _ := os.Getwd()
		if err := os.Chdir(t.TempDir()); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chdir(wd) })
		os.MkdirAll(uploadsDir, 0755)
		path := filepath.Join(uploadsDir, "file1_f.txt")
		if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}

		s.SaveFileMeta("file1", path, "text/plain", "f.txt")
		if err := s.AppendMessage(Message{ID: "fm", User: "alice", Text: "f.txt", Channel: "temp", Timestamp: time.Now(),
			IsFile: true, FileID: "file1", FileName: "f.txt", FileSize: 4, MimeType: "text/plain", Reactions: map[string][]string{}}); err != nil {
			t.Fatal(err)
		}
		if err := e.a.DeleteChannel(e.tk["alice"], "temp", "alice"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.GetChannel("temp"); err == nil {
			t.Error("channel still exists")
		}
		if _, _, _, err := s.FileMeta("file1"); err == nil {
			t.Error("file metadata survived the channel")
		}
		if _, err := os.Stat(path); err == nil {
			t.Error("the uploaded file was not removed from disk")
		}
	})
}
