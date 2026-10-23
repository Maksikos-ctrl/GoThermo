package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// Team calendar.
//
// An event belongs to a channel and is visible to everyone who can see that
// channel (public: all users, private channel or DM: its members). People
// answer with going / maybe / declined, and "going" people get a reminder
// shortly before the start.
//
// Redis layout:
//   event:<id>             JSON of the event
//   events:index           sorted set, member = event id, score = start (unix)
//   event:<id>:reminded    set of users who already got the reminder

const (
	eventsIndexKey = "events:index"
	reminderLead   = 15 * time.Minute

	maxEventTitle    = 100
	maxEventDesc     = 500
	minEventMinutes  = 5
	maxEventMinutes  = 24 * 60
	defaultEventMins = 60
	maxEventRange    = 100 * 24 * time.Hour
)

const (
	rsvpGoing    = "going"
	rsvpMaybe    = "maybe"
	rsvpDeclined = "declined"
)

type Event struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Channel     string            `json:"channel"`
	CreatedBy   string            `json:"createdBy"`
	Start       time.Time         `json:"start"`
	End         time.Time         `json:"end"`
	RSVP        map[string]string `json:"rsvp"`
	CreatedAt   time.Time         `json:"createdAt"`
}

// eventMu serializes read-modify-write cycles on events.
var eventMu sync.Mutex

func eventKey(id string) string         { return "event:" + id }
func eventRemindedKey(id string) string { return "event:" + id + ":reminded" }

func saveEvent(ev *Event) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if err := redisClient.Set(ctx, eventKey(ev.ID), data, 0).Err(); err != nil {
		return err
	}
	return redisClient.ZAdd(ctx, eventsIndexKey, &redis.Z{Score: float64(ev.Start.Unix()), Member: ev.ID}).Err()
}

func loadEvent(id string) (*Event, error) {
	data, err := redisClient.Get(ctx, eventKey(id)).Result()
	if err != nil {
		return nil, fmt.Errorf("event not found")
	}
	var ev Event
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		return nil, fmt.Errorf("event not found")
	}
	if ev.RSVP == nil {
		ev.RSVP = map[string]string{}
	}
	return &ev, nil
}

func removeEvent(id string) {
	redisClient.Del(ctx, eventKey(id), eventRemindedKey(id))
	redisClient.ZRem(ctx, eventsIndexKey, id)
}

// deleteChannelEvents drops every event of a channel that is being deleted.
func deleteChannelEvents(channel string) {
	ids, err := redisClient.ZRange(ctx, eventsIndexKey, 0, -1).Result()
	if err != nil {
		return
	}
	for _, id := range ids {
		ev, err := loadEvent(id)
		if err != nil {
			redisClient.ZRem(ctx, eventsIndexKey, id) // stale index entry
			continue
		}
		if ev.Channel == channel {
			removeEvent(id)
		}
	}
}

// eventVisible reports whether a user may see an event: its channel must
// still exist, and a private channel only shows to its members.
func eventVisible(username string, ev *Event, cache map[string]*Channel) bool {
	ch, seen := cache[ev.Channel]
	if !seen {
		c, err := GetChannel(ev.Channel)
		if err != nil {
			c = nil
		}
		ch = c
		cache[ev.Channel] = ch
	}
	if ch == nil {
		return false
	}
	return !ch.IsPrivate || channelHasMember(ch.Members, username)
}

// parseEventTimes validates user input and returns clean values.
func parseEventTimes(startISO string, durationMin int) (time.Time, time.Time, error) {
	start, err := time.Parse(time.RFC3339, strings.TrimSpace(startISO))
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start time")
	}
	if durationMin == 0 {
		durationMin = defaultEventMins
	}
	if durationMin < minEventMinutes || durationMin > maxEventMinutes {
		return time.Time{}, time.Time{}, fmt.Errorf("duration must be between %d minutes and 24 hours", minEventMinutes)
	}
	return start, start.Add(time.Duration(durationMin) * time.Minute), nil
}

