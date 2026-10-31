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

// ───────────────────────── storage ─────────────────────────

// resolvePins attaches the messages to the stored pins. A pin whose message no
// longer exists is dropped from the result and from the store.
func resolvePins(channel string) ([]PinnedMessage, error) {
	return store.ResolvedPins(channel)
}

// removePin deletes the pin of one message, reporting whether there was one.
func removePin(channel, messageID string) (bool, error) {
	return store.RemovePin(channel, messageID)
}

// deleteChannelPins removes the pins of a channel (channel deleted).
func deleteChannelPins(channel string) {
	store.DeleteChannelPins(channel)
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

	entries, err := store.PinEntries(channel)
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

	if err := store.AddPin(channel, pinEntry{MessageID: messageID, PinnedBy: username, PinnedAt: time.Now()}); err != nil {
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
