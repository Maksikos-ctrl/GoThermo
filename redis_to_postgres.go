package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// One-off move of the durable data from Redis to Postgres:
//
//	gothermo-server migrate -dry-run     read Redis, report, change nothing
//	gothermo-server migrate              copy everything, then verify it
//	gothermo-server migrate -reset -yes  empty the Postgres data first, then copy
//
// Redis is only read. Sessions, login limits, link previews, typing and
// presence stay in Redis and are not part of this.

type lastReadRow struct {
	User, Channel string
	At            time.Time
}

type threadReadRow struct {
	User, Channel, Parent string
	At                    time.Time
}

type fileRow struct{ ID, Path, MimeType, Name string }

// legacyData is everything read from Redis, in the order it will be written.
type legacyData struct {
	users       []User
	passwords   map[string]string // email -> bcrypt hash
	channels    []Channel
	messages    map[string][]Message            // channel -> messages, oldest first
	replies     map[string]map[string][]Message // channel -> parent id -> replies, oldest first
	pins        map[string][]pinEntry           // channel -> pins, oldest first
	files       []fileRow
	lastReads   []lastReadRow
	threadReads []threadReadRow
	threadBase  map[string]time.Time // user -> start of the unread-replies feature
	events      []Event
	reminders   map[string][]string // event id -> users already reminded

	problems []string // stop the migration
	warnings []string // something is left out, the rest goes on
}

func (d *legacyData) problem(format string, a ...any) {
	d.problems = append(d.problems, fmt.Sprintf(format, a...))
}
func (d *legacyData) warn(format string, a ...any) {
	d.warnings = append(d.warnings, fmt.Sprintf(format, a...))
}

func (d *legacyData) countMessages() (messages, replies int) {
	for _, list := range d.messages {
		messages += len(list)
	}
	for _, byParent := range d.replies {
		for _, list := range byParent {
			replies += len(list)
		}
	}
	return
}

func parseStamp(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, s)
	return t, err == nil
}

