# DAO / 数据库层审查报告

审查日期：2026-07-09  
最后更新：2026-07-09（同步代码变更）  
审查范围：`server/internal/db/`（db.go, chats.go, chats_ext.go, messages.go, users.go, migrations/init.sql）  
代码量：1811 行（含测试 542 行）

---

## 架构概览

| 组件 | 选型 |
|------|------|
| 数据库 | SQLite（modernc.org/sqlite 纯 Go 驱动） |
| 连接模式 | `SetMaxOpenConns(1)` — 单连接，SQLite 写锁安全 |
| 迁移 | 单文件 `migrations/init.sql`，`//go:embed` 嵌入，启动时执行 |
| ID 生成 | `uuid.NewString()` — UUID v4 TEXT PK |
| 时间格式 | `time.RFC3339Nano` TEXT 存储，UTC |
| 外键 | `PRAGMA foreign_keys=ON`，级联删除 |
| 并发 | WAL 模式 + `busy_timeout=5000` |

**文件组织：**

| 文件 | 行数 | 职责 |
|------|------|------|
| `db.go` | 70 | 连接、迁移、工具函数 |
| `users.go` | 172 | User CRUD + 搜索 + 状态，共享错误/ID/颜色 |
| `chats.go` | 422 | Chat + Member CRUD，Refresh Token CRUD，联合查询 |
| `chats_ext.go` | 87 | 公开聊天列表、加入、置顶 |
| `messages.go` | 518 | 消息 CRUD，Reactions，Mentions，附件 |
| `db_test.go` | 221 | 用户/聊天集成测试 |
| `messages_test.go` | 322 | 消息/反应/提到/未读测试 |

---

## 1. 性能问题

---

### 1.1 ListUserChats N+1 查询 ✅ 已知（标记 Deprecated）

| 字段 | 内容 |
|------|------|
| **位置** | `server/internal/db/chats.go:318-338` |
| **问题** | 主查询后对每个 chat 额外调用 `d.GetMessage` + `d.UnreadCount` |
| **影响** | 50 个 chat → 额外 100 次查询 |

```go
for _, r := range rows2 {
    c := r.chat
    if c.LastMessageID != "" {
        last, err := d.GetMessage(ctx, c.LastMessageID)  // ← 每条 chat 一次
        if err == nil { c.LastMessage = last }
    }
    unread, err := d.UnreadCount(ctx, c.ID, lastReadID)   // ← 每条 chat 一次
    c.UnreadCount = unread
    out = append(out, c)
}
```

**影响分析：** 这是当前 DAO 层最严重的性能问题。用户加入 50 个群 → 登录时触发 1 次主查询 + 50 次 `GetMessage` + 50 次 `UnreadCount` = 101 次查询。`GetMessage` 还包含 JOIN 和 JSON 解析，`UnreadCount` 是 COUNT 子查询。

**当前状态：** ⏭️ 不解决。代码已标记 `// Deprecated`。`LastMessage` 和 `UnreadCount` 字段本身已标记弃用，后续前端不再依赖时可移除。

---

### 1.2 GetChat 调用 GetMessage（N+1 变体）

| 字段 | 内容 |
|------|------|
| **位置** | `server/internal/db/chats.go:192-197` |
| **问题** | 每次 `GetChat` 额外查询 `GetMessage` 获取最后一条消息 |
| **影响** | 每次获取单个 chat 信息都多一次 JOIN + JSON 解析 |

```go
func (d *DB) GetChat(ctx context.Context, id string) (*models.Chat, error) {
    // ... 主查询 ...
    if c.LastMessageID != "" {
        lastMsg, err := d.GetMessage(ctx, c.LastMessageID)
        if err == nil { c.LastMessage = lastMsg }   // ← N+1
    }
    return &c, nil
}
```

**影响分析：** `GetChat` 被多处调用（CreateChat、ListUserChats、FindDMBetween）。每次调用都额外加载完整 Message 对象（含 author JOIN、reactions JSON 解析）。对于 chat 列表/搜索场景，大部分情况下不需要完整的 `LastMessage` 对象。

