package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// GIF search goes through the server so the Giphy key never reaches a
// browser. Without GIPHY_API_KEY the feature is simply off.

const (
	maxGifQueryRunes = 80
	maxGifsPerMinute = 30
	gifLimit         = 24
)

var (
	giphyBase       = "https://api.giphy.com"
	giphyHTTPClient = &http.Client{Timeout: 8 * time.Second}
)

// Gif is one search result: a small preview and the URL that gets posted.
type Gif struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Preview string `json:"preview"`
}

func giphyKey() string { return strings.TrimSpace(os.Getenv("GIPHY_API_KEY")) }

// validGifURL only lets Giphy's own media hosts through, over https.
func validGifURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return h == "giphy.com" || strings.HasSuffix(h, ".giphy.com")
}

// GifsEnabled tells the app whether to show the GIF button at all.
func (a *App) GifsEnabled(token string) bool {
	if _, err := sessionUser(token); err != nil {
		return false
	}
	return giphyKey() != ""
}

// SearchGifs returns trending GIFs for an empty query, matches otherwise.
func (a *App) SearchGifs(token, query string) ([]Gif, error) {
	user, err := sessionUser(token)
	if err != nil {
		return nil, err
	}
	key := giphyKey()
	if key == "" {
		return nil, fmt.Errorf("GIFs are not set up on this server")
	}
	query = strings.TrimSpace(query)
	if r := []rune(query); len(r) > maxGifQueryRunes {
		query = string(r[:maxGifQueryRunes])
	}

	rateKey := "gifrate:" + strings.ToLower(user)
	n, err := redisClient.Incr(ctx, rateKey).Result()
	if err != nil {
		return nil, internalFailure("gif rate", err)
	}
	if n == 1 {
		redisClient.Expire(ctx, rateKey, time.Minute)
	}
	if n > maxGifsPerMinute {
		return nil, fmt.Errorf("too many searches, slow down a little")
	}

	q := url.Values{}
	q.Set("api_key", key)
	q.Set("limit", fmt.Sprint(gifLimit))
	q.Set("rating", "pg-13")
	endpoint := "/v1/gifs/trending"
	if query != "" {
		endpoint = "/v1/gifs/search"
		q.Set("q", query)
	}
	resp, err := giphyHTTPClient.Get(giphyBase + endpoint + "?" + q.Encode())
	if err != nil {
		return nil, fmt.Errorf("GIF search is unavailable right now")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GIF search is unavailable right now")
	}
	var body struct {
		Data []struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Images map[string]struct {
				URL string `json:"url"`
			} `json:"images"`
		} `json:"data"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 4<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("GIF search is unavailable right now")
	}
	out := make([]Gif, 0, len(body.Data))
	for _, d := range body.Data {
		full := d.Images["fixed_height"].URL
		small := d.Images["fixed_height_small"].URL
		if small == "" {
			small = full
		}
		if !validGifURL(full) || !validGifURL(small) {
			continue
		}
		// Drop tracking query strings; the bare media URL is what gets posted.
		out = append(out, Gif{ID: d.ID, Title: d.Title, URL: stripQuery(full), Preview: stripQuery(small)})
	}
	return out, nil
}

func stripQuery(raw string) string {
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		return raw[:i]
	}
	return raw
}
