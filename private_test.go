package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func setupPrivate(t *testing.T) (*App, map[string]string) {
	mr := useMiniRedis(t)
	hub := NewHub()
	go hub.Run()
	a := &App{hub: hub}
	tokens := map[string]string{}
	for _, u := range []string{"alice", "bob", "carol"} {
		tokens[u] = mkUser(t, mr, u)
	}
	SaveChannel(Channel{Name: "general", CreatedBy: "system", CreatedAt: time.Now()})
	return a, tokens
}

func TestPrivateChannelLifecycle(t *testing.T) {
	a, tk := setupPrivate(t)

	if _, err := a.CreateChannel(tk["alice"], "secret", "d", "alice", true, []string{"bob", "ghost"}); err == nil {
		t.Fatal("unknown member accepted")
	}
	ch, err := a.CreateChannel(tk["alice"], "secret", "d", "alice", true, []string{"bob", "bob", " ", "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if !ch.IsPrivate || len(ch.Members) != 2 || ch.Members[0] != "alice" || ch.Members[1] != "bob" {
		t.Fatalf("members: %+v", ch)
	}
	if _, err := a.CreateChannel(tk["bob"], "x", "", "alice", true, nil); err == nil {
		t.Fatal("impersonated creator")
	}

	names := func(token, user string) string {
		cs, err := a.GetChannels(token)
		if err != nil {
			t.Fatal(err)
		}
		var n []string
		for _, c := range cs {
			n = append(n, c.Name)
		}
		return strings.Join(n, ",")
	}
	if got := names(tk["alice"], "alice"); !strings.Contains(got, "secret") {
		t.Fatal("creator cannot see it:", got)
	}
	if got := names(tk["bob"], "bob"); !strings.Contains(got, "secret") {
		t.Fatal("member cannot see it:", got)
	}
	if got := names(tk["carol"], "carol"); strings.Contains(got, "secret") {
		t.Fatal("outsider sees it:", got)
	}

	// messages / search / unread are closed for outsiders
	if _, err := a.SendMessage(tk["alice"], "alice", "top secret", "secret"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SendMessage(tk["carol"], "carol", "hi", "secret"); err == nil {
		t.Fatal("outsider posted")
	}
	if _, err := a.GetMessages(tk["carol"], "secret"); err == nil {
		t.Fatal("outsider read")
	}
	if r, _ := a.SearchMessages(tk["carol"], "carol", "top secret"); len(r) != 0 {
		t.Fatal("outsider found message in search")
	}
	if r, _ := a.SearchMessages(tk["bob"], "bob", "top secret"); len(r) != 1 {
		t.Fatal("member cannot search private channel")
	}
	if c, _ := a.GetUnreadCounts(tk["carol"], "carol"); func() bool { _, ok := c["secret"]; return ok }() {
		t.Fatal("outsider unread has private channel")
	}
	if c, _ := a.GetUnreadCounts(tk["bob"], "bob"); func() bool { _, ok := c["secret"]; return !ok }() {
		t.Fatal("member unread misses private channel")
	}
	if err := a.JoinChannel(tk["carol"], "secret", "carol"); err == nil {
		t.Fatal("outsider joined")
	}
	if _, err := a.ExportChannel(tk["carol"], "secret", "carol", "txt"); err == nil {
		t.Fatal("outsider exported")
	}

	// invites
	if _, err := a.AddChannelMember(tk["carol"], "secret", "carol", "carol"); err == nil {
		t.Fatal("outsider invited self")
	}
	if _, err := a.AddChannelMember(tk["bob"], "secret", "alice", "carol"); err == nil {
		t.Fatal("impersonated inviter")
	}
	if _, err := a.AddChannelMember(tk["bob"], "secret", "bob", "ghost"); err == nil {
		t.Fatal("unknown user invited")
	}
	if _, err := a.AddChannelMember(tk["bob"], "general", "bob", "carol"); err == nil {
		t.Fatal("added member to public channel")
	}
	got, err := a.AddChannelMember(tk["bob"], "secret", "bob", "carol")
	if err != nil || len(got.Members) != 3 {
		t.Fatal(err, got)
	}
	if again, _ := a.AddChannelMember(tk["bob"], "secret", "bob", "carol"); len(again.Members) != 3 {
		t.Fatal("duplicate member added")
	}
	if got := names(tk["carol"], "carol"); !strings.Contains(got, "secret") {
		t.Fatal("new member cannot see it")
	}
	if m, err := a.GetMessages(tk["carol"], "secret"); err != nil || len(m) != 1 {
		t.Fatal(err, m)
	}

	// only the creator deletes
	if err := a.DeleteChannel(tk["bob"], "secret", "bob"); err == nil {
		t.Fatal("non-creator deleted")
	}
	if err := a.DeleteChannel(tk["alice"], "secret", "alice"); err != nil {
		t.Fatal(err)
	}
}

type wsClient struct {
	conn *websocket.Conn
	got  chan WSMessage
}

func dial(t *testing.T, srv *httptest.Server, token string) *wsClient {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?token=" + token
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	c := &wsClient{conn: conn, got: make(chan WSMessage, 100)}
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			for _, line := range strings.Split(string(data), "\n") {
				var m WSMessage
				if json.Unmarshal([]byte(line), &m) == nil {
					c.got <- m
				}
			}
		}
	}()
	t.Cleanup(func() { conn.Close() })
	return c
}

