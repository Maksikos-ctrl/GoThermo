package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// Pinned messages: a short list of important messages kept at the top of a
// channel.
//
// Storage: the Redis list "channel:<name>:pins" holds one small JSON entry per
// pin (message id, who pinned it, when), newest first. The messages themselves
// stay where they are, so editing or reacting to a pinned message needs no
// extra work.

const maxPinsPerChannel = 50

func pinsKey(channel string) string {
	return fmt.Sprintf("channel:%s:pins", channel)
}

type pinEntry struct {
	MessageID string    `json:"messageId"`
	PinnedBy  string    `json:"pinnedBy"`
	PinnedAt  time.Time `json:"pinnedAt"`
}

// PinnedMessage is a pinned message with the pin's details.
type PinnedMessage struct {
	Message  Message   `json:"message"`
	PinnedBy string    `json:"pinnedBy"`
	PinnedAt time.Time `json:"pinnedAt"`
}

type PinsChangedPayload struct {
	Channel string          `json:"channel"`
	Pins    []PinnedMessage `json:"pins"`
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

// resolvePins attaches the messages to the stored pins in one pass over the
// channel. A pin whose message no longer exists is dropped from the result
// and removed from Redis.
func resolvePins(channel string) ([]PinnedMessage, error) {
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
func removePin(channel, messageID string) (bool, error) {
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
func deleteChannelPins(channel string) {
	redisClient.Del(ctx, pinsKey(channel))
}

// ───────────────────────── App methods ─────────────────────────

// PinMessage pins a channel message. Anyone who can see the channel may pin.
// Pinning an already pinned message changes nothing.
func (a *App) PinMessage(token, channel, messageID, username string) error {
	if err := requireUser(token, username); err != nil {
		return err
	}
	if err := requireChannelAccess(username, channel); err != nil {
		return err
	}
	msg, err := findChannelMessage(channel, messageID)
	if err != nil {
		return err
	}
	if msg.IsCall {
		return fmt.Errorf("you cannot pin a call log")
	}

	entries, _, err := loadPinEntries(channel)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.MessageID == messageID {
			return nil
		}
	}
	if len(entries) >= maxPinsPerChannel {
		return fmt.Errorf("this channel already has %d pinned messages, unpin one first", maxPinsPerChannel)
	}

	data, err := json.Marshal(pinEntry{MessageID: messageID, PinnedBy: username, PinnedAt: time.Now()})
	if err != nil {
		return err
	}
	if err := redisClient.LPush(ctx, pinsKey(channel), data).Err(); err != nil {
		return fmt.Errorf("error saving pin: %v", err)
	}
	a.announcePins(channel)
	log.Printf("📌 %s pinned a message in #%s", username, channel)
	return nil
}

// UnpinMessage removes a pin. Anyone who can see the channel may unpin.
func (a *App) UnpinMessage(token, channel, messageID, username string) error {
	if err := requireUser(token, username); err != nil {
		return err
	}
	if err := requireChannelAccess(username, channel); err != nil {
		return err
	}
	removed, err := removePin(channel, messageID)
	if err != nil {
		return fmt.Errorf("error removing pin: %v", err)
	}
	if removed {
		a.announcePins(channel)
		log.Printf("📌 %s unpinned a message in #%s", username, channel)
	}
	return nil
}

// GetPinnedMessages returns the pinned messages of a channel, newest pin first.
func (a *App) GetPinnedMessages(token, channel string) ([]PinnedMessage, error) {
	username, err := sessionUser(token)
	if err != nil {
		return nil, err
	}
	if err := requireChannelAccess(username, channel); err != nil {
		return nil, err
	}
	return resolvePins(channel)
}

// announcePins sends the current pin list to everyone who can see the channel.
func (a *App) announcePins(channel string) {
	pins, err := resolvePins(channel)
	if err != nil {
		log.Printf("⚠️ could not read pins of #%s: %v", channel, err)
		return
	}
	a.hub.BroadcastPins(channel, pins)
}

// BroadcastPins announces the new pin list; clients replace their copy.
func (h *Hub) BroadcastPins(channel string, pins []PinnedMessage) {
	data, err := json.Marshal(WSMessage{
		Type:    "pins_changed",
		Payload: PinsChangedPayload{Channel: channel, Pins: pins},
	})
	if err != nil {
		log.Printf("Error marshaling pins_changed: %v", err)
		return
	}
	h.sendToChannelViewers(channel, data)
}
