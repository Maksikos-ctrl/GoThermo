package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type App struct {
	ctx context.Context
	hub *Hub
}

func NewApp() *App {
	initRedis()
	hub := NewHub()
	go hub.Run()
	userManager.LoadUsersFromRedis()

	userManager.ResetAllStatusesToOffline()

	return &App{hub: hub}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.initDefaultChannels()
	log.Println("✓ GoThermo application started successfully")
}

func (a *App) initDefaultChannels() {
	channels, err := GetAllChannels()
	if err != nil {
		log.Printf("Error fetching channels: %v", err)
		return
	}
	if len(channels) > 0 {
		log.Printf("✓ Found %d channels", len(channels))
		return
	}
	defaultChannels := []Channel{
		{ID: uuid.New().String(), Name: "general", Description: "General discussions", Members: []string{}, CreatedBy: "system", CreatedAt: time.Now(), IsPrivate: false},
		{ID: uuid.New().String(), Name: "random", Description: "Random stuff", Members: []string{}, CreatedBy: "system", CreatedAt: time.Now(), IsPrivate: false},
		{ID: uuid.New().String(), Name: "dev-team", Description: "Development team", Members: []string{}, CreatedBy: "system", CreatedAt: time.Now(), IsPrivate: false},
	}
	for _, channel := range defaultChannels {
		if err := SaveChannel(channel); err != nil {
			log.Printf("Error creating channel %s: %v", channel.Name, err)
		} else {
			log.Printf("✓ Channel created: #%s", channel.Name)
		}
	}
}

func (a *App) SendMessage(user, text, channel string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("message cannot be empty")
	}
	msg := Message{ID: uuid.New().String(), User: user, Text: text, Channel: channel, Timestamp: time.Now(), Reactions: make(map[string][]string), IsPost: false}
	if err := SaveMessage(msg); err != nil {
		return "", fmt.Errorf("error saving message: %v", err)
	}
	a.hub.BroadcastToChannel(channel, msg)
	log.Printf("📨 %s -> #%s: %s", user, channel, truncate(text, 50))
	return msg.ID, nil
}

func (a *App) SendPost(user, text, channel string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("post cannot be empty")
	}
	msg := Message{ID: uuid.New().String(), User: user, Text: text, Channel: channel, Timestamp: time.Now(), Reactions: make(map[string][]string), IsPost: true}
	if err := SaveMessage(msg); err != nil {
		return "", fmt.Errorf("error saving post: %v", err)
	}
	a.hub.BroadcastToChannel(channel, msg)
	log.Printf("📌 %s created a post in #%s: %s", user, channel, truncate(text, 50))
	return msg.ID, nil
}

func (a *App) GetMessages(channel string) ([]Message, error) {
	messages, err := GetMessages(channel, 100)
	if err != nil {
		log.Printf("Error fetching messages from #%s: %v", channel, err)
		return []Message{}, nil
	}
	return messages, nil
}

func (a *App) AddReaction(messageID, emoji, username, channel string) error {
	messages, err := GetMessages(channel, 1000)
	if err != nil {
		return fmt.Errorf("error fetching messages: %v", err)
	}
	var foundMsg *Message
	for _, msg := range messages {
		if msg.ID == messageID {
			foundMsg = &msg
			break
		}
	}
	if foundMsg == nil {
		return fmt.Errorf("message not found")
	}
	if foundMsg.Reactions == nil {
		foundMsg.Reactions = make(map[string][]string)
	}
	users := foundMsg.Reactions[emoji]
	newUsers := []string{}
	alreadyReacted := false
	for _, u := range users {
		if u == username {
			alreadyReacted = true
			continue
		}
		newUsers = append(newUsers, u)
	}
	if !alreadyReacted {
		newUsers = append(newUsers, username)
	}
	if len(newUsers) == 0 {
		delete(foundMsg.Reactions, emoji)
	} else {
		foundMsg.Reactions[emoji] = newUsers
	}
	if err = UpdateMessage(channel, *foundMsg); err != nil {
		return fmt.Errorf("error updating message: %v", err)
	}
	a.hub.BroadcastToChannel(channel, *foundMsg)
	return nil
}

