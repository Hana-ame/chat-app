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

### `refreshReq` — `POST /api/auth/refresh`, `POST /api/auth/logout`
```go
type refreshReq struct {
    RefreshToken string `json:"refresh_token"`
}
```

### `sessionResp` — Response for register / login / refresh
```go
type sessionResp struct {
    User         any    `json:"user"`
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int64  `json:"expires_in"`
}
```

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

## Shared Models

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

| Struct | Used By |
|--------|---------|
| `registerReq` | POST /api/auth/register |
| `loginReq` | POST /api/auth/login |
| `refreshReq` | POST /api/auth/refresh, POST /api/auth/logout |
| `sessionResp` | Response for register, login, refresh |
| `updateProfileReq` | PATCH /api/users/me |
| `createChatReq` | POST /api/chats |
| `createDMReq` | POST /api/dms |
| `renameChatReq` | PATCH /api/chats/{chatID} |
| `addMemberReq` | POST /api/chats/{chatID}/members |
| `sendMsgReq` | POST /api/chats/{chatID}/messages |
| `editMsgReq` | PATCH /api/chats/{chatID}/messages/{messageID} |
| `readReq` | POST /api/chats/{chatID}/read |