// readLegacy loads everything from Redis. It writes nothing.
func readLegacy() (*legacyData, error) {
	d := &legacyData{
		passwords:  map[string]string{},
		messages:   map[string][]Message{},
		replies:    map[string]map[string][]Message{},
		pins:       map[string][]pinEntry{},
		threadBase: map[string]time.Time{},
		reminders:  map[string][]string{},
	}

	users, err := redisListUsers()
	if err != nil {
		return nil, fmt.Errorf("users: %w", err)
	}
	d.users = users
	for _, u := range users {
		if u.Provider != "" {
			continue
		}
		if hash, err := redisGetPassword(u.Email); err == nil && hash != "" {
			d.passwords[u.Email] = hash
		}
	}

	channels, err := redisListChannels()
	if err != nil {
		return nil, fmt.Errorf("channels: %w", err)
	}
	sort.SliceStable(channels, func(i, j int) bool { return channels[i].CreatedAt.Before(channels[j].CreatedAt) })
	d.channels = channels

	for _, c := range channels {
		msgs, err := redisGetMessages(c.Name, 1<<40)
		if err != nil {
			return nil, fmt.Errorf("messages of #%s: %w", c.Name, err)
		}
		d.messages[c.Name] = msgs

		parents, err := redisClient.SMembers(ctx, threadIndexKey(c.Name)).Result()
		if err != nil {
			return nil, fmt.Errorf("threads of #%s: %w", c.Name, err)
		}
		sort.Strings(parents)
		for _, parent := range parents {
			list, _, err := loadReplies(c.Name, parent)
			if err != nil {
				return nil, fmt.Errorf("thread %s of #%s: %w", parent, c.Name, err)
			}
			if len(list) == 0 {
				continue
			}
			if d.replies[c.Name] == nil {
				d.replies[c.Name] = map[string][]Message{}
			}
			d.replies[c.Name][parent] = list
		}

		entries, _, err := loadPinEntries(c.Name)
		if err != nil {
			return nil, fmt.Errorf("pins of #%s: %w", c.Name, err)
		}
		for i := len(entries) - 1; i >= 0; i-- { // stored newest first
			d.pins[c.Name] = append(d.pins[c.Name], entries[i])
		}
	}

	fileKeys, err := scanAllKeys("file:*")
	if err != nil {
		return nil, fmt.Errorf("files: %w", err)
	}
	sort.Strings(fileKeys)
	for _, key := range fileKeys {
		id := strings.TrimPrefix(key, "file:")
		path, mime, name, err := redisGetFileMeta(id)
		if err != nil {
			continue
		}
		d.files = append(d.files, fileRow{id, path, mime, name})
	}

	readKeys, err := scanAllKeys("user:*:lastread:*")
	if err != nil {
		return nil, fmt.Errorf("read marks: %w", err)
	}
	sort.Strings(readKeys)
	for _, key := range readKeys {
		rest := strings.TrimPrefix(key, "user:")
		i := strings.Index(rest, ":lastread:")
		if i < 0 {
			continue
		}
		val, err := redisClient.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		if t, ok := parseStamp(val); ok {
			d.lastReads = append(d.lastReads, lastReadRow{rest[:i], rest[i+len(":lastread:"):], t})
		} else {
			d.warn("read mark %q has an unreadable time and is dropped", key)
		}
	}

	trKeys, err := scanAllKeys("threadread:*")
	if err != nil {
		return nil, fmt.Errorf("thread read marks: %w", err)
	}
	sort.Strings(trKeys)
	for _, key := range trKeys {
		user := strings.TrimPrefix(key, "threadread:")
		fields, err := redisClient.HGetAll(ctx, key).Result()
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(fields))
		for f := range fields {
			names = append(names, f)
		}
		sort.Strings(names)
		for _, f := range names {
			i := strings.LastIndex(f, ":")
			t, ok := parseStamp(fields[f])
			if i < 0 || !ok {
				d.warn("thread read mark %q of %s is unreadable and is dropped", f, user)
				continue
			}
			d.threadReads = append(d.threadReads, threadReadRow{user, f[:i], f[i+1:], t})
		}
	}
	baseKeys, err := scanAllKeys("threadbase:*")
	if err != nil {
		return nil, fmt.Errorf("thread bases: %w", err)
	}
	for _, key := range baseKeys {
		if val, err := redisClient.Get(ctx, key).Result(); err == nil {
			if t, ok := parseStamp(val); ok {
				d.threadBase[strings.TrimPrefix(key, "threadbase:")] = t
			}
		}
	}

	ids, err := redisClient.ZRange(ctx, eventsIndexKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("events: %w", err)
	}
	for _, id := range ids {
		ev, err := redisLoadEvent(id)
		if err != nil {
			d.warn("event %s is in the index but unreadable and is dropped", id)
			continue
		}
		d.events = append(d.events, *ev)
		if who, err := redisClient.SMembers(ctx, eventRemindedKey(id)).Result(); err == nil && len(who) > 0 {
			sort.Strings(who)
			d.reminders[id] = who
		}
	}
	return d, nil
}

