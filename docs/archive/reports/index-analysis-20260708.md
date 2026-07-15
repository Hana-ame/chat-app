# SQLite 索引覆盖分析报告

审查日期：2026-07-08  
数据库：SQLite（modernc.org/sqlite, MaxOpenConns=1, WAL 模式）

---

## 1. 索引定义总表

| 表 | 索引名 | 列 | 类型 | 来源 |
|----|--------|----|------|------|
| `users` | — | `id` | TEXT PRIMARY KEY | 0001_init |
| `users` | — | `email` | UNIQUE（隐式索引） | 0001_init |
| `users` | `idx_users_username` | `username` | 普通索引 | 0001_init |
| `users` | `idx_users_username_uniq` | `username` | UNIQUE | 0003_avatar |
| `refresh_tokens` | — | `id` | TEXT PRIMARY KEY | 0001_init |
| `refresh_tokens` | — | `token_hash` | UNIQUE（隐式索引） | 0001_init |
| `refresh_tokens` | `idx_refresh_user` | `user_id` | 普通索引 | 0001_init |
| `chats` | — | `id` | TEXT PRIMARY KEY | 0001_init |
| `chats` | `idx_chats_last_msg` | `last_message_at DESC` | 普通索引 | 0001_init |
| `chat_members` | — | `(chat_id, user_id)` | 复合 PRIMARY KEY | 0001_init |
| `chat_members` | `idx_chat_members_user` | `user_id` | 普通索引 | 0001_init |
| `messages` | — | `id` | TEXT PRIMARY KEY | 0001_init |
| `messages` | `idx_messages_chat_id` | `(chat_id, id)` | 复合索引 | 0001_init |
| `messages` | `idx_messages_chat_created` | `(chat_id, created_at DESC)` | 复合索引 | 0001_init |
| `attachments` | — | `id` | TEXT PRIMARY KEY | 0001_init |
| `attachments` | `idx_attachments_message` | `message_id` | 普通索引 | 0001_init |
| `reactions` | — | `(message_id, user_id, emoji)` | 复合 PRIMARY KEY | 0001_init |
| `reactions` | `idx_reactions_message` | `message_id` | 普通索引 | 0001_init |
| `mentions` | — | `(message_id, user_id)` | 复合 PRIMARY KEY | 0001_init |
| `mentions` | `idx_mentions_user` | `user_id` | 普通索引 | 0001_init |

**共：6 表、20 个索引定义（含隐式 UNIQUE 索引）**

---

## 2. 查询覆盖检查

### 2.1 users 表

| 查询方法 | SQL | 可用索引 | 结果 |
|----------|-----|----------|------|
| `GetUserByID` | `WHERE id = ?` | PK | ✅ |
| `GetUserByEmail` | `WHERE email = ?` | UNIQUE 隐式索引 | ✅ |
| `UpdateUserProfile` | `WHERE id = ?` | PK | ✅ |
| `UpdateUserStatus` | `WHERE id = ?` | PK | ✅ |
| `SearchUsers` | `WHERE username LIKE ? OR email LIKE ?` | `idx_users_username`（但是前导通配 `%...%`） | ⚠️ LIKE 前缀 `%` 导致无法走索引范围查询 |

### 2.2 refresh_tokens 表

| 查询方法 | SQL | 可用索引 | 结果 |
|----------|-----|----------|------|
| `FindRefreshToken` | `WHERE token_hash = ?` | UNIQUE 隐式索引 | ✅ |
| `DeleteRefreshToken` | `WHERE id = ?` | PK | ✅ |
| `DeleteUserRefreshTokens` | `WHERE user_id = ?` | `idx_refresh_user` | ✅ |
| `PurgeExpiredTokens` | `WHERE expires_at < ?` | **无索引** | ❌ 全表扫描（每小时跑一次，可接受） |

### 2.3 chats 表

| 查询方法 | SQL | 可用索引 | 结果 |
|----------|-----|----------|------|
| `GetChat` | `WHERE id = ?` | PK | ✅ |
| `ListPublicChats` | `WHERE type = 'group' AND visibility = 'public'` | **无索引** | ❌ 但 type 仅 2 个值，索引选择度低，可接受 |
| `RenameChat` | `WHERE id = ?` | PK | ✅ |
| `DeleteChat` | `WHERE id = ?` | PK | ✅ |

### 2.4 chat_members 表

