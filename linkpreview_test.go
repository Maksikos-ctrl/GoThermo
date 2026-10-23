package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestBlockedIP(t *testing.T) {
	blocked := []string{"127.0.0.1", "127.8.8.8", "10.0.0.5", "172.16.5.5", "172.31.255.255", "192.168.1.1",
		"169.254.169.254", "0.0.0.0", "100.64.1.1", "224.0.0.1", "255.255.255.255", "::1", "::", "fe80::1",
		"fd00::1", "::ffff:127.0.0.1", "::ffff:10.1.1.1", "64:ff9b::7f00:1", "2002:7f00:1::1"}
	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "172.32.0.1", "11.0.0.1", "2606:4700:4700::1111", "2a00:1450:4001::200e", "::ffff:8.8.8.8"}
	for _, s := range blocked {
		if !blockedIP(net.ParseIP(s)) {
			t.Errorf("%s should be blocked", s)
		}
	}
	for _, s := range allowed {
		if blockedIP(net.ParseIP(s)) {
			t.Errorf("%s should be allowed", s)
		}
	}
}

func TestSSRFControl(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:80", "[::1]:443", "10.1.2.3:8080", "[::ffff:192.168.1.1]:80", "localhost:80", "not-an-ip:80", "garbage"} {
		if ssrfControl("tcp", addr, nil) == nil {
			t.Errorf("%s should be refused", addr)
		}
	}
	for _, addr := range []string{"8.8.8.8:443", "[2606:4700:4700::1111]:443"} {
		if err := ssrfControl("tcp", addr, nil); err != nil {
			t.Errorf("%s should pass: %v", addr, err)
		}
	}
}

func TestNormalizePreviewURL(t *testing.T) {
	for _, bad := range []string{"", "javascript:alert(1)", "file:///etc/passwd", "ftp://x.com/a", "http://", "//x.com", "http://user:pw@x.com/", "x.com", strings.Repeat("a", 3000)} {
		if _, err := normalizePreviewURL(bad); err == nil {
			t.Errorf("should reject %q", bad)
		}
	}
	u, err := normalizePreviewURL(" https://Example.com/a?b=1#frag ")
	if err != nil || u.String() != "https://Example.com/a?b=1" {
		t.Fatalf("%v %v", u, err)
	}
}

func solidPNG(w, h int, c color.Color) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

func decodeDataURL(t *testing.T, s string) image.Image {
	t.Helper()
	if !strings.HasPrefix(s, "data:image/jpeg;base64,") {
		t.Fatalf("not a jpeg data url: %.40s", s)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, "data:image/jpeg;base64,"))
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestMakeThumbnail(t *testing.T) {
	// wide image is shrunk to 160 wide, aspect ratio kept
	d, err := makeThumbnail(solidPNG(640, 320, color.RGBA{255, 0, 0, 255}))
	if err != nil {
		t.Fatal(err)
	}
	img := decodeDataURL(t, d)
	if b := img.Bounds(); b.Dx() != 160 || b.Dy() != 80 {
		t.Fatalf("size %v", b)
	}
	if r, g, b, _ := img.At(80, 40).RGBA(); r>>8 < 230 || g>>8 > 40 || b>>8 > 40 {
		t.Fatalf("colour changed: %d %d %d", r>>8, g>>8, b>>8)
	}

	// small images are not enlarged
	d, _ = makeThumbnail(solidPNG(40, 20, color.Black))
	if b := decodeDataURL(t, d).Bounds(); b.Dx() != 40 || b.Dy() != 20 {
		t.Fatalf("small image resized: %v", b)
	}

	// transparent pixels become white
	d, _ = makeThumbnail(solidPNG(100, 100, color.NRGBA{0, 0, 0, 0}))
	if r, g, b, _ := decodeDataURL(t, d).At(50, 50).RGBA(); r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
		t.Fatal("transparent should be white")
	}

	// GIF and JPEG inputs work
	var gb bytes.Buffer
	gif.Encode(&gb, image.NewPaletted(image.Rect(0, 0, 50, 50), color.Palette{color.White, color.Black}), nil)
	if _, err := makeThumbnail(gb.Bytes()); err != nil {
		t.Errorf("gif: %v", err)
	}
	var jb bytes.Buffer
	jpeg.Encode(&jb, image.NewRGBA(image.Rect(0, 0, 50, 50)), nil)
	if _, err := makeThumbnail(jb.Bytes()); err != nil {
		t.Errorf("jpeg: %v", err)
	}

	// not an image / too many pixels
	if _, err := makeThumbnail([]byte("<html>")); err == nil {
		t.Error("garbage must fail")
	}
	var bomb bytes.Buffer
	png.Encode(&bomb, image.NewGray(image.Rect(0, 0, 5001, 5001)))
	if _, err := makeThumbnail(bomb.Bytes()); err == nil {
		t.Error("25M+ pixel image must be refused")
	}
}

