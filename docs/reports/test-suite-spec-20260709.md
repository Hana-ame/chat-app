# Test Suite 规范

## db_test — DAO 层测试

**文件:** `server/internal/db/db_test.go`（221 行）

---

### TestDBOpenAndMigrate

**目的:** 验证 DB 打开和迁移成功，用户创建基本正确。

**依赖链:** `testutil.New → db.Open → d.Migrate → DB.CreateUser`

**条件分支:**
- `f.DB == nil` → `Fatal("DB is nil")`
- `u.ID == "" || u.Username != "test-user" || u.AvatarColor == ""` → `Fatal("user creation incomplete")`

---

### TestUserCreateDuplicateEmail

**目的:** 验证重复 email 返回 `db.ErrConflict`。

**依赖链:** `DB.CreateUser → DB.CreateUser`

**条件分支:**
- 第二次 `CreateUser` 错误不是 `db.ErrConflict` → `Fatalf`

---

### TestGetUserByEmail

**目的:** 验证通过 email 查用户和 hash，不存在时返回 `ErrNotFound`。

**依赖链:** `DB.CreateUser → DB.GetUserByEmail → DB.GetUserByEmail`

**条件分支:**
- `u.Username != "getme" || hash != "hash12345678"` → `Fatal("wrong user")`
- `GetUserByEmail("nope@x.com")` 没有 `ErrNotFound` → `Fatal("want not found")`

---

### TestSearchUsers

**目的:** 验证按 username 模糊搜索和完整 UUID 搜索，结果数量正确。

**依赖链:** `DB.CreateUser x3 → DB.SearchUsers → DB.SearchUsers`

**条件分支:**
- `len(users) != 1 || users[0].Username != "Alpha"` → `Fatalf`
- `len(users) < 2` → `Fatalf`

---

### TestCreateChatGroupAndDM

**目的:** 验证创建群聊和 DM，检查 metadata 和 member count。

**依赖链:** `DB.CreateUser x3 → DB.CreateChat → DB.GetChatMembers → DB.CreateChat`

**条件分支:**
- `chat.Name != "TestGroup" || chat.Type != "group" || chat.OwnerID != a.ID` → `Fatal`
- `chat.MemberCount != 3` → `Fatalf`
- `dm.Type != "dm" || dm.Name != ""` → `Fatal`
- `dm.OwnerID != ""` → `Fatal("DM should have no owner")`

---

### TestFindDMBetween

**目的:** 验证 DM 查找、不存在的用户返回 ErrNotFound、自 DM 报错。

**依赖链:** `DB.CreateUser x2 → DB.CreateChat → DB.FindDMBetween x3`

**条件分支:**
- `dm == nil || dm.Type != "dm"` → `Fatal`
- `FindDMBetween(a.ID, "nonexistent")` 不是 `ErrNotFound` → `Fatal`
- `FindDMBetween(a.ID, a.ID)` 没有报错 → `Fatal`

---

### TestListUserChats

**目的:** 验证用户聊天列表返回正确的数量和类型。

**依赖链:** `DB.CreateUser x2 → DB.CreateChat x2 → DB.ListUserChats`

**条件分支:**
- `len(chats) != 2` → `Fatalf`
- 缺少 group 或 dm → `Fatal`

---

### TestAddRemoveMember

**目的:** 验证添加/移除成员、重复添加返回 ErrConflict。

**依赖链:** `DB.CreateUser x3 → DB.CreateChat → DB.AddChatMember → DB.IsChatMember → DB.AddChatMember → DB.RemoveChatMember → DB.IsChatMember`

**条件分支:**
- `AddChatMember` 失败 → `Fatal`
- `!IsChatMember` → `Fatal("should be member")`
- `AddChatMember` 重复不是 `ErrConflict` → `Fatal`
- `RemoveChatMember` 失败 → `Fatal`
- `IsChatMember` 仍为 true → `Fatal("should be removed")`

---

### TestDeleteChat

**目的:** 验证删除聊天后 `GetChat` 返回 `ErrNotFound`。

**依赖链:** `DB.CreateUser → DB.CreateChat → DB.DeleteChat → DB.GetChat`

**条件分支:**
- `DeleteChat` 失败 → `Fatal`
- `GetChat` 不是 `ErrNotFound` → `Fatal("should be gone")`

---

### TestRenameChat

**目的:** 验证重命名聊天成功。

**依赖链:** `DB.CreateUser → DB.CreateChat → DB.RenameChat → DB.GetChat`

**条件分支:**
- `RenameChat` 失败 → `Fatal`
- `updated.Name != "NewName"` → `Fatal`

---

## handler_test — HTTP Handler 层测试

**文件:** `server/internal/testutil/handler_test.go`（800 行）

---

### TestHealthz

