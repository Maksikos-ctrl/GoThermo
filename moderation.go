package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Reports and bans.
//
// A report is kept in Redis (list "reports", newest first, at most
// maxStoredReports). The gothermo-admin tool lists them and bans accounts.
// A ban is the Redis set "banned_users": a banned account cannot sign in and
// every session it still has stops working. Keep the key names in sync with
// cmd/gothermo-admin.
const (
	reportsKey        = "reports"
	bannedKey         = "banned_users"
	maxStoredReports  = 1000
	maxReportsPerHour = 20
	reportSnippetLen  = 300
)

var errSuspended = errors.New("this account has been suspended")

var reportReasons = map[string]bool{
	"spam":       true,
	"nsfw":       true,
	"harassment": true,
	"malicious":  true,
	"other":      true,
}

type Report struct {
	ID        string    `json:"id"`
	Reporter  string    `json:"reporter"`
	Author    string    `json:"author"`
	Channel   string    `json:"channel"`
	MessageID string    `json:"messageId"`
	Reason    string    `json:"reason"`
	Snippet   string    `json:"snippet"`
	FileName  string    `json:"fileName,omitempty"`
	At        time.Time `json:"at"`
}

func isBanned(username string) bool {
	if username == "" {
		return false
	}
	ok, err := redisClient.SIsMember(ctx, bannedKey, strings.ToLower(username)).Result()
	return err == nil && ok
}

func snippet(s string) string {
	if utf8.RuneCountInString(s) <= reportSnippetLen {
		return s
	}
	return string([]rune(s)[:reportSnippetLen]) + "…"
}

// ReportMessage lets a member flag a message of someone else in a channel they
// can see. Reporting the same message twice is accepted and ignored.
func (a *App) ReportMessage(token, user, channel, messageID, reason string) error {
	if err := requireUser(token, user); err != nil {
		return err
	}
	if err := requireChannelAccess(user, channel); err != nil {
		return err
	}
	reason = strings.ToLower(strings.TrimSpace(reason))
	if !reportReasons[reason] {
		return fmt.Errorf("pick a reason for the report")
	}
	msg, err := findChannelMessage(channel, messageID)
	if err != nil || msg == nil {
		return fmt.Errorf("that message no longer exists")
	}
	if msg.User == user {
		return fmt.Errorf("you cannot report your own message")
	}

	rateKey := "reportrate:" + strings.ToLower(user)
	n, err := redisClient.Incr(ctx, rateKey).Result()
	if err != nil {
		return internalFailure("report rate", err)
	}
	if n == 1 {
		redisClient.Expire(ctx, rateKey, time.Hour)
	}
	if n > maxReportsPerHour {
		return fmt.Errorf("too many reports, try again later")
	}

	first, err := redisClient.SetNX(ctx, "reported:"+strings.ToLower(user)+":"+msg.ID, "1", 30*24*time.Hour).Result()
	if err != nil {
		return internalFailure("report dedupe", err)
	}
	if !first {
		return nil
	}

	r := Report{
		ID: uuid.New().String(), Reporter: user, Author: msg.User, Channel: channel,
		MessageID: msg.ID, Reason: reason, Snippet: snippet(msg.Text), At: time.Now().UTC(),
	}
	if msg.IsFile {
		r.FileName = msg.FileName
	}
	b, err := json.Marshal(r)
	if err != nil {
		return internalFailure("report", err)
	}
	pipe := redisClient.TxPipeline()
	pipe.LPush(ctx, reportsKey, b)
	pipe.LTrim(ctx, reportsKey, 0, maxStoredReports-1)
	if _, err := pipe.Exec(ctx); err != nil {
		return internalFailure("report save", err)
	}
	return nil
}

// sessionFailure is what a failed CreateSession becomes for the person: the
// suspension notice as it is, anything else a generic message.
func sessionFailure(what string, err error) error {
	if errors.Is(err, errSuspended) {
		return err
	}
	return internalFailure(what, err)
}
