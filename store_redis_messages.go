package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// The Redis side of messages, threads, pins, thread read marks and files.
// This is the code that used to live in threads.go, pins.go, threadextras.go,
// messagepage.go and uploads.go, moved here unchanged apart from the names.
//
// Redis layout:
//   channel:<name>:messages      list of message JSON, oldest first
//   thread:<channel>:<parentID>  list of reply JSON, oldest first
//   channel:<name>:threads       set of parent ids that have a thread
//   channel:<name>:pins          list of pin JSON, newest first
//   threadread:<user>            hash "<channel>:<parentID>" -> time opened
//   threadbase:<user>            time the unread-replies feature first saw the user
//   file:<id>                    hash path / mimeType / fileName

func threadKey(channel, parentID string) string {
	return fmt.Sprintf("thread:%s:%s", channel, parentID)
}

func threadIndexKey(channel string) string {
	return fmt.Sprintf("channel:%s:threads", channel)
}

func pinsKey(channel string) string {
	return fmt.Sprintf("channel:%s:pins", channel)
}

// loadPinEntries returns the stored pins (newest first) with their raw JSON.
func loadPinEntries(channel string) ([]pinEntry, []string, error) {
	raw, err := redisClient.LRange(ctx, pinsKey(channel), 0, -1).Result()
	if err != nil {
		return nil, nil, err
	}
	entries := make([]pinEntry, 0, len(raw))
	raws := make([]string, 0, len(raw))
	for _, data := range raw {
		var e pinEntry
		if json.Unmarshal([]byte(data), &e) != nil || e.MessageID == "" {
			continue
		}
		entries = append(entries, e)
		raws = append(raws, data)
	}
	return entries, raws, nil
}

func threadReadKey(user string) string { return "threadread:" + user }

func threadBaseKey(user string) string { return "threadbase:" + user }

func threadReadField(channel, parentID string) string { return channel + ":" + parentID }