func TestParsePage(t *testing.T) {
	base, _ := normalizePreviewURL("https://www.example.com/blog/post")
	doc := `<html><head>
	<title>  Fallback   title </title>
	<meta property="og:title" content="Fish &amp; Chips: the &quot;best&quot;">
	<meta name='description' content='plain desc'>
	<meta property="og:description" content="  Line one
	   line two ">
	<META PROPERTY="og:image" CONTENT="/img/cover.png">
	</head><body>x</body></html>`
	p, img := parsePage([]byte(doc), base)
	if p.Title != `Fish & Chips: the "best"` {
		t.Errorf("title %q", p.Title)
	}
	if p.Description != "Line one line two" {
		t.Errorf("desc %q", p.Description)
	}
	if p.SiteName != "example.com" {
		t.Errorf("site %q", p.SiteName)
	}
	if img != "https://www.example.com/img/cover.png" {
		t.Errorf("img %q", img)
	}

	// no OpenGraph: falls back to <title> and name=description
	p, img = parsePage([]byte(`<title>Only title</title><meta name="description" content="d">`), base)
	if p.Title != "Only title" || p.Description != "d" || img != "" {
		t.Errorf("fallback %+v %q", p, img)
	}

	// javascript: image is dropped; long text truncated
	long := strings.Repeat("word ", 100)
	p, img = parsePage([]byte(`<meta property="og:image" content="javascript:alert(1)"><meta property="og:title" content="`+long+`">`), base)
	if img != "" || len([]rune(p.Title)) > 150 || !strings.HasSuffix(p.Title, "…") {
		t.Errorf("limits: %q %q", p.Title, img)
	}
}

func previewServer(t *testing.T, hits *int32) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hits, 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><head><meta property="og:title" content="Hello"><meta property="og:description" content="World"><meta property="og:image" content="/cover.png"></head></html>`)
	})
	mux.HandleFunc("/cover.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(solidPNG(400, 200, color.RGBA{0, 0, 255, 255}))
	})
	mux.HandleFunc("/noimage", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<title>T</title><meta property="og:image" content="/huge.png">`)
	})
	mux.HandleFunc("/huge.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(make([]byte, previewMaxImage+10))
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"title":"x"}`)
	})
	mux.HandleFunc("/loop", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/loop", http.StatusFound) })
	mux.HandleFunc("/redir", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/page", http.StatusFound) })
	mux.HandleFunc("/bigdoc", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<title>Top title</title>`)
		w.Write(bytes.Repeat([]byte("<p>filler</p>"), 100000))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestGetLinkPreview(t *testing.T) {
	mr := useMiniRedis(t)
	var hits int32
	srv := previewServer(t, &hits)
	a := &App{}
	tok, _ := CreateSession("tester")

	if _, err := a.GetLinkPreview("", "https://example.com"); err == nil {
		t.Fatal("a call without a session must be refused")
	}

	// 1. guard ON (the default): the local test server must be unreachable
	if _, err := a.GetLinkPreview(tok, srv.URL+"/page"); err == nil {
		t.Fatal("loopback target must be blocked")
	}
	if atomic.LoadInt32(&hits) != 0 {
		t.Fatal("the request must never reach the server")
	}
	// and the failure is cached
	if v, _ := mr.Get(previewCacheKey(mustURL(srv.URL + "/page"))); v != previewFailMark {
		t.Fatalf("negative cache missing: %q", v)
	}

	allowPrivateTargets = true
	t.Cleanup(func() { allowPrivateTargets = false })
	mr.FlushAll()
	tok, _ = CreateSession("tester") // the flush removed the session too

	// 2. happy path with thumbnail
	p, err := a.GetLinkPreview(tok, srv.URL+"/page")
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Hello" || p.Description != "World" || p.URL != srv.URL+"/page" {
		t.Fatalf("%+v", p)
	}
	if b := decodeDataURL(t, p.Image).Bounds(); b.Dx() != 160 || b.Dy() != 80 {
		t.Fatalf("thumb %v", b)
	}

	// 3. second call comes from the cache
	if _, err := a.GetLinkPreview(tok, srv.URL+"/page"); err != nil || atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("cache miss: hits=%d err=%v", hits, err)
	}
	if ttl := mr.TTL(previewCacheKey(mustURL(srv.URL + "/page"))); ttl <= 0 {
		t.Fatal("cache entry has no TTL")
	}

	// 4. redirect is followed
	if p, err := a.GetLinkPreview(tok, srv.URL+"/redir"); err != nil || p.Title != "Hello" {
		t.Fatalf("redirect: %+v %v", p, err)
	}

	// 5. failures
	for _, path := range []string{"/json", "/loop", "/missing"} {
		if _, err := a.GetLinkPreview(tok, srv.URL+path); err == nil {
			t.Errorf("%s should fail", path)
		}
	}
	if _, err := a.GetLinkPreview(tok, "javascript:alert(1)"); err == nil {
		t.Error("bad scheme should fail")
	}

	// 6. oversized image: card still works, without image
	p, err = a.GetLinkPreview(tok, srv.URL+"/noimage")
	if err != nil || p.Title != "T" || p.Image != "" {
		t.Fatalf("noimage: %+v %v", p, err)
	}

	// 7. huge page is cut off, head is still parsed
	if p, err := a.GetLinkPreview(tok, srv.URL+"/bigdoc"); err != nil || p.Title != "Top title" {
		t.Fatalf("bigdoc: %+v %v", p, err)
	}
}

func mustURL(s string) *url.URL {
	u, err := normalizePreviewURL(s)
	if err != nil {
		panic(err)
	}
	return u
}
