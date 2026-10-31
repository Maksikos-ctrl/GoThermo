package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Threads: a reply to a message, shown in a side panel instead of the channel.
//
// A reply never appears in the channel history, so paging, unread counts and
// the channel view stay exactly as they were. The parent message carries
// ReplyCount / LastReplyAt / ReplyUsers, always recomputed from the replies
// (never incremented), so they cannot drift. How replies are stored is up to
// the active store (store.go); the Redis layout is in store_redis_messages.go.

const (
	maxRepliesPerThread = 1000
	maxReplyAvatars     = 4
)

// Thread is a parent message with all of its replies, oldest first.
type Thread struct {
	Parent  Message   `json:"parent"`
	Replies []Message `json:"replies"`
}

type ThreadReplyPayload struct {
	Channel  string  `json:"channel"`
	ThreadID string  `json:"threadId"`
	Message  Message `json:"message"`
}

type ThreadReplyDeletedPayload struct {
	Channel   string `json:"channel"`
	ThreadID  string `json:"threadId"`
	MessageID string `json:"messageId"`
}

// ───────────────────────── storage ─────────────────────────

// The functions below read and write through the active store (store.go).

// findChannelMessage looks a message up in the whole channel list.
func findChannelMessage(channel, id string) (*Message, error) {
	return store.FindMessage(channel, id)
}

// refreshThreadMeta returns the parent with its ReplyCount / LastReplyAt /
// ReplyUsers up to date.
func refreshThreadMeta(channel, parentID string) (*Message, error) {
	return store.RefreshThreadMeta(channel, parentID)
}

// deleteThread removes a thread (used when its parent message is deleted).
func deleteThread(channel, parentID string) {
	store.DeleteThread(channel, parentID)
}

// ───────────────────────── App methods ─────────────────────────

func (a *App) SendReply(token, user, channel, parentID, text string) (Message, error) {
	if err := requireUser(token, user); err != nil {
		return Message{}, err
	}
	if err := requireChannelAccess(user, channel); err != nil {
		return Message{}, err
	}
	if strings.TrimSpace(text) == "" {
		return Message{}, fmt.Errorf("reply cannot be empty")
	}
	parent, err := findChannelMessage(channel, parentID)
	if err != nil {
		return Message{}, err
	}
	if parent.IsCall {
		return Message{}, fmt.Errorf("you cannot reply to a call log")
	}
	if n, _ := store.ReplyCount(channel, parentID); n >= maxRepliesPerThread {
		return Message{}, fmt.Errorf("this thread is full")
	}

	reply := Message{
		ID:        uuid.New().String(),
		User:      user,
		Text:      text,
		Channel:   channel,
		Timestamp: time.Now(),
		Reactions: make(map[string][]string),
		ThreadID:  parentID,
	}
	if err := store.AddReply(reply); err != nil {
		return Message{}, fmt.Errorf("error saving reply: %v", err)
	}

	updated, err := refreshThreadMeta(channel, parentID)
	if err != nil {
		// The reply itself is saved; the counter is repaired by the next change.
		log.Printf("⚠️ thread meta not updated for %s: %v", parentID, err)
	}

	a.hub.BroadcastThreadReply(channel, parentID, reply)
	if updated != nil {
		a.hub.BroadcastToChannel(channel, *updated)
	}
	log.Printf("🧵 %s replied in #%s: %s", user, channel, truncate(text, 50))
	return reply, nil
}

func (a *App) GetThread(token, channel, parentID string) (Thread, error) {
	username, err := sessionUser(token)
	if err != nil {
		return Thread{}, err
	}
	if err := requireChannelAccess(username, channel); err != nil {
		return Thread{}, err
	}
	parent, err := findChannelMessage(channel, parentID)
	if err != nil {
		return Thread{}, err
	}
	replies, err := store.Replies(channel, parentID)
	if err != nil {
		return Thread{}, err
	}
	return Thread{Parent: *parent, Replies: replies}, nil
}

