# Test Suite 规范

> 原始来源：
> - `server/internal/db/db_test.go`
> - `server/internal/testutil/handler_test.go`
> - `server/internal/testutil/auth_flow_test.go`
> - `server/internal/testutil/integration_test.go`
> - `server/internal/ws/ws_test.go`
>
> 依赖骨架：`server/internal/testutil/testutil.go`

---

## 一、测试骨架

```go
// server/internal/testutil/testutil.go
package testutil

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/auth"
	"github.com/Hana-ame/chat-app/server/internal/config"
	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/handlers"
	"github.com/Hana-ame/chat-app/server/internal/ws"
)

type Fixture struct {
	Cfg     *config.Config
	DB      *db.DB
	Auth    *auth.Service
	Hub     *ws.Hub
	Gateway *ws.Gateway
	Server  *handlers.Server
	HTTP    *httptest.Server
}

func New(t *testing.T) *Fixture {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		Addr:            ":0",
		DBPath:          filepath.Join(dir, "test.db"),
		UploadDir:       filepath.Join(dir, "uploads"),
		JWTSecret:       []byte("test-secret-very-secret-test-secret-very-secret"),
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
		MaxUploadBytes:  5 << 20,
		StaticDir:       "",
		AllowOrigins:    []string{"*"},
	}
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	authSvc := auth.New(cfg.JWTSecret, cfg.AccessTokenTTL)
	hub := ws.NewHub(database)
	gateway := ws.NewGateway(hub, database, authSvc)
	srv := handlers.New(cfg, database, authSvc, hub)

	httpSrv := httptest.NewServer(srv.Router(gateway))
	t.Cleanup(httpSrv.Close)

	return &Fixture{
		Cfg: cfg, DB: database, Auth: authSvc,
		Hub: hub, Gateway: gateway, Server: srv, HTTP: httpSrv,
	}
}

func (f *Fixture) Ctx() context.Context { return context.Background() }
```

### Fixture 初始化流程

| 步骤 | 调用 | 说明 |
|------|------|------|
| 1 | `t.TempDir()` | 创建临时目录 |
| 2 | `config.Config{...}` | 硬编码测试配置 |
| 3 | `db.Open(cfg.DBPath)` | SQLite 文件 DB + 迁移 |
| 4 | `auth.New(secret, ttl)` | JWT 认证服务 |
| 5 | `ws.NewHub(database)` | WebSocket Hub |
| 6 | `ws.NewGateway(hub, db, auth)` | WS 网关 |
| 7 | `handlers.New(cfg, db, auth, hub)` | HTTP 路由 |
| 8 | `httptest.NewServer(router)` | 测试 HTTP 服务器 |

### `Fixture` 方法

| 方法 | 签名 | 说明 |
|------|------|------|
| `Ctx` | `() context.Context` | 返回 `context.Background()` |
| `Do` | `(t, method, path, token, body) *http.Response` | 构造 HTTP 请求并执行。`token` 非空时设置 `Authorization: Bearer <token>`。`body` 为 nil 时无 body，为 string 时原样发送，否则 JSON 编码 |
| `Register` | `(t, email, username, password) *Session` | 调用 `POST /api/auth/register`，解析响应返回 `{UserID, Email, AccessToken, RefreshCookie}` |
| `ReadAll` | `(t, resp) string` | 读取响应体并关闭 |

```go
// Do 方法逻辑：
func (f *Fixture) Do(t *testing.T, method, path, token string, body any) *http.Response {
    var reqBody io.Reader
    switch v := body.(type) {
    case nil:
        reqBody = nil
    case string:
        reqBody = strings.NewReader(v)
    default:
        b, _ := json.Marshal(v)
        reqBody = bytes.NewReader(b)
    }
    req, _ := http.NewRequest(method, f.HTTP.URL+path, reqBody)
    if token != "" {
        req.Header.Set("Authorization", "Bearer "+token)
    }
    if body != nil {
        req.Header.Set("Content-Type", "application/json")
    }
    return f.HTTP.Client().Do(req)
}
```