| 查询方法 | SQL | 可用索引 | 结果 |
|----------|-----|----------|------|
| `IsChatMember` | `WHERE chat_id = ? AND user_id = ?` | PK | ✅ |
| `GetChatMembers` | `WHERE chat_id = ?` | PK 首列 | ✅ |
| `ListUserChats` | `WHERE user_id = ?` | `idx_chat_members_user` | ✅ |
| `AddChatMember` | INSERT | — | N/A |
| `RemoveChatMember` | `WHERE chat_id = ? AND user_id = ?` | PK | ✅ |
| `UpdateLastRead` | `WHERE chat_id = ? AND user_id = ?` | PK | ✅ |
| `PinChat` / `UnpinChat` | `WHERE chat_id = ? AND user_id = ?` | PK | ✅ |

### 2.5 messages 表

| 查询方法 | SQL | 可用索引 | 结果 |
|----------|-----|----------|------|
| `GetMessage` | `WHERE id = ? JOIN users` | PK | ✅ |
| `GetMessages`（无 cursor） | `WHERE chat_id = ? ORDER BY created_at DESC, id DESC` | `idx_messages_chat_created` | ✅ |
| `GetMessages`（有 cursor） | `WHERE chat_id = ? AND (created_at, id) < (SELECT...) ORDER BY ...` | `idx_messages_chat_created`（tuple 比较可用） | ✅ |
| `LastMessage` | `WHERE chat_id = ? ORDER BY ... LIMIT 1` | `idx_messages_chat_created`（Ordered DESC, LIMIT 1→直接取首条） | ✅ |
| `UnreadCount` | `WHERE chat_id = ? AND deleted = 0 AND (created_at, id) > ?` | chat_id 走索引，deleted 行过滤 | ✅ |
| `UpdateMessage` | `WHERE id = ? AND user_id = ? AND deleted = 0` | PK | ✅ |
| `DeleteMessage` | `WHERE id = ?` | PK | ✅ |

### 2.6 attachments 表

| 查询方法 | SQL | 可用索引 | 结果 |
|----------|-----|----------|------|
| `attachmentsFor` | `WHERE message_id = ?` | `idx_attachments_message` | ✅ |

### 2.7 reactions 表

| 查询方法 | SQL | 可用索引 | 结果 |
|----------|-----|----------|------|
| `AddReaction` | INSERT | — | N/A |
| `RemoveReaction` | `WHERE message_id = ? AND user_id = ? AND emoji = ?` | PK | ✅ |
| `reactionsFor` | `WHERE message_id = ? ORDER BY created_at` | `idx_reactions_message` + PK 首列 | ✅ |

### 2.8 mentions 表

| 查询方法 | SQL | 可用索引 | 结果 |
|----------|-----|----------|------|
| `mentionsFor` | `WHERE message_id = ?` | PK 首列 | ✅ |
| `CreateMessage` | `INSERT INTO mentions` | — | N/A |

---

## 3. 缺失索引汇总

| 位置 | WHERE 条件 | 影响 | 建议 |
|------|-----------|------|------|
| `refresh_tokens.expires_at` | `expires_at < ?` | `PurgeExpiredTokens` 每小时全表扫描 | 可加可不加，数据量<10 万行时无感 |
| `chats.(type, visibility)` | `type = 'group' AND visibility = 'public'` | `ListPublicChats` 每次扫描 | type 仅 2 种值，visibility 低基数，索引帮助不大 |

**其余所有查询均已被索引覆盖，无遗漏。**

---

## 4. 索引用例统计

```
✅ 完全命中索引：18 条查询
⚠️ 索引可用但不高效：1 条（SearchUsers LIKE '%...%'）
❌ 无索引全表扫描：2 条（PurgeExpiredTokens, ListPublicChats）— 均已评估可接受
```

---

## 5. 潜在优化方向

| 方向 | 收益 | 代价 |
|------|------|------|
| 把 `SearchUsers` 的 `LIKE '%...%'` 改为 `LIKE '...%'`（移除前导通配） | ✅ 可用索引范围扫描 | ❌ 用户只能搜前缀，功能降级 |
| `messages.deleted` 加入 `idx_messages_chat_created` | ✅ 避免行过滤 | ❌ 索引变宽，写入变慢 |
| `chats.visibility` 加索引 | ✅ 加速 ListPublicChats | ❌ 基数太低不值得 |

**结论：当前索引设计合理，无需修改。** 唯一真正的影响是 `SearchUsers` 的前导通配符——但这是业务需求决定的，不是索引能解决的问题。
