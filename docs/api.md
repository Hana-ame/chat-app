# Chat App API

Base URL: `https://chat.moonchan.xyz`

## 认证

所有 `/api` 端点（除健康检查、注册、登录、刷新 token 外）需要 `Authorization: Bearer <access_token>` 请求头。

access_token 有效期 15 分钟，通过 `/api/auth/refresh` 续期。

## 错误格式

```json
{ "error": "error_code", "message": "human-readable message" }
```

常见错误码：`bad_request`、`unauthorized`、`forbidden`、`not_found`、`rate_limited`、`content_too_long`

---

## 健康检查

### `GET /healthz`

无认证。返回请求头回显。

**Response `200`**:
```json
{ "status": "ok", "echo": { "Header-Name": "value" } }
```

---

## Auth

### `POST /api/auth/register`

**Rate limit**: 5/min/IP

**Request**:
```json
{ "email": "user@example.com", "username": "alice", "password": "securePassword123" }
```

**Response `200`**:
```json
{
  "user": { "id": "...", "username": "alice", "avatar_color": "#5865F2", "status": "online", "created_at": "..." },
  "access_token": "jwt...",
  "expires_in": 900
}
```

Set-Cookie: `refresh_token` (httpOnly), `access_token`

**Errors**: `already_taken` (409)

### `POST /api/auth/login`

**Rate limit**: 10/min/IP

**Request**:
```json
{ "email": "user@example.com", "password": "securePassword123" }
```

**Response `200`**: 同 register

**Errors**: `invalid_credentials` (401)

### `POST /api/auth/refresh`

读取 Cookie `refresh_token`（一次性轮换）。

**Response `200`**: 同 register

**Errors**: `refresh_invalid` (401)

### `POST /api/auth/logout`

清除所有 refresh token。

**Response `200`**:
```json
{ "ok": true }
```

---

## Users

### `GET /api/users/me`

**Response `200`**:
```json
{
  "id": "...", "username": "alice", "email": "user@example.com",
  "avatar_color": "#5865F2", "avatar_url": "https://...",
  "status": "online", "last_seen": "...", "created_at": "..."
}
```

### `PATCH /api/users/me`

**Request**:
```json
{ "username": "newname", "avatar_color": "#23a559", "avatar_url": "https://..." }
```

全部可选。

**Response `200`**: 完整 User 对象

**Errors**: `username_taken` (409)

### `GET /api/users?q=<keyword>`

**Rate limit**: 30/min/user

**Response `200`**:
```json
{ "users": [ { "id": "...", "username": "...", ... } ] }
```

---

## Chats

### `GET /api/chats/my`

当前用户的聊天列表。

**Response `200`**:
```json
{
  "chats": [
    {
      "id": "...", "type": "group",
      "name": "General", "visibility": "public",
      "icon_color": "#5865F2",
      "owner_id": "...",
      "member_count": 5, "pinned": false,
      "created_at": "...", "last_message_at": "...",
      "members": [ { "id": "...", "username": "...", "role": "admin", ... } ],
      "pinned_message": { "content": "...", "pinned_at": "..." },
      "pinned_updated_at": "...", "pinned_last_read_at": "...",
      "last_active_at": "..."
    }
  ]
}
```

### `GET /api/chats/public?page=1&limit=20`

**Response `200`**: `{ "chats": [...] }`

### `POST /api/chats`

创建群组。

**Request**:
```json
{
  "type": "group",
  "name": "My Group",
  "visibility": "private",
  "member_ids": ["user_id_1", "user_id_2"]
}
```

**Response `201`**: Chat 对象

### `GET /api/chats/{chatID}`

**Response `200`**: Chat 对象

### `PATCH /api/chats/{chatID}`

重命名。

**Request**:
```json
{ "name": "New Name" }
```

**Response `200`**: Chat 对象

### `DELETE /api/chats/{chatID}`

**Response `200`**:
```json
{ "ok": true }
```

### `POST /api/chats/{chatID}/join`

加入公开聊天。

**Response `200`**:
```json
{ "chat": { ... } }
```

### `POST /api/chats/{chatID}/pin`

置顶到列表顶部。

**Response `200`**:
```json
{ "ok": true, "pinned": true }
```

### `POST /api/chats/{chatID}/unpin`

取消置顶。

**Response `200`**:
```json
{ "ok": true, "pinned": false }
```

### `PUT /api/chats/{chatID}/avatar`

**Request**:
```json
{ "url": "https://..." }
```

**Response `200`**: Chat 对象

### `PUT /api/chats/{chatID}/banner`

**Request**:
```json
{ "banner_url": "https://...", "banner_opacity": 0.5 }
```

**Response `200`**: Chat 对象

### `PUT /api/chats/{chatID}/background`

**Request**:
```json
{ "url": "https://..." }
```

