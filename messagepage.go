package main

import (
	"encoding/json"
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

func clampPageSize(n int) int64 {
	if n <= 0 {
		return defaultPageSize
	}
	if n > maxPageSize {
		return maxPageSize
	}
	return int64(n)
}

// GetMessagePage returns up to `limit` messages. With an empty beforeID it is
// the newest page; otherwise it is the page right before the message with that
// id. The cursor is a message id, not a list position, so deletes and edits
// between two requests cannot shift the page.
func GetMessagePage(channel, beforeID string, limit int64) (MessagePage, error) {
	key := fmt.Sprintf("channel:%s:messages", channel)

	if beforeID == "" {
		total, err := redisClient.LLen(ctx, key).Result()
		if err != nil {
			return MessagePage{}, err
		}
		msgs, err := GetMessages(channel, limit)
		if err != nil {
			return MessagePage{}, err
		}
		// "DELETED" placeholders may make this a false positive; the next
		// request then just answers with HasMore=false.
		return MessagePage{Messages: msgs, HasMore: total > limit}, nil
	}

	raw, err := redisClient.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return MessagePage{}, err
	}
	all := make([]Message, 0, len(raw))
	cursor := -1
	for _, data := range raw {
		if data == "DELETED" {
			continue
		}
		var m Message
		if json.Unmarshal([]byte(data), &m) != nil {
			continue
		}
		if m.ID == beforeID {
			cursor = len(all)
		}
		all = append(all, m)
	}
	if cursor == -1 {
		// the message the client scrolled from is gone: nothing sensible to send
		return MessagePage{Messages: []Message{}}, nil
	}
	start := cursor - int(limit)
	if start < 0 {
		start = 0
	}
	page := make([]Message, cursor-start)
	copy(page, all[start:cursor])
	return MessagePage{Messages: page, HasMore: start > 0}, nil
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
