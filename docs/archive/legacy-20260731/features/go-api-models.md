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

### `pinContentReq` — `POST /api/chats/{chatID}/announcement`
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

### `readReq` — `POST /api/chats/{chatID}/read` ⚠️ Deprecated
```go
type readReq struct {
	MessageID string `json:"message_id"`
}
```
> MarkRead 已不再读取 body，改为直接更新 `last_active_at`。`readReq` 保留仅为兼容，实际未使用。

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
	Role        string    `json:"role,omitempty"`
	LastSeen    time.Time `json:"last_seen,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
```

### `models.Chat`

```go
type Chat struct {
	ID            string     `json:"id"`
	Type          string     `json:"type"`            // "dm" | "group"
	Name          string     `json:"name,omitempty"`
	IconColor     string     `json:"icon_color,omitempty"`
	Visibility    string     `json:"visibility,omitempty"`  // "public" | "unlisted" | "private"
	OwnerID       string     `json:"owner_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	LastMessageAt time.Time  `json:"last_message_at"`
	MemberCount   int        `json:"member_count"`
	// Deprecated.
	UnreadCount    int            `json:"unread_count"`
	PinnedMessage  *PinnedContent `json:"pinned_message,omitempty"`
	PinnedUpdatedAt *time.Time    `json:"pinned_updated_at,omitempty"`
	PinnedLastReadAt *time.Time   `json:"pinned_last_read_at,omitempty"`
	Pinned          bool          `json:"pinned"`
	LastActiveAt    *time.Time    `json:"last_active_at,omitempty"`
	LastMessageID  string         `json:"last_message_id,omitempty"`
	// Deprecated.
	LastMessage   *Message   `json:"last_message,omitempty"`
}
```

### `models.ChatMember`

```go
type ChatMember struct {
	ChatID   string    `json:"chat_id"`
	UserID   string    `json:"user_id"`
	Role     string    `json:"role"`      // "owner" | "admin" | "member"
	JoinedAt time.Time `json:"joined_at"`
	LastActiveAt *time.Time `json:"last_active_at,omitempty"`
	// Deprecated.
	LastReadMessageID string     `json:"last_read_message_id,omitempty"`
	Pinned          bool        `json:"pinned"`
	PinnedLastReadAt *time.Time `json:"pinned_last_read_at,omitempty"`
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
	Emoji   string   `json:"emoji"`
	Count   int      `json:"count"`
	UserIDs []string `json:"user_ids,omitempty"`
	Me      bool     `json:"me"`
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

### `models.RefreshToken`

```go
type RefreshToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
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
| `pinContentReq` | POST /api/chats/{chatID}/announcement | |
| `sendMsgReq` | POST /api/chats/{chatID}/messages | |
| `editMsgReq` | PATCH /api/chats/{chatID}/messages/{messageID} | |
| `readReq` | POST /api/chats/{chatID}/read | ⚠️ Deprecated |
| `joinReq` | POST /api/chats/{chatID}/join | internal |
| `models.User` | Response for /users/* | |
| `models.Chat` | Response for /chats/* | |
| `models.ChatMember` | Response for /chats/{id}/members | |
| `models.Message` | Response for /chats/{id}/messages | |
| `models.Attachment` | Embedded in Message | |
| `models.Reaction` | Response for reactions endpoints | |
| `models.PinnedContent` | Embedded in Chat | |
| `models.RefreshToken` | Internal (DB only) | |
