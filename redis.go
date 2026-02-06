package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-redis/redis/v8"
)

var ctx = context.Background()
var redisClient *redis.Client

func initRedis() {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		fmt.Println("Не удалось подключиться к Redis:", err)
	} else {
		fmt.Println("✓ Успешно подключено к Redis")
	}
}

func SaveChannel(channel Channel) error {
	data, err := json.Marshal(channel)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("channel:%s", channel.Name)
	err = redisClient.Set(ctx, key, data, 0).Err()
	if err != nil {
		return err
	}

	return redisClient.SAdd(ctx, "channels", channel.Name).Err()
}

func GetChannel(name string) (*Channel, error) {
	key := fmt.Sprintf("channel:%s", name)
	data, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var channel Channel
	if err := json.Unmarshal([]byte(data), &channel); err != nil {
		return nil, err
	}

	return &channel, nil
}

func GetAllChannels() ([]Channel, error) {
	channelNames, err := redisClient.SMembers(ctx, "channels").Result()
	if err != nil {
		return nil, err
	}

	channels := make([]Channel, 0, len(channelNames))
	for _, name := range channelNames {
		channel, err := GetChannel(name)
		if err == nil {
			channels = append(channels, *channel)
		}
	}

	return channels, nil
}

func DeleteChannel(name string) error {
	err := redisClient.SRem(ctx, "channels", name).Err()
	if err != nil {
		return err
	}

	key := fmt.Sprintf("channel:%s", name)
	return redisClient.Del(ctx, key).Err()
}

func SaveMessage(msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("channel:%s:messages", msg.Channel)
	return redisClient.RPush(ctx, key, data).Err()
}

func GetMessages(channel string, limit int64) ([]Message, error) {
	key := fmt.Sprintf("channel:%s:messages", channel)

	result, err := redisClient.LRange(ctx, key, -limit, -1).Result()
	if err != nil {
		return nil, err
	}

	messages := make([]Message, 0, len(result))
	for _, data := range result {
		if data == "DELETED" {
			continue
		}

		var msg Message
		if err := json.Unmarshal([]byte(data), &msg); err == nil {
			messages = append(messages, msg)
		}
	}

	return messages, nil
}

func UpdateMessage(channel string, updatedMsg Message) error {
	key := fmt.Sprintf("channel:%s:messages", channel)

	messages, err := redisClient.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return err
	}

	for i, msgData := range messages {
		if msgData == "DELETED" {
			continue
		}

		var msg Message
		if err := json.Unmarshal([]byte(msgData), &msg); err != nil {
			continue
		}

		if msg.ID == updatedMsg.ID {
			updatedData, err := json.Marshal(updatedMsg)
			if err != nil {
				return err
			}

			return redisClient.LSet(ctx, key, int64(i), updatedData).Err()
		}
	}

	return fmt.Errorf("сообщение с ID %s не найдено", updatedMsg.ID)
}

func SaveUserToRedis(user *User) error {
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("user:%s", user.Email)
	return redisClient.Set(ctx, key, data, 0).Err()
}

func GetUserFromRedis(email string) (*User, error) {
	key := fmt.Sprintf("user:%s", email)
	data, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var user User
	if err := json.Unmarshal([]byte(data), &user); err != nil {
		return nil, err
	}

	return &user, nil
}

// Получаем всех пользователей из Redis
func GetAllUsersFromRedis() ([]User, error) {
	// Получаем все ключи пользователей
	keys, err := redisClient.Keys(ctx, "user:*").Result()
	if err != nil {
		return nil, err
	}

	var users []User
	for _, key := range keys {
		data, err := redisClient.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var user User
		if err := json.Unmarshal([]byte(data), &user); err == nil {
			users = append(users, user)
		}
	}

	return users, nil
}
