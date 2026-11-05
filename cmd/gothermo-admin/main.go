// gothermo-admin is a small maintenance tool for GoThermo. Sessions and login
// limits are read from Redis; accounts and uploads from Postgres when a database
// url is given (-database-url or $DATABASE_URL), otherwise from Redis.
//
//	go run ./cmd/gothermo-admin users
//	go run ./cmd/gothermo-admin locked
//	go run ./cmd/gothermo-admin unlock someone@example.com
//	go run ./cmd/gothermo-admin sessions
//	go run ./cmd/gothermo-admin revoke some-username
//	go run ./cmd/gothermo-admin reports
//	go run ./cmd/gothermo-admin ban some-username
//	go run ./cmd/gothermo-admin unban some-username
//	go run ./cmd/gothermo-admin banned
//	go run ./cmd/gothermo-admin uploads            (dry run)
//	go run ./cmd/gothermo-admin uploads --clean    (asks before deleting)
//	go run ./cmd/gothermo-admin -database-url "$DATABASE_URL" users
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/go-redis/redis/v8"
)

// Keep in sync with maxLoginFailures in the app's ratelimit.go.
const maxLoginFailures = 5

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func usage(w io.Writer) {
	fmt.Fprint(w, `GoThermo admin tool

Usage: gothermo-admin [-addr host:port] [-db N] <command> [arguments]

Commands:
  users                 list all accounts
  locked                show emails with failed password attempts
  unlock <email>        reset the failed-attempt counter of an email
  sessions              number of active sessions per user
  revoke <username>     log a user out everywhere (deletes all their sessions)
  reports [-n 20]       newest reports of messages (who, why, a snippet)
  ban <username>        block an account: no sign-in, sessions end at once
  unban <username>      let it back in
  banned                list banned accounts
  uploads [flags]       find uploaded files that no message uses
      -dir uploads      uploads directory (default "uploads", relative to where you run this)
      -min-age 24h      never touch files newer than this
      -clean            delete the orphans found (asks first)
      -yes              do not ask

Global flags:
`)
}

func run(args []string, in io.Reader, out, errOut io.Writer) int {
	global := flag.NewFlagSet("gothermo-admin", flag.ContinueOnError)
	global.SetOutput(errOut)
	global.Usage = func() { usage(errOut); global.PrintDefaults() }
	addr := global.String("addr", "localhost:6379", "Redis address")
	db := global.Int("db", 0, "Redis database number")
	storeMode := global.String("store", os.Getenv("GOTHERMO_STORE"), "where accounts and messages are: redis or postgres (default: postgres when a database url is given)")
	databaseURL := global.String("database-url", os.Getenv("DATABASE_URL"), "Postgres url (default: $DATABASE_URL)")
	if err := global.Parse(args); err != nil {
		return 2
	}
	rest := global.Args()
	if len(rest) == 0 {
		global.Usage()
		return 2
	}

	rdb := redis.NewClient(&redis.Options{Addr: *addr, DB: *db})
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		fmt.Fprintf(errOut, "cannot connect to Redis at %s: %v\n", *addr, err)
		return 1
	}

	// The accounts and messages may live in Postgres; only the commands that
	// read them connect to it.
	var err error
	withSource := func(fn func(src dataSource) error) error {
		src, closeSrc, err := chooseSource(ctx, *storeMode, *databaseURL, rdb, *addr)
		if err != nil {
			return err
		}
		defer closeSrc()
		return fn(src)
	}
	switch cmd, cargs := rest[0], rest[1:]; cmd {
	case "users":
		err = withSource(func(src dataSource) error { return cmdUsers(ctx, src, out) })
	case "locked":
		err = cmdLocked(ctx, rdb, out)
	case "unlock":
		err = cmdUnlock(ctx, rdb, out, cargs)
	case "sessions":
		err = cmdSessions(ctx, rdb, out)
	case "revoke":
		err = cmdRevoke(ctx, rdb, out, cargs)
	case "reports":
		err = cmdReports(ctx, rdb, out, cargs)
	case "ban":
		err = cmdBan(ctx, rdb, out, cargs)
	case "unban":
		err = cmdUnban(ctx, rdb, out, cargs)
	case "banned":
		err = cmdBanned(ctx, rdb, out)
	case "uploads":
		err = withSource(func(src dataSource) error { return cmdUploads(ctx, src, in, out, errOut, cargs) })
	case "help":
		global.Usage()
	default:
		fmt.Fprintf(errOut, "unknown command %q\n\n", cmd)
		global.Usage()
		return 2
	}
	if err != nil {
		fmt.Fprintln(errOut, "error:", err)
		return 1
	}
	return 0
}

