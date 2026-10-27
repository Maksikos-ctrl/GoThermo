package main

import (
	"strings"
	"testing"
)

func TestBuildChannelExportOverRPC(t *testing.T) {
	useMiniRedis(t)
	SaveChannel(Channel{Name: "general"})
	SaveMessage(Message{ID: "1", User: "ann", Text: "hello, world", Channel: "general", Timestamp: t0})
	ann, _ := CreateSession("ann")
	h := ServeRPC(&App{hub: NewHub()})

	for _, c := range []struct{ format, want string }{
		{"json", `"hello, world"`},
		{"csv", "hello, world"},
		{"txt", "hello, world"},
	} {
		code, rep := rpcCall(t, h, "BuildChannelExport", `["`+ann+`","general","ann","`+c.format+`"]`)
		if code != 200 || rep.Error != "" {
			t.Fatalf("%s: %d %q", c.format, code, rep.Error)
		}
		f, _ := rep.Result.(map[string]interface{})
		name, _ := f["filename"].(string)
		content, _ := f["content"].(string)
		if !strings.HasPrefix(name, "gothermo-general-") || !strings.HasSuffix(name, "."+c.format) {
			t.Fatalf("%s: filename %q", c.format, name)
		}
		if !strings.Contains(content, c.want) {
			t.Fatalf("%s: content %q", c.format, content)
		}
	}
}

func TestBuildChannelExportRefusals(t *testing.T) {
	useMiniRedis(t)
	SaveChannel(Channel{Name: "general"})
	SaveChannel(Channel{Name: "dm_ann_bob", IsPrivate: true, Members: []string{"ann", "bob"}})
	SaveMessage(Message{ID: "1", User: "ann", Text: "top secret", Channel: "dm_ann_bob", Timestamp: t0})
	ann, _ := CreateSession("ann")
	eve, _ := CreateSession("eve")
	h := ServeRPC(&App{hub: NewHub()})

	cases := []struct{ name, body, want string }{
		{"no token", `["","dm_ann_bob","ann","json"]`, ""},
		{"someone else's token", `["` + eve + `","dm_ann_bob","ann","json"]`, ""},
		{"outsider", `["` + eve + `","dm_ann_bob","eve","json"]`, "not a participant"},
		{"unknown channel", `["` + ann + `","nope","ann","json"]`, "channel not found"},
		{"bad format", `["` + ann + `","general","ann","pdf"]`, "unknown export format"},
	}
	for _, c := range cases {
		_, rep := rpcCall(t, h, "BuildChannelExport", c.body)
		if rep.Error == "" || !strings.Contains(rep.Error, c.want) {
			t.Errorf("%s: error %q", c.name, rep.Error)
		}
		if rep.Result != nil {
			t.Errorf("%s: content leaked: %v", c.name, rep.Result)
		}
	}
}

func TestSaveTextFileIsNotOverTheNetwork(t *testing.T) {
	useMiniRedis(t)
	h := ServeRPC(&App{hub: NewHub()})
	if code, _ := rpcCall(t, h, "SaveTextFile", `["x.txt","data"]`); code != 404 {
		t.Fatalf("SaveTextFile must not be reachable over the network, got %d", code)
	}
	if code, _ := rpcCall(t, h, "ExportChannel", `["t","c","u","json"]`); code != 404 {
		t.Fatalf("ExportChannel must stay desktop-only, got %d", code)
	}
}
