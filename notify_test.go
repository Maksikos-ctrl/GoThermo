package main

import (
	"strings"
	"testing"
)

func TestCleanNotice(t *testing.T) {
	if got := cleanNotice("hi\n\x00there\t you", 50); got != "hi there you" {
		t.Fatalf("got %q", got)
	}
	long := strings.Repeat("я", 500)
	if got := cleanNotice(long, 10); len([]rune(got)) != 10 || !strings.HasSuffix(got, "…") {
		t.Fatalf("got %q", got)
	}
}

func TestNotifyCommandKeepsTextOutOfScripts(t *testing.T) {
	evil := `"; do shell script "rm -rf ~" --`
	name, args, _, ok := notifyCommand("darwin", evil, evil)
	if !ok || name != "osascript" {
		t.Fatal("darwin")
	}
	if strings.Contains(args[1], "rm -rf") {
		t.Fatal("text leaked into the AppleScript source")
	}
	if args[2] == "" || args[3] == "" {
		t.Fatal("text should travel as arguments")
	}

	_, args, _, ok = notifyCommand("linux", "t", "-b")
	if !ok || args[len(args)-3] != "--" {
		t.Fatalf("linux args %v", args)
	}

	_, args, env, ok := notifyCommand("windows", "T'; calc", "B")
	if !ok || strings.Contains(strings.Join(args, " "), "calc") {
		t.Fatal("windows text leaked into script")
	}
	if len(env) != 2 || !strings.HasPrefix(env[0], "GT_TITLE=") {
		t.Fatalf("env %v", env)
	}

	if _, _, _, ok := notifyCommand("plan9", "a", "b"); ok {
		t.Fatal("unknown OS should not notify")
	}
	_, args, _, _ = notifyCommand("linux", "", "x")
	if args[len(args)-2] != "GoThermo" {
		t.Fatalf("default title: %v", args)
	}
}