// findChannelMessage looks a message up in the whole channel list.
func redisFindMessage(channel, id string) (*Message, error) {
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
func redisRefreshThreadMeta(channel, parentID string) (*Message, error) {
	parent, err := redisFindMessage(channel, parentID)
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
	if err := redisUpdateMessage(channel, *parent); err != nil {
		return nil, err
	}
	return parent, nil
}

// deleteThread removes a thread (used when its parent message is deleted).
func redisDeleteThread(channel, parentID string) {
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

// GetMessagePage returns up to `limit` messages. With an empty beforeID it is
// the newest page; otherwise it is the page right before the message with that
// id. The cursor is a message id, not a list position, so deletes and edits
// between two requests cannot shift the page.
func redisMessagePage(channel, beforeID string, limit int64) (MessagePage, error) {
	key := fmt.Sprintf("channel:%s:messages", channel)

	if beforeID == "" {
		total, err := redisClient.LLen(ctx, key).Result()
		if err != nil {
			return MessagePage{}, err
		}
		msgs, err := redisGetMessages(channel, limit)
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

// resolvePins attaches the messages to the stored pins in one pass over the
// channel. A pin whose message no longer exists is dropped from the result
// and removed from Redis.
func redisResolvePins(channel string) ([]PinnedMessage, error) {
	entries, raws, err := loadPinEntries(channel)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return []PinnedMessage{}, nil
	}

	wanted := make(map[string]bool, len(entries))
	for _, e := range entries {
		wanted[e.MessageID] = true
	}
	list, err := redisClient.LRange(ctx, fmt.Sprintf("channel:%s:messages", channel), 0, -1).Result()
	if err != nil {
		return nil, err
	}
	found := make(map[string]Message, len(entries))
	for _, data := range list {
		if data == "DELETED" {
			continue
		}
		var m Message
		if json.Unmarshal([]byte(data), &m) == nil && wanted[m.ID] {
			found[m.ID] = m
		}
	}

	pins := make([]PinnedMessage, 0, len(entries))
	for i, e := range entries {
		m, ok := found[e.MessageID]
		if !ok {
			redisClient.LRem(ctx, pinsKey(channel), 1, raws[i])
			continue
		}
		pins = append(pins, PinnedMessage{Message: m, PinnedBy: e.PinnedBy, PinnedAt: e.PinnedAt})
	}
	return pins, nil
}

// removePin deletes the pin of one message, reporting whether there was one.
func redisRemovePin(channel, messageID string) (bool, error) {
	entries, raws, err := loadPinEntries(channel)
	if err != nil {
		return false, err
	}
	removed := false
	for i, e := range entries {
		if e.MessageID == messageID {
			// by exact value, so a concurrent change cannot remove another pin
			if err := redisClient.LRem(ctx, pinsKey(channel), 0, raws[i]).Err(); err != nil {
				return false, err
			}
			removed = true
		}
	}
	return removed, nil
}

// deleteChannelPins removes the pins of a channel (channel deleted).
func redisDeleteChannelPins(channel string) {
	redisClient.Del(ctx, pinsKey(channel))
}

// ───────────────────────── Store methods ─────────────────────────

func (redisStore) AppendMessage(m Message) error { return redisSaveMessage(m) }
func (redisStore) RecentMessages(channel string, limit int64) ([]Message, error) {
	return redisGetMessages(channel, limit)
}
func (redisStore) MessagePage(channel, beforeID string, limit int64) (MessagePage, error) {
	return redisMessagePage(channel, beforeID, limit)
}
func (redisStore) FindMessage(channel, id string) (*Message, error) {
	return redisFindMessage(channel, id)
}
func (redisStore) UpdateMessage(channel string, m Message) error {
	return redisUpdateMessage(channel, m)
}
func (redisStore) DeleteMessage(channel, id string) error { return redisDeleteMessage(channel, id) }

func (redisStore) Replies(channel, parentID string) ([]Message, error) {
	msgs, _, err := loadReplies(channel, parentID)
	return msgs, err
}

func (redisStore) ReplyCount(channel, parentID string) (int, error) {
	n, err := redisClient.LLen(ctx, threadKey(channel, parentID)).Result()
	return int(n), err
}

func (redisStore) AddReply(r Message) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := redisClient.RPush(ctx, threadKey(r.Channel, r.ThreadID), data).Err(); err != nil {
		return err
	}
	redisClient.SAdd(ctx, threadIndexKey(r.Channel), r.ThreadID)
	return nil
}

func (redisStore) UpdateReply(channel, parentID string, r Message) error {
	replies, _, err := loadReplies(channel, parentID)
	if err != nil {
		return err
	}
	for i, old := range replies {
		if old.ID != r.ID {
			continue
		}
		data, err := json.Marshal(r)
		if err != nil {
			return err
		}
		return redisClient.LSet(ctx, threadKey(channel, parentID), int64(i), data).Err()
	}
	return fmt.Errorf("message not found")
}

func (redisStore) DeleteReply(channel, parentID, replyID string) error {
	replies, raws, err := loadReplies(channel, parentID)
	if err != nil {
		return err
	}
	for i, r := range replies {
		if r.ID == replyID {
			// by exact value, so a concurrent change cannot make us remove another reply
			return redisClient.LRem(ctx, threadKey(channel, parentID), 1, raws[i]).Err()
		}
	}
	return fmt.Errorf("message not found")
}

func (redisStore) RefreshThreadMeta(channel, parentID string) (*Message, error) {
	return redisRefreshThreadMeta(channel, parentID)
}

func (redisStore) DeleteThread(channel, parentID string) error {
	redisDeleteThread(channel, parentID)
	return nil
}

func (redisStore) MarkThreadRead(username, channel, parentID string) error {
	return redisClient.HSet(ctx, threadReadKey(username), threadReadField(channel, parentID),
		time.Now().Format(time.RFC3339Nano)).Err()
}

func (redisStore) ThreadUnread(username, channel string) (map[string]int, error) {
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

func (redisStore) PinEntries(channel string) ([]pinEntry, error) {
	entries, _, err := loadPinEntries(channel)
	return entries, err
}

func (redisStore) AddPin(channel string, e pinEntry) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return redisClient.LPush(ctx, pinsKey(channel), data).Err()
}

func (redisStore) RemovePin(channel, messageID string) (bool, error) {
	return redisRemovePin(channel, messageID)
}

func (redisStore) ResolvedPins(channel string) ([]PinnedMessage, error) {
	return redisResolvePins(channel)
}

func (redisStore) DeleteChannelPins(channel string) error {
	redisDeleteChannelPins(channel)
	return nil
}

func (redisStore) SaveFileMeta(fileID, filePath, mimeType, fileName string) error {
	return redisSaveFileMeta(fileID, filePath, mimeType, fileName)
}

func (redisStore) FileMeta(fileID string) (path, mimeType, fileName string, err error) {
	return redisGetFileMeta(fileID)
}

func (redisStore) DeleteFileMeta(fileID string) error {
	return redisClient.Del(ctx, "file:"+fileID).Err()
}
