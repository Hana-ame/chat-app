# 弃用项调用链全面调查

> 2026-07-12 — 针对 `// Deprecated.` 标记与实际使用不一致的问题，逐项追踪调用链

---

## 1. `Chat.UnreadCount`（models/models.go:35）

```go
// Deprecated.
UnreadCount int `json:"unread_count"`
```

### 设置位置

**唯一生产者：** `db/chats.go:349-354` 在 `ListUserChats()` 中
```
unread, err := d.UnreadCount(ctx, c.ID, lastReadID)  // 行 349
c.UnreadCount = unread                                   // 行 354
```

### 读取位置

- **Go 后端：** `internal/testutil/handler_test.go:423`（`c.UnreadCount > 0` 检查——仅测试验证，非业务逻辑）
- **JS 前端：** `store/chat.js:171`（`unread_count: old.unread_count || 0`——在 `setChats` 合并期间保留）
- **JS 前端：** `store/chat.js:237`（有新消息时自增：`(c.unread_count || 0) + 1`）
- **JS 前端：** `store/chat.js:296`（在 `setActiveChat` 中重置为 0）
- **JS 前端：** `components/ChatListItem.jsx:21,49`（`const unread = chat.unread_count || 0`，显示 `unread-badge`）
- **JS 前端：** `dev/dummy.js:276`（模拟数据生成器设置硬编码的 `unread_count`）

### 调用链

```
ListUserChats()  →  UnreadCount() SQL 查询  →  设置到 Chat.UnreadCount
                                           →  JSON 序列化到前端
                                              →  前端 store 保留/自增/显示
```

### 能否移除？

**不可以——没有可行的替代品。** 尽管标记为"已弃用"，但从未引入替代方案。`UnreadCount` 字段是：
1. 从数据库计算（`ListUserChats` → `UnreadCount` SQL）
2. 通过 JSON 传送到前端
3. 前端广泛使用——在侧边栏显示未读红色标记（`ChatListItem.jsx:49`），在收到新消息时自增（`chat.js:237`），在选择聊天时重置（`chat.js:296`）

### 弃用状态：**错误标记。** 标记为"已弃用"，但仍是这个应用中唯一的未读计数机制。

### 移除前需要做什么

引入替代方案：在聊天成员表中添加一个 `unread_count` 列（或 `last_read_message_at` 时间戳），并在服务器端而不是客户端追踪它。这会是一个重大特性。

---

## 2. `Chat.LastMessage`（models/models.go:42）

```go
// Deprecated.
LastMessage *Message `json:"last_message,omitempty"`
```

### 设置位置

- `db/chats.go:197-202`（`GetChat` 中）：如果 `LastMessageID` 不为空，通过 `GetMessage(ctx, c.LastMessageID)` 加载完整 `*Message`
- `db/chats.go:338-343`（`ListUserChats` 中）：同上——每个聊天额外一次 `GetMessage` 调用

### 读取位置

- **Go 后端：** `testutil/handler_test.go:422`（测试验证 `c.LastMessage.Content == "hello from bob"`）
- **JS 前端：** `components/ChatListItem.jsx:43`（`chat.last_message.author?.username + ': ' + chat.last_message.content`——在行预览中显示）
- **JS 前端：** `store/chat.js:169`（`setChats` 中在合并时保留字段）
- **JS 前端：** `store/chat.js:237`（`onMessageCreate` 中设置 `last_message: msg`）
- **JS 前端：** `api/mock.js:95,323`（模拟列表构建器和 `dummy.js` 生成器填充它）
- **JS 前端：** `dev/dummy.js:277,323`（模拟数据）

### 替换：`last_message_id`

列存在于 `chats` 表中（`init.sql:59`）。通过以下方式设置：
- `db/messages.go:76`（`UPDATE chats SET last_message_at = ?, last_message_id = ? WHERE id = ?` 在 `CreateMessage` 中）

通过以下方式读取：
- 在 `GetChat`（`chats.go:196`）和 `ListUserChats`（`chats.go:324`）中扫描为 `c.LastMessageID`

**替换不完整。** `last_message_id` 是一个高效的字符串 ID，但前端需要显示用户界面预览（`"用户名: 消息内容"`）。为了不加载完整的 `Message` 对象，需要一个新的轻量级端点或字段——也许是 `last_message_preview`（一个包含 `{user_id, content_preview, deleted}` 的结构体），或者在前端 store 中独立缓存。

### 能否移除 `LastMessage`？

