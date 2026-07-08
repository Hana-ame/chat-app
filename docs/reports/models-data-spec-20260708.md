# Models 定义与数据生成规范

> 原始来源：`server/internal/models/models.go`
> 生成代码散见于：`server/internal/db/users.go`、`server/internal/auth/auth.go`、`server/internal/config/config.go`

---

## 一、原始代码

```go
package models

import (
	"encoding/json"
	"time"
)

type User struct {
    ID          string    `json:"id"`
    Email       string    `json:"email,omitempty"`
    Username    string    `json:"username"`
    AvatarColor string    `json:"avatar_color"`
    AvatarURL   string    `json:"avatar_url,omitempty"`
    Status      string    `json:"status"`
    LastSeen    time.Time `json:"last_seen,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
}

type PinnedContent struct {
    Content  string    `json:"content"`
    PinnedAt time.Time `json:"pinned_at"`
}

type Chat struct {
    ID              string     `json:"id"`
    Type            string     `json:"type"`
    Name            string     `json:"name,omitempty"`
    IconColor       string     `json:"icon_color,omitempty"`
    Visibility      string     `json:"visibility,omitempty"`
    OwnerID         string     `json:"owner_id,omitempty"`
    CreatedAt       time.Time  `json:"created_at"`
    LastMessageAt   time.Time  `json:"last_message_at"`
    MemberCount     int        `json:"member_count"`
    UnreadCount     int        `json:"unread_count"`
    PinnedMessage   *PinnedContent `json:"pinned_message,omitempty"`
    LastMessage     *Message   `json:"last_message,omitempty"`
}

type ChatMember struct {
    ChatID            string    `json:"chat_id"`
    UserID            string    `json:"user_id"`
    Role              string    `json:"role"`
    LastSeen          time.Time `json:"last_seen,omitempty"`
    JoinedAt          time.Time `json:"joined_at"`
    LastReadMessageID string    `json:"last_read_message_id,omitempty"`
}