---

## 二、测试总表

### 2.1 `db_test.go` — DAO 层

**文件:** `server/internal/db/db_test.go`（221 行）

| # | 测试名 | 行号 | 覆盖函数 | 验证点 |
|---|--------|------|----------|--------|
| 1 | `TestDBOpenAndMigrate` | 12 | `DB.Open`, `DB.CreateUser` | DB 非 nil；用户字段完整 |
| 2 | `TestUserCreateDuplicateEmail` | 27 | `DB.CreateUser` | 重复 email → `ErrConflict` |
| 3 | `TestGetUserByEmail` | 39 | `DB.CreateUser`, `DB.GetUserByEmail` | 存在返回 user+hash；不存在 → `ErrNotFound` |
| 4 | `TestSearchUsers` | 58 | `DB.SearchUsers` | 模糊搜索命中正确数量 |
| 5 | `TestCreateChatGroupAndDM` | 79 | `DB.CreateChat`, `DB.GetChatMembers` | group/dm 创建正确；MemberCount；成员列表完整 |
| 6 | `TestFindDMBetween` | 117 | `DB.FindDMBetween` | 存在返回 DM；不存在 → `ErrNotFound`；自 DM 报错 |
| 7 | `TestListUserChats` | 142 | `DB.ListUserChats` | 用户聊天列表数量、类型完整 |
| 8 | `TestAddRemoveMember` | 171 | `DB.AddChatMember`, `DB.RemoveChatMember`, `DB.IsChatMember` | 添加/移除成功；重复添加 → `ErrConflict` |
| 9 | `TestDeleteChat` | 197 | `DB.DeleteChat`, `DB.GetChat` | 删除后 `GetChat` → `ErrNotFound` |
| 10 | `TestRenameChat` | 210 | `DB.RenameChat`, `DB.GetChat` | 重命名成功 |

### 2.2 `handler_test.go` — HTTP Handler 层

**文件:** `server/internal/testutil/handler_test.go`（800 行）

| # | 测试名 | 行号 | 覆盖端点 | 验证点 |
|---|--------|------|----------|--------|
| 11 | `TestCreateGroupChatAndSendMessage` | 13 | `POST /api/chats`, `POST .../messages`, `GET .../messages`, `GET .../members`, `POST .../read` | 创建群聊、发消息、分页、成员列表、标记已读 |
| 12 | `TestCreateDM` | 87 | `POST /api/dms` | DM 类型正确 |
| 13 | `TestAddRemoveMembers` | 108 | `POST .../members`, `GET .../members`, `DELETE .../members/{id}` | 添加/移除成员、非成员 403、踢 owner 403 |
| 14 | `TestReactionsFlow` | 160 | `PUT .../reactions/{emoji}`, `DELETE .../reactions/{emoji}` | 添加/移除 emoji 反应 |
| 15 | `TestUpdateProfile` | 204 | `PATCH /api/users/me` | 更新 username/avatar_color/avatar_url |
| 16 | `TestSearchUsers` | 240 | `GET /api/users?q=` | 模糊搜索 |
| 17 | `TestDeleteMessageAsAdmin` | 267 | `DELETE .../messages/{id}` | owner 删他人消息 |
| 18 | `TestLeaveGroupChat` | 299 | `DELETE .../members/{self}` | 成员退出群聊 |
| 19 | `TestAuthEndpoints` | 349 | 所有认证路由 | 无 token → 401 |
| 20 | `TestListChatsWithUnreads` | 378 | `GET /api/chats/my` | 聊天列表含未读 |
| 21 | `TestUploadFile` | 591 | `POST /api/uploads` | 上传成功 |
| 22 | `TestUploadExceedsSizeLimit` | 634 | `POST /api/uploads` | 超限 → 413 |
| 23 | `TestUploadRejectsUnsupportedMime` | 666 | `POST /api/uploads` | 非法 MIME → 415 |
| 24 | `TestUpdateMeUsernameConflict` | 700 | `PATCH /api/users/me` | 用户名冲突 → 409 |
| 25 | `TestCreateChatInvalidInput` | 721 | `POST /api/chats` | 缺少 name → 400 |
| 26 | `TestSendMessageNonMember` | 744 | `POST .../messages` | 非成员发消息 → 403 |
| 27 | `TestCreateOrGetDM` | 762 | `POST /api/dms` | 重复创建 DM 返回已有 |
| 28 | `TestHealthz` | 582 | `GET /healthz` | 200 + status ok + 非空 echo |