技术上可以，**但**：
- `last_message_id` 替换列仅提供 ID，不提供内容/作者
- 前端 `ChatListItem.jsx:43` 需要消息内容和作者用户名来显示预览行
- 如果没有替代的轻量级机制，移除 `LastMessage` 会破坏聊天列表预览

### 弃用状态：**部分弃用但未完成迁移。** 添加了列，但旧字段仍被后端和前端活跃使用。`GetChat`/`ListUserChats` 中的 N+1 负载仍然存在。

### 移除前需要做什么

1. 添加 `last_message_user_id`、`last_message_content`、`last_message_deleted` 到 `chats` 行（通过 `CreateMessage` 中的触发器或更新设置）
2. 用这些轻量级列替换 `LastMessage *Message`
3. 更新前端 `ChatListItem.jsx` 使用新字段
4. 移除 `GetChat` 和 `ListUserChats` 中的 N+1 `GetMessage` 调用

---

## 3. `Message.Author`（models/models.go:62）

```go
// Deprecated.
Author *User `json:"author,omitempty"`
```

### 设置位置

`db/messages.go:133-178` `scanMessage()` 函数始终填充它：
```go
err := s.Scan(
    &m.ID, ..., &m.UserID, ..., &author.ID, &author.Username, &author.AvatarColor, &author.Status,
)
m.Author = &author  // 行 177
```

每次通过 `GetMessage`、`GetMessages`、`CreateMessage`、`UpdateMessage`、`LastMessage` 获取消息时触发——所有这些都调用 `scanMessage`。

### 读取位置

- **Go 测试：** `db/messages_test.go:25`（`msg.Author == nil || msg.Author.Username != "MsgUser"` 断言）
- **JS 前端：** `components/MessageItem.jsx:30-34`——核心 fallback：
    ```js
    const author = useMemo(() => {
        const chat = chats.find(c => c.id === chatId);
        if (msg.user_id === user.id) return user;
        return chat?.members?.find(m => m.id === msg.user_id) || msg.author || { ... };
    }, [chats, chatId, msg.user_id, msg.author, user]);
    ```
- **JS 前端：** `components/ChatListItem.jsx:43`：`chat.last_message.author?.username`
- **JS 前端：** `api/mock.js:95,234,254,280`（模拟作者数据）

### 调用链

```
scanMessage() → 总是 SQL JOIN users 表  →  m.Author = &author
     ↓
GetMessage / GetMessages / CreateMessage / UpdateMessage / LastMessage
     ↓
JSON 响应 → 前端 MessageItem → fallback 解析
```

### 能否移除？

**技术上可以，但非常复杂。** 其声称的替代方案是使用 `chat.members[]`（消息视图可用），但在 `ChatListItem` 预览行和消息加载前的初始渲染中用作 fallback。

前端在 `MessageItem.jsx:33` 中有一个三层 fallback：
1. 检查 `chat.members[]` 中的用户（主要来源）
2. 回退到 `msg.author`（默认）
3. 回退到硬编码的 "Unknown"

如果你移除 `Author`，第二层 fallback 消失，但第一层通常有效（消息加载时 `chats` 列表已填充成员）。然而：
- `ChatListItem.jsx:43`（`chat.last_message.author?.username`）会中断，除非你在 `last_message` 被移除之前保留它，或者你向 `last_message` 添加了一个独立的 `username` 字段
- 由于消息是通过 `scanMessage` 中的 `JOIN` 获取的，移除 `Author` 不会提高性能——`users` 表无论如何都会被 JOIN。你只会停止填充 Go 模型中的 `.Author` 字段

### 弃用状态：**误标记。** 这在消息获取的单个 SQL 查询中几乎没有成本（始终 JOIN users），并且是有用的缓存/fallback。没有一个高效的替代方案可以仅通过一个 ID 获取作者信息而不需要额外的查询。

### 移除前需要做什么

1. 确保前端可以从 `chat.members` 或类似的地方解析作者
2. 移除 `MessageItem.jsx` 中的 `msg.author` fallback
3. 移除 `ChatListItem.jsx` 中对 `chat.last_message.author?.username` 的依赖
4. 从 `scanMessage` 中移除 SQL `JOIN` 和 `Author` 赋值
5. 更新测试（`messages_test.go:25`、`handler_test.go:422`）

---

## 4. `ChatMember.LastReadMessageID`（models/models.go:52）

```go
// Deprecated.
LastReadMessageID string `json:"last_read_message_id,omitempty"`
```