// ---------- helpers ----------

func newTable(out io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
}

// scan lists keys with SCAN (never blocks Redis like KEYS can).
func scan(ctx context.Context, rdb *redis.Client, pattern string) ([]string, error) {
	var (
		keys   []string
		cursor uint64
	)
	for {
		batch, next, err := rdb.Scan(ctx, cursor, pattern, 500).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		cursor = next
		if cursor == 0 {
			return keys, nil
		}
	}
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

func loginFailKey(email string) string {
	return "loginfail:" + strings.ToLower(strings.TrimSpace(email))
}

// ---------- users ----------

type userRec struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Provider string `json:"provider"`
	Status   string `json:"status"`
	LastSeen string `json:"lastSeen"`
}

func cmdUsers(ctx context.Context, src dataSource, out io.Writer) error {
	users, err := src.Users(ctx)
	if err != nil {
		return err
	}
	sort.Slice(users, func(i, j int) bool { return strings.ToLower(users[i].Username) < strings.ToLower(users[j].Username) })

	tw := newTable(out)
	fmt.Fprintln(tw, "USERNAME\tEMAIL\tSIGN-IN\tSTATUS\tLAST SEEN")
	for _, u := range users {
		provider := u.Provider
		if provider == "" {
			provider = "password"
		}
		last := "-"
		if t, err := time.Parse(time.RFC3339, u.LastSeen); err == nil {
			last = t.Local().Format("2006-01-02 15:04")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", u.Username, u.Email, provider, u.Status, last)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(out, "\n%d accounts\n", len(users))
	return nil
}

// ---------- locked / unlock ----------

func cmdLocked(ctx context.Context, rdb *redis.Client, out io.Writer) error {
	keys, err := scan(ctx, rdb, "loginfail:*")
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		fmt.Fprintln(out, "No failed login attempts recorded.")
		return nil
	}

	type row struct {
		email string
		fails int
		ttl   time.Duration
	}
	var rows []row
	for _, key := range keys {
		n, err := rdb.Get(ctx, key).Int()
		if err != nil {
			continue
		}
		ttl, _ := rdb.TTL(ctx, key).Result()
		rows = append(rows, row{strings.TrimPrefix(key, "loginfail:"), n, ttl})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].fails > rows[j].fails })

	tw := newTable(out)
	fmt.Fprintln(tw, "EMAIL\tFAILS\tSTATE\tRESETS IN")
	for _, r := range rows {
		state := "counting"
		if r.fails >= maxLoginFailures {
			state = "LOCKED"
		}
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", r.email, r.fails, state, r.ttl.Round(time.Second))
	}
	return tw.Flush()
}

func cmdUnlock(ctx context.Context, rdb *redis.Client, out io.Writer, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: unlock <email>")
	}
	n, err := rdb.Del(ctx, loginFailKey(args[0])).Result()
	if err != nil {
		return err
	}
	if n == 0 {
		fmt.Fprintf(out, "%s had no failed attempts recorded.\n", args[0])
	} else {
		fmt.Fprintf(out, "%s unlocked.\n", args[0])
	}
	return nil
}

// ---------- sessions / revoke ----------

