package main

import (
	"net"
	"strings"
	"testing"
	"time"
)

func okDNS(t *testing.T, fn func()) {
	withLookups(func(string) ([]*net.MX, error) { return []*net.MX{{Host: "mx."}}, nil }, nil, fn)
}

func TestRegisterAndLogin(t *testing.T) {
	mr := useMiniRedis(t)
	a := &App{}

	okDNS(t, func() {
		if _, err := a.Register("not-an-email", "secret1"); err == nil {
			t.Error("bad email accepted")
		}
		if _, err := a.Register("zed@example.com", "123"); err == nil {
			t.Error("short password accepted")
		}
		u, err := a.Register("zed@example.com", "secret1")
		if err != nil {
			t.Fatal(err)
		}
		if u.Token == "" || u.Username != "zed" {
			t.Fatalf("%+v", u)
		}
		if name, ok := GetSessionUsername(u.Token); !ok || name != "zed" {
			t.Fatal("registration did not open a session")
		}
		if _, err := a.Register("zed@example.com", "secret1"); err == nil {
			t.Error("duplicate registration accepted")
		}
		// the password is stored hashed, never as text
		if h, _ := mr.Get("user:zed@example.com:password"); h == "" || strings.Contains(h, "secret1") {
			t.Errorf("password not hashed: %q", h)
		}
	})

	if _, err := a.Login("zed@example.com", ""); err == nil {
		t.Error("empty password accepted")
	}
	if _, err := a.Login("zed@example.com", "wrong-pass"); err == nil || err.Error() != "incorrect email or password" {
		t.Errorf("wrong password: %v", err)
	}
	if _, err := a.Login("nobody@example.com", "secret1"); err == nil || err.Error() != "incorrect email or password" {
		t.Errorf("unknown email must look the same as a wrong password: %v", err)
	}
	u, err := a.Login("zed@example.com", "secret1")
	if err != nil || u.Token == "" {
		t.Fatalf("login: %+v %v", u, err)
	}
	// every login gets its own session
	u2, _ := a.Login("zed@example.com", "secret1")
	if u2.Token == u.Token {
		t.Error("two logins share a token")
	}
}

func TestLoginLockout(t *testing.T) {
	mr := useMiniRedis(t)
	a := &App{}
	okDNS(t, func() {
		if _, err := a.Register("lock@example.com", "secret1"); err != nil {
			t.Fatal(err)
		}
	})

	for i := 0; i < maxLoginFailures; i++ {
		if _, err := a.Login("lock@example.com", "bad-pass"); err == nil || strings.Contains(err.Error(), "too many") {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	// now even the right password is refused
	if _, err := a.Login("lock@example.com", "secret1"); err == nil || !strings.Contains(err.Error(), "too many failed attempts") {
		t.Fatalf("not locked: %v", err)
	}
	// the lock is per account, case-insensitive
	if _, err := a.Login("LOCK@example.com", "secret1"); err == nil {
		t.Error("lock bypassed by changing case")
	}
	// it expires
	mr.FastForward(loginLockWindow + time.Minute)
	if _, err := a.Login("lock@example.com", "secret1"); err != nil {
		t.Fatalf("still locked after the window: %v", err)
	}
	// success resets the counter
	for i := 0; i < maxLoginFailures-1; i++ {
		a.Login("lock@example.com", "bad-pass")
	}
	if _, err := a.Login("lock@example.com", "secret1"); err != nil {
		t.Fatal(err)
	}
	if mr.Exists(loginFailKey("lock@example.com")) {
		t.Error("counter not cleared after a successful login")
	}
}

func TestSessions(t *testing.T) {
	mr := useMiniRedis(t)
	a := &App{}
	tok := mkUser(t, mr, "sam")

	if name, ok := GetSessionUsername(tok); !ok || name != "sam" {
		t.Fatal("valid token rejected")
	}
	if _, ok := GetSessionUsername(""); ok {
		t.Error("empty token accepted")
	}
	if _, ok := GetSessionUsername("deadbeef"); ok {
		t.Error("unknown token accepted")
	}
	if ttl := mr.TTL("session:" + tok); ttl <= 0 || ttl > sessionTTL {
		t.Errorf("session ttl = %v", ttl)
	}

	if s, err := a.CheckAuth(tok); err != nil || !strings.Contains(s, `"sam"`) {
		t.Errorf("CheckAuth: %q %v", s, err)
	}
	if s, _ := a.CheckAuth("deadbeef"); s != "" {
		t.Error("CheckAuth answered for an unknown token")
	}

	a.Logout(tok)
	if _, ok := GetSessionUsername(tok); ok {
		t.Error("token still valid after logout")
	}
	if err := requireUser(tok, "sam"); err == nil {
		t.Error("logged-out token still authorizes")
	}
}
