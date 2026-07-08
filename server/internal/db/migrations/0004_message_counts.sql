-- Add count columns to messages to avoid N+1 queries on attachExtras.
-- Set at write time, read to skip SELECT from attachments/mentions/reactions tables.
ALTER TABLE messages ADD COLUMN attachment_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE messages ADD COLUMN mention_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE messages ADD COLUMN reaction_count INTEGER NOT NULL DEFAULT 0;