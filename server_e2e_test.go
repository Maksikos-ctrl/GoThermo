package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// The path a remote client takes: open a WebSocket on the server, send a
// message over RPC, and see it arrive on the socket.
func TestServerModeEndToEnd(t *testing.T) {
	mr := useMiniRedis(t)
	tok := mkUser(t, mr, "alice")
	if err := SaveChannel(Channel{ID: "c1", Name: "general"}); err != nil {
		t.Fatal(err)
	}
	hub := NewHub()
	go hub.Run()
	srv := httptest.NewServer(newServerMux(&App{hub: hub}, hub))
	// A connecting client starts a background job that reads Redis; let it
	// finish before the next test swaps the Redis client.
	defer time.Sleep(300 * time.Millisecond)
	defer srv.Close()

	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws?token="+tok, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	_ = ws.SetReadDeadline(time.Now().Add(3 * time.Second))

	resp, err := http.Post(srv.URL+"/rpc/SendMessage", "application/json",
		bytes.NewBufferString(`["`+tok+`","alice","over the wire","general"]`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || strings.Contains(string(body), `"error"`) {
		t.Fatalf("send failed: %d %s", resp.StatusCode, body)
	}

	for {
		_, data, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("message did not arrive over the socket: %v", err)
		}
		if strings.Contains(string(data), "over the wire") {
			return
		}
	}
}

func TestHealthz(t *testing.T) {
	srv := httptest.NewServer(newServerMux(&App{}, NewHub()))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("healthz: %v", err)
	}
}
