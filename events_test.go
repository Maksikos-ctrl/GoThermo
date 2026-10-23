package main

import (
	"strings"
	"testing"
	"time"
)

func iso(t time.Time) string { return t.UTC().Format(time.RFC3339) }

type calEnv struct {
	a  *App
	tk map[string]string
}

func setupCalendar(t *testing.T) calEnv {
	mr := useMiniRedis(t)
	hub := NewHub()
	go hub.Run()
	env := calEnv{a: &App{hub: hub}, tk: map[string]string{}}
	for _, u := range []string{"alice", "bob", "carol"} {
		env.tk[u] = mkUser(t, mr, u)
	}
	SaveChannel(Channel{Name: "general", CreatedBy: "system", CreatedAt: time.Now()})
	SaveChannel(Channel{Name: "secret", IsPrivate: true, Members: []string{"alice", "bob"}, CreatedBy: "alice", CreatedAt: time.Now()})
	return env
}

func TestCreateEventValidation(t *testing.T) {
	e := setupCalendar(t)
	start := iso(time.Now().Add(2 * time.Hour))
	bad := []struct {
		name                  string
		token, channel, title string
		start                 string
		mins                  int
	}{
		{"no token", "", "general", "x", start, 60},
		{"bad token", "nope", "general", "x", start, 60},
		{"empty title", e.tk["alice"], "general", "   ", start, 60},
		{"long title", e.tk["alice"], "general", strings.Repeat("a", maxEventTitle+1), start, 60},
		{"bad start", e.tk["alice"], "general", "x", "tomorrow", 60},
		{"too short", e.tk["alice"], "general", "x", start, 1},
		{"too long", e.tk["alice"], "general", "x", start, maxEventMinutes + 1},
		{"no channel", e.tk["alice"], "nowhere", "x", start, 60},
		{"outsider in private", e.tk["carol"], "secret", "x", start, 60},
	}
	for _, c := range bad {
		if _, err := e.a.CreateEvent(c.token, c.channel, c.title, "", c.start, c.mins); err == nil {
			t.Errorf("%s: accepted", c.name)
		}
	}
	if _, err := e.a.CreateEvent(e.tk["alice"], "general", "x", strings.Repeat("d", maxEventDesc+1), start, 60); err == nil {
		t.Error("long description accepted")
	}

	ev, err := e.a.CreateEvent(e.tk["alice"], "general", "  Team sync  ", "agenda", start, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Title != "Team sync" || ev.CreatedBy != "alice" || ev.RSVP["alice"] != rsvpGoing {
		t.Fatalf("%+v", ev)
	}
	if ev.End.Sub(ev.Start) != defaultEventMins*time.Minute {
		t.Errorf("default duration: %v", ev.End.Sub(ev.Start))
	}

	msgs, _ := GetMessages("general", 10)
	if len(msgs) != 1 || !strings.Contains(msgs[0].Text, "Team sync") {
		t.Errorf("no chat announcement: %+v", msgs)
	}
}

func TestGetEventsRangeAndVisibility(t *testing.T) {
	e := setupCalendar(t)
	base := time.Now().Add(48 * time.Hour).Truncate(time.Hour)
	mk := func(tok, ch, title string, off time.Duration) Event {
		ev, err := e.a.CreateEvent(tok, ch, title, "", iso(base.Add(off)), 60)
		if err != nil {
			t.Fatal(err)
		}
		return ev
	}
	pub := mk(e.tk["alice"], "general", "public", 0)
	sec := mk(e.tk["alice"], "secret", "secret", time.Hour)
	later := mk(e.tk["bob"], "general", "later", 24*time.Hour)

	from, to := iso(base.Add(-time.Hour)), iso(base.Add(12*time.Hour))
	titles := func(tok string) string {
		evs, err := e.a.GetEvents(tok, from, to)
		if err != nil {
			t.Fatal(err)
		}
		var n []string
		for _, ev := range evs {
			n = append(n, ev.Title)
		}
		return strings.Join(n, ",")
	}
	if got := titles(e.tk["alice"]); got != "public,secret" {
		t.Errorf("alice (ordered, in range): %q", got)
	}
	if got := titles(e.tk["bob"]); got != "public,secret" {
		t.Errorf("bob: %q", got)
	}
	if got := titles(e.tk["carol"]); got != "public" {
		t.Errorf("carol must not see the private channel's event: %q", got)
	}
	_ = pub
	_ = later

	if evs, _ := e.a.GetEvents(e.tk["alice"], iso(base.Add(30*time.Minute)), iso(base.Add(40*time.Minute))); len(evs) != 1 || evs[0].Title != "public" {
		t.Errorf("running event: %+v", evs)
	}
	if _, err := e.a.GetEvents("", from, to); err == nil {
		t.Error("no token accepted")
	}
	if _, err := e.a.GetEvents(e.tk["alice"], to, from); err == nil {
		t.Error("reversed range accepted")
	}
	if _, err := e.a.GetEvents(e.tk["alice"], "x", to); err == nil {
		t.Error("bad range accepted")
	}
	if _, err := e.a.GetEvents(e.tk["alice"], iso(base), iso(base.Add(200*24*time.Hour))); err == nil {
		t.Error("huge range accepted")
	}

	if _, err := e.a.RespondToEvent(e.tk["carol"], sec.ID, "carol", rsvpGoing); err == nil {
		t.Error("outsider answered a private event")
	}
	if err := e.a.DeleteEvent(e.tk["carol"], sec.ID, "carol"); err == nil {
		t.Error("outsider deleted a private event")
	}
}

func TestRespondUpdateDelete(t *testing.T) {
	e := setupCalendar(t)
	ev, _ := e.a.CreateEvent(e.tk["alice"], "general", "plan", "", iso(time.Now().Add(3*time.Hour)), 60)

	if _, err := e.a.RespondToEvent(e.tk["bob"], ev.ID, "alice", rsvpGoing); err == nil {
		t.Error("answered as someone else")
	}
	if _, err := e.a.RespondToEvent(e.tk["bob"], ev.ID, "bob", "whatever"); err == nil {
		t.Error("invalid answer accepted")
	}
	if _, err := e.a.RespondToEvent(e.tk["bob"], "missing", "bob", rsvpGoing); err == nil {
		t.Error("answered a missing event")
	}
	got, err := e.a.RespondToEvent(e.tk["bob"], ev.ID, "bob", rsvpMaybe)
	if err != nil || got.RSVP["bob"] != rsvpMaybe || got.RSVP["alice"] != rsvpGoing {
		t.Fatalf("%+v %v", got, err)
	}
	got, _ = e.a.RespondToEvent(e.tk["bob"], ev.ID, "bob", rsvpGoing)
	if got.RSVP["bob"] != rsvpGoing {
		t.Error("answer not changed")
	}
	got, _ = e.a.RespondToEvent(e.tk["bob"], ev.ID, "bob", "")
	if _, still := got.RSVP["bob"]; still {
		t.Error("answer not cleared")
	}

	newStart := iso(time.Now().Add(5 * time.Hour))
	if _, err := e.a.UpdateEvent(e.tk["bob"], ev.ID, "bob", "hijack", "", newStart, 30); err == nil {
		t.Error("non-creator edited")
	}
	if _, err := e.a.UpdateEvent(e.tk["alice"], ev.ID, "alice", "", "", newStart, 30); err == nil {
		t.Error("empty title accepted on edit")
	}
	up, err := e.a.UpdateEvent(e.tk["alice"], ev.ID, "alice", "plan v2", "new", newStart, 30)
	if err != nil || up.Title != "plan v2" || up.End.Sub(up.Start) != 30*time.Minute {
		t.Fatalf("%+v %v", up, err)
	}
	if up.RSVP["alice"] != rsvpGoing {
		t.Error("answers lost on edit")
	}

	if err := e.a.DeleteEvent(e.tk["bob"], ev.ID, "bob"); err == nil {
		t.Error("non-creator deleted")
	}
	if err := e.a.DeleteEvent(e.tk["alice"], ev.ID, "alice"); err != nil {
		t.Fatal(err)
	}
	if evs, _ := e.a.GetEvents(e.tk["alice"], iso(time.Now()), iso(time.Now().Add(24*time.Hour))); len(evs) != 0 {
		t.Errorf("deleted event still listed: %+v", evs)
	}
}

func TestReminders(t *testing.T) {
	e := setupCalendar(t)
	soon, _ := e.a.CreateEvent(e.tk["alice"], "general", "soon", "", iso(time.Now().Add(10*time.Minute)), 30)
	e.a.CreateEvent(e.tk["alice"], "general", "far", "", iso(time.Now().Add(3*time.Hour)), 30)
	e.a.CreateEvent(e.tk["alice"], "general", "past", "", iso(time.Now().Add(-2*time.Hour)), 30)
	secret, _ := e.a.CreateEvent(e.tk["alice"], "secret", "secret soon", "", iso(time.Now().Add(5*time.Minute)), 30)

	due, err := e.a.GetDueReminders(e.tk["alice"])
	if err != nil || len(due) != 2 {
		t.Fatalf("alice should get the two near events: %+v %v", due, err)
	}

	if again, _ := e.a.GetDueReminders(e.tk["alice"]); len(again) != 0 {
		t.Errorf("reminder repeated: %+v", again)
	}

	if d, _ := e.a.GetDueReminders(e.tk["bob"]); len(d) != 0 {
		t.Errorf("bob was not going: %+v", d)
	}
	e.a.RespondToEvent(e.tk["bob"], soon.ID, "bob", rsvpGoing)
	e.a.RespondToEvent(e.tk["bob"], secret.ID, "bob", rsvpMaybe)

	if d, _ := e.a.GetDueReminders(e.tk["bob"]); len(d) != 2 {
		t.Errorf("bob: %+v", d)
	}

	maybeEv, _ := e.a.CreateEvent(e.tk["alice"], "general", "maybe one", "", iso(time.Now().Add(8*time.Minute)), 30)
	e.a.RespondToEvent(e.tk["bob"], maybeEv.ID, "bob", rsvpMaybe)
	if d, _ := e.a.GetDueReminders(e.tk["bob"]); len(d) != 1 || d[0].ID != maybeEv.ID {
		t.Errorf("maybe must be reminded: %+v", d)
	}

	declEv, _ := e.a.CreateEvent(e.tk["alice"], "general", "declined one", "", iso(time.Now().Add(9*time.Minute)), 30)
	e.a.RespondToEvent(e.tk["bob"], declEv.ID, "bob", rsvpDeclined)
	e.a.RespondToEvent(e.tk["bob"], secret.ID, "bob", rsvpDeclined)
	if d, _ := e.a.GetDueReminders(e.tk["bob"]); len(d) != 0 {
		t.Errorf("declined: %+v", d)
	}

	e.a.GetDueReminders(e.tk["alice"])

	e.a.UpdateEvent(e.tk["alice"], soon.ID, "alice", "soon", "", iso(time.Now().Add(12*time.Minute)), 30)
	if d, _ := e.a.GetDueReminders(e.tk["alice"]); len(d) != 1 || d[0].ID != soon.ID {
		t.Errorf("moved event must remind again: %+v", d)
	}
	if _, err := e.a.GetDueReminders(""); err == nil {
		t.Error("no token accepted")
	}
}

func TestEventsDieWithTheirChannel(t *testing.T) {
	e := setupCalendar(t)
	e.a.CreateChannel(e.tk["alice"], "temp", "", "alice", false, nil)
	ev, _ := e.a.CreateEvent(e.tk["alice"], "temp", "gone soon", "", iso(time.Now().Add(time.Hour)), 30)
	keep, _ := e.a.CreateEvent(e.tk["alice"], "general", "stays", "", iso(time.Now().Add(time.Hour)), 30)

	if err := e.a.DeleteChannel(e.tk["alice"], "temp", "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := loadEvent(ev.ID); err == nil {
		t.Error("event of a deleted channel was kept")
	}
	if _, err := loadEvent(keep.ID); err != nil {
		t.Error("another channel's event was removed")
	}
}
