package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

// Calendar events in Redis.
//
// Redis layout:
//   event:<id>             JSON of the event (answers included)
//   events:index           sorted set, member = event id, score = start (unix)
//   event:<id>:reminded    set of users who already got the reminder

const eventsIndexKey = "events:index"

func eventKey(id string) string         { return "event:" + id }
func eventRemindedKey(id string) string { return "event:" + id + ":reminded" }

func redisSaveEvent(ev *Event) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if err := redisClient.Set(ctx, eventKey(ev.ID), data, 0).Err(); err != nil {
		return err
	}
	return redisClient.ZAdd(ctx, eventsIndexKey, &redis.Z{Score: float64(ev.Start.Unix()), Member: ev.ID}).Err()
}

func redisLoadEvent(id string) (*Event, error) {
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

func redisRemoveEvent(id string) {
	redisClient.Del(ctx, eventKey(id), eventRemindedKey(id))
	redisClient.ZRem(ctx, eventsIndexKey, id)
}

// loadEventsByScore loads the events whose start is within [min, max] seconds.
func loadEventsByScore(min, max int64) ([]Event, error) {
	ids, err := redisClient.ZRangeByScore(ctx, eventsIndexKey, &redis.ZRangeBy{
		Min: strconv.FormatInt(min, 10),
		Max: strconv.FormatInt(max, 10),
	}).Result()
	if err != nil {
		return nil, err
	}
	events := make([]Event, 0, len(ids))
	for _, id := range ids {
		if ev, err := redisLoadEvent(id); err == nil {
			events = append(events, *ev)
		}
	}
	return events, nil
}

func sortEvents(events []Event) {
	sort.SliceStable(events, func(i, j int) bool { return events[i].Start.Before(events[j].Start) })
}

func (redisStore) SaveEvent(ev *Event) error {
	// Same rule as Postgres, where the channel is a foreign key.
	if n, err := redisClient.Exists(ctx, "channel:"+ev.Channel).Result(); err != nil || n == 0 {
		return fmt.Errorf("channel %q not found", ev.Channel)
	}
	return redisSaveEvent(ev)
}

func (redisStore) GetEvent(id string) (*Event, error) { return redisLoadEvent(id) }

func (redisStore) EventsOverlapping(from, to time.Time) ([]Event, error) {
	// An event can start up to maxEventMinutes before `from` and still overlap.
	found, err := loadEventsByScore(from.Add(-maxEventMinutes*time.Minute).Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	events := found[:0]
	for _, ev := range found {
		if ev.End.After(from) && ev.Start.Before(to) {
			events = append(events, ev)
		}
	}
	sortEvents(events)
	return events, nil
}

func (redisStore) EventsStarting(from, to time.Time) ([]Event, error) {
	found, err := loadEventsByScore(from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	events := found[:0]
	for _, ev := range found {
		if !ev.Start.Before(from) && !ev.Start.After(to) {
			events = append(events, ev)
		}
	}
	sortEvents(events)
	return events, nil
}

func (redisStore) SetAnswer(eventID, username, status string) error {
	ev, err := redisLoadEvent(eventID)
	if err != nil {
		return err
	}
	if status == "" {
		delete(ev.RSVP, username)
	} else {
		ev.RSVP[username] = status
	}
	return redisSaveEvent(ev)
}

func (redisStore) RemoveEvent(id string) error {
	redisRemoveEvent(id)
	return nil
}

func (redisStore) RemoveChannelEvents(channel string) error {
	ids, err := redisClient.ZRange(ctx, eventsIndexKey, 0, -1).Result()
	if err != nil {
		return err
	}
	for _, id := range ids {
		ev, err := redisLoadEvent(id)
		if err != nil {
			redisClient.ZRem(ctx, eventsIndexKey, id) // stale index entry
			continue
		}
		if ev.Channel == channel {
			redisRemoveEvent(id)
		}
	}
	return nil
}

func (redisStore) ClaimReminder(eventID, username string) (bool, error) {
	// SADD returns 1 only the first time, so a reminder is sent once.
	added, err := redisClient.SAdd(ctx, eventRemindedKey(eventID), username).Result()
	if err != nil || added == 0 {
		return false, err
	}
	redisClient.Expire(ctx, eventRemindedKey(eventID), 48*time.Hour)
	return true, nil
}

func (redisStore) ResetReminders(eventID string) error {
	return redisClient.Del(ctx, eventRemindedKey(eventID)).Err()
}
