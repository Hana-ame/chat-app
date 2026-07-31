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

另有运行时 `db_fixups.go` 动态补列（`ensureColumn`，幂等）：`chats.avatar_url / banner_url / background_url / banner_opacity`、`chat_members.notify_enabled / unread_count`、`messages.type`（与 001 重复定义，安全）、`users.notify_blocked`，以及 `last_visited_at → last_active_at` 改名。

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
