package main

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

// useMiniRedis points the app at a throw-away in-memory Redis for one test.
// Your real Redis is never touched.
func useMiniRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr := miniredis.RunT(t)
	redisClient = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		// RegisterUser saves in a background goroutine; give it a moment so it
		// cannot touch the next test's Redis.
		time.Sleep(30 * time.Millisecond)
	})
	return mr
}

// mkUser registers a user in memory and waits until its background save has
// reached Redis. It returns the session token.
func mkUser(t *testing.T, mr *miniredis.Miniredis, name string) string {
	t.Helper()
	userManager.RegisterUser(name, name+"@x.com")
	deadline := time.Now().Add(2 * time.Second)
	for !mr.Exists("user:" + name + "@x.com") {
		if time.Now().After(deadline) {
			t.Fatalf("user %s was not saved", name)
		}
		time.Sleep(5 * time.Millisecond)
	}
	token, err := CreateSession(name)
	if err != nil {
		t.Fatal(err)
	}
	return token
}
