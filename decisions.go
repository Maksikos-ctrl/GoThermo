package main

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Decisions and tasks: any message of a channel can be marked as a decision or
// a task, and the channel's "Decisions" list collects them so nothing is lost
// in the stream. A task can be ticked off. The text is copied at the moment of
// marking, so the entry stays readable if the message is edited or deleted.
//
// Storage: the Redis hash "decisions:<channel>", message id -> JSON entry.
// (Like reports, this lives in Redis for every store; Redis is already
// required for sessions and keeps its data on disk in the compose setup.)

const (
	maxDecisionsPerChannel = 200
	maxDecisionTextRunes   = 500
)

var decisionKinds = map[string]bool{"decision": true, "task": true}

type Decision struct {
	MessageID string    `json:"messageId"`
	Channel   string    `json:"channel"`
	Kind      string    `json:"kind"`
	Text      string    `json:"text"`
	Author    string    `json:"author"`
	MarkedBy  string    `json:"markedBy"`
	Assignee  string    `json:"assignee,omitempty"`
	Done      bool      `json:"done"`
	DoneBy    string    `json:"doneBy,omitempty"`
	At        time.Time `json:"at"`
}

func decisionsKey(channel string) string { return "decisions:" + channel }

var mentionRe = regexp.MustCompile(`@([A-Za-z0-9_]+)`)

// firstMentionedUser is the first @name in the text that is a real account.
func firstMentionedUser(text string) string {
	for _, m := range mentionRe.FindAllStringSubmatch(text, -1) {
		if u, ok := userManager.GetUser(m[1]); ok && u != nil {
			return u.Username
		}
	}
	return ""
}

func trimRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

func loadDecision(channel, messageID string) (*Decision, error) {
	raw, err := redisClient.HGet(ctx, decisionsKey(channel), messageID).Result()
	if err != nil {
		return nil, err
	}
	var d Decision
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func saveDecision(d Decision) error {
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return redisClient.HSet(ctx, decisionsKey(d.Channel), d.MessageID, b).Err()
}

// deleteChannelDecisions removes the list of a channel (channel deleted).
func deleteChannelDecisions(channel string) {
	redisClient.Del(ctx, decisionsKey(channel))
}

// MarkDecision marks a message as a "decision" or a "task". Marking it again
// only changes the kind.
func (a *App) MarkDecision(token, user, channel, messageID, kind string) error {
	if err := requireUser(token, user); err != nil {
		return err
	}
	if err := requireChannelAccess(user, channel); err != nil {
		return err
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	if !decisionKinds[kind] {
		return fmt.Errorf("choose decision or task")
	}
	msg, err := findChannelMessage(channel, messageID)
	if err != nil || msg == nil {
		return fmt.Errorf("that message no longer exists")
	}
	if msg.IsCall {
		return fmt.Errorf("a call log cannot be marked")
	}

	if existing, err := loadDecision(channel, messageID); err == nil {
		existing.Kind = kind
		if kind == "decision" {
			existing.Done, existing.DoneBy = false, ""
		}
		if err := saveDecision(*existing); err != nil {
			return internalFailure("decision update", err)
		}
		return nil
	}

	n, err := redisClient.HLen(ctx, decisionsKey(channel)).Result()
	if err != nil {
		return internalFailure("decision count", err)
	}
	if n >= maxDecisionsPerChannel {
		return fmt.Errorf("this channel already has %d entries, remove some first", maxDecisionsPerChannel)
	}

	text := msg.Text
	if text == "" && msg.IsFile {
		text = "📎 " + msg.FileName
	}
	d := Decision{
		MessageID: messageID, Channel: channel, Kind: kind,
		Text: trimRunes(text, maxDecisionTextRunes), Author: msg.User, MarkedBy: user,
		At: time.Now().UTC(),
	}
	if kind == "task" {
		d.Assignee = firstMentionedUser(msg.Text)
	}
	if err := saveDecision(d); err != nil {
		return internalFailure("decision save", err)
	}
	log.Printf("✅ %s marked a %s in #%s", user, kind, channel)
	return nil
}

// GetDecisions lists the entries of a channel, newest first.
func (a *App) GetDecisions(token, channel string) ([]Decision, error) {
	user, err := sessionUser(token)
	if err != nil {
		return nil, err
	}
	if err := requireChannelAccess(user, channel); err != nil {
		return nil, err
	}
	all, err := redisClient.HGetAll(ctx, decisionsKey(channel)).Result()
	if err != nil {
		return nil, internalFailure("decision list", err)
	}
	out := make([]Decision, 0, len(all))
	for _, raw := range all {
		var d Decision
		if json.Unmarshal([]byte(raw), &d) == nil {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out, nil
}

// SetDecisionDone ticks a task off or back on. Anyone in the channel may.
func (a *App) SetDecisionDone(token, user, channel, messageID string, done bool) error {
	if err := requireUser(token, user); err != nil {
		return err
	}
	if err := requireChannelAccess(user, channel); err != nil {
		return err
	}
	d, err := loadDecision(channel, messageID)
	if err != nil {
		return fmt.Errorf("that entry no longer exists")
	}
	if d.Kind != "task" {
		return fmt.Errorf("only tasks can be ticked off")
	}
	d.Done = done
	d.DoneBy = ""
	if done {
		d.DoneBy = user
	}
	if err := saveDecision(*d); err != nil {
		return internalFailure("decision done", err)
	}
	return nil
}

// RemoveDecision takes an entry off the list. The person who marked it and the
// author of the message may do that.
func (a *App) RemoveDecision(token, user, channel, messageID string) error {
	if err := requireUser(token, user); err != nil {
		return err
	}
	if err := requireChannelAccess(user, channel); err != nil {
		return err
	}
	d, err := loadDecision(channel, messageID)
	if err != nil {
		return nil // already gone
	}
	if d.MarkedBy != user && d.Author != user {
		return fmt.Errorf("only the person who marked it or its author can remove it")
	}
	if err := redisClient.HDel(ctx, decisionsKey(channel), messageID).Err(); err != nil {
		return internalFailure("decision remove", err)
	}
	return nil
}
