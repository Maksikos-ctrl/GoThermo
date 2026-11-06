package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchGifs(t *testing.T) {
	mr := useMiniRedis(t)
	tok := mkUser(t, mr, "ann")
	a := &App{hub: NewHub()}

	var gotQ, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQ = r.URL.Query().Get("q")
		gotPath = r.URL.Path
		w.Write([]byte(`{"data":[
		 {"id":"1","title":"ok","images":{"fixed_height":{"url":"https://media1.giphy.com/media/x/200.gif?cid=abc"},"fixed_height_small":{"url":"https://media1.giphy.com/media/x/100.gif?cid=abc"}}},
		 {"id":"2","title":"evil","images":{"fixed_height":{"url":"https://evil.example/a.gif"},"fixed_height_small":{"url":"https://evil.example/a.gif"}}},
		 {"id":"3","title":"http","images":{"fixed_height":{"url":"http://media.giphy.com/a.gif"}}}]}`))
	}))
	defer srv.Close()
	old := giphyBase
	giphyBase = srv.URL
	defer func() { giphyBase = old }()

	t.Setenv("GIPHY_API_KEY", "")
	if a.GifsEnabled(tok) {
		t.Fatal("enabled without key")
	}
	if _, err := a.SearchGifs(tok, "cat"); err == nil {
		t.Fatal("search without key should fail")
	}

	t.Setenv("GIPHY_API_KEY", "k")
	if !a.GifsEnabled(tok) || a.GifsEnabled("bogus") {
		t.Fatal("enabled flag wrong")
	}
	gifs, err := a.SearchGifs(tok, " cat ")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/gifs/search" || gotQ != "cat" {
		t.Fatalf("path=%s q=%q", gotPath, gotQ)
	}
	if len(gifs) != 1 || gifs[0].URL != "https://media1.giphy.com/media/x/200.gif" || gifs[0].Preview != "https://media1.giphy.com/media/x/100.gif" {
		t.Fatalf("got %+v", gifs)
	}
	if _, err := a.SearchGifs(tok, ""); err != nil || gotPath != "/v1/gifs/trending" {
		t.Fatalf("trending: %v %s", err, gotPath)
	}
	if _, err := a.SearchGifs("bogus", "x"); err == nil {
		t.Fatal("needs login")
	}
	for i := 0; i < maxGifsPerMinute; i++ {
		a.SearchGifs(tok, "x")
	}
	if _, err := a.SearchGifs(tok, "x"); err == nil {
		t.Fatal("rate limit not applied")
	}
}

func TestValidGifURL(t *testing.T) {
	for u, want := range map[string]bool{
		"https://media.giphy.com/a.gif": true, "https://giphy.com/x": true,
		"http://media.giphy.com/a.gif": false, "https://evilgiphy.com/a": false,
		"https://giphy.com.evil.io/a": false, "https://u@media.giphy.com/a": false, "": false,
	} {
		if validGifURL(u) != want {
			t.Errorf("%q want %v", u, want)
		}
	}
}
