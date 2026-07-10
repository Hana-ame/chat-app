# Go Server — Request & Response Models

All request bodies are decoded via the shared `decodeJSON()` helper in `internal/handlers/handlers.go`. Auth-protected handlers receive the authenticated user via `context`.

---

## Auth

### `registerReq` — `POST /api/auth/register`
```go
type registerReq struct {
    Email    string `json:"email"`
    Username string `json:"username"`
    Password string `json:"password"`
}
```

### `loginReq` — `POST /api/auth/login`
```go
type loginReq struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}
```

### `sessionResp` — Response for register / login / refresh
```go
type sessionResp struct {
    User        any   `json:"user"`
    AccessToken string `json:"access_token"`
    ExpiresIn   int64 `json:"expires_in"`
}
```

> Refresh token 改为 httpOnly cookie (`Path=/api/auth/refresh`)，不再出现于 JSON 响应中。

---

## Users

### `updateProfileReq` — `PATCH /api/users/me`
```go
type updateProfileReq struct {
    Username    string `json:"username"`
    AvatarColor string `json:"avatar_color"`
    AvatarURL   string `json:"avatar_url"`
}
```

---

## Chats

### `createChatReq` — `POST /api/chats`
```go
type createChatReq struct {
    Type       string   `json:"type"`       // "group"
    Name       string   `json:"name"`
    Visibility string   `json:"visibility"` // "public" | "unlisted" | "private"
    MemberIDs  []string `json:"member_ids"`
}
```

### `createDMReq` — `POST /api/dms`
```go
type createDMReq struct {
    UserID string `json:"user_id"`
}
```

### `renameChatReq` — `PATCH /api/chats/{chatID}`
```go
type renameChatReq struct {
    Name string `json:"name"`
}
```

### `addMemberReq` — `POST /api/chats/{chatID}/members`
```go
type addMemberReq struct {
    UserID string `json:"user_id"`
}
```

### `pinContentReq` — `POST /api/chats/{chatID}/pin`
```go
type pinContentReq struct {
    Content string `json:"content"`
}
```

---

## Messages

### `sendMsgReq` — `POST /api/chats/{chatID}/messages`
```go
type sendMsgReq struct {
    Content     string              `json:"content"`
    Attachments []models.Attachment `json:"attachments"`
}
```

### `editMsgReq` — `PATCH /api/chats/{chatID}/messages/{messageID}`
```go
type editMsgReq struct {
    Content string `json:"content"`
}
```

### `readReq` — `POST /api/chats/{chatID}/read`
```go
type readReq struct {
    MessageID string `json:"message_id"`
}
```

---

## Response Models

> 以下为 API 返回的完整结构体定义。标注 `(Deprecated)` 的字段仅为兼容性保留，前端不应依赖。

### `models.User`

```go
type User struct {
    ID          string    `json:"id"`
    Email       string    `json:"email,omitempty"`
    Username    string    `json:"username"`
    AvatarColor string    `json:"avatar_color"`
    AvatarURL   string    `json:"avatar_url,omitempty"`
    Status      string    `json:"status"`           // "online" | "offline"
    LastSeen    time.Time `json:"last_seen,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
}
```

### `models.Chat`

```go
type Chat struct {
    ID            string          `json:"id"`
    Type          string          `json:"type"`            // "dm" | "group"
    Name          string          `json:"name,omitempty"`
    IconColor     string          `json:"icon_color,omitempty"`
    Visibility    string          `json:"visibility,omitempty"`  // "public" | "unlisted" | "private"
    OwnerID       string          `json:"owner_id,omitempty"`
    CreatedAt     time.Time       `json:"created_at"`
    LastMessageAt time.Time       `json:"last_message_at"`
    MemberCount   int             `json:"member_count"`
    PinnedMessage *PinnedContent  `json:"pinned_message,omitempty"`
    LastMessageID string          `json:"last_message_id,omitempty"`
    UnreadCount   int             `json:"unread_count"`          // Deprecated
    LastMessage   *Message        `json:"last_message,omitempty"` // Deprecated
}
```

### `models.ChatMember`

```go
type ChatMember struct {
    ChatID   string    `json:"chat_id"`
    UserID   string    `json:"user_id"`
    Role     string    `json:"role"`      // "owner" | "admin" | ""
    LastSeen time.Time `json:"last_seen,omitempty"`
    JoinedAt time.Time `json:"joined_at"`
    LastReadMessageID string `json:"last_read_message_id,omitempty"` // Deprecated
}
```

### `models.Message`

```go
type Message struct {
    ID              string          `json:"id"`
    ChatID          string          `json:"chat_id"`
    UserID          string          `json:"user_id"`
    Author          *User           `json:"author,omitempty"`          // Deprecated
    Content         string          `json:"content"`
    CreatedAt       time.Time       `json:"created_at"`
    EditedAt        *time.Time      `json:"edited_at,omitempty"`
    DeletedAt       *time.Time      `json:"deleted_at,omitempty"`
    AttachmentCount int             `json:"attachment_count"`
    MentionCount    int             `json:"mention_count"`
    ReactionCount   int             `json:"reaction_count"`
    Attachments     json.RawMessage `json:"attachments,omitempty"`     // JSON array
    Reactions       json.RawMessage `json:"reactions,omitempty"`       // JSON array
    Mentions        json.RawMessage `json:"mentions,omitempty"`        // JSON array
}
```

### `models.PinnedContent`

```go
type PinnedContent struct {
    Content  string    `json:"content"`
    PinnedAt time.Time `json:"pinned_at"`
}
```

### `models.Reaction`（聚合结构，非原始行）

```go
type Reaction struct {
    Emoji string `json:"emoji"`
    Count int    `json:"count"`
}
```

### `models.Attachment`

```go
type Attachment struct {
    ID        string `json:"id"`
    MessageID string `json:"message_id"`
    Filename  string `json:"filename"`
    MimeType  string `json:"mime_type"`
    Size      int64  `json:"size"`
    URL       string `json:"url"`
}
```

---

## Uploads

> ⚠️ **Deprecated** — Go 端 `POST /api/uploads` 已废弃，前端直接上传到 `upload.moonchan.xyz`。

### `POST /api/uploads` — multipart form
```
Field: "file" (file blob)
Constraints:
  - Max size: Config.MaxUploadBytes (default 20 MiB)
  - Allowed MIME types: PNG, JPEG, GIF, WebP, MP4, WebM, MP3, OGG, WAV, PDF, text/plain, ZIP, octet-stream
```

Response:
```json
{
  "id": "string",
  "url": "string",
  "filename": "string",
  "mime_type": "string",
  "size": 0
}
```

---

## Reference

| Struct | Used By | 状态 |
|--------|---------|------|
| `registerReq` | POST /api/auth/register | |
| `loginReq` | POST /api/auth/login | |
| `sessionResp` | Response for register, login, refresh | |
| `updateProfileReq` | PATCH /api/users/me | |
| `createChatReq` | POST /api/chats | |
| `createDMReq` | POST /api/dms | ⚠️ Deprecated |
| `renameChatReq` | PATCH /api/chats/{chatID} | |
| `addMemberReq` | POST /api/chats/{chatID}/members | |
| `pinContentReq` | POST /api/chats/{chatID}/pin | |
| `sendMsgReq` | POST /api/chats/{chatID}/messages | |
| `editMsgReq` | PATCH /api/chats/{chatID}/messages/{messageID} | |
| `readReq` | POST /api/chats/{chatID}/read | ⚠️ Deprecated |
