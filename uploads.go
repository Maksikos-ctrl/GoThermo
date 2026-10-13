package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// 25 MB per file. The frontend sends files base64-encoded through Wails,
// so very large files would also eat a lot of memory on both sides.
const maxUploadBytes = 25 << 20

func tooLargeErr() error {
	return fmt.Errorf("file is too large (max %d MB)", maxUploadBytes>>20)
}

// checkUploadSize rejects an oversized upload before it is decoded.
// Base64 text is 4/3 the size of the data (plus a little for "data:" prefix
// and padding).
func checkUploadSize(encodedLen int) error {
	if encodedLen > maxUploadBytes/3*4+64 {
		return tooLargeErr()
	}
	return nil
}

// Types that run code when opened in a browser/webview. They are stored and
// served as opaque downloads, never rendered.
var activeContentTypes = map[string]bool{
	"text/html":              true,
	"application/xhtml+xml":  true,
	"image/svg+xml":          true,
	"text/javascript":        true,
	"application/javascript": true,
}

// validateUpload enforces the size limit and returns the MIME type to store.
// The type sent by the client is not trusted: an "image/*" that does not look
// like an image, or any active content type, becomes application/octet-stream.
func validateUpload(declaredMime string, data []byte) (string, error) {
	if len(data) > maxUploadBytes {
		return "", tooLargeErr()
	}

	sniffed := strings.ToLower(strings.SplitN(http.DetectContentType(data), ";", 2)[0])
	mime := strings.ToLower(strings.TrimSpace(strings.SplitN(declaredMime, ";", 2)[0]))

	switch {
	case mime == "":
		mime = sniffed
	case activeContentTypes[mime]:
		mime = "application/octet-stream"
	case strings.HasPrefix(mime, "image/") && !strings.HasPrefix(sniffed, "image/"):
		mime = "application/octet-stream"
	}
	if activeContentTypes[mime] {
		mime = "application/octet-stream"
	}
	return mime, nil
}

// insideUploads reports whether path is located in the uploads directory
// (guards deletes against a tampered Redis entry).
func insideUploads(path string) bool {
	abs, err1 := filepath.Abs(path)
	base, err2 := filepath.Abs(uploadsDir)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(base, abs)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..")
}

// removeUpload deletes an uploaded file and its metadata.
func removeUpload(fileID string) {
	if fileID == "" {
		return
	}
	if path, _, _, err := GetFileMeta(fileID); err == nil && path != "" && insideUploads(path) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Printf("⚠️ Could not delete upload %s: %v", path, err)
		}
	}
	redisClient.Del(ctx, "file:"+fileID)
}

// channelFileIDs lists the files attached to messages of a channel. Call it
// BEFORE deleting the channel, then pass the result to removeUploads.
func channelFileIDs(channel string) []string {
	msgs, err := GetMessages(channel, 1000000)
	if err != nil {
		return nil
	}
	var ids []string
	for _, m := range msgs {
		if m.IsFile && m.FileID != "" {
			ids = append(ids, m.FileID)
		}
	}
	return ids
}

func removeUploads(ids []string) {
	for _, id := range ids {
		removeUpload(id)
	}
}