// waitFor returns true if a message of the given type (matching check) arrives.
func (c *wsClient) waitFor(typ string, d time.Duration, check func(WSMessage) bool) bool {
	deadline := time.After(d)
	for {
		select {
		case m := <-c.got:
			if m.Type == typ && (check == nil || check(m)) {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

func TestPrivateChannelWebSocketDelivery(t *testing.T) {
	a, tk := setupPrivate(t)
	srv := httptest.NewServer(ServeWS(a.hub))
	defer srv.Close()

	ca, cb, cc := dial(t, srv, tk["alice"]), dial(t, srv, tk["bob"]), dial(t, srv, tk["carol"])
	time.Sleep(400 * time.Millisecond) // auto-subscribe to public channels

	if _, err := a.CreateChannel(tk["alice"], "secret", "", "alice", true, []string{"bob"}); err != nil {
		t.Fatal(err)
	}
	isSecret := func(m WSMessage) bool {
		b, _ := json.Marshal(m.Payload)
		return strings.Contains(string(b), "secret")
	}
	if !cb.waitFor("channel_created", time.Second, isSecret) {
		t.Fatal("member not told about new private channel")
	}
	if cc.waitFor("channel_created", 300*time.Millisecond, isSecret) {
		t.Fatal("outsider told about private channel")
	}

	// carol tries to subscribe by hand
	cc.conn.WriteJSON(WSMessage{Type: "subscribe_channel", Payload: "secret"})
	if !cc.waitFor("subscribed", time.Second, func(m WSMessage) bool {
		return m.Payload.(map[string]interface{})["success"] == false
	}) {
		t.Fatal("outsider subscription not refused")
	}
	// bob never subscribed explicitly but still receives (member delivery)
	if _, err := a.SendMessage(tk["alice"], "alice", "for bob only", "secret"); err != nil {
		t.Fatal(err)
	}
	isText := func(m WSMessage) bool {
		b, _ := json.Marshal(m.Payload)
		return strings.Contains(string(b), "for bob only")
	}
	if !cb.waitFor("channel_message", time.Second, isText) {
		t.Fatal("member did not get the message")
	}
	if !ca.waitFor("channel_message", time.Second, isText) {
		t.Fatal("creator did not get the message")
	}
	if cc.waitFor("channel_message", 400*time.Millisecond, isText) {
		t.Fatal("OUTSIDER GOT A PRIVATE MESSAGE")
	}

	// public channels still reach everyone
	if _, err := a.SendMessage(tk["alice"], "alice", "hello all", "general"); err != nil {
		t.Fatal(err)
	}
	isAll := func(m WSMessage) bool {
		b, _ := json.Marshal(m.Payload)
		return strings.Contains(string(b), "hello all")
	}
	if !cc.waitFor("channel_message", time.Second, isAll) {
		t.Fatal("public message not delivered")
	}

	// invite carol: she is told, then gets messages
	if _, err := a.AddChannelMember(tk["bob"], "secret", "bob", "carol"); err != nil {
		t.Fatal(err)
	}
	if !cc.waitFor("channel_created", time.Second, isSecret) {
		t.Fatal("invited user not told")
	}
	a.SendMessage(tk["alice"], "alice", "welcome carol", "secret")
	if !cc.waitFor("channel_message", time.Second, func(m WSMessage) bool {
		b, _ := json.Marshal(m.Payload)
		return strings.Contains(string(b), "welcome carol")
	}) {
		t.Fatal("invited user does not get messages")
	}

	// deletion is announced to members only
	if err := a.DeleteChannel(tk["alice"], "secret", "alice"); err != nil {
		t.Fatal(err)
	}
	if !cb.waitFor("channel_deleted", time.Second, nil) {
		t.Fatal("member not told about deletion")
	}
}
