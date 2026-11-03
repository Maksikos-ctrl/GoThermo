package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// fakeGoogleFlow stands in for Google's token and userinfo endpoints.
func fakeGoogleFlow(t *testing.T, sub, email string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("code") != "good-code" || r.Form.Get("client_secret") != "web-secret" || r.Form.Get("code_verifier") == "" {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		w.Write([]byte(`{"access_token":"good-token"}`))
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"sub":"` + sub + `","email":"` + email + `","email_verified":true}`))
	})
	srv := httptest.NewServer(mux)
	oldT, oldU := googleTokenURL, googleUserinfoURL
	googleTokenURL, googleUserinfoURL = srv.URL+"/token", srv.URL+"/userinfo"
	t.Cleanup(func() {
		googleTokenURL, googleUserinfoURL = oldT, oldU
		srv.Close()
	})
}

func setupGoogleFlow(t *testing.T) (http.Handler, http.Handler) {
	t.Helper()
	t.Setenv("GOOGLE_WEB_CLIENT_ID", "web-client")
	t.Setenv("GOOGLE_WEB_CLIENT_SECRET", "web-secret")
	t.Setenv("GOTHERMO_PUBLIC_URL", "https://chat.example.com/")
	rpcH := newInviteEnv(t, "")
	app := &App{hub: NewHub()}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/google/callback", googleCallback(app))
	return rpcH, mux
}

func flowStart(t *testing.T, h http.Handler, signUp bool, invite string) (GoogleStartResult, rpcReply) {
	t.Helper()
	body := `[false]`
	if signUp {
		body = `[true]`
	}
	req := httptest.NewRequest(http.MethodPost, "/rpc/GoogleLoginStart", strings.NewReader(body))
	if invite != "" {
		req.Header.Set("X-Invite-Code", invite)
	}
	_, rep := doReq(h, req)
	var st GoogleStartResult
	if rep.Error == "" {
		raw, _ := json.Marshal(rep.Result)
		_ = json.Unmarshal(raw, &st)
	}
	return st, rep
}

func flowPoll(t *testing.T, h http.Handler, ticket string) GooglePollResult {
	t.Helper()
	_, rep := rpcCall(t, h, "GoogleLoginPoll", `["`+ticket+`"]`)
	if rep.Error != "" {
		t.Fatalf("poll: %s", rep.Error)
	}
	var p GooglePollResult
	raw, _ := json.Marshal(rep.Result)
	_ = json.Unmarshal(raw, &p)
	return p
}

func callbackGet(mux http.Handler, query string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/google/callback?"+query, nil))
	return rec
}

func stateOf(t *testing.T, startURL string) string {
	t.Helper()
	u, err := url.Parse(startURL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query().Get("state")
}

func TestGoogleFlowSignsTheAppIn(t *testing.T) {
	h, mux := setupGoogleFlow(t)
	fakeGoogleFlow(t, "sub-flow-1", "flow.one@example.com")

	st, rep := flowStart(t, h, true, "")
	if rep.Error != "" {
		t.Fatal(rep.Error)
	}
	u, _ := url.Parse(st.URL)
	q := u.Query()
	if q.Get("client_id") != "web-client" || q.Get("redirect_uri") != "https://chat.example.com/auth/google/callback" ||
		q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("state") == "" {
		t.Fatalf("bad Google address: %s", st.URL)
	}

	if p := flowPoll(t, h, st.Ticket); p.Status != "pending" {
		t.Fatalf("before the callback: %+v", p)
	}

	rec := callbackGet(mux, "code=good-code&state="+q.Get("state"))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "signed in") {
		t.Fatalf("callback: %d %s", rec.Code, rec.Body.String())
	}

	p := flowPoll(t, h, st.Ticket)
	if p.Status != "done" || p.User == nil || p.User.Token == "" || p.User.Email != "flow.one@example.com" {
		t.Fatalf("after the callback: %+v", p)
	}
	if name, ok := GetSessionUsername(p.User.Token); !ok || name != p.User.Username {
		t.Fatal("the login did not open a session")
	}

	// the result is handed out once
	if again := flowPoll(t, h, st.Ticket); again.Status != "error" {
		t.Fatalf("second poll: %+v", again)
	}
}

func TestGoogleFlowStateWorksOnce(t *testing.T) {
	h, mux := setupGoogleFlow(t)
	fakeGoogleFlow(t, "sub-flow-2", "flow.two@example.com")
	st, _ := flowStart(t, h, true, "")
	state := stateOf(t, st.URL)

	if rec := callbackGet(mux, "code=good-code&state="+state); rec.Code != 200 {
		t.Fatalf("first: %d", rec.Code)
	}
	if rec := callbackGet(mux, "code=good-code&state="+state); rec.Code != http.StatusBadRequest {
		t.Fatalf("replay must be refused, got %d", rec.Code)
	}
	if rec := callbackGet(mux, "code=good-code&state=made-up"); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown state must be refused, got %d", rec.Code)
	}
}

func TestGoogleFlowCancelled(t *testing.T) {
	h, mux := setupGoogleFlow(t)
	st, _ := flowStart(t, h, false, "")
	callbackGet(mux, "error=access_denied&state="+stateOf(t, st.URL))
	p := flowPoll(t, h, st.Ticket)
	if p.Status != "error" || !strings.Contains(p.Error, "cancelled") {
		t.Fatalf("got %+v", p)
	}
}

func TestGoogleFlowSignInNeedsAnAccount(t *testing.T) {
	h, mux := setupGoogleFlow(t)
	fakeGoogleFlow(t, "sub-flow-3", "nobody@example.com")
	st, _ := flowStart(t, h, false, "")
	callbackGet(mux, "code=good-code&state="+stateOf(t, st.URL))
	p := flowPoll(t, h, st.Ticket)
	if p.Status != "error" || !strings.Contains(p.Error, "Sign Up first") {
		t.Fatalf("got %+v", p)
	}
}

func TestGoogleFlowBadCodeShowsFriendlyError(t *testing.T) {
	h, mux := setupGoogleFlow(t)
	fakeGoogleFlow(t, "sub-flow-4", "bad@example.com")
	st, _ := flowStart(t, h, true, "")
	rec := callbackGet(mux, "code=wrong&state="+stateOf(t, st.URL))
	if strings.Contains(rec.Body.String(), "invalid_grant") {
		t.Fatalf("Google's raw error leaked: %s", rec.Body.String())
	}
	if p := flowPoll(t, h, st.Ticket); p.Status != "error" || p.Error == "" {
		t.Fatalf("got %+v", p)
	}
}

func TestGoogleFlowNeedsServerSetup(t *testing.T) {
	t.Setenv("GOOGLE_WEB_CLIENT_ID", "")
	t.Setenv("GOOGLE_WEB_CLIENT_SECRET", "")
	t.Setenv("GOTHERMO_PUBLIC_URL", "")
	h := newInviteEnv(t, "")
	if _, rep := flowStart(t, h, false, ""); !strings.Contains(rep.Error, "not set up") {
		t.Fatalf("got %q", rep.Error)
	}
}

func TestGoogleFlowSignUpNeedsTheInviteCode(t *testing.T) {
	t.Setenv("GOOGLE_WEB_CLIENT_ID", "web-client")
	t.Setenv("GOOGLE_WEB_CLIENT_SECRET", "web-secret")
	t.Setenv("GOTHERMO_PUBLIC_URL", "https://chat.example.com")
	h := newInviteEnv(t, "sekret")

	if _, rep := flowStart(t, h, true, ""); !strings.Contains(rep.Error, "needs an invite code") {
		t.Fatalf("sign-up without a code: %q", rep.Error)
	}
	if _, rep := flowStart(t, h, true, "sekret"); rep.Error != "" {
		t.Fatalf("sign-up with the code: %q", rep.Error)
	}
	// signing in to an existing account needs no code
	if _, rep := flowStart(t, h, false, ""); rep.Error != "" {
		t.Fatalf("sign-in: %q", rep.Error)
	}
}
