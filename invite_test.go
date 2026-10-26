package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func registerReq(code, remote, xff string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/rpc/Register", bytes.NewBufferString(`["new@nowhere.invalid","Passw0rd!x"]`))
	if code != "" {
		req.Header.Set("X-Invite-Code", code)
	}
	if remote != "" {
		req.RemoteAddr = remote
	}
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	return req
}

func doReq(h http.Handler, req *http.Request) (int, rpcReply) {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var rep rpcReply
	_ = json.Unmarshal(rec.Body.Bytes(), &rep)
	return rec.Code, rep
}

func newInviteEnv(t *testing.T, code string) http.Handler {
	t.Helper()
	useMiniRedis(t)
	t.Setenv("GOTHERMO_INVITE_CODE", code)
	inviteLimiter = newFailLimiter(inviteMaxFailures, inviteWindow)
	return ServeRPC(&App{hub: NewHub()})
}

func TestRegistrationIsOpenWithoutInviteCode(t *testing.T) {
	h := newInviteEnv(t, "")
	_, rep := doReq(h, registerReq("", "1.2.3.4:5", ""))
	if strings.Contains(rep.Error, "invite") {
		t.Fatalf("no code is configured, yet got: %s", rep.Error)
	}
}

func TestInviteCodeRequiredToRegister(t *testing.T) {
	h := newInviteEnv(t, "s3cret")

	_, rep := doReq(h, registerReq("", "1.2.3.4:5", ""))
	if !strings.Contains(rep.Error, "needs an invite code") {
		t.Fatalf("missing code: %q", rep.Error)
	}
	_, rep = doReq(h, registerReq("wrong", "1.2.3.4:5", ""))
	if !strings.Contains(rep.Error, "not correct") {
		t.Fatalf("wrong code: %q", rep.Error)
	}
	// the right code reaches Register itself, which may fail for its own reasons
	_, rep = doReq(h, registerReq("s3cret", "1.2.3.4:5", ""))
	if strings.Contains(rep.Error, "invite") {
		t.Fatalf("right code was refused: %q", rep.Error)
	}
}

func TestInviteCodeOnlyGuardsRegister(t *testing.T) {
	h := newInviteEnv(t, "s3cret")
	req := httptest.NewRequest(http.MethodPost, "/rpc/Login", bytes.NewBufferString(`["a@b.c","x"]`))
	_, rep := doReq(h, req)
	if strings.Contains(rep.Error, "invite") {
		t.Fatalf("Login must not need a code: %q", rep.Error)
	}
}

func TestWrongInviteGuessesAreRateLimited(t *testing.T) {
	h := newInviteEnv(t, "s3cret")
	for i := 0; i < inviteMaxFailures; i++ {
		doReq(h, registerReq("guess", "9.9.9.9:1", ""))
	}
	code, rep := doReq(h, registerReq("s3cret", "9.9.9.9:2", ""))
	if code != http.StatusTooManyRequests || !strings.Contains(rep.Error, "Too many") {
		t.Fatalf("expected a block, got %d %q", code, rep.Error)
	}
	// another address is not affected
	_, rep = doReq(h, registerReq("s3cret", "8.8.8.8:1", ""))
	if strings.Contains(rep.Error, "invite") || strings.Contains(rep.Error, "Too many") {
		t.Fatalf("other client blocked: %q", rep.Error)
	}
}

func TestForwardedForOnlyTrustedBehindProxy(t *testing.T) {
	h := newInviteEnv(t, "s3cret")
	// without the proxy setting, a forged header must not dodge the limit
	for i := 0; i < inviteMaxFailures; i++ {
		doReq(h, registerReq("guess", "9.9.9.9:1", "10.0.0."+string(rune('1'+i%9))))
	}
	code, _ := doReq(h, registerReq("guess", "9.9.9.9:1", "11.11.11.11"))
	if code != http.StatusTooManyRequests {
		t.Fatalf("forged X-Forwarded-For bypassed the limit: %d", code)
	}

	// behind a proxy each real client is counted on its own
	t.Setenv("GOTHERMO_TRUST_PROXY", "1")
	inviteLimiter = newFailLimiter(inviteMaxFailures, inviteWindow)
	for i := 0; i < inviteMaxFailures; i++ {
		doReq(h, registerReq("guess", "172.18.0.2:1", "5.5.5.5"))
	}
	if code, _ := doReq(h, registerReq("guess", "172.18.0.2:1", "5.5.5.5")); code != http.StatusTooManyRequests {
		t.Fatalf("client 5.5.5.5 not blocked: %d", code)
	}
	if code, _ := doReq(h, registerReq("guess", "172.18.0.2:1", "6.6.6.6")); code == http.StatusTooManyRequests {
		t.Fatalf("client 6.6.6.6 blocked because of someone else")
	}
}

func TestCORSAllowsInviteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	ServeRPC(&App{}).ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/rpc/Register", nil))
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "X-Invite-Code") {
		t.Fatalf("preflight would block the invite header: %v", rec.Header())
	}
}
