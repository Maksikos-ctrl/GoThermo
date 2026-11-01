package main

import (
	"reflect"
	"testing"
	"time"
)

func mustSaveEventChannel(t *testing.T, s Store, id, name string) {
	t.Helper()
	if err := s.SaveChannel(Channel{ID: id, Name: name, Members: []string{}, CreatedBy: "system", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
}

func mkEvent(id, channel, title string, start time.Time, mins int) *Event {
	return &Event{ID: id, Title: title, Channel: channel, CreatedBy: "alice",
		Start: start, End: start.Add(time.Duration(mins) * time.Minute),
		RSVP: map[string]string{"alice": rsvpGoing}, CreatedAt: time.Now()}
}

func eventIDs(events []Event) []string {
	ids := []string{}
	for _, e := range events {
		ids = append(ids, e.ID)
	}
	return ids
}

func TestStoreEventRoundTripAndUpdate(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		mustSaveEventChannel(t, s, "c1", "general")
		if _, err := s.GetEvent("nope"); err == nil {
			t.Fatal("a missing event must be an error")
		}

		start := time.Now().Add(time.Hour).Truncate(time.Second)
		ev := mkEvent("e1", "general", "Sync", start, 45)
		ev.Description = "agenda"
		ev.RSVP["bob"] = rsvpMaybe
		if err := s.SaveEvent(ev); err != nil {
			t.Fatal(err)
		}
		got, err := s.GetEvent("e1")
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != "Sync" || got.Description != "agenda" || got.Channel != "general" || got.CreatedBy != "alice" {
			t.Errorf("round trip: %+v", got)
		}
		if !got.Start.Equal(start) || !got.End.Equal(start.Add(45*time.Minute)) {
			t.Errorf("times: %v – %v", got.Start, got.End)
		}
		sameInstant(t, "createdAt", got.CreatedAt, ev.CreatedAt, time.Second)
		if !reflect.DeepEqual(got.RSVP, map[string]string{"alice": rsvpGoing, "bob": rsvpMaybe}) {
			t.Errorf("answers: %v", got.RSVP)
		}

		// Saving again replaces title, time and answers.
		ev.Title = "Sync 2"
		ev.Start, ev.End = start.Add(time.Hour), start.Add(2*time.Hour)
		ev.RSVP = map[string]string{"carol": rsvpGoing}
		if err := s.SaveEvent(ev); err != nil {
			t.Fatal(err)
		}
		got, _ = s.GetEvent("e1")
		if got.Title != "Sync 2" || !got.Start.Equal(start.Add(time.Hour)) ||
			!reflect.DeepEqual(got.RSVP, map[string]string{"carol": rsvpGoing}) {
			t.Errorf("after update: %+v", got)
		}

		// An event of a channel that does not exist cannot be saved.
		if err := s.SaveEvent(mkEvent("e2", "ghost", "x", start, 30)); err == nil {
			t.Error("saved into a missing channel")
		}
		if _, err := s.GetEvent("e2"); err == nil {
			t.Error("the failed save left an event behind")
		}
	})
}

func TestStoreEventAnswers(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		mustSaveEventChannel(t, s, "c1", "general")
		start := time.Now().Add(time.Hour)
		if err := s.SaveEvent(mkEvent("e1", "general", "x", start, 30)); err != nil {
			t.Fatal(err)
		}
		if err := s.SetAnswer("e1", "bob", rsvpGoing); err != nil {
			t.Fatal(err)
		}
		if err := s.SetAnswer("e1", "bob", rsvpDeclined); err != nil { // changes the answer
			t.Fatal(err)
		}
		if err := s.SetAnswer("e1", "alice", ""); err != nil { // takes it back
			t.Fatal(err)
		}
		got, _ := s.GetEvent("e1")
		if !reflect.DeepEqual(got.RSVP, map[string]string{"bob": rsvpDeclined}) {
			t.Errorf("answers: %v", got.RSVP)
		}
		if err := s.SetAnswer("missing", "bob", rsvpGoing); err == nil {
			t.Error("answered a missing event")
		}
	})
}

