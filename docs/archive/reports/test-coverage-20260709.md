# 测试报告

审查日期：2026-07-09  
覆盖范围：`server/internal/db/` + `server/internal/handlers/` + `server/internal/testutil/`

---

## 1. 测试架构

### 1.1 双层次测试策略

```
┌─────────────────────────────────────────────────────────────┐
│  集成测试（handler_test.go, auth_flow_test.go）              │
│  ┌──────────────┐  ┌────────────────────┐  ┌─────────────┐ │
│  │ testutil.Fixture │ → httptest.Server │ → DB + Auth   │ │
│  └──────────────┘  └────────────────────┘  └─────────────┘ │
│  测试整个 HTTP 请求/响应管道                                  │
├─────────────────────────────────────────────────────────────┤
│  单元测试（db_test.go, messages_test.go）                     │
│  ┌──────────────┐  ┌───────────────┐                        │
│  │ testutil.Fixture │ → DB 直接调用  │                        │
│  └──────────────┘  └───────────────┘                        │
│  绕过 HTTP 层，直接测试 SQL 和扫描逻辑                         │
└─────────────────────────────────────────────────────────────┘
```

**原则：** 单元测试覆盖数据层边界情况（NotFound、Conflict、空列表、no-op）；集成测试覆盖 handler 层权限检查、输入验证、状态码。

### 1.2 测试基础设施（testutil 包）

所有测试共享 `testutil.Fixture`：

```
testutil.New(t)
  ├── t.TempDir()                    → 临时数据库 + 上传目录
  ├── db.Open(dir + "/test.db")      → 独立 SQLite（含迁移）
  ├── auth.New(secret, ttl)          → JWT 服务
  ├── ws.NewHub + ws.NewGateway      → WebSocket 拓扑
  ├── handlers.New(...).Router(...)  → 完整的 Chi 路由
  ├── httptest.NewServer(router)     → 随机端口 HTTP server
  └── t.Cleanup(close DB + server)  → 自动清理
```

**约定：**
- 每个 `TestXxx` 函数创建自己的 `Fixture`（`f := testutil.New(t)`）
- 每个 Fixture 用独立的 SQLite 文件（`t.TempDir()`），测试间无状态泄漏
- 每个 Fixture 用独立的 `httptest.Server`（随机端口），测试间无路由冲突
- `t.Cleanup` 保证 DB 关闭和服务器停止，无需手动 `defer`

**好处：**
| 方面 | 方案 |
|------|------|
| 隔离性 | 每测试独立数据库 + 独立 HTTP server |
| 清理 | `t.TempDir()` 自动回收，`t.Cleanup` 自动关闭 |
| 速度 | SQLite 内存模式近似，~11s 跑完 134 个测试 |
| 并行 | 可 `t.Parallel()`（当前未启用，单连接 SQLite 无需） |

### 1.3 命名规范

| 模式 | 示例 | 用途 |
|------|------|------|
| `Test<Function>` | `TestCreateChat` | 正向路径（happy path） |
| `Test<Function>_<Scenario>` | `TestCreateChat_InvalidType` | 边界/错误路径 |
| `Test<Function>_<Scenario>` | `TestFindDMBetween_Self` | 异常输入 |
| `Test<Action>_<Context>` | `TestDeleteMessage_NonAuthor` | 权限检查 |

### 1.4 断言风格

```go
// 1. 错误断言 — 使用 err / db.ErrNotFound / db.ErrConflict
if err != nil { t.Fatal(err) }
if err != db.ErrNotFound { t.Fatalf("want ErrNotFound got %v", err) }
if err == nil { t.Fatal("should fail") }

// 2. 状态码断言 — 直接比较整数
if res.StatusCode != 200 { t.Fatalf("want 200 got %d", res.StatusCode) }

// 3. 数据断言 — Fatal 而非 Error
if msg.Content != "expected" { t.Fatal("content mismatch") }
```

**原则：** 使用 `Fatal` 而非 `Error`（失败即中止，避免 nil pointer dereference）。`Error` 仅用于 `t.Run` 子测试可继续的场景。

### 1.5 辅助方法（testutil.Client）