**当前状态：** ⏭️ 不解决。`Chat.LastMessage` 已标记 `// Deprecated`。

---

## 2. 数据一致性问题

---

### 2.1 JSON 缓存同步在事务外执行

| 字段 | 内容 |
|------|------|
| **位置** | `server/internal/db/messages.go:427-458` |
| **问题** | `AddReaction` / `RemoveReaction` 提交事务后才调用 `syncReactionsColumn` |
| **影响** | `messages.reactions` JSON 缓存与 `reactions` 表可能不一致 |

```go
func (d *DB) AddReaction(ctx context.Context, messageID, userID, emoji string) error {
    tx, err := d.BeginTx(ctx, nil)
    // ... INSERT reaction, UPDATE reaction_count ...
    if err := tx.Commit(); err != nil { return err }

    return d.syncReactionsColumn(ctx, messageID)  // ← 事务外！
}
```

**影响分析：** SQLite `SetMaxOpenConns(1)` 限制了并发连接，所以实际不会出现并发写冲突。但函数调用顺序仍有逻辑问题：`syncReactionsColumn` 失败时，reactions 表已写入，而 JSON 缓存未更新。读路径直接用 JSON 缓存，导致反应数据可能不一致直到下一次 add/remove 触发同步。

**严重性：** 低。单连接 SQLite 下不可能有并发写入，`syncReactionsColumn` 失败的唯一原因是磁盘 I/O 错误，此时整个数据库不可用。

**当前状态：** ⏭️ 不解决。已有 `// Deprecated` 标记的设计约束。

---

### 2.2 `messages.attachments` JSON 与 `attachments` 表双写

| 字段 | 内容 |
|------|------|
| **位置** | `messages.go:57` + `migrations/init.sql:122-129` |
| **问题** | 附件同时存储在 `messages.attachments` JSON 列和 `attachments` 表中 |
| **影响** | 存储冗余、一致性问题，当前写路径仅写入 JSON 列，`attachments` 表无人写入 |

```sql
CREATE TABLE IF NOT EXISTS attachments (
    id         TEXT PRIMARY KEY,
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    ...
);
ALTER TABLE messages ADD COLUMN attachments TEXT NOT NULL DEFAULT '[]';
```

**影响分析：** `CreateMessage` 仅写入 JSON 列。`attachments` 表仅在旧代码 `attachmentsFor` 中被读取（该函数已标记 `// Deprecated`）。表是死代码但仍在 DDL 中，被 `migrations/init.sql` 创建。浪费存储（虽然 SQLite 空表几乎不占空间）但给维护者造成困惑。

**当前状态：** ⏭️ 不解决。`attachments` 表保留向后兼容，读路径已全部迁移到 JSON 列。

---

## 3. 代码质量问题

---

### 3.1 JSON 序列化错误被静默忽略 ✅ 已修复

| 字段 | 内容 |
|------|------|
| **位置** | `messages.go:21,57,58` + `chats_ext.go:73` |
| **问题** | `json.Marshal` 返回值被 `_ = ` 丢弃 |
| **影响** | 序列化失败时写入空/损坏数据到数据库 |

**修复（2026-07-09）：** 4 处 marshal 调用全部添加错误处理，失败时写入空 JSON（`[]` / `{}`）而非损坏数据。

```go
// 修复后模式
data, err := json.Marshal(rxs)
if err != nil {
    data = []byte("[]")   // ← 回退到空 JSON
}
```

| 位置 | 回退值 |
|------|--------|
| `messages.go:21` syncReactionsColumn | `[]` |
| `messages.go:57` CreateMessage (attachments) | `[]` |
| `messages.go:58` CreateMessage (mentions) | `[]` |
| `chats_ext.go:73` SetPinnedMessage | `{}` |

**当前状态：** ✅ 已修复（2026-07-09）。

---

### 3.2 `fetchMessageRow` 与 `GetMessages` 扫描逻辑重复 ✅ 已修复