### SQL 设置位置

`last_read_message_id` 列在 `init.sql:81` 中定义：
```sql
last_read_message_id TEXT
```

由 `UpdateLastRead` 更新（`chats.go:432-438`）：
```sql
UPDATE chat_members SET last_read_message_id = ? WHERE chat_id = ? AND user_id = ?
```

### SQL 读取位置

在 `ListUserChats`（`chats.go:274`）中扫描为 `cm.last_read_message_id`：
```go
SELECT ..., cm.last_read_message_id, ... FROM chat_members cm JOIN chats c ...
```
但扫描到 `sql.NullString` 本地变量 `lastRead`（行 295），然后**从未映射到 `ChatMember.LastReadMessageID`**。相反，它被用作 `UnreadCount()` 的输入（行 345-354）。

### Go 模型读取位置

**Go 中无读取器。** 字段 `LastReadMessageID` 在 Go 代码中任何地方都未被引用（除了模型定义本身）。它被 JSON 序列化，但从来没有任何消费者真正依赖 `chat_member.last_read_message_id` JSON 字段。

### 调用链

```
MarkRead handler → UpdateLastRead() SQL 写入 → last_read_message_id 列
                                                       ↓
ListUserChats → 读取 last_read_message_id 列（到本地变量）
                     ↓ 用于 UnreadCount 计算（从不填充到 ChatMember.LastReadMessageID）
```

### 能否移除？

**可以，从 Go 模型中移除。** 该列仍被 `UpdateLastRead` 和 `ListUserChats` 内部使用，但 Go 模型字段 `LastReadMessageID` 从未被读取。这是序列化死代码。列本身不能移除（`UnreadCount` 函数和 `UpdateLastRead` 依赖它），但 JSON 字段不需要暴露。

### 弃用状态：**Go 模型中正确弃用。** 列仍然活跃且被需要；只是模型序列化是死的。

### 移除前需要做什么

从 `ChatMember` 结构体中移除 `LastReadMessageID` 字段。不涉及功能损失。清理前端的任何 `last_read_message_id` 引用（当前没有）。

---

## 5. `CreateOrGetDM` / `POST /api/dms`（handlers/chat.go:98-137，router.go:79）

### 生产者

**路由：** `router.go:79`：`r.Post("/dms", s.CreateOrGetDM) // Deprecated.`
**处理器：** `chat.go:98-137`——接收 `{ "user_id": "..." }`，调用 `FindDMBetween`，如果不存在则回退到 `CreateChat(typ:"dm")`。

### 消费者

- **前端 `client.js:95-96`：** `createDM: (token, userId) => request('POST', '/api/dms', token, { user_id: userId })`——已定义但**未在 UI 中调用**。
- **前端 `mock.js:159-181`：** `mockCreateDM`——已注册但**未被 UI 调用**。
- **测试：** 多个 Go 测试调用底层的 `FindDMBetween` 和 `CreateChat(typ:"dm")`，但**没有测试命中了 `/api/dms` HTTP 端点**。

### 前端 DM 创建流

当前创建 DM 的 UI 方式**不存在**：`DmSearchPanel` 组件在 `ChatList.jsx` 中甚至没有被导入；`ChatList.jsx:120-126` 中的 `filteredChats` 通过 `if (c.type === 'dm') return false` 过滤掉 DM。DM 甚至不在侧边栏中显示。

### 调用链

```
[无前端调用者]  →  POST /api/dms  →  CreateOrGetDM
                                         →  FindDMBetween (DB)
                                         →  CreateChat(typ:"dm") (DB)
                                         →  Hub.BroadcastChatCreated
```

### 能否移除？

**可以。** 该路由未被 UI 调用。DM 功能似乎被完全暂停了。DM 创建的一般机制（`CreateChat(typ:"dm")`、`FindDMBetween`）也被 Go 测试用于 DM 功能，但 HTTP 端点是死的。

### 弃用状态：**正确弃用。** 前端没有 DM UI，也没有调用这个端点。

### 移除前需要做什么

1. 从 `router.go` 中移除路由
2. 从 `chat.go` 中移除 `CreateOrGetDM` 处理器
3. 从 `client.js` 中移除 `createDM` API 方法
4. 从 `mock.js` 中移除 `mockCreateDM` 函数
5. 从 `MOCKABLE` 数组中移除映射
6. 决定 DM 功能的未来：要么完全关闭它（安全的——没有 UI 路径），要么稍后重新激活（在这种情况下，保留路由但移除其 `// Deprecated.` 标签）