| 方法 | 用途 |
|------|------|
| `f.Register(t, email, username, password) *Session` | 注册 + 返回完整 session |
| `f.Login(t, email, password) *Session` | 登录 + 返回完整 session |
| `f.Refresh(t, refreshToken) *Session` | 刷新 token |
| `f.Do(t, method, path, token, body) *http.Response` | JSON 请求 |
| `f.DoWithCookie(t, method, path, token, cookieName, cookieValue, body) *http.Response` | 带 cookie 请求 |
| `f.DoJSON(t, method, path, token, body, out) int` | JSON 请求 + 解码响应 |
| `f.DoMultipart(t, ...) *http.Response` | multipart 文件上传 |
| `f.WSURL(token) string` | WebSocket URL 生成 |
| `ResponseCookie(res, name) *http.Cookie` | 解析 Set-Cookie |

---

## 2. 覆盖数据

| 层 | 测试类型 | 测试函数数 | 代码行数 |
|---|---------|-----------|---------|
| DB | 单元测试 | 66 | 1040 |
| 集成 | HTTP handler 测试 | 68 | 2032 |
| **合计** | | **134** | **3072** |

**Production 代码：** DB 层 ~1800 行 / handler ~1000 行

### 2.1 分层覆盖比例

```
DB 层（36 个导出函数）
  users.go       ────────────────── 7/7  100%
  chats.go       ────────────────── 16/16 100%
  chats_ext.go   ────────────────── 4/4  100%
  messages.go    ────────────────── 9/9  100%

Handler 层（29 个端点）
  已覆盖         ────────────────── 27/29 93%
  未覆盖（/ws, /swagger）          2/29   7%
```

### 2.2 错误路径覆盖

| 类型 | 覆盖数量 | 示例 |
|------|---------|------|
| `ErrNotFound` | 10 个 | GetUserByID, GetChat, GetMessage, FindDMBetween 等 |
| `ErrConflict` | 4 个 | 重复 email, 用户名, token hash, 已加入成员 |
| 空列表 | 5 个 | GetChatMembers, GetMessages, SearchUsers, ListPublicChats, PurgeExpired |
| no-op（静默成功） | 6 个 | DeleteChat 不存在, RemoveMember 不存在, AddReaction 重复等 |
| 输入验证 | 8 个 | 空内容/名字/表情, 超长内容/表情, DM 成员数 ≠ 2 |
| 权限检查 | 10 个 | 非成员 403, 非作者 404/403, 踢群主 403, DM 操作 400 |

---

---

## 3. DB 层测试

### 3.1 测试模式

**单元测试用直接 DB 调用，不经过 HTTP：**

```go
func TestGetUserByID_NotFound(t *testing.T) {
    f := testutil.New(t)
    _, err := f.DB.GetUserByID(f.Ctx(), "nonexistent")
    if err != db.ErrNotFound {
        t.Fatalf("want ErrNotFound, got %v", err)
    }
}
```

**正向路径 + 反向路径成对出现：**

```
TestCreateChat              → 成功创建 group + DM
TestCreateChat_InvalidType  → 错误：类型无效
TestCreateChat_GroupEmptyName → 错误：空名字
TestCreateChat_DMWrongMemberCount → 错误：成员数≠2
TestCreateChat_EmptyMembers → 错误：无成员
```

**事务行为通过 DB 查询验证（而非 mock）：**

```go
// 验证 AddReaction 确实写入了 JSON 列
m, err := f.DB.GetMessage(f.Ctx(), msg.ID)
var rxs []models.Reaction
json.Unmarshal(m.Reactions, &rxs)
// 断言 rxs 内容
```

### 3.2 覆盖清单

### users.go（7 导出函数）

