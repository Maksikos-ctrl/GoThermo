package main

import (
	"strings"
	"testing"
	"time"
)

// A client leaving used to freeze the hub for good: every later login or
// status change then hung waiting for the lock.
func TestHubSurvivesAClientLeaving(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	mk := func(name string) *Client {
		return &Client{Username: name, Send: make(chan []byte, 16)}
	}
	a, b := mk("ann"), mk("bob")
	hub.register <- a
	hub.register <- b
	time.Sleep(100 * time.Millisecond)

	hub.unregister <- a

	done := make(chan struct{})
	go func() {
		hub.BroadcastStatusUpdate("bob", "online")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("hub is stuck after a client disconnected")
	}

	// the remaining client heard that ann went offline
	deadline := time.After(2 * time.Second)
	for {
		select {
		case m := <-b.Send:
			if strings.Contains(string(m), `"offline"`) {
				return
			}
		case <-deadline:
			t.Fatal("bob never learned that ann left")
		}
	}
}