func (a *App) EditReply(token, channel, parentID, replyID, username, newText string) (Message, error) {
	if err := requireUser(token, username); err != nil {
		return Message{}, err
	}
	if err := requireChannelAccess(username, channel); err != nil {
		return Message{}, err
	}
	if strings.TrimSpace(newText) == "" {
		return Message{}, fmt.Errorf("reply cannot be empty")
	}
	replies, err := store.Replies(channel, parentID)
	if err != nil {
		return Message{}, err
	}
	for _, r := range replies {
		if r.ID != replyID {
			continue
		}
		if r.User != username {
			return Message{}, fmt.Errorf("you can only edit your own messages")
		}
		r.Text = newText
		r.IsEdited = true
		if err := store.UpdateReply(channel, parentID, r); err != nil {
			return Message{}, fmt.Errorf("error updating reply: %v", err)
		}
		a.hub.BroadcastThreadReply(channel, parentID, r)
		return r, nil
	}
	return Message{}, fmt.Errorf("message not found")
}

func (a *App) DeleteReply(token, channel, parentID, replyID, username string) error {
	if err := requireUser(token, username); err != nil {
		return err
	}
	if err := requireChannelAccess(username, channel); err != nil {
		return err
	}
	replies, err := store.Replies(channel, parentID)
	if err != nil {
		return err
	}
	for _, r := range replies {
		if r.ID != replyID {
			continue
		}
		if r.User != username {
			return fmt.Errorf("you can only delete your own messages")
		}
		if err := store.DeleteReply(channel, parentID, replyID); err != nil {
			return fmt.Errorf("error deleting reply: %v", err)
		}
		a.hub.BroadcastThreadReplyDeleted(channel, parentID, replyID)
		if updated, err := refreshThreadMeta(channel, parentID); err == nil {
			a.hub.BroadcastToChannel(channel, *updated)
		}
		log.Printf("🗑️ %s deleted a reply in #%s", username, channel)
		return nil
	}
	return fmt.Errorf("message not found")
}

func (a *App) AddReplyReaction(token, channel, parentID, replyID, emoji, username string) error {
	if err := requireUser(token, username); err != nil {
		return err
	}
	if err := requireChannelAccess(username, channel); err != nil {
		return err
	}
	replies, err := store.Replies(channel, parentID)
	if err != nil {
		return err
	}
	for _, r := range replies {
		if r.ID != replyID {
			continue
		}
		if r.Reactions == nil {
			r.Reactions = make(map[string][]string)
		}
		// toggle, same as for channel messages
		kept := []string{}
		had := false
		for _, u := range r.Reactions[emoji] {
			if u == username {
				had = true
				continue
			}
			kept = append(kept, u)
		}
		if !had {
			kept = append(kept, username)
		}
		if len(kept) == 0 {
			delete(r.Reactions, emoji)
		} else {
			r.Reactions[emoji] = kept
		}
		if err := store.UpdateReply(channel, parentID, r); err != nil {
			return fmt.Errorf("error updating reply: %v", err)
		}
		a.hub.BroadcastThreadReply(channel, parentID, r)
		return nil
	}
	return fmt.Errorf("message not found")
}

// ───────────────────────── live updates ─────────────────────────

func (h *Hub) sendToChannelViewers(channel string, data []byte) {
	ch, _ := GetChannel(channel)

	h.mutex.RLock()
	defer h.mutex.RUnlock()

	for username, client := range h.clients {
		if !shouldDeliver(client, ch, channel) {
			continue
		}
		select {
		case client.Send <- data:
		default:
			close(client.Send)
			delete(h.clients, username)
		}
	}
}

// BroadcastThreadReply announces a new or changed reply. Clients upsert by id.
func (h *Hub) BroadcastThreadReply(channel, threadID string, msg Message) {
	data, err := json.Marshal(WSMessage{
		Type:    "thread_reply",
		Payload: ThreadReplyPayload{Channel: channel, ThreadID: threadID, Message: msg},
	})
	if err != nil {
		log.Printf("Error marshaling thread_reply: %v", err)
		return
	}
	h.sendToChannelViewers(channel, data)
}

func (h *Hub) BroadcastThreadReplyDeleted(channel, threadID, messageID string) {
	data, err := json.Marshal(WSMessage{
		Type:    "thread_reply_deleted",
		Payload: ThreadReplyDeletedPayload{Channel: channel, ThreadID: threadID, MessageID: messageID},
	})
	if err != nil {
		log.Printf("Error marshaling thread_reply_deleted: %v", err)
		return
	}
	h.sendToChannelViewers(channel, data)
}