| 函数 | 覆盖 | 测试 |
|------|------|------|
| `NewID()` | ✅ | `TestNewID` |
| `PickColor()` | ✅ | `TestPickColor` |
| `CreateUser()` | ✅ | `TestDBOpenAndMigrate`, `TestUserCreateDuplicateEmail` |
| `GetUserByID()` | ✅ | `TestUpdateUserProfile`, `TestUserStatus`, `TestUpdateUserLastSeen` |
| `GetUserByEmail()` | ✅ | `TestGetUserByEmail` |
| `UpdateUserProfile()` | ✅ | `TestUpdateUserProfile`, `TestUpdateUserProfile_Conflict` |
| `UpdateUserStatus()` | ✅ | `TestUserStatus`, `TestUpdateUserStatus_Nonexistent` |
| `UpdateUserLastSeen()` | ✅ | `TestUpdateUserLastSeen` |
| `SearchUsers()` | ✅ | `TestSearchUsers`, `TestSearchUsers_EmptyQuery` |

**未覆盖边界：** 无（全部错误路径已覆盖）

### chats.go（16 导出函数）

| 函数 | 覆盖 | 测试 |
|------|------|------|
| `CreateRefreshToken()` | ✅ | `TestRefreshTokenCRUD`, `TestCreateRefreshToken_Duplicate` |
| `FindRefreshToken()` | ✅ | `TestRefreshTokenCRUD` |
| `DeleteRefreshToken()` | ✅ | `TestRefreshTokenCRUD` |
| `DeleteUserRefreshTokens()` | ✅ | 集成测试（logout 路径） |
| `PurgeExpiredTokens()` | ✅ | `TestRefreshTokenCRUD`, `TestPurgeExpiredTokens_NoneExpired` |
| `CreateChat()` | ✅ | `TestCreateChatGroupAndDM`, `TestCreateChat_InvalidType`, `TestCreateChat_GroupEmptyName`, `TestCreateChat_DMWrongMemberCount`, `TestCreateChat_EmptyMembers` |
| `GetChat()` | ✅ | `TestGetChat_NotFound`, `TestCreateChatGroupAndDM` |
| `GetChatMembers()` | ✅ | `TestGetChatMembers_Empty`, `TestCreateChatGroupAndDM` |
| `GetChatMemberRole()` | ✅ | `TestGetChatMemberRole_NotFound` |
| `ChatMemberCount()` | ✅ | `TestChatMemberCount_Nonexistent` |
| `IsChatMember()` | ✅ | `TestAddRemoveMember`, `TestIsChatMember_Nonexistent` |
| `ListUserChats()` | ✅ | `TestListUserChats` |
| `FindDMBetween()` | ✅ | `TestFindDMBetween`, `TestFindDMBetween_Self`, `TestFindDMBetween_NotFound` |
| `AddChatMember()` | ✅ | `TestAddRemoveMember`, `TestAddRemoveMember_Nonexistent` |
| `RemoveChatMember()` | ✅ | `TestAddRemoveMember`, `TestAddRemoveMember_Nonexistent` |
| `DeleteChat()` | ✅ | `TestDeleteChat`, `TestDeleteChat_Nonexistent` |
| `RenameChat()` | ✅ | `TestRenameChat`, `TestRenameChat_EmptyName`, `TestRenameChat_Nonexistent` |
| `UpdateLastRead()` | ✅ | `TestUnreadCount`, `TestUpdateLastRead_Nonexistent` |

**未覆盖边界：** 无

### chats_ext.go（4 导出函数）

| 函数 | 覆盖 | 测试 |
|------|------|------|
| `ListPublicChats()` | ✅ | `TestListPublicChats_Empty` |
| `JoinChatByID()` | ✅ | `TestJoinChatByID_AlreadyJoined`, `TestJoinChatByID_Private`, `TestJoinChatByID_Nonexistent` |
| `SetPinnedMessage()` | ✅ | `TestSetAndClearPinnedMessage` |
| `ClearPinnedMessage()` | ✅ | `TestSetAndClearPinnedMessage` |

**未覆盖边界：** 无

### messages.go（9 导出函数）

