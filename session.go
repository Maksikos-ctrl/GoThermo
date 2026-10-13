package main

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const sessionTTL = 30 * 24 * time.Hour

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func CreateSession(username string) (string, error) {
	token, err := newSessionToken()
	if err != nil {
		return "", err
	}
	if err := redisClient.Set(ctx, "session:"+token, username, sessionTTL).Err(); err != nil {
		return "", err
	}
	return token, nil
}

func GetSessionUsername(token string) (string, bool) {
	if token == "" {
		return "", false
	}
	username, err := redisClient.Get(ctx, "session:"+token).Result()
	if err != nil || username == "" {
		return "", false
	}
	return username, true
}

func DeleteSession(token string) {
	if token == "" {
		return
	}
	redisClient.Del(ctx, "session:"+token)
}

func withToken(user *User, token string) User {
	u := *user
	u.Token = token
	return u
}
