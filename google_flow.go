package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Google sign-in for the desktop app when it talks to a server.
//
// The app asks the server to start a sign-in and gets a Google address to open
// in the system browser. Google sends the person back to the server's
// /auth/google/callback, the server swaps the code for the person's profile
// (it holds the client secret, so nothing has to be installed on the
// computer), and the app picks up the finished login by polling with the
// ticket it was given. The ticket is a random secret, works once and expires.
// ---------------------------------------------------------------------------

const (
	googleFlowTTL     = 5 * time.Minute
	googleFlowMaxOpen = 1000
)

type googleTicket struct {
	state    string
	verifier string
	signUp   bool
	expires  time.Time
	done     bool
	user     *User
	errMsg   string
}

var googleFlow = struct {
	sync.Mutex
	byTicket map[string]*googleTicket
	byState  map[string]string // state -> ticket
}{byTicket: map[string]*googleTicket{}, byState: map[string]string{}}

// GoogleStartResult is what GoogleLoginStart returns.
type GoogleStartResult struct {
	URL    string `json:"url"`
	Ticket string `json:"ticket"`
}

// GooglePollResult is what GoogleLoginPoll returns.
type GooglePollResult struct {
	Status string `json:"status"` // pending, done or error
	User   *User  `json:"user,omitempty"`
	Error  string `json:"error,omitempty"`
}

// googleWebFlowConfig returns the browser client of this server and the
// address Google sends people back to.
func googleWebFlowConfig() (cfg googleConfig, redirectURI string, err error) {
	cfg = googleConfig{
		ClientID:     strings.TrimSpace(os.Getenv("GOOGLE_WEB_CLIENT_ID")),
		ClientSecret: strings.TrimSpace(os.Getenv("GOOGLE_WEB_CLIENT_SECRET")),
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("GOTHERMO_PUBLIC_URL")), "/")
	if cfg.ClientID == "" || cfg.ClientSecret == "" || base == "" {
		return cfg, "", fmt.Errorf("Google sign-in through the server is not set up")
	}
	return cfg, base + "/auth/google/callback", nil
}

// forgetGoogleTicket drops a ticket. The caller holds googleFlow's lock.
func forgetGoogleTicket(id string, t *googleTicket) {
	delete(googleFlow.byTicket, id)
	delete(googleFlow.byState, t.state)
}

// GoogleLoginStart begins a sign-in through the server and returns the Google
// address to open and the ticket to poll with. signUp is the same flag as in
// LoginWithGoogleToken (the invite code is checked on this call).
func (a *App) GoogleLoginStart(signUp bool) (GoogleStartResult, error) {
	cfg, redirectURI, err := googleWebFlowConfig()
	if err != nil {
		return GoogleStartResult{}, err
	}
	verifier, challenge, err := newPKCE()
	if err != nil {
		return GoogleStartResult{}, internalFailure("google pkce", err)
	}
	state, err := randomHex(16)
	if err != nil {
		return GoogleStartResult{}, internalFailure("google state", err)
	}
	ticket, err := randomHex(32)
	if err != nil {
		return GoogleStartResult{}, internalFailure("google ticket", err)
	}

	googleFlow.Lock()
	now := time.Now()
	for id, t := range googleFlow.byTicket {
		if now.After(t.expires) {
			forgetGoogleTicket(id, t)
		}
	}
	if len(googleFlow.byTicket) >= googleFlowMaxOpen {
		googleFlow.Unlock()
		return GoogleStartResult{}, errTryAgain
	}
	googleFlow.byTicket[ticket] = &googleTicket{state: state, verifier: verifier, signUp: signUp, expires: now.Add(googleFlowTTL)}
	googleFlow.byState[state] = ticket
	googleFlow.Unlock()

	q := url.Values{}
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("prompt", "select_account")
	return GoogleStartResult{URL: googleAuthURL + "?" + q.Encode(), Ticket: ticket}, nil
}

// GoogleLoginPoll reports how a sign-in started with GoogleLoginStart is
// doing. A finished result is handed out once.
func (a *App) GoogleLoginPoll(ticket string) (GooglePollResult, error) {
	googleFlow.Lock()
	defer googleFlow.Unlock()
	t, ok := googleFlow.byTicket[ticket]
	if !ok || time.Now().After(t.expires) {
		if ok {
			forgetGoogleTicket(ticket, t)
		}
		return GooglePollResult{Status: "error", Error: "Google sign-in expired. Please try again."}, nil
	}
	if !t.done {
		return GooglePollResult{Status: "pending"}, nil
	}
	forgetGoogleTicket(ticket, t)
	if t.errMsg != "" {
		return GooglePollResult{Status: "error", Error: t.errMsg}, nil
	}
	return GooglePollResult{Status: "done", User: t.user}, nil
}

// googleCallback is where Google sends the person back after signing in.
func googleCallback(app *App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		state := q.Get("state")

		// The state is single-use: take the ticket out before doing anything.
		googleFlow.Lock()
		id, ok := googleFlow.byState[state]
		var t *googleTicket
		if ok {
			t = googleFlow.byTicket[id]
			delete(googleFlow.byState, state)
		}
		googleFlow.Unlock()
		if t == nil || time.Now().After(t.expires) {
			w.WriteHeader(http.StatusBadRequest)
			writeCallbackPage(w, false, "This sign-in link has expired. Go back to GoThermo and try again.")
			return
		}

		finish := func(user *User, msg string) {
			googleFlow.Lock()
			t.user, t.errMsg, t.done = user, msg, true
			googleFlow.Unlock()
		}

		if q.Get("error") != "" || q.Get("code") == "" {
			finish(nil, "Google sign-in was cancelled.")
			writeCallbackPage(w, false, "Sign-in was cancelled. You can close this window.")
			return
		}

		cfg, redirectURI, err := googleWebFlowConfig()
		if err != nil {
			finish(nil, err.Error())
			writeCallbackPage(w, false, err.Error())
			return
		}
		c, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		accessToken, err := exchangeGoogleCode(c, cfg, q.Get("code"), t.verifier, redirectURI)
		var info googleUserInfo
		if err == nil {
			info, err = googleProfile(c, accessToken)
		}
		var user User
		if err == nil {
			user, err = app.finishGoogleLogin(info, t.signUp)
		}
		if err != nil {
			msg := publicError(err)
			finish(nil, msg)
			writeCallbackPage(w, false, msg)
			return
		}
		finish(&user, "")
		writeCallbackPage(w, true, "You are signed in. You can close this window and go back to GoThermo.")
	}
}