---

## 6. `Upload` 处理器 + 路由（router.go:103,116；uploads.go:39-115）

### 生产者

**路由：** `router.go:103`：`r.Post("/uploads", s.Upload)` + 行 116：`r.Get("/uploads/*", s.serveUpload)`
**处理器：** `uploads.go:39-115`——完整的 multipart 表单文件上传处理器，包含 MIME 验证、大小限制、磁盘写入。

### 前端消费者

**前端根本不调用这个。** `Composer.jsx:56` 中的上传路径是：
```js
const UPLOAD_BASE = 'https://upload.moonchan.xyz';
const data = await api.upload(f);  // 调用 client.js:134-147
```
`client.js` 中的 `api.upload` 发送到：
```js
const res = await fetch(UPLOAD_BASE + '/api/upload', { method: 'PUT', body: file });
```
**零请求到达服务器的 `POST /api/uploads`。**

### 测试消费者

**`TestUploadNotLoggedIn`**（handler_test.go:432）——专门测试这个处理器。
**`TestUploadFile`**（handler_test.go:616）——测试完整的上传路径。
**`TestUploadExceedsSizeLimit`**（handler_test.go:647）——测试大小限制。
**`TestUploadRejectsUnsupportedMime`**（handler_test.go:669）——测试 MIME 验证。
**`TestUpload_MissingFile`**（handler_test.go:1575）——测试缺少文件字段。

共有 5 个测试直接命中这个处理器。

### 调用链

```
[零前端调用]  →  POST /api/uploads  →  Upload handler
                                        →  os.MkdirAll(UploadDir)
                                        →  将文件写入磁盘
                                        →  JSON 响应 { url, filename, ... }

[零前端调用]  →  GET /uploads/{key}  →  serveUpload → 提供本地文件
```

### 能否移除？

**可以——但测试会失败。** 移除处理器和路由会使 5 个测试失败。需要重写测试以使用 `upload.moonchan.xyz` 外部 URL 或模拟上传。

### 弃用状态：**正确弃用且为死代码（生产环境）。** 前端已经 100% 迁移到外部上传服务。唯一使用者是测试套件。

### 移除前需要做什么

1. 从 `router.go` 中移除路由（`POST /api/uploads` 和 `GET /uploads/*`）
2. 移除 `uploads.go` 中的 `Upload` 处理器
3. 移除 `router.go` 中的 `serveUpload`
4. 移除 `uploads.go` 中的辅助函数（`randomKey`、`sanitizeFilename`、`allowedMime`）
5. 重写 5 个上传测试以模拟上传到 `upload.moonchan.xyz` 或使用可丢弃的本地测试端点
6. 可选：从 `server/cmd/chatd/main.go:46` 中移除 `os.MkdirAll(cfg.UploadDir, ...)`

---

## 7. URL 查询令牌回退（handler.go:77-84，sse.go:18-21，gateway.go:44-48）

### 所在地

- **`handler.go:77-84`：** `bearerToken()` 函数——首先检查 `Authorization: Bearer`，然后回退到 `r.URL.Query().Get("access_token")`，然后检查 `Cookie`
- **`sse.go:18`：** 注释说明查询字符串是为 EventSource API 兼容保留的
- **`ws/gateway.go:44`：** WebSocket 升级**仅**使用 `r.URL.Query().Get("access_token")`

### 前端消费者

- **SSE 连接（`store/chat.js:99-100`）：** `api.sseUrl(token)` 返回 `API_BASE + '/api/events?access_token=' + encodeURIComponent(token)`。然后 `new EventSource(url)` 打开连接
- **WebSocket 连接（`store/chat.js:42`）：** `const url = proto + '://' + host + '/ws?access_token=' + token`
- **测试（`client.go:207`）：** `WSURL()` 返回 `ws://host/ws?access_token=...`
- **测试（`handler_test.go:1204`）：** SSE 测试使用 `"/api/events?access_token=" + ...`

### EventSource 能否使用 Authorization 头？

