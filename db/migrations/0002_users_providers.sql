-- Version 2: sign-in providers live on the user.
--
-- Found while porting the code: a Google account and a password account may
-- share an email address (the Redis layout keeps them under different keys on
-- purpose). So the email must be unique only among password accounts, and a
-- Google account is identified by (provider, provider_id) instead of by the
-- separate google_links table, which is no longer needed.

ALTER TABLE users
    ADD COLUMN provider    text,
    ADD COLUMN provider_id text,
    ADD CONSTRAINT users_provider_pair CHECK ((provider IS NULL) = (provider_id IS NULL));

DROP INDEX users_email_key;
CREATE UNIQUE INDEX users_email_key    ON users (lower(email)) WHERE provider IS NULL;
CREATE UNIQUE INDEX users_provider_key ON users (provider, provider_id) WHERE provider IS NOT NULL;

DROP TABLE google_links;

-- Public channels keep their creator in the member list too (the app has
-- always saved it that way), so this table is not only for private chats.
COMMENT ON TABLE channel_members IS
    'Member list exactly as the app saves it: creator first. Private channels and DMs rely on it for access.';
