package main

import "time"

type Message struct {
	ID        string              `json:"id"`
	User      string              `json:"user"`
	Text      string              `json:"text"`
	Channel   string              `json:"channel"`
	Timestamp time.Time           `json:"timestamp"`
	Reactions map[string][]string `json:"reactions"` // emoji -> [usernames]
	IsPost    bool                `json:"isPost"`
}

type Channel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Members     []string  `json:"members"`
	CreatedBy   string    `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
	IsPrivate   bool      `json:"isPrivate"`
}

type Reaction struct {
	MessageID string `json:"messageId"`
	Emoji     string `json:"emoji"`
	Username  string `json:"username"`
}