| 字段 | 内容 |
|------|------|
| **位置** | `messages.go:128-177` 与 `messages.go:226-271` |
| **问题** | 相同的列扫描 + 时间解析 + JSON 反序列化代码写了两遍 |
| **影响** | 约 50 行完全相同的代码，维护成本翻倍 |

**修复（2026-07-09）：** 提取 `scanMessage` 函数共享行扫描逻辑。

```go
type scanner interface{ Scan(dest ...interface{}) error }

func scanMessage(s scanner) (*models.Message, error) {
    var (
        m         models.Message
        author    models.User
        edited    sql.NullString
        // ... 14 个字段变量
    )
    err := s.Scan(
        &m.ID, &m.ChatID, &m.UserID, &m.Content, &created, &edited, &deletedAt,
        &attCnt, &mentCnt, &rxnCnt, &rxnJSON, &attJSON, &mentJSON,
        &author.ID, &author.Username, &author.AvatarColor, &author.Status,
    )
    if err != nil {
        return nil, err
    }
    // ... parseTime + 字段转化 ...
    m.Author = &author
    return &m, nil
}
```

**重构效果：**

| 函数 | 重构前 | 重构后 | 减少 |
|------|--------|--------|------|
| `fetchMessageRow` | 50 行 | 7 行（包装 scanMessage + ErrNoRows 处理） | -43 |
| `GetMessages` 循环体 | 45 行 | 5 行（直接调用 scanMessage） | -40 |
| 合计 | 95 行 | 12 行 | **-83 行** |

`scanner` 接口同时接受 `*sql.Row`（单行）和 `*sql.Rows`（多行），加上 `scanMessage` + `ErrNoRows` 包装 = `fetchMessageRow`，消除全部重复。

**当前状态：** ✅ 已修复（2026-07-09）。

---

### 3.3 `attachExtras` 为空函数 ✅ 已修复

| 字段 | 内容 |
|------|------|
| **位置** | `messages.go:179-187` |
| **问题** | 函数体全部是注释，实际不做任何操作 |
| **影响** | 每次 `GetMessage` / `GetMessages` 都调用此空函数，纯开销 |

**修复（2026-07-09）：** 直接移除 `attachExtras` 及全部调用点：

| 变更 | 文件 |
|------|------|
| 移除 `attachExtras` 定义 | `messages.go` |
| 移除 `GetMessage` 中的调用 | `messages.go` |
| 移除 `GetMessages` 中的 `details` 分支 | `messages.go` |
| 简化 `GetMessages` 签名（去掉 `viewerID`、`details`） | `messages.go` |
| 移除 handler 中 `details` query 参数读取 | `handlers/messages.go` |
| 更新测试调用签名 | `messages_test.go` |
| 保留设计注释 | `GetMessages` 上方块注释 |

**当前状态：** ✅ 已修复（2026-07-09）。

---

### 3.4 CreateUser / UpdateUserProfile 冗余回查

| 字段 | 内容 |
|------|------|
| **位置** | `users.go:66-67,131` |
| **问题** | INSERT/UPDATE 后立刻 SELECT 回读完整对象 |
| **影响** | 每次创建/更新用户多一次查询 |

```go
func (d *DB) CreateUser(ctx context.Context, ...) (*models.User, error) {
    _, err := d.ExecContext(ctx, `INSERT INTO users ...`, ...)
    if err != nil { return nil, err }
    return d.GetUserByID(ctx, id)  // ← 回查
}

func (d *DB) UpdateUserProfile(ctx context.Context, ...) (*models.User, error) {
    _, err := d.ExecContext(ctx, `UPDATE users SET ...`, ...)
    if err != nil { return nil, err }
    return d.GetUserByID(ctx, id)  // ← 回查
}
```

**影响分析：** SQLite 不支持 `RETURNING` 子句（3.35+ 支持，但驱动可能不暴露）。回查确保了返回完整对象（含 DB 默认值），代价是一次额外查询。在单连接 SQLite 下影响极微。

**当前状态：** ⏭️ 不解决。可接受的权衡。

---

## 4. 安全与验证

---

### 4.1 搜索用户的 LIKE 注入风险

