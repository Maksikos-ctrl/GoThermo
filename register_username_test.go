package main

import (
	"errors"
	"sync"
	"testing"
)

func useStore(t *testing.T, s Store) {
	t.Helper()
	useMiniRedis(t)
	prev := store
	store = s
	t.Cleanup(func() { store = prev })
}

func forgetUsers(t *testing.T, names ...string) {
	t.Cleanup(func() {
		userManager.mu.Lock()
		defer userManager.mu.Unlock()
		for _, n := range names {
			delete(userManager.users, n)
		}
	})
}

// A name that exists in the database but not in the in-memory cache must be
// skipped, not turned into a "duplicate key" error for the person signing up.
// Only Postgres enforces unique usernames.
func TestRegisterSkipsUsernameTakenInStore(t *testing.T) {
	s := pgStore{pool: migratedTestDatabase(t)}
	{
		useStore(t, s)
		forgetUsers(t, "rutaken", "rutaken2")
		mustSaveUser(t, s, User{ID: "db-1", Username: "rutaken", Email: "other@x.com"})

		okDNS(t, func() {
			u, err := (&App{}).Register("rutaken@gmail.com", "secret1")
			if err != nil {
				t.Fatalf("register: %v", err)
			}
			if u.Username != "rutaken2" {
				t.Fatalf("username = %q, want rutaken2", u.Username)
			}
			if got, err := s.GetUserByEmail("rutaken@gmail.com"); err != nil || got.Username != "rutaken2" {
				t.Fatalf("stored user = %+v, %v", got, err)
			}
		})
	}
}

type failingSave struct {
	Store
	err error
}

func (f failingSave) SaveUser(*User) error { return f.err }

// A failed save must not leave a half-registered user in memory.
func TestRegisterFailedSaveLeavesNoGhost(t *testing.T) {
	useStore(t, failingSave{Store: redisStore{}, err: errors.New("disk full")})
	forgetUsers(t, "rughost")

	okDNS(t, func() {
		if _, err := (&App{}).Register("rughost@gmail.com", "secret1"); err == nil {
			t.Fatal("expected an error")
		}
	})
	if _, ok := userManager.GetUser("rughost"); ok {
		t.Fatal("failed registration left a user in memory")
	}
}

func TestRegisterSameLocalPartGetsDifferentNames(t *testing.T) {
	forEachStore(t, func(t *testing.T, s Store) {
		useStore(t, s)
		forgetUsers(t, "rupair", "rupair2", "rupair3", "rupair4")

		okDNS(t, func() {
			var wg sync.WaitGroup
			names := make(chan string, 4)
			for _, e := range []string{"rupair@a.com", "rupair@b.com", "rupair@c.com", "rupair@d.com"} {
				wg.Add(1)
				go func(e string) {
					defer wg.Done()
					u, err := (&App{}).Register(e, "secret1")
					if err != nil {
						t.Errorf("%s: %v", e, err)
						return
					}
					names <- u.Username
				}(e)
			}
			wg.Wait()
			close(names)
			seen := map[string]bool{}
			for n := range names {
				if seen[n] {
					t.Fatalf("username %q given twice", n)
				}
				seen[n] = true
			}
			if len(seen) != 4 {
				t.Fatalf("got %d names, want 4", len(seen))
			}
		})
	})
}
