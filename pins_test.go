package main

import (
	"strings"
	"testing"
	"time"
)

func sendMsg(t *testing.T, e calEnv, channel, user, text string) string {
	t.Helper()
	id, err := e.a.SendMessage(e.tk[user], user, text, channel)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestPinAndList(t *testing.T) {
	e := setupCalendar(t)
	m1 := sendMsg(t, e, "general", "alice", "first")
	m2 := sendMsg(t, e, "general", "bob", "second")
	sendMsg(t, e, "general", "alice", "not pinned")

	if err := e.a.PinMessage(e.tk["alice"], "general", m1, "alice"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := e.a.PinMessage(e.tk["carol"], "general", m2, "carol"); err != nil {
		t.Fatal(err)
	}

	pins, err := e.a.GetPinnedMessages(e.tk["bob"], "general")
	if err != nil {
		t.Fatal(err)
	}
	if len(pins) != 2 {
		t.Fatalf("want 2 pins, got %d", len(pins))
	}
	if pins[0].Message.ID != m2 || pins[1].Message.ID != m1 {
		t.Errorf("newest pin must come first: %s, %s", pins[0].Message.Text, pins[1].Message.Text)
	}
	if pins[0].PinnedBy != "carol" || pins[1].PinnedBy != "alice" {
		t.Errorf("wrong pinnedBy: %s, %s", pins[0].PinnedBy, pins[1].PinnedBy)
	}
	if pins[0].PinnedAt.IsZero() {
		t.Error("pinnedAt missing")
	}
	if pins[0].Message.Text != "second" {
		t.Errorf("message text missing: %+v", pins[0].Message)
	}
}

func TestPinningTwiceChangesNothing(t *testing.T) {
	e := setupCalendar(t)
	m := sendMsg(t, e, "general", "alice", "x")
	bob := listen(e, "bob")
	e.a.PinMessage(e.tk["alice"], "general", m, "alice")
	drain(bob)
	if err := e.a.PinMessage(e.tk["bob"], "general", m, "bob"); err != nil {
		t.Fatal(err)
	}
	pins, _ := e.a.GetPinnedMessages(e.tk["alice"], "general")
	if len(pins) != 1 || pins[0].PinnedBy != "alice" {
		t.Fatalf("second pin changed the first: %+v", pins)
	}
	if len(drain(bob)) != 0 {
		t.Error("a no-op pin was announced")
	}
}

func TestUnpin(t *testing.T) {
	e := setupCalendar(t)
	m := sendMsg(t, e, "general", "alice", "x")
	e.a.PinMessage(e.tk["alice"], "general", m, "alice")
	if err := e.a.UnpinMessage(e.tk["bob"], "general", m, "bob"); err != nil {
		t.Fatal(err)
	}
	pins, _ := e.a.GetPinnedMessages(e.tk["alice"], "general")
	if len(pins) != 0 {
		t.Fatalf("still pinned: %+v", pins)
	}
	// unpinning something not pinned is harmless
	if err := e.a.UnpinMessage(e.tk["bob"], "general", m, "bob"); err != nil {
		t.Fatal(err)
	}
}

func TestPinValidation(t *testing.T) {
	e := setupCalendar(t)
	m := sendMsg(t, e, "general", "alice", "x")
	call, _ := e.a.LogCall(e.tk["alice"], "general", "alice", "ended", 5)

	bad := []struct {
		name string
		err  error
	}{
		{"no token", e.a.PinMessage("", "general", m, "alice")},
		{"bad token", e.a.PinMessage("nope", "general", m, "alice")},
		{"someone else's name", e.a.PinMessage(e.tk["bob"], "general", m, "alice")},
		{"no channel", e.a.PinMessage(e.tk["alice"], "nowhere", m, "alice")},
		{"no message", e.a.PinMessage(e.tk["alice"], "general", "missing", "alice")},
		{"call log", e.a.PinMessage(e.tk["alice"], "general", call.ID, "alice")},
		{"unpin bad token", e.a.UnpinMessage("nope", "general", m, "alice")},
		{"unpin someone else's name", e.a.UnpinMessage(e.tk["bob"], "general", m, "alice")},
	}
	for _, c := range bad {
		if c.err == nil {
			t.Errorf("%s: accepted", c.name)
		}
	}
	if _, err := e.a.GetPinnedMessages("nope", "general"); err == nil {
		t.Error("list accepted a bad token")
	}
}

func TestPinsRespectPrivateChannels(t *testing.T) {
	e := setupCalendar(t) // secret = alice + bob
	m := sendMsg(t, e, "secret", "alice", "private")

	if e.a.PinMessage(e.tk["carol"], "secret", m, "carol") == nil {
		t.Error("an outsider pinned in a private channel")
	}
	e.a.PinMessage(e.tk["alice"], "secret", m, "alice")
	if _, err := e.a.GetPinnedMessages(e.tk["carol"], "secret"); err == nil {
		t.Error("an outsider read private pins")
	}
	if e.a.UnpinMessage(e.tk["carol"], "secret", m, "carol") == nil {
		t.Error("an outsider unpinned in a private channel")
	}
	if pins, _ := e.a.GetPinnedMessages(e.tk["bob"], "secret"); len(pins) != 1 {
		t.Error("the pin was lost")
	}
}

func TestPinLimit(t *testing.T) {
	e := setupCalendar(t)
	var last string
	for i := 0; i < maxPinsPerChannel; i++ {
		last = sendMsg(t, e, "general", "alice", "m")
		if err := e.a.PinMessage(e.tk["alice"], "general", last, "alice"); err != nil {
			t.Fatalf("pin %d: %v", i, err)
		}
	}
	extra := sendMsg(t, e, "general", "alice", "one too many")
	err := e.a.PinMessage(e.tk["alice"], "general", extra, "alice")
	if err == nil || !strings.Contains(err.Error(), "unpin one first") {
		t.Fatalf("limit not enforced: %v", err)
	}
	// re-pinning an already pinned message at the limit is still fine
	if err := e.a.PinMessage(e.tk["alice"], "general", last, "alice"); err != nil {
		t.Fatalf("re-pin at the limit failed: %v", err)
	}
	e.a.UnpinMessage(e.tk["alice"], "general", last, "alice")
	if err := e.a.PinMessage(e.tk["alice"], "general", extra, "alice"); err != nil {
		t.Fatalf("no room after unpinning: %v", err)
	}
}

func TestDeletingAMessageRemovesItsPin(t *testing.T) {
	e := setupCalendar(t)
	m := sendMsg(t, e, "general", "alice", "gone soon")
	keep := sendMsg(t, e, "general", "alice", "stays")
	e.a.PinMessage(e.tk["alice"], "general", m, "alice")
	e.a.PinMessage(e.tk["alice"], "general", keep, "alice")
	carol := listen(e, "carol")
	drain(carol)

	if err := e.a.DeleteMessage(e.tk["alice"], m, "general", "alice"); err != nil {
		t.Fatal(err)
	}
	pins, _ := e.a.GetPinnedMessages(e.tk["carol"], "general")
	if len(pins) != 1 || pins[0].Message.ID != keep {
		t.Fatalf("deleted message still pinned: %+v", pins)
	}
	entries, _, _ := loadPinEntries("general")
	if len(entries) != 1 {
		t.Errorf("pin entry not removed from Redis: %d left", len(entries))
	}
	if countType(drain(carol), "pins_changed") != 1 {
		t.Error("clients were not told the pin list changed")
	}
}

func TestStalePinsAreCleanedUp(t *testing.T) {
	e := setupCalendar(t)
	m := sendMsg(t, e, "general", "alice", "x")
	e.a.PinMessage(e.tk["alice"], "general", m, "alice")
	// the message disappears behind the pin's back
	DeleteMessageInRedis("general", m)
	pins, err := e.a.GetPinnedMessages(e.tk["alice"], "general")
	if err != nil || len(pins) != 0 {
		t.Fatalf("stale pin shown: %+v %v", pins, err)
	}
	if entries, _, _ := loadPinEntries("general"); len(entries) != 0 {
		t.Error("stale pin entry not removed")
	}
}

func TestPinnedMessageShowsEditsAndReactions(t *testing.T) {
	e := setupCalendar(t)
	m := sendMsg(t, e, "general", "alice", "old text")
	e.a.PinMessage(e.tk["alice"], "general", m, "alice")
	e.a.EditMessage(e.tk["alice"], m, "general", "alice", "new text")
	e.a.AddReaction(e.tk["bob"], m, "👍", "bob", "general")
	pins, _ := e.a.GetPinnedMessages(e.tk["carol"], "general")
	if pins[0].Message.Text != "new text" || len(pins[0].Message.Reactions["👍"]) != 1 {
		t.Fatalf("pinned copy is stale: %+v", pins[0].Message)
	}
}

func TestDeletingAChannelRemovesPins(t *testing.T) {
	e := setupCalendar(t)
	e.a.CreateChannel(e.tk["alice"], "temp", "", "alice", false, nil)
	m := sendMsg(t, e, "temp", "alice", "x")
	e.a.PinMessage(e.tk["alice"], "temp", m, "alice")
	if !miniHas(t, pinsKey("temp")) {
		t.Fatal("pin was not stored")
	}
	if err := e.a.DeleteChannel(e.tk["alice"], "temp", "alice"); err != nil {
		t.Fatal(err)
	}
	if miniHas(t, pinsKey("temp")) {
		t.Error("pins of a deleted channel were left behind")
	}
}

func TestPinEventsAndPrivacy(t *testing.T) {
	e := setupCalendar(t) // secret = alice + bob
	alice := listen(e, "alice")
	carol := listen(e, "carol")
	m := sendMsg(t, e, "secret", "alice", "private")
	g := sendMsg(t, e, "general", "alice", "public")
	drain(alice)
	drain(carol)

	e.a.PinMessage(e.tk["alice"], "secret", m, "alice")
	if len(drain(carol)) != 0 {
		t.Error("a private pin event leaked to a non-member")
	}
	if countType(drain(alice), "pins_changed") != 1 {
		t.Error("the member did not get the event")
	}

	e.a.PinMessage(e.tk["alice"], "general", g, "alice")
	got := drain(carol)
	if countType(got, "pins_changed") != 1 {
		t.Fatalf("carol should be told about the public pin: %v", got)
	}
	e.a.UnpinMessage(e.tk["alice"], "general", g, "alice")
	if countType(drain(carol), "pins_changed") != 1 {
		t.Error("unpin must be announced")
	}
}