// check finds what cannot be copied as it is. Problems stop the migration,
// warnings only name what is left out. It also removes the left-out items so
// that the write step and the verification see exactly what will be copied.
func (d *legacyData) check() {
	d.problems = nil

	usernames, emails := map[string]bool{}, map[string]bool{}
	for _, u := range d.users {
		switch {
		case u.ID == "":
			d.problem("user %q has no id", u.Username)
		case u.Username == "" || u.Email == "":
			d.problem("user %q <%s> has an empty username or email", u.Username, u.Email)
		}
		if usernames[u.Username] {
			d.problem("username %q is used by more than one account", u.Username)
		}
		usernames[u.Username] = true
		if u.Provider == "" {
			key := strings.ToLower(u.Email)
			if emails[key] {
				d.problem("email %q is used by more than one password account", u.Email)
			}
			emails[key] = true
		}
	}

	channelNames := map[string]bool{}
	for i := range d.channels {
		c := &d.channels[i]
		if c.ID == "" {
			c.ID = uuid.New().String()
		}
		channelNames[c.Name] = true
	}

	seen := map[string]bool{}
	for _, c := range d.channels {
		kept := d.messages[c.Name][:0:0]
		for _, m := range d.messages[c.Name] {
			if m.ID == "" || seen[m.ID] {
				d.warn("#%s: message %q is a duplicate or has no id, the copy is dropped", c.Name, m.ID)
				continue
			}
			seen[m.ID] = true
			m.Channel, m.ThreadID = c.Name, ""
			kept = append(kept, m)
		}
		d.messages[c.Name] = kept
	}
	parentOf := map[string]string{} // message id -> channel, for non-replies
	for ch, list := range d.messages {
		for _, m := range list {
			parentOf[m.ID] = ch
		}
	}
	for _, c := range d.channels {
		parents := make([]string, 0, len(d.replies[c.Name]))
		for p := range d.replies[c.Name] {
			parents = append(parents, p)
		}
		sort.Strings(parents)
		for _, parent := range parents {
			list := d.replies[c.Name][parent]
			if parentOf[parent] != c.Name {
				d.warn("#%s: %d repl(ies) to message %s, which does not exist, are dropped", c.Name, len(list), parent)
				delete(d.replies[c.Name], parent)
				continue
			}
			kept := list[:0:0]
			for _, r := range list {
				if r.ID == "" || seen[r.ID] {
					d.warn("#%s: reply %q is a duplicate or has no id, the copy is dropped", c.Name, r.ID)
					continue
				}
				seen[r.ID] = true
				r.Channel, r.ThreadID = c.Name, parent
				kept = append(kept, r)
			}
			d.replies[c.Name][parent] = kept
		}
	}
	for ch, list := range d.messages {
		if !channelNames[ch] {
			delete(d.messages, ch)
		}
		_ = list
	}

	for _, c := range d.channels {
		kept := d.pins[c.Name][:0:0]
		pinned := map[string]bool{}
		for _, p := range d.pins[c.Name] {
			if parentOf[p.MessageID] != c.Name {
				d.warn("#%s: the pin of message %s is dropped, the message does not exist", c.Name, p.MessageID)
				continue
			}
			if pinned[p.MessageID] {
				continue
			}
			pinned[p.MessageID] = true
			kept = append(kept, p)
		}
		d.pins[c.Name] = kept
	}

	reads := d.lastReads[:0:0]
	for _, r := range d.lastReads {
		if channelNames[r.Channel] {
			reads = append(reads, r)
		}
	}
	if n := len(d.lastReads) - len(reads); n > 0 {
		d.warn("%d read mark(s) of channels that no longer exist are dropped", n)
	}
	d.lastReads = reads

	trs := d.threadReads[:0:0]
	for _, r := range d.threadReads {
		if parentOf[r.Parent] == r.Channel && r.Channel != "" {
			trs = append(trs, r)
		}
	}
	if n := len(d.threadReads) - len(trs); n > 0 {
		d.warn("%d thread read mark(s) of threads that no longer exist are dropped", n)
	}
	d.threadReads = trs

	events := d.events[:0:0]
	for _, ev := range d.events {
		switch {
		case !channelNames[ev.Channel]:
			d.warn("event %q: its channel #%s does not exist, the event is dropped", ev.Title, ev.Channel)
		case ev.Title == "" || !ev.End.After(ev.Start):
			d.warn("event %s has no title or ends before it starts, the event is dropped", ev.ID)
		default:
			for u, status := range ev.RSVP {
				if status != rsvpGoing && status != rsvpMaybe && status != rsvpDeclined {
					d.warn("event %q: answer %q of %s is dropped", ev.Title, status, u)
					delete(ev.RSVP, u)
				}
			}
			events = append(events, ev)
		}
	}
	d.events = events
}

func (d *legacyData) summary(w io.Writer) {
	msgs, replies := d.countMessages()
	pins := 0
	for _, p := range d.pins {
		pins += len(p)
	}
	fmt.Fprintf(w, "  users          %d (%d with a password)\n", len(d.users), len(d.passwords))
	fmt.Fprintf(w, "  channels       %d\n", len(d.channels))
	fmt.Fprintf(w, "  messages       %d (+ %d thread replies)\n", msgs, replies)
	fmt.Fprintf(w, "  pins           %d\n", pins)
	fmt.Fprintf(w, "  files          %d\n", len(d.files))
	fmt.Fprintf(w, "  read marks     %d (+ %d for threads)\n", len(d.lastReads), len(d.threadReads))
	fmt.Fprintf(w, "  events         %d\n", len(d.events))
}

