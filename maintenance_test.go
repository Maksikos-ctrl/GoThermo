package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func msgJSON(id string, extra func(*Message)) string {
	m := Message{ID: id, User: "u", Text: "t-" + id, Channel: "c"}
	if extra != nil {
		extra(&m)
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func TestCompactDeletedMessages(t *testing.T) {
	mr := useMiniRedis(t)
	a, b, c := msgJSON("a", nil), msgJSON("b", nil), msgJSON("c", nil)
	mr.RPush("channel:one:messages", a, "DELETED", b, "DELETED", "DELETED", c)
	mr.RPush("channel:two:messages", "DELETED", "DELETED")
	mr.RPush("channel:clean:messages", a, b)
	mr.RPush("other:list", "DELETED") // must not be touched

	CompactDeletedMessages()

	one, _ := mr.List("channel:one:messages")
	if len(one) != 3 || one[0] != a || one[1] != b || one[2] != c {
		t.Fatalf("one = %v", one)
	}
	if mr.Exists("channel:two:messages") {
		t.Fatal("all-deleted list should disappear")
	}
	clean, _ := mr.List("channel:clean:messages")
	if len(clean) != 2 {
		t.Fatalf("clean = %v", clean)
	}
	other, _ := mr.List("other:list")
	if len(other) != 1 {
		t.Fatalf("other list touched: %v", other)
	}
	msgs, err := GetMessages("one", 100)
	if err != nil || len(msgs) != 3 {
		t.Fatalf("GetMessages: %v %v", msgs, err)
	}
	// second run is a no-op
	CompactDeletedMessages()
	one, _ = mr.List("channel:one:messages")
	if len(one) != 3 {
		t.Fatal("not idempotent")
	}
}

func TestCompactRetriesWhenListChanges(t *testing.T) {
	mr := useMiniRedis(t)
	mr.RPush("channel:x:messages", msgJSON("a", nil), "DELETED")
	// a competing writer appends between our WATCH and EXEC on the first attempt
	n := 0
	redisClient.AddHook(hookFunc(func() {
		if n == 0 {
			n++
			mr.RPush("channel:x:messages", msgJSON("late", nil))
		}
	}))
	removed, err := compactMessageList("channel:x:messages")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("competing write did not fire (n=%d), test is not exercising the retry", n)
	}
	items, _ := mr.List("channel:x:messages")
	hasLate := false
	for _, it := range items {
		if it == msgJSON("late", nil) {
			hasLate = true
		}
		if it == "DELETED" {
			t.Fatal("placeholder left")
		}
	}
	if !hasLate || removed != 1 {
		t.Fatalf("late message lost or wrong count: removed=%d items=%v", removed, items)
	}
}

func TestValidateUpload(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	cases := []struct {
		name, declared string
		data           []byte
		want           string
	}{
		{"real png", "image/png", png, "image/png"},
		{"fake image", "image/png", []byte("hello world"), "application/octet-stream"},
		{"html upload", "text/html", []byte("<html><script>1</script></html>"), "application/octet-stream"},
		{"svg upload", "image/svg+xml", []byte("<svg onload=alert(1)>"), "application/octet-stream"},
		{"empty declared", "", []byte("just text"), "text/plain"},
		{"html sniffed", "", []byte("<!DOCTYPE html><html></html>"), "application/octet-stream"},
		{"pdf passes", "application/pdf", []byte("%PDF-1.4 ..."), "application/pdf"},
		{"charset param", "Text/Plain; charset=utf-8", []byte("hi"), "text/plain"},
	}
	for _, c := range cases {
		got, err := validateUpload(c.declared, c.data)
		if err != nil || got != c.want {
			t.Errorf("%s: got %q err=%v want %q", c.name, got, err, c.want)
		}
	}
	if _, err := validateUpload("application/zip", make([]byte, maxUploadBytes+1)); err == nil {
		t.Error("oversized must fail")
	}
	if checkUploadSize(maxUploadBytes/3*4) != nil {
		t.Error("limit-sized upload must pass")
	}
	if checkUploadSize(maxUploadBytes*2) == nil {
		t.Error("huge upload must fail")
	}
}

func TestRemoveUploads(t *testing.T) {
	mr := useMiniRedis(t)
	dir := t.TempDir()
	wd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(wd)
	os.MkdirAll(uploadsDir, 0755)

	p := filepath.Join(uploadsDir, "id1_a.txt")
	os.WriteFile(p, []byte("x"), 0644)
	SaveFileMeta("id1", p, "text/plain", "a.txt")
	mr.RPush("channel:c:messages",
		msgJSON("m1", func(m *Message) { m.IsFile = true; m.FileID = "id1" }),
		msgJSON("m2", nil), "DELETED")

	ids := channelFileIDs("c")
	if len(ids) != 1 || ids[0] != "id1" {
		t.Fatalf("ids=%v", ids)
	}
	removeUploads(ids)
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("file should be gone")
	}
	if mr.Exists("file:id1") {
		t.Fatal("meta should be gone")
	}

	// a tampered path outside uploads/ must never be deleted
	outside := filepath.Join(dir, "precious.txt")
	os.WriteFile(outside, []byte("keep"), 0644)
	SaveFileMeta("id2", outside, "text/plain", "x")
	removeUpload("id2")
	if _, err := os.Stat(outside); err != nil {
		t.Fatal("file outside uploads was deleted!")
	}
	if insideUploads(uploadsDir) {
		t.Error("the uploads dir itself is not a file inside it")
	}
}
