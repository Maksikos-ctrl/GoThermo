package main

import (
	"crypto/subtle"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Closed registration for server mode. When GOTHERMO_INVITE_CODE is set, the
// Register call over the network must carry the same code in the
// X-Invite-Code header. Sign-in is not affected.

const (
	inviteHeader      = "X-Invite-Code"
	inviteMaxFailures = 10
	inviteWindow      = 15 * time.Minute
)

func inviteRequired() string { return os.Getenv("GOTHERMO_INVITE_CODE") }

// failLimiter counts wrong guesses per client so the code cannot be brute-forced.
type failLimiter struct {
	mu    sync.Mutex
	max   int
	win   time.Duration
	fails map[string][]time.Time
}

func newFailLimiter(max int, win time.Duration) *failLimiter {
	return &failLimiter{max: max, win: win, fails: map[string][]time.Time{}}
}

func (l *failLimiter) recent(key string, now time.Time) []time.Time {
	kept := l.fails[key][:0]
	for _, t := range l.fails[key] {
		if now.Sub(t) < l.win {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.fails, key)
	} else {
		l.fails[key] = kept
	}
	return kept
}

func (l *failLimiter) blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(key, time.Now())) >= l.max
}

func (l *failLimiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.recent(key, now)
	l.fails[key] = append(l.fails[key], now)
}

var inviteLimiter = newFailLimiter(inviteMaxFailures, inviteWindow)

// clientIP is the caller's address. Behind a reverse proxy (GOTHERMO_TRUST_PROXY=1)
// the proxy's X-Forwarded-For header is used; otherwise it is ignored because
// anyone could forge it.
func clientIP(r *http.Request) string {
	if os.Getenv("GOTHERMO_TRUST_PROXY") == "1" {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// checkInvite returns "" when the request may go on, or the message to send back.
func checkInvite(r *http.Request) (msg string, status int) {
	want := inviteRequired()
	if want == "" {
		return "", 0
	}
	ip := clientIP(r)
	if inviteLimiter.blocked(ip) {
		return "Too many wrong invite codes. Try again in a few minutes.", http.StatusTooManyRequests
	}
	got := r.Header.Get(inviteHeader)
	if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		inviteLimiter.fail(ip)
		if got == "" {
			return "This server needs an invite code to register. Add it under Server, Change.", http.StatusOK
		}
		return "The invite code is not correct.", http.StatusOK
	}
	return "", 0
}
