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
| `006__add_push_subscriptions.sql` | 【本地改动 2026-08-31】Web Push：`push_subscriptions` 表（endpoint 全局唯一，p256dh/auth 加密密钥，FK 级联删用户） |
| `007__add_threads.sql` | 【本地改动 2026-08-31】线程聚合：`messages.thread_root_message_id`（自引用 FK，顶层为空、StartThread 自指、回复继承祖先根）+ `thread_follows`（每用户每根唯一，关注通知 opt-in）+ `thread_read_state`（每用户每根的已读游标） |
| `008__add_chat_pins.sql` | 【本地改动 2026-09-02】消息置顶（chatto FDR-037，多消息，区别于聊天公告）：`chat_pins` 表（chat↔message 关联，幂等唯一索引），FK CASCADE 清理 |

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
| type / thinking / reply_to_message_id / thread_root_message_id | AI 流式与回复（001/002/004 迁移） + 【本地改动 2026-08-31】线程归属根 ID（自引用 FK，顶层消息为空、StartThread 自指、回复继承祖先根） |
| attachment_count / mention_count / reaction_count | 冗余计数（读侧免聚合） |
| reactions / attachments / mentions | **预聚合 JSON 缓存列**（`reactions` 表 + `attachments`/`mentions` 表的镜像） |

索引：`(chat_id, id)`、`(chat_id, created_at DESC)`、`(chat_id, created_at DESC, id DESC)`、`(reply_to_message_id)`、【本地改动 2026-08-31】`(thread_root_message_id, chat_id, created_at DESC)`。

### notification_occurrences（【本地改动 2026-08-31】持久化通知）

| 列 | 说明 |
|---|---|
| id | UUID (TEXT) PK |
| user_id | FK → users ON DELETE CASCADE（收件人） |
| kind | `mention` / `reply` / `reply_in_thread` / `system` |
| chat_id / message_id | 触发通知的聊天与源消息 |
| actor_id | 触发者（发送/回复消息的人） |
| title / body | 通知展示内容（body 截断 120 字符） |
| read | 是否已读 |
| created_at / expires_at | 创建时间 / TTL（默认 90 天，清理 worker 删除过期行） |

唯一约束 `UNIQUE(user_id, kind, chat_id, message_id)`：每用户每事件唯一，重复触发
（同一条源消息被重复投递）不重复插行、不重置已读——数据层兜底，无需应用锁
（与 notify chat 的「锁 + 唯一索引」同思路，但这里是单一 INSERT，锁不必要）。
索引：`(user_id, read, created_at DESC)`（列表/未读）、`(expires_at)`（清理）。

### push_subscriptions（【本地改动 2026-08-31】Web Push）

| 列 | 类型 | 说明 |
|---|---|---|
| id | TEXT PK | 订阅 id（db.NewID） |
| user_id | TEXT NOT NULL FK→users ON DELETE CASCADE | 订阅归属；删用户自动清空 |
| endpoint | TEXT NOT NULL UNIQUE | 浏览器 PushManager 签发端点，全局唯一；重复注册覆盖归属（SavePushSubscription 两步写法：先 DO NOTHING 区分新插/覆盖，再 UPDATE） |
| p256dh | TEXT NOT NULL | RFC 8291 加密公钥（浏览器订阅密钥） |
| auth | TEXT NOT NULL | RFC 8291 加密 auth 密钥 |
| created_at | TEXT NOT NULL | 注册时间（RFC3339Nano） |

索引：idx_push_subscriptions_user (user_id)。

生命周期：订阅无 TTL；失效由发送时的 404/410 响应即时删除（PushService.sendOne），用户注销由 FK CASCADE 清空。VAPID 三件套未配置（env 缺 key）时 IsConfigured()==false，订阅端点 503、发送静默跳过（Web Push 整体 opt-in 默认关闭）。

### thread_follows（【本地改动 2026-08-31】线程关注 opt-in）

| 列 | 说明 |
|---|---|
| user_id | FK → users ON DELETE CASCADE（关注者） |
| thread_root_message_id | FK → messages ON DELETE CASCADE（线程根） |
| created_at | 关注时间（RFC3339Nano） |

唯一约束 `(user_id, thread_root_message_id)`：每用户对每线程关注幂等，重复 POST 不报错、不重插行。关注无 TTL；根消息删除时级联清空，用户注销时级联清空。关注是 opt-in，触发 `reply_in_thread` 通知的前提。

