package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeGoogle stands in for Google's tokeninfo and userinfo endpoints.
func fakeGoogle(t *testing.T, aud, sub, email string, verified bool) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/tokeninfo", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("access_token") != "good-token" {
			http.Error(w, `{"error":"invalid_token"}`, http.StatusBadRequest)
			return
		}
		w.Write([]byte(`{"aud":"` + aud + `","expires_in":"3000"}`))
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good-token" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		v := "false"
		if verified {
			v = "true"
		}
		w.Write([]byte(`{"sub":"` + sub + `","email":"` + email + `","email_verified":` + v + `}`))
	})
	srv := httptest.NewServer(mux)
	oldI, oldT := googleUserinfoURL, googleTokenInfoURL
	googleUserinfoURL, googleTokenInfoURL = srv.URL+"/userinfo", srv.URL+"/tokeninfo"
	t.Cleanup(func() {
		googleUserinfoURL, googleTokenInfoURL = oldI, oldT
		srv.Close()
	})
}

func googleTokenCall(h http.Handler, token string, signUp bool, invite string) rpcReply {
	flag := "false"
	if signUp {
		flag = "true"
	}
	req := httptest.NewRequest(http.MethodPost, "/rpc/LoginWithGoogleToken", bytes.NewBufferString(`["`+token+`",`+flag+`]`))
	if invite != "" {
		req.Header.Set("X-Invite-Code", invite)
	}
	_, rep := doReq(h, req)
	return rep
}

func TestGoogleTokenSignUpThenSignIn(t *testing.T) {
	h := newInviteEnv(t, "")
	t.Setenv("GOOGLE_CLIENT_ID", "my-client")
	fakeGoogle(t, "my-client", "sub-1", "Ann.Lee@Example.com", true)

	rep := googleTokenCall(h, "good-token", true, "")
	if rep.Error != "" {
		t.Fatalf("sign up: %s", rep.Error)
	}
	u, _ := rep.Result.(map[string]interface{})
	if u["token"] == nil || u["token"] == "" {
		t.Fatalf("no session token in %v", rep.Result)
	}
	if u["email"] != "ann.lee@example.com" {
		t.Fatalf("email not normalised: %v", u["email"])
	}

	// the same account cannot sign up twice, but can sign in
	if rep := googleTokenCall(h, "good-token", true, ""); !strings.Contains(rep.Error, "already registered") {
		t.Fatalf("second sign up: %q", rep.Error)
	}
	if rep := googleTokenCall(h, "good-token", false, ""); rep.Error != "" {
		t.Fatalf("sign in: %s", rep.Error)
	}
}

func TestGoogleTokenSignInNeedsAccount(t *testing.T) {
	h := newInviteEnv(t, "")
	t.Setenv("GOOGLE_CLIENT_ID", "my-client")
	fakeGoogle(t, "my-client", "sub-2", "new@example.com", true)
	if rep := googleTokenCall(h, "good-token", false, ""); !strings.Contains(rep.Error, "Sign Up first") {
		t.Fatalf("got %q", rep.Error)
	}
}

func TestGoogleTokenFromAnotherAppIsRefused(t *testing.T) {
	h := newInviteEnv(t, "")
	t.Setenv("GOOGLE_CLIENT_ID", "my-client")
	fakeGoogle(t, "someone-elses-client", "sub-3", "x@example.com", true)
	rep := googleTokenCall(h, "good-token", true, "")
	if !strings.Contains(rep.Error, "different app") {
		t.Fatalf("got %q", rep.Error)
	}
	if rep.Result != nil {
		t.Fatalf("a user was returned: %v", rep.Result)
	}
}

func TestGoogleTokenRejectsBadAndUnverified(t *testing.T) {
	h := newInviteEnv(t, "")
	t.Setenv("GOOGLE_CLIENT_ID", "my-client")
	fakeGoogle(t, "my-client", "sub-4", "u@example.com", false)
	if rep := googleTokenCall(h, "bad-token", true, ""); rep.Error == "" {
		t.Fatal("a token Google rejects was accepted")
	}
	if rep := googleTokenCall(h, "good-token", true, ""); !strings.Contains(rep.Error, "not verified") {
		t.Fatalf("unverified email: %q", rep.Error)
	}
	if rep := googleTokenCall(h, "", true, ""); !strings.Contains(rep.Error, "missing") {
		t.Fatalf("empty token: %q", rep.Error)
	}
}

func TestGoogleTokenNeedsServerConfig(t *testing.T) {
	h := newInviteEnv(t, "")
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	t.Chdir(t.TempDir()) // no google_oauth.json here
	fakeGoogle(t, "my-client", "sub-5", "u@example.com", true)
	if rep := googleTokenCall(h, "good-token", true, ""); !strings.Contains(rep.Error, "not configured") {
		t.Fatalf("got %q", rep.Error)
	}
}

func TestInviteCodeGuardsGoogleSignUpOnly(t *testing.T) {
	h := newInviteEnv(t, "s3cret")
	t.Setenv("GOOGLE_CLIENT_ID", "my-client")
	fakeGoogle(t, "my-client", "sub-6", "g@example.com", true)

	if rep := googleTokenCall(h, "good-token", true, ""); !strings.Contains(rep.Error, "needs an invite code") {
		t.Fatalf("sign up without code: %q", rep.Error)
	}
	if rep := googleTokenCall(h, "good-token", true, "s3cret"); rep.Error != "" {
		t.Fatalf("sign up with code: %s", rep.Error)
	}
	// signing in to an existing account needs no code
	if rep := googleTokenCall(h, "good-token", false, ""); rep.Error != "" {
		t.Fatalf("sign in: %s", rep.Error)
	}
}

func TestCreatesAccount(t *testing.T) {
	raw := func(s ...string) []json.RawMessage {
		var out []json.RawMessage
		for _, x := range s {
			out = append(out, json.RawMessage(x))
		}
		return out
	}
	cases := []struct {
		name string
		args []json.RawMessage
		want bool
	}{
		{"Register", raw(`"a"`, `"b"`), true},
		{"Login", raw(`"a"`, `"b"`), false},
		{"LoginWithGoogleToken", raw(`"t"`, `true`), true},
		{"LoginWithGoogleToken", raw(`"t"`, `false`), false},
		{"LoginWithGoogleToken", raw(`"t"`), true},
		{"LoginWithGoogleToken", raw(`"t"`, `"false"`), true},
	}
	for _, c := range cases {
		if got := createsAccount(c.name, c.args); got != c.want {
			t.Errorf("%s %v: got %v", c.name, c.args, got)
		}
	}
}
