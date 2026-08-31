-- 通知 occurrences：每用户每事件唯一的持久化通知存储。
-- 设计（持久化通知机制到 SQLite 栈）：
--   - 每行 = 一个用户的一条通知事件（mention/reply 等），(user_id, kind, chat_id,
--     message_id) 唯一 → 同源事件重复触发不产生重复行（数据层兜底，无需应用锁）。
--   - read 标记已读；expires_at 为 TTL，由启动时的定期 worker 清理（与 token 清理同款）。
--   - FK ON DELETE CASCADE：删用户自动清空其通知。
-- SQLite 的 UNIQUE 约束对 NULL 放开，但我们约定这些列一律 NOT NULL，保证唯一性实义。
CREATE TABLE IF NOT EXISTS notification_occurrences (
  id         TEXT PRIMARY KEY,
  user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind       TEXT NOT NULL,                -- 'mention' | 'reply' | 'system'
  chat_id    TEXT NOT NULL,
  message_id TEXT NOT NULL DEFAULT '',     -- 触发事件（消息）id；system 通知可为空串
  actor_id   TEXT NOT NULL DEFAULT '',     -- 触发者（发送/回复消息的人）
  title      TEXT NOT NULL DEFAULT '',
  body       TEXT NOT NULL DEFAULT '',
  read       INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  UNIQUE (user_id, kind, chat_id, message_id)
);

CREATE INDEX IF NOT EXISTS idx_notification_occurrences_user
  ON notification_occurrences(user_id, read, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notification_occurrences_expiry
  ON notification_occurrences(expires_at);
