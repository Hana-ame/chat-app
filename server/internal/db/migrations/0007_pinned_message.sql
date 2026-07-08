ALTER TABLE chats ADD COLUMN pinned_message TEXT NOT NULL DEFAULT '';
ALTER TABLE chats ADD COLUMN pinned_updated_at TEXT;