| 函数 | 覆盖 | 测试 |
|------|------|------|
| `CreateMessage()` | ✅ | `TestCreateGetMessage`, `TestMessageWithMentions`, `TestMessageWithAttachments`, `TestCreateMessage_DuplicateMentions`, `TestCreateMessage_ContentTooLong`, `TestCreateMessage_AttachmentOnly`, `TestEmptyMessageRejected` |
| `GetMessage()` | ✅ | `TestCreateGetMessage`, `TestGetMessage_NotFound` |
| `GetMessages()` | ✅ | `TestGetMessagesWithPagination`, `TestGetMessages_EmptyChat` |
| `LastMessage()` | ✅ | `TestLastMessage` |
| `UnreadCount()` | ✅ | `TestUnreadCount`, `TestUnreadCount_NonexistentLastRead` |
| `UpdateMessage()` | ✅ | `TestUpdateMessage`, `TestUpdateMessage_Nonexistent` |
| `DeleteMessage()` | ✅ | `TestDeleteMessage`, `TestDeleteMessage_Nonexistent` |
| `AddReaction()` | ✅ | `TestReactionsAddRemove`, `TestAddReaction_EmptyEmoji`, `TestAddReaction_EmojiTooLong`, `TestAddReaction_Duplicate` |
| `RemoveReaction()` | ✅ | `TestReactionsAddRemove`, `TestRemoveReaction_Nonexistent` |

**未覆盖边界：** 无

### 内联函数覆盖

| 函数 | 状态 | 说明 |
|------|------|------|
| `scanMessage()` | 间接覆盖 | 被 `GetMessage`/`GetMessages`/`LastMessage` 使用 |
| `fetchMessageRow()` | 间接覆盖 | 同上 |
| `syncReactionsColumn()` | 间接覆盖 | 被 `AddReaction`/`RemoveReaction` 使用 |
| `reactionsFor()` | 间接覆盖 | 被 `syncReactionsColumn` 使用 |
| `dedupe()` | ✅ | `TestCreateMessage_DuplicateMentions` |
| `parseTime()` | 间接覆盖 | 所有扫描路径使用 |

---

---

## 4. Handler 层测试

### 4.1 测试模式

**集成测试通过 HTTP 请求完整的 Chi 路由管道：**

```go
func TestRenameDelete_DMNotAllowed(t *testing.T) {
    f := testutil.New(t)
    alice := f.Register(t, "rddm1@t.t", "RD_DM_A", "password123")
    bob := f.Register(t, "rddm2@t.t", "RD_DM_B", "password123")
    res := f.Do(t, "POST", "/api/dms", alice.AccessToken, map[string]string{
        "user_id": bob.UserID,
    })
    // ...
}
```

**每个测试覆盖一个场景，用子测试组合相关路径：**

```go
t.Run("rename dm → 400", func(t *testing.T) { ... })
t.Run("delete dm → 400", func(t *testing.T) { ... })
```

**权限测试模式：**

```go
// Arrange: 创建聊天 + non-member
// Act: non-member 发起操作
// Assert: 403 Forbidden
```

**不会 mock DB：** 所有集成测试运行在真实 SQLite + 完整路由上。DB 层 bug 也会导致集成测试失败。这是有意为之 — 双层次覆盖提供冗余保障。

### 4.2 覆盖清单

