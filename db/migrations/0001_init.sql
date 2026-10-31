-- GoThermo schema, version 1.
--
-- Durable data lives here. Sessions, login rate limits, link-preview cache,
-- typing and presence stay in Redis (they are short-lived by nature).
--
-- Conventions
--   * ids are text (the app already generates uuid strings; text avoids any
--     format surprise while migrating old data)
--   * people are referenced by username, exactly like the code does today.
--     No foreign key to users: history written by "system" or by an account
--     that is gone must stay readable
--   * everything that belongs to a channel or a message is removed by
--     ON DELETE CASCADE, which replaces the hand-written cleanup in
--     DeleteChannel (messages, threads, pins, read marks, events)

CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ───────────────────────── users ─────────────────────────

CREATE TABLE users (
    id                  text        PRIMARY KEY,
    username            text        NOT NULL,
    email               text        NOT NULL,
    password_hash       text,                       -- NULL for Google-only accounts
    status              text        NOT NULL DEFAULT 'offline',
    last_seen_at        timestamptz,
    thread_unread_since timestamptz,                -- starting point for "unread replies"
    created_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_username_not_empty CHECK (username <> ''),
    CONSTRAINT users_email_not_empty    CHECK (email <> '')
);
CREATE UNIQUE INDEX users_username_key ON users (username);
CREATE UNIQUE INDEX users_email_key    ON users (lower(email));

-- "Sign in with Google": the Google account id (sub) -> our user.
CREATE TABLE google_links (
    sub        text        PRIMARY KEY,
    username   text        NOT NULL REFERENCES users (username) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX google_links_username_idx ON google_links (username);

-- ───────────────────────── channels ─────────────────────────

CREATE TABLE channels (
    id          text        PRIMARY KEY,
    name        text        NOT NULL UNIQUE,
    description text        NOT NULL DEFAULT '',
    is_private  boolean     NOT NULL DEFAULT false,  -- true for private channels and dm_...
    created_by  text        NOT NULL,                -- a username, or "system"
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT channels_name_not_empty CHECK (name <> '')
);

-- Members of private channels and direct messages. Public channels have none.
-- position keeps the order the app shows (creator first).
CREATE TABLE channel_members (
    channel_id text        NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    username   text        NOT NULL,
    position   integer     NOT NULL,
    added_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (channel_id, username)
);
CREATE INDEX channel_members_username_idx ON channel_members (username);

-- ───────────────────────── messages ─────────────────────────

-- One table for channel messages and thread replies. A reply points at its
-- parent with thread_id; a channel message has thread_id NULL.
-- seq gives a stable insertion order (what the Redis list position was). It is
-- generated, never written by the app, so it needs no unique index of its own:
-- the per-channel indexes below do the work.
CREATE TABLE messages (
    seq            bigint      GENERATED ALWAYS AS IDENTITY,
    id             text        PRIMARY KEY,
    channel_id     text        NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    thread_id      text        REFERENCES messages (id) ON DELETE CASCADE,
    author         text        NOT NULL,
    body           text        NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    edited_at      timestamptz,
    is_post        boolean     NOT NULL DEFAULT false,

    -- file attachment (the bytes are on disk; see files)
    file_id        text,
    file_name      text,
    file_size      bigint,
    mime_type      text,

    -- call log entry
    is_call        boolean     NOT NULL DEFAULT false,
    call_status    text,
    call_duration  integer,
    call_initiator text,

    CONSTRAINT messages_not_own_thread CHECK (thread_id IS NULL OR thread_id <> id)
);

-- History of a channel, newest page first, and paging by cursor.
CREATE INDEX messages_channel_seq_idx ON messages (channel_id, seq DESC) WHERE thread_id IS NULL;
-- Replies of one thread, oldest first.
CREATE INDEX messages_thread_seq_idx  ON messages (thread_id, seq)       WHERE thread_id IS NOT NULL;
-- Who wrote what (user history, exports).
CREATE INDEX messages_author_idx      ON messages (author);
-- Deleting a file / finding its message.
CREATE INDEX messages_file_idx        ON messages (file_id) WHERE file_id IS NOT NULL;
-- Search: ILIKE '%word%' without scanning every message.
CREATE INDEX messages_body_trgm_idx   ON messages USING gin (body gin_trgm_ops);

CREATE TABLE reactions (
    message_id text        NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    emoji      text        NOT NULL,
    username   text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),  -- keeps the order people reacted in
    PRIMARY KEY (message_id, emoji, username),
    CONSTRAINT reactions_emoji_not_empty CHECK (emoji <> '')
);

-- Pinned messages. A message can be pinned once.
CREATE TABLE pins (
    message_id text        PRIMARY KEY REFERENCES messages (id) ON DELETE CASCADE,
    channel_id text        NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    pinned_by  text        NOT NULL,
    pinned_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pins_channel_idx ON pins (channel_id, pinned_at DESC);

-- Uploaded files: where the bytes are on disk.
CREATE TABLE files (
    id         text        PRIMARY KEY,
    path       text        NOT NULL,
    mime_type  text        NOT NULL DEFAULT 'application/octet-stream',
    file_name  text        NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

-- ───────────────────────── read marks ─────────────────────────

-- user:<u>:lastread:<channel> in Redis.
CREATE TABLE channel_reads (
    username     text        NOT NULL,
    channel_id   text        NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    last_read_at timestamptz NOT NULL,
    PRIMARY KEY (username, channel_id)
);

-- threadread:<u> in Redis. The thread is its parent message.
CREATE TABLE thread_reads (
    username  text        NOT NULL,
    thread_id text        NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    read_at   timestamptz NOT NULL,
    PRIMARY KEY (username, thread_id)
);

-- ───────────────────────── calendar ─────────────────────────

CREATE TABLE events (
    id          text        PRIMARY KEY,
    channel_id  text        NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    title       text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    created_by  text        NOT NULL,
    start_at    timestamptz NOT NULL,
    end_at      timestamptz NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT events_title_not_empty CHECK (title <> ''),
    CONSTRAINT events_ends_after_start CHECK (end_at > start_at)
);
CREATE INDEX events_start_idx   ON events (start_at);
CREATE INDEX events_channel_idx ON events (channel_id);

CREATE TABLE event_rsvps (
    event_id text NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    username text NOT NULL,
    status   text NOT NULL,
    PRIMARY KEY (event_id, username),
    CONSTRAINT event_rsvps_status CHECK (status IN ('going', 'maybe', 'declined'))
);

-- Who already got the reminder (event:<id>:reminded in Redis).
CREATE TABLE event_reminders (
    event_id text        NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    username text        NOT NULL,
    sent_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, username)
);
