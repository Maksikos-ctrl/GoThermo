package main

import (
	"errors"
	"strings"
	"testing"
)

func TestPublicErrorHidesTechnicalDetails(t *testing.T) {
	hidden := []string{
		`ERROR: duplicate key value violates unique constraint "users_username_key" (SQLSTATE 23505)`,
		"dial tcp 10.0.0.5:5432: connect: connection refused",
		"redis: nil",
		"context deadline exceeded",
		"internal error",
		"runtime error: invalid memory address or nil pointer dereference",
	}
	for _, msg := range hidden {
		if got := publicError(errors.New(msg)); got != errTryAgain.Error() {
			t.Errorf("%q leaked as %q", msg, got)
		}
	}
}

func TestPublicErrorKeepsMessagesWrittenForPeople(t *testing.T) {
	keep := []string{
		"incorrect email or password",
		"user with such email already exists",
		"This server needs an invite code to register.",
		"no GoThermo account for a@b.com yet, please use Sign Up first",
		"Password must be at least 4 characters",
	}
	for _, msg := range keep {
		if got := publicError(errors.New(msg)); got != msg {
			t.Errorf("%q was changed to %q", msg, got)
		}
	}
	if publicError(nil) != "" {
		t.Error("nil error must give an empty message")
	}
}

func TestRegisterFailureShowsFriendlyText(t *testing.T) {
	useStore(t, failingSave{Store: redisStore{}, err: errors.New(`ERROR: duplicate key (SQLSTATE 23505)`)})
	forgetUsers(t, "rufriendly")
	okDNS(t, func() {
		_, err := (&App{}).Register("rufriendly@gmail.com", "secret1")
		if err == nil || err.Error() != errTryAgain.Error() {
			t.Fatalf("got %v", err)
		}
		if strings.Contains(err.Error(), "SQLSTATE") {
			t.Fatal("technical text reached the user")
		}
	})
}

func TestRPCReplyHidesTechnicalErrors(t *testing.T) {
	h := newInviteEnv(t, "")
	t.Setenv("GOOGLE_CLIENT_ID", "my-client")
	// A Google outage is the network kind of failure: the person sees the calm text.
	oldI, oldT := googleUserinfoURL, googleTokenInfoURL
	googleUserinfoURL, googleTokenInfoURL = "http://127.0.0.1:1/userinfo", "http://127.0.0.1:1/tokeninfo"
	t.Cleanup(func() { googleUserinfoURL, googleTokenInfoURL = oldI, oldT })

	rep := googleTokenCall(h, "any", false, "")
	if rep.Error == "" || strings.Contains(rep.Error, "127.0.0.1") || strings.Contains(rep.Error, "dial tcp") {
		t.Fatalf("got %q", rep.Error)
	}
}
