package main

import (
	"fmt"
	"strings"
	"time"
)

// Thread extras: exporting one thread, and "unread replies" markers.
//
// Unread is kept per person. threadread:<user> is a hash with one field per
// thread ("<channel>:<parentID>") holding the time the person last opened it.
// A reply is unread when somebody else wrote it after that time. For a thread
// the person never opened, the cut-off is the moment this feature first
// looked at that person (threadbase:<user>), so old threads do not light up
// all at once.

func threadReadKey(user string) string { return "threadread:" + user }
func threadBaseKey(user string) string { return "threadbase:" + user }

func threadReadField(channel, parentID string) string { return channel + ":" + parentID }

// BuildThreadExport renders one thread (the parent first, then its replies)
// without saving it anywhere; the app saves it with SaveTextFile. Works the
// same locally and over a server.
func (a *App) BuildThreadExport(token, channel, parentID, format string) (ExportFile, error) {
	username, err := sessionUser(token)
	if err != nil {
		return ExportFile{}, err
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "json" && format != "csv" && format != "txt" {
		return ExportFile{}, fmt.Errorf("unknown export format %q", format)
	}
	if err := requireChannelAccess(username, channel); err != nil {
		return ExportFile{}, err
	}
	parent, err := findChannelMessage(channel, parentID)
	if err != nil {
		return ExportFile{}, err
	}
	replies, _, err := loadReplies(channel, parentID)
	if err != nil {
		return ExportFile{}, fmt.Errorf("error reading replies: %v", err)
	}

	msgs := append([]Message{*parent}, replies...)
	now := time.Now()
	data, err := renderExport(format, channel+" (thread)", msgs, now)
	if err != nil {
		return ExportFile{}, fmt.Errorf("error building export: %v", err)
	}
	return ExportFile{Filename: exportFilename(channel+"-thread", format, now), Content: string(data)}, nil
}

// MarkThreadRead records that the person has seen the thread up to now.
func (a *App) MarkThreadRead(token, channel, parentID string) error {
	username, err := sessionUser(token)
	if err != nil {
		return err
	}
	if err := requireChannelAccess(username, channel); err != nil {
		return err
	}
	return redisClient.HSet(ctx, threadReadKey(username), threadReadField(channel, parentID),
		time.Now().Format(time.RFC3339Nano)).Err()
}

// GetThreadUnread returns, for each thread of the channel that has unread
// replies, how many there are. Threads without unread replies are left out.
func (a *App) GetThreadUnread(token, channel string) (map[string]int, error) {
	username, err := sessionUser(token)
	if err != nil {
		return nil, err
	}
	if err := requireChannelAccess(username, channel); err != nil {
		return nil, err
	}

	// First time we look at this person: everything so far counts as seen.
	now := time.Now()
	redisClient.SetNX(ctx, threadBaseKey(username), now.Format(time.RFC3339Nano), 0)
	base := now
	if s, err := redisClient.Get(ctx, threadBaseKey(username)).Result(); err == nil {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			base = t
		}
	}

	reads, _ := redisClient.HGetAll(ctx, threadReadKey(username)).Result()
	ids, err := redisClient.SMembers(ctx, threadIndexKey(channel)).Result()
	if err != nil {
		return nil, err
	}

	out := map[string]int{}
	for _, id := range ids {
		since := base
		if s, ok := reads[threadReadField(channel, id)]; ok {
			if t, err := time.Parse(time.RFC3339Nano, s); err == nil && t.After(since) {
				since = t
			}
		}
		replies, _, err := loadReplies(channel, id)
		if err != nil {
			continue
		}
		n := 0
		for _, r := range replies {
			if r.User != username && r.Timestamp.After(since) {
				n++
			}
		}
		if n > 0 {
			out[id] = n
		}
	}
	return out, nil
}
