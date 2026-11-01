package main

import (
	"fmt"
	"time"
)

// Calendar events in Postgres: events, event_rsvps (the answers) and
// event_reminders (who already got the reminder). Deleting a channel removes
// its events, and deleting an event removes its answers and reminders, by
// foreign keys.

const eventSelect = `
	SELECT e.id, c.name, e.title, e.description, e.created_by, e.start_at, e.end_at, e.created_at
	FROM events e JOIN channels c ON c.id = e.channel_id`

func (p pgStore) queryEvents(where string, args ...any) ([]Event, error) {
	rows, err := p.pool.Query(ctx, eventSelect+" "+where+" ORDER BY e.start_at, e.created_at, e.id", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []Event{}
	index := map[string]int{}
	ids := []string{}
	for rows.Next() {
		var ev Event
		if err := rows.Scan(&ev.ID, &ev.Channel, &ev.Title, &ev.Description, &ev.CreatedBy,
			&ev.Start, &ev.End, &ev.CreatedAt); err != nil {
			return nil, err
		}
		ev.RSVP = map[string]string{}
		index[ev.ID] = len(events)
		ids = append(ids, ev.ID)
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return events, nil
	}
	answers, err := p.pool.Query(ctx, "SELECT event_id, username, status FROM event_rsvps WHERE event_id = ANY($1)", ids)
	if err != nil {
		return nil, err
	}
	defer answers.Close()
	for answers.Next() {
		var id, user, status string
		if err := answers.Scan(&id, &user, &status); err != nil {
			return nil, err
		}
		events[index[id]].RSVP[user] = status
	}
	return events, answers.Err()
}

// SaveEvent creates the event or replaces its title, description, time and
// answers. The channel, creator and creation time of an existing event stay.
func (p pgStore) SaveEvent(ev *Event) error {
	created := ev.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		INSERT INTO events (id, channel_id, title, description, created_by, start_at, end_at, created_at)
		SELECT $2, c.id, $3, $4, $5, $6, $7, $8 FROM channels c WHERE c.name = $1
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title, description = EXCLUDED.description,
			start_at = EXCLUDED.start_at, end_at = EXCLUDED.end_at`,
		ev.Channel, ev.ID, ev.Title, ev.Description, ev.CreatedBy, ev.Start, ev.End, created)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("channel %q not found", ev.Channel)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM event_rsvps WHERE event_id = $1", ev.ID); err != nil {
		return err
	}
	for user, status := range ev.RSVP {
		if _, err := tx.Exec(ctx,
			"INSERT INTO event_rsvps (event_id, username, status) VALUES ($1, $2, $3)", ev.ID, user, status); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (p pgStore) GetEvent(id string) (*Event, error) {
	events, err := p.queryEvents("WHERE e.id = $1", id)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("event not found")
	}
	return &events[0], nil
}

func (p pgStore) EventsOverlapping(from, to time.Time) ([]Event, error) {
	return p.queryEvents("WHERE e.end_at > $1 AND e.start_at < $2", from, to)
}

func (p pgStore) EventsStarting(from, to time.Time) ([]Event, error) {
	return p.queryEvents("WHERE e.start_at >= $1 AND e.start_at <= $2", from, to)
}

func (p pgStore) SetAnswer(eventID, username, status string) error {
	var exists bool
	if err := p.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM events WHERE id = $1)", eventID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("event not found")
	}
	if status == "" {
		_, err := p.pool.Exec(ctx, "DELETE FROM event_rsvps WHERE event_id = $1 AND username = $2", eventID, username)
		return err
	}
	_, err := p.pool.Exec(ctx, `
		INSERT INTO event_rsvps (event_id, username, status) VALUES ($1, $2, $3)
		ON CONFLICT (event_id, username) DO UPDATE SET status = EXCLUDED.status`,
		eventID, username, status)
	return err
}

func (p pgStore) RemoveEvent(id string) error {
	_, err := p.pool.Exec(ctx, "DELETE FROM events WHERE id = $1", id)
	return err
}

func (p pgStore) RemoveChannelEvents(channel string) error {
	_, err := p.pool.Exec(ctx,
		"DELETE FROM events WHERE channel_id = (SELECT id FROM channels WHERE name = $1)", channel)
	return err
}

func (p pgStore) ClaimReminder(eventID, username string) (bool, error) {
	tag, err := p.pool.Exec(ctx, `
		INSERT INTO event_reminders (event_id, username) VALUES ($1, $2)
		ON CONFLICT (event_id, username) DO NOTHING`, eventID, username)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (p pgStore) ResetReminders(eventID string) error {
	_, err := p.pool.Exec(ctx, "DELETE FROM event_reminders WHERE event_id = $1", eventID)
	return err
}
