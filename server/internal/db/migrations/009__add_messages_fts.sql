-- 【本地改动 2026-09-03】消息全文搜索（FTS5）。
--
-- 用 SQLite FTS5 虚拟表索引 messages.content（tokenize='unicode61' 默认分词器）。
--
-- 表结构：`content` 为可搜索字段；`msg_id` 为 UNINDEXED 辅助列，存储
-- messages.id（UUID 字符串），用于 JOIN 回查原消息。UNINDEXED 确保 msg_id
-- 只存储不进入搜索索引（否则用户搜 UUID 也可能命中）。
--
-- 用 `INSERT OR REPLACE` 语义维护：msg_id 隐式唯一（FTS5 内部对 UNINDEXED
-- 列隐式 UNIQUE 约束），重复写入自动替换（对齐 UpdateMessage 的 upsert 语义）。
-- 注意：此点踩坑——一开始用 FTS5 rebuild/integrate 控制操作，但 modernc.org/sqlite
-- 对 FTS5 external content 支持不完整（rowid 强制 INTEGER，与 UUID msg ID 冲突）；
-- 改用 UNINDEXED 辅助列方案后 INSERT OR REPLACE 正常。
--
-- 搜索 SQL：
--   SELECT ... FROM messages m
--   INNER JOIN messages_fts f ON f.msg_id = m.id
--   WHERE f.content MATCH ?
--
-- MATCH 语义：空格分词，多词默认 OR；"" 精确短语；* 前缀通配；AND 逻辑运算。
--
-- 边界：仅索引 messages.content 文本，不索引 attachments/mentions/reactions；
-- 这些字段需要搜索时再增列。

CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
    content,
    msg_id UNINDEXED,
    tokenize='unicode61'
);
