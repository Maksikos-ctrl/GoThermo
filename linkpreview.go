package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"syscall"
	"time"
)

type LinkPreview struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	SiteName    string `json:"siteName"`
	Image       string `json:"image"`
}

const (
	previewTTL       = 24 * time.Hour
	previewFailTTL   = time.Hour
	previewMaxHTML   = 256 << 10
	previewMaxImage  = 2 << 20
	previewThumbW    = 160
	previewMaxPixels = 25_000_000
	previewTimeout   = 6 * time.Second
	previewFailMark  = "-"
)

var previewSlots = make(chan struct{}, 4)

var allowPrivateTargets = false

// ---------- SSRF guard ----------
//
// The URL comes from a chat message, i.e. from someone else, and the request is
// made from this computer. Without a guard a link like http://192.168.0.1/ or
// http://127.0.0.1:6379/ would make the app poke at the local network.
// The check runs when the connection is actually opened (on the resolved IP),
// so it also covers redirects and DNS tricks.

func mustCIDRs(list ...string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(list))
	for _, c := range list {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			panic(err)
		}
		nets = append(nets, n)
	}
	return nets
}

var blockedNets = mustCIDRs(
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
	"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.168.0.0/16", "198.18.0.0/15",
	"198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
	"::/96", "64:ff9b::/96", "100::/64", "2001::/32", "2001:db8::/32",
	"2002::/16", "fc00::/7", "fe80::/10", "ff00::/8",
)

func blockedIP(ip net.IP) bool {
	// ::ffff:127.0.0.1 is checked as 127.0.0.1. (Do not add ::ffff:0:0/96 to
	// the list above: Go would read it as "every IPv4 address".)
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	for _, n := range blockedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func ssrfControl(network, address string, _ syscall.RawConn) error {
	if allowPrivateTargets {
		return nil
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || blockedIP(ip) {
		return fmt.Errorf("blocked address %s", host)
	}
	return nil
}

func newPreviewClient() *http.Client {
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, Control: ssrfControl}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		DisableKeepAlives:     true,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   previewTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 4 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return errors.New("redirect to unsupported scheme")
			}
			return nil
		},
	}
}

func normalizePreviewURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 2048 {
		return nil, errors.New("invalid link")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return nil, errors.New("invalid link")
	}
	u.Fragment = ""
	return u, nil
}

func fetch(client *http.Client, u *url.URL, accept string, max int64, strict bool) ([]byte, *http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; GoThermoLinkPreview/1.0)")
	req.Header.Set("Accept", accept)

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	if strict && resp.ContentLength > max {
		return nil, nil, errors.New("too large")
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, nil, err
	}
	if int64(len(body)) > max {
		if strict {
			return nil, nil, errors.New("too large")
		}
		body = body[:max]
	}
	return body, resp, nil
}

