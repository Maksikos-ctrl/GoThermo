package main

import (
	"strings"
	"testing"
	"time"
)

func setupTeam(t *testing.T) calEnv {
	e := setupCalendar(t)
	if _, err := e.a.CreateChannel(e.tk["alice"], "team", "", "alice", true, []string{"bob", "carol"}); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestLeaveChannel(t *testing.T) {
	e := setupTeam(t)
	SaveMessage(Message{ID: "old", User: "alice", Text: "hello", Channel: "team", Timestamp: time.Now()})

	if err := e.a.LeaveChannel(e.tk["bob"], "team", "bob"); err != nil {
		t.Fatal(err)
	}
	ch, _ := GetChannel("team")
	if channelHasMember(ch.Members, "bob") || len(ch.Members) != 2 {
		t.Errorf("bob still in: %v", ch.Members)
	}
	// the channel disappears from bob's list, stays for the others
	if list, _ := e.a.GetChannels(e.tk["bob"]); hasChannel(list, "team") {
		t.Error("bob still sees the channel")
	}
	if list, _ := e.a.GetChannels(e.tk["alice"]); !hasChannel(list, "team") {
		t.Error("alice lost the channel")
	}
	// and bob can no longer read it
	if _, err := e.a.GetMessages(e.tk["bob"], "team"); err == nil {
		t.Error("bob still reads the channel")
	}
	if _, err := e.a.GetMessagesPage(e.tk["bob"], "team", "", 10); err == nil {
		t.Error("bob still pages the channel")
	}
	if _, err := e.a.SendMessage(e.tk["bob"], "bob", "still here?", "team"); err == nil {
		t.Error("bob still posts to the channel")
	}
	// a note was left in the chat
	msgs, _ := GetMessages("team", 10)
	last := msgs[len(msgs)-1]
	if !strings.Contains(last.Text, "bob left") {
		t.Errorf("no leave note: %+v", last)
	}
	// a member can invite him back
	if _, err := e.a.AddChannelMember(e.tk["alice"], "team", "alice", "bob"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.GetMessages(e.tk["bob"], "team"); err != nil {
		t.Errorf("re-invited bob cannot read: %v", err)
	}
}

func hasChannel(list []Channel, name string) bool {
	for _, c := range list {
		if c.Name == name {
			return true
		}
	}
	return false
}

func TestLeaveChannelCreatorHandsOver(t *testing.T) {
	e := setupTeam(t)
	if err := e.a.LeaveChannel(e.tk["alice"], "team", "alice"); err != nil {
		t.Fatal(err)
	}
	ch, _ := GetChannel("team")
	if ch.CreatedBy != "bob" {
		t.Errorf("owner should pass to the oldest member, got %q", ch.CreatedBy)
	}
	// the new owner can delete it, the old one cannot
	if err := e.a.DeleteChannel(e.tk["alice"], "team", "alice"); err == nil {
		t.Error("former creator deleted the channel")
	}
	if err := e.a.DeleteChannel(e.tk["bob"], "team", "bob"); err != nil {
		t.Errorf("new owner could not delete: %v", err)
	}
}

func TestLeaveChannelRules(t *testing.T) {
	e := setupTeam(t)

	if err := e.a.LeaveChannel("", "team", "bob"); err == nil {
		t.Error("no token accepted")
	}
	if err := e.a.LeaveChannel(e.tk["alice"], "team", "bob"); err == nil {
		t.Error("left on behalf of somebody else")
	}
	if err := e.a.LeaveChannel(e.tk["bob"], "nowhere", "bob"); err == nil {
		t.Error("left a channel that does not exist")
	}
	if err := e.a.LeaveChannel(e.tk["bob"], "general", "bob"); err == nil {
		t.Error("left a public channel")
	}
	dm, _ := e.a.GetOrCreateDMChannel(e.tk["alice"], "alice", "bob")
	if err := e.a.LeaveChannel(e.tk["bob"], dm.Name, "bob"); err == nil {
		t.Error("left a DM")
	}
	// an outsider
	e.a.CreateChannel(e.tk["alice"], "inner", "", "alice", true, []string{"bob"})
	if err := e.a.LeaveChannel(e.tk["carol"], "inner", "carol"); err == nil {
		t.Error("a non-member left")
	}

	// the last member cannot leave, the channel stays intact
	e.a.LeaveChannel(e.tk["bob"], "inner", "bob")
	if err := e.a.LeaveChannel(e.tk["alice"], "inner", "alice"); err == nil {
		t.Error("the last member left")
	}
	ch, _ := GetChannel("inner")
	if ch == nil || len(ch.Members) != 1 || ch.Members[0] != "alice" {
		t.Errorf("channel damaged: %+v", ch)
	}
	// leaving twice is an error, not a silent success
	if err := e.a.LeaveChannel(e.tk["bob"], "inner", "bob"); err == nil {
		t.Error("left twice")
	}
}
