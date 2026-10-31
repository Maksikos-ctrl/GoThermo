package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ---------------------------------------------------------------------------
// Google sign-in for the desktop app.
//
// Flow (OAuth 2.0 "installed app" + PKCE, as Google requires for desktop apps):
//  1. open Google's consent page in the SYSTEM browser (embedded webviews are
//     blocked by Google),
//  2. Google redirects to a one-shot server on 127.0.0.1:<random port>,
//  3. we swap the returned code for an access token (proving possession with
//     the PKCE verifier), ask Google who the user is, and
//  4. find-or-create the GoThermo account and issue a normal session token.
//
// Google accounts are kept SEPARATE from email/password accounts (even with
// the same email address): they live under their own Redis key and are
// matched by Google's stable user id ("sub"), not by email.
// ---------------------------------------------------------------------------

const (
	googleAuthURL   = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL  = "https://oauth2.googleapis.com/token"
	googleLoginWait = 3 * time.Minute
)

var googleHTTPClient = &http.Client{Timeout: 15 * time.Second}

// Variables, not constants, so tests can point them at a local fake of Google.
var (
	googleUserinfoURL  = "https://openidconnect.googleapis.com/v1/userinfo"
	googleTokenInfoURL = "https://oauth2.googleapis.com/tokeninfo"
)

var (
	googleLoginMu    sync.Mutex // only one sign-in at a time
	googleCancelMu   sync.Mutex
	googleCancelFunc context.CancelFunc
)

type googleConfig struct {
	ClientID     string
	ClientSecret string
}

type googleClientJSON struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// loadGoogleConfig reads the OAuth client from env vars or from the
// google_oauth.json file downloaded from Google Cloud Console (kept out of git).
func loadGoogleConfig() (googleConfig, error) {
	id, secret := os.Getenv("GOOGLE_CLIENT_ID"), os.Getenv("GOOGLE_CLIENT_SECRET")
	if id != "" && secret != "" {
		return googleConfig{ClientID: id, ClientSecret: secret}, nil
	}

	paths := []string{"google_oauth.json"}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), "google_oauth.json"))
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var raw struct {
			Installed googleClientJSON `json:"installed"`
			Web       googleClientJSON `json:"web"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return googleConfig{}, fmt.Errorf("google_oauth.json is not valid JSON: %v", err)
		}
		c := raw.Installed
		if c.ClientID == "" {
			c = raw.Web
		}
		if c.ClientID == "" || c.ClientSecret == "" {
			return googleConfig{}, fmt.Errorf("google_oauth.json has no client_id / client_secret")
		}
		return googleConfig{ClientID: c.ClientID, ClientSecret: c.ClientSecret}, nil
	}

	return googleConfig{}, fmt.Errorf("Google sign-in is not configured: put google_oauth.json next to the app")
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// newPKCE returns a random code verifier and its S256 challenge.
func newPKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

type googleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

func exchangeGoogleCode(c context.Context, cfg googleConfig, code, verifier, redirectURI string) (string, error) {
	form := url.Values{}
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(c, http.MethodPost, googleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := googleHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach Google: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var tok struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &tok)
	if resp.StatusCode != http.StatusOK || tok.AccessToken == "" {
		log.Printf("❌ Google token exchange failed: %d %s", resp.StatusCode, tok.Error)
		if tok.ErrorDescription != "" {
			return "", fmt.Errorf("Google sign-in failed: %s", tok.ErrorDescription)
		}
		return "", fmt.Errorf("Google sign-in failed (%s)", resp.Status)
	}
	return tok.AccessToken, nil
}

func fetchGoogleUserInfo(c context.Context, accessToken string) (googleUserInfo, error) {
	req, err := http.NewRequestWithContext(c, http.MethodGet, googleUserinfoURL, nil)
	if err != nil {
		return googleUserInfo{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := googleHTTPClient.Do(req)
	if err != nil {
		return googleUserInfo{}, fmt.Errorf("could not reach Google: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode != http.StatusOK {
		return googleUserInfo{}, fmt.Errorf("could not read Google profile (%s)", resp.Status)
	}
	var info googleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return googleUserInfo{}, fmt.Errorf("unexpected response from Google")
	}
	return info, nil
}

var usernameCleaner = regexp.MustCompile(`[^a-zA-Z0-9.\-]+`)

// sanitizeUsername keeps usernames safe for channel names like dm_<a>_<b>
// (no spaces, no underscores).
func sanitizeUsername(s string) string {
	s = usernameCleaner.ReplaceAllString(s, "")
	if s == "" {
		s = "user"
	}
	return s
}

// userRedisKey is where a user is stored. Email/password users keep their
// existing key (user:<email>); provider users get their own key so they never
// overlap with an email account that happens to share the address. The key
// still contains '@' so the startup loader picks it up.
func userRedisKey(u *User) string {
	if u.Provider != "" && u.ProviderID != "" {
		return fmt.Sprintf("user:%s@%s", u.ProviderID, u.Provider)
	}
	return fmt.Sprintf("user:%s", u.Email)
}

// createProviderUser registers a brand new provider (e.g. Google) user with a
// username that is guaranteed not to clash with anyone else.
func (um *UserManager) createProviderUser(baseUsername, email, provider, providerID string) *User {
	um.mu.Lock()
	defer um.mu.Unlock()

	base := sanitizeUsername(baseUsername)
	username := base
	for i := 2; ; i++ {
		if _, taken := um.users[username]; !taken {
			break
		}
		username = fmt.Sprintf("%s-%d", base, i)
	}

	user := &User{
		ID:         generateID(),
		Username:   username,
		Email:      email,
		IsOnline:   true,
		Status:     "online",
		LastSeen:   time.Now().Format(time.RFC3339),
		Provider:   provider,
		ProviderID: providerID,
	}
	um.users[username] = user
	return user
}

// finishGoogleLogin completes the flow. signUp=false (Sign In tab) only lets
// existing accounts in; signUp=true (Sign Up tab) creates the account and
// refuses if it already exists.
func (a *App) finishGoogleLogin(info googleUserInfo, signUp bool) (User, error) {
	var user *User
	if username, found, err := store.UsernameByProvider("google", info.Sub); err == nil && found {
		if existing, ok := userManager.GetUser(username); ok && existing.Provider == "google" && existing.ProviderID == info.Sub {
			// Returning user: mark online (this also persists it).
			user = userManager.RegisterUser(username, existing.Email)
		}
	}

	if user != nil && signUp {
		return User{}, fmt.Errorf("this Google account is already registered, please use Sign In")
	}
	if user == nil && !signUp {
		return User{}, fmt.Errorf("no GoThermo account for %s yet, please use Sign Up first", info.Email)
	}

	if user == nil {
		local := strings.Split(info.Email, "@")[0]
		user = userManager.createProviderUser(local, info.Email, "google", info.Sub)
		if err := SaveUserToRedis(user); err != nil {
			return User{}, fmt.Errorf("error saving user: %v", err)
		}
		// (the Google link is saved together with the user)
		log.Printf("✅ New Google user: %s (%s)", user.Username, info.Email)
	}

	token, err := CreateSession(user.Username)
	if err != nil {
		return User{}, fmt.Errorf("error creating session: %v", err)
	}

	if globalHub != nil {
		globalHub.BroadcastStatusUpdate(user.Username, "online")
	}

	log.Printf("✅ User logged in with Google: %s", user.Username)
	return withToken(user, token), nil
}

func writeCallbackPage(w http.ResponseWriter, ok bool, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	title, color := "Google confirmed your account", "#2e7d32"
	if !ok {
		title, color = "Sign-in failed", "#c62828"
	}
	fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>GoThermo</title></head>
<body style="font-family:-apple-system,system-ui,sans-serif;background:#1b2636;color:#e6e9ef;display:flex;align-items:center;justify-content:center;height:100vh;margin:0">
<div style="text-align:center"><h2 style="color:%s">%s</h2><p>%s</p></div></body></html>`,
		color, html.EscapeString(title), html.EscapeString(msg))
}

