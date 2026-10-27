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

// After a quick reconnect the old connection's cleanup used to remove the NEW
// connection from the hub, so the user stopped getting live messages until the
// next reconnect.
func TestReconnectKeepsTheNewConnection(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	mk := func() *Client { return &Client{Username: "bob", Send: make(chan []byte, 16)} }
	oldC, newC := mk(), mk()

	hub.register <- oldC
	time.Sleep(50 * time.Millisecond)
	hub.register <- newC // reconnected before the old one was noticed as dead
	time.Sleep(50 * time.Millisecond)
	hub.unregister <- oldC // ...and now the old connection finally goes away
	time.Sleep(100 * time.Millisecond)

	if !hub.SendToUser("bob", []byte(`{"type":"ping"}`)) {
		t.Fatal("the new connection was removed together with the old one")
	}
	deadline := time.After(time.Second)
	for {
		select {
		case m := <-newC.Send:
			if strings.Contains(string(m), `"ping"`) {
				return
			}
		case <-deadline:
			t.Fatal("the new connection did not get the message")
		}
	}
}

func TestReplacedConnectionIsShutDown(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	oldC := &Client{Username: "bob", Send: make(chan []byte, 16)}
	newC := &Client{Username: "bob", Send: make(chan []byte, 16)}
	hub.register <- oldC
	time.Sleep(50 * time.Millisecond)
	hub.register <- newC
	time.Sleep(50 * time.Millisecond)

	// the old connection's channel is closed, so its writer stops
	closed := false
	for i := 0; i < 20 && !closed; i++ {
		select {
		case _, ok := <-oldC.Send:
			if !ok {
				closed = true
			}
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !closed {
		t.Fatal("the replaced connection was left open")
	}
}

func TestUnregisterOfAnUnknownClientIsHarmless(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	stranger := &Client{Username: "ghost", Send: make(chan []byte, 1)}
	hub.unregister <- stranger // never registered: must not panic or block
	done := make(chan struct{})
	go func() {
		hub.BroadcastStatusUpdate("x", "online")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("hub is stuck")
	}
}
