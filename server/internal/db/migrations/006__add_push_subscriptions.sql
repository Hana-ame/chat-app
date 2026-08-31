-- Web Push 订阅（移植 chatto 的 push subscription 语义到 SQLite 栈）。
-- 设计：
--   - 每行 = 一个用户在某浏览器/设备上的一条推送订阅（endpoint 由浏览器
--     PushManager 签发，全局唯一）。
--   - p256dh/auth 是浏览器订阅的加密密钥（RFC 8291），后端发推送时用它们
--     加密 payload；VAPID 私钥在后端签名，推送服务验签确认发送者身份。
--   - endpoint UNIQUE：同一端点的重复注册直接覆盖（数据层兜底，无需应用锁）。
--   - 发送返回 404/410（订阅失效）时由 service 删除该行；FK ON DELETE
--     CASCADE 保证删用户自动清空其订阅。
--   - 不设 TTL：订阅随用户注销/设备退订即时删除，不留僵尸行。
CREATE TABLE IF NOT EXISTS push_subscriptions (
  id         TEXT PRIMARY KEY,
  user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  endpoint   TEXT NOT NULL UNIQUE,
  p256dh     TEXT NOT NULL,
  auth       TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_push_subscriptions_user
  ON push_subscriptions(user_id);