**Response `200`**: Chat 对象

### `PUT /api/chats/{chatID}/notify`

**Request**:
```json
{ "enabled": true }
```

**Response `200`**:
```json
{ "enabled": true }
```

---

## Members

### `GET /api/chats/{chatID}/members`

**Response `200`**:
```json
{
  "members": [
    { "id": "...", "username": "...", "avatar_color": "...",
      "role": "admin", "last_active_at": "...", "pinned": false }
  ]
}
```

### `POST /api/chats/{chatID}/members`

**Request**:
```json
{ "user_id": "..." }
```

**Response `200`**: Chat 对象

### `DELETE /api/chats/{chatID}/members/{userID}`

**Response `200`**:
```json
{ "ok": true }
```

---

## Messages

### `GET /api/chats/{chatID}/messages?limit=50&before=<messageID>`

游标分页。

**Response `200`**:
```json
{
  "messages": [
    {
      "id": "...", "chat_id": "...", "user_id": "...",
      "type": "", "content": "Hello",
      "created_at": "...", "edited_at": null, "deleted_at": null,
      "attachment_count": 0, "mention_count": 0, "reaction_count": 0,
      "attachments": [], "reactions": [], "mentions": []
    }
  ]
}
```

### `POST /api/chats/{chatID}/messages`

**Rate limit**: 30/min/user

发送普通消息或 AI stream 消息。

**普通消息 Request**:
```json
{
  "content": "Hello",
  "attachments": [ { "id": "...", "filename": "...", "mime_type": "...", "size": 123, "url": "..." } ]
}
```

**普通消息 Response `201`**: Message 对象

**AI stream 消息 Request**:
```json
{
  "content": "",
  "type": "stream",
  "source": {
    "endpoint": "https://api.siliconflow.cn/v1/chat/completions",
    "auth_key": "sk-...",
    "body": { "model": "gpt-4", "messages": [...], "temperature": 0.7 }
  },
  "msg_id": "optional-client-generated-uuid"
}
```

**AI stream 消息 Response**: SSE 事件流（`text/event-stream`）

SSE 事件格式：
```
data: {"type":"content","content":"Hel"}

data: {"type":"content","content":"lo"}

data: {"type":"reasoning","content":"thinking text"}

data: [DONE]

```

### `PATCH /api/chats/{chatID}/messages/{messageID}`

编辑消息（仅作者可编辑）。

**Request**:
```json
{ "content": "Updated content" }
```

**Response `200`**: Message 对象

### `DELETE /api/chats/{chatID}/messages/{messageID}`

**Response `200`**:
```json
{ "ok": true }
```

### `POST /api/chats/{chatID}/read`

标记已读。

**Response `200`**:
```json
{ "ok": true }
```

### `GET /api/chats/{chatID}/messages/{messageID}/stream`

获取 AI stream 消息的实时 SSE 内容。适用于客户端未收到 WebSocket 广播时（如重连后）主动拉取。

**Response**: SSE 事件流，格式同 AI stream 响应。

---

## Reactions

### `PUT /api/chats/{chatID}/messages/{messageID}/reactions/{emoji}`

`emoji` 需要 URL 编码。

**Response `200`**: Message 对象（含更新后的 reactions）

### `DELETE /api/chats/{chatID}/messages/{messageID}/reactions/{emoji}`

**Response `200`**: Message 对象

### `GET /api/chats/{chatID}/messages/{messageID}/reactions`

**Response `200`**:
```json
{
  "reactions": [
    { "emoji": "👍", "count": 3, "user_ids": ["id1", "id2", "id3"], "me": true }
  ]
}
```

---

## Notifications（系统通知）

### `GET /api/chats/notify`

获取系统通知聊天对象（虚拟聊天）。

**Response `200`**: Chat 对象

### `GET /api/notifications/messages?limit=50&before=<messageID>`

**Response `200`**: `{ "messages": [...] }`

### `POST /api/notifications/messages`

**Request**:
```json
{ "content": "Notification text", "attachments": [] }
```

**Response `201`**: Message 对象

### `DELETE /api/notifications/messages/{messageID}`

**Response `200`**: `{ "ok": true }`

### `POST /api/notifications/read`

全部标记已读。

**Response `200`**: `{ "ok": true }`

---

## Announcement（公告 / Pin）

### `POST /api/chats/{chatID}/announcement`

**Request**:
```json
{ "content": "Announcement text (max 500 chars)" }
```

**Response `200`**:
```json
{ "ok": true }
```

### `PATCH /api/chats/{chatID}/announcement`

委托给 POST，同上。

### `DELETE /api/chats/{chatID}/announcement`

**Response `200`**: `{ "ok": true }`

### `POST /api/chats/{chatID}/announcement/read`

标记已读。

