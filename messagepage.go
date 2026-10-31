package main

import (
	"fmt"
)

// History is loaded in pages: the newest page when a channel is opened, then
// older pages while the user scrolls up.

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

// MessagePage is one page of history, oldest message first.
type MessagePage struct {
	Messages []Message `json:"messages"`
	// HasMore is true when there may be older messages before this page.
	HasMore bool `json:"hasMore"`
}

// GetMessagePage returns up to `limit` messages. With an empty beforeID it is
// the newest page; otherwise it is the page right before the message with that
// id. The cursor is a message id, not a list position, so deletes and edits
// between two requests cannot shift the page.
func GetMessagePage(channel, beforeID string, limit int64) (MessagePage, error) {
	return store.MessagePage(channel, beforeID, limit)
}

func clampPageSize(n int) int64 {
	if n <= 0 {
		return defaultPageSize
	}
	if n > maxPageSize {
		return maxPageSize
	}
	return int64(n)
}

// GetMessagesPage is the call the frontend uses. beforeID "" = newest page.
func (a *App) GetMessagesPage(token, channel, beforeID string, limit int) (MessagePage, error) {
	username, err := sessionUser(token)
	if err != nil {
		return MessagePage{}, err
	}
	if err := requireChannelAccess(username, channel); err != nil {
		return MessagePage{}, err
	}
	page, err := GetMessagePage(channel, beforeID, clampPageSize(limit))
	if err != nil {
		return MessagePage{Messages: []Message{}}, fmt.Errorf("failed to load messages")
	}
	return page, nil
}