func cleanEventText(title, description string) (string, string, error) {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if title == "" {
		return "", "", fmt.Errorf("event title cannot be empty")
	}
	if len([]rune(title)) > maxEventTitle {
		return "", "", fmt.Errorf("event title is too long (max %d characters)", maxEventTitle)
	}
	if len([]rune(description)) > maxEventDesc {
		return "", "", fmt.Errorf("event description is too long (max %d characters)", maxEventDesc)
	}
	return title, description, nil
}

// CreateEvent adds an event to a channel and announces it in the chat.
func (a *App) CreateEvent(token, channel, title, description, startISO string, durationMin int) (Event, error) {
	username, err := sessionUser(token)
	if err != nil {
		return Event{}, err
	}
	ch, err := GetChannel(channel)
	if err != nil || ch == nil {
		return Event{}, fmt.Errorf("channel not found")
	}
	if ch.IsPrivate && !channelHasMember(ch.Members, username) {
		return Event{}, fmt.Errorf("you are not a participant of this chat")
	}
	title, description, err = cleanEventText(title, description)
	if err != nil {
		return Event{}, err
	}
	start, end, err := parseEventTimes(startISO, durationMin)
	if err != nil {
		return Event{}, err
	}

	ev := Event{
		ID:          uuid.New().String(),
		Title:       title,
		Description: description,
		Channel:     channel,
		CreatedBy:   username,
		Start:       start,
		End:         end,
		RSVP:        map[string]string{username: rsvpGoing},
		CreatedAt:   time.Now(),
	}
	eventMu.Lock()
	err = saveEvent(&ev)
	eventMu.Unlock()
	if err != nil {
		return Event{}, fmt.Errorf("failed to save event: %v", err)
	}
	log.Printf("📅 %s created event %q in #%s", username, title, channel)

	// A chat message so the channel notices the event.
	text := fmt.Sprintf("📅 New event: %s, %s–%s", title,
		start.Local().Format("Mon 2 Jan, 15:04"), end.Local().Format("15:04"))
	msg := Message{ID: uuid.New().String(), User: username, Text: text, Channel: channel,
		Timestamp: time.Now(), Reactions: make(map[string][]string)}
	if err := SaveMessage(msg); err == nil && a.hub != nil {
		a.hub.BroadcastToChannel(channel, msg)
	}
	return ev, nil
}

// GetEvents returns the events between two instants that the user may see,
// ordered by start time.
func (a *App) GetEvents(token, fromISO, toISO string) ([]Event, error) {
	username, err := sessionUser(token)
	if err != nil {
		return nil, err
	}
	from, err := time.Parse(time.RFC3339, fromISO)
	if err != nil {
		return nil, fmt.Errorf("invalid range start")
	}
	to, err := time.Parse(time.RFC3339, toISO)
	if err != nil || !to.After(from) {
		return nil, fmt.Errorf("invalid range end")
	}
	if to.Sub(from) > maxEventRange {
		return nil, fmt.Errorf("range is too long")
	}

	// An event is in range if it overlaps it; starts can be up to a day
	// before `from` for an event that is still running.
	ids, err := redisClient.ZRangeByScore(ctx, eventsIndexKey, &redis.ZRangeBy{
		Min: strconv.FormatInt(from.Add(-maxEventMinutes*time.Minute).Unix(), 10),
		Max: strconv.FormatInt(to.Unix(), 10),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to read events")
	}

	cache := map[string]*Channel{}
	events := make([]Event, 0, len(ids))
	for _, id := range ids {
		ev, err := loadEvent(id)
		if err != nil || !ev.End.After(from) || !ev.Start.Before(to) {
			continue
		}
		if eventVisible(username, ev, cache) {
			events = append(events, *ev)
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].Start.Before(events[j].Start) })
	return events, nil
}

