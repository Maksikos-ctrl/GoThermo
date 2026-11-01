package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	reminderLead = 15 * time.Minute

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

var eventMu sync.Mutex

// deleteChannelEvents drops every event of a channel that is being deleted.
func deleteChannelEvents(channel string) { store.RemoveChannelEvents(channel) }

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
	err = store.SaveEvent(&ev)
	eventMu.Unlock()
	if err != nil {
		return Event{}, fmt.Errorf("failed to save event: %v", err)
	}
	log.Printf("📅 %s created event %q in #%s", username, title, channel)

	text := fmt.Sprintf("📅 New event: %s, %s–%s", title,
		start.Local().Format("Mon 2 Jan, 15:04"), end.Local().Format("15:04"))
	msg := Message{ID: uuid.New().String(), User: username, Text: text, Channel: channel,
		Timestamp: time.Now(), Reactions: make(map[string][]string)}
	if err := SaveMessage(msg); err == nil && a.hub != nil {
		a.hub.BroadcastToChannel(channel, msg)
	}
	return ev, nil
}

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

	found, err := store.EventsOverlapping(from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to read events")
	}

	cache := map[string]*Channel{}
	events := make([]Event, 0, len(found))
	for i := range found {
		if eventVisible(username, &found[i], cache) {
			events = append(events, found[i])
		}
	}
	return events, nil
}

func (a *App) RespondToEvent(token, eventID, username, status string) (Event, error) {
	if err := requireUser(token, username); err != nil {
		return Event{}, err
	}
	if status != "" && status != rsvpGoing && status != rsvpMaybe && status != rsvpDeclined {
		return Event{}, fmt.Errorf("invalid answer")
	}
	eventMu.Lock()
	defer eventMu.Unlock()
	ev, err := store.GetEvent(eventID)
	if err != nil {
		return Event{}, err
	}
	if !eventVisible(username, ev, map[string]*Channel{}) {
		return Event{}, fmt.Errorf("event not found")
	}
	if err := store.SetAnswer(eventID, username, status); err != nil {
		return Event{}, fmt.Errorf("failed to save answer: %v", err)
	}
	if status == "" {
		delete(ev.RSVP, username)
	} else {
		ev.RSVP[username] = status
	}
	return *ev, nil
}

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
	ev, err := store.GetEvent(eventID)
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
	if err := store.SaveEvent(ev); err != nil {
		return Event{}, fmt.Errorf("failed to save event: %v", err)
	}
	if moved {
		store.ResetReminders(ev.ID)
	}
	return *ev, nil
}

func (a *App) DeleteEvent(token, eventID, username string) error {
	if err := requireUser(token, username); err != nil {
		return err
	}
	eventMu.Lock()
	defer eventMu.Unlock()
	ev, err := store.GetEvent(eventID)
	if err != nil {
		return err
	}
	if !eventVisible(username, ev, map[string]*Channel{}) {
		return fmt.Errorf("event not found")
	}
	if ev.CreatedBy != username {
		return fmt.Errorf("only the creator can delete this event")
	}
	return store.RemoveEvent(eventID)
}

func (a *App) GetDueReminders(token string) ([]Event, error) {
	username, err := sessionUser(token)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	found, err := store.EventsStarting(now, now.Add(reminderLead))
	if err != nil {
		return nil, fmt.Errorf("failed to read events")
	}

	cache := map[string]*Channel{}
	due := []Event{}
	for i := range found {
		ev := &found[i]
		answer := ev.RSVP[username]
		if (answer != rsvpGoing && answer != rsvpMaybe) || !eventVisible(username, ev, cache) {
			continue
		}
		// Only the first call for this event and person gets it.
		if claimed, err := store.ClaimReminder(ev.ID, username); err != nil || !claimed {
			continue
		}
		due = append(due, *ev)
	}
	return due, nil
}
