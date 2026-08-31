-- 【本地改动 2026-08-31】消息线程聚合：messages.thread_root_message_id 自引用
--，thread_follows 用户关注表，
-- thread_read_state 每个用户每线程的已读游标（用于 has_unread_replies 判断）。

ALTER TABLE messages ADD COLUMN thread_root_message_id TEXT;
CREATE INDEX IF NOT EXISTS idx_messages_thread_root ON messages(thread_root_message_id);

CREATE TABLE IF NOT EXISTS thread_follows (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    thread_root_message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    UNIQUE(user_id, thread_root_message_id)
);
CREATE INDEX IF NOT EXISTS idx_thread_follows_user ON thread_follows(user_id);

CREATE TABLE IF NOT EXISTS thread_read_state (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    thread_root_message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    last_seen_message_id TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(user_id, thread_root_message_id)
);
CREATE INDEX IF NOT EXISTS idx_thread_read_state_user ON thread_read_state(user_id);