```go
func TestHealthz(t *testing.T) {
	f := testutil.New(t)
	res := f.Do(t, "GET", "/healthz", "", nil)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("healthz: %d", res.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body["status"])
	}
	echo, ok := body["echo"].(map[string]any)
	if !ok {
		t.Fatalf("expected echo object, got %T", body["echo"])
	}
	if len(echo) == 0 {
		t.Fatalf("expected non-empty echo object")
	}
}
```

**目的:** 健康检查返回 200 + `status: ok` + 非空 echo headers。

**依赖链:** `testutil.New → Do(GET /healthz) → json.Decode`

**条件分支:**
- `StatusCode != 200` → `Fatalf`
- `body["status"] != "ok"` → `Fatalf`
- `echo` 不是 `map[string]any` → `Fatalf`
- `echo` 为空 → `Fatalf`

---

### TestCreateGroupChatAndSendMessage

**目的:** 创建群聊、发送消息、验证消息内容、成员列表、已读状态。

**依赖链:** `Register x2 → Do(POST /api/chats) → Do(POST /chats/{id}/messages) → Do(GET /chats/{id}/messages) → Do(GET /chats/{id}/members) → Do(POST /chats/{id}/read)`

---

### TestCreateDM

**目的:** 验证创建 DM 成功，类型为 dm。

**依赖链:** `Register x2 → Do(POST /api/dms)`

**条件分支:**
- 状态码 != 200/201 → `Fatalf`
- 类型不是 dm → `Fatal`

---

### TestAddRemoveMembers

**目的:** 验证添加成员、列表包含新成员、移除成员、非成员不可加人、owner 不可被踢。

**依赖链:** `Register x3 → Do(POST /api/chats) → Do(POST /members) → Do(GET /members) → Do(DELETE /members/{id}) → Do(DELETE /members/{id})`

**条件分支:**
- 加人失败或状态码不对 → `Fatalf`
- 移除后成员列表仍包含目标 → `Fatalf`
- 非成员加人返回 403 → `Fatalf`
- 踢 owner 返回 403 → `Fatalf`

---

### TestReactionsFlow

**目的:** 验证添加/移除 emoji 反应。

**依赖链:** `Register x2 → Do(POST /api/chats) → Do(POST /messages) → Do(PUT /reactions/{emoji}) → Do(DELETE /reactions/{emoji})`

---

### TestUpdateProfile

**目的:** 验证更新用户名、头像颜色、头像 URL。

**依赖链:** `Register → Do(PATCH /api/users/me)`

**条件分支:**
- 状态码 != 200 → `Fatalf`
- 更新后的值不匹配 → `Fatalf`

---

### TestSearchUsers

**目的:** 验证用户搜索返回正确结果且排除自身。

**依赖链:** `Register → Do(GET /api/users?q=xxx)`

---

### TestDeleteMessageAsAdmin

**目的:** 验证 owner 可删除他人的消息。

**依赖链:** `Register x2 → Do(POST /api/chats) → Do(POST /messages) → Do(DELETE /messages/{id})`

---

### TestLeaveGroupChat

**目的:** 验证成员可退出群聊（踢自己）。

**依赖链:** `Register x3 → Do(POST /api/chats) → Do(DELETE /members/{self})`

---

### TestAuthEndpoints

**目的:** 验证未认证请求返回 401。

**依赖链:** `Do(all API endpoints without token)`

---

### TestListChatsWithUnreads

**目的:** 验证聊天列表返回正确的未读计数。

**依赖链:** `Register x2 → Do(POST /api/chats) → Do(POST /messages) → Do(GET /api/chats/my)`

---

### TestUpload/TestUploadExceedsSizeLimit/TestUploadRejectsUnsupportedMime

**目的:** 验证文件上传成功、超限返回 413、非法 MIME 返回 415。

**依赖链:** `Register → Do(POST /api/uploads with multipart)`

---

### TestUpdateMeUsernameConflict

**目的:** 验证用户名冲突返回 409。

**依赖链:** `Register x2 → Do(PATCH /api/users/me)`

---

### TestCreateChatInvalidInput

**目的:** 验证缺少 name 返回 400。

**依赖链:** `Register → Do(POST /api/chats without name)`

---

### TestSendMessageNonMember

**目的:** 验证非成员发消息返回 403。

**依赖链:** `Register x2 → Do(POST /api/chats) → Do(POST /messages as non-member)`

---

### TestCreateOrGetDM

**目的:** 验证重复创建 DM 返回已有 DM。

**依赖链:** `Register x2 → Do(POST /api/dms x2)`

---

## auth_flow_test — 认证流程测试

**文件:** `server/internal/testutil/auth_flow_test.go`（363 行）

**共用依赖链:** `testutil.New → Register → Do(all endpoints)`

### TestRegisterLoginRefresh

| 步骤 | 调用 | 验证 |
|------|------|------|
| 注册 | `POST /api/auth/register` | 200 + access_token |
| 登出 | `POST /api/auth/logout` | 200 |
| 登录 | `POST /api/auth/login` | 200 + 新的 access_token |
| 刷新 | `POST /api/auth/refresh` | 200 + 新的 session |

---

