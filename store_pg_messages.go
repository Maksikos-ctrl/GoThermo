package main

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// Messages, threads, pins and files in Postgres (see db/migrations).
//
// A channel is addressed by its name, as everywhere in the app, and found
// through channels.name. Replies are rows of the same table with thread_id set.
// ReplyCount / LastReplyAt / ReplyUsers of a parent are computed when it is read.

func nullIfZero64(n int64) any {
	if n == 0 {
		return nil
	}
	return n
}

func nullIfZero(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

var messageSelect = fmt.Sprintf(`
	SELECT m.id, c.name, m.author, m.body, m.created_at, m.edited_at IS NOT NULL, m.is_post,
	       coalesce(m.file_id, ''), coalesce(m.file_name, ''), coalesce(m.file_size, 0), coalesce(m.mime_type, ''),
	       m.is_call, coalesce(m.call_status, ''), coalesce(m.call_duration, 0), coalesce(m.call_initiator, ''),
	       coalesce(m.thread_id, ''),
	       coalesce(rc.n, 0), rc.last,
	       ARRAY(SELECT x.a FROM (SELECT r.author AS a, min(r.seq) AS s FROM messages r
	                              WHERE r.thread_id = m.id GROUP BY r.author ORDER BY min(r.seq) LIMIT %d) x
	             ORDER BY x.s)
	FROM messages m
	JOIN channels c ON c.id = m.channel_id
	LEFT JOIN LATERAL (SELECT count(*)::int AS n, max(r.created_at) AS last
	                   FROM messages r WHERE r.thread_id = m.id) rc ON true`, maxReplyAvatars)

func scanMessage(row pgx.Row) (Message, error) {
	var (
		m      Message
		edited bool
		last   *time.Time
		users  []string
	)
	if err := row.Scan(&m.ID, &m.Channel, &m.User, &m.Text, &m.Timestamp, &edited, &m.IsPost,
		&m.FileID, &m.FileName, &m.FileSize, &m.MimeType,
		&m.IsCall, &m.CallStatus, &m.CallDuration, &m.CallInitiator,
		&m.ThreadID, &m.ReplyCount, &last, &users); err != nil {
		return Message{}, err
	}
	m.IsEdited = edited
	m.IsFile = m.FileID != ""
	if last != nil {
		m.LastReplyAt = last.Format(time.RFC3339Nano)
	}
	if len(users) > 0 {
		m.ReplyUsers = users
	}
	m.Reactions = map[string][]string{}
	return m, nil
}

// queryMessages runs a messageSelect-based query and attaches the reactions.
func (p pgStore) queryMessages(where string, args ...any) ([]Message, error) {
	rows, err := p.pool.Query(ctx, messageSelect+" "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	msgs := []Message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	return msgs, p.attachReactions(msgs)
}

func (p pgStore) attachReactions(msgs []Message) error {
	if len(msgs) == 0 {
		return nil
	}
	index := make(map[string]int, len(msgs))
	ids := make([]string, len(msgs))
	for i, m := range msgs {
		index[m.ID] = i
		ids[i] = m.ID
	}
	rows, err := p.pool.Query(ctx,
		"SELECT message_id, emoji, username FROM reactions WHERE message_id = ANY($1) ORDER BY seq", ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, emoji, user string
		if err := rows.Scan(&id, &emoji, &user); err != nil {
			return err
		}
		m := msgs[index[id]]
		m.Reactions[emoji] = append(m.Reactions[emoji], user)
	}
	return rows.Err()
}

func reverseMessages(msgs []Message) {
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
}

// ───────────────────────── writing ─────────────────────────

func (p pgStore) AppendMessage(m Message) error {
	ts := m.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		INSERT INTO messages (id, channel_id, thread_id, author, body, created_at, edited_at, is_post,
		                      file_id, file_name, file_size, mime_type,
		                      is_call, call_status, call_duration, call_initiator)
		SELECT $2, c.id, $3, $4, $5, $6::timestamptz, CASE WHEN $7::boolean THEN $6::timestamptz END, $8,
		       $9, $10, $11, $12, $13, $14, $15, $16
		FROM channels c WHERE c.name = $1`,
		m.Channel, m.ID, nullIfEmpty(m.ThreadID), m.User, m.Text, ts, m.IsEdited, m.IsPost,
		nullIfEmpty(m.FileID), nullIfEmpty(m.FileName), nullIfZero64(m.FileSize), nullIfEmpty(m.MimeType),
		m.IsCall, nullIfEmpty(m.CallStatus), nullIfZero(m.CallDuration), nullIfEmpty(m.CallInitiator))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("channel %q not found", m.Channel)
	}
	if err := syncReactions(tx, m.ID, m.Reactions); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// updateRow replaces the editable fields of a channel message (parentID "")
// or of a reply (parentID set).
func (p pgStore) updateRow(channel, parentID string, m Message) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE messages SET
			body = $4,
			edited_at = CASE WHEN $5::boolean THEN coalesce(edited_at, $6::timestamptz) END,
			is_post = $7,
			file_id = $8, file_name = $9, file_size = $10, mime_type = $11,
			is_call = $12, call_status = $13, call_duration = $14, call_initiator = $15
		WHERE id = $1
		  AND channel_id = (SELECT id FROM channels WHERE name = $2)
		  AND thread_id IS NOT DISTINCT FROM $3::text`,
		m.ID, channel, nullIfEmpty(parentID), m.Text, m.IsEdited, time.Now(), m.IsPost,
		nullIfEmpty(m.FileID), nullIfEmpty(m.FileName), nullIfZero64(m.FileSize), nullIfEmpty(m.MimeType),
		m.IsCall, nullIfEmpty(m.CallStatus), nullIfZero(m.CallDuration), nullIfEmpty(m.CallInitiator))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("message with ID %s not found", m.ID)
	}
	if err := syncReactions(tx, m.ID, m.Reactions); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// syncReactions makes the stored reactions equal to the map. Reactions that
// stay keep their place in the order; new ones go to the end.
func syncReactions(tx pgx.Tx, messageID string, want map[string][]string) error {
	type key struct{ emoji, user string }
	wanted := map[key]bool{}
	for emoji, users := range want {
		for _, u := range users {
			wanted[key{emoji, u}] = true
		}
	}

	rows, err := tx.Query(ctx, "SELECT emoji, username FROM reactions WHERE message_id = $1", messageID)
	if err != nil {
		return err
	}
	have := map[key]bool{}
	for rows.Next() {
		var k key
		if err := rows.Scan(&k.emoji, &k.user); err != nil {
			rows.Close()
			return err
		}
		have[k] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for k := range have {
		if !wanted[k] {
			if _, err := tx.Exec(ctx, "DELETE FROM reactions WHERE message_id = $1 AND emoji = $2 AND username = $3",
				messageID, k.emoji, k.user); err != nil {
				return err
			}
		}
	}
	emojis := make([]string, 0, len(want))
	for e := range want {
		emojis = append(emojis, e)
	}
	sort.Strings(emojis)
	for _, emoji := range emojis {
		for _, u := range want[emoji] {
			k := key{emoji, u}
			if have[k] {
				continue
			}
			have[k] = true
			if _, err := tx.Exec(ctx, "INSERT INTO reactions (message_id, emoji, username) VALUES ($1, $2, $3)",
				messageID, emoji, u); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p pgStore) UpdateMessage(channel string, m Message) error { return p.updateRow(channel, "", m) }

func (p pgStore) DeleteMessage(channel, id string) error {
	tag, err := p.pool.Exec(ctx, `
		DELETE FROM messages
		WHERE id = $2 AND thread_id IS NULL AND channel_id = (SELECT id FROM channels WHERE name = $1)`,
		channel, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("message with ID %s not found", id)
	}
	return nil
}

// ───────────────────────── reading ─────────────────────────

func (p pgStore) RecentMessages(channel string, limit int64) ([]Message, error) {
	var lim any
	if limit > 0 {
		lim = limit
	}
	msgs, err := p.queryMessages(
		"WHERE c.name = $1 AND m.thread_id IS NULL ORDER BY m.seq DESC LIMIT $2", channel, lim)
	if err != nil {
		return nil, err
	}
	reverseMessages(msgs)
	return msgs, nil
}

func (p pgStore) MessagePage(channel, beforeID string, limit int64) (MessagePage, error) {
	var (
		msgs []Message
		err  error
	)
	if beforeID == "" {
		msgs, err = p.queryMessages(
			"WHERE c.name = $1 AND m.thread_id IS NULL ORDER BY m.seq DESC LIMIT $2", channel, limit+1)
	} else {
		var seq int64
		err = p.pool.QueryRow(ctx, `
			SELECT m.seq FROM messages m JOIN channels c ON c.id = m.channel_id
			WHERE c.name = $1 AND m.id = $2 AND m.thread_id IS NULL`, channel, beforeID).Scan(&seq)
		if errors.Is(err, pgx.ErrNoRows) {
			// the message the client scrolled from is gone: nothing sensible to send
			return MessagePage{Messages: []Message{}}, nil
		}
		if err != nil {
			return MessagePage{}, err
		}
		msgs, err = p.queryMessages(
			"WHERE c.name = $1 AND m.thread_id IS NULL AND m.seq < $2 ORDER BY m.seq DESC LIMIT $3",
			channel, seq, limit+1)
	}
	if err != nil {
		return MessagePage{}, err
	}
	more := int64(len(msgs)) > limit
	if more {
		msgs = msgs[:limit]
	}
	reverseMessages(msgs)
	return MessagePage{Messages: msgs, HasMore: more}, nil
}

func (p pgStore) FindMessage(channel, id string) (*Message, error) {
	msgs, err := p.queryMessages("WHERE c.name = $1 AND m.id = $2 AND m.thread_id IS NULL", channel, id)
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("message not found")
	}
	return &msgs[0], nil
}

// ───────────────────────── threads ─────────────────────────

func (p pgStore) Replies(channel, parentID string) ([]Message, error) {
	return p.queryMessages("WHERE c.name = $1 AND m.thread_id = $2 ORDER BY m.seq", channel, parentID)
}

func (p pgStore) ReplyCount(channel, parentID string) (int, error) {
	var n int
	err := p.pool.QueryRow(ctx, `
		SELECT count(*) FROM messages m JOIN channels c ON c.id = m.channel_id
		WHERE c.name = $1 AND m.thread_id = $2`, channel, parentID).Scan(&n)
	return n, err
}

func (p pgStore) AddReply(r Message) error {
	if r.ThreadID == "" {
		return fmt.Errorf("a reply needs the id of its parent message")
	}
	return p.AppendMessage(r)
}

func (p pgStore) UpdateReply(channel, parentID string, r Message) error {
	return p.updateRow(channel, parentID, r)
}

func (p pgStore) DeleteReply(channel, parentID, replyID string) error {
	tag, err := p.pool.Exec(ctx, `
		DELETE FROM messages
		WHERE id = $3 AND thread_id = $2 AND channel_id = (SELECT id FROM channels WHERE name = $1)`,
		channel, parentID, replyID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("message not found")
	}
	return nil
}

// RefreshThreadMeta: the counters are derived on every read, so there is
// nothing to recompute; this just returns the parent.
func (p pgStore) RefreshThreadMeta(channel, parentID string) (*Message, error) {
	return p.FindMessage(channel, parentID)
}

func (p pgStore) DeleteThread(channel, parentID string) error {
	_, err := p.pool.Exec(ctx, `
		DELETE FROM messages
		WHERE thread_id = $2 AND channel_id = (SELECT id FROM channels WHERE name = $1)`, channel, parentID)
	return err
}

func (p pgStore) MarkThreadRead(username, channel, parentID string) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO thread_reads (username, thread_id, read_at)
		SELECT $1, m.id, $4 FROM messages m JOIN channels c ON c.id = m.channel_id
		WHERE c.name = $2 AND m.id = $3 AND m.thread_id IS NULL
		ON CONFLICT (username, thread_id) DO UPDATE SET read_at = EXCLUDED.read_at`,
		username, channel, parentID, time.Now())
	return err
}

func (p pgStore) ThreadUnread(username, channel string) (map[string]int, error) {
	// First time we look at this person: everything so far counts as seen.
	now := time.Now()
	if _, err := p.pool.Exec(ctx,
		"UPDATE users SET thread_unread_since = $2 WHERE username = $1 AND thread_unread_since IS NULL",
		username, now); err != nil {
		return nil, err
	}
	base := now
	var stored *time.Time
	err := p.pool.QueryRow(ctx, "SELECT thread_unread_since FROM users WHERE username = $1", username).Scan(&stored)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if stored != nil {
		base = *stored
	}

	rows, err := p.pool.Query(ctx, `
		SELECT r.thread_id, count(*)::int
		FROM messages r
		JOIN channels c ON c.id = r.channel_id
		LEFT JOIN thread_reads tr ON tr.thread_id = r.thread_id AND tr.username = $1
		WHERE c.name = $2 AND r.thread_id IS NOT NULL AND r.author <> $1
		  AND r.created_at > GREATEST($3::timestamptz, coalesce(tr.read_at, $3::timestamptz))
		GROUP BY r.thread_id`, username, channel, base)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// ───────────────────────── pins ─────────────────────────

func (p pgStore) PinEntries(channel string) ([]pinEntry, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT p.message_id, p.pinned_by, p.pinned_at
		FROM pins p JOIN channels c ON c.id = p.channel_id
		WHERE c.name = $1 ORDER BY p.seq DESC`, channel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []pinEntry{}
	for rows.Next() {
		var e pinEntry
		if err := rows.Scan(&e.MessageID, &e.PinnedBy, &e.PinnedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (p pgStore) AddPin(channel string, e pinEntry) error {
	tag, err := p.pool.Exec(ctx, `
		INSERT INTO pins (message_id, channel_id, pinned_by, pinned_at)
		SELECT m.id, m.channel_id, $3, $4 FROM messages m JOIN channels c ON c.id = m.channel_id
		WHERE c.name = $1 AND m.id = $2 AND m.thread_id IS NULL
		ON CONFLICT (message_id) DO NOTHING`, channel, e.MessageID, e.PinnedBy, e.PinnedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// already pinned is fine; a message that does not exist is not
		if _, err := p.FindMessage(channel, e.MessageID); err != nil {
			return err
		}
	}
	return nil
}

func (p pgStore) RemovePin(channel, messageID string) (bool, error) {
	tag, err := p.pool.Exec(ctx, `
		DELETE FROM pins WHERE message_id = $2 AND channel_id = (SELECT id FROM channels WHERE name = $1)`,
		channel, messageID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (p pgStore) ResolvedPins(channel string) ([]PinnedMessage, error) {
	entries, err := p.PinEntries(channel)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return []PinnedMessage{}, nil
	}
	ids := make([]string, len(entries))
	for i, e := range entries {
		ids[i] = e.MessageID
	}
	msgs, err := p.queryMessages("WHERE c.name = $1 AND m.id = ANY($2)", channel, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]Message, len(msgs))
	for _, m := range msgs {
		byID[m.ID] = m
	}
	pins := make([]PinnedMessage, 0, len(entries))
	for _, e := range entries {
		if m, ok := byID[e.MessageID]; ok {
			pins = append(pins, PinnedMessage{Message: m, PinnedBy: e.PinnedBy, PinnedAt: e.PinnedAt})
		}
	}
	return pins, nil
}

func (p pgStore) DeleteChannelPins(channel string) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM pins WHERE channel_id = (SELECT id FROM channels WHERE name = $1)`, channel)
	return err
}

// ───────────────────────── files ─────────────────────────

func (p pgStore) SaveFileMeta(fileID, filePath, mimeType, fileName string) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO files (id, path, mime_type, file_name) VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET path = EXCLUDED.path, mime_type = EXCLUDED.mime_type, file_name = EXCLUDED.file_name`,
		fileID, filePath, mimeType, fileName)
	return err
}

func (p pgStore) FileMeta(fileID string) (path, mimeType, fileName string, err error) {
	err = p.pool.QueryRow(ctx, "SELECT path, mime_type, file_name FROM files WHERE id = $1", fileID).
		Scan(&path, &mimeType, &fileName)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", fmt.Errorf("file metadata not found")
	}
	return path, mimeType, fileName, err
}

func (p pgStore) DeleteFileMeta(fileID string) error {
	_, err := p.pool.Exec(ctx, "DELETE FROM files WHERE id = $1", fileID)
	return err
}