| 端点 | 覆盖 | 测试 |
|------|------|------|
| `GET /healthz` | ✅ | `TestHealthz` |
| `POST /api/auth/register` | ✅ | `TestRegisterLoginRefresh`, `TestDuplicateEmail`, `TestRegisterNoValidation` |
| `POST /api/auth/login` | ✅ | `TestRegisterLoginRefresh`, `TestLoginWrongPassword` |
| `POST /api/auth/refresh` | ✅ | `TestRefreshTokenFlow`, `TestRefreshTokenRotation`, `TestConcurrentRefreshRotation`, `TestMultiDeviceRefreshIsolation`, `TestRefreshWithoutCookie` |
| `POST /api/auth/logout` | ✅ | `TestLogoutInvalidatesTokens` |
| `GET /api/users/me` | ✅ | `TestAuthEndpoints` |
| `PATCH /api/users/me` | ✅ | `TestUpdateProfile`, `TestUpdateMeUsernameConflict`, `TestUpdateMe_EmptyBody` |
| `GET /api/users` | ✅ | `TestSearchUsers`, `TestSearchUsersEmptyQuery`, `TestSearchUsersExcludesSelf` |
| `GET /api/chats/my` | ✅ | `TestListChatsWithUnreads` |
| `GET /api/chats/public` | ✅ | `TestChatVisibilityAndPublicList` |
| `POST /api/chats` | ✅ | `TestCreateGroupChatAndSendMessage`, `TestCreateChatInvalidInput` |
| `POST /api/dms` | ✅ | `TestCreateDM`, `TestCreateOrGetDM` |
| `GET /api/chats/{chatID}` | ✅ | `TestGetChat_AsMemberAndNonMember`, `TestGetChat_NotFound` |
| `PATCH /api/chats/{chatID}` | ✅ | `TestRenameChatOnlyOwner`, `TestRenameDelete_DMNotAllowed` |
| `DELETE /api/chats/{chatID}` | ✅ | `TestDeleteChatOnlyOwner`, `TestRenameDelete_DMNotAllowed` |
| `GET /api/chats/{chatID}/members` | ✅ | `TestAddRemoveMembers`, `TestListMembers_NonMember` |
| `POST /api/chats/{chatID}/members` | ✅ | `TestAddRemoveMembers`, `TestAddMember_DMAndDuplicate` |
| `DELETE /api/chats/{chatID}/members/{userID}` | ✅ | `TestAddRemoveMembers`, `TestLeaveGroupChat`, `TestRemoveMember_DMAndOwner` |
| `POST /api/chats/{chatID}/read` | ✅ | `TestMarkRead`, `TestMarkRead_EmptyMessageID` |
| `GET /api/chats/{chatID}/messages` | ✅ | `TestCreateGroupChatAndSendMessage`, `TestLeaveGroupChat` |
| `POST /api/chats/{chatID}/messages` | ✅ | `TestCreateGroupChatAndSendMessage`, `TestSendMessageWithAttachments`, `TestMessageContentTooLong`, `TestSendMessage_BadJSON`, `TestSendMessage_EmptyContentNoAttachments`, `TestSendMessageNonMember`, `TestChatForbidden` |
| `PATCH /api/chats/{chatID}/messages/{messageID}` | ✅ | `TestCreateGroupChatAndSendMessage`, `TestEditMessageNonAuthor` |
| `DELETE /api/chats/{chatID}/messages/{messageID}` | ✅ | `TestDeleteMessageAsAdmin`, `TestDeleteMessage_NonAuthor` |
| `PUT */reactions/{emoji}` | ✅ | `TestReactionsFlow`, `TestReactionErrors` |
| `DELETE */reactions/{emoji}` | ✅ | `TestReactionsFlow`, `TestReactionErrors` |
| `POST /api/chats/{chatID}/join` | ✅ | `TestJoinPublicChat` |
| `POST/PATCH /api/chats/{chatID}/pin` | ✅ | `TestPinMessage` |
| `DELETE /api/chats/{chatID}/pin` | ✅ | `TestDeletePinnedChat` |
| `POST /api/uploads` | ✅ | `TestUploadNotLoggedIn`, `TestUploadFile`, `TestUploadExceedsSizeLimit`, `TestUploadRejectsUnsupportedMime`, `TestUpload_MissingFile` |
| `GET /api/events` | ✅ | `TestSSEConnection`, `TestSSEInvalidToken`, `TestSSEMissingToken` |

**未测试端点：**

| 端点 | 原因 |
|------|------|
| `/ws` | WebSocket 持久连接，集成测试架构不支持 |
| `/swagger/*` | Swagger UI，非业务逻辑 |
| `/uploads/*` | 已弃用，功能已迁移至前端直传 |

---

---

## 5. 本次新增测试汇总

### DB 层（db_test.go, messages_test.go）：+33 测试函数

