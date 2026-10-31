package main

import (
	"errors"
	"time"
)

// Storage for the durable data. Today Redis holds everything; Postgres is
// being added behind this interface one area at a time (see db/migrations).
// Callers keep using the plain functions below (SaveChannel, GetChannel, ...);
// they forward to whichever store is active.
//
// Contract, the same for every implementation:
//   - a missing record is an error (the caller only checks err != nil)
//   - deleting something that does not exist is not an error
//   - callers must not rely on nil versus an empty slice

// errNotFound is what the Postgres store returns for a missing record.
var errNotFound = errors.New("not found")

type Store interface {
	// Users. Email/password accounts and provider (Google) accounts are
	// different kinds of record and may share an email address.
	SaveUser(u *User) error
	GetUserByEmail(email string) (*User, error) // password accounts only
	ListUsers() ([]User, error)
	SetPasswordHash(email, hash string) error
	PasswordHash(email string) (string, error)
	UsernameByProvider(provider, providerID string) (username string, found bool, err error)

	// Channels (including private channels and DMs, which are channels too).
	SaveChannel(c Channel) error // create or replace, keyed by name
	GetChannel(name string) (*Channel, error)
	ListChannels() ([]Channel, error)
	DeleteChannel(name string) error

	// When a person last opened a channel.
	SaveLastRead(username, channel string) error
	LastRead(username, channel string) (t time.Time, found bool, err error)

	// Messages of a channel, addressed by the channel's name. A reply is a
	// message too, but it belongs to a thread and never shows up here.
	AppendMessage(m Message) error
	RecentMessages(channel string, limit int64) ([]Message, error) // newest `limit`, oldest first
	MessagePage(channel, beforeID string, limit int64) (MessagePage, error)
	FindMessage(channel, id string) (*Message, error)
	// UpdateMessage replaces what can change: text, edited flag, reactions and
	// the attachment / call fields. Thread counters are not stored, see below.
	UpdateMessage(channel string, m Message) error
	DeleteMessage(channel, id string) error // also removes its thread, reactions and pin

	// Threads. ReplyCount / LastReplyAt / ReplyUsers of a parent message are
	// always derived from its replies.
	Replies(channel, parentID string) ([]Message, error) // oldest first
	ReplyCount(channel, parentID string) (int, error)
	AddReply(r Message) error // r.Channel and r.ThreadID say where it goes
	UpdateReply(channel, parentID string, r Message) error
	DeleteReply(channel, parentID, replyID string) error
	RefreshThreadMeta(channel, parentID string) (*Message, error)
	DeleteThread(channel, parentID string) error
	MarkThreadRead(username, channel, parentID string) error
	ThreadUnread(username, channel string) (map[string]int, error) // parent id -> unread replies

	// Pins, newest first.
	PinEntries(channel string) ([]pinEntry, error)
	AddPin(channel string, e pinEntry) error
	RemovePin(channel, messageID string) (bool, error)
	ResolvedPins(channel string) ([]PinnedMessage, error)
	DeleteChannelPins(channel string) error

	// Uploaded files: where the bytes are on disk.
	SaveFileMeta(fileID, filePath, mimeType, fileName string) error
	FileMeta(fileID string) (path, mimeType, fileName string, err error)
	DeleteFileMeta(fileID string) error
}

// store is the active implementation. Redis until the migration is finished.
var store Store = redisStore{}

// The names below are what the rest of the code has always called.

func SaveChannel(channel Channel) error            { return store.SaveChannel(channel) }
func GetChannel(name string) (*Channel, error)     { return store.GetChannel(name) }
func GetAllChannels() ([]Channel, error)           { return store.ListChannels() }
func DeleteChannel(name string) error              { return store.DeleteChannel(name) }
func SaveUserToRedis(user *User) error             { return store.SaveUser(user) }
func GetUserFromRedis(email string) (*User, error) { return store.GetUserByEmail(email) }
func GetAllUsersFromRedis() ([]User, error)        { return store.ListUsers() }
func SaveUserPasswordToRedis(email, hash string) error {
	return store.SetPasswordHash(email, hash)
}
func GetUserPasswordFromRedis(email string) (string, error) { return store.PasswordHash(email) }
func SaveLastRead(username, channel string) error           { return store.SaveLastRead(username, channel) }
func GetLastRead(username, channel string) (time.Time, bool, error) {
	return store.LastRead(username, channel)
}

func SaveMessage(msg Message) error { return store.AppendMessage(msg) }
func GetMessages(channel string, limit int64) ([]Message, error) {
	return store.RecentMessages(channel, limit)
}
func UpdateMessage(channel string, msg Message) error { return store.UpdateMessage(channel, msg) }
func DeleteMessageInRedis(channel, messageID string) error {
	return store.DeleteMessage(channel, messageID)
}
func SaveFileMeta(fileID, filePath, mimeType, fileName string) error {
	return store.SaveFileMeta(fileID, filePath, mimeType, fileName)
}
func GetFileMeta(fileID string) (path, mimeType, fileName string, err error) {
	return store.FileMeta(fileID)
}

// ───────────────────────── Redis implementation ─────────────────────────

// redisStore is the existing code in redis.go, unchanged.
type redisStore struct{}

func (redisStore) SaveUser(u *User) error {
	if err := redisSaveUser(u); err != nil {
		return err
	}
	// Google accounts are also found by their Google id.
	if u.Provider == "google" && u.ProviderID != "" {
		return redisClient.Set(ctx, "google:"+u.ProviderID, u.Username, 0).Err()
	}
	return nil
}

func (redisStore) GetUserByEmail(email string) (*User, error) { return redisGetUser(email) }
func (redisStore) ListUsers() ([]User, error)                 { return redisListUsers() }
func (redisStore) SetPasswordHash(email, hash string) error   { return redisSetPassword(email, hash) }
func (redisStore) PasswordHash(email string) (string, error)  { return redisGetPassword(email) }

func (redisStore) UsernameByProvider(provider, providerID string) (string, bool, error) {
	username, err := redisClient.Get(ctx, provider+":"+providerID).Result()
	if err != nil {
		return "", false, nil
	}
	return username, true, nil
}

func (redisStore) SaveChannel(c Channel) error           { return redisSaveChannel(c) }
func (redisStore) GetChannel(n string) (*Channel, error) { return redisGetChannel(n) }
func (redisStore) ListChannels() ([]Channel, error)      { return redisListChannels() }
func (redisStore) DeleteChannel(n string) error          { return redisDeleteChannel(n) }

func (redisStore) SaveLastRead(username, channel string) error {
	return redisSaveLastRead(username, channel)
}
func (redisStore) LastRead(username, channel string) (time.Time, bool, error) {
	return redisGetLastRead(username, channel)
}