var (
	titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	metaRe  = regexp.MustCompile(`(?is)<meta\b[^>]*>`)
	attrRe  = regexp.MustCompile(`(?is)([a-zA-Z:_-]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)
)

func cleanText(s string, max int) string {
	s = strings.Join(strings.Fields(html.UnescapeString(s)), " ")
	if r := []rune(s); len(r) > max {
		s = string(r[:max-1]) + "…"
	}
	return s
}

func parsePage(doc []byte, base *url.URL) (p LinkPreview, imageURL string) {
	text := strings.ToValidUTF8(string(doc), "")

	meta := map[string]string{}
	for _, tag := range metaRe.FindAllString(text, -1) {
		attrs := map[string]string{}
		for _, m := range attrRe.FindAllStringSubmatch(tag, -1) {
			val := m[2]
			if val == "" {
				val = m[3]
			}
			if val == "" {
				val = m[4]
			}
			attrs[strings.ToLower(m[1])] = val
		}
		key := attrs["property"]
		if key == "" {
			key = attrs["name"]
		}
		if key = strings.ToLower(key); key != "" && attrs["content"] != "" {
			if _, seen := meta[key]; !seen {
				meta[key] = attrs["content"]
			}
		}
	}

	first := func(keys ...string) string {
		for _, k := range keys {
			if v := cleanText(meta[k], 1000); v != "" {
				return v
			}
		}
		return ""
	}

	title := first("og:title", "twitter:title")
	if title == "" {
		if m := titleRe.FindStringSubmatch(text); m != nil {
			title = cleanText(m[1], 1000)
		}
	}
	p.Title = cleanText(title, 150)
	p.Description = cleanText(first("og:description", "twitter:description", "description"), 300)
	p.SiteName = cleanText(first("og:site_name"), 60)
	if p.SiteName == "" {
		p.SiteName = strings.TrimPrefix(base.Hostname(), "www.")
	}

	if img := first("og:image", "og:image:url", "twitter:image"); img != "" {
		if ref, err := url.Parse(img); err == nil {
			abs := base.ResolveReference(ref)
			if abs.Scheme == "http" || abs.Scheme == "https" {
				imageURL = abs.String()
			}
		}
	}
	return p, imageURL
}

func makeThumbnail(data []byte) (string, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > previewMaxPixels {
		return "", errors.New("image dimensions not supported")
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	w := previewThumbW
	if cfg.Width < w {
		w = cfg.Width
	}
	h := cfg.Height * w / cfg.Width
	if h < 1 {
		h = 1
	}

	// White background (JPEG has no transparency), then box-average the pixels.
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	b := src.Bounds()
	for y := 0; y < h; y++ {
		sy0 := b.Min.Y + y*b.Dy()/h
		sy1 := b.Min.Y + (y+1)*b.Dy()/h
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for x := 0; x < w; x++ {
			sx0 := b.Min.X + x*b.Dx()/w
			sx1 := b.Min.X + (x+1)*b.Dx()/w
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			var r, g, bl, n uint64
			for sy := sy0; sy < sy1; sy++ {
				for sx := sx0; sx < sx1; sx++ {
					pr, pg, pb, pa := src.At(sx, sy).RGBA()
					// composite over white using the alpha
					r += uint64(pr) + uint64(0xffff-pa)
					g += uint64(pg) + uint64(0xffff-pa)
					bl += uint64(pb) + uint64(0xffff-pa)
					n++
				}
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(r / n >> 8), uint8(g / n >> 8), uint8(bl / n >> 8), 255})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 75}); err != nil {
		return "", err
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func previewCacheKey(u *url.URL) string {
	sum := sha256.Sum256([]byte(u.String()))
	return "linkpreview:" + hex.EncodeToString(sum[:])
}

func (a *App) GetLinkPreview(rawURL string) (LinkPreview, error) {
	u, err := normalizePreviewURL(rawURL)
	if err != nil {
		return LinkPreview{}, err
	}

	key := previewCacheKey(u)
	if cached, err := redisClient.Get(ctx, key).Result(); err == nil {
		if cached == previewFailMark {
			return LinkPreview{}, errors.New("no preview available")
		}
		var p LinkPreview
		if json.Unmarshal([]byte(cached), &p) == nil {
			return p, nil
		}
	}

	select {
	case previewSlots <- struct{}{}:
		defer func() { <-previewSlots }()
	case <-time.After(10 * time.Second):
		return LinkPreview{}, errors.New("busy, try again later")
	}

	p, err := buildPreview(newPreviewClient(), u)
	if err != nil {
		log.Printf("🔗 No preview for %s: %v", u.Hostname(), err)
		redisClient.Set(ctx, key, previewFailMark, previewFailTTL)
		return LinkPreview{}, errors.New("no preview available")
	}

	if data, err := json.Marshal(p); err == nil {
		redisClient.Set(ctx, key, data, previewTTL)
	}
	return p, nil
}

func buildPreview(client *http.Client, u *url.URL) (LinkPreview, error) {
	body, resp, err := fetch(client, u, "text/html,application/xhtml+xml", previewMaxHTML, false)
	if err != nil {
		return LinkPreview{}, err
	}
	if ct := strings.ToLower(resp.Header.Get("Content-Type")); !strings.HasPrefix(ct, "text/html") && !strings.HasPrefix(ct, "application/xhtml") {
		return LinkPreview{}, errors.New("not a web page")
	}

	final := resp.Request.URL
	p, imageURL := parsePage(body, final)
	if p.Title == "" && p.Description == "" {
		return LinkPreview{}, errors.New("page has no title")
	}
	p.URL = u.String()

	if imageURL != "" {
		if iu, err := normalizePreviewURL(imageURL); err == nil {
			if data, _, err := fetch(client, iu, "image/png,image/jpeg,image/gif", previewMaxImage, true); err == nil {
				if thumb, err := makeThumbnail(data); err == nil {
					p.Image = thumb
				}
			}
		}
	}
	return p, nil
}
