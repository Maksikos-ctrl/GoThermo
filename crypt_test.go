package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

func withKey(t *testing.T, key string) {
	t.Helper()
	t.Setenv("GOTHERMO_DATA_KEY", key)
	if err := loadDataKey(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dataKey = nil })
}

func goodKey() string { return base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))) }

func TestNoKeyMeansNoEncryption(t *testing.T) {
	t.Setenv("GOTHERMO_DATA_KEY", "")
	if err := loadDataKey(); err != nil || encryptionOn() {
		t.Fatal("should be off")
	}
	if sealText("hello") != "hello" || openText("hello") != "hello" {
		t.Fatal("plain text must pass through")
	}
}

func TestSealOpenRoundTrip(t *testing.T) {
	withKey(t, goodKey())
	for _, s := range []string{"hello", "привет 👋", strings.Repeat("x", 5000), "line1\nline2", "enc:v1:AAAA"} {
		sealed := sealText(s)
		if !strings.HasPrefix(sealed, encPrefix) || strings.Contains(sealed, s) && len(s) > 6 {
			t.Fatalf("not sealed: %q", sealed)
		}
		if got := openText(sealed); got != s {
			t.Fatalf("round trip %q -> %q", s, got)
		}
	}
	if sealText("") != "" {
		t.Fatal("empty text stays empty")
	}
	a, b := sealText("same"), sealText("same")
	if a == b {
		t.Fatal("nonce must differ each time")
	}
	if got := openText(sealText(a)); got != a {
		t.Fatalf("text that looks sealed must round-trip, got %q", got)
	}
}

func TestOldPlainRowsStillReadable(t *testing.T) {
	withKey(t, goodKey())
	if openText("written before the key") != "written before the key" {
		t.Fatal("legacy row")
	}
}

func TestWrongOrMissingKey(t *testing.T) {
	withKey(t, goodKey())
	sealed := sealText("secret")
	other := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("z", 32)))
	t.Setenv("GOTHERMO_DATA_KEY", other)
	loadDataKey()
	if openText(sealed) != badKeyNotice {
		t.Fatal("wrong key must not decrypt")
	}
	t.Setenv("GOTHERMO_DATA_KEY", "")
	loadDataKey()
	if openText(sealed) != noKeyNotice {
		t.Fatal("missing key notice")
	}
	if strings.Contains(openText(sealed), "secret") {
		t.Fatal("leak")
	}
}

func TestTamperedCiphertextIsRefused(t *testing.T) {
	withKey(t, goodKey())
	sealed := sealText("secret")
	raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(sealed, encPrefix))
	raw[len(raw)-1] ^= 1
	if openText(encPrefix+base64.StdEncoding.EncodeToString(raw)) != badKeyNotice {
		t.Fatal("tampering must be detected")
	}
	if openText(encPrefix+"!!!not base64") != badKeyNotice {
		t.Fatal("garbage must be refused")
	}
}

func TestBadKeyIsAnError(t *testing.T) {
	for _, v := range []string{"short", base64.StdEncoding.EncodeToString([]byte("too short")), "!!!"} {
		t.Setenv("GOTHERMO_DATA_KEY", v)
		if err := loadDataKey(); err == nil {
			t.Fatalf("%q should be refused", v)
		}
	}
	dataKey = nil
}
