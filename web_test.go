package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func webFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("app/index.html", "<html>chat</html>")
	write("app/assets/app-1a2b.js", "console.log(1)")
	write("app/assets/nested/other.js", "x")
	write("secret.txt", "outside the app folder")
	return dir
}

func webServer(t *testing.T, dir string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(newServerMuxForWeb(dir))
	t.Cleanup(srv.Close)
	return srv
}

func newServerMuxForWeb(dir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/rpc/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("rpc"))
	})
	mountWeb(mux, dir)
	return mux
}

func webGet(t *testing.T, method, url string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(method, url, nil)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

func TestWebRootOpensTheChat(t *testing.T) {
	srv := webServer(t, webFixture(t))
	res, _ := webGet(t, "GET", srv.URL+"/")
	if res.StatusCode != http.StatusFound || res.Header.Get("Location") != "/app/" {
		t.Fatalf("/ -> %d %q", res.StatusCode, res.Header.Get("Location"))
	}
	res, _ = webGet(t, "GET", srv.URL+"/app")
	if res.StatusCode != http.StatusFound || res.Header.Get("Location") != "/app/" {
		t.Fatalf("/app -> %d %q", res.StatusCode, res.Header.Get("Location"))
	}
}

func TestWebServesTheApp(t *testing.T) {
	srv := webServer(t, webFixture(t))

	res, body := webGet(t, "GET", srv.URL+"/app/")
	if res.StatusCode != 200 || !strings.Contains(body, "chat") {
		t.Fatalf("index: %d %q", res.StatusCode, body)
	}
	if got := res.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("index must be revalidated, Cache-Control = %q", got)
	}
	if res.Header.Get("X-Content-Type-Options") != "nosniff" || res.Header.Get("X-Frame-Options") != "DENY" {
		t.Errorf("security headers missing: %v", res.Header)
	}

	res, body = webGet(t, "GET", srv.URL+"/app/assets/app-1a2b.js")
	if res.StatusCode != 200 || body != "console.log(1)" {
		t.Fatalf("asset: %d %q", res.StatusCode, body)
	}
	if got := res.Header.Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("hashed assets should be cached for good, Cache-Control = %q", got)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Errorf("Content-Type = %q", ct)
	}

	res, _ = webGet(t, "HEAD", srv.URL+"/app/assets/app-1a2b.js")
	if res.StatusCode != 200 {
		t.Errorf("HEAD: %d", res.StatusCode)
	}
}

func TestWebStaysInsideTheAppFolder(t *testing.T) {
	srv := webServer(t, webFixture(t))
	for _, p := range []string{
		"/app/../secret.txt",
		"/app/%2e%2e/secret.txt",
		"/app/..%2fsecret.txt",
		"/app/assets/../../secret.txt",
		"/secret.txt",
	} {
		res, body := webGet(t, "GET", srv.URL+p)
		if strings.Contains(body, "outside the app folder") {
			t.Errorf("%s leaked a file outside the app folder", p)
		}
		if res.StatusCode == 200 {
			t.Errorf("%s -> 200", p)
		}
	}
}

func TestWebNoListingsAndOnlyReads(t *testing.T) {
	srv := webServer(t, webFixture(t))

	for _, p := range []string{"/app/assets/", "/app/assets", "/app/assets/nested/", "/app/missing.js", "/nope"} {
		res, body := webGet(t, "GET", srv.URL+p)
		if res.StatusCode != http.StatusNotFound || strings.Contains(body, "app-1a2b.js") {
			t.Errorf("%s -> %d, want 404 and no listing: %q", p, res.StatusCode, body)
		}
	}

	res, _ := webGet(t, "POST", srv.URL+"/app/")
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST /app/ -> %d", res.StatusCode)
	}
}

func TestWebLeavesTheAPIAlone(t *testing.T) {
	srv := webServer(t, webFixture(t))
	if _, body := webGet(t, "GET", srv.URL+"/healthz"); body != `{"ok":true}` {
		t.Errorf("healthz: %q", body)
	}
	if _, body := webGet(t, "POST", srv.URL+"/rpc/Login"); body != "rpc" {
		t.Errorf("rpc: %q", body)
	}
}

func TestWebOffWithoutAFolder(t *testing.T) {
	for _, dir := range []string{"", filepath.Join(t.TempDir(), "missing"), t.TempDir()} {
		mux := http.NewServeMux()
		if mountWeb(mux, dir) {
			t.Errorf("mounted %q", dir)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET / with web off (%q) -> %d", dir, rec.Code)
		}
	}
}

func TestServerMuxMountsTheWebFolderFromTheEnvironment(t *testing.T) {
	dir := webFixture(t)
	t.Setenv("GOTHERMO_WEB_DIR", dir)
	srv := httptest.NewServer(newServerMux(&App{}, NewHub()))
	defer srv.Close()
	if res, body := webGet(t, "GET", srv.URL+"/app/"); res.StatusCode != 200 || !strings.Contains(body, "chat") {
		t.Fatalf("app: %d %q", res.StatusCode, body)
	}
	if res, body := webGet(t, "GET", srv.URL+"/healthz"); res.StatusCode != 200 || !strings.Contains(body, `"app":"gothermo"`) {
		t.Fatalf("healthz: %d %q", res.StatusCode, body)
	}
}