### 2.3 `auth_flow_test.go` — 认证流程

**文件:** `server/internal/testutil/auth_flow_test.go`（363 行）

| # | 测试名 | 行号 | 覆盖端点 | 验证点 |
|---|--------|------|----------|--------|
| 29 | `TestRegisterLoginRefresh` | 13 | `POST /api/auth/register`, `/login`, `/refresh`, `/logout` | 完整 auth 生命周期 |
| 30 | `TestAccessDeniedWithoutToken` | 43 | 7 个认证路由 | 401 |
| 31 | `TestInvalidAccessToken` | 67 | `GET /api/users/me` | 伪造 token → 401 |
| 32 | `TestTamperedRefreshToken` | 88 | `POST /api/auth/refresh` | 篡改 refresh cookie → 401 |
| 33 | `TestRefreshTokenRotation` | 110 | `POST /api/auth/refresh`（2次） | 旧 token 第二次 → 401 |
| 34 | `TestRefreshWithoutCookie` | 133 | `POST /api/auth/refresh` | 无 cookie → 400 |
| 35 | `TestRegisterDuplicateEmail` | 146 | `POST /api/auth/register`（2次） | 重复 email → 409 |
| 36 | `TestRegisterDuplicateUsername` | 171 | `POST /api/auth/register`（2次） | 重复 username → 409 |
| 37 | `TestLoginWrongPassword` | 198 | `POST /api/auth/login` | 错误密码 → 401 |
| 38 | `TestConcurrentRefreshRotation` | 213 | `POST /api/auth/refresh`（并发） | 并发刷新仅一个成功 |
| 39 | `TestLogoutInvalidatesTokens` | 268 | `POST /api/auth/logout`, `/refresh` | 登出后 refresh 失败 |
| 40 | `TestCookieSecurityAttributes` | 298 | `POST /api/auth/register` | HttpOnly=true, SameSite=Lax |
| 41 | `TestMultiDeviceRefreshIsolation` | 334 | `POST /api/auth/logout`, `/refresh` | 多设备独立 refresh token |

### 2.4 `integration_test.go` — 集成测试

**文件:** `server/internal/testutil/integration_test.go`（66 行）

| # | 测试名 | 行号 | 覆盖路径 | 验证点 |
|---|--------|------|----------|--------|
| 42 | `TestFixtureSetup` | 10 | — | Fixture 初始化正常 |
| 43 | `TestUserRegisterLogin` | 18 | 注册 + 登录 | 返回值含 user + access_token |
| 44 | `TestDuplicateEmail` | 30 | 注册（2次） | 重复 → 409 |
| 45 | `TestUnauthorizedAccess` | 39 | `GET /api/users/me`, `/api/chats/my` | 无 token → 401 |
| 46 | `TestRefreshTokenFlow` | 52 | 注册 + 刷新 | 刷新成功，旧 token 不可用 |

### 2.5 `ws_test.go` — WebSocket 测试

**文件:** `server/internal/ws/ws_test.go`（255 行）

| # | 测试名 | 行号 | 覆盖事件 | 验证点 |
|---|--------|------|----------|--------|
| 47 | `TestWSConnectAndReady` | 33 | `ready` | 连接后收到 ready 事件 |
| 48 | `TestWSPingPong` | 46 | `ping`, `pong` | ping → pong |
| 49 | `TestWSSubscribeAndReceiveMessage` | 60 | `message_created` | Alice 发消息，Bob 广播接收 |
| 50 | `TestWSTyping` | 107 | `typing` | 成员收到 typing 通知 |
| 51 | `TestWSUnauthorized` | 155 | — | 无 token → 连接拒绝 |
| 52 | `TestWSPresence` | 179 | `presence` | 连接/断开 → 在线状态变化 |

