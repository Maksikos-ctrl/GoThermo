package main

import (
	"encoding/base64"
	"image/color"
	"testing"
)

func TestSetAndRemoveAvatar(t *testing.T) {
	mr := useMiniRedis(t)
	a := &App{hub: NewHub()}
	go a.hub.Run()
	ta := mkUser(t, mr, "alice")
	tb := mkUser(t, mr, "bob")
	data := base64.StdEncoding.EncodeToString(pngBytes(t, solid(300, 200, color.RGBA{10, 200, 30, 255})))

	if err := a.SetAvatar("bogus", "alice", data); err == nil {
		t.Fatal("anonymous upload accepted")
	}
	if err := a.SetAvatar(tb, "alice", data); err == nil {
		t.Fatal("bob set alice's picture")
	}
	if err := a.SetAvatar(ta, "alice", "data:image/png;base64,"+data); err != nil {
		t.Fatal(err)
	}
	if err := a.SetAvatar(ta, "alice", "not base64 at all !!"); err == nil {
		t.Fatal("garbage accepted")
	}

	users, err := a.GetUsers(tb)
	if err != nil {
		t.Fatal(err)
	}
	var alice, bob *User
	for i := range users {
		switch users[i].Username {
		case "alice":
			alice = &users[i]
		case "bob":
			bob = &users[i]
		}
	}
	if alice == nil || bob == nil {
		t.Fatalf("users: %+v", users)
	}
	decodeAvatar(t, alice.Avatar)
	if bob.Avatar != "" {
		t.Fatal("bob has no picture")
	}

	if err := a.RemoveAvatar(ta, "alice"); err != nil {
		t.Fatal(err)
	}
	users, _ = a.GetUsers(tb)
	for _, u := range users {
		if u.Avatar != "" {
			t.Fatalf("%s still has a picture", u.Username)
		}
	}
}