// CancelGoogleLogin aborts a sign-in that is waiting for the browser.
func (a *App) CancelGoogleLogin() {
	googleCancelMu.Lock()
	defer googleCancelMu.Unlock()
	if googleCancelFunc != nil {
		googleCancelFunc()
	}
}

// LoginWithGoogle runs the whole browser sign-in and returns the logged-in
// user (with a session token), exactly like Login does. signUp tells whether
// the user pressed the button on the Sign Up tab (create account) or on the
// Sign In tab (existing accounts only).
func (a *App) LoginWithGoogle(signUp bool) (User, error) {
	accessToken, err := a.googleSignInAccessToken()
	if err != nil {
		return User{}, err
	}
	info, err := googleProfile(context.Background(), accessToken)
	if err != nil {
		return User{}, err
	}
	return a.finishGoogleLogin(info, signUp)
}

// GoogleAccessToken runs only the browser part of Google sign-in on this
// computer and hands back Google's access token. A desktop app that talks to a
// remote server uses it, then gives the token to the server's
// LoginWithGoogleToken so the server can sign the user in.
func (a *App) GoogleAccessToken() (string, error) {
	return a.googleSignInAccessToken()
}

// googleClientID is the OAuth client this server trusts tokens from.
func googleClientID() (string, error) {
	if id := os.Getenv("GOOGLE_CLIENT_ID"); id != "" {
		return id, nil
	}
	cfg, err := loadGoogleConfig()
	if err != nil {
		return "", err
	}
	return cfg.ClientID, nil
}

type googleTokenInfo struct {
	Aud       string `json:"aud"`
	ExpiresIn string `json:"expires_in"`
}