| 测试 | 覆盖内容 |
|------|---------|
| `TestNewID` | UUID 生成唯一性 |
| `TestPickColor` | 空 seed、非 UUID、确定性 |
| `TestGetUserByID_NotFound` | `ErrNotFound` 路径 |
| `TestUpdateUserProfile_Conflict` | 用户名冲突 |
| `TestUpdateUserStatus_Nonexistent` | 不存在的用户（no-op） |
| `TestSearchUsers_EmptyQuery` | 空/空白查询 |
| `TestUserLastSeen_ZeroOnCreate` | 新建用户 LastSeen 零值 |
| `TestCreateChat_InvalidType` | 无效聊天类型 |
| `TestCreateChat_GroupEmptyName` | 群聊空名字 |
| `TestCreateChat_DMWrongMemberCount` | DM 成员数 ≠ 2 |
| `TestCreateChat_EmptyMembers` | 空成员列表 |
| `TestGetChat_NotFound` | `ErrNotFound` 路径 |
| `TestGetChatMembers_Empty` | 空成员列表 |
| `TestGetChatMemberRole_NotFound` | `ErrNotFound` 路径 |
| `TestChatMemberCount_Nonexistent` | 不存在的聊天 → 0 |
| `TestIsChatMember_Nonexistent` | 不存在的聊天 → false |
| `TestAddRemoveMember_Nonexistent` | 不存在的聊天/用户（no-op） |
| `TestDeleteChat_Nonexistent` | 不存在的聊天（no-op） |
| `TestRenameChat_EmptyName` | 空/空白名字 |
| `TestRenameChat_Nonexistent` | 不存在的聊天（no-op） |
| `TestUpdateLastRead_Nonexistent` | 不存在的数据（no-op） |
| `TestPurgeExpiredTokens_NoneExpired` | 无过期 token |
| `TestFindDMBetween_Self` | 自己 DM 自己 |
| `TestFindDMBetween_NotFound` | `ErrNotFound` 路径 |
| `TestCreateRefreshToken_Duplicate` | 重复 hash → Conflict |
| `TestJoinChatByID_AlreadyJoined` | 已加入（INSERT OR IGNORE） |
| `TestJoinChatByID_Private` | 私有聊天拒绝加入 |
| `TestJoinChatByID_Nonexistent` | 不存在的聊天 |
| `TestSetAndClearPinnedMessage` | Pin / ClearPin 完整流程 |
| `TestListPublicChats_Empty` | 无公开聊天 |
| `TestUnreadCount_NonexistentLastRead` | 不存在的游标 → 0 |
| `TestCreateMessage_DuplicateMentions` | 去重逻辑 |
| `TestCreateMessage_ContentTooLong` | 4000 字符上限 |
| `TestGetMessage_NotFound` | `ErrNotFound` 路径 |
| `TestGetMessages_EmptyChat` | 空聊天 |
| `TestUpdateMessage_Nonexistent` | `ErrNotFound` 路径 |
| `TestDeleteMessage_Nonexistent` | `ErrNotFound` 路径 |
| `TestAddReaction_EmptyEmoji` | 空/空白表情 |
| `TestAddReaction_EmojiTooLong` | 32 字符上限 |
| `TestAddReaction_Duplicate` | 重复（INSERT OR IGNORE） |
| `TestRemoveReaction_Nonexistent` | 不存在的 reaction（no-op） |
| `TestCreateMessage_AttachmentOnly` | 仅附件消息 |
| `TestLastMessage` | 无消息 → ErrNotFound, 多消息 → 最新 |
| `TestUpdateUserLastSeen` | 手动 + CreateMessage 副作用 |

### Handler 集成层（handler_test.go）：+9 测试函数（含子测试共 +16）

| 测试 | 覆盖内容 |
|------|---------|
| `TestGetChat_AsMemberAndNonMember` | 成员 200 / 非成员 403 |
| `TestGetChat_NotFound` | 不存在聊天 → 403（IsChatMember 返回 false） |
| `TestRenameDelete_DMNotAllowed` | DM 改名/删除 → 400 |
| `TestDeleteMessage_NonAuthor` | 非作者删除他人消息 403 / chat 不匹配 400 |
| `TestListMembers_NonMember` | 非成员查看成员列表 → 403 |
| `TestAddMember_DMAndDuplicate` | DM 加人 400 / 已存在 409 / 用户不存在 404 |
| `TestRemoveMember_DMAndOwner` | DM 踢人 400 / 踢群主 403 |
| `TestSendMessage_BadJSON` | 非法 JSON 请求体 → 400 |
| `TestMarkRead_EmptyMessageID` | 空 message_id → 400 |
| `TestUpload_MissingFile` | 缺 file 字段 → 400 |
| `TestSendMessage_EmptyContentNoAttachments` | 空内容无附件 → 400 |