// RespondToEvent sets the user's answer: going, maybe, declined, or "" to clear.
func (a *App) RespondToEvent(token, eventID, username, status string) (Event, error) {
	if err := requireUser(token, username); err != nil {
		return Event{}, err
	}
	if status != "" && status != rsvpGoing && status != rsvpMaybe && status != rsvpDeclined {
		return Event{}, fmt.Errorf("invalid answer")
	}
	eventMu.Lock()
	defer eventMu.Unlock()
	ev, err := loadEvent(eventID)
	if err != nil {
		return Event{}, err
	}
	if !eventVisible(username, ev, map[string]*Channel{}) {
		return Event{}, fmt.Errorf("event not found")
	}
	if status == "" {
		delete(ev.RSVP, username)
	} else {
		ev.RSVP[username] = status
	}
	if err := saveEvent(ev); err != nil {
		return Event{}, fmt.Errorf("failed to save answer: %v", err)
	}
	return *ev, nil
}

// UpdateEvent lets the creator change an event.
func (a *App) UpdateEvent(token, eventID, username, title, description, startISO string, durationMin int) (Event, error) {
	if err := requireUser(token, username); err != nil {
		return Event{}, err
	}
	title, description, err := cleanEventText(title, description)
	if err != nil {
		return Event{}, err
	}
	start, end, err := parseEventTimes(startISO, durationMin)
	if err != nil {
		return Event{}, err
	}
	eventMu.Lock()
	defer eventMu.Unlock()
	ev, err := loadEvent(eventID)
	if err != nil {
		return Event{}, err
	}
	if !eventVisible(username, ev, map[string]*Channel{}) {
		return Event{}, fmt.Errorf("event not found")
	}
	if ev.CreatedBy != username {
		return Event{}, fmt.Errorf("only the creator can edit this event")
	}
	moved := !ev.Start.Equal(start)
	ev.Title, ev.Description, ev.Start, ev.End = title, description, start, end
	if err := saveEvent(ev); err != nil {
		return Event{}, fmt.Errorf("failed to save event: %v", err)
	}
	if moved {
		redisClient.Del(ctx, eventRemindedKey(ev.ID)) // remind again for the new time
	}
	return *ev, nil
}

// DeleteEvent lets the creator remove an event.
func (a *App) DeleteEvent(token, eventID, username string) error {
	if err := requireUser(token, username); err != nil {
		return err
	}
	eventMu.Lock()
	defer eventMu.Unlock()
	ev, err := loadEvent(eventID)
	if err != nil {
		return err
	}
	if !eventVisible(username, ev, map[string]*Channel{}) {
		return fmt.Errorf("event not found")
	}
	if ev.CreatedBy != username {
		return fmt.Errorf("only the creator can delete this event")
	}
	removeEvent(eventID)
	return nil
}

// GetDueReminders returns the events that start within the reminder window
// and that this user is going to. Each event is returned once per user.
func (a *App) GetDueReminders(token string) ([]Event, error) {
	username, err := sessionUser(token)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	ids, err := redisClient.ZRangeByScore(ctx, eventsIndexKey, &redis.ZRangeBy{
		Min: strconv.FormatInt(now.Unix(), 10),
		Max: strconv.FormatInt(now.Add(reminderLead).Unix(), 10),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to read events")
	}

	cache := map[string]*Channel{}
	due := []Event{}
	for _, id := range ids {
		ev, err := loadEvent(id)
		if err != nil || ev.RSVP[username] != rsvpGoing || !eventVisible(username, ev, cache) {
			continue
		}
		// SADD returns 1 only the first time, so a reminder is sent once.
		added, err := redisClient.SAdd(ctx, eventRemindedKey(id), username).Result()
		if err != nil || added == 0 {
			continue
		}
		redisClient.Expire(ctx, eventRemindedKey(id), 48*time.Hour)
		due = append(due, *ev)
	}
	return due, nil
}
