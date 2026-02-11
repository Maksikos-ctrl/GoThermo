package main

import (
	"context"
	"fmt"
	"log"
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

	// ✅ ДОБАВЛЕНО - сбрасываем все статусы в offline при старте
	userManager.ResetAllStatusesToOffline()

	return &App{hub: hub}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.initDefaultChannels()
	log.Println("✓ GoThermo запущен")
}

func (a *App) initDefaultChannels() {
	channels, err := GetAllChannels()
	if err != nil {
		log.Printf("Ошибка получения каналов: %v", err)
		return
	}
	if len(channels) > 0 {
		log.Printf("✓ Найдено %d каналов", len(channels))
		return
	}
	defaultChannels := []Channel{
		{ID: uuid.New().String(), Name: "general", Description: "General discussions", Members: []string{}, CreatedBy: "system", CreatedAt: time.Now(), IsPrivate: false},
		{ID: uuid.New().String(), Name: "random", Description: "Random stuff", Members: []string{}, CreatedBy: "system", CreatedAt: time.Now(), IsPrivate: false},
		{ID: uuid.New().String(), Name: "dev-team", Description: "Development team", Members: []string{}, CreatedBy: "system", CreatedAt: time.Now(), IsPrivate: false},
	}
	for _, channel := range defaultChannels {
		if err := SaveChannel(channel); err != nil {
			log.Printf("Ошибка создания канала %s: %v", channel.Name, err)
		} else {
			log.Printf("✓ Канал создан: #%s", channel.Name)
		}
	}
}

func (a *App) SendMessage(user, text, channel string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("сообщение не может быть пустым")
	}
	msg := Message{ID: uuid.New().String(), User: user, Text: text, Channel: channel, Timestamp: time.Now(), Reactions: make(map[string][]string), IsPost: false}
	if err := SaveMessage(msg); err != nil {
		return "", fmt.Errorf("не удалось сохранить сообщение: %v", err)
	}
	a.hub.BroadcastToChannel(channel, msg)
	log.Printf("📨 %s -> #%s: %s", user, channel, truncate(text, 50))
	return msg.ID, nil
}

func (a *App) SendPost(user, text, channel string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("пост не может быть пустым")
	}
	msg := Message{ID: uuid.New().String(), User: user, Text: text, Channel: channel, Timestamp: time.Now(), Reactions: make(map[string][]string), IsPost: true}
	if err := SaveMessage(msg); err != nil {
		return "", fmt.Errorf("не удалось сохранить пост: %v", err)
	}
	a.hub.BroadcastToChannel(channel, msg)
	log.Printf("📌 %s создал пост в #%s: %s", user, channel, truncate(text, 50))
	return msg.ID, nil
}

func (a *App) GetMessages(channel string) ([]Message, error) {
	messages, err := GetMessages(channel, 100)
	if err != nil {
		log.Printf("Ошибка получения сообщений из #%s: %v", channel, err)
		return []Message{}, nil
	}
	return messages, nil
}

func (a *App) AddReaction(messageID, emoji, username, channel string) error {
	messages, err := GetMessages(channel, 1000)
	if err != nil {
		return fmt.Errorf("не удалось получить сообщения: %v", err)
	}
	var foundMsg *Message
	for _, msg := range messages {
		if msg.ID == messageID {
			foundMsg = &msg
			break
		}
	}
	if foundMsg == nil {
		return fmt.Errorf("сообщение не найдено")
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
		return fmt.Errorf("не удалось обновить сообщение: %v", err)
	}
	a.hub.BroadcastToChannel(channel, *foundMsg)
	return nil
}

func (a *App) CreateChannel(name, description, createdBy string) (Channel, error) {
	if name == "" {
		return Channel{}, fmt.Errorf("имя канала не может быть пустым")
	}
	existingChannel, err := GetChannel(name)
	if err == nil && existingChannel != nil {
		return Channel{}, fmt.Errorf("канал #%s уже существует", name)
	}
	channel := Channel{ID: uuid.New().String(), Name: name, Description: description, Members: []string{createdBy}, CreatedBy: createdBy, CreatedAt: time.Now(), IsPrivate: false}
	if err = SaveChannel(channel); err != nil {
		return Channel{}, fmt.Errorf("не удалось создать канал: %v", err)
	}
	log.Printf("📢 Канал #%s создан пользователем %s", name, createdBy)
	return channel, nil
}

func (a *App) GetChannels() ([]Channel, error) {
	channels, err := GetAllChannels()
	if err != nil {
		log.Printf("Ошибка получения каналов: %v", err)
		return []Channel{}, nil
	}
	return channels, nil
}

func (a *App) DeleteChannel(name, username string) error {
	channel, err := GetChannel(name)
	if err != nil {
		return fmt.Errorf("канал не найден")
	}
	if channel.CreatedBy != username && channel.CreatedBy != "system" {
		return fmt.Errorf("только создатель канала может его удалить")
	}
	systemChannels := map[string]bool{"general": true, "random": true, "dev-team": true}
	if systemChannels[name] {
		return fmt.Errorf("нельзя удалить системный канал")
	}
	if err = DeleteChannel(name); err != nil {
		return fmt.Errorf("не удалось удалить канал: %v", err)
	}
	log.Printf("🗑️ Канал #%s удален пользователем %s", name, username)
	return nil
}

func (a *App) JoinChannel(channelName, username string) error {
	channel, err := GetChannel(channelName)
	if err != nil {
		return fmt.Errorf("канал не найден")
	}
	for _, member := range channel.Members {
		if member == username {
			return nil
		}
	}
	channel.Members = append(channel.Members, username)
	if err = SaveChannel(*channel); err != nil {
		return fmt.Errorf("не удалось присоединиться к каналу: %v", err)
	}
	log.Printf("✅ %s присоединился к #%s", username, channelName)
	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