### TestAccessDeniedWithoutToken

**目的:** 验证未带 token 的认证路由全部返回 401。

**端点列表:** `GET /api/users/me`, `PATCH /api/users/me`, `GET /api/users`, `GET /api/chats/my`, `POST /api/chats`, `POST /api/dms`, `POST /api/auth/logout`

---

### TestInvalidAccessToken

**目的:** 验证使用伪造的 token 访问返回 401。

---

### TestTamperedRefreshToken

**目的:** 验证篡改 refresh_token cookie 返回 401。

**依赖链:** `Register → 篡改 refresh_token → Do(POST /api/auth/refresh)`

---

### TestRefreshTokenRotation

**目的:** 验证 refresh token 单次使用：使用旧 token 第二次刷新返回 401。

**依赖链:** `Register → Do(Refresh) → Do(Refresh 旧 token) → 401`

---

### TestRefreshWithoutCookie

**目的:** 验证无 refresh_token cookie 的刷新请求返回 400。

---

### TestRegisterDuplicateEmail

**目的:** 验证重复注册 email 返回 409。

---

### TestRegisterDuplicateUsername

**目的:** 验证重复用户名返回 409。

---

### TestLoginWrongPassword

**目的:** 验证错误密码返回 401。

---

### TestConcurrentRefreshRotation

**目的:** 验证并发刷新时只有一个成功（互斥锁）。

**依赖链:** 多 goroutine 同时 `POST /api/auth/refresh`，仅一个成功

---

### TestLogoutInvalidatesTokens

**目的:** 验证登出后 access_token 仍可用直到过期，但 refresh 失败。

---

### TestCookieSecurityAttributes

**目的:** 验证 access_token cookie: HttpOnly=true, SameSite=Lax, Secure 在有 TLS 时为 true。

**条件分支:**
- `HttpOnly != true` → `Fatalf`
- `SameSite` 不是 `Lax` → `Fatalf`

---

### TestMultiDeviceRefreshIsolation

**目的:** 验证多设备各自持有独立的 refresh token，一个设备登出不影响其他。

---

### TestRegisterNoValidation

**目的:** 验证注册时无验证约束（空 password 不导致 panic）。

---

## integration_test — 集成测试

**文件:** `server/internal/testutil/integration_test.go`（66 行）

### TestFixtureSetup

**目的:** 验证 Fixture 初始化正常，DB 非 nil。

### TestUserRegisterLogin

**目的:** 注册 + 登录完整流程，返回值含 user 和 access_token。

### TestDuplicateEmail

**目的:** 重复 email 注册返回 409。

### TestUnauthorizedAccess

**目的:** 未认证请求返回 401（GET /api/users/me 和 GET /api/chats/my）。

### TestRefreshTokenFlow

**目的:** 注册 → 刷新 → 刷新后旧 token 不可用。

---

## ws_test — WebSocket 测试

**文件:** `server/internal/ws/ws_test.go`（255 行）

### TestWSConnectAndReady

**目的:** WS 连接成功后收到 `ready` 事件（含 user、chats、online_user_ids）。

**依赖链:** `testutil.New → Register → ws.Dial → ReadMessage`

---

### TestWSPingPong

**目的:** 发送 `ping` 收到 `pong` 响应。

**依赖链:** `Register → ws.Dial → WriteMessage(ping) → ReadMessage(pong)`

---

### TestWSSubscribeAndReceiveMessage

**目的:** 两个用户各自连接 WS，Alice 发消息，Bob 收到广播。

**依赖链:** `Register x2 → ws.Dial x2 → Do(POST /messages) → ReadMessage`

---

### TestWSTyping

**目的:** 发送 typing 事件，其他成员收到 typing 通知。

**依赖链:** `Register x2 → ws.Dial x2 → WriteMessage(typing) → ReadMessage(typing)`

---

### TestWSUnauthorized

**目的:** 无 token 的 WS 连接被拒绝。

**依赖链:** `ws.Dial without token → 连接失败`

---

### TestWSPresence

**目的:** 用户连接/断开时在线状态广播变化。

**依赖链:** `Register x2 → ws.Dial → Close → ReadMessage(presence)`

---

## Fixture 依赖

**文件:** `server/internal/testutil/testutil.go`

```go
type Fixture struct {
	Cfg     *config.Config
	DB      *db.DB
	Auth    *auth.Service
	Hub     *ws.Hub
	Gateway *ws.Gateway
	Server  *handlers.Server
	HTTP    *httptest.Server
}
```

**初始化流程:** `db.Open → auth.New → ws.NewHub → ws.NewGateway → handlers.New → httptest.NewServer`

**Do 方法:** `f.HTTP.Client().Do(req)` — 自动填充 `Authorization: Bearer <token>`（token 参数非空时），自动设置 `Content-Type: application/json`，body JSON 编码。

**Register 方法:** `Do(POST /api/auth/register, {email, username, password})` → 返回 `{UserID, Email, AccessToken, RefreshCookie}`