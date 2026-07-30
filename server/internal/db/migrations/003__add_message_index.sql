CREATE INDEX IF NOT EXISTS idx_messages_chat_created_id ON messages(chat_id, created_at DESC, id DESC);
