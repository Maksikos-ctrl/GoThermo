package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type typingEnv struct {
	srv *httptest.Server
	tk  map[string]string
}

func newTypingEnv(t *testing.T) typingEnv {
	t.Helper()
	mr := useMiniRedis(t)
	tk := map[string]string{}
	for _, u := range []string{"alice", "bob", "eve"} {
		tk[u] = mkUser(t, mr, u)
	}
	SaveChannel(Channel{Name: "general", CreatedBy: "system", CreatedAt: time.Now()})
	SaveChannel(Channel{Name: "dm_alice_bob", IsPrivate: true, Members: []string{"alice", "bob"}, CreatedBy: "alice", CreatedAt: time.Now()})
	hub := NewHub()
	go hub.Run()
	srv := httptest.NewServer(newServerMux(&App{hub: hub}, hub))
	t.Cleanup(func() {
		srv.Close()
		time.Sleep(300 * time.Millisecond)
	})
	return typingEnv{srv, tk}
}

// tconn is a websocket client that keeps collecting the typing events it gets.
// (A read that times out poisons a gorilla connection, so a background reader
// does the reading and the tests only look at what it has collected.)
type tconn struct {
	ws     *websocket.Conn
	mu     sync.Mutex
	events []TypingPayload
}

func (e typingEnv) dial(t *testing.T, user string) *tconn {
	t.Helper()
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(e.srv.URL, "http")+"/ws?token="+e.tk[user], nil)
	if err != nil {
		t.Fatal(err)
	}
	c := &tconn{ws: ws}
	go func() {
		for {
			_, data, err := ws.ReadMessage()
			if err != nil {
				return
			}
			var m struct {
				Type    string        `json:"type"`
				Payload TypingPayload `json:"payload"`
			}
			if json.Unmarshal(data, &m) == nil && m.Type == "typing" {
				c.mu.Lock()
				c.events = append(c.events, m.Payload)
				c.mu.Unlock()
			}
		}
	}()
	t.Cleanup(func() { ws.Close() })
	return c
}

func typing(t *testing.T, c *tconn, channel string) {
	t.Helper()
	b, _ := json.Marshal(WSMessage{Type: "typing", Payload: channel})
	if err := c.ws.WriteMessage(websocket.TextMessage, b); err != nil {
		t.Fatal(err)
	}
}

// typingSeen waits a moment, then returns (and forgets) what arrived.
func typingSeen(c *tconn, wait time.Duration) []TypingPayload {
	time.Sleep(wait)
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.events
	c.events = nil
	return out
}

func TestTypingReachesOthersButNotTheTypist(t *testing.T) {
	e := newTypingEnv(t)
	a, b := e.dial(t, "alice"), e.dial(t, "bob")
	time.Sleep(400 * time.Millisecond) // auto-subscribe to public channels

	typing(t, a, "general")

	got := typingSeen(b, 600*time.Millisecond)
	if len(got) != 1 || got[0].Channel != "general" || got[0].Username != "alice" {
		t.Fatalf("bob saw %v", got)
	}
	if mine := typingSeen(a, 300*time.Millisecond); len(mine) != 0 {
		t.Fatalf("the typist got her own event: %v", mine)
	}
}

func TestTypingIsRateLimitedPerChannel(t *testing.T) {
	e := newTypingEnv(t)
	a, b := e.dial(t, "alice"), e.dial(t, "bob")
	time.Sleep(400 * time.Millisecond)

	for i := 0; i < 20; i++ {
		typing(t, a, "general")
	}
	got := typingSeen(b, 600*time.Millisecond)
	if len(got) != 1 {
		t.Fatalf("20 quick keystrokes must give one event, got %d", len(got))
	}
}

func TestTypingInAPrivateChatStaysPrivate(t *testing.T) {
	e := newTypingEnv(t)
	a, b, eve := e.dial(t, "alice"), e.dial(t, "bob"), e.dial(t, "eve")
	time.Sleep(400 * time.Millisecond)

	typing(t, a, "dm_alice_bob")
	if got := typingSeen(b, 500*time.Millisecond); len(got) != 1 {
		t.Fatalf("the other member must see it: %v", got)
	}
	if got := typingSeen(eve, 300*time.Millisecond); len(got) != 0 {
		t.Fatalf("an outsider saw a private chat: %v", got)
	}

	// an outsider cannot type into it either
	typing(t, eve, "dm_alice_bob")
	if got := typingSeen(b, 400*time.Millisecond); len(got) != 0 {
		t.Fatalf("an outsider's typing reached the chat: %v", got)
	}
}

func TestTypingIgnoresJunk(t *testing.T) {
	e := newTypingEnv(t)
	a, b := e.dial(t, "alice"), e.dial(t, "bob")
	time.Sleep(400 * time.Millisecond)

	typing(t, a, "no-such-channel")
	typing(t, a, "")
	raw, _ := json.Marshal(WSMessage{Type: "typing", Payload: map[string]string{"channel": "general"}})
	_ = a.ws.WriteMessage(websocket.TextMessage, raw)
	if got := typingSeen(b, 500*time.Millisecond); len(got) != 0 {
		t.Fatalf("junk produced events: %v", got)
	}
}