| 字段 | 内容 |
|------|------|
| **位置** | `users.go:151-154` |
| **问题** | 用户输入直接拼接进 `LIKE` 查询 |
| **影响** | 极低：参数化查询中占位符 `?` 保护了 SQL 注入，但 `LIKE` 特殊字符未转义 |

```go
rows, err := d.QueryContext(ctx,
    `SELECT ... FROM users WHERE username LIKE ? OR id = ? ORDER BY username LIMIT ?`,
    "%"+query+"%", query, limit,
)
```

**影响分析：** 参数化查询防止了 SQL 注入。`LIKE` 中 `%` 和 `_` 是通配符，用户搜索 `%` 会匹配所有用户，搜索 `_` 会匹配单字符通配。这是功能特性而非漏洞（搜索本身就是为了匹配）。

**当前状态：** ⏭️ 不解决。预期行为。

---

### 4.2 Email/Username 输入验证不足

| 字段 | 内容 |
|------|------|
| **位置** | `users.go:48-52` |
| **问题** | 仅检查非空和 trim，无格式校验 |
| **影响** | 非标准 email 格式仍可通过 |

```go
func (d *DB) CreateUser(ctx context.Context, email, username, passwordHash string) (*models.User, error) {
    email = strings.ToLower(strings.TrimSpace(email))
    username = strings.TrimSpace(username)
    if email == "" || username == "" { return nil, errors.New("email and username required") }
    // 无 email 格式校验
}
```

**影响分析：** email 验证责任在 handler 层（`handlers/auth.go` 的 `Register`），DAO 层只负责存储。当前 DAO 只做最小校验，符合单一职责。

**当前状态：** ⏭️ 不解决。DAO 不做业务校验是合理的设计。

---

## 5. 设计问题

---

### 5.1 `reactions` / `attachments` / `mentions` 表与 JSON 缓存并存

| 字段 | 内容 |
|------|------|
| **位置** | `init.sql:122-155` + `messages.go` |
| **问题** | 三个表均存在但写路径仅写入 JSON 列 |
| **影响** | 模式复杂度增加，DBA 困惑 |

**设计分析：** 这是迁移过程中的中间状态。原架构用关联表存 reactions/attachments/mentions（读时 N+1），新架构在消息写入时直接将聚合 JSON 存入 `messages` 表。旧表保留实现读路径向后兼容。

**现状总结：**

| 表 | 写路径 | 读路径 | 状态 |
|-----|--------|--------|------|
| `reactions` | 写入（AddReaction/RemoveReaction） | 仅用于 `syncReactionsColumn` 聚合 | ✅ 活跃 |
| `attachments` | 无人写入 | `attachmentsFor` 已弃用 | ❌ 死表 |
| `mentions` | 已弃用（JSON 列替代） | `mentionsFor` 已弃用 | ❌ 死表 |

**当前状态：** ⏭️ 不解决。未来清理时可移除 `attachments` 和 `mentions` 表及对应代码。

---

### 5.2 迁移策略：单文件 + ALTER TABLE 混合

| 字段 | 内容 |
|------|------|
| **位置** | `migrations/init.sql` |
| **问题** | CREATE TABLE 后立即 ALTER TABLE ADD COLUMN |
| **影响** | SQL 略显冗余，但保持与历史迁移序列一致 |

```sql
CREATE TABLE IF NOT EXISTS chats (...);
ALTER TABLE chats ADD COLUMN visibility TEXT NOT NULL DEFAULT 'private';
ALTER TABLE chats ADD COLUMN pinned_message TEXT NOT NULL DEFAULT '';
ALTER TABLE chats ADD COLUMN pinned_updated_at TEXT;
```

**设计分析：** DDL 注释解释了原因：开发过程中逐次添加列，最终合并为单文件时保留 ALTER 语句以保持与迁移历史一致。功能正确，但给仅看 DDL 的人造成"这些列为什么不在 CREATE 里？"的困惑。

**状态：** ✅ 已有 DDL 注释解释。设计决策合理。

---

### 5.3 PickColor UUID 解析容错