func (a *App) CreateChannel(name, description, createdBy string) (Channel, error) {
	if name == "" {
		return Channel{}, fmt.Errorf("channel name cannot be empty")
	}

	if strings.HasPrefix(name, "dm_") {
		return Channel{}, fmt.Errorf("invalid channel name")
	}
	existingChannel, err := GetChannel(name)
	if err == nil && existingChannel != nil {
		return Channel{}, fmt.Errorf("channel #%s already exists", name)
	}
	channel := Channel{ID: uuid.New().String(), Name: name, Description: description, Members: []string{createdBy}, CreatedBy: createdBy, CreatedAt: time.Now(), IsPrivate: false}
	if err = SaveChannel(channel); err != nil {
		return Channel{}, fmt.Errorf("failed to create channel: %v", err)
	}
	log.Printf("📢 Channel #%s created by %s", name, createdBy)
	return channel, nil
}

// TODO: CHANGED now returns only public channels, consider adding a parameter to fetch private channels for the users
func (a *App) GetChannels() ([]Channel, error) {
	channels, err := GetAllChannels()
	if err != nil {
		log.Printf("Error fetching channels: %v", err)
		return []Channel{}, nil
	}

	publicChannels := make([]Channel, 0, len(channels))
	for _, channel := range channels {
		if !channel.IsPrivate {
			publicChannels = append(publicChannels, channel)
		}
	}
	return publicChannels, nil
}

func (a *App) GetDMChannels(username string) ([]Channel, error) {
	channels, err := GetAllChannels()
	if err != nil {
		log.Printf("Error fetching channels: %v", err)
		return []Channel{}, nil
	}

	dmChannels := make([]Channel, 0)
	for _, channel := range channels {
		if !channel.IsPrivate && strings.HasPrefix(channel.Name, "dm_") {
			continue
		}
		for _, member := range channel.Members {
			if member == username {
				dmChannels = append(dmChannels, channel)
				break
			}
		}
	}
	return dmChannels, nil
}

func (a *App) GetOrCreateDMChannel(user1, user2 string) (Channel, error) {
	if user1 == user2 {
		return Channel{}, fmt.Errorf("cannot create a DM channel with yourself")
	}
	if user1 == "" || user2 == "" {
		return Channel{}, fmt.Errorf("usernames cannot be empty")
	}

	// Sort usernames to ensure consistent channel naming
	// nevertheless, we can sort them to ensure the channel name is consistent regardless of the order of users
	pair := []string{user1, user2}
	sort.Strings(pair)
	dmName := fmt.Sprintf("dm_%s_%s", pair[0], pair[1])

	existingChannel, err := GetChannel(dmName)
	if err == nil && existingChannel != nil {
		return *existingChannel, nil
	}

	channel := Channel{
		ID:          uuid.New().String(),
		Name:        dmName,
		Description: fmt.Sprintf("Direct message between %s and %s", user1, user2),
		Members:     []string{user1, user2},
		CreatedBy:   "system",
		CreatedAt:   time.Now(),
		IsPrivate:   true,
	}

	if err = SaveChannel(channel); err != nil {
		return Channel{}, fmt.Errorf("failed to create DM channel: %v", err)
	}
	log.Printf("📩 DM channel #%s created between %s and %s", dmName, user1, user2)
	return channel, nil

}

func (a *App) DeleteChannel(name, username string) error {
	channel, err := GetChannel(name)
	if err != nil {
		return fmt.Errorf("channel not found")
	}
	if channel.CreatedBy != username && channel.CreatedBy != "system" {
		return fmt.Errorf("only the channel creator can delete it")
	}
	systemChannels := map[string]bool{"general": true, "random": true, "dev-team": true}
	if systemChannels[name] {
		return fmt.Errorf("cannot delete system channel")
	}
	if err = DeleteChannel(name); err != nil {
		return fmt.Errorf("failed to delete channel: %v", err)
	}
	log.Printf("🗑️ Channel #%s deleted by %s", name, username)
	return nil
}

func (a *App) JoinChannel(channelName, username string) error {
	channel, err := GetChannel(channelName)
	if err != nil {
		return fmt.Errorf("channel not found")
	}
	for _, member := range channel.Members {
		if member == username {
			return nil
		}
	}
	channel.Members = append(channel.Members, username)
	if err = SaveChannel(*channel); err != nil {
		return fmt.Errorf("failed to join channel: %v", err)
	}
	log.Printf("✅ %s joined #%s", username, channelName)
	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