// verifyGoogleToken asks Google which app an access token was issued to and
// refuses tokens that belong to anyone else's app, so a token stolen from an
// unrelated site cannot be used to log in here.
func verifyGoogleToken(c context.Context, accessToken, wantClientID string) error {
	req, err := http.NewRequestWithContext(c, http.MethodGet, googleTokenInfoURL+"?access_token="+url.QueryEscape(accessToken), nil)
	if err != nil {
		return err
	}
	resp, err := googleHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach Google: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Google did not accept this sign-in")
	}
	var ti googleTokenInfo
	if err := json.Unmarshal(body, &ti); err != nil {
		return fmt.Errorf("unexpected response from Google")
	}
	if ti.Aud == "" || ti.Aud != wantClientID {
		return fmt.Errorf("this Google sign-in was issued for a different app")
	}
	return nil
}

// LoginWithGoogleToken is the server side of Google sign-in: the desktop app
// did the browser part and sends the access token here. The server checks the
// token was issued to its own OAuth client, reads the profile from Google
// itself (never trusting what the client says), and signs the user in.
func (a *App) LoginWithGoogleToken(accessToken string, signUp bool) (User, error) {
	if accessToken == "" {
		return User{}, fmt.Errorf("missing Google token")
	}
	clientID, err := googleClientID()
	if err != nil {
		return User{}, fmt.Errorf("Google sign-in is not configured on this server")
	}
	c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := verifyGoogleToken(c, accessToken, clientID); err != nil {
		return User{}, err
	}
	info, err := googleProfile(c, accessToken)
	if err != nil {
		return User{}, err
	}
	return a.finishGoogleLogin(info, signUp)
}

// googleSignInAccessToken runs the browser part of the sign-in on this
// computer and returns Google's access token for the user who signed in.
func (a *App) googleSignInAccessToken() (string, error) {
	if !googleLoginMu.TryLock() {
		return "", fmt.Errorf("Google sign-in is already in progress")
	}
	defer googleLoginMu.Unlock()

	cfg, err := loadGoogleConfig()
	if err != nil {
		return "", err
	}

	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	flowCtx, cancel := context.WithTimeout(parent, googleLoginWait)
	googleCancelMu.Lock()
	googleCancelFunc = cancel
	googleCancelMu.Unlock()
	defer func() {
		cancel()
		googleCancelMu.Lock()
		googleCancelFunc = nil
		googleCancelMu.Unlock()
	}()

	verifier, challenge, err := newPKCE()
	if err != nil {
		return "", fmt.Errorf("could not start sign-in: %v", err)
	}
	state, err := randomHex(16)
	if err != nil {
		return "", fmt.Errorf("could not start sign-in: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("could not start local sign-in listener: %v", err)
	}
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", listener.Addr().(*net.TCPAddr).Port)

	type callbackResult struct {
		code string
		err  error
	}
	resCh := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var res callbackResult
		switch {
		case q.Get("state") != state:
			res.err = fmt.Errorf("sign-in response did not match the request")
		case q.Get("error") != "":
			res.err = fmt.Errorf("Google sign-in was not completed (%s)", q.Get("error"))
		case q.Get("code") == "":
			res.err = fmt.Errorf("Google did not return an authorization code")
		default:
			res.code = q.Get("code")
		}

		if res.err != nil {
			writeCallbackPage(w, false, res.err.Error())
		} else {
			writeCallbackPage(w, true, "You can close this tab and return to GoThermo.")
		}
		select {
		case resCh <- res:
		default:
		}
	})

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(listener) }()
	defer func() {
		sctx, c := context.WithTimeout(context.Background(), 2*time.Second)
		defer c()
		_ = srv.Shutdown(sctx)
	}()

	params := url.Values{}
	params.Set("client_id", cfg.ClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid email profile")
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")
	params.Set("state", state)
	params.Set("prompt", "select_account")

	log.Printf("🌐 Opening browser for Google sign-in")
	wailsRuntime.BrowserOpenURL(a.ctx, googleAuthURL+"?"+params.Encode())

	var res callbackResult
	select {
	case res = <-resCh:
	case <-flowCtx.Done():
		if errors.Is(flowCtx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("Google sign-in timed out, please try again")
		}
		return "", fmt.Errorf("Google sign-in cancelled")
	}
	if res.err != nil {
		return "", res.err
	}

	accessToken, err := exchangeGoogleCode(flowCtx, cfg, res.code, verifier, redirectURI)
	if err != nil {
		return "", err
	}
	return accessToken, nil
}

// googleProfile asks Google who an access token belongs to and checks that the
// answer is usable: a stable id, and a verified email address.
func googleProfile(c context.Context, accessToken string) (googleUserInfo, error) {
	info, err := fetchGoogleUserInfo(c, accessToken)
	if err != nil {
		return googleUserInfo{}, err
	}
	if info.Sub == "" || info.Email == "" {
		return googleUserInfo{}, fmt.Errorf("Google did not return an email address")
	}
	if !info.EmailVerified {
		return googleUserInfo{}, fmt.Errorf("your Google email address is not verified")
	}
	info.Email = strings.ToLower(info.Email)
	return info, nil
}