func TestStoreEventRanges(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		mustSaveEventChannel(t, s, "c1", "general")
		base := time.Now().Truncate(time.Second).Add(24 * time.Hour)
		at := func(h float64) time.Time { return base.Add(time.Duration(h * float64(time.Hour))) }
		for _, e := range []*Event{
			mkEvent("late", "general", "late", at(5), 60),
			mkEvent("early", "general", "early", at(1), 60),
			mkEvent("long", "general", "long", at(-10), 24*60), // started the day before, still running
			mkEvent("before", "general", "before", at(-3), 60),
			mkEvent("after", "general", "after", at(30), 60),
		} {
			if err := s.SaveEvent(e); err != nil {
				t.Fatal(err)
			}
		}

		// ordered by start; includes the one that began earlier but overlaps
		got, err := s.EventsOverlapping(at(0), at(10))
		if err != nil || !reflect.DeepEqual(eventIDs(got), []string{"long", "early", "late"}) {
			t.Fatalf("overlapping: %v %v", eventIDs(got), err)
		}
		// touching edges do not count: "before" ends exactly at -2h
		got, _ = s.EventsOverlapping(at(-2), at(1))
		if !reflect.DeepEqual(eventIDs(got), []string{"long"}) {
			t.Errorf("edges: %v", eventIDs(got))
		}

		// start inside [from, to], both ends inclusive
		got, err = s.EventsStarting(at(1), at(5))
		if err != nil || !reflect.DeepEqual(eventIDs(got), []string{"early", "late"}) {
			t.Fatalf("starting: %v %v", eventIDs(got), err)
		}
		got, _ = s.EventsStarting(at(1).Add(time.Second), at(5).Add(-time.Second))
		if len(got) != 0 {
			t.Errorf("starting, strict window: %v", eventIDs(got))
		}
		// the window is exact, not rounded to whole seconds
		got, _ = s.EventsStarting(at(1).Add(500*time.Millisecond), at(5))
		if !reflect.DeepEqual(eventIDs(got), []string{"late"}) {
			t.Errorf("sub-second window: %v", eventIDs(got))
		}
		// answers come with the events
		got, _ = s.EventsStarting(at(1), at(1))
		if len(got) != 1 || got[0].RSVP["alice"] != rsvpGoing {
			t.Errorf("answers in a range query: %+v", got)
		}
	})
}

func TestStoreEventReminders(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		mustSaveEventChannel(t, s, "c1", "general")
		if err := s.SaveEvent(mkEvent("e1", "general", "x", time.Now().Add(time.Hour), 30)); err != nil {
			t.Fatal(err)
		}
		first, err := s.ClaimReminder("e1", "alice")
		if err != nil || !first {
			t.Fatalf("first claim: %v %v", first, err)
		}
		if again, _ := s.ClaimReminder("e1", "alice"); again {
			t.Error("the same person was reminded twice")
		}
		if other, _ := s.ClaimReminder("e1", "bob"); !other {
			t.Error("another person must still be reminded")
		}
		if err := s.ResetReminders("e1"); err != nil {
			t.Fatal(err)
		}
		if again, _ := s.ClaimReminder("e1", "alice"); !again {
			t.Error("after a reset the reminder must be possible again")
		}
		if err := s.ResetReminders("missing"); err != nil {
			t.Errorf("resetting a missing event is not an error: %v", err)
		}
	})
}

func TestStoreEventRemoval(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		mustSaveEventChannel(t, s, "c1", "general")
		mustSaveEventChannel(t, s, "c2", "temp")
		start := time.Now().Add(time.Hour)
		for _, e := range []*Event{
			mkEvent("g1", "general", "g1", start, 30),
			mkEvent("t1", "temp", "t1", start, 30),
			mkEvent("t2", "temp", "t2", start.Add(time.Hour), 30),
		} {
			if err := s.SaveEvent(e); err != nil {
				t.Fatal(err)
			}
		}
		s.ClaimReminder("t1", "alice")

		if err := s.RemoveEvent("g1"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.GetEvent("g1"); err == nil {
			t.Error("removed event is still there")
		}
		if err := s.RemoveEvent("g1"); err != nil {
			t.Errorf("removing twice is not an error: %v", err)
		}

		if err := s.RemoveChannelEvents("temp"); err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"t1", "t2"} {
			if _, err := s.GetEvent(id); err == nil {
				t.Errorf("%s survived its channel's removal", id)
			}
		}
		if err := s.RemoveChannelEvents("ghost"); err != nil {
			t.Errorf("unknown channel: %v", err)
		}
		left, _ := s.EventsOverlapping(time.Now().Add(-time.Hour), time.Now().Add(48*time.Hour))
		if len(left) != 0 {
			t.Errorf("left over: %v", eventIDs(left))
		}
	})
}

// Deleting the channel itself must take its events along on Postgres (foreign
// key) and, through the app, on Redis.
func TestStoreDeleteChannelDropsEvents(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		mustSaveEventChannel(t, s, "c1", "temp")
		s.SaveEvent(mkEvent("e1", "temp", "x", time.Now().Add(time.Hour), 30))
		if err := s.DeleteChannel("temp"); err != nil {
			t.Fatal(err)
		}
		if _, isPG := s.(pgStore); isPG {
			if _, err := s.GetEvent("e1"); err == nil {
				t.Error("the event outlived the channel")
			}
		} else {
			s.RemoveChannelEvents("temp") // what App.DeleteChannel does for Redis
			if _, err := s.GetEvent("e1"); err == nil {
				t.Error("the event outlived the channel")
			}
		}
	})
}
