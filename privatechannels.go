package main

import (
	"fmt"
	"log"
	"strings"
)

// normalizeMembers builds the member list of a new private channel: the
// creator first, then the requested users without duplicates. Every name must
// be a real user, so a typo or a made-up name cannot end up in the list.
func normalizeMembers(creator string, requested []string) ([]string, error) {
	members := []string{creator}
	seen := map[string]bool{creator: true}
	for _, name := range requested {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		if _, ok := userManager.GetUser(name); !ok {
			return nil, fmt.Errorf("user %q not found", name)
		}
		seen[name] = true
		members = append(members, name)
	}
	return members, nil
}

// isRegularPrivate is true for a private channel made by a user, as opposed
// to a direct message (dm_...), which has its own rules.
func isRegularPrivate(ch *Channel) bool {
	return ch != nil && ch.IsPrivate && !strings.HasPrefix(ch.Name, "dm_")
}

// listVisibleChannels returns what a user may see in the channel list: every
// public channel plus the private channels they belong to. Direct messages
// are listed separately (listDMChannels).
func listVisibleChannels(username string) ([]Channel, error) {
	channels, err := GetAllChannels()
	if err != nil {
		return nil, err
	}
	visible := make([]Channel, 0, len(channels))
	for i := range channels {
		ch := &channels[i]
		switch {
		case !ch.IsPrivate:
			visible = append(visible, *ch)
		case isRegularPrivate(ch) && channelHasMember(ch.Members, username):
			visible = append(visible, *ch)
		}
	}
	return visible, nil
}

// AddChannelMember lets a member of a private channel invite another user.
func (a *App) AddChannelMember(token, channelName, caller, newMember string) (Channel, error) {
	if err := requireUser(token, caller); err != nil {
		return Channel{}, err
	}
	ch, err := GetChannel(channelName)
	if err != nil || ch == nil {
		return Channel{}, fmt.Errorf("channel not found")
	}
	if !isRegularPrivate(ch) {
		return Channel{}, fmt.Errorf("members can only be added to private channels")
	}
	if !channelHasMember(ch.Members, caller) {
		return Channel{}, fmt.Errorf("you are not a member of this channel")
	}
	newMember = strings.TrimSpace(newMember)
	if _, ok := userManager.GetUser(newMember); !ok {
		return Channel{}, fmt.Errorf("user %q not found", newMember)
	}
	if channelHasMember(ch.Members, newMember) {
		return *ch, nil
	}

	ch.Members = append(ch.Members, newMember)
	if err := SaveChannel(*ch); err != nil {
		return Channel{}, fmt.Errorf("failed to add member: %v", err)
	}
	log.Printf("➕ %s added %s to private channel #%s", caller, newMember, channelName)

	// Members get the updated channel; the new member sees it appear.
	a.hub.BroadcastChannelCreated(*ch)
	return *ch, nil
}
