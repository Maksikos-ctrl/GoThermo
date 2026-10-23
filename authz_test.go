package main

import (
	"testing"
	"time"
)

func TestAuthz(t *testing.T) {
	useMiniRedis(t)
	a := &App{hub: NewHub()}
	go a.hub.Run()
	SaveChannel(Channel{Name: "general", Members: []string{}, CreatedBy: "system", CreatedAt: time.Now()})
	SaveChannel(Channel{Name: "dm_alice_bob", Members: []string{"alice", "bob"}, IsPrivate: true, CreatedBy: "system", CreatedAt: time.Now()})
	ta, _ := CreateSession("alice")
	tb, _ := CreateSession("bob")
	tc, _ := CreateSession("carol")

	if _, err := a.SendMessage("", "alice", "hi", "general"); err == nil {
		t.Fatal("empty token accepted")
	}
	if _, err := a.SendMessage("bogus", "alice", "hi", "general"); err == nil {
		t.Fatal("bogus token accepted")
	}
	if _, err := a.SendMessage(tb, "alice", "hi", "general"); err == nil {
		t.Fatal("impersonation accepted")
	}
	if _, err := a.SendMessage(ta, "alice", "hi", "general"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SendMessage(ta, "alice", "secret", "dm_alice_bob"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SendMessage(tc, "carol", "hack", "dm_alice_bob"); err == nil {
		t.Fatal("non-member posted to DM")
	}
	if _, err := a.GetMessages(tc, "dm_alice_bob"); err == nil {
		t.Fatal("non-member read DM")
	}
	if m, err := a.GetMessages(tb, "dm_alice_bob"); err != nil || len(m) != 1 {
		t.Fatal(err, m)
	}
	if _, err := a.GetMessages("bogus", "general"); err == nil {
		t.Fatal("bogus read")
	}
	if err := a.JoinChannel(tc, "dm_alice_bob", "carol"); err == nil {
		t.Fatal("joined private")
	}
	if err := a.JoinChannel(tc, "general", "carol"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetOrCreateDMChannel(tc, "alice", "bob"); err == nil {
		t.Fatal("dm impersonation")
	}
	if _, err := a.GetOrCreateGroupChannel(tc, []string{"alice", "bob"}, "carol"); err == nil {
		t.Fatal("group without self")
	}
	if _, err := a.GetOrCreateGroupChannel(tc, []string{"alice", "carol"}, "carol"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SearchMessages(tc, "alice", "secret"); err == nil {
		t.Fatal("search impersonation")
	}
	if r, err := a.SearchMessages(ta, "alice", "secret"); err != nil || len(r) != 1 {
		t.Fatal(err, r)
	}
	if r, err := a.SearchMessages(tc, "carol", "secret"); err != nil || len(r) != 0 {
		t.Fatal("carol sees DM", r)
	}
	if err := a.DeleteChannel(tc, "general", "alice"); err == nil {
		t.Fatal("delete impersonation")
	}
	if c, err := a.GetUnreadCounts(tb, "bob"); err != nil || c == nil {
		t.Fatal(err)
	}
	if _, err := a.GetUnreadCounts(tb, "alice"); err == nil {
		t.Fatal("unread impersonation")
	}
	if _, err := a.GetChannels(""); err == nil {
		t.Fatal("channels no token")
	}
	if _, err := a.GetLinkPreview("", "https://example.com"); err == nil {
		t.Fatal("preview no token")
	}
	if _, err := a.ExportChannel(tc, "dm_alice_bob", "carol", "txt"); err == nil {
		t.Fatal("export non-member")
	}
	if _, err := a.GetFileData("", "x"); err == nil {
		t.Fatal("file no token")
	}
}

func TestUsersAuthz(t *testing.T) {
	mr := useMiniRedis(t)
	a := &App{}
	ta := mkUser(t, mr, "alice")
	mkUser(t, mr, "bob")
	if _, err := a.GetUsers(""); err == nil {
		t.Fatal("GetUsers no token")
	}
	if us, err := a.GetUsers(ta); err != nil || len(us) < 2 {
		t.Fatal(err, us)
	}
	if a.UpdateUserStatus(ta, "bob", "away") {
		t.Fatal("changed someone else")
	}
	if a.UpdateUserStatus("", "alice", "away") {
		t.Fatal("no token")
	}
	if !a.UpdateUserStatus(ta, "alice", "away") {
		t.Fatal("own status failed")
	}
	if a.UpdateUserStatus(ta, "alice", "bogus") {
		t.Fatal("bad status")
	}
	if u, _ := userManager.GetUser("alice"); u.Status != "away" {
		t.Fatal(u.Status)
	}
}