---

## 三、测试详细说明

### 3.1 `db_test.go` — DAO 层

---

#### `TestDBOpenAndMigrate` (line 12)

```go
func TestDBOpenAndMigrate(t *testing.T) {
	f := testutil.New(t)
	if f.DB == nil {
		t.Fatal("DB is nil")
	}
	ctx := f.Ctx()
	u, err := f.DB.CreateUser(ctx, "test1@x.com", "test-user", "hash12345678")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID == "" || u.Username != "test-user" || u.AvatarColor == "" {
		t.Fatal("user creation incomplete")
	}
}
```

**依赖链:** `testutil.New → db.Open → d.Migrate → DB.CreateUser`

**条件分支:**
- `f.DB == nil` → `Fatal("DB is nil")`
- `CreateUser` 失败 → `Fatal(err)`
- `u.ID == "" || u.Username != "test-user" || u.AvatarColor == ""` → `Fatal("user creation incomplete")`

---

#### `TestUserCreateDuplicateEmail` (line 27)

```go
func TestUserCreateDuplicateEmail(t *testing.T) {
	f := testutil.New(t)
	_, err := f.DB.CreateUser(f.Ctx(), "same@x.com", "u1", "hash12345678")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.DB.CreateUser(f.Ctx(), "same@x.com", "u2", "hash12345678")
	if err != db.ErrConflict {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}
```

**依赖链:** `testutil.New → DB.CreateUser x2`

**条件分支:**
- 首次 `CreateUser` 失败 → `Fatal(err)`
- 二次 `CreateUser` 错误不是 `db.ErrConflict` → `Fatalf`

---

#### `TestGetUserByEmail` (line 39)

```go
func TestGetUserByEmail(t *testing.T) {
	f := testutil.New(t)
	_, err := f.DB.CreateUser(f.Ctx(), "getme@x.com", "getme", "hash12345678")
	if err != nil {
		t.Fatal(err)
	}
	u, hash, err := f.DB.GetUserByEmail(f.Ctx(), "getme@x.com")
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "getme" || hash != "hash12345678" {
		t.Fatal("wrong user")
	}
	_, _, err = f.DB.GetUserByEmail(f.Ctx(), "nope@x.com")
	if err != db.ErrNotFound {
		t.Fatal("want not found")
	}
}
```

**依赖链:** `DB.CreateUser → DB.GetUserByEmail x2`

**条件分支:**
- `CreateUser` 失败 → `Fatal`
- `GetUserByEmail(存在)` 失败 → `Fatal`
- `u.Username != "getme" || hash != "hash12345678"` → `Fatal("wrong user")`
- `GetUserByEmail(不存在)` 不是 `db.ErrNotFound` → `Fatal`

---

#### `TestSearchUsers` (line 58)

```go
func TestSearchUsers(t *testing.T) {
	f := testutil.New(t)
	f.DB.CreateUser(f.Ctx(), "alpha@x.com", "Alpha", "pw12345678")
	f.DB.CreateUser(f.Ctx(), "beta@x.com", "Beta", "pw12345678")
	f.DB.CreateUser(f.Ctx(), "gamma@x.com", "gamma", "pw12345678")
	users, err := f.DB.SearchUsers(f.Ctx(), "alp", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Username != "Alpha" {
		t.Fatalf("want 1 result (Alpha), got %d", len(users))
	}
	users, err = f.DB.SearchUsers(f.Ctx(), "a", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) < 2 {
		t.Fatalf("want at least 2 results for fuzzy 'a', got %d", len(users))
	}
}
```

**依赖链:** `DB.CreateUser x3 → DB.SearchUsers x2`

**条件分支:**
- `SearchUsers("alp")` 失败 → `Fatal`
- 结果不是 1 条或不是 Alpha → `Fatalf`
- `SearchUsers("a")` 失败 → `Fatal`
- 结果 < 2 → `Fatalf`

