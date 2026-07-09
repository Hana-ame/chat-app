# testutil 测试工具规范

> 原始来源：
> - `server/internal/testutil/testutil.go`
> - `server/internal/testutil/client.go`
> - `server/internal/testutil/multipart.go`

---

## 一、原始代码

### Fixture 初始化

**文件:** `testutil/testutil.go`

```go
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
	dir := t.TempDir()
	cfg := &config.Config{
		Addr: ":0", DBPath: filepath.Join(dir, "test.db"),
		UploadDir:       filepath.Join(dir, "uploads"),
		JWTSecret:       []byte("test-secret-very-secret-test-secret-very-secret"),
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
		MaxUploadBytes:  5 << 20,
		StaticDir:       "",
		AllowOrigins:    []string{"*"},
	}
	database, err := db.Open(cfg.DBPath)
	if err != nil { t.Fatalf("db open: %v", err) }
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

### HTTP 客户端辅助

**文件:** `testutil/client.go`

```go
// Session 结构
type Session struct {
	UserID       string `json:"-"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	User         struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		Username    string `json:"username"`
		AvatarColor string `json:"avatar_color"`
		Status      string `json:"status"`
	} `json:"user"`
}

func ResponseCookie(res *http.Response, name string) *http.Cookie
func cookieValue(res *http.Response, name string) string

func (f *Fixture) Register(t, email, username, password) *Session
func (f *Fixture) Login(t, email, password) *Session
func (f *Fixture) Refresh(t, refreshToken) *Session
func (f *Fixture) Do(t, method, path, token, body) *http.Response
func (f *Fixture) DoWithCookie(t, method, path, token, cookieName, cookieValue, body) *http.Response
func (f *Fixture) DoJSON(t, method, path, token, body, out) int
func (f *Fixture) DoMultipart(t, method, path, token, fields, fileField, filename, content, contentType) *http.Response
func (f *Fixture) WSURL(token) string
func NewRecorder() *httptest.ResponseRecorder
func ContainsJSONError(body, errCode) bool
```

### Multipart 辅助

**文件:** `testutil/multipart.go`

```go
package testutil

import (
	"io"
	"mime/multipart"
)

func newMultipart(w io.Writer) *multipart.Writer {
	return multipart.NewWriter(w)
}
```

---

## 二、Fixture 字段总表

| 字段 | 类型 | 说明 |
|------|------|------|
| Cfg | `*config.Config` | 测试配置（硬编码） |
| DB | `*db.DB` | SQLite 测试数据库 |
| Auth | `*auth.Service` | JWT 认证服务 |
| Hub | `*ws.Hub` | WebSocket Hub |
| Gateway | `*ws.Gateway` | WS 网关 |
| Server | `*handlers.Server` | HTTP handler 容器 |
| HTTP | `*httptest.Server` | 测试 HTTP 服务器 |

---

## 三、Fixture 初始化流程

```
New(t)
  ├─ t.TempDir() → 独立临时目录（每个测试隔离）
  ├─ config.Config{硬编码}
  ├─ db.Open → 迁移 + SQLite
  ├─ auth.New → JWT (secret: "test-secret-...")
  ├─ ws.NewHub → Hub
  ├─ ws.NewGateway → Gateway
  ├─ handlers.New → Server + Router
  ├─ httptest.NewServer → HTTP
  └─ t.Cleanup → 关闭 DB + HTTP
```

---

## 四、Session 字段总表

| 字段 | 类型 | JSON | 说明 |
|------|------|------|------|
| UserID | `string` | `-` | 用户 UUID（从 User.ID 复制） |
| AccessToken | `string` | `"access_token"` | JWT |
| RefreshToken | `string` | `"refresh_token"` | 原始 refresh token（从 cookie 提取） |
| ExpiresIn | `int64` | `"expires_in"` | 过期秒数 |
| User | `struct` | `"user"` | 用户基本信息 |
| User.ID | `string` | `"id"` | 用户 UUID |
| User.Email | `string` | `"email"` | email |
| User.Username | `string` | `"username"` | 用户名 |
| User.AvatarColor | `string` | `"avatar_color"` | 头像颜色 |
| User.Status | `string` | `"status"` | 在线状态 |

---

## 五、方法总表

| 方法 | 签名 | 说明 |
|------|------|------|
| `Ctx` | `context.Context` | 返回 `context.Background()` |
| `Register` | `(t, email, username, password) *Session` | 注册 + 解析响应 |
| `Login` | `(t, email, password) *Session` | 登录 + 解析响应 |
| `Refresh` | `(t, refreshToken) *Session` | 刷新 + 解析响应 |
| `Do` | `(t, method, path, token, body) *http.Response` | 通用 HTTP 请求 |
| `DoWithCookie` | `(t, method, path, token, cookieName, cookieValue, body) *http.Response` | 带 cookie 的 HTTP 请求 |
| `DoJSON` | `(t, method, path, token, body, out) int` | HTTP 请求 + JSON 解码（仅 2xx） |
| `DoMultipart` | `(t, method, path, token, fields, fileField, filename, content, contentType) *http.Response` | multipart 上传 |
| `WSURL` | `(token) string` | 生成 WS URL（含 access_token） |
| `ResponseCookie` | `(res, name) *http.Cookie` | 从响应中提取 cookie |
| `NewRecorder` | `*httptest.ResponseRecorder` | 快速创建 ResponseRecorder |
| `ContainsJSONError` | `(body, errCode) bool` | 检查 JSON 响应是否包含指定 error code |

---

## 六、`Do` 方法逻辑

```go
func (f *Fixture) Do(t, method, path, token, body) *http.Response {
    var reader io.Reader
    if body != nil {
        b, _ := json.Marshal(body)
        reader = bytes.NewReader(b)
    }
    req, _ := http.NewRequest(method, f.HTTP.URL+path, reader)
    if body != nil {
        req.Header.Set("Content-Type", "application/json")
    }
    if token != "" {
        req.Header.Set("Authorization", "Bearer "+token)
    }
    return http.DefaultClient.Do(req)
}
```

---

## 七、测试配置

| 配置项 | 值 |
|--------|-----|
| DB 路径 | `t.TempDir() + "/test.db"` |
| Upload 路径 | `t.TempDir() + "/uploads"` |
| JWT Secret | `"test-secret-very-secret-test-secret-very-secret"` |
| AccessTokenTTL | `15m` |
| RefreshTokenTTL | `24h` |
| MaxUploadBytes | `5MB` |
| AllowOrigins | `["*"]` |

---

## 八、约束汇总

| 约束 | 说明 |
|------|------|
| 隔离 | 每个测试独立 `t.TempDir()`，互不污染 |
| 清理 | `t.Cleanup` 自动关闭 DB 和 HTTP 服务器 |
| 测试 Server | 随机端口，仅用于本测试 |
| Secret 硬编码 | 测试用 JWT secret 固定，与生产无关 |
| Body 编码 | `Do` 方法自动 JSON 编码 body |
| Token 注入 | `Do` 方法自动设置 `Authorization: Bearer <token>` |
| Multipart | `DoMultipart` 自动构造 multipart/form-data |