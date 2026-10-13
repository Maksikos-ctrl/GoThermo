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
	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserinfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
	googleLoginWait   = 3 * time.Minute
)

var googleHTTPClient = &http.Client{Timeout: 15 * time.Second}

var (
	googleLoginMu    sync.Mutex
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

func sanitizeUsername(s string) string {
	s = usernameCleaner.ReplaceAllString(s, "")
	if s == "" {
		s = "user"
	}
	return s
}

func userRedisKey(u *User) string {
	if u.Provider != "" && u.ProviderID != "" {
		return fmt.Sprintf("user:%s@%s", u.ProviderID, u.Provider)
	}
	return fmt.Sprintf("user:%s", u.Email)
}

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

func (a *App) finishGoogleLogin(info googleUserInfo, signUp bool) (User, error) {
	mapKey := "google:" + info.Sub

	var user *User
	if username, err := redisClient.Get(ctx, mapKey).Result(); err == nil {
		if existing, ok := userManager.GetUser(username); ok && existing.Provider == "google" && existing.ProviderID == info.Sub {

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
		if err := redisClient.Set(ctx, mapKey, user.Username, 0).Err(); err != nil {
			return User{}, fmt.Errorf("error saving Google link: %v", err)
		}
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

func (a *App) CancelGoogleLogin() {
	googleCancelMu.Lock()
	defer googleCancelMu.Unlock()
	if googleCancelFunc != nil {
		googleCancelFunc()
	}
}

func (a *App) LoginWithGoogle(signUp bool) (User, error) {
	if !googleLoginMu.TryLock() {
		return User{}, fmt.Errorf("Google sign-in is already in progress")
	}
	defer googleLoginMu.Unlock()

	cfg, err := loadGoogleConfig()
	if err != nil {
		return User{}, err
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
		return User{}, fmt.Errorf("could not start sign-in: %v", err)
	}
	state, err := randomHex(16)
	if err != nil {
		return User{}, fmt.Errorf("could not start sign-in: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return User{}, fmt.Errorf("could not start local sign-in listener: %v", err)
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
			return User{}, fmt.Errorf("Google sign-in timed out, please try again")
		}
		return User{}, fmt.Errorf("Google sign-in cancelled")
	}
	if res.err != nil {
		return User{}, res.err
	}

	accessToken, err := exchangeGoogleCode(flowCtx, cfg, res.code, verifier, redirectURI)
	if err != nil {
		return User{}, err
	}

	info, err := fetchGoogleUserInfo(flowCtx, accessToken)
	if err != nil {
		return User{}, err
	}
	if info.Sub == "" || info.Email == "" {
		return User{}, fmt.Errorf("Google did not return an email address")
	}
	if !info.EmailVerified {
		return User{}, fmt.Errorf("your Google email address is not verified")
	}
	info.Email = strings.ToLower(info.Email)

	return a.finishGoogleLogin(info, signUp)
}