---

#### `TestCreateChatGroupAndDM` (line 79)

```go
func TestCreateChatGroupAndDM(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "a@g.com", "Alice", "pw00000000")
	b, _ := f.DB.CreateUser(f.Ctx(), "b@g.com", "Bob", "pw00000000")
	c, _ := f.DB.CreateUser(f.Ctx(), "c@g.com", "Carol", "pw00000000")

	chat, err := f.DB.CreateChat(f.Ctx(), "group", "TestGroup", "", a.ID, []string{a.ID, b.ID, c.ID})
	if err != nil {
		t.Fatal(err)
	}
	if chat.Name != "TestGroup" || chat.Type != "group" || chat.OwnerID != a.ID {
		t.Fatal("chat metadata wrong")
	}
	if chat.MemberCount != 3 {
		t.Fatalf("want 3 members, got %d", chat.MemberCount)
	}
	members, err := f.DB.GetChatMembers(f.Ctx(), chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range members {
		if m.ID != a.ID && m.ID != b.ID && m.ID != c.ID {
			t.Fatalf("unexpected member: %s", m.ID)
		}
	}

	dm, err := f.DB.CreateChat(f.Ctx(), "dm", "", "", "", []string{a.ID, b.ID})
	if err != nil {
		t.Fatal(err)
	}
	if dm.Type != "dm" || dm.Name != "" {
		t.Fatal("DM metadata wrong")
	}
	if dm.OwnerID != "" {
		t.Fatal("DM should have no owner")
	}
}
```

**依赖链:** `DB.CreateUser x3 → DB.CreateChat(group) → DB.GetChatMembers → DB.CreateChat(dm)`

**条件分支:**
- `CreateChat(group)` 失败 → `Fatal`
- metadata 不匹配 → `Fatal("chat metadata wrong")`
- `MemberCount != 3` → `Fatalf`
- `GetChatMembers` 失败 → `Fatal`
- 成员列表包含意外用户 → `Fatalf`
- `CreateChat(dm)` 失败 → `Fatal`
- DM 类型或名称不对 → `Fatal`
- DM 有 owner → `Fatal`

---

#### `TestFindDMBetween` (line 117)

```go
func TestFindDMBetween(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "dma@x.com", "DMA", "pw00000000")
	b, _ := f.DB.CreateUser(f.Ctx(), "dmb@x.com", "DMB", "pw00000000")
	f.DB.CreateChat(f.Ctx(), "dm", "", "", "", []string{a.ID, b.ID})

	dm, err := f.DB.FindDMBetween(f.Ctx(), a.ID, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if dm == nil || dm.Type != "dm" {
		t.Fatal("DM not found")
	}

	_, err = f.DB.FindDMBetween(f.Ctx(), a.ID, "nonexistent")
	if err != db.ErrNotFound {
		t.Fatal("want not found")
	}

	_, err = f.DB.FindDMBetween(f.Ctx(), a.ID, a.ID)
	if err == nil {
		t.Fatal("should error on self-DM")
	}
}
```

**依赖链:** `DB.CreateUser x2 → DB.CreateChat(dm) → DB.FindDMBetween x3`

**条件分支:**
- `FindDMBetween(存在)` 失败 → `Fatal`
- `dm == nil || dm.Type != "dm"` → `Fatal`
- `FindDMBetween(不存在)` 不是 `ErrNotFound` → `Fatal`
- `FindDMBetween(self)` 无错误 → `Fatal`

---

#### `TestListUserChats` (line 142)

```go
func TestListUserChats(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "list1@x.com", "List1", "pw00000000")
	b, _ := f.DB.CreateUser(f.Ctx(), "list2@x.com", "List2", "pw00000000")
	f.DB.CreateChat(f.Ctx(), "group", "Chat1", "", a.ID, []string{a.ID, b.ID})
	f.DB.CreateChat(f.Ctx(), "dm", "", "", "", []string{a.ID, b.ID})

	chats, err := f.DB.ListUserChats(f.Ctx(), a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 2 {
		t.Fatalf("want 2 chats, got %d", len(chats))
	}
	foundGroup := false
	foundDM := false
	for _, c := range chats {
		if c.Type == "group" {
			foundGroup = true
		}
		if c.Type == "dm" {
			foundDM = true
		}
	}
	if !foundGroup || !foundDM {
		t.Fatal("missing chat type in list")
	}
}
```