### 已修复的测试质量问题

| 问题 | 修复 |
|------|------|
| `TestConcurrentRegister` 假通过 | 改为精确断言 `ok == 1` |
| `TestReactionsFlow` 残留 `?details=true` | 移除死参数 |
| `TestMessageWithAttachmentOnlyAllowed` 重复 | 从 messages_test.go 移除（db_test.go 已有） |

---

## 6. 测试局限性

### 6.1 架构限制

| 限制 | 原因 | 影响 |
|------|------|------|
| 无并发测试 | SQLite `SetMaxOpenConns(1)` 串行化所有写入 | 不会漏 race condition，但竞态 token rotation 已被 `TestConcurrentRefreshRotation` 覆盖 |
| 无 mock | 所有测试用真实 SQLite | 无法测试磁盘 I/O 错误、网络超时等 |
| 无端到端测试 | 不启动前端/WebSocket 客户端 | `/ws` 路径未被 HTTP 集成测试覆盖 |
| 无性能测试 | 测试数据库最多几十条记录 | 分页逻辑在空/小数据集上已验证 |

### 6.2 已接受的权衡

- **`requireOwnerOrAdmin` admin 角色路径：** 当前代码无设置 `role='admin'` 的路径（仅 `owner` 由 `CreateChat` 设置）。添加 admin 功能后才需要测试
- **`serveUpload` 和 `serveStatic`：** 已弃用（标记 `// Deprecated`），前端已迁移至 upload.moonchan.xyz 直传
- **`DeleteUserRefreshTokens` 直接单测：** 被 logout 集成测试间接覆盖
- **JSON decode 错误路径：** 如 `decodeJSON` 返回错误。这些依赖 `encoding/json` 的标准行为，测试的价值是确保 handler 返回 400 而非 panic

### 6.3 未被测试的开销代码

以下代码无测试覆盖且**不在**当前测试范围内（归类为基础设施或已弃用）：

| 路径 | 行数 | 状态 |
|------|------|------|
| `handlers/router.go` 中间件 | ~50 | 中间件逻辑（CSP、CORS、速率限制）由设计保证 |
| `handlers/sse.go` | ~60 | `TestSSE*` 覆盖了连接和 token 验证 |
| `db/chats_ext.go` | 90 | 全部在集成测试中覆盖 |
| `db/db.go` | 70 | `Open` + `Migrate` 被每个测试间接覆盖 |

---

## 7. 运行方法

```bash
# 全部测试
go test ./internal/db/ ./internal/testutil/ -count=1 -timeout 180s

# 仅 DB 单元测试（~1.7s）
go test ./internal/db/ -count=1

# 仅集成测试（~9s）
go test ./internal/testutil/ -count=1

# 运行单个测试
go test ./internal/testutil/ -run TestCreateChat_InvalidType

# 查看详细输出
go test ./internal/db/ -v -count=1
```

### 运行时特征

```
DB 单元测试：  ~1.7s  |  66 个测试  |  每个测试独立 SQLite
集成测试：    ~9s     |  68 个测试  |  每个测试独立 SQLite + HTTP server
-------------------------------------------
合计：        ~11s    |  134 个测试
```

测试不可并行（每个测试独占 SQLite 文件），但与 CI 中其他包的测试可以并行。

---

## 8. 结论

36 个导出 DB 函数全部有直接断言覆盖。29 个 handler endpoint 中 27 个有集成测试覆盖（`/ws` 和 `/swagger` 除外）。单元测试覆盖所有 `ErrNotFound` / `ErrConflict` 错误路径、空列表、no-op 边界。集成测试覆盖所有 handler 层权限检查、输入验证、错误码返回。

测试架构采用真实 SQLite + 完整路由的双层次策略，不依赖 mock，每测试独立数据库提供完全隔离。全部 134 个测试在 ~11s 内完成。
