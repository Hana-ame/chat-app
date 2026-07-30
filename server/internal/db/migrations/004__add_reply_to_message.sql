ALTER TABLE messages ADD COLUMN reply_to_message_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_messages_reply_to ON messages(reply_to_message_id);