// sessionOwners maps session key -> username. Tokens are never printed.
func sessionOwners(ctx context.Context, rdb *redis.Client) (map[string]string, error) {
	keys, err := scan(ctx, rdb, "session:*")
	if err != nil {
		return nil, err
	}
	owners := make(map[string]string, len(keys))
	const batch = 500
	for i := 0; i < len(keys); i += batch {
		end := i + batch
		if end > len(keys) {
			end = len(keys)
		}
		vals, err := rdb.MGet(ctx, keys[i:end]...).Result()
		if err != nil {
			return nil, err
		}
		for j, v := range vals {
			if s, ok := v.(string); ok {
				owners[keys[i+j]] = s
			}
		}
	}
	return owners, nil
}

func cmdSessions(ctx context.Context, rdb *redis.Client, out io.Writer) error {
	owners, err := sessionOwners(ctx, rdb)
	if err != nil {
		return err
	}
	perUser := map[string]int{}
	for _, u := range owners {
		perUser[u]++
	}
	names := make([]string, 0, len(perUser))
	for u := range perUser {
		names = append(names, u)
	}
	sort.Strings(names)

	tw := newTable(out)
	fmt.Fprintln(tw, "USERNAME\tSESSIONS")
	for _, u := range names {
		fmt.Fprintf(tw, "%s\t%d\n", u, perUser[u])
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(out, "\n%d sessions, %d users\n", len(owners), len(names))
	return nil
}

func cmdRevoke(ctx context.Context, rdb *redis.Client, out io.Writer, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: revoke <username>")
	}
	owners, err := sessionOwners(ctx, rdb)
	if err != nil {
		return err
	}
	var toDelete []string
	for key, user := range owners {
		if user == args[0] {
			toDelete = append(toDelete, key)
		}
	}
	if len(toDelete) > 0 {
		if err := rdb.Del(ctx, toDelete...).Err(); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "Revoked %d sessions of %s.\n", len(toDelete), args[0])
	return nil
}

// ---------- uploads ----------

// The app stores uploads as "<uuid>_<original name>".
func fileIDFromName(name string) (string, bool) {
	if len(name) <= 37 || name[36] != '_' {
		return "", false
	}
	id := name[:36]
	for i, r := range id {
		switch {
		case i == 8 || i == 13 || i == 18 || i == 23:
			if r != '-' {
				return "", false
			}
		case !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F'):
			return "", false
		}
	}
	return id, true
}

type fileRec struct {
	path   string
	id     string
	size   int64
	mod    time.Time
	status string // in use | ORPHAN | recent | unknown
}

