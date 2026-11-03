package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestIceStunOnlyWithoutTurn(t *testing.T) {
	got := buildIceServers("", "", "ann", time.Now())
	if len(got) != 1 || !strings.HasPrefix(got[0].URLs[0], "stun:") {
		t.Fatalf("want STUN only, got %+v", got)
	}
	// URLs without a secret must not hand out half-working TURN entries.
	if got := buildIceServers("turn:x:3478", "", "ann", time.Now()); len(got) != 1 {
		t.Fatalf("TURN without secret: %+v", got)
	}
}

func TestIceTurnCredentials(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	got := buildIceServers("turn:h:3478?transport=udp, turn:h:3478?transport=tcp", "s3cret", "ann", now)
	if len(got) != 2 || len(got[1].URLs) != 2 {
		t.Fatalf("got %+v", got)
	}
	wantName := "1700043200:ann"
	if got[1].Username != wantName {
		t.Fatalf("username %q", got[1].Username)
	}
	mac := hmac.New(sha1.New, []byte("s3cret"))
	mac.Write([]byte(wantName))
	if got[1].Credential != base64.StdEncoding.EncodeToString(mac.Sum(nil)) {
		t.Fatal("credential does not match coturn's scheme")
	}
}

func TestGetIceServersNeedsSession(t *testing.T) {
	useMiniRedis(t)
	t.Setenv("GOTHERMO_TURN_URLS", "turn:h:3478")
	t.Setenv("GOTHERMO_TURN_SECRET", "s")
	a := &App{hub: NewHub()}
	if _, err := a.GetIceServers("bogus"); err == nil {
		t.Fatal("anonymous caller got TURN credentials")
	}
	tok, _ := CreateSession("ann")
	s, err := a.GetIceServers(tok)
	if err != nil {
		t.Fatal(err)
	}
	var list []iceServer
	if err := json.Unmarshal([]byte(s), &list); err != nil || len(list) != 2 {
		t.Fatalf("%v %s", err, s)
	}
}
