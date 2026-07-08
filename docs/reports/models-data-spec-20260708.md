# Models 定义与数据生成规范

> 原始来源：`server/internal/models/models.go`
> 生成代码散见于：`server/internal/db/users.go`、`server/internal/auth/auth.go`、`server/internal/config/config.go`

---

## 一、原始代码

```go
package models

import "time"

type User struct {
    ID          string    `json:"id"`
    Email       string    `json:"email,omitempty"`
    Username    string    `json:"username"`
    AvatarColor string    `json:"avatar_color"`
    AvatarURL   string    `json:"avatar_url,omitempty"`
    Status      string    `json:"status"`
    CreatedAt   time.Time `json:"created_at"`
}

type Chat struct {
    ID            string     `json:"id"`
    Type          string     `json:"type"`
    Name          string     `json:"name,omitempty"`
    IconColor     string     `json:"icon_color,omitempty"`
    Visibility    string     `json:"visibility,omitempty"`
    OwnerID       string     `json:"owner_id,omitempty"`
    CreatedAt     time.Time  `json:"created_at"`
    LastMessageAt *time.Time `json:"last_message_at,omitempty"`
    Members       []User     `json:"members,omitempty"`
    UnreadCount   int        `json:"unread_count"`
    Pinned        bool       `json:"pinned"`
    LastMessage   *Message   `json:"last_message,omitempty"`
}

type ChatMember struct {
    ChatID            string    `json:"chat_id"`
    UserID            string    `json:"user_id"`
    JoinedAt          time.Time `json:"joined_at"`
    LastReadMessageID string    `json:"last_read_message_id,omitempty"`
    Pinned            bool      `json:"pinned"`
}

type Message struct {
    ID          string       `json:"id"`
    ChatID      string       `json:"chat_id"`
    UserID      string       `json:"user_id"`
    Author      *User        `json:"author,omitempty"`
    Content     string       `json:"content"`
    CreatedAt   time.Time    `json:"created_at"`
    EditedAt    *time.Time   `json:"edited_at,omitempty"`
    Deleted     bool         `json:"deleted"`
    Attachments []Attachment `json:"attachments,omitempty"`
    Reactions   []Reaction   `json:"reactions,omitempty"`
    Mentions    []string     `json:"mentions,omitempty"`
}

type Attachment struct {
    ID        string `json:"id"`
    MessageID string `json:"message_id"`
    Filename  string `json:"filename"`
    MimeType  string `json:"mime_type"`
    Size      int64  `json:"size"`
    URL       string `json:"url"`
}

type Reaction struct {
    Emoji   string   `json:"emoji"`
    Count   int      `json:"count"`
    UserIDs []string `json:"user_ids"`
    Me      bool     `json:"me"`
}

type RefreshToken struct {
    ID        string    `json:"id"`
    UserID    string    `json:"user_id"`
    TokenHash string    `json:"-"`
    ExpiresAt time.Time `json:"expires_at"`
    CreatedAt time.Time `json:"created_at"`
}
```

---

## 二、模型字段总表

### User

| 字段 | 类型 | JSON | 来源 | 生成规则 |
|------|------|------|------|----------|
| ID | `string` | `"id"` | 服务端生成 | `uuid.NewString()` → UUID v4 |
| Email | `string` | `"email"` | 用户输入 | `strings.ToLower` + `mail.ParseAddress` 校验；客户端 `omitempty` |
| Username | `string` | `"username"` | 用户输入 | 长度 2‑32，排除控制字符 (0x00‑0x1F, 0x7F) |
| AvatarColor | `string` | `"avatar_color"` | 服务端计算 | `PickColor(username)` → palette\[h % 8\]（哈希取模） |
| AvatarURL | `string` | `"avatar_url"` | 用户上传 | 外部 `upload.moonchan.xyz` 返回的 URL |
| Status | `string` | `"status"` | 服务端设置 | 默认 `"offline"`，WS 连接时 `"online"` |
| CreatedAt | `time.Time` | `"created_at"` | 服务端生成 | SQLite `strftime('%Y-%m-%dT%H:%M:%fZ','now')` |

### Chat

| 字段 | 类型 | JSON | 来源 | 生成规则 |
|------|------|------|------|----------|
| ID | `string` | `"id"` | 服务端生成 | UUID v4 |
| Type | `string` | `"type"` | 用户指定 | 约束 `IN ('dm','group')` |
| Name | `string` | `"name"` | 用户输入 | group 必填，dm 为 null |
| IconColor | `string` | `"icon_color"` | 服务端计算 | group: `PickColor(name)`；dm: `PickColor(id)` |
| Visibility | `string` | `"visibility"` | 用户指定 | 枚举 `public / unlisted / private`；dm 为空串 |
| OwnerID | `string` | `"owner_id"` | 用户指定 | 引用 `users(id)`，可为 null（dm） |
| CreatedAt | `time.Time` | `"created_at"` | 服务端生成 | SQLite default |
| LastMessageAt | `*time.Time` | `"last_message_at"` | 服务端更新 | 发消息时 `UPDATE chats SET last_message_at = now` |
| Members | `[]User` | `"members"` | JOIN 查询 | `GetChatMembers` → `SELECT ... FROM chat_members JOIN users` |
| UnreadCount | `int` | `"unread_count"` | 服务端计算 | `COUNT(messages) WHERE deleted=0 AND (created_at,id) > lastReadID` |
| Pinned | `bool` | `"pinned"` | 用户动作 | `POST /pin` → `pinned = 1`，`/unpin` → `0` |
| LastMessage | `*Message` | `"last_message"` | 服务端查询 | `GetMessage(LastMessageID)` |