### thread_read_state（【本地改动 2026-08-31】线程已读游标）

| 列 | 说明 |
|---|---|
| user_id | FK → users ON DELETE CASCADE |
| thread_root_message_id | FK → messages ON DELETE CASCADE |
| cursor_message_id | FK → messages ON DELETE CASCADE（最后已读消息 id，无回复时为根） |
| last_seen_at | 游标消息的 created_at 快照，用于 has_unread 判定 |

唯一约束 `(user_id, thread_root_message_id)`：POST `/api/threads/read` 原子推进游标（cursor = 线程内最新回复；无回复则 cursor = 根）。`has_unread` 判定用反向谓词 `last_seen_at < last_reply_at`（游标为空 = 从未打开 = 未读；游标存在但最新回复晚于游标 = 未读）。

### chat_pins（【本地改动 2026-09-02】消息置顶，区别于聊天公告）

| 列 | 说明 |
|---|---|
| id | 主键，`db.NewID()` |
| chat_id | FK → chats ON DELETE CASCADE |
| message_id | FK → messages ON DELETE CASCADE |
| pinned_by | FK → users（置顶操作者） |
| created_at | TEXT RFC3339Nano |

唯一约束 `(chat_id, message_id)`：同一聊天同一消息最多一个 pin（前端/服务端幂等）。
索引：`idx_chat_pins_chat_created (chat_id, created_at DESC)`（倒序分页）、`idx_chat_pins_message (message_id)`（批量清理）。

**与聊天公告的边界**：`chats.pinned_message` JSON 列存储自写文本公告（单条、owner-only）；`chat_pins` 存储指向现有消息的多条置顶，owner/admin 可操作、member 可读列表；两者独立、不冲突。

**消息软删除联动**：FK CASCADE 只对硬删除生效；消息软删除（DeleteMessage 仅置 deleted_at）时应用层同步调用 `RemovePinsForMessage` 清理关联 pin，避免列表中残留不可读消息。

### attachments / reactions / mentions

| 存储位置 | 说明 |
|---|---|
| `messages.attachments`（JSON 缓存列） | 附件数组 `[{id, filename, mime_type, size, url}]`；无独立表（预聚合，读侧免 JOIN） |
| `messages.reactions`（JSON 缓存列） | 反应数组 `[{emoji, count, user_ids, me}]`；与 `reactions` 表同步 |
| `messages.mentions`（JSON 缓存列） | 提及用户 id 数组 |
| `reactions` 表 | 每行 = 一个用户对一条消息的一个 emoji，PK `(message_id, user_id, emoji)` |
| `mentions` 表 | 提及关系 PK `(message_id, user_id)` |

`messages.reactions` JSON 缓存由 `AddReaction`/`RemoveReaction` 同步（`syncReactionsColumn`），保证与 `reactions` 表一致。

### 附件 URL 模式（【本地改动 2026-09-02】fork 公开稳定 URL）

新上传文件：

- **URL**：`/assets/files/{assetID}/{fn.ext}`，`assetID` = UUIDv4，作为凭证（无 ticket、无成员校验）。
- **缓存**：`Cache-Control: public, max-age=31536000, immutable`，`ETag: "{assetID}"`，CDN 可永久缓存。
- **存盘**：`{UPLOAD_DIR}/uploads/{assetID}/{fn.ext}`（uuid 目录隔离）。
- **安全头**：`X-Content-Type-Options: nosniff`；HTML/XML/SVG 加 `CSP: sandbox`。
- **删除**：`DELETE /api/files/{assetID}`（Bearer 认证）；消息删除时级联清理。

旧上传文件（向后兼容）：

- **URL**：`/api/local/{ts}/{fn.ext}`，`?delete={hash}` 为路径凭据。
- **缓存**：`Cache-Control: public, max-age=2592000`（30 天）。
- **删除**：`GET /api/local/{ts}/{fn.ext}?delete={hash}`。

两种 URL 模式共存，前端按响应字段获取即可；service 层 URL 校验接受两种前缀。

## 设计决策

- **写路径维护冗余**（counts + JSON 缓存）：聊天列表/消息列表读侧零子查询
- **TEXT 时间戳**：UTC ISO-8601（RFC3339Nano），人类可读且避免时区坑
- **软删除**：`deleted_at` 标记，广播 `message_delete` 后保留行（引用完整性）
- **外键 ON DELETE CASCADE**：删聊天/删用户自动清理从表