func cmdUploads(ctx context.Context, src dataSource, in io.Reader, out, errOut io.Writer, args []string) error {
	flags := flag.NewFlagSet("uploads", flag.ContinueOnError)
	flags.SetOutput(errOut)
	dir := flags.String("dir", "uploads", "uploads directory")
	minAge := flags.Duration("min-age", 24*time.Hour, "never touch files newer than this")
	clean := flags.Bool("clean", false, "delete orphaned files")
	yes := flags.Bool("yes", false, "do not ask before deleting")
	if err := flags.Parse(args); err != nil {
		return err
	}

	absDir, err := filepath.Abs(*dir)
	if err != nil {
		return err
	}
	if st, err := os.Stat(absDir); err != nil || !st.IsDir() {
		return fmt.Errorf("%s is not a directory (run this from the project folder or use -dir)", absDir)
	}

	referenced, err := src.ReferencedFileIDs(ctx)
	if err != nil {
		return err
	}

	// Walk the directory tree (files only, symlinks are never followed).
	var files []fileRec
	onDisk := map[string]bool{}
	err = filepath.WalkDir(absDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rec := fileRec{path: path, size: info.Size(), mod: info.ModTime()}
		id, ok := fileIDFromName(d.Name())
		rec.id = id
		switch {
		case !ok:
			rec.status = "unknown"
		case referenced[id]:
			rec.status = "in use"
			onDisk[id] = true
		case time.Since(rec.mod) < *minAge:
			rec.status = "recent"
		default:
			rec.status = "ORPHAN"
		}
		files = append(files, rec)
		return nil
	})
	if err != nil {
		return err
	}

	// File records without a message and without a file.
	metas, err := src.FileMetas(ctx)
	if err != nil {
		return err
	}
	var staleMeta []string
	orphanIDs := map[string]bool{}
	for _, f := range files {
		if f.status == "ORPHAN" {
			orphanIDs[f.id] = true
		}
	}
	for _, m := range metas {
		if referenced[m.ID] || orphanIDs[m.ID] {
			continue
		}
		if m.Path == "" {
			staleMeta = append(staleMeta, m.ID)
			continue
		}
		if abs, err := filepath.Abs(m.Path); err == nil {
			if _, err := os.Stat(abs); errors.Is(err, fs.ErrNotExist) {
				staleMeta = append(staleMeta, m.ID)
			}
		}
	}

	// Messages whose file is gone.
	broken := 0
	for id := range referenced {
		if !onDisk[id] {
			broken++
		}
	}

	var inUse, orphans, recent, unknown int
	var inUseSize, orphanSize int64
	for _, f := range files {
		switch f.status {
		case "in use":
			inUse++
			inUseSize += f.size
		case "ORPHAN":
			orphans++
			orphanSize += f.size
		case "recent":
			recent++
		case "unknown":
			unknown++
		}
	}

	fmt.Fprintf(out, "Uploads directory: %s\nData: %s\n\n", absDir, src.Label())
	tw := newTable(out)
	fmt.Fprintln(tw, "STATUS\tFILE\tSIZE\tMODIFIED")
	shown := 0
	for _, f := range files {
		if f.status == "in use" {
			continue
		}
		rel, _ := filepath.Rel(absDir, f.path)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", f.status, rel, humanSize(f.size), f.mod.Local().Format("2006-01-02 15:04"))
		shown++
	}
	if shown > 0 {
		tw.Flush()
		fmt.Fprintln(out)
	}
	fmt.Fprintf(out, "In use:           %d files (%s)\n", inUse, humanSize(inUseSize))
	fmt.Fprintf(out, "Orphans:          %d files (%s)\n", orphans, humanSize(orphanSize))
	fmt.Fprintf(out, "Too new to judge: %d files (younger than %s)\n", recent, *minAge)
	fmt.Fprintf(out, "Unknown names:    %d files (never touched)\n", unknown)
	fmt.Fprintf(out, "Stale metadata:   %d entries\n", len(staleMeta))
	fmt.Fprintf(out, "Broken messages:  %d (message points to a missing file)\n", broken)

	if orphans == 0 && len(staleMeta) == 0 {
		return nil
	}
	if !*clean {
		fmt.Fprintln(out, "\nDry run. Add -clean to delete the orphans.")
		return nil
	}

	if !*yes {
		fmt.Fprintf(out, "\nDelete %d files (%s) and %d metadata entries from %s using %s? [y/N] ",
			orphans, humanSize(orphanSize), len(staleMeta), absDir, src.Label())
		answer, _ := bufio.NewReader(in).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
			fmt.Fprintln(out, "Cancelled.")
			return nil
		}
	}

	deleted := 0
	drop := append([]string{}, staleMeta...)
	for _, f := range files {
		if f.status != "ORPHAN" {
			continue
		}
		if err := os.Remove(f.path); err != nil {
			fmt.Fprintf(errOut, "could not delete %s: %v\n", f.path, err)
			continue
		}
		drop = append(drop, f.id)
		deleted++
	}
	if err := src.DeleteFileMetas(ctx, drop); err != nil {
		return err
	}
	fmt.Fprintf(out, "Deleted %d files and %d metadata entries.\n", deleted, len(staleMeta))
	return nil
}
