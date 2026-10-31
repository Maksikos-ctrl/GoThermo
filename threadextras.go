package main

import (
	"fmt"
	"strings"
	"time"
)

// Thread extras: exporting one thread, and "unread replies" markers.
//
// Unread is kept per person: the time they last opened each thread. A reply is
// unread when somebody else wrote it after that time. For a thread the person
// never opened, the cut-off is the moment this feature first looked at that
// person, so old threads do not light up all at once.

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
	return store.MarkThreadRead(username, channel, parentID)
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

	return store.ThreadUnread(username, channel)
}