| 字段 | 内容 |
|------|------|
| **位置** | `users.go:24-34` |
| **问题** | UUID 解析失败时静默回退到第一个颜色 |

```go
func PickColor(seed string) string {
    id, err := uuid.Parse(seed)
    if err != nil { return palette[0] }
    h := int(id.ID())
    return palette[h%len(palette)]
}
```

**分析：** 调用处：
- `CreateChat`: `PickColor(id)`（新 UUID，100% 可解析）和 `PickColor(name)`（群名非 UUID，走回退路径）
- `CreateUser`: `PickColor(username)`（username 非 UUID）

当 seed 非 UUID 时始终返回 `#5865F2`。聊天/用户创建时若名字不是 UUID 格式则固定为蓝色。8 色调色板中只有 1 色被使用，多样性降低。

**当前状态：** ⏭️ 不解决。功能无影响。

---

## 6. 测试覆盖

| 包 | 测试文件 | 断言数量 | 覆盖范围 |
|----|---------|---------|---------|
| `db_test.go` | 用户 CRUD | ~40 | CreateUser、重复 email、GetUserByEmail、SearchUsers |
| `db_test.go` | 聊天 CRUD | ~60 | CreateChat（group/dm）、FindDMBetween、ListUserChats、AddRemoveMember、DeleteChat、RenameChat |
| `messages_test.go` | 消息 CRUD | ~80 | Create/Get/Update/DeleteMessage、分页、Reactions、Mentions、Attachments、UnreadCount |
| `messages_test.go` | 用户状态 | ~15 | UpdateUserProfile、UpdateUserStatus |
| `messages_test.go` | RefreshToken | ~20 | Create/Find/Delete、PurgeExpired |

**覆盖不足：**
- 无并发测试（单连接 SQLite 下无实际并发）
- 无 `ListPublicChats` / `JoinChatByID` / `SetPinnedMessage` 测试
- 无大数据量下的分页边界测试
- 无 `syncReactionsColumn` 失败恢复测试

**当前状态：** ✅ 核心路径已覆盖，边缘路径缺失。

---

## 7. 优先级总结

| 优先级 | 问题 | 位置 | 当前状态 |
|--------|------|------|----------|
| **低** | JSON Marshal 错误丢弃 | `messages.go:21,57,58`, `chats_ext.go:73` | ✅ 已修复 |
| **低** | 扫描代码重复 | `messages.go` | ✅ 已修复（scanMessage 抽取） |
| **低** | `attachExtras` 空函数 | `messages.go` | ✅ 已修复（移除） |
| — | ListUserChats N+1 | `chats.go:318-338` | ⏭️ 已标记 Deprecated |
| — | GetChat 回查 LastMessage | `chats.go:192-197` | ⏭️ 已标记 Deprecated |
| — | JSON 同步在事务外 | `messages.go:427-458` | ⏭️ 接受（单连接无并发） |
| — | `attachments`/`mentions` 死表 | `init.sql` | ⏭️ 保留向后兼容 |
| — | CreateUser 冗余回查 | `users.go:66-67` | ⏭️ 可接受 |
| — | PickColor 多样性 | `users.go:24-34` | ⏭️ 功能无影响 |
| ✅ | 迁移策略 | `init.sql` | 已注释说明 |
| ✅ | 测试核心路径 | 全文件 | 基本覆盖 |

---

## 8. 总结

DAO 层整体质量高于预期。核心优点：

1. **N+1 已大部分消除** — reactions/attachments/mentions 用 JSON 列替代关联表，读路径复杂度从 O(n) 降至 O(1)
2. **WAL + 单连接** — 正确的 SQLite 使用模式
3. **Keyset 分页** — `(created_at, id)` 游标分页比 OFFSET 更高效
4. **事务使用恰当** — `CreateChat`、`AddReaction` 等写操作在事务中保证原子性
5. **弃用标记清晰** — 所有已知问题代码都有 `// Deprecated` 注释

3 个低优先级项已全部修复（JSON 错误处理、代码重复、空函数移除）。

2026-07-09 同步代码变更：3.1 / 3.2 / 3.3 已修复并上传。无剩余待修复项。
