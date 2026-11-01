package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedLegacy fills Redis the way the app has filled it over time.
func seedLegacy(t *testing.T) {
	t.Helper()
	r := redisStore{}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	base := time.Now().Add(-48 * time.Hour).Truncate(time.Millisecond)
	at := func(m int) time.Time { return base.Add(time.Duration(m) * time.Minute) }

	must(r.SaveUser(&User{ID: "u-alice", Username: "alice", Email: "alice@x.com", Status: "online", IsOnline: true}))
	must(r.SaveUser(&User{ID: "u-bob", Username: "bob", Email: "Bob@x.com", Status: "offline", LastSeen: at(5).Format(time.RFC3339)}))
	must(r.SaveUser(&User{ID: "u-gina", Username: "gina", Email: "gina@gmail.com", Provider: "google", ProviderID: "g-123", Status: "away", IsOnline: true}))
	must(r.SetPasswordHash("alice@x.com", "$2a$10$alicehash"))
	must(r.SetPasswordHash("Bob@x.com", "$2a$10$bobhash"))

	must(r.SaveChannel(Channel{ID: "c-gen", Name: "general", Members: []string{}, CreatedBy: "system", CreatedAt: at(0)}))
	must(r.SaveChannel(Channel{ID: "c-sec", Name: "secret", IsPrivate: true, Members: []string{"bob", "alice"}, Description: "hush", CreatedBy: "bob", CreatedAt: at(1)}))
	must(r.SaveChannel(Channel{Name: "old", Members: []string{}, CreatedBy: "alice", CreatedAt: at(2)})) // an old channel without an id

	m1 := Message{ID: "m1", User: "alice", Text: "hello", Channel: "general", Timestamp: at(10), Reactions: map[string][]string{"👍": {"bob", "gina"}, "🎉": {"alice"}}}
	m2 := Message{ID: "m2", User: "bob", Text: "second", Channel: "general", Timestamp: at(11), Reactions: map[string][]string{}, IsEdited: true}
	m3 := Message{ID: "m3", User: "alice", Text: "file", Channel: "general", Timestamp: at(12), Reactions: map[string][]string{}, IsFile: true, FileID: "f1", FileName: "a.png", FileSize: 12, MimeType: "image/png"}
	must(r.AppendMessage(m1))
	must(r.AppendMessage(m2))
	must(redisClient.RPush(ctx, "channel:general:messages", "DELETED").Err()) // a removed message
	must(r.AppendMessage(m3))
	must(r.AppendMessage(Message{ID: "s1", User: "bob", Text: "secret talk", Channel: "secret", Timestamp: at(13), Reactions: map[string][]string{}}))
	must(r.AppendMessage(Message{ID: "o1", User: "alice", Text: "in old", Channel: "old", Timestamp: at(14), Reactions: map[string][]string{}}))

	// bob is seen by the unread-replies feature before the replies arrive
	for _, user := range []string{"bob", "alice", "gina"} {
		if _, err := r.ThreadUnread(user, "general"); err != nil {
			t.Fatal(err)
		}
	}
	// The replies come shortly after bob was first seen and before the migration runs.
	now := time.Now()
	time.Sleep(10 * time.Millisecond)
	must(r.AddReply(Message{ID: "r1", User: "alice", Text: "reply 1", Channel: "general", ThreadID: "m2", Timestamp: now.Add(time.Millisecond), Reactions: map[string][]string{}}))
	must(r.AddReply(Message{ID: "r2", User: "gina", Text: "reply 2", Channel: "general", ThreadID: "m2", Timestamp: now.Add(2 * time.Millisecond), Reactions: map[string][]string{}}))
	if _, err := r.RefreshThreadMeta("general", "m2"); err != nil { // what the app does after a reply
		t.Fatal(err)
	}
	// alice opened the thread between the two replies
	must(redisClient.HSet(ctx, threadReadKey("alice"), threadReadField("general", "m2"),
		now.Add(1500*time.Microsecond).Format(time.RFC3339Nano)).Err())

	must(r.AddPin("general", pinEntry{MessageID: "m1", PinnedBy: "alice", PinnedAt: at(20)}))
	must(r.AddPin("general", pinEntry{MessageID: "m3", PinnedBy: "bob", PinnedAt: at(21)}))

	must(r.SaveFileMeta("f1", "/srv/uploads/f1.png", "image/png", "a.png"))
	must(r.SaveLastRead("alice", "general"))
	must(r.SaveLastRead("bob", "secret"))
	must(r.SaveLastRead("bob", "ghost")) // a channel that no longer exists

	ev := mkEvent("e1", "general", "Sync", time.Now().Add(time.Hour).Truncate(time.Millisecond), 30)
	ev.RSVP["bob"] = rsvpMaybe
	must(r.SaveEvent(ev))
	if ok, err := r.ClaimReminder("e1", "bob"); err != nil || !ok {
		t.Fatal(ok, err)
	}
}

