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
// Storage: every thread has its own Redis list, "thread:<channel>:<parentID>"
// (oldest reply first). The channel list is never touched by a reply, so
// paging, unread counts and the channel view stay exactly as they were.
// "channel:<name>:threads" is a set of the parent ids that have a thread, so a
// channel can be cleaned up without scanning all keys.
//
// The parent message in the channel list carries ReplyCount / LastReplyAt /
// ReplyUsers. They are always recomputed from the thread list (never
// incremented), so they cannot drift.

const (
	maxRepliesPerThread = 1000
	maxReplyAvatars     = 4
)

func threadKey(channel, parentID string) string {
	return fmt.Sprintf("thread:%s:%s", channel, parentID)
}

func threadIndexKey(channel string) string {
	return fmt.Sprintf("channel:%s:threads", channel)
}

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

// findChannelMessage looks a message up in the whole channel list.
func findChannelMessage(channel, id string) (*Message, error) {
	raw, err := redisClient.LRange(ctx, fmt.Sprintf("channel:%s:messages", channel), 0, -1).Result()
	if err != nil {
		return nil, err
	}
	for _, data := range raw {
		if data == "DELETED" {
			continue
		}
		var m Message
		if json.Unmarshal([]byte(data), &m) == nil && m.ID == id {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("message not found")
}

// loadReplies returns the replies of a thread together with their raw JSON
// (needed to remove one exactly).
func loadReplies(channel, parentID string) ([]Message, []string, error) {
	raw, err := redisClient.LRange(ctx, threadKey(channel, parentID), 0, -1).Result()
	if err != nil {
		return nil, nil, err
	}
	msgs := make([]Message, 0, len(raw))
	raws := make([]string, 0, len(raw))
	for _, data := range raw {
		var m Message
		if json.Unmarshal([]byte(data), &m) != nil {
			continue
		}
		msgs = append(msgs, m)
		raws = append(raws, data)
	}
	return msgs, raws, nil
}

// refreshThreadMeta recomputes ReplyCount / LastReplyAt / ReplyUsers on the
// parent from the thread list and saves the parent when they changed.
func refreshThreadMeta(channel, parentID string) (*Message, error) {
	parent, err := findChannelMessage(channel, parentID)
	if err != nil {
		return nil, err
	}
	replies, _, err := loadReplies(channel, parentID)
	if err != nil {
		return nil, err
	}

	var users []string
	var last time.Time
	for i := range replies {
		r := replies[i]
		if !channelHasMember(users, r.User) && len(users) < maxReplyAvatars {
			users = append(users, r.User)
		}
		if r.Timestamp.After(last) {
			last = r.Timestamp
		}
	}

	parent.ReplyCount = len(replies)
	parent.LastReplyAt = ""
	if !last.IsZero() {
		parent.LastReplyAt = last.Format(time.RFC3339Nano)
	}
	parent.ReplyUsers = users
	if err := UpdateMessage(channel, *parent); err != nil {
		return nil, err
	}
	return parent, nil
}

// deleteThread removes a thread (used when its parent message is deleted).
func deleteThread(channel, parentID string) {
	redisClient.Del(ctx, threadKey(channel, parentID))
	redisClient.SRem(ctx, threadIndexKey(channel), parentID)
}

// deleteChannelThreads removes every thread of a channel (channel deleted).
func deleteChannelThreads(channel string) {
	ids, err := redisClient.SMembers(ctx, threadIndexKey(channel)).Result()
	if err == nil {
		for _, id := range ids {
			redisClient.Del(ctx, threadKey(channel, id))
		}
	}
	redisClient.Del(ctx, threadIndexKey(channel))
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
	if n, _ := redisClient.LLen(ctx, threadKey(channel, parentID)).Result(); n >= maxRepliesPerThread {
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
	data, err := json.Marshal(reply)
	if err != nil {
		return Message{}, err
	}
	if err := redisClient.RPush(ctx, threadKey(channel, parentID), data).Err(); err != nil {
		return Message{}, fmt.Errorf("error saving reply: %v", err)
	}
	redisClient.SAdd(ctx, threadIndexKey(channel), parentID)

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
	replies, _, err := loadReplies(channel, parentID)
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
	replies, _, err := loadReplies(channel, parentID)
	if err != nil {
		return Message{}, err
	}
	for i, r := range replies {
		if r.ID != replyID {
			continue
		}
		if r.User != username {
			return Message{}, fmt.Errorf("you can only edit your own messages")
		}
		r.Text = newText
		r.IsEdited = true
		data, _ := json.Marshal(r)
		if err := redisClient.LSet(ctx, threadKey(channel, parentID), int64(i), data).Err(); err != nil {
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
	replies, raws, err := loadReplies(channel, parentID)
	if err != nil {
		return err
	}
	for i, r := range replies {
		if r.ID != replyID {
			continue
		}
		if r.User != username {
			return fmt.Errorf("you can only delete your own messages")
		}
		// by exact value, so a concurrent change cannot make us remove another reply
		if err := redisClient.LRem(ctx, threadKey(channel, parentID), 1, raws[i]).Err(); err != nil {
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
	replies, _, err := loadReplies(channel, parentID)
	if err != nil {
		return err
	}
	for i, r := range replies {
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
		data, _ := json.Marshal(r)
		if err := redisClient.LSet(ctx, threadKey(channel, parentID), int64(i), data).Err(); err != nil {
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