### ChatMember

| 字段 | 类型 | JSON | 来源 | 生成规则 |
|------|------|------|------|----------|
| ChatID | `string` | `"chat_id"` | 关联 | 引用 `chats(id)` ON DELETE CASCADE |
| UserID | `string` | `"user_id"` | 关联 | 引用 `users(id)` ON DELETE CASCADE |
| JoinedAt | `time.Time` | `"joined_at"` | 服务端生成 | SQLite default |
| LastReadMessageID | `string` | `"last_read_message_id"` | 用户动作 | 调用 `POST /read` 时更新 |
| Pinned | `bool` | `"pinned"` | 用户动作 | 同上 |

### Message

| 字段 | 类型 | JSON | 来源 | 生成规则 |
|------|------|------|------|----------|
| ID | `string` | `"id"` | 服务端生成 | UUID v4 |
| ChatID | `string` | `"chat_id"` | 用户关联 | 引用 `chats(id)` |
| UserID | `string` | `"user_id"` | 用户关联 | 引用 `users(id)` |
| Author | `*User` | `"author"` | JOIN 查询 | `messages JOIN users` |
| Content | `string` | `"content"` | 用户输入 | 最大 4000 字符，`strings.TrimRight` |
| CreatedAt | `time.Time` | `"created_at"` | 服务端生成 | Go `time.Now().UTC().Format("2006-01-02T15:04:05.000Z")` |
| EditedAt | `*time.Time` | `"edited_at"` | 服务端设置 | `UpdateMessage` 时记录当前时间 |
| Deleted | `bool` | `"deleted"` | 服务端设置 | soft delete（设 `deleted=1`, `content=''`） |
| Attachments | `[]Attachment` | `"attachments"` | 子查询 | `SELECT ... FROM attachments WHERE message_id = ?` |
| Reactions | `[]Reaction` | `"reactions"` | 子查询 | `SELECT emoji, user_id FROM reactions ... ORDER BY created_at` |
| Mentions | `[]string` | `"mentions"` | 子查询 | `SELECT user_id FROM mentions WHERE message_id = ?` |

### Attachment

| 字段 | 类型 | JSON | 来源 | 生成规则 |
|------|------|------|------|----------|
| ID | `string` | `"id"` | 服务端生成 | 若客户端未传，用 `NewID()` |
| MessageID | `string` | `"message_id"` | 关联 | 引用 `messages(id)` |
| Filename | `string` | `"filename"` | 用户传入 | `filepath.Base` + `Replace("..", "_")` + 截断 200 字符 |
| MimeType | `string` | `"mime_type"` | 用户/推断 | 根据 `Content-Type` 或 `mime.TypeByExtension`，默认 `application/octet-stream` |
| Size | `int64` | `"size"` | 用户传入 | 文件字节数 |
| URL | `string` | `"url"` | 用户传入 | 指向 `/uploads/...` 或外部存储 |

### Reaction（API 响应结构，非原始行）

| 字段 | 类型 | JSON | 来源 | 生成规则 |
|------|------|------|------|----------|
| Emoji | `string` | `"emoji"` | 用户输入 | 最大 32 字符 |
| Count | `int` | `"count"` | 服务端聚合 | `GROUP BY emoji → COUNT(*)` |
| UserIDs | `[]string` | `"user_ids"` | 服务端聚合 | `GROUP BY emoji → json_agg(user_id)` |
| Me | `bool` | `"me"` | 服务端计算 | 当前 viewer 是否在此 emoji 的 user_ids 中 |

### RefreshToken

| 字段 | 类型 | JSON | 来源 | 生成规则 |
|------|------|------|------|----------|
| ID | `string` | `"id"` | 服务端生成 | UUID v4 |
| UserID | `string` | `"user_id"` | 关联 | 引用 `users(id)` |
| TokenHash | `string` | `"-"` | 服务端生成 | `sha256(base64.RawURLEncoding(random32Bytes))` |
| ExpiresAt | `time.Time` | `"expires_at"` | 服务端计算 | `time.Now().UTC().Add(RefreshTokenTTL)`，默认 365 天 |
| CreatedAt | `time.Time` | `"created_at"` | 服务端生成 | `time.Now().UTC()` |

---

## 三、数据生成来源与规则

### 3.1 ID 生成（所有主键）

```go
// server/internal/db/users.go:17
func NewID() string { return uuid.NewString() }
```

**规则**：标准 UUID v4，纯随机，无冲突检测（概率可忽略）。所有实体的 ID 统一使用此函数。

### 3.2 颜色选择

