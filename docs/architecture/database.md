# 数据库

SQLite（WAL 模式、`foreign_keys=ON`、`synchronous=NORMAL`）。schema 由迁移文件维护，启动时自动执行缺失的迁移。

## 迁移

`server/internal/db/migrations/`：

| 文件 | 内容 |
|---|---|
| `000__init.sql` | 全量初始 schema（幂等 CREATE TABLE IF NOT EXISTS） |
| `001__add_type_column.sql` | `messages.type`（''=普通，'stream'=AI 流式） |
| `002__add_thinking_column.sql` | `messages.thinking`（AI 推理块） |
| `003__add_message_index.sql` | `(chat_id, created_at DESC, id DESC)` 复合索引 |
| `004__add_reply_to_message.sql` | `messages.reply_to_message_id` + 索引 |
| `005__add_notification_occurrences.sql` | 【本地改动 2026-08-31】持久化通知：`notification_occurrences` 表（每用户每事件唯一）+ 未读/过期索引 |

另有运行时 `db_fixups.go` 动态补列（`ensureColumn`，幂等）：`chats.avatar_url / banner_url / background_url / banner_opacity / last_message_*`、`chat_members.notify_enabled / unread_count`、`messages.type`（与 001 重复定义，安全）、`users.notify_blocked`，以及 `last_visited_at → last_active_at` 改名。

**列补齐机制（v0.9.5+）**：`ensureSchemaColumns` 在**每次启动无条件执行**（不依赖迁移版本记录），任何旧库缺列都能自愈。历史教训：列补齐曾挂在 go 迁移版本（v3/v4）下，旧库记录"已应用"后新列永不补齐，线上报 `no such column`。往列清单加列不需要新增迁移版本；go 迁移只保留一次性结构变更（如 `chats` 表重建去掉 CHECK 约束）。

## 表结构

### users

| 列 | 说明 |
|---|---|
| id | UUID v4 (TEXT) PK |
| email / username | UNIQUE；username 另有唯一索引 |
| password_hash | bcrypt |
| avatar_color | 默认 `#5865F2` |
| status / last_seen | 在线状态 / 最后活跃 |
| avatar_url | 头像 |
| notify_blocked | JSON 数组（忽略通知的用户 id） |

### refresh_tokens

| 列 | 说明 |
|---|---|
| id | PK |
| user_id | FK → users ON DELETE CASCADE |
| token_hash | 存储 refresh token 的哈希（UNIQUE） |
| expires_at / created_at | 时间戳 |

### chats

| 列 | 说明 |
|---|---|
| id / type | type: `group` / `dm` / `notify` |
| name / icon_color | 显示信息 |
| owner_id | FK → users ON DELETE SET NULL |
| last_message_at / last_message_id | 列表预览（由写路径维护） |
| member_count | 冗余计数 |
| visibility | `public` / `unlisted` / `private` |
| pinned_message / pinned_updated_at | 置顶公告 |
| avatar_url / banner_url / background_url / banner_opacity | 群组视觉（运行时补列） |

### chat_members

| 列 | 说明 |
|---|---|
| chat_id + user_id | 复合 PK，均 FK ON DELETE CASCADE |
| role | `owner` / `admin` / '' |
| joined_at / last_active_at | 加入时间 / 最后活跃 |
| last_read_message_id / pinned_last_read_at | 已读位置 |
| pinned | 是否置顶聊天 |
| notify_enabled / unread_count | 通知开关 / 未读计数（运行时补列） |

### messages

| 列 | 说明 |
|---|---|
| id / chat_id / user_id | FK（chat/user ON DELETE CASCADE） |
| content | 消息正文 |
| created_at / edited_at / deleted_at | 时间戳（软删除标记） |
| type / thinking / reply_to_message_id | AI 流式与回复（001/002/004 迁移） |
| attachment_count / mention_count / reaction_count | 冗余计数（读侧免聚合） |
| reactions / attachments / mentions | **预聚合 JSON 缓存列**（`reactions` 表 + `attachments`/`mentions` 表的镜像） |

索引：`(chat_id, id)`、`(chat_id, created_at DESC)`、`(chat_id, created_at DESC, id DESC)`、`(reply_to_message_id)`。

### notification_occurrences（【本地改动 2026-08-31】移植 chatto 持久化通知）

| 列 | 说明 |
|---|---|
| id | UUID (TEXT) PK |
| user_id | FK → users ON DELETE CASCADE（收件人） |
| kind | `mention` / `reply` / `system` |
| chat_id / message_id | 触发通知的聊天与源消息 |
| actor_id | 触发者（发送/回复消息的人） |
| title / body | 通知展示内容（body 截断 120 字符） |
| read | 是否已读 |
| created_at / expires_at | 创建时间 / TTL（默认 90 天，清理 worker 删除过期行） |

唯一约束 `UNIQUE(user_id, kind, chat_id, message_id)`：每用户每事件唯一，重复触发
（同一条源消息被重复投递）不重复插行、不重置已读——数据层兜底，无需应用锁
（与 notify chat 的「锁 + 唯一索引」同思路，但这里是单一 INSERT，锁不必要）。
索引：`(user_id, read, created_at DESC)`（列表/未读）、`(expires_at)`（清理）。

### attachments / reactions / mentions

| 表 | 说明 |
|---|---|
| attachments | 附件行（filename/mime_type/size/url），FK → messages |
| reactions | 每行 = 一个用户对一条消息的一个 emoji，PK `(message_id, user_id, emoji)` |
| mentions | 提及关系 PK `(message_id, user_id)` |

`messages.reactions` JSON 缓存由 `AddReaction`/`RemoveReaction` 同步（`syncReactionsColumn`），保证与 `reactions` 表一致。

## 设计决策

- **写路径维护冗余**（counts + JSON 缓存）：聊天列表/消息列表读侧零子查询
- **TEXT 时间戳**：UTC ISO-8601（RFC3339Nano），人类可读且避免时区坑
- **软删除**：`deleted_at` 标记，广播 `message_delete` 后保留行（引用完整性）
- **外键 ON DELETE CASCADE**：删聊天/删用户自动清理从表