**不能。** 浏览器 [EventSource API](https://developer.mozilla.org/en-US/docs/Web/API/EventSource) 不支持自定义头。你不能在 `EventSource(url)` 构造函数上设置 `Authorization`。唯一的可能性是 cookie。

### WebSocket 能否使用 Authorization 头？

**技术上可以** 在升级请求中使用 `Authorization` 头，但 gorilla/websocket 升级器也经常依赖查询参数，因为 WebSocket 客户端（例如浏览器中的 JS `new WebSocket(url)`）不能设置自定义头。一些实现支持 `Sec-WebSocket-Protocol` 中的子协议，但 gorilla/websocket 标准用法是查询参数。

### 调用链

```
前端：new EventSource("/api/events?access_token=...")
                                        ↓
SSE 处理器：bearerToken() → 从 URL 提取 access_token  → 验证
                                        ↓
前端：new WebSocket("/ws?access_token=...")
                                        ↓
ws/gateway.go: tok = r.URL.Query().Get("access_token")  → 验证
```

### 能否移除查询令牌？

**不可以完全移除。** EventSource **必须**使用查询参数（不能使用头）。WebSocket **必须**使用查询参数（浏览器 `new WebSocket()` 无法设置头）。

### 弃用状态：**误标记。** 这对 EventSource 和浏览器 WebSocket 是必要的，因为 API 限制。不能移除，除非：
- 所有连接都切换到 cookie 认证（用于 SSE 的 `credentials: 'include'`，也是可行的）
- 所有 WebSocket 客户端切换到 cookie 或子协议认证（更难——gorilla/websocket 不常用这个）

### 移除前需要做什么

1. **对于 SSE（可行）：** 使用 `withCredentials: true` 或 `credentials: 'include'` 和设置 cookie。不需要查询参数
2. **对于 WS（困难）：** 迁移到 cookie 认证或 WebSocket 子协议。需要自定义 gorilla/websocket 升级器逻辑
3. 删除 `bearerToken()` 中的 `access_token` 查询回退
4. 更新所有前端连接代码

---

## 8. `attachmentsFor`（db/messages.go:334-353）

### 定义

```go
// Deprecated.
func (d *DB) attachmentsFor(ctx context.Context, messageID string) ([]models.Attachment, error) {
    SELECT id, message_id, filename, mime_type, size, url FROM attachments WHERE message_id = ?
}
```

### 引用者

**整个代码库中零个引用**——没有被 Go 文件、测试或任何东西调用。该函数是 `attachmentsFor`（小写 'a'），是包私有的，因此不能从包外调用。

### `attachments` 表状态

该表在 `init.sql:123-132` 中创建，但**从未被写入**。`CreateMessage`（`messages.go:68-71`）将附件数据作为 JSON 存储在 `messages.attachments` 列中。`attachments` 表是一个完全的死表——零插入、零更新、零读取除了这个死函数。

### 调用链

```
[无调用者]  →  attachmentsFor  →  SELECT FROM attachments
```

### 能否移除？

**可以，完全移除函数和表。**

### 弃用状态：**正确弃用且已死。** 迁移到 JSON 列是完整的。

### 移除前需要做什么

1. 从 `messages.go` 中移除此函数
2. 从 `init.sql` 中移除 `attachments` 表 DDL
3. 可选：清理任何维护备注（列上的注释说"已弃用"）

---

## 9. `FindDMBetween`（db/chats.go:364-385）

### 定义

```go
// Deprecated.
func (d *DB) FindDMBetween(ctx context.Context, a, b string) (*models.Chat, error) {
    SELECT c.id FROM chats c ... WHERE c.type = 'dm' LIMIT 1
    return d.GetChat(ctx, id)
}
```

### 引用者

1. **处理器 `chat.go:124`：** `CreateOrGetDM` 内部唯一的生产用途
2. **测试 `db_test.go`：** 三个测试：
    - `TestFindDMBetween`（行 715）
    - `TestFindDMBetween_Self`（行 233）
    - `TestFindDMBetween_NotFound`（行 242）

### 调用链

```
CreateOrGetDM (处理器)  →  FindDMBetween (DB)  →  GetChat (DB)
                                                    →  GetMessage (N+1!)
```

### 能否移除？

**如果您也移除 `CreateOrGetDM`，则可以，但测试会失败。** `FindDMBetween` 是 DM 创建流程的一部分。HTTP 端点不被 UI 调用，但底层函数被 `CreateOrGetDM` 使用。

### 弃用状态：**正确弃用。** 它只被一个同样被弃用的端点使用。

### 移除前需要做什么

1. 移除 `CreateOrGetDM` 处理器（参见上述 #5）
2. 移除 `FindDMBetween` 函数
3. 移除/重写相关的测试（3 个测试）

---

## 10. `UnreadCount` 函数（db/messages.go:266-286）

### 定义

```go
// Deprecated.
func (d *DB) UnreadCount(ctx context.Context, chatID, lastReadID string) (int, error) {
    SELECT COUNT(*) FROM messages WHERE chat_id = ? AND deleted_at IS NULL
      AND (created_at, id) > (SELECT created_at, id FROM messages WHERE id = ?)
}
```

### 引用者

1. **唯一生产者：** `db/chats.go:349`（`ListUserChats` 内部）
2. **测试：** `db/messages_test.go:210`（`TestUnreadCount`）、`db/db_test.go:449`（`TestUnreadCount_NonexistentLastRead`）

### 调用链

```
ListUserChats  →  UnreadCount(ctx, chatID, lastReadID)  →  返回 int
                     ↓
                  设置到 Chat.UnreadCount
                     ↓
                  JSON 响应 → 前端
```

### 为什么需要它？

这是服务器端未读计数的唯一来源。没有它，`Chat.UnreadCount` 字段保持为零，前端不会显示未读徽章。前端不能自我维护未读计数，因为它在页面加载时不了解服务器端状态。

### 能否移除？

**仅在移除 `Chat.UnreadCount` 时可以（参见 #1）。** 它们是配对的——函数提供数据，模型承载它。

### 弃用状态：**错误标记。** 这是一个关键函数，没有替代方案。

### 移除前需要做什么

引入服务器端的替代跟踪系统。也许：
- 聊天成员表中的 `unread_count` 列，在消息创建时更新
- 或基于 `last_read_message_id` 的计数比较（当前实现的更精简版本）

---

## 11. `UploadDir` / `MaxUploadBytes` 配置

### 定义

`config/config.go:17,22`：
```go
UploadDir      string  // Deprecated
MaxUploadBytes int64   // Deprecated
```

### 使用者

- **`router.go:130`：** `serveUpload` 使用 `UploadDir` 提供静态文件
- **`uploads.go:59,60,62,71,89,95`：** `Upload` 处理器广泛使用两者
- **`cmd/chatd/main.go:46`：** `os.MkdirAll(cfg.UploadDir, ...)` 在启动时
- **`testutil/testutil.go:34,38`：** 测试夹具设置两者
- **`testutil/handler_test.go:651`：** 注释说"MaxUploadBytes is 5MB in test config"

### 调用链

```
Config.Load() → 设置 UploadDir, MaxUploadBytes
     ↓
main.go → os.MkdirAll(UploadDir)
     ↓
Upload handler → r.Body = MaxBytesReader(w, r.Body, MaxUploadBytes)
                → os.MkdirAll(UploadDir)
                → 将文件写入 UploadDir
     ↓
serveUpload → 从 UploadDir 提供文件
```

### 能否移除？

**仅在移除 Upload 处理器和 serveUpload 时可以（参见 #6）。** 它们是同一上传架构的一部分。测试也使用它们。

### 弃用状态：**正确弃用。** 前端 100% 使用外部的 `upload.moonchan.xyz`。

### 移除前需要做什么

1. 从 `config.go` 中移除字段
2. 移除 `main.go` 中的 `MkdirAll`
3. 移除 `router.go` 中的 `serveUpload`
4. 移除 `uploads.go` 中的 `Upload`
5. 更新 `testutil/testutil.go` 以移除或忽略它们

---

## 12. `mockCreateDM`（client/src/api/mock.js:159-181）

### 定义

```js
// @deprecated DMs are now handled via createChat with type='dm'
export function mockCreateDM(_token, userId) { ... }
```

### 注册位置

`client.js:207`：`['createDM', mockCreateDM]`——在 `MOCKABLE` 数组中注册，允许 `api.enableMock()` 替换 `api.createDM`。

### 被 UI 调用？

**不。** `api.createDM` 在任何 React 组件中都没有被调用。（搜索 `createDM` 仅在 `client.js` 定义和 `MOCKABLE` 注册中出现。）

### 调用链

```
[无前端调用]
     ↓
仅：api.enableMock() → 将 mockCreateDM 交换到 api.createDM
     ↓
[api.createDM 实际从未被调用]
```

### 能否移除？

**可以。** 它是死的模拟代码。

### 弃用状态：**正确弃用且已死。**

### 移除前需要做什么

1. 从 `mock.js` 中移除函数
2. 从 `client.js` 中的 `MOCKABLE` 数组中移除 `['createDM', mockCreateDM]`
3. 从 `client.js` 中移除 `mockCreateDM` 导入