**依赖链:** `DB.CreateUser x2 → DB.CreateChat x2 → DB.ListUserChats`

**条件分支:**
- `ListUserChats` 失败 → `Fatal`
- `len(chats) != 2` → `Fatalf`
- 缺少 group 或 dm → `Fatal`

---

#### `TestAddRemoveMember` (line 171)

```go
func TestAddRemoveMember(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "mem1@x.com", "Mem1", "pw00000000")
	b, _ := f.DB.CreateUser(f.Ctx(), "mem2@x.com", "Mem2", "pw00000000")
	c, _ := f.DB.CreateUser(f.Ctx(), "mem3@x.com", "Mem3", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "MemTest", "", a.ID, []string{a.ID, b.ID})

	if err := f.DB.AddChatMember(f.Ctx(), chat.ID, c.ID); err != nil {
		t.Fatal(err)
	}
	ok, _ := f.DB.IsChatMember(f.Ctx(), chat.ID, c.ID)
	if !ok {
		t.Fatal("should be member")
	}
	if err := f.DB.AddChatMember(f.Ctx(), chat.ID, c.ID); err != db.ErrConflict {
		t.Fatal("double add should conflict")
	}
	if err := f.DB.RemoveChatMember(f.Ctx(), chat.ID, c.ID); err != nil {
		t.Fatal(err)
	}
	ok, _ = f.DB.IsChatMember(f.Ctx(), chat.ID, c.ID)
	if ok {
		t.Fatal("should be removed")
	}
}
```

**依赖链:** `DB.CreateUser x3 → DB.CreateChat → DB.AddChatMember → DB.IsChatMember → DB.AddChatMember → DB.RemoveChatMember → DB.IsChatMember`

**条件分支:**
- `AddChatMember` 失败 → `Fatal`
- `!IsChatMember` → `Fatal`
- 重复 `AddChatMember` 不是 `ErrConflict` → `Fatal`
- `RemoveChatMember` 失败 → `Fatal`
- `IsChatMember` 仍 true → `Fatal`

---

#### `TestDeleteChat` (line 197)

```go
func TestDeleteChat(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "del1@x.com", "Del1", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "DeleteMe", "", a.ID, []string{a.ID})
	if err := f.DB.DeleteChat(f.Ctx(), chat.ID); err != nil {
		t.Fatal(err)
	}
	_, err := f.DB.GetChat(f.Ctx(), chat.ID)
	if err != db.ErrNotFound {
		t.Fatal("should be gone")
	}
}
```

**依赖链:** `DB.CreateUser → DB.CreateChat → DB.DeleteChat → DB.GetChat`

**条件分支:**
- `DeleteChat` 失败 → `Fatal`
- `GetChat` 不是 `ErrNotFound` → `Fatal`

---

#### `TestRenameChat` (line 210)

```go
func TestRenameChat(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "r1@x.com", "Renamer", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "OldName", "", a.ID, []string{a.ID})
	if err := f.DB.RenameChat(f.Ctx(), chat.ID, "NewName"); err != nil {
		t.Fatal(err)
	}
	updated, _ := f.DB.GetChat(f.Ctx(), chat.ID)
	if updated.Name != "NewName" {
		t.Fatal("rename didn't stick")
	}
}
```

**依赖链:** `DB.CreateUser → DB.CreateChat → DB.RenameChat → DB.GetChat`

**条件分支:**
- `RenameChat` 失败 → `Fatal`
- `updated.Name != "NewName"` → `Fatal`

---

### 3.2 `handler_test.go` — HTTP Handler 层

#### `TestCreateGroupChatAndSendMessage` (line 13)