```go
// server/internal/db/users.go:24-33
func PickColor(seed string) string {
    if seed == "" { return "#5865F2" }
    h := 0
    for _, r := range seed { h = (h*31 + int(r)) & 0x7fffffff }
    return palette[h % 8]
}
```

**规则**：DJBernstein 哈希变体 × 31 → 取模 8 → 从固定 8 色调色板选色。相同 seed 产生相同颜色。

### 3.3 时间戳

| 位置 | 精度 | 格式 |
|------|------|------|
| SQL 默认值（migration） | 微秒 | `strftime('%Y-%m-%dT%H:%M:%fZ','now')` |
| Go `CreateMessage` | 毫秒 | `"2006-01-02T15:04:05.000Z"` |
| Go `UpdateMessage` | 毫秒 | 同上 |
| Go `CreateRefreshToken` | 纳秒 | `time.RFC3339Nano` |
| Go `PurgeExpiredTokens` | 纳秒 | `time.RFC3339Nano` |

**规则**：统一使用 UTC，以 TEXT 存储。**注意精度不统一**：message 的时间截断到毫秒，chat 的 `last_message_at` 和 token 用纳秒级格式。

### 3.4 Token 生成

| Token 类型 | 算法 | 长度 | 存储 |
|------------|------|------|------|
| Access Token | `HS256 JWT` + `claims{userID, exp, iat, sub}` | 不定（约 190 字节） | 不存储，全靠签名验证 |
| Refresh Token raw | `crypto/rand(32)` → `base64.RawURLEncoding` | 43 字符 | 不存储（只在 HTTP cookie 中） |
| Refresh Token hash | `sha256(raw)` → `hex` | 64 字符 | 存表 |

### 3.5 密码

```go
// server/internal/auth/auth.go:101-113
HashPassword → bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
```

**规则**：bcrypt cost 10；密码长于 72 字节时截断；至少 8 字符。

### 3.6 邮箱

```go
// server/internal/auth/auth.go:125-134
NormalizeEmail → strings.ToLower + strings.TrimSpace + mail.ParseAddress
```

**规则**：小写、去空格、`mail.ParseAddress` 验证格式。

### 3.7 用户名

```go
// server/internal/auth/auth.go:136-149
ValidateUsername → 长度 2-32，排除控制字符 (0x00-0x1F, 0x7F)
```

### 3.8 上传文件命名

```go
// server/internal/handlers/uploads.go:29-36
sanitizeFilename → filepath.Base + strings.ReplaceAll("..", "_") + 截断 200
```

### 3.9 消息内容

```go
// server/internal/db/messages.go:16-18
content = strings.TrimRight(content, " \n\t")
if len(content) > 4000 { content = content[:4000] }
```

**规则**：去尾部空白 + 截断 4000 字符。空内容但有 attachment 也允许。

### 3.10 Emoji 反应

```go
// server/internal/db/messages.go:334-339
emoji = strings.TrimSpace(emoji)
if emoji == "" || len(emoji) > 32 { return error }
```

**规则**：非空、≤32 字符。未做 Unicode 有效性校验。

---

## 四、数据流图

```
用户输入
  ├── email / username / password ──→ User
  ├── chat type / name / visibility ──→ Chat
  ├── message content ──→ Message
  ├── file upload ──→ Attachment（经 sanitizeFilename 清洗）
  └── emoji ──→ Reaction

服务端计算
  ├── uuid.NewString() ──→ 所有实体 ID
  ├── PickColor(seed) ──→ AvatarColor / IconColor
  ├── time.Now().UTC() ──→ CreatedAt / ExpiresAt / LastMessageAt
  ├── bcrypt(password) ──→ password_hash
  ├── JWT(jti, uid, exp) ──→ access_token
  ├── sha256(randomBytes) ──→ refresh_token hash
  └── status "online"/"offline" (WS hook)

服务端推断（子查询 / JOIN）
  ├── Members ──→ ChatMember → JOIN users
  ├── Attachments ──→ attachment 表 WHERE message_id
  ├── Reactions ──→ reaction 表 GROUP BY emoji → 聚合结构
  ├── Mentions ──→ mention 表 WHERE message_id
  └── UnreadCount ──→ COUNT(messages) WHERE deleted=0 AND >lastRead
```

---

## 五、约束汇总

| 约束 | 检查位置 | 违反后果 |
|------|----------|----------|
| email 必须可解析 | `auth.NormalizeEmail` | 400/`invalid_email` |
| username 2-32 字符，无控制符 | `auth.ValidateUsername` | 400/`invalid_username` |
| password ≥ 8 字符 | `auth.HashPassword` | 400/`weak_password` |
| 唯一 email/username | 数据库 UNIQUE 索引 | 409/`already_taken` |
| chat type = dm / group | DB CHECK 约束 | 语句失败 |
| message content ≤ 4000 | `strings[:4000]` | 静默截断 |
| emoji ≤ 32 字符 | DB 层校验 | 400/`bad_request` |
| 外键引用 | DB FOREIGN KEY | 级联删除或 ABORT |
| 时间格式 | Go layout 解析 | 回退 `time.Time{}` |
