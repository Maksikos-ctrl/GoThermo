package main

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// LeaveChannel lets a member walk out of a private channel.
//
//   - Only regular private channels can be left (a public channel has no
//     member list that matters, a DM is not a channel you leave).
//   - The last member cannot leave: the channel would be left behind with
//     nobody to see or delete it. They can delete it instead.
//   - If the creator leaves, the oldest remaining member becomes the owner,
//     otherwise nobody could delete the channel any more.
//   - The channel stays closed to the person who left; any member can invite
//     them back.
func (a *App) LeaveChannel(token, channelName, username string) error {
	if err := requireUser(token, username); err != nil {
		return err
	}
	ch, err := GetChannel(channelName)
	if err != nil || ch == nil {
		return fmt.Errorf("channel not found")
	}
	if !isRegularPrivate(ch) {
		return fmt.Errorf("you can only leave private channels")
	}
	if !channelHasMember(ch.Members, username) {
		return fmt.Errorf("you are not a member of this channel")
	}
	if len(ch.Members) <= 1 {
		return fmt.Errorf("you are the last member: delete the channel instead")
	}

	remaining := make([]string, 0, len(ch.Members)-1)
	for _, m := range ch.Members {
		if m != username {
			remaining = append(remaining, m)
		}
	}
	ch.Members = remaining
	if ch.CreatedBy == username {
		ch.CreatedBy = remaining[0]
	}
	if err := SaveChannel(*ch); err != nil {
		return fmt.Errorf("failed to leave channel: %v", err)
	}
	log.Printf("👋 %s left private channel #%s", username, channelName)

	if a.hub == nil {
		return nil
	}
	// Stop live delivery to the person who left, and take the channel off their screen.
	a.hub.RemoveChannelFromClient(username, channelName)
	a.hub.BroadcastChannelDeleted(channelName, username)
	// Everybody still inside gets the new member list, plus a note in the chat.
	a.hub.BroadcastChannelCreated(*ch)
	msg := Message{
		ID: uuid.New().String(), User: username, Channel: channelName,
		Text: "👋 " + username + " left the channel", Timestamp: time.Now(),
		Reactions: make(map[string][]string),
	}
	if err := SaveMessage(msg); err == nil {
		a.hub.BroadcastToChannel(channelName, msg)
	}
	return nil
}