func runMigrate(t *testing.T, pool *pgxpool.Pool, args ...string) (int, string) {
	t.Helper()
	prev := openMigrationTarget
	openMigrationTarget = func() (*pgxpool.Pool, error) { return pool, nil }
	t.Cleanup(func() { openMigrationTarget = prev })
	var out bytes.Buffer
	code := runMigrateCommand(args, strings.NewReader("yes\n"), &out)
	return code, out.String()
}

func TestMigrateDryRunReportsAndChangesNothing(t *testing.T) {
	useMiniRedis(t)
	seedLegacy(t)
	prev := openMigrationTarget
	openMigrationTarget = func() (*pgxpool.Pool, error) { t.Fatal("a dry run must not open Postgres"); return nil, nil }
	t.Cleanup(func() { openMigrationTarget = prev })

	var out bytes.Buffer
	if code := runMigrateCommand([]string{"-dry-run"}, strings.NewReader(""), &out); code != 0 {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	text := out.String()
	for _, want := range []string{"users          3 (2 with a password)", "channels       3", "messages       5 (+ 2 thread replies)",
		"pins           2", "files          1", "events         1", "Dry run"} {
		if !strings.Contains(text, want) {
			t.Errorf("report lacks %q:\n%s", want, text)
		}
	}
	if !strings.Contains(text, "read mark(s) of channels that no longer exist") {
		t.Errorf("the read mark of a deleted channel is not reported:\n%s", text)
	}
}

func TestMigrateCopiesEverythingAndVerifies(t *testing.T) {
	useMiniRedis(t)
	seedLegacy(t)
	pool := migratedTestDatabase(t)

	code, out := runMigrate(t, pool)
	if code != 0 || !strings.Contains(out, "Everything was copied and verified") {
		t.Fatalf("exit %d\n%s", code, out)
	}

	// The app must see the same thing through Postgres as it saw through Redis.
	pg := pgStore{pool: pool}
	r := redisStore{}
	for _, ch := range []string{"general", "secret", "old"} {
		a, _ := r.RecentMessages(ch, 100)
		b, err := pg.RecentMessages(ch, 100)
		if err != nil || !reflect.DeepEqual(messageIDs(a), messageIDs(b)) {
			t.Errorf("#%s: %v vs %v (%v)", ch, messageIDs(a), messageIDs(b), err)
		}
	}
	// reply counters are derived in Postgres and stored in Redis: they agree
	m2r, _ := r.FindMessage("general", "m2")
	m2p, err := pg.FindMessage("general", "m2")
	if err != nil || m2p.ReplyCount != 2 || m2r.ReplyCount != 2 || !reflect.DeepEqual(m2p.ReplyUsers, m2r.ReplyUsers) {
		t.Errorf("thread counters: %+v vs %+v (%v)", m2p, m2r, err)
	}
	// the "unread replies" base moved with the user: bob has 2 unread, alice read hers
	for _, user := range []string{"bob", "alice", "gina"} {
		a, _ := r.ThreadUnread(user, "general")
		b, err := pg.ThreadUnread(user, "general")
		if err != nil || !reflect.DeepEqual(a, b) {
			t.Errorf("unread replies of %s: redis %v, postgres %v (%v)", user, a, b, err)
		}
	}
	if u, _ := pg.ThreadUnread("bob", "general"); u["m2"] != 2 {
		t.Errorf("bob should have 2 unread replies, got %v", u)
	}
	// pins keep their order (newest first)
	pa, _ := r.PinEntries("general")
	pb, err := pg.PinEntries("general")
	if err != nil || len(pa) != 2 || len(pb) != 2 || pa[0].MessageID != pb[0].MessageID || pa[1].MessageID != pb[1].MessageID {
		t.Errorf("pins: %+v vs %+v (%v)", pa, pb, err)
	}
	if u, _ := pg.ThreadUnread("alice", "general"); u["m2"] != 1 {
		t.Errorf("alice should have 1 unread reply (gina's), got %v", u)
	}
	// accounts: login by email, case-insensitively, and by Google id
	if u, err := pg.GetUserByEmail("bob@x.com"); err != nil || u.Username != "bob" {
		t.Errorf("bob by email: %+v %v", u, err)
	}
	if h, _ := pg.PasswordHash("alice@x.com"); h != "$2a$10$alicehash" {
		t.Errorf("password hash: %q", h)
	}
	if name, found, _ := pg.UsernameByProvider("google", "g-123"); !found || name != "gina" {
		t.Errorf("google link: %q %v", name, found)
	}
	// an old channel without an id got one
	if c, err := pg.GetChannel("old"); err != nil || c.ID == "" {
		t.Errorf("channel without id: %+v %v", c, err)
	}
	// the deleted-message placeholder is not a message
	if n := mustCount(t, pool, "SELECT count(*) FROM messages WHERE thread_id IS NULL"); n != 5 {
		t.Errorf("channel messages: %d", n)
	}
	// the read mark of the missing channel was left out, the others kept
	if n := mustCount(t, pool, "SELECT count(*) FROM channel_reads"); n != 2 {
		t.Errorf("read marks: %d", n)
	}
	if ok, _ := pg.ClaimReminder("e1", "bob"); ok {
		t.Error("the reminder that was already sent would be sent again")
	}
}

func TestMigrateRefusesNonEmptyTargetUntilReset(t *testing.T) {
	useMiniRedis(t)
	seedLegacy(t)
	pool := migratedTestDatabase(t)

	if code, out := runMigrate(t, pool); code != 0 {
		t.Fatalf("first run: %d\n%s", code, out)
	}
	before := mustCount(t, pool, "SELECT count(*) FROM messages")

	code, out := runMigrate(t, pool)
	if code == 0 || !strings.Contains(out, "already contains data") {
		t.Fatalf("a second run must refuse:\n%s", out)
	}
	if mustCount(t, pool, "SELECT count(*) FROM messages") != before {
		t.Error("the refused run changed the data")
	}

	code, out = runMigrate(t, pool, "-reset", "-yes")
	if code != 0 {
		t.Fatalf("reset run: %d\n%s", code, out)
	}
	if mustCount(t, pool, "SELECT count(*) FROM messages") != before {
		t.Error("after a reset the data must be the same, not doubled")
	}
}

func TestMigrateStopsOnProblems(t *testing.T) {
	useMiniRedis(t)
	seedLegacy(t)
	// a second account with the same username
	if err := (redisStore{}).SaveUser(&User{ID: "u-other", Username: "alice", Email: "other@x.com"}); err != nil {
		t.Fatal(err)
	}
	pool := migratedTestDatabase(t)

	code, out := runMigrate(t, pool)
	if code == 0 || !strings.Contains(out, `username "alice" is used by more than one account`) {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if n := mustCount(t, pool, "SELECT count(*) FROM users"); n != 0 {
		t.Errorf("nothing may be written when there are problems, found %d users", n)
	}
}

func TestMigrateLeavesOutWhatCannotBeCopied(t *testing.T) {
	useMiniRedis(t)
	seedLegacy(t)
	r := redisStore{}
	// reply to a message that is gone, a pin on a gone message, an event of a gone channel
	if err := redisClient.SAdd(ctx, threadIndexKey("general"), "vanished").Err(); err != nil {
		t.Fatal(err)
	}
	if err := redisClient.RPush(ctx, threadKey("general", "vanished"),
		`{"id":"x1","user":"bob","text":"lost","channel":"general","threadId":"vanished","timestamp":"2026-01-01T00:00:00Z"}`).Err(); err != nil {
		t.Fatal(err)
	}
	if err := redisClient.LPush(ctx, pinsKey("general"), `{"messageId":"gone","pinnedBy":"bob","pinnedAt":"2026-01-01T00:00:00Z"}`).Err(); err != nil {
		t.Fatal(err)
	}
	if err := redisSaveEvent(mkEvent("e9", "nowhere", "orphan", time.Now().Add(time.Hour), 30)); err != nil {
		t.Fatal(err)
	}
	_ = r
	pool := migratedTestDatabase(t)

	code, out := runMigrate(t, pool)
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	for _, want := range []string{"does not exist, are dropped", "the pin of message gone is dropped", "its channel #nowhere does not exist"} {
		if !strings.Contains(out, want) {
			t.Errorf("no warning %q:\n%s", want, out)
		}
	}
	if n := mustCount(t, pool, "SELECT count(*) FROM messages WHERE thread_id IS NOT NULL"); n != 2 {
		t.Errorf("replies: %d", n)
	}
	if n := mustCount(t, pool, "SELECT count(*) FROM pins"); n != 2 {
		t.Errorf("pins: %d", n)
	}
}
