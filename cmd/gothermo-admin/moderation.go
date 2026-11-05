package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// Keep these in sync with moderation.go of the app.
const (
	reportsKey = "reports"
	bannedKey  = "banned_users"
)

type reportRec struct {
	ID        string    `json:"id"`
	Reporter  string    `json:"reporter"`
	Author    string    `json:"author"`
	Channel   string    `json:"channel"`
	MessageID string    `json:"messageId"`
	Reason    string    `json:"reason"`
	Snippet   string    `json:"snippet"`
	FileName  string    `json:"fileName,omitempty"`
	At        time.Time `json:"at"`
}

func cmdReports(ctx context.Context, rdb *redis.Client, out io.Writer, args []string) error {
	fs := flag.NewFlagSet("reports", flag.ContinueOnError)
	n := fs.Int("n", 20, "how many of the newest reports to show")
	if err := fs.Parse(args); err != nil {
		return err
	}
	raw, err := rdb.LRange(ctx, reportsKey, 0, int64(*n)-1).Result()
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		fmt.Fprintln(out, "No reports.")
		return nil
	}
	for _, s := range raw {
		var r reportRec
		if json.Unmarshal([]byte(s), &r) != nil {
			continue
		}
		fmt.Fprintf(out, "%s  [%s] %s reported by %s in #%s\n", r.At.Local().Format("2006-01-02 15:04"), r.Reason, r.Author, r.Reporter, r.Channel)
		if r.FileName != "" {
			fmt.Fprintf(out, "    file: %s\n", r.FileName)
		}
		if r.Snippet != "" {
			fmt.Fprintf(out, "    %q\n", r.Snippet)
		}
	}
	return nil
}

func cmdBan(ctx context.Context, rdb *redis.Client, out io.Writer, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: ban <username>")
	}
	name := strings.ToLower(strings.TrimSpace(args[0]))
	if name == "" {
		return errors.New("usage: ban <username>")
	}
	if err := rdb.SAdd(ctx, bannedKey, name).Err(); err != nil {
		return err
	}
	// Their sessions stop working at once (the app checks the ban list), and
	// deleting them as well keeps the list of sessions clean.
	owners, err := sessionOwners(ctx, rdb)
	if err != nil {
		return err
	}
	var toDelete []string
	for key, user := range owners {
		if strings.ToLower(user) == name {
			toDelete = append(toDelete, key)
		}
	}
	if len(toDelete) > 0 {
		if err := rdb.Del(ctx, toDelete...).Err(); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "Banned %s (%d sessions ended).\n", name, len(toDelete))
	return nil
}

func cmdUnban(ctx context.Context, rdb *redis.Client, out io.Writer, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: unban <username>")
	}
	name := strings.ToLower(strings.TrimSpace(args[0]))
	removed, err := rdb.SRem(ctx, bannedKey, name).Result()
	if err != nil {
		return err
	}
	if removed == 0 {
		fmt.Fprintf(out, "%s was not banned.\n", name)
		return nil
	}
	fmt.Fprintf(out, "Unbanned %s.\n", name)
	return nil
}

func cmdBanned(ctx context.Context, rdb *redis.Client, out io.Writer) error {
	names, err := rdb.SMembers(ctx, bannedKey).Result()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		fmt.Fprintln(out, "Nobody is banned.")
		return nil
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintln(out, n)
	}
	return nil
}