// ───────────────────────── writing ─────────────────────────

func pgCount(pool *pgxpool.Pool, query string) (int, error) {
	var n int
	err := pool.QueryRow(context.Background(), query).Scan(&n)
	return n, err
}

// targetIsEmpty says whether the Postgres data tables hold nothing yet.
func targetIsEmpty(pool *pgxpool.Pool) (bool, error) {
	n, err := pgCount(pool, `SELECT (SELECT count(*) FROM users) + (SELECT count(*) FROM channels)
		+ (SELECT count(*) FROM messages) + (SELECT count(*) FROM files)`)
	return n == 0, err
}

func resetTarget(pool *pgxpool.Pool) error {
	_, err := pool.Exec(context.Background(), "TRUNCATE users, channels, files RESTART IDENTITY CASCADE")
	return err
}

func writePostgres(pool *pgxpool.Pool, d *legacyData) error {
	pg := pgStore{pool: pool}
	for _, u := range d.users {
		u := u
		if err := pg.SaveUser(&u); err != nil {
			return fmt.Errorf("user %s: %w", u.Username, err)
		}
	}
	for email, hash := range d.passwords {
		if err := pg.SetPasswordHash(email, hash); err != nil {
			return fmt.Errorf("password of %s: %w", email, err)
		}
	}
	for user, t := range d.threadBase {
		if _, err := pool.Exec(ctx, "UPDATE users SET thread_unread_since = $2 WHERE username = $1", user, t); err != nil {
			return fmt.Errorf("thread base of %s: %w", user, err)
		}
	}
	for _, c := range d.channels {
		if err := pg.SaveChannel(c); err != nil {
			return fmt.Errorf("channel #%s: %w", c.Name, err)
		}
	}
	for _, c := range d.channels {
		for _, m := range d.messages[c.Name] {
			if err := pg.AppendMessage(m); err != nil {
				return fmt.Errorf("message %s in #%s: %w", m.ID, c.Name, err)
			}
		}
		parents := make([]string, 0, len(d.replies[c.Name]))
		for p := range d.replies[c.Name] {
			parents = append(parents, p)
		}
		sort.Strings(parents)
		for _, parent := range parents {
			for _, r := range d.replies[c.Name][parent] {
				if err := pg.AddReply(r); err != nil {
					return fmt.Errorf("reply %s in #%s: %w", r.ID, c.Name, err)
				}
			}
		}
		for _, p := range d.pins[c.Name] {
			if err := pg.AddPin(c.Name, p); err != nil {
				return fmt.Errorf("pin %s in #%s: %w", p.MessageID, c.Name, err)
			}
		}
	}
	for _, f := range d.files {
		if err := pg.SaveFileMeta(f.ID, f.Path, f.MimeType, f.Name); err != nil {
			return fmt.Errorf("file %s: %w", f.ID, err)
		}
	}
	for _, r := range d.lastReads {
		if _, err := pool.Exec(ctx, `
			INSERT INTO channel_reads (username, channel_id, last_read_at)
			SELECT $1, id, $3 FROM channels WHERE name = $2
			ON CONFLICT (username, channel_id) DO UPDATE SET last_read_at = EXCLUDED.last_read_at`,
			r.User, r.Channel, r.At); err != nil {
			return fmt.Errorf("read mark of %s in #%s: %w", r.User, r.Channel, err)
		}
	}
	for _, r := range d.threadReads {
		if _, err := pool.Exec(ctx, `
			INSERT INTO thread_reads (username, thread_id, read_at) VALUES ($1, $2, $3)
			ON CONFLICT (username, thread_id) DO UPDATE SET read_at = EXCLUDED.read_at`,
			r.User, r.Parent, r.At); err != nil {
			return fmt.Errorf("thread read mark of %s: %w", r.User, err)
		}
	}
	for i := range d.events {
		ev := d.events[i]
		if err := pg.SaveEvent(&ev); err != nil {
			return fmt.Errorf("event %q: %w", ev.Title, err)
		}
		for _, who := range d.reminders[ev.ID] {
			if _, err := pg.ClaimReminder(ev.ID, who); err != nil {
				return fmt.Errorf("reminder of event %q: %w", ev.Title, err)
			}
		}
	}
	return nil
}

