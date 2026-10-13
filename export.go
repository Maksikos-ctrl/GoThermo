package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ExportChannel saves a channel's history to a file chosen by the user.
// format is "json", "csv" or "txt". It returns the path that was written,
// or "" if the user closed the save dialog.
func (a *App) ExportChannel(channel, username, format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "json" && format != "csv" && format != "txt" {
		return "", fmt.Errorf("unknown export format %q", format)
	}

	ch, err := GetChannel(channel)
	if err != nil || ch == nil {
		return "", fmt.Errorf("channel not found")
	}
	if ch.IsPrivate && !channelHasMember(ch.Members, username) {
		return "", fmt.Errorf("you are not a participant of this chat")
	}

	msgs, err := GetMessages(channel, 1000000)
	if err != nil {
		return "", fmt.Errorf("error reading messages: %v", err)
	}

	now := time.Now()
	data, err := renderExport(format, channel, msgs, now)
	if err != nil {
		return "", fmt.Errorf("error building export: %v", err)
	}

	savePath, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		DefaultFilename: exportFilename(channel, format, now),
		Title:           "Export chat history",
	})
	if err != nil {
		return "", fmt.Errorf("save dialog failed: %v", err)
	}
	if savePath == "" {
		return "", nil
	}

	// Private chats end up in this file, so keep it readable by the owner only.
	if err := os.WriteFile(savePath, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write file: %v", err)
	}

	log.Printf("📤 %s exported #%s (%d messages, %s) to %s", username, channel, len(msgs), format, savePath)
	return savePath, nil
}

func channelHasMember(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func exportFilename(channel, format string, now time.Time) string {
	name := strings.Trim(unsafeFileChars.ReplaceAllString(channel, "_"), "._-")
	if name == "" {
		name = "chat"
	}
	return fmt.Sprintf("gothermo-%s-%s.%s", name, now.Format("2006-01-02"), format)
}

// renderExport turns messages into the bytes of the chosen format.
func renderExport(format, channel string, msgs []Message, now time.Time) ([]byte, error) {
	switch format {
	case "json":
		return exportJSON(channel, msgs, now)
	case "csv":
		return exportCSV(msgs)
	case "txt":
		return exportTXT(channel, msgs, now)
	}
	return nil, fmt.Errorf("unknown export format %q", format)
}

// ---------- shared text helpers ----------

func exportSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func exportDuration(sec int) string {
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	return fmt.Sprintf("%dm %02ds", sec/60, sec%60)
}

// messageKind and messageBody give a plain-text view of any message type.
func messageKind(m Message) string {
	switch {
	case m.IsCall:
		return "call"
	case m.IsFile:
		return "file"
	case m.IsPost:
		return "post"
	}
	return "message"
}

func messageBody(m Message) string {
	switch {
	case m.IsCall:
		who := m.CallInitiator
		if who == "" {
			who = m.User
		}
		s := fmt.Sprintf("Call by %s: %s", who, m.CallStatus)
		if m.CallStatus == "completed" {
			s += " (" + exportDuration(m.CallDuration) + ")"
		}
		return s
	case m.IsFile:
		return fmt.Sprintf("%s (%s)", m.FileName, exportSize(m.FileSize))
	}
	return m.Text
}

// reactionList gives "👍 alice, bob" lines in a stable order.
func reactionList(m Message) []string {
	emojis := make([]string, 0, len(m.Reactions))
	for e, users := range m.Reactions {
		if len(users) > 0 {
			emojis = append(emojis, e)
		}
	}
	sort.Strings(emojis)

	out := make([]string, 0, len(emojis))
	for _, e := range emojis {
		out = append(out, e+" "+strings.Join(m.Reactions[e], ", "))
	}
	return out
}

// ---------- JSON ----------

func exportJSON(channel string, msgs []Message, now time.Time) ([]byte, error) {
	if msgs == nil {
		msgs = []Message{}
	}
	doc := struct {
		Channel    string    `json:"channel"`
		ExportedAt time.Time `json:"exportedAt"`
		Count      int       `json:"count"`
		Messages   []Message `json:"messages"`
	}{channel, now, len(msgs), msgs}
	return json.MarshalIndent(doc, "", "  ")
}

// ---------- CSV ----------

// csvSafe stops spreadsheet programs from running a cell as a formula:
// a text starting with = + - @ (or a tab/CR) gets a leading apostrophe.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func exportCSV(msgs []Message) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM so Excel shows emoji and Cyrillic correctly

	w := csv.NewWriter(&buf)
	if err := w.Write([]string{"timestamp", "user", "type", "text", "file_name", "file_size", "reactions", "edited"}); err != nil {
		return nil, err
	}
	for _, m := range msgs {
		text, fileName, size := m.Text, "", ""
		if m.IsCall || m.IsFile {
			text = messageBody(m)
		}
		if m.IsFile {
			fileName, size = m.FileName, fmt.Sprint(m.FileSize)
		}
		edited := ""
		if m.IsEdited {
			edited = "yes"
		}
		row := []string{
			m.Timestamp.Format(time.RFC3339),
			csvSafe(m.User),
			messageKind(m),
			csvSafe(text),
			csvSafe(fileName),
			size,
			csvSafe(strings.Join(reactionList(m), "; ")),
			edited,
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// ---------- TXT (text/template) ----------

type txtLine struct {
	NewDay    bool
	Day       string
	Time      string
	User      string
	Flags     string
	Body      string
	Reactions []string
}

var txtTemplate = template.Must(template.New("transcript").Parse(
	`GoThermo chat export: {{.Channel}}
Exported {{.ExportedAt}}, {{.Count}} messages
{{range .Lines}}{{if .NewDay}}
=== {{.Day}} ===
{{end}}[{{.Time}}] {{.User}}{{.Flags}}: {{.Body}}
{{range .Reactions}}    {{.}}
{{end}}{{end}}`))

func exportTXT(channel string, msgs []Message, now time.Time) ([]byte, error) {
	lines := make([]txtLine, 0, len(msgs))
	lastDay := ""
	for _, m := range msgs {
		t := m.Timestamp.Local()
		day := t.Format("Monday, 2 January 2006")

		flags := ""
		if m.IsPost {
			flags += " [post]"
		}
		if m.IsEdited {
			flags += " (edited)"
		}

		lines = append(lines, txtLine{
			NewDay:    day != lastDay,
			Day:       day,
			Time:      t.Format("15:04"),
			User:      m.User,
			Flags:     flags,
			Body:      strings.ReplaceAll(strings.TrimRight(messageBody(m), "\n"), "\n", "\n    "),
			Reactions: reactionList(m),
		})
		lastDay = day
	}

	var buf bytes.Buffer
	err := txtTemplate.Execute(&buf, map[string]any{
		"Channel":    channel,
		"ExportedAt": now.Format("2006-01-02 15:04"),
		"Count":      len(msgs),
		"Lines":      lines,
	})
	return buf.Bytes(), err
}
