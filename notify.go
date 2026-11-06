package main

import (
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode"
)

// Desktop notifications. The webview in the desktop app has no reliable
// Notification API (macOS WKWebView has none), so the app asks its own Go
// side to show one with the system's tool. Text is passed as arguments or
// environment variables, never spliced into a script, so a message can't
// inject commands.

const (
	maxNoticeTitle = 80
	maxNoticeBody  = 240
)

func cleanNotice(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > max {
		s = string(r[:max-1]) + "…"
	}
	return s
}

const winToastScript = `
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
$x = New-Object Windows.Data.Xml.Dom.XmlDocument
$x.LoadXml('<toast><visual><binding template="ToastGeneric"><text id="1"></text><text id="2"></text></binding></visual></toast>')
$t = $x.GetElementsByTagName('text')
$t.Item(0).InnerText = $env:GT_TITLE
$t.Item(1).InnerText = $env:GT_BODY
$n = New-Object Windows.UI.Notifications.ToastNotification $x
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('GoThermo').Show($n)
`

// notifyCommand builds the command that shows a notice on the given OS.
func notifyCommand(goos, title, body string) (name string, args []string, env []string, ok bool) {
	title = cleanNotice(title, maxNoticeTitle)
	body = cleanNotice(body, maxNoticeBody)
	if title == "" {
		title = "GoThermo"
	}
	switch goos {
	case "darwin":
		script := "on run argv\n display notification (item 1 of argv) with title (item 2 of argv)\nend run"
		return "osascript", []string{"-e", script, body, title}, nil, true
	case "linux":
		return "notify-send", []string{"--app-name=GoThermo", "--", title, body}, nil, true
	case "windows":
		return "powershell", []string{"-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", winToastScript},
			[]string{"GT_TITLE=" + title, "GT_BODY=" + body}, true
	}
	return "", nil, nil, false
}

// Notify shows a system notification on this computer. Desktop app only.
// Failure is silent on purpose: a missing notifier must never break chat.
func (a *App) Notify(title, body string) bool {
	name, args, env, ok := notifyCommand(runtime.GOOS, title, body)
	if !ok {
		return false
	}
	cmd := exec.Command(name, args...)
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}
	if err := cmd.Start(); err != nil {
		return false
	}
	go func() {
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			_ = cmd.Process.Kill()
		}
	}()
	return true
}
