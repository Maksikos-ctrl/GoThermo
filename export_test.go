package main

import (
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 14, 9, 5, 0, 0, time.Local)

func sampleMsgs() []Message {
	return []Message{
		{ID: "1", User: "ann", Text: "hello\nsecond line", Timestamp: t0, Reactions: map[string][]string{"👍": {"bob", "cy"}, "❤️": {"bob"}, "x": {}}},
		{ID: "2", User: "bob", Text: "=HYPERLINK(\"http://evil\")", Timestamp: t0.Add(time.Minute), IsEdited: true},
		{ID: "3", User: "ann", Text: "Big news", IsPost: true, Timestamp: t0.Add(2 * time.Minute)},
		{ID: "4", User: "bob", IsFile: true, FileName: "plan.pdf", FileSize: 1536, Timestamp: t0.Add(3 * time.Minute)},
		{ID: "5", User: "ann", IsCall: true, CallStatus: "completed", CallDuration: 125, CallInitiator: "ann", Timestamp: t0.Add(24 * time.Hour)},
	}
}

func TestExportJSON(t *testing.T) {
	b, err := renderExport("json", "general", sampleMsgs(), t0)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Channel  string
		Count    int
		Messages []Message
	}
	if err := json.Unmarshal(b, &doc); err != nil || doc.Channel != "general" || doc.Count != 5 || len(doc.Messages) != 5 {
		t.Fatalf("bad json: %v %+v", err, doc)
	}
	empty, _ := renderExport("json", "x", nil, t0)
	if !strings.Contains(string(empty), `"messages": []`) {
		t.Fatalf("empty export should have [] not null: %s", empty)
	}
}

func TestExportCSV(t *testing.T) {
	b, err := renderExport("csv", "general", sampleMsgs(), t0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "\xEF\xBB\xBF") {
		t.Fatal("missing BOM")
	}
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(b), "\xEF\xBB\xBF"))).ReadAll()
	if err != nil || len(rows) != 6 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	if rows[1][3] != "hello\nsecond line" || rows[1][6] != "❤️ bob; 👍 bob, cy" {
		t.Fatalf("row1: %q", rows[1])
	}
	if !strings.HasPrefix(rows[2][3], "'=") || rows[2][7] != "yes" {
		t.Fatalf("formula must be neutralised, got %q", rows[2][3])
	}
	if rows[3][2] != "post" || rows[4][2] != "file" || rows[4][4] != "plan.pdf" || rows[4][5] != "1536" {
		t.Fatalf("types: %q %q", rows[3], rows[4])
	}
	if rows[5][2] != "call" || rows[5][3] != "Call by ann: completed (2m 05s)" {
		t.Fatalf("call row: %q", rows[5])
	}
}

func TestExportTXT(t *testing.T) {
	b, err := renderExport("txt", "general", sampleMsgs(), t0)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		"GoThermo chat export: general",
		"5 messages",
		"[09:05] ann: hello\n    second line\n    ❤️ bob\n    👍 bob, cy\n",
		"[09:06] bob (edited): =HYPERLINK", // TXT is plain text, no escaping needed
		"[09:07] ann [post]: Big news",
		"[09:08] bob: plan.pdf (1.5 KB)",
		"Call by ann: completed (2m 05s)",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Count(s, "=== ") != 2 {
		t.Errorf("expected 2 day headers:\n%s", s)
	}
	if strings.Contains(s, "x \n") {
		t.Error("empty reaction list must not be printed")
	}
}

func TestExportFilenameAndUnknownFormat(t *testing.T) {
	if got := exportFilename("dm_ann_bob", "csv", t0); got != "gothermo-dm_ann_bob-2026-10-14.csv" {
		t.Fatal(got)
	}
	if got := exportFilename("../../etc/passwd", "txt", t0); strings.ContainsAny(got, "/\\") || !strings.HasSuffix(got, ".txt") {
		t.Fatal(got)
	}
	if got := exportFilename("!!!", "json", t0); got != "gothermo-chat-2026-10-14.json" {
		t.Fatal(got)
	}
	if _, err := renderExport("xml", "c", nil, t0); err == nil {
		t.Fatal("xml must be rejected")
	}
}

func TestExportChannelPermissions(t *testing.T) {
	useMiniRedis(t)
	SaveChannel(Channel{Name: "general"})
	SaveChannel(Channel{Name: "dm_ann_bob", IsPrivate: true, Members: []string{"ann", "bob"}})
	SaveMessage(Message{ID: "1", User: "ann", Text: "secret", Channel: "dm_ann_bob", Timestamp: t0})
	ann, _ := CreateSession("ann")
	eve, _ := CreateSession("eve")

	// Only the refusals are tested here: a successful export opens a native
	// save dialog, which needs the running Wails app.
	a := &App{}
	if _, err := a.ExportChannel("", "dm_ann_bob", "ann", "json"); err == nil {
		t.Fatal("no token must be refused")
	}
	if _, err := a.ExportChannel(eve, "dm_ann_bob", "ann", "json"); err == nil {
		t.Fatal("someone else's token must be refused")
	}
	if _, err := a.ExportChannel(eve, "dm_ann_bob", "eve", "json"); err == nil || !strings.Contains(err.Error(), "not a participant") {
		t.Fatalf("outsider must be refused: %v", err)
	}
	if _, err := a.ExportChannel(ann, "nope", "ann", "json"); err == nil {
		t.Fatal("unknown channel must fail")
	}
	if _, err := a.ExportChannel(ann, "general", "ann", "pdf"); err == nil {
		t.Fatal("bad format must fail")
	}
}
