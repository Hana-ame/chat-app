-- 【本地改动 2026-09-02】Chatto FDR-037 消息置顶：多消息置顶（区别于单条公告 pinned_message）。
--
-- 设计：chat_pins 表存储 chat↔message 的置顶关联，(chat_id, message_id) 唯一，
-- 支持多消息置顶（分页 sidebar）。DM 不支持置顶（由 service 层拒绝）。
-- 每条消息每条聊天最多一条 pin（幂等）。消息删除时不自动 unpin（简化实现，
-- 由前端在列表中过滤不可读消息）。
-- 边界：只用于消息置顶；现有 chat 表的 pinned_message JSON 字段是「聊天公告」，
-- 与本功能完全独立（不修改、不冲突）。
-- 索引：(chat_id, created_at DESC) 保证列表按置顶时间倒序高效；
--        (message_id) 支持批量清理；FK 级联删 chat/message。
-- 踩坑：若同时删聊天和消息，FK CASCADE 自动清理 pin 行，无需应用锁。

CREATE TABLE IF NOT EXISTS chat_pins (
    id                   TEXT PRIMARY KEY,
    chat_id              TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    message_id           TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    pinned_by            TEXT NOT NULL REFERENCES users(id),
    created_at           TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_pins_chat_message
    ON chat_pins(chat_id, message_id);

CREATE INDEX IF NOT EXISTS idx_chat_pins_chat_created
    ON chat_pins(chat_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_chat_pins_message
    ON chat_pins(message_id);