type Message struct {
    ID              string       `json:"id"`
    ChatID          string       `json:"chat_id"`
    UserID          string       `json:"user_id"`
    Author          *User        `json:"author,omitempty"` // Deprecated.
    Content         string       `json:"content"`
    CreatedAt       time.Time    `json:"created_at"`
    EditedAt        *time.Time   `json:"edited_at,omitempty"`
    DeletedAt       *time.Time   `json:"deleted_at,omitempty"`
	AttachmentCount int          `json:"attachment_count"`
	MentionCount    int          `json:"mention_count"`
	ReactionCount   int          `json:"reaction_count"`
	Attachments     json.RawMessage `json:"attachments,omitempty"`
	Reactions       json.RawMessage `json:"reactions,omitempty"`
	Mentions        json.RawMessage `json:"mentions,omitempty"`
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
    Emoji string `json:"emoji"`
    Count int    `json:"count"`
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

> **⚠️ 重要提示**：所有标注为 `(Deprecated)` 的字段目前仅为兼容性保留。在收到明确指令前，**严禁删除**任何标注为 `(Deprecated)` 的字段。

### User

| 字段 | 类型 | JSON | 来源 | 生成规则 |
|------|------|------|------|----------|
| ID | `string` | `"id"` | 服务端生成 | `uuid.NewString()` → UUID v4 |
| Email | `string` | `"email"` | 用户输入 | `strings.ToLower` + `strings.TrimSpace`；仅查重 |
| Username | `string` | `"username"` | 用户输入 | `strings.TrimSpace`；仅查重 |
| AvatarColor | `string` | `"avatar_color"` | 服务端计算 | `PickColor(username)` → palette\[uuid.ID() % 8\]（UUID 取模） |
| AvatarURL | `string` | `"avatar_url"` | 用户上传 | 外部 `upload.moonchan.xyz` 返回的 URL |
| Status | `string` | `"status"` | 服务端设置 | 默认 `"offline"`，WS 连接时 `"online"` |
| LastSeen | `time.Time` | `"last_seen"` | 服务端更新 | WS 连接/断开 及 发消息时更新为 `now` |
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
| LastMessageAt | `time.Time` | `"last_message_at"` | 服务端更新 | 发消息时 `UPDATE chats SET last_message_at = now`；DB NULL 时降级为 `CreatedAt` |
| MemberCount | `int` | `"member_count"` | 服务端计算 | 子查询 `(SELECT COUNT(*) FROM chat_members WHERE chat_id = c.id)` |
| UnreadCount | `int` | `"unread_count"` | (Deprecated) 服务端计算 | `COUNT(messages) WHERE deleted_at IS NULL AND (created_at,id) > lastReadID` |
| PinnedMessage | `*PinnedContent` | `"pinned_message"` | 用户动作 | JSON 对象 `{"content","pinned_at"}`；由 `SetPinnedMessage` 写入 |
| LastMessageID | `string` | `"last_message_id"` | 服务端查询 | 最后一条消息的 ID |
| LastMessage | `*Message` | `"last_message"` | (Deprecated) 服务端查询 | `fetchMessageRow(chat_id, LIMIT 1)`·无 `attachExtras` |

### ChatMember

| 字段 | 类型 | JSON | 来源 | 生成规则 |
|------|------|------|------|----------|
| ChatID | `string` | `"chat_id"` | 关联 | 引用 `chats(id)` ON DELETE CASCADE |
| UserID | `string` | `"user_id"` | 关联 | 引用 `users(id)` ON DELETE CASCADE |
| Role | `string` | `"role"` | 服务端设置 | `"owner"` (创建者), `"admin"` (管理员), `""` (普通成员) |
| LastSeen | `time.Time` | `"last_seen"` | 服务端更新 | WS 连接/断开 及 发消息时更新为 `now` |
| JoinedAt | `time.Time` | `"joined_at"` | 服务端生成 | SQLite default |
| LastReadMessageID | `string` | `"last_read_message_id"` | 用户动作 | 调用 `POST /read` 时更新 |

### Message

| 字段 | 类型 | JSON | 来源 | 生成规则 |
|------|------|------|------|----------|
| ID | `string` | `"id"` | 服务端生成 | UUID v4 |
| ChatID | `string` | `"chat_id"` | 用户关联 | 引用 `chats(id)` |
| UserID | `string` | `"user_id"` | 用户关联 | 引用 `users(id)` |
| Author | `*User` | `"author"` | (Deprecated) JOIN 查询 | `messages JOIN users` |
| Content | `string` | `"content"` | 用户输入 | 最大 4000 字符，超长返回 403/`content_too_long` |

| CreatedAt | `time.Time` | `"created_at"` | 服务端生成 | Go `time.Now().UTC().Format("2006-01-02T15:04:05.000Z")` |
| EditedAt | `*time.Time` | `"edited_at"` | 服务端设置 | `UpdateMessage` 时记录当前时间 |
| DeletedAt | `*time.Time` | `"deleted_at"` | 服务端设置 | soft delete（设 `deleted_at=now`, `content=''`） |
| AttachmentCount | `int` | `"attachment_count"` | 服务端设置 | 写入时从 `attachments` 数组长度计算 |
| MentionCount | `int` | `"mention_count"` | 服务端设置 | 写入时从 `mentions` 数组去重后长度计算 |
| ReactionCount | `int` | `"reaction_count"` | 服务端更新 | `AddReaction`/`RemoveReaction` 时 `COUNT(*)` 重新计算 |
| Attachments | `json.RawMessage` | `"attachments"` | JSON 列 | 存储在 `messages.attachments` TEXT 列中 |
| Reactions | `json.RawMessage` | `"reactions"` | JSON 列 | 存储在 `messages.reactions` TEXT 列中 |
| Mentions | `json.RawMessage` | `"mentions"` | JSON 列 | 存储在 `messages.mentions` TEXT 列中 |

### Attachment

| 字段 | 类型 | JSON | 来源 | 生成规则 |
|------|------|------|------|----------|
| ID | `string` | `"id"` | 服务端生成 | 若客户端未传，用 `NewID()` |
| MessageID | `string` | `"message_id"` | 关联 | 引用 `messages(id)` |
| Filename | `string` | `"filename"` | 用户传入 | `filepath.Base` + `Replace("..", "_")`；若长度 > 200 则改为 `file + ext` |
| MimeType | `string` | `"mime_type"` | 用户/推断 | 根据 `Content-Type` 或 `mime.TypeByExtension`，默认 `application/octet-stream` |
| Size | `int64` | `"size"` | 用户传入 | 文件字节数 |
| URL | `string` | `"url"` | 用户传入 | 指向 `upload.moonchan.xyz` 外部存储 |

### Reaction（API 响应聚合结构）

**说明**：该结构非数据库原始行，而是基于 `reactions` 表的 `GROUP BY emoji` 聚合结果。

| 字段 | 类型 | JSON | 来源 | 生成规则 |
|------|------|------|------|----------|
| Emoji | `string` | `"emoji"` | 用户输入 | 最大 32 字符 |
| Count | `int` | `"count"` | 服务端聚合 | `GROUP BY emoji → COUNT(*)` |

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
// server/internal/db/users.go:24-31
func PickColor(seed string) string {
    if seed == "" { return "#5865F2" }
    id, _ := uuid.Parse(seed)
    return palette[int(id.ID()) % 8]
}
```

**规则**：将 seed 解析为 UUID，取 `id.ID()`（UUID 第 9‑16 字节的前 8 位）→ 取模 8 → 从固定 8 色调色板选色。

### 3.3 时间戳

| 位置 | 精度 | 格式 |
|------|------|------|
| SQL 默认值（migration） | 微秒 | `strftime('%Y-%m-%dT%H:%M:%fZ','now')` |
| Go `CreateMessage` | 毫秒 | `"2006-01-02T15:04:05.000Z"` |
| Go `UpdateMessage` | 毫秒 | 同上 |
| Go `DeleteMessage` | 纳秒 | `time.RFC3339Nano` |
| Go `AddReaction` | 纳秒 | SQLite default |
| Go `CreateRefreshToken` | 纳秒 | `time.RFC3339Nano` |
| Go `PurgeExpiredTokens` | 纳秒 | `time.RFC3339Nano` |

**规则**：统一使用 UTC，以 TEXT 存储。

### 3.4 Token 生成

| Token 类型 | 算法 | 长度 | 存储 |
|------------|------|------|------|
| Access Token | `HS256 JWT` + `claims{userID, exp, iat, sub}` | 不定（约 190 字节） | 不存储，全靠签名验证 |
| Refresh Token raw | `crypto/rand(32)` → `base64.RawURLEncoding` | 43 字符 | 不存储（只在 HTTP cookie 中） |
| Refresh Token hash | `sha256(raw)` → `hex` | 64 字符 | 存表 |

### 3.5 密码

```go
// server/internal/auth/auth.go:99-104
HashPassword → bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
if len(password) > 72 { password = password[:72] } // bcrypt 硬限制
```

**规则**：bcrypt cost 10；仅因 bcrypt 72 字节限制截断。无最小长度、无格式要求。
**变更**：移除了 8 字符最小长度检查。

### 3.6 邮箱

```go
// server/internal/auth/auth.go:115-117
NormalizeEmail → strings.ToLower + strings.TrimSpace
```

**规则**：只做小写和去空格。不再校验格式、不再调用 `mail.ParseAddress`。
**变更**：移除了 email 格式验证。

### 3.7 用户名

```go
// server/internal/auth/auth.go:127-131
ValidateUsername → strings.TrimSpace, 非空检查
```

**规则**：只做去空格和空值检查。无长度限制、无字符限制。
**变更**：移除了 2‑32 长度、控制字符、最大长度等所有校验。

### 3.8 上传

```
废弃，保留代码但标记 Deprecated。
前端直接上传到 upload.moonchan.xyz，返回 URL + MIME 后随消息体传入。
```

### 3.9 消息内容

```go
// server/internal/db/messages.go
if len(content) > 4000 {
    return nil, errors.New("content too long, use file upload instead")
}
```

**规则**：最大 4000 字符。超长内容不再截断，而是直接返回 403/`content_too_long`，强制用户通过附件上传。

### 3.10 Emoji 反应

```go
// server/internal/db/messages.go
emoji = strings.TrimSpace(emoji)
if emoji == "" || len(emoji) > 32 { return error }
```

**规则**：非空、≤32 字符。API 响应中仅包含 `emoji` 与 `count`。

### 3.11 消息详情懒加载

```
?details=true 控制 attachExtras（1 条子查询）。默认跳过。
LastMessage（聊天列表预览）永远不查 attachExtras。
```

**变更**：Attachments、Reactions 与 Mentions 现在全部通过 `messages` 表的 JSON 列直接读取，不再使用子查询。

---

## 四、数据流图

```
用户输入
  ├── email / username / password ──→ User（仅查重，无格式限制）
  ├── chat type / name / visibility ──→ Chat
  ├── message content (≤4000) ──→ Message；超长 → 403
  ├── file upload（upload.moonchan.xyz）──→ Attachment URL
  └── emoji ──→ Reaction

服务端计算
  ├── uuid.NewString() ──→ 所有实体 ID
  ├── PickColor(uuid) ──→ AvatarColor / IconColor（UUID.ID() 取模）
  ├── time.Now().UTC() ──→ CreatedAt / ExpiresAt / LastMessageAt / DeletedAt
  ├── bcrypt(password) ──→ password_hash（72 字节截断）
  ├── JWT(jti, uid, exp) ──→ access_token
  ├── sha256(randomBytes) ──→ refresh_token hash
  ├── status "online"/"offline" (WS hook)
  └── counts ──→ attachment_count / mention_count / reaction_count（写入时计算）

服务端推断（子查询 / JOIN，仅 ?details=true）
  ├── Members ──→ ChatMember → JOIN users
  ├── Attachments ──→ JSON 列存储
  ├── Reactions ──→ JSON 列存储
  ├── Mentions ──→ JSON 列存储
  └── UnreadCount ──→ COUNT(messages) WHERE deleted_at IS NULL AND >lastRead
```

---

## 五、约束汇总

| 约束 | 检查位置 | 违反后果 | 变更说明 |
|------|----------|----------|----------|
| 唯一 email | 数据库 UNIQUE 索引 | 409/`already_taken` | 移除了格式校验 |
| 唯一 username | 数据库 UNIQUE 索引 | 409/`already_taken` | 移除了长度/字符校验 |
| chat type = dm / group | DB CHECK 约束 | 语句失败 | 不变 |
| message content ≤ 4000 | DB 层 `CreateMessage`/`UpdateMessage` | 403/`content_too_long` | 变更：从静默截断改为拒绝 |
| emoji ≤ 32 字符 | DB 层校验 | 400/`bad_request` | 不变 |
| 外键引用 | DB FOREIGN KEY | 级联删除或 ABORT | 不变 |
| 密码 bcrypt 72 字节 | `HashPassword`/`VerifyPassword` 截断 | 自动截断 | 移除了 8 字符最小长度 |
| 附件上限 20MB（默认） | `MaxUploadBytes` config | 413/`too_large` | 废弃（前端直传 upload.moonchan.xyz） |
| 时间格式 | Go layout 解析 | 回退 `time.Time{}` | 不变 |
