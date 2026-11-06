package main

import (
	"fmt"
	"testing"
	"time"
)

type decisionEnv struct {
	a                *App
	ta, tb, tc       string
	generalMessageID string
}

func newDecisionEnv(t *testing.T) decisionEnv {
	t.Helper()
	mr := useMiniRedis(t)
	a := &App{hub: NewHub()}
	go a.hub.Run()
	SaveChannel(Channel{Name: "general", Members: []string{}, CreatedBy: "system", CreatedAt: time.Now()})
	SaveChannel(Channel{Name: "dm_alice_bob", Members: []string{"alice", "bob"}, IsPrivate: true, CreatedBy: "system", CreatedAt: time.Now()})
	e := decisionEnv{a: a, ta: mkUser(t, mr, "alice"), tb: mkUser(t, mr, "bob"), tc: mkUser(t, mr, "carol")}
	id, err := a.SendMessage(e.ta, "alice", "We ship on Friday, @bob please deploy and tell @ghost", "general")
	if err != nil {
		t.Fatal(err)
	}
	e.generalMessageID = id
	return e
}

func TestMarkAndListDecisions(t *testing.T) {
	e := newDecisionEnv(t)
	a := e.a

	if err := a.MarkDecision("bogus", "alice", "general", e.generalMessageID, "task"); err == nil {
		t.Fatal("anonymous mark accepted")
	}
	if err := a.MarkDecision(e.tb, "bob", "general", e.generalMessageID, "wish"); err == nil {
		t.Fatal("unknown kind accepted")
	}
	if err := a.MarkDecision(e.tb, "bob", "general", "no-such", "task"); err == nil {
		t.Fatal("missing message accepted")
	}

	if err := a.MarkDecision(e.tb, "bob", "general", e.generalMessageID, "TASK"); err != nil {
		t.Fatal(err)
	}
	list, err := a.GetDecisions(e.tc, "general")
	if err != nil || len(list) != 1 {
		t.Fatalf("%v %+v", err, list)
	}
	d := list[0]
	if d.Kind != "task" || d.Author != "alice" || d.MarkedBy != "bob" || d.Assignee != "bob" || d.Done {
		t.Fatalf("%+v", d)
	}

	// marking again only changes the kind, it does not duplicate
	if err := a.MarkDecision(e.tc, "carol", "general", e.generalMessageID, "decision"); err != nil {
		t.Fatal(err)
	}
	list, _ = a.GetDecisions(e.tc, "general")
	if len(list) != 1 || list[0].Kind != "decision" || list[0].MarkedBy != "bob" {
		t.Fatalf("%+v", list)
	}
}

func TestNewestFirst(t *testing.T) {
	e := newDecisionEnv(t)
	second, err := e.a.SendMessage(e.tb, "bob", "second", "general")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.a.MarkDecision(e.ta, "alice", "general", e.generalMessageID, "decision"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := e.a.MarkDecision(e.ta, "alice", "general", second, "decision"); err != nil {
		t.Fatal(err)
	}
	list, _ := e.a.GetDecisions(e.ta, "general")
	if len(list) != 2 || list[0].MessageID != second {
		t.Fatalf("%+v", list)
	}
}

func TestTaskDone(t *testing.T) {
	e := newDecisionEnv(t)
	a := e.a
	if err := a.MarkDecision(e.ta, "alice", "general", e.generalMessageID, "task"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetDecisionDone(e.tb, "bob", "general", e.generalMessageID, true); err != nil {
		t.Fatal(err)
	}
	list, _ := a.GetDecisions(e.ta, "general")
	if !list[0].Done || list[0].DoneBy != "bob" {
		t.Fatalf("%+v", list[0])
	}
	if err := a.SetDecisionDone(e.tb, "bob", "general", e.generalMessageID, false); err != nil {
		t.Fatal(err)
	}
	list, _ = a.GetDecisions(e.ta, "general")
	if list[0].Done || list[0].DoneBy != "" {
		t.Fatalf("%+v", list[0])
	}

	// a decision has nothing to tick off
	if err := a.MarkDecision(e.ta, "alice", "general", e.generalMessageID, "decision"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetDecisionDone(e.tb, "bob", "general", e.generalMessageID, true); err == nil {
		t.Fatal("a decision was ticked off")
	}
}

func TestRemoveDecision(t *testing.T) {
	e := newDecisionEnv(t)
	a := e.a
	if err := a.MarkDecision(e.tb, "bob", "general", e.generalMessageID, "task"); err != nil {
		t.Fatal(err)
	}
	if err := a.RemoveDecision(e.tc, "carol", "general", e.generalMessageID); err == nil {
		t.Fatal("a stranger removed an entry")
	}
	// the message's author (alice) may, so may the one who marked it (bob)
	if err := a.RemoveDecision(e.ta, "alice", "general", e.generalMessageID); err != nil {
		t.Fatal(err)
	}
	if list, _ := a.GetDecisions(e.ta, "general"); len(list) != 0 {
		t.Fatalf("%+v", list)
	}
	if err := a.RemoveDecision(e.ta, "alice", "general", e.generalMessageID); err != nil {
		t.Fatal("removing twice should be fine:", err)
	}
	if err := a.MarkDecision(e.tb, "bob", "general", e.generalMessageID, "task"); err != nil {
		t.Fatal(err)
	}
	if err := a.RemoveDecision(e.tb, "bob", "general", e.generalMessageID); err != nil {
		t.Fatal(err)
	}
}

func TestDecisionsRespectPrivateChannels(t *testing.T) {
	e := newDecisionEnv(t)
	id, err := e.a.SendMessage(e.ta, "alice", "secret plan", "dm_alice_bob")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.a.MarkDecision(e.tc, "carol", "dm_alice_bob", id, "decision"); err == nil {
		t.Fatal("outsider marked a private message")
	}
	if err := e.a.MarkDecision(e.ta, "alice", "dm_alice_bob", id, "decision"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.GetDecisions(e.tc, "dm_alice_bob"); err == nil {
		t.Fatal("outsider read a private list")
	}
}

func TestDecisionLimitAndChannelCleanup(t *testing.T) {
	e := newDecisionEnv(t)
	for i := 0; i < maxDecisionsPerChannel; i++ {
		d := Decision{MessageID: fmt.Sprintf("m%d", i), Channel: "general", Kind: "decision", At: time.Now()}
		if err := saveDecision(d); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.a.MarkDecision(e.ta, "alice", "general", e.generalMessageID, "decision"); err == nil {
		t.Fatal("limit not enforced")
	}
	deleteChannelDecisions("general")
	if err := e.a.MarkDecision(e.ta, "alice", "general", e.generalMessageID, "decision"); err != nil {
		t.Fatal(err)
	}
}