// ───────────────────────── verifying ─────────────────────────

func closeTo(a, b time.Time) bool {
	d := a.Sub(b)
	if d < 0 {
		d = -d
	}
	return d < time.Millisecond
}

func messageIDs(list []Message) []string {
	ids := make([]string, 0, len(list))
	for _, m := range list {
		ids = append(ids, m.ID)
	}
	return ids
}

// verifyPostgres reads everything back through the Store interface and
// compares it with what was meant to be copied. It returns the differences.
func verifyPostgres(pool *pgxpool.Pool, d *legacyData) []string {
	pg := pgStore{pool: pool}
	var diffs []string
	add := func(format string, a ...any) { diffs = append(diffs, fmt.Sprintf(format, a...)) }

	got, err := pg.ListUsers()
	if err != nil || len(got) != len(d.users) {
		add("users: %d in Postgres, %d expected (%v)", len(got), len(d.users), err)
	}
	for email, hash := range d.passwords {
		if h, err := pg.PasswordHash(email); err != nil || h != hash {
			add("password hash of %s differs", email)
		}
	}
	for _, u := range d.users {
		if u.Provider != "" {
			if name, found, _ := pg.UsernameByProvider(u.Provider, u.ProviderID); !found || name != u.Username {
				add("%s account of %s is not found by its provider id", u.Provider, u.Username)
			}
		}
	}

	for _, c := range d.channels {
		ch, err := pg.GetChannel(c.Name)
		if err != nil {
			add("channel #%s is missing: %v", c.Name, err)
			continue
		}
		if ch.IsPrivate != c.IsPrivate || ch.CreatedBy != c.CreatedBy || ch.Description != c.Description ||
			!reflect.DeepEqual(append([]string{}, ch.Members...), append([]string{}, c.Members...)) {
			add("channel #%s differs: %+v vs %+v", c.Name, ch, c)
		}

		msgs, err := pg.RecentMessages(c.Name, 1<<40)
		want := d.messages[c.Name]
		if err != nil || !reflect.DeepEqual(messageIDs(msgs), messageIDs(want)) {
			add("#%s: the message list differs (%d in Postgres, %d expected) %v", c.Name, len(msgs), len(want), err)
			continue
		}
		for i, m := range want {
			g := msgs[i]
			if g.Text != m.Text || g.User != m.User || !closeTo(g.Timestamp, m.Timestamp) ||
				g.IsEdited != m.IsEdited || g.FileID != m.FileID || g.IsCall != m.IsCall {
				add("#%s: message %s differs", c.Name, m.ID)
			}
			if len(m.Reactions) != len(g.Reactions) {
				add("#%s: reactions of message %s differ", c.Name, m.ID)
				continue
			}
			for emoji, users := range m.Reactions {
				if !reflect.DeepEqual(append([]string{}, g.Reactions[emoji]...), append([]string{}, users...)) {
					add("#%s: reaction %s of message %s differs", c.Name, emoji, m.ID)
				}
			}
		}
		for parent, list := range d.replies[c.Name] {
			rs, err := pg.Replies(c.Name, parent)
			if err != nil || !reflect.DeepEqual(messageIDs(rs), messageIDs(list)) {
				add("#%s: the thread of %s differs (%v)", c.Name, parent, err)
			}
		}
		pins, err := pg.PinEntries(c.Name)
		var gotPins, wantPins []string
		for i := len(pins) - 1; i >= 0; i-- { // Postgres answers newest first
			gotPins = append(gotPins, pins[i].MessageID)
		}
		for _, p := range d.pins[c.Name] {
			wantPins = append(wantPins, p.MessageID)
		}
		if err != nil || !reflect.DeepEqual(gotPins, wantPins) {
			add("#%s: pins differ: %v vs %v (%v)", c.Name, gotPins, wantPins, err)
		}
	}

	for _, f := range d.files {
		path, mime, name, err := pg.FileMeta(f.ID)
		if err != nil || path != f.Path || mime != f.MimeType || name != f.Name {
			add("file %s differs", f.ID)
		}
	}
	for _, r := range d.lastReads {
		if t, found, err := pg.LastRead(r.User, r.Channel); err != nil || !found || !closeTo(t, r.At) {
			add("read mark of %s in #%s differs", r.User, r.Channel)
		}
	}
	for _, ev := range d.events {
		g, err := pg.GetEvent(ev.ID)
		if err != nil || g.Title != ev.Title || g.Channel != ev.Channel || !closeTo(g.Start, ev.Start) ||
			!closeTo(g.End, ev.End) || !reflect.DeepEqual(g.RSVP, ev.RSVP) {
			add("event %q differs (%v)", ev.Title, err)
		}
	}
	return diffs
}