**HTTP 流程:**

| 步骤 | 方法 | 路径 | 状态码预期 | 验证 |
|------|------|------|-----------|------|
| 注册 Alice | `POST /api/auth/register` | — | 200 | 获取 token |
| 注册 Bob | `POST /api/auth/register` | — | 200 | 获取 token |
| 创建群聊 | `POST /api/chats` | — | 201 | 获取 chatID |
| 发消息 | `POST .../messages` | — | 201 | 获取 msgID |
| 查消息 | `GET .../messages?limit=5` | — | 200 | 消息列表含刚发的 |
| 查成员 | `GET .../members` | — | 200 | 成员列表含 Alice、Bob |
| 标记已读 | `POST .../read` | — | 200 | — |

**条件分支:**
- 任一步骤状态码不符 → `Fatalf`
- 消息内容不匹配 → `Fatalf`
- 成员列表缺少用户 → `Fatalf`

---

#### `TestCreateDM` (line 87)

**依赖链:** `Register x2 → Do(POST /api/dms)`

**条件分支:**
- 状态码不是 200/201 → `Fatalf`
- 响应 chat type 不是 dm → `Fatal`

---

#### `TestAddRemoveMembers` (line 108)

**HTTP 流程:**

| 步骤 | 路径 | 预期 |
|------|------|------|
| 注册 3 用户，创建群聊 | — | — |
| 添加用户 C | `POST /api/chats/{id}/members` | 200 |
| 列出成员 | `GET /api/chats/{id}/members` | 200，含 C |
| 移除用户 C | `DELETE /api/chats/{id}/members/{C}` | 200 |
| 列出成员 | `GET /api/chats/{id}/members` | 200，不含 C |
| 非成员加人 | `POST /api/chats/{id}/members`（C 加 D） | 403 |
| 踢 owner | `DELETE /api/chats/{id}/members/{owner}` | 403 |

**条件分支:**
- 添加/移除失败 → `Fatalf`
- 成员列表包含已移除 → `Fatalf`
- 非成员操作非 403 → `Fatalf`
- 踢 owner 非 403 → `Fatalf`

---

#### `TestReactionsFlow` (line 160)

**HTTP 流程:**

| 步骤 | 路径 | 预期 |
|------|------|------|
| 注册 2 用户，创建群聊，发消息 | — | — |
| Alice 添加反应 | `PUT .../reactions/👍` | 200 |
| Alice 移除反应 | `DELETE .../reactions/👍` | 200 |

---

#### `TestUpdateProfile` (line 204)

**依赖链:** `Register → Do(PATCH /api/users/me)`

**条件分支:**
- 状态码 != 200 → `Fatalf`
- 更新后 username/avatar_color/avatar_url 不匹配 → `Fatalf`

---

#### `TestDeleteMessageAsAdmin` (line 267)

**HTTP 流程:**

| 步骤 | 路径 | 预期 |
|------|------|------|
| 注册 owner + member，创建群聊 | — | — |
| member 发消息 | `POST .../messages` | 201 |
| owner 删 member 消息 | `DELETE .../messages/{id}` | 200 |

**条件分支:**
- `DeleteMessage` 非 200 → `Fatalf`

---

#### `TestLeaveGroupChat` (line 299)

**HTTP 流程:**

| 步骤 | 路径 | 预期 |
|------|------|------|
| 注册 3 用户，创建群聊 | — | — |
| C 退出 | `DELETE .../members/{C.ID}` | 200 |
| 列表不含 C | `GET .../members` | 200 |

---

#### `TestAuthEndpoints` (line 349)

**条件分支:** 所有列出的认证路由无 token 请求返回 401。

**端点列表:** `GET /api/users/me`, `PATCH /api/users/me`, `GET /api/users`, `GET /api/chats/my`, `POST /api/chats`, `POST /api/dms`, `POST /api/auth/logout`

---

#### `TestHealthz` (line 582)

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

**依赖链:** `testutil.New → Do(GET /healthz) → json.Decode`

