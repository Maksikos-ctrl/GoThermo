package main

import (
	"sort"
	"testing"
	"time"
)

// Contract tests: the same checks run against every Store implementation, so
// Redis and Postgres are held to exactly the same behaviour.

type storeCase struct {
	name string
	make func(t *testing.T) Store
}

func storeCases() []storeCase {
	return []storeCase{
		{"redis", func(t *testing.T) Store {
			useMiniRedis(t)
			return redisStore{}
		}},
		{"postgres", func(t *testing.T) Store {
			return pgStore{pool: migratedTestDatabase(t)}
		}},
	}
}

func forEachStore(t *testing.T, fn func(t *testing.T, s Store)) {
	for _, c := range storeCases() {
		c := c
		t.Run(c.name, func(t *testing.T) { fn(t, c.make(t)) })
	}
}

func sameInstant(t *testing.T, what string, got, want time.Time, within time.Duration) {
	t.Helper()
	d := got.Sub(want)
	if d < 0 {
		d = -d
	}
	if d > within {
		t.Errorf("%s: got %v, want %v (±%v)", what, got, want, within)
	}
}

func mustSaveUser(t *testing.T, s Store, u User) {
	t.Helper()
	if err := s.SaveUser(&u); err != nil {
		t.Fatalf("SaveUser(%s): %v", u.Username, err)
	}
}

// ───────────────────────── users ─────────────────────────

func TestStoreUserRoundTrip(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		if _, err := s.GetUserByEmail("nobody@x.com"); err == nil {
			t.Fatal("a missing user must be an error")
		}

		seen := time.Now().Truncate(time.Second)
		mustSaveUser(t, s, User{ID: "u1", Username: "alice", Email: "alice@x.com",
			Status: "online", IsOnline: true, LastSeen: seen.Format(time.RFC3339)})

		got, err := s.GetUserByEmail("alice@x.com")
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != "u1" || got.Username != "alice" || got.Email != "alice@x.com" || got.Status != "online" || !got.IsOnline {
			t.Fatalf("wrong user back: %+v", got)
		}
		lastSeen, err := time.Parse(time.RFC3339, got.LastSeen)
		if err != nil {
			t.Fatalf("LastSeen %q: %v", got.LastSeen, err)
		}
		sameInstant(t, "LastSeen", lastSeen, seen, time.Second)

		// Saving again updates the same person.
		// (the app always keeps IsOnline in step with Status: online unless offline)
		mustSaveUser(t, s, User{ID: "u1", Username: "alice", Email: "alice@x.com", Status: "away", IsOnline: true, LastSeen: seen.Format(time.RFC3339)})
		got, _ = s.GetUserByEmail("alice@x.com")
		if got.Status != "away" || got.IsOnline != true {
			t.Fatalf("status not updated: %+v", got)
		}
		all, err := s.ListUsers()
		if err != nil || len(all) != 1 {
			t.Fatalf("want exactly one user, got %d (err %v)", len(all), err)
		}
	})
}

func TestStorePasswordHash(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		mustSaveUser(t, s, User{ID: "u1", Username: "alice", Email: "alice@x.com", Status: "offline"})

		if h, err := s.PasswordHash("alice@x.com"); err == nil && h != "" {
			t.Fatalf("no password yet, got %q", h)
		}
		if err := s.SetPasswordHash("alice@x.com", "hash-1"); err != nil {
			t.Fatal(err)
		}
		if h, err := s.PasswordHash("alice@x.com"); err != nil || h != "hash-1" {
			t.Fatalf("got %q, %v", h, err)
		}

		// A later status update (which saves the user again) must not lose it.
		mustSaveUser(t, s, User{ID: "u1", Username: "alice", Email: "alice@x.com", Status: "online"})
		if h, _ := s.PasswordHash("alice@x.com"); h != "hash-1" {
			t.Fatalf("saving the user wiped the password, got %q", h)
		}

		if err := s.SetPasswordHash("alice@x.com", "hash-2"); err != nil {
			t.Fatal(err)
		}
		if h, _ := s.PasswordHash("alice@x.com"); h != "hash-2" {
			t.Fatalf("password not replaced, got %q", h)
		}
	})
}

func TestStoreGoogleAccountIsSeparateFromPasswordAccount(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		// The same email address on purpose.
		mustSaveUser(t, s, User{ID: "u1", Username: "ivan", Email: "ivan@gmail.com", Status: "offline"})
		mustSaveUser(t, s, User{ID: "u2", Username: "ivan-2", Email: "ivan@gmail.com", Status: "offline",
			Provider: "google", ProviderID: "sub-1"})
		if err := s.SetPasswordHash("ivan@gmail.com", "pw-hash"); err != nil {
			t.Fatal(err)
		}

		// Password sign-in finds the password account, never the Google one.
		got, err := s.GetUserByEmail("ivan@gmail.com")
		if err != nil || got.Username != "ivan" {
			t.Fatalf("GetUserByEmail: %+v, %v", got, err)
		}
		if h, _ := s.PasswordHash("ivan@gmail.com"); h != "pw-hash" {
			t.Fatalf("password belongs to the password account, got %q", h)
		}

		// Google sign-in finds the Google account by its Google id.
		name, found, err := s.UsernameByProvider("google", "sub-1")
		if err != nil || !found || name != "ivan-2" {
			t.Fatalf("UsernameByProvider: %q %v %v", name, found, err)
		}
		if _, found, err := s.UsernameByProvider("google", "unknown"); err != nil || found {
			t.Fatalf("an unknown Google id must be 'not found' without an error: %v %v", found, err)
		}

		all, _ := s.ListUsers()
		if len(all) != 2 {
			t.Fatalf("both accounts must be listed, got %d", len(all))
		}
		for _, u := range all {
			if u.Username == "ivan-2" && (u.Provider != "google" || u.ProviderID != "sub-1") {
				t.Fatalf("provider fields lost: %+v", u)
			}
		}
	})
}

// ───────────────────────── channels ─────────────────────────

func TestStoreChannels(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		if _, err := s.GetChannel("general"); err == nil {
			t.Fatal("a missing channel must be an error")
		}

		created := time.Now().Add(-time.Hour)
		if err := s.SaveChannel(Channel{ID: "c1", Name: "general", Description: "everyone",
			Members: []string{"alice", "bob"}, CreatedBy: "system", CreatedAt: created}); err != nil {
			t.Fatal(err)
		}
		got, err := s.GetChannel("general")
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != "c1" || got.Name != "general" || got.Description != "everyone" || got.CreatedBy != "system" || got.IsPrivate {
			t.Fatalf("wrong channel back: %+v", got)
		}
		if len(got.Members) != 2 || got.Members[0] != "alice" || got.Members[1] != "bob" {
			t.Fatalf("members must keep their order, got %v", got.Members)
		}
		sameInstant(t, "CreatedAt", got.CreatedAt, created, time.Millisecond)

		// Saving again replaces description and members.
		if err := s.SaveChannel(Channel{ID: "c1", Name: "general", Description: "changed",
			Members: []string{"bob", "carol", "alice"}, CreatedBy: "system", CreatedAt: created}); err != nil {
			t.Fatal(err)
		}
		got, _ = s.GetChannel("general")
		if got.Description != "changed" || len(got.Members) != 3 || got.Members[0] != "bob" || got.Members[2] != "alice" {
			t.Fatalf("channel not replaced: %+v", got)
		}

		// A private chat.
		if err := s.SaveChannel(Channel{ID: "c2", Name: "dm_alice_bob", IsPrivate: true,
			Members: []string{"alice", "bob"}, CreatedBy: "alice", CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
		all, err := s.ListChannels()
		if err != nil || len(all) != 2 {
			t.Fatalf("want 2 channels, got %d (err %v)", len(all), err)
		}
		sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
		if all[0].Name != "dm_alice_bob" || !all[0].IsPrivate || len(all[0].Members) != 2 || all[1].Name != "general" || len(all[1].Members) != 3 {
			t.Fatalf("list is wrong: %+v", all)
		}

		// Delete.
		if err := s.DeleteChannel("general"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.GetChannel("general"); err == nil {
			t.Fatal("deleted channel is still there")
		}
		if all, _ := s.ListChannels(); len(all) != 1 || all[0].Name != "dm_alice_bob" {
			t.Fatalf("the other channel must stay: %+v", all)
		}
		if err := s.DeleteChannel("never-existed"); err != nil {
			t.Fatalf("deleting a missing channel is not an error: %v", err)
		}
	})
}

// ───────────────────────── read marks ─────────────────────────

func TestStoreLastRead(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		for _, name := range []string{"general", "random"} {
			if err := s.SaveChannel(Channel{ID: "id-" + name, Name: name, Members: []string{}, CreatedBy: "system", CreatedAt: time.Now()}); err != nil {
				t.Fatal(err)
			}
		}

		if _, found, err := s.LastRead("alice", "general"); err != nil || found {
			t.Fatalf("never opened: found=%v err=%v", found, err)
		}

		before := time.Now()
		if err := s.SaveLastRead("alice", "general"); err != nil {
			t.Fatal(err)
		}
		got, found, err := s.LastRead("alice", "general")
		if err != nil || !found {
			t.Fatalf("found=%v err=%v", found, err)
		}
		sameInstant(t, "first mark", got, before, 2*time.Second)

		time.Sleep(20 * time.Millisecond)
		if err := s.SaveLastRead("alice", "general"); err != nil {
			t.Fatal(err)
		}
		again, _, _ := s.LastRead("alice", "general")
		if !again.After(got) {
			t.Fatalf("a second mark must move the time forward: %v then %v", got, again)
		}

		// Marks are per person and per channel.
		if _, found, _ := s.LastRead("bob", "general"); found {
			t.Fatal("bob has not opened it")
		}
		if _, found, _ := s.LastRead("alice", "random"); found {
			t.Fatal("alice has not opened #random")
		}

		// Deleting the channel removes its marks.
		s.SaveLastRead("bob", "general")
		if err := s.DeleteChannel("general"); err != nil {
			t.Fatal(err)
		}
		s.SaveChannel(Channel{ID: "id-new", Name: "general", Members: []string{}, CreatedBy: "system", CreatedAt: time.Now()})
		for _, who := range []string{"alice", "bob"} {
			if _, found, _ := s.LastRead(who, "general"); found {
				t.Fatalf("%s: a re-created channel must not inherit old read marks", who)
			}
		}
	})
}
