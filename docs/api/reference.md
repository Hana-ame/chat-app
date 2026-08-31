# API 参考

Base URL：生产 `https://chat.moonchan.xyz`，本地 `http://localhost:8080`。

在线 OpenAPI：`/swagger/`（服务端内嵌 `server/internal/handlers/swagger.json`）。

## 通用约定

- 请求/响应均为 JSON（`Content-Type: application/json`）
- 时间戳：UTC ISO-8601（RFC3339）
- 认证：`Authorization: Bearer <access_token>` 或 `access_token` Cookie；access token 默认 30m，refresh 轮换见 [backend.md](../architecture/backend.md#认证auth)
- 错误响应：`{"error": "<code>", "message": "<human readable>"}`，错误码见 [error-codes.md](error-codes.md)
- 分页：消息列表用 `before`（游标，排除该 id）+ `limit`（默认 50，上限 100）；聊天/通知列表用 `page`（1 基）+ `limit`

## 认证

| 方法 | 路径 | 认证 | 说明 |
|---|---|---|---|
| POST | `/api/auth/register` | 无 | 注册 `{username, email, password}` → `{access_token, refresh_token, user}` |
| POST | `/api/auth/login` | 无 | 登录 → 同上 |
| POST | `/api/auth/refresh` | 无 | `{refresh_token}` → 新 token 对（轮换） |
| POST | `/api/auth/logout` | Bearer | 吊销 refresh token |
| GET | `/api/users/me` | Bearer | 当前用户 |
| PATCH | `/api/users/me` | Bearer | 更新资料（username/avatar_color/status/avatar_url） |

## 聊天

| 方法 | 路径 | 认证 | 说明 |
|---|---|---|---|
| GET | `/api/chats/my` | Bearer | 我的聊天列表（含未读计数、最后消息） |
| GET | `/api/chats/public` | Bearer | 公开频道列表（可分页 `page`/`limit`） |
| POST | `/api/chats` | Bearer | 创建群组 `{name, visibility, icon_color}` |
| POST | `/api/dms` | Bearer | 创建/获取 DM `{user_id}` |
| GET | `/api/chats/notify` | Bearer | 系统通知聊天（不存在则创建） |
| GET | `/api/chats/{chatID}` | Bearer | 聊天详情（含 `members`、`pinned_message`） |
| PATCH | `/api/chats/{chatID}` | Bearer | 改名 / 改可见性 |
| DELETE | `/api/chats/{chatID}` | Bearer | 删除聊天（仅 owner） |
| GET | `/api/chats/{chatID}/members` | Bearer | 成员列表 |
| POST | `/api/chats/{chatID}/members` | Bearer | 添加成员 `{user_id, role}` |
| DELETE | `/api/chats/{chatID}/members/{userID}` | Bearer | 移除成员 |
| POST | `/api/chats/{chatID}/join` | Bearer | 加入公开聊天 |
| POST | `/api/chats/{chatID}/read` | Bearer | 标记已读 `{message_id}` |
| POST | `/api/chats/{chatID}/pin` | Bearer | 置顶聊天（列表排序） |
| POST | `/api/chats/{chatID}/unpin` | Bearer | 取消置顶 |
| POST | `/api/chats/{chatID}/announcement` | Bearer | 设置置顶公告（owner） |
| PATCH | `/api/chats/{chatID}/announcement` | Bearer | 更新置顶公告（owner） |
| DELETE | `/api/chats/{chatID}/announcement` | Bearer | 取消置顶公告 |
| POST | `/api/chats/{chatID}/announcement/read` | Bearer | 公告已读 |
| PUT | `/api/chats/{chatID}/avatar` | Bearer | 群头像（multipart 文件字段 `file`） |
| PUT | `/api/chats/{chatID}/banner` | Bearer | 群横幅（multipart `file`） |
| PUT | `/api/chats/{chatID}/background` | Bearer | 群背景图 |
| PUT | `/api/chats/{chatID}/notify` | Bearer | 通知开关 `{enabled}` |

## 消息

| 方法 | 路径 | 认证 | 说明 |
|---|---|---|---|
| GET | `/api/chats/{chatID}/messages` | Bearer | 消息列表（`before`/`limit`，含 attachments/reactions 冗余字段） |
| POST | `/api/chats/{chatID}/messages` | Bearer | 发送 `{content, attachments?, reply_to?, thread_root?, start_thread?, type?, source?}`；`type=stream` 时 `source` 为 AI 源（`{endpoint, auth_key, body}`），响应含 `msg_id`，流式内容另取 |
| PATCH | `/api/chats/{chatID}/messages/{messageID}` | Bearer | 编辑（本人） |
| DELETE | `/api/chats/{chatID}/messages/{messageID}` | Bearer | 删除（本人；owner/admin 可删他人） |
| GET | `/api/chats/{chatID}/messages/{messageID}/stream` | Bearer | 读取 AI 流式内容（SSE 行格式） |
| PUT | `/api/chats/{chatID}/messages/{messageID}/reactions/{emoji}` | Bearer | 添加反应（emoji URL 编码） |
| DELETE | `/api/chats/{chatID}/messages/{messageID}/reactions/{emoji}` | Bearer | 移除反应 |
| GET | `/api/chats/{chatID}/messages/{messageID}/reactions` | Bearer | 反应列表 |

## 线程（【本地改动 2026-08-31】）

| 方法 | 路径 | 认证 | 说明 |
|---|---|---|---|
| GET | `/api/threads?before=&limit=` | Bearer | 当前用户关注的线程列表（含 `ThreadMeta` + `root_message`） |
| POST | `/api/threads/follow` | Bearer | 关注线程 `{thread_root_message_id}`，幂等 |
| DELETE | `/api/threads/follow` | Bearer | 取关线程 `{thread_root_message_id}`，幂等 |
| POST | `/api/threads/read` | Bearer | 标记已读 `{thread_root_message_id}`；游标自动推进到最新回复 |
| GET | `/api/chats/{chatID}/threads/{threadRootID}` | Bearer | 单线程详情（root_message + ThreadMeta，含 `is_following`/`has_unread`） |

消息列表 GET 支持 `?in_thread=<thread_root_message_id>` 过滤线程内消息（含根）。发送 POST 支持 `thread_root` 显式指定线程根、`start_thread` 让本消息成为根。

## 通知

| 方法 | 路径 | 认证 | 说明 |
|---|---|---|---|
| GET | `/api/notifications/messages` | Bearer | 通知消息列表（分页） |
| POST | `/api/notifications/messages` | Bearer | 发送通知 `{content, attachments?}`（存 notify 聊天） |
| DELETE | `/api/notifications/messages/{messageID}` | Bearer | 删除通知 |
| POST | `/api/notifications/read` | Bearer | 全部标记已读 |
| GET | `/api/notifications` | Bearer | 持久化通知列表（【本地改动 2026-08-31】移植 chatto；`limit`/`before` 分页） |
| GET | `/api/notifications/unread-count` | Bearer | 未读持久化通知计数 |
| POST | `/api/notifications/read-all` | Bearer | 全部持久化通知标记已读 |
| POST | `/api/notifications/{id}/read` | Bearer | 单条持久化通知标记已读 |
| DELETE | `/api/notifications/{id}` | Bearer | 删除单条持久化通知 |

## 用户

| 方法 | 路径 | 认证 | 说明 |
|---|---|---|---|
| GET | `/api/users` | Bearer | 搜索用户（`q` 参数，30 req/min/user） |

## 上传（无认证）

| 方法 | 路径 | 认证 | 说明 |
|---|---|---|---|
| GET | `/api/upload` | 无 | 独立上传页（HTML） |
| PUT / POST | `/api/upload` | 无 | 上传文件（raw body 或 multipart `file`；≤ `CHAT_MAX_UPLOAD`），响应返回公开稳定 URL |
| GET | `/assets/files/{assetID}/{filename}` | 无 | 下载附件（【本地改动 2026-09-02】公开稳定 URL，assetID 即凭证，CDN 可 1 年 immutable 缓存） |
| GET | `/assets/files/{assetID}` | 无 | 同上，文件名从磁盘推断 |
| DELETE | `/api/files/{assetID}` | Bearer | 删除附件文件（Bearer 认证，替代旧 `?delete=hash` 路径凭据） |
| GET | `/api/local/{path}` | 无 | 旧模式下载（向后兼容） |
| GET | `/api/local/{path}?delete=<hash>` | 无 | 旧模式删除（hash = `sha256(path+salt)` 前 8 字节 hex） |

上传响应 `200`（【本地改动 2026-09-02】新格式）：

```json
{
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "filename": "photo.jpg",
  "mime_type": "image/jpeg",
  "size": 12345,
  "url": "https://chat.moonchan.xyz/assets/files/a1b2c3d4-e5f6-7890-abcd-ef1234567890/photo.jpg",
  "delete_url": "https://chat.moonchan.xyz/api/files/a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

**`url`/`delete_url` 恒为绝对 URL**（`CHAT_BASE_URL` 优先，否则 `X-Forwarded-Proto` + `Host` 推导）。`/assets/files/` 响应头：`Cache-Control: public, max-age=31536000, immutable`，`ETag: "{assetID}"`，`X-Content-Type-Options: nosniff`。

## 实时

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/ws` | WebSocket（Bearer 头或 cookie；**拒绝 URL `?token=`**，防访问日志/Referer 泄露），协议见 [realtime.md](../architecture/realtime.md) |
| GET | `/api/events` | SSE（Bearer 头），协议同上 |

## 系统

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/healthz` | 健康检查（回显请求头） |
| GET | `/api/version` | `{"version": "0.9.7"}` |
| GET | `/swagger/` | OpenAPI UI |

## 主要模型

```ts
// User
{ id, username, email, avatar_color, status, last_seen, created_at, avatar_url }

// Chat
{ id, type: "group"|"dm"|"notify", name, icon_color, owner_id, created_at,
  last_message_at, last_message_id, member_count, visibility,
  pinned_message, pinned_updated_at, avatar_url, banner_url, banner_opacity,
  background_url, members?, last_message? }

// Message
{ id, chat_id, user_id, content, created_at, edited_at, deleted, type,
  thinking, reply_to, thread_root_message_id, attachments: [], reactions: [],
  author?: User, mention_count, reaction_count }

// ThreadMeta（【本地改动 2026-08-31】）
{ thread_root_message_id, chat_id, reply_count, last_reply_at, latest_reply_id,
  is_following, has_unread }

// ThreadSummary（【本地改动 2026-08-31】，ThreadMeta + root_message 展平）
{ root_message: Message, thread_root_message_id, chat_id, reply_count,
  last_reply_at, latest_reply_id, is_following, has_unread }
```
