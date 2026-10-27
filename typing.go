package main

import (
	"encoding/json"
	"time"
)

// Typing indicator. A client sends {"type":"typing","payload":"<channel>"}
// while its user is typing; everybody who may see that channel (except the
// sender) gets {"type":"typing","payload":{"channel":..,"username":..}}.
// Nothing is stored. The receiving app hides the hint after a few seconds
// unless it is refreshed.

const (
	typingMinGap     = 2 * time.Second // at most one event per channel per client in this time
	maxTypingTracked = 64              // channels remembered per connection
)

type TypingPayload struct {
	Channel  string `json:"channel"`
	Username string `json:"username"`
}

func (c *Client) handleTyping(payload interface{}) {
	channel, ok := payload.(string)
	if !ok || channel == "" || len(channel) > 200 {
		return
	}
	now := time.Now()
	if c.lastTyping == nil || len(c.lastTyping) > maxTypingTracked {
		c.lastTyping = map[string]time.Time{}
	}
	if last, seen := c.lastTyping[channel]; seen && now.Sub(last) < typingMinGap {
		return
	}
	ch, err := GetChannel(channel)
	if err != nil || ch == nil {
		return // not a real channel
	}
	if ch.IsPrivate && !channelHasMember(ch.Members, c.Username) {
		return
	}
	c.lastTyping[channel] = now
	c.Hub.BroadcastTyping(channel, c.Username)
}

// BroadcastTyping tells the viewers of a channel, except the typist.
func (h *Hub) BroadcastTyping(channel, typist string) {
	data, err := json.Marshal(WSMessage{
		Type:    "typing",
		Payload: TypingPayload{Channel: channel, Username: typist},
	})
	if err != nil {
		return
	}
	ch, _ := GetChannel(channel)

	h.mutex.RLock()
	defer h.mutex.RUnlock()
	for username, client := range h.clients {
		if username == typist || !shouldDeliver(client, ch, channel) {
			continue
		}
		select {
		case client.Send <- data:
		default:
			// a slow client just misses a typing hint; it is not worth dropping it
		}
	}
}
