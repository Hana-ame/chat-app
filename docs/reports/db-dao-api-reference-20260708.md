# DB / DAO 接口参考

Package: `github.com/Hana-ame/chat-app/server/internal/db`  
数据库: SQLite（纯 Go 驱动 `modernc.org/sqlite`）  
连接: `MaxOpenConns(1)`, WAL 模式, 外键 ON  

---

## 目录

- [db.go — 基础设施](#dbgo--基础设施)
- [users.go — User 操作](#usersgo--user-操作)
- [chats.go — Chat & RefreshToken 操作](#chatsgo--chat--refreshtoken-操作)
- [chats_ext.go — 扩展 Chat 操作](#chats_extgo--扩展-chat-操作)
- [messages.go — Message & Reaction & Mention 操作](#messagesgo--message--reaction--mention-操作)

---

## db.go — 基础设施

### `Open(path string) (*DB, error)`

| 项 | 内容 |
|---|------|
| **操作** | 初始化数据库连接 + 运行迁移 |
| **目标 Model** | — |
| **SQL** | `PRAGMA journal_mode=WAL`, `busy_timeout=5000`, `foreign_keys=ON` |

```go
func Open(path string) (*DB, error) {
    dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_time_format=sqlite", path)
    conn, err := sql.Open("sqlite", dsn)
    // ...
    d := &DB{DB: conn}
    if err := d.Migrate(); err != nil {
        conn.Close()
        return nil, err
    }
    return d, nil
}
```

### `(d *DB) Migrate() error`

| 项 | 内容 |
|---|------|
| **操作** | 执行嵌入的 SQL 迁移文件，按文件名排序，幂等（忽略 `duplicate column` 错误） |
| **目标 Model** | — |

```go
//go:embed migrations/*.sql
var migrationFS embed.FS

func (d *DB) Migrate() error {
    entries, _ := fs.ReadDir(migrationFS, "migrations")
    // sort names → read → ExecContext
    for _, n := range names {
        b, _ := migrationFS.ReadFile("migrations/" + n)
        if _, err := d.ExecContext(context.Background(), string(b)); err != nil {
            if !isDupColumnErr(err) {
                return fmt.Errorf("apply migration %s: %w", n, err)
            }
        }
    }
    return nil
}
```

### 辅助函数

| 函数 | 用途 |
|------|------|
| `NewID() string` | 生成 UUID v4 |
| `PickColor(seed string) string` | 根据字符串哈希选择一个颜色 |
| `parseTime(s string) time.Time` | 解析多种时间格式为 UTC Time |
| `parseTimePtr(s sql.NullString) *time.Time` | 同上，返回指针 |
| `isDupColumnErr(err error) bool` | 判断是否为 `duplicate column` 错误 |

---

## users.go — User 操作

### `(d *DB) CreateUser(ctx, email, username, passwordHash string) (*models.User, error)`

| 项 | 内容 |
|---|------|
| **操作** | **C** — 插入新用户 |
| **目标 Model** | `models.User` |
| **表** | `users` |
| **返回** | 创建后的完整 User（含 id, color） |

```go
func (d *DB) CreateUser(ctx context.Context, email, username, passwordHash string) (*models.User, error) {
    id := NewID()
    color := PickColor(username)
    _, err := d.ExecContext(ctx,
        `INSERT INTO users (id, email, username, password_hash, avatar_color) VALUES (?,?,?,?,?)`,
        id, email, username, passwordHash, color,
    )
    if err != nil {
        if strings.Contains(err.Error(), "UNIQUE") {
            return nil, ErrConflict
        }
        return nil, err
    }
    return d.GetUserByID(ctx, id)
}
```

### `(d *DB) GetUserByID(ctx, id string) (*models.User, error)`

| 项 | 内容 |
|---|------|
| **操作** | **R** — 按 ID 查用户 |
| **目标 Model** | `models.User` |
| **SQL** | `SELECT ... FROM users WHERE id = ?` |

```go
func (d *DB) GetUserByID(ctx context.Context, id string) (*models.User, error) {
    err := d.QueryRowContext(ctx,
        `SELECT id, email, username, avatar_color, avatar_url, status, created_at FROM users WHERE id = ?`,
        id,
    ).Scan(&u.ID, ...)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, ErrNotFound
    }
    // ...
}
```

### `(d *DB) GetUserByEmail(ctx, email string) (*models.User, string, error)`

| 项 | 内容 |
|---|------|
| **操作** | **R** — 按邮箱查用户（含密码 hash） |
| **目标 Model** | `models.User` + `passwordHash` |
| **SQL** | `SELECT id, email, username, avatar_color, avatar_url, status, last_seen, created_at, password_hash FROM users WHERE email = ?` |
| **返回** | `(*User, passwordHash, error)` |

### `(d *DB) UpdateUserProfile(ctx, id, username, avatarColor, avatarURL string) (*models.User, error)`

| 项 | 内容 |
|---|------|
| **操作** | **U** — 更新用户资料 |
| **目标 Model** | `models.User` |
| **表** | `users` |
| **返回** | 更新后的 User |

```go
func (d *DB) UpdateUserProfile(ctx context.Context, id, username, avatarColor, avatarURL string) (*models.User, error) {
    _, err := d.ExecContext(ctx,
        `UPDATE users SET username = ?, avatar_color = ?, avatar_url = ? WHERE id = ?`,
        username, avatarColor, avatarURL, id,
    )
    // ...
    return d.GetUserByID(ctx, id)
}
```

### `(d *DB) UpdateUserStatus(ctx, id, status string) error`

| 项 | 内容 |
|---|------|
| **操作** | **U** — 更新在线状态 |
| **目标 Model** | `models.User`（仅 status 字段） |
| **SQL** | `UPDATE users SET status = ? WHERE id = ?` |

### `(d *DB) SearchUsers(ctx, query string, limit int) ([]models.User, error)`

| 项 | 内容 |
|---|------|
| **操作** | **R** — 按用户名/邮箱搜索（LIKE） |
| **目标 Model** | `[]models.User` |
| **SQL** | `SELECT id, username, avatar_color, avatar_url, status, last_seen, created_at FROM users ...` |
| **注意** | limit 上限 50，默认 25 |

```go
func (d *DB) SearchUsers(ctx context.Context, query string, limit int) ([]models.User, error) {
    rows, err := d.QueryContext(ctx,
        `SELECT id, username, avatar_color, avatar_url, status, created_at FROM users
         WHERE username LIKE ? OR email LIKE ?
         ORDER BY username LIMIT ?`,
        "%"+query+"%", "%"+strings.ToLower(query)+"%", limit,
    )
    // ...
}
```

### Sentinel 错误

```go
var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")
```

---

## chats.go — Chat & RefreshToken 操作

### RefreshToken

#### `(d *DB) CreateRefreshToken(ctx, userID, tokenHash string, ttl time.Duration) (*models.RefreshToken, error)`

| 项 | 内容 |
|---|------|
| **操作** | **C** — 创建 refresh token 记录 |
| **目标 Model** | `models.RefreshToken` |
| **表** | `refresh_tokens` |

```go
func (d *DB) CreateRefreshToken(ctx context.Context, userID, tokenHash string, ttl time.Duration) (*models.RefreshToken, error) {
    id := NewID()
    expires := time.Now().UTC().Add(ttl)
    _, err := d.ExecContext(ctx,
        `INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES (?,?,?,?)`,
        id, userID, tokenHash, expires.Format(time.RFC3339Nano),
    )
    // ...
}
```

#### `(d *DB) FindRefreshToken(ctx, tokenHash string) (*models.RefreshToken, error)`

| 项 | 内容 |
|---|------|
| **操作** | **R** — 按 hash 查 refresh token |
| **SQL** | `SELECT ... FROM refresh_tokens WHERE token_hash = ?` |

#### `(d *DB) DeleteRefreshToken(ctx, id string) error`

| 项 | 内容 |
|---|------|
| **操作** | **D** — 按 ID 删除单条 refresh token |

#### `(d *DB) DeleteUserRefreshTokens(ctx, userID string) error`

| 项 | 内容 |
|---|------|
| **操作** | **D** — 删除某用户所有 refresh token（登出时用） |
| **SQL** | `DELETE FROM refresh_tokens WHERE user_id = ?` |

#### `(d *DB) PurgeExpiredTokens(ctx) (int64, error)`

| 项 | 内容 |
|---|------|
| **操作** | **D** — 清理过期 token（定时任务） |
| **SQL** | `DELETE FROM refresh_tokens WHERE expires_at < ?` |

---

### Chat

#### `(d *DB) CreateChat(ctx, typ, name, visibility, ownerID string, memberIDs []string) (*models.Chat, error)`

| 项 | 内容 |
|---|------|
| **操作** | **C** — 创建群聊或私聊（含成员写入） |
| **目标 Model** | `models.Chat` + `chat_members` |
| **事务** | ✅ 使用 `BeginTx` |

```go
func (d *DB) CreateChat(ctx context.Context, typ, name, visibility, ownerID string, memberIDs []string) (*models.Chat, error) {
    tx, err := d.BeginTx(ctx, nil)
    defer tx.Rollback()
    // INSERT INTO chats ...
    // for each member: INSERT OR IGNORE INTO chat_members ...
    tx.Commit()
    return d.GetChat(ctx, id)
}
```

#### `(d *DB) GetChat(ctx, id string) (*models.Chat, error)`

| 项 | 内容 |
|---|------|
| **操作** | **R** — 查询 chat 详情 |
| **目标 Model** | `models.Chat` |
| **SQL** | `SELECT ... pinned_message, (SELECT COUNT(*) FROM chat_members WHERE chat_id = ?) AS member_count FROM chats WHERE id = ?` |

```go
func (d *DB) GetChat(ctx context.Context, id string) (*models.Chat, error) {
    // SELECT ... FROM chats WHERE id = ?
    // + d.GetChatMembers() 填充 c.Members
    // + d.LastMessage() 填充 c.LastMessage（仅在 ListUserChats 中）
    // + d.UnreadCount()（仅在 ListUserChats 中）
}
```

#### `(d *DB) ListUserChats(ctx, userID string) ([]models.Chat, error)`

| 项 | 内容 |
|---|------|
| **操作** | **R** — 查询用户所有聊天（含成员数、最后消息、未读数） |
| **目标 Model** | `[]models.Chat` |
| **排序** | `last_message_at DESC` |
| **SQL** | `SELECT ... (SELECT COUNT(*) FROM chat_members WHERE chat_id = c.id) AS member_count FROM chat_members JOIN chats ...` |

```go
func (d *DB) ListUserChats(ctx context.Context, userID string) ([]models.Chat, error) {
    rows, err := d.QueryContext(ctx,
        `SELECT c.id, c.type, c.name, c.icon_color, c.visibility, c.owner_id,
                c.created_at, c.last_message_at,
                cm.last_read_message_id, COALESCE(cm.pinned,0)
         FROM chat_members cm JOIN chats c ON c.id = cm.chat_id
         WHERE cm.user_id = ?
         ORDER BY cm.pinned DESC, COALESCE(c.last_message_at, c.created_at) DESC`, userID,
    )
    // for each row: d.GetChatMembers + d.LastMessage + d.UnreadCount
}
```

#### `(d *DB) FindDMBetween(ctx, a, b string) (*models.Chat, error)`

| 项 | 内容 |
|---|------|
| **操作** | **R** — 查询两人之间是否已有私聊 |
| **SQL** | 使用两个 JOIN 找出两个成员都存在的 type='dm' chat |

```go
func (d *DB) FindDMBetween(ctx context.Context, a, b string) (*models.Chat, error) {
    err := d.QueryRowContext(ctx,
        `SELECT c.id FROM chats c
         JOIN chat_members cm1 ON cm1.chat_id = c.id AND cm1.user_id = ?
         JOIN chat_members cm2 ON cm2.chat_id = c.id AND cm2.user_id = ?
         WHERE c.type = 'dm' LIMIT 1`, a, b,
    ).Scan(&id)
    return d.GetChat(ctx, id)
}
```

#### `(d *DB) IsChatMember(ctx, chatID, userID string) (bool, error)`

| 项 | 内容 |
|---|------|
| **操作** | **R** — 检查用户是否为聊天成员 |
| **SQL** | `SELECT 1 FROM chat_members WHERE chat_id = ? AND user_id = ?` |

#### `(d *DB) GetChatMembers(ctx, chatID string) ([]models.User, error)`

| 项 | 内容 |
|---|------|
| **操作** | **R** — 查询聊天成员列表 |
| **目标 Model** | `[]models.User` |
| **SQL** | `SELECT u.id, u.username, u.avatar_color, u.avatar_url, u.status, u.last_seen, u.created_at FROM chat_members cm JOIN users u ...` |

#### `(d *DB) AddChatMember(ctx, chatID, userID string) error`

| 项 | 内容 |
|---|------|
| **操作** | **C** — 添加成员（幂等） |
| **目标 Model** | `chat_members` 记录 |

```go
func (d *DB) AddChatMember(ctx context.Context, chatID, userID string) error {
    res, err := d.ExecContext(ctx,
        `INSERT OR IGNORE INTO chat_members (chat_id, user_id) VALUES (?,?)`,
        chatID, userID,
    )
    // if n==0 → ErrConflict
}
```

#### `(d *DB) RemoveChatMember(ctx, chatID, userID string) error`

| 项 | 内容 |
|---|------|
| **操作** | **D** — 删除成员 |
| **表** | `chat_members` |

#### `(d *DB) DeleteChat(ctx, chatID string) error`

| 项 | 内容 |
|---|------|
| **操作** | **D** — 删除聊天（级联删除 messages, members, attachments, reactions, mentions） |

#### `(d *DB) RenameChat(ctx, chatID, name string) error`

| 项 | 内容 |
|---|------|
| **操作** | **U** — 重命名群聊 |
| **SQL** | `UPDATE chats SET name = ? WHERE id = ?` |

#### `(d *DB) UpdateLastRead(ctx, chatID, userID, messageID string) error`

| 项 | 内容 |
|---|------|
| **操作** | **U** — 更新最后读取消息位置 |
| **表** | `chat_members` |
| **SQL** | `UPDATE chat_members SET last_read_message_id = ? WHERE chat_id = ? AND user_id = ?` |

---

## chats_ext.go — 扩展 Chat 操作

#### `(d *DB) ListPublicChats(ctx) ([]models.Chat, error)`

| 项 | 内容 |
|---|------|
| **操作** | **R** — 列出所有公开群聊 |
| **目标 Model** | `[]models.Chat` |
| **过滤** | `type = 'group' AND visibility = 'public'` |

```go
func (d *DB) ListPublicChats(ctx context.Context) ([]models.Chat, error) {
    rows, err := d.QueryContext(ctx,
        `SELECT id, type, name, icon_color, COALESCE(visibility,'private'),
                owner_id, created_at, last_message_at
         FROM chats WHERE type = 'group' AND visibility = 'public'
         ORDER BY created_at DESC`,
    )
    // + d.GetChatMembers() per row
}
```

#### `(d *DB) JoinChatByID(ctx, chatID, userID string) error`

| 项 | 内容 |
|---|------|
| **操作** | **C** — 加入公开/未列出聊天 |
| **检查** | 拒绝 `visibility = 'private'` |
| **SQL** | `INSERT OR IGNORE INTO chat_members` |

#### `(d *DB) PinChat(ctx, chatID, userID string) error`

| 项 | 内容 |
|---|------|
| **操作** | **U** — 置顶聊天 |
| **SQL** | `UPDATE chat_members SET pinned = 1 WHERE chat_id = ? AND user_id = ?` |

#### `(d *DB) UnpinChat(ctx, chatID, userID string) error`

| 项 | 内容 |
|---|------|
| **操作** | **U** — 取消置顶 |
| **SQL** | `UPDATE chat_members SET pinned = 0 WHERE chat_id = ? AND user_id = ?` |

---

## messages.go — Message & Reaction & Mention 操作

### Message

#### `(d *DB) CreateMessage(ctx, chatID, userID, content string, mentions []string, attachments []models.Attachment) (*models.Message, error)`

| 项 | 内容 |
|---|------|
| **操作** | **C** — 创建消息（含 mention、attachment 写入） |
| **目标 Model** | `models.Message` |
| **事务** | ✅ `BeginTx` |
| **实现** | 附件和提及均通过 JSON 序列化存入 `messages.attachments` 和 `messages.mentions` 列 |
| **副作用** | 同时更新 `chats.last_message_at` 和 `users.last_seen` |

```go
func (d *DB) CreateMessage(ctx context.Context, chatID, userID, content string, mentions []string, attachments []models.Attachment) (*models.Message, error) {
    tx, err := d.BeginTx(ctx, nil)
    defer tx.Rollback()

    // 1. INSERT INTO messages
    // 2. UPDATE chats SET last_message_at = ?
    // 3. for each mention: INSERT OR IGNORE INTO mentions
    // 4. for each attachment: INSERT INTO attachments

    tx.Commit()
    return d.GetMessage(ctx, id)
}
```

#### `(d *DB) GetMessage(ctx, id string) (*models.Message, error)`

| 项 | 内容 |
|---|------|
| **操作** | **R** — 按 ID 查消息（含 author, attachments, reactions, mentions） |
| **目标 Model** | `models.Message` |
| **实现** | 附件、反应、提及均直接从 `messages` 表的 JSON 列读取，无需子查询 |

```go
func (d *DB) GetMessage(ctx context.Context, id string) (*models.Message, error) {
    m, err := d.fetchMessageRow(ctx, `SELECT ... FROM messages m JOIN users u ON u.id = m.user_id WHERE m.id = ?`, id)
    d.attachExtras(ctx, m, "")  // 填充 Attachments, Reactions, Mentions
    return m, nil
}
```

#### `(d *DB) GetMessages(ctx, chatID, viewerID, before string, limit int) ([]models.Message, error)`

| 项 | 内容 |
|---|------|
| **操作** | **R** — 分页查询消息（按时间倒序） |
| **目标 Model** | `[]models.Message` |
| **实现** | 附件、反应、提及均从 JSON 列读取 |
| **分页** | `before` 为消息 ID 时使用游标 `(created_at, id) <` |
| **排序** | 从 DB 取 DESC，Go 中反转回 ASC |

```go
func (d *DB) GetMessages(ctx context.Context, chatID, viewerID, before string, limit int) ([]models.Message, error) {
    if before == "" {
        // ORDER BY created_at DESC, id DESC LIMIT ?
    } else {
        // WHERE chat_id = ? AND (created_at, id) < (SELECT created_at, id FROM messages WHERE id = ?)
    }
    // for each: d.attachExtras()
    // reverse slice to chronological order
}
```

#### `(d *DB) LastMessage(ctx, chatID string) (*models.Message, error)`

| 项 | 内容 |
|---|------|
| **操作** | **R** — 查聊天最后一条消息 |
| **方法** | 先 `SELECT id ... ORDER BY created_at DESC LIMIT 1`，再 `GetMessage` |

#### `(d *DB) UnreadCount(ctx, chatID, lastReadID string) (int, error)`

| 项 | 内容 |
|---|------|
| **操作** | **R** — 统计未读消息数 |
| **条件** | `deleted_at IS NULL` 且大于最后读取位置 |

```go
func (d *DB) UnreadCount(ctx context.Context, chatID, lastReadID string) (int, error) {
    if lastReadID == "" {
        return COUNT(*) WHERE chat_id = ? AND deleted = 0
    }
    return COUNT(*) WHERE chat_id = ? AND deleted = 0 AND (created_at, id) > (SELECT ...)
}
```

#### `(d *DB) UpdateMessage(ctx, id, userID, content string) (*models.Message, error)`

| 项 | 内容 |
|---|------|
| **操作** | **U** — 编辑消息（仅作者可编辑，且未删除） |
| **限制** | `user_id = ? AND deleted_at IS NULL` |

```go
func (d *DB) UpdateMessage(ctx context.Context, id, userID, content string) (*models.Message, error) {
    res, err := d.ExecContext(ctx,
        `UPDATE messages SET content = ?, edited_at = ? WHERE id = ? AND user_id = ? AND deleted = 0`,
        content, time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), id, userID,
    )
    return d.GetMessage(ctx, id)
}
```

#### `(d *DB) DeleteMessage(ctx, id, userID string, allowAny bool) error`

| 项 | 内容 |
|---|------|
| **操作** | **U**（软删除）— 标记 `deleted_at = now` 并清空 content |
| **权限** | `allowAny=true` 时忽略 `user_id` 检查（群主可删） |
| **注意** | 非硬删除，级联 attachments/reactions/mentions 保留 |

```go
func (d *DB) DeleteMessage(ctx context.Context, id, userID string, allowAny bool) error {
    if allowAny {
        UPDATE messages SET deleted = 1, content = '' WHERE id = ?
    } else {
        UPDATE messages SET deleted = 1, content = '' WHERE id = ? AND user_id = ?
    }
}
```

---

### Attachment（内部方法 - 已弃用）

#### `(d *DB) attachmentsFor(ctx, messageID string) ([]models.Attachment, error)`

| 项 | 内容 |
|---|------|
| **操作** | **R** — 查消息的所有附件（Deprecated） |
| **注意** | 现改为从 `messages.attachments` JSON 列直接读取 |

---

### Reaction

#### `(d *DB) AddReaction(ctx, messageID, userID, emoji string) error`

| 项 | 内容 |
|---|------|
| **操作** | **C** — 添加表情回应（幂等） |
| **目标 Model** | `reactions` 记录 |
| **表** | `reactions` |
| **SQL** | `INSERT OR IGNORE INTO reactions (message_id, user_id, emoji) VALUES (?,?,?)` |

#### `(d *DB) RemoveReaction(ctx, messageID, userID, emoji string) error`

| 项 | 内容 |
|---|------|
| **操作** | **D** — 删除表情回应 |
| **SQL** | `DELETE FROM reactions WHERE message_id = ? AND user_id = ? AND emoji = ?` |

#### `(d *DB) reactionsFor(ctx, messageID, viewerID string) ([]models.Reaction, error)`

| 项 | 内容 |
|---|------|
| **操作** | **R** — 查消息的所有反应（已按 emoji 聚合） |
| **目标 Model** | `[]models.Reaction`（含 count, user_ids, me） |

```go
func (d *DB) reactionsFor(ctx context.Context, messageID, viewerID string) ([]models.Reaction, error) {
    rows, err := d.QueryContext(ctx,
        `SELECT emoji, user_id FROM reactions WHERE message_id = ? ORDER BY created_at`, messageID,
    )
    // 内存聚合：group by emoji → count + user_ids + me
}
```

---

### Mention（内部方法）

#### `(d *DB) mentionsFor(ctx, messageID string) ([]string, error)`

| 项 | 内容 |
|---|------|
| **操作** | **R** — 查消息中提及的用户 ID 列表 |
| **目标 Model** | `[]string`（用户 ID） |
| **表** | `mentions` |
| **SQL** | `SELECT user_id FROM mentions WHERE message_id = ?` |

---

## 汇总统计

| 文件 | 方法数 | 操作模型 | 说明 |
|------|--------|----------|------|
| `db.go` | 2 | — | 初始化 + 迁移 |
| `users.go` | 6+3 helper | `User` | CRUD + 搜索 |
| `chats.go` | 13 | `RefreshToken` + `Chat` + `chat_members` | 含事务、含锁 |
| `chats_ext.go` | 4 | `Chat` + `chat_members` | 公开聊天 + 置顶 |
| `messages.go` | 12 | `Message` + `Attachment` + `Reaction` + `Mention` | 含游标分页、软删除 |

**总计：36 个 `*DB` 接收器方法**

```
CRUD 分布:
  Create: 8  (User, RefreshToken, Chat, Message, Attachment, Mention, Reaction, chat_member)
  Read:   11 (User ×2, RefreshToken, Chat ×3, Message ×3, Attachment, Reaction, Mention)
  Update: 7  (User ×2, Chat ×2, Message, chat_member ×2)
  Delete: 8  (RefreshToken ×3, Chat, chat_member, Message, Reaction, purge)
```