**条件分支:**
- `StatusCode != 200` → `Fatalf`
- `body[\"status\"] != \"ok\"` → `Fatalf`
- `echo` 不是 `map[string]any` → `Fatalf`
- `echo` 为空 → `Fatalf`

---

### 3.3 `auth_flow_test.go` — 认证流程

#### `TestRefreshTokenRotation` (line 110)

```go
func TestRefreshTokenRotation(t *testing.T) {
	f := testutil.New(t)
	s := f.Register(t, "rotate@test.dev", "Rotate", "testPass1!")
	// 第一次刷新 — 成功
	res := f.Do(t, "POST", "/api/auth/refresh", "", nil)
	if res.StatusCode != 200 {
		t.Fatalf("first refresh: %d", res.StatusCode)
	}
	// 第二次用旧 cookie — 失败
	res2 := f.Do(t, "POST", "/api/auth/refresh", "", nil)
	if res2.StatusCode != 401 {
		t.Fatalf("second refresh with old token: want 401 got %d", res2.StatusCode)
	}
}
```

**依赖链:** `Register → Do(Refresh) → Do(Refresh)`

**条件分支:**
- 首次刷新非 200 → `Fatalf`
- 二次刷新非 401 → `Fatalf`

---

#### `TestConcurrentRefreshRotation` (line 213)

**目的:** 验证多 goroutine 并发刷新时只有一个成功，其余返回 401（`refreshMu` 互斥锁）。

**依赖链:** `Register → 10 goroutine 同时 Do(Refresh)`

**条件分支:**
- 成功的请求数不为 1 → `Fatalf`
- 失败的请求状态码不为 401 → `Fatalf`

---

### 3.4 `ws_test.go` — WebSocket 测试

#### `TestWSConnectAndReady` (line 33)

**依赖链:** `Register → ws.Dial → ReadMessage(ready)`

**条件分支:**
- 连接失败 → `Fatalf`
- 未收到 ready 事件 → `Fatalf`
- ready 事件缺少 user/chats/online_user_ids → `Fatalf`

---

#### `TestWSSubscribeAndReceiveMessage` (line 60)

**HTTP 流程:**

| 步骤 | 说明 |
|------|------|
| 注册 Alice + Bob | 各自得到 token |
| 创建群聊，Alice 和 Bob 都在 | — |
| Alice 连接 WS | 收到 ready |
| Bob 连接 WS | 收到 ready |
| Alice 发消息 `POST /api/chats/{id}/messages` | — |
| Bob 收到 `message_created` 事件 | 消息内容匹配 |

---

#### `TestWSUnauthorized` (line 155)

**依赖链:** `testutil.New → ws.Dial(无 token)`

**条件分支:**
- 连接没有返回 error → `Fatalf`（应拒绝无 token 连接）

---

#### `TestWSPresence` (line 179)

**HTTP 流程:**

| 步骤 | 说明 |
|------|------|
| 注册 Alice + Bob，加入同群聊 | — |
| Alice 连 WS | 上线 |
| Bob 连 WS | 收到 `presence`（Alice online） |
| Alice 断开 | Bob 收到 `presence`（Alice offline） |

---

## 四、测试约束汇总

| 约束 | 说明 |
|------|------|
| DB 文件 | 每个测试独立 `t.TempDir()` 防止污染 |
| DB 迁移 | 每次 `db.Open` 自动运行 `init.sql` |
| 并发安全 | `TestConcurrentRefreshRotation` 验证互斥锁 |
| HTTP 服务器 | `httptest.NewServer` 随机端口，`t.Cleanup` 自动关闭 |
| WS 服务器 | 同上，共享同一端口 |
| Token | JWT secret 硬编码 `\"test-secret-very-secret-...\"` |
| Cookie | `Register` 自动解析 `Set-Cookie` 返回 `RefreshCookie` |
| 时间 | AccessTokenTTL=15m, RefreshTokenTTL=24h（测试范围内够用） |", "filePath": "/mnt/d/WorkPlace/chat-app/docs/reports/test-suite-spec-20260709.md"}