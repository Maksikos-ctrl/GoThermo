-- Version 3: keep the order in which people reacted and the order of pins.
--
-- Redis kept both as list order. A timestamp is not enough (two clicks can
-- land in the same microsecond), so each row gets a generated sequence number
-- that only ever grows.

ALTER TABLE reactions ADD COLUMN seq bigint GENERATED ALWAYS AS IDENTITY;
CREATE INDEX reactions_message_seq_idx ON reactions (message_id, seq);

ALTER TABLE pins ADD COLUMN seq bigint GENERATED ALWAYS AS IDENTITY;
DROP INDEX pins_channel_idx;
CREATE INDEX pins_channel_seq_idx ON pins (channel_id, seq DESC);
