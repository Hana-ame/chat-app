ALTER TABLE chats ADD COLUMN last_message_user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE chats ADD COLUMN last_message_content TEXT NOT NULL DEFAULT '';
ALTER TABLE chats ADD COLUMN last_message_created_at TEXT NOT NULL DEFAULT '';

ALTER TABLE chat_members ADD COLUMN unread_count INTEGER NOT NULL DEFAULT 0;