// ───────────────────────── the command ─────────────────────────

// openMigrationTarget connects to the Postgres named by DATABASE_URL (and
// brings its schema up to date). Tests replace it.
var openMigrationTarget = func() (*pgxpool.Pool, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}
	return openPostgres(context.Background(), url)
}

// runMigrateCommand is `gothermo-server migrate`; the caller has already
// connected to Redis. It returns the exit code.
func runMigrateCommand(args []string, in io.Reader, out io.Writer) int {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(out)
	dryRun := fs.Bool("dry-run", false, "read Redis and report, change nothing")
	reset := fs.Bool("reset", false, "empty the Postgres data tables before copying")
	yes := fs.Bool("yes", false, "do not ask for confirmation")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	fmt.Fprintln(out, "Reading Redis …")
	d, err := readLegacy()
	if err != nil {
		fmt.Fprintln(out, "❌ could not read Redis:", err)
		return 1
	}
	d.check()
	fmt.Fprintln(out, "Found in Redis:")
	d.summary(out)
	for _, w := range d.warnings {
		fmt.Fprintln(out, "⚠️ ", w)
	}
	for _, p := range d.problems {
		fmt.Fprintln(out, "❌", p)
	}
	if len(d.problems) > 0 {
		fmt.Fprintln(out, "\nFix the problems above in Redis first (nothing was changed).")
		return 1
	}
	if *dryRun {
		fmt.Fprintln(out, "\nDry run: nothing was written. Run without -dry-run to copy.")
		return 0
	}

	pool, err := openMigrationTarget()
	if err != nil {
		fmt.Fprintln(out, "❌", err)
		return 1
	}

	empty, err := targetIsEmpty(pool)
	if err != nil {
		fmt.Fprintln(out, "❌", err)
		return 1
	}
	if !empty {
		if !*reset {
			fmt.Fprintln(out, "❌ Postgres already contains data. Nothing was copied.")
			fmt.Fprintln(out, "   If it is a failed earlier attempt, run again with -reset to empty it first.")
			return 1
		}
		if !*yes {
			fmt.Fprint(out, "This DELETES all users, channels, messages and files in Postgres first. Type yes: ")
			var answer string
			fmt.Fscanln(in, &answer)
			if strings.TrimSpace(answer) != "yes" {
				fmt.Fprintln(out, "Cancelled.")
				return 1
			}
		}
		if err := resetTarget(pool); err != nil {
			fmt.Fprintln(out, "❌", err)
			return 1
		}
	}

	fmt.Fprintln(out, "Copying …")
	if err := writePostgres(pool, d); err != nil {
		fmt.Fprintln(out, "❌ copy failed:", err)
		fmt.Fprintln(out, "   Redis is untouched. Run again with -reset to start over.")
		return 1
	}
	fmt.Fprintln(out, "Verifying …")
	if diffs := verifyPostgres(pool, d); len(diffs) > 0 {
		for _, x := range diffs {
			fmt.Fprintln(out, "❌", x)
		}
		fmt.Fprintln(out, "\nThe copy does not match Redis. Do NOT switch over; send me this output.")
		return 1
	}
	fmt.Fprintln(out, "✅ Everything was copied and verified. Redis still has the original data.")
	fmt.Fprintln(out, "   Start the server as usual: with DATABASE_URL set it now runs on Postgres.")
	return 0
}
