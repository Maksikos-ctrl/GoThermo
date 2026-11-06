package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// Encryption of message text in the database.
//
// With GOTHERMO_DATA_KEY set (32 random bytes, base64), the text of every
// message and reply is stored as "enc:v1:" + base64(nonce + AES-256-GCM
// ciphertext). A copy of the database or a disk snapshot then shows no
// message text. The server holds the key, so this does NOT hide messages from
// whoever controls the server itself: that would need end-to-end encryption.
//
// Rows written before the key was set stay readable (they have no prefix);
// `gothermo-server encrypt` converts them. Without a key nothing is encrypted,
// exactly as before.
const encPrefix = "enc:v1:"

var dataKey []byte

// loadDataKey reads GOTHERMO_DATA_KEY. Unset means "no encryption"; a value
// that is set but wrong is an error, so a typo never silently disables it.
func loadDataKey() error {
	v := strings.TrimSpace(os.Getenv("GOTHERMO_DATA_KEY"))
	if v == "" {
		dataKey = nil
		return nil
	}
	b, err := base64.StdEncoding.DecodeString(v)
	if err != nil || len(b) != 32 {
		return fmt.Errorf("GOTHERMO_DATA_KEY must be 32 random bytes in base64 (make one with: openssl rand -base64 32)")
	}
	dataKey = b
	return nil
}

func encryptionOn() bool { return len(dataKey) == 32 }

func newGCM() (cipher.AEAD, error) {
	block, err := aes.NewCipher(dataKey)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// sealText encrypts s when a key is set (empty text stays empty). Callers
// always pass plain text, so text that merely looks like a sealed value is
// sealed like any other.
func sealText(s string) string {
	if !encryptionOn() || s == "" {
		return s
	}
	gcm, err := newGCM()
	if err != nil {
		return s // cannot happen with a 32-byte key
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return s
	}
	out := gcm.Seal(nonce, nonce, []byte(s), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(out)
}

const (
	noKeyNotice  = "🔒 [encrypted message: this server has no data key]"
	badKeyNotice = "🔒 [could not decrypt this message: wrong data key?]"
)

// openText reverses sealText. Unprefixed text (older rows) is returned as is.
func openText(s string) string {
	if !strings.HasPrefix(s, encPrefix) {
		return s
	}
	if !encryptionOn() {
		return noKeyNotice
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, encPrefix))
	if err != nil {
		return badKeyNotice
	}
	gcm, err := newGCM()
	if err != nil || len(raw) < gcm.NonceSize() {
		return badKeyNotice
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return badKeyNotice
	}
	return string(plain)
}
