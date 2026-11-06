package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func solid(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func pngBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func decodeAvatar(t *testing.T, dataURL string) image.Image {
	t.Helper()
	if !strings.HasPrefix(dataURL, avatarDataPrefix) {
		t.Fatalf("not a jpeg data url: %.30s", dataURL)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(dataURL, avatarDataPrefix))
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestMakeAvatarScalesAndCrops(t *testing.T) {
	// wide picture: red in the middle square, blue on the sides
	src := solid(600, 300, color.RGBA{0, 0, 255, 255})
	for y := 0; y < 300; y++ {
		for x := 150; x < 450; x++ {
			src.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	url, err := makeAvatar(pngBytes(t, src))
	if err != nil {
		t.Fatal(err)
	}
	img := decodeAvatar(t, url)
	if b := img.Bounds(); b.Dx() != avatarSize || b.Dy() != avatarSize {
		t.Fatalf("size %v", b)
	}
	r, _, bl, _ := img.At(avatarSize/2, avatarSize/2).RGBA()
	if r>>8 < 200 || bl>>8 > 80 {
		t.Fatalf("the middle should be red, got r=%d b=%d", r>>8, bl>>8)
	}
	r, _, bl, _ = img.At(2, 2).RGBA() // corner of the middle square: still red, the blue sides are cut off
	if r>>8 < 200 || bl>>8 > 80 {
		t.Fatalf("the sides should be cut off, got r=%d b=%d", r>>8, bl>>8)
	}
}

func TestMakeAvatarFlattensTransparency(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64)) // fully transparent
	url, err := makeAvatar(pngBytes(t, img))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := decodeAvatar(t, url).At(10, 10).RGBA()
	if r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
		t.Fatalf("transparent should become white, got %d %d %d", r>>8, g>>8, b>>8)
	}
}

func TestMakeAvatarSmallPictureStillWorks(t *testing.T) {
	if _, err := makeAvatar(pngBytes(t, solid(20, 20, color.White))); err != nil {
		t.Fatal(err)
	}
}

func TestMakeAvatarRefusesJunk(t *testing.T) {
	if _, err := makeAvatar(nil); err == nil {
		t.Fatal("empty accepted")
	}
	if _, err := makeAvatar([]byte("<html><script>alert(1)</script></html>")); err == nil {
		t.Fatal("html accepted")
	}
	if _, err := makeAvatar([]byte("\x89PNG\r\n\x1a\nnot really")); err == nil {
		t.Fatal("broken png accepted")
	}
	if _, err := makeAvatar(make([]byte, maxAvatarBytes+1)); err == nil {
		t.Fatal("huge upload accepted")
	}
}

func TestMakeAvatarRefusesDecompressionBomb(t *testing.T) {
	// a header that claims a gigantic picture: the size check must refuse it before decoding
	var b bytes.Buffer
	b.Write([]byte("\x89PNG\r\n\x1a\n"))
	b.Write([]byte{0, 0, 0, 13})
	b.WriteString("IHDR")
	b.Write([]byte{0, 0, 0xff, 0xff, 0, 0, 0xff, 0xff, 8, 6, 0, 0, 0})
	b.Write([]byte{0, 0, 0, 0})
	if _, err := makeAvatar(b.Bytes()); err == nil {
		t.Fatal("bomb accepted")
	}
}