**Response `200`**: `{ "ok": true }`

---

## Uploads

### `PUT /api/upload`

裸 body 上传文件。需要 `Content-Type` 头。

**Response `201`**:
```json
{ "path": "..." }
```

文件 URL: `/api/local/{path}`

### `POST /api/upload`

multipart/form-data 上传，字段名 `file`。

**Response `201`**: 同上

### `PUT /api/upload/{path}`

带文件名上传。

### `GET /api/local/{path}`

下载文件。支持 `?delete=true` 查询参数。

---

## 版本

### `GET /api/version`

```json
{ "version": "0.8.15" }
```

---

## 实时推送

### `GET /ws?token=<access_token>`

WebSocket 端点。事件类型与 SSE 一致。

认证：URL 查询参数 `token`。

### `GET /api/events?access_token=<token>`

SSE 端点（Server-Sent Events），用于不支持 WebSocket 的客户端。

#### 事件格式

SSE 事件流，每个事件格式为 `data: <json>\n\n`。

**`ready` 事件**（连接成功后首个事件）:
```json
{
  "op": "ready",
  "payload": {
    "online_user_ids": ["id1", "id2"],
    "chats": [ ... ]
  }
}
```

**`message_create`**:
```json
{ "op": "message_create", "payload": { /* Message 对象 */ } }
```

**`message_update`**:
```json
{ "op": "message_update", "payload": { "id": "...", "content": "..." } }
```

**`message_delete`**:
```json
{ "op": "message_delete", "payload": { "message_id": "...", "chat_id": "..." } }
```

**`reaction_add` / `reaction_remove`**:
```json
{ "op": "reaction_add", "payload": { "message_id": "...", "emoji": "👍", "user_id": "..." } }
```

**`chat_create` / `chat_update`**:
```json
{ "op": "chat_update", "payload": { /* Chat 对象 */ } }
```

**`chat_delete`**:
```json
{ "op": "chat_delete", "payload": { "chat_id": "..." } }
```

---

## 数据类型

### User

```json
{
  "id": "string (required)",
  "email": "string",
  "username": "string (required)",
  "avatar_color": "string (hex, required)",
  "avatar_url": "string (uri)",
  "status": "online | offline | away (required)",
  "role": "string",
  "last_seen": "datetime",
  "created_at": "datetime (required)"
}
```

### Chat

```json
{
  "id": "string (required)",
  "type": "group | dm (required)",
  "name": "string",
  "icon_color": "string",
  "visibility": "public | private",
  "owner_id": "string",
  "created_at": "datetime (required)",
  "last_message_at": "datetime (required)",
  "member_count": "integer (required)",
  "unread_count": "integer (required, deprecated)",
  "pinned": "boolean (required)",
  "pinned_message": "PinnedContent",
  "pinned_updated_at": "datetime",
  "pinned_last_read_at": "datetime",
  "last_active_at": "datetime",
  "last_message_id": "string",
  "last_message": "Message (deprecated)",
  "members": ["User"]
}
```

### Message

```json
{
  "id": "string (required)",
  "chat_id": "string (required)",
  "user_id": "string (required)",
  "type": "string (空 = 普通, 'stream' = AI 流式)",
  "stream_url": "string (type=stream 时存在)",
  "author": "User (deprecated)",
  "content": "string (required)",
  "thinking": "string (思考链, AI stream 时存在)",
  "created_at": "datetime (required)",
  "edited_at": "datetime | null",
  "deleted_at": "datetime | null",
  "attachment_count": "integer (required)",
  "mention_count": "integer (required)",
  "reaction_count": "integer (required)",
  "attachments": ["Attachment"],
  "reactions": ["Reaction"],
  "mentions": ["string"]
}
```

### Attachment

```json
{
  "id": "string (required)",
  "message_id": "string (required)",
  "filename": "string (required)",
  "mime_type": "string (required)",
  "size": "integer (required)",
  "url": "string (uri, required)"
}
```

### Reaction

```json
{
  "emoji": "string (required)",
  "count": "integer (required)",
  "user_ids": ["string"],
  "me": "boolean (required)"
}
```

### PinnedContent

```json
{
  "content": "string",
  "pinned_at": "datetime"
}
```

### ChatMember

```json
{
  "chat_id": "string (required)",
  "user_id": "string (required)",
  "role": "owner | admin | member (required)",
  "joined_at": "datetime (required)",
  "last_active_at": "datetime",
  "last_read_message_id": "string (deprecated)",
  "pinned": "boolean (required)",
  "pinned_last_read_at": "datetime"
}
```

### AISource（请求体）

```json
{
  "endpoint": "AI upstream API URL (required)",
  "auth_key": "Authorization Bearer token (required)",
  "body": { "model": "...", "messages": [...], ... }
}
```
