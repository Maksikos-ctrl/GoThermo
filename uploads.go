package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const maxUploadBytes = 25 << 20

func tooLargeErr() error {
	return fmt.Errorf("file is too large (max %d MB)", maxUploadBytes>>20)
}

func checkUploadSize(encodedLen int) error {
	if encodedLen > maxUploadBytes/3*4+64 {
		return tooLargeErr()
	}
	return nil
}

var activeContentTypes = map[string]bool{
	"text/html":              true,
	"application/xhtml+xml":  true,
	"image/svg+xml":          true,
	"text/javascript":        true,
	"application/javascript": true,
}

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

func insideUploads(path string) bool {
	abs, err1 := filepath.Abs(path)
	base, err2 := filepath.Abs(uploadsDir)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(base, abs)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..")
}

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
