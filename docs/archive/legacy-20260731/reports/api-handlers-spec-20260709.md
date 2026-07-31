# API 接口规范 (API Handlers Spec)

---

## 目录

- [依赖组件](#依赖组件)
<!--
  CommonMark/goldmark 锚点规则：全小写，只保留 a-z 0-9 -，空格变 -，其余字符丢弃。
  目录中锚点由下方 section title 自动生成，此处仅作导航。
-->
- [认证中间件](#认证中间件)
- [辅助方法](#辅助方法)
  - [`bearerToken(r *http.Request) string`](#bearertokenr-httprequest-string)
  - [`writeJSON(w, status, body)`](#writejsonw-status-body)
  - [`writeError(w, status, code, message)`](#writeerrorw-status-code-message)
  - [`decodeJSON(r, into)`](#decodejsonr-into)
  - [`userFrom(ctx) *models.User`](#userfromctx-modeluser)
  - [`tokenFrom(ctx) string`](#tokenfromctx-string)
- [公开路由](#公开路由)
  - [`GET /healthz`](#get-healthz)
  - [`POST /api/auth/register`](#post-apiauthregister)
  - [`POST /api/auth/login`](#post-apiauthlogin)
  - [`POST /api/auth/refresh`](#post-apiauthrefresh)
  - [`POST /api/auth/logout`](#post-apiauthlogout)
- [认证路由](#认证路由)
  - [`GET /api/users/me`](#get-apiusersme)
  - [`PATCH /api/users/me`](#patch-apiusersme)
  - [`GET /api/users`](#get-apiusers)
  - [`GET /api/chats/my`](#get-apichatsmy)
  - [`GET /api/chats/public`](#get-apichatspublic)
  - [`POST /api/chats`](#post-apichats)
  - [`POST /api/dms`](#post-apidms-deprecated)
  - [`GET /api/chats/{chatID}`](#get-apichatschatid)
  - [`PATCH /api/chats/{chatID}`](#patch-apichatschatid)
  - [`DELETE /api/chats/{chatID}`](#delete-apichatschatid)
  - [`GET /api/chats/{chatID}/members`](#get-apichatschatidmembers)
  - [`POST /api/chats/{chatID}/members`](#post-apichatschatidmembers)
  - [`DELETE /api/chats/{chatID}/members/{userID}`](#delete-apichatschatidmembersuserid)
  - [`POST /api/chats/{chatID}/read`](#post-apichatschatidread-deprecated)
  - [`GET /api/chats/{chatID}/messages`](#get-apichatschatidmessages)
  - [`POST /api/chats/{chatID}/messages`](#post-apichatschatidmessages)
  - [`PATCH /api/chats/{chatID}/messages/{messageID}`](#patch-apichatschatidmessagesmessageid)
  - [`DELETE /api/chats/{chatID}/messages/{messageID}`](#delete-apichatschatidmessagesmessageid)
  - [`PUT /api/chats/{chatID}/messages/{messageID}/reactions/{emoji}`](#put-apichatschatidmessagesmessageidreactionsemoji)
  - [`DELETE /api/chats/{chatID}/messages/{messageID}/reactions/{emoji}`](#delete-apichatschatidmessagesmessageidreactionsemoji)
  - [`POST /api/chats/{chatID}/join`](#post-apichatschatidjoin)
  - [`POST /api/chats/{chatID}/pin`](#post-apichatschatidpin)
  - [`PATCH /api/chats/{chatID}/pin`](#patch-apichatschatidpin)
  - [`DELETE /api/chats/{chatID}/pin`](#delete-apichatschatidpin)
  - [`POST /api/uploads`](#post-apiuploads-deprecated)
- [额外路由](#额外路由不经过认证中间件)
  - [`GET /ws`](#get-ws)
  - [`GET /api/events`](#get-apievents)
  - [`GET /swagger/*`](#get-swagger)
  - [`GET /uploads/*`](#get-uploads-deprecated)
  - [`GET /*`](#get-spa)
- [辅助方法（Cookie 操作）](#辅助方法cookie操作)
  - [`setAuthCookie(w, r, name, value, path, ttl)`](#setauthcookiew-r-name-value-path-ttl)
  - [`setRefreshCookie(w, r, value, ttl)`](#setrefreshcookiew-r-value-ttl)
  - [`clearRefreshCookie(w, r)`](#clearrefreshcookiew-r)
  - [`timeNow()`](#timenow)

---

## 依赖组件

```go
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/Hana-ame/chat-app/server/internal/auth"
	"github.com/Hana-ame/chat-app/server/internal/config"
	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/models"
	"github.com/Hana-ame/chat-app/server/internal/ws"
)

type ctxKey string

const (
	ctxKeyUser  ctxKey = "user"
	ctxKeyToken ctxKey = "token"
)

type Server struct {
	Cfg       *config.Config
	DB        *db.DB
	Auth      *auth.Service
	Hub       *ws.Hub
	refreshMu sync.Mutex
}

func New(cfg *config.Config, database *db.DB, authSvc *auth.Service, hub *ws.Hub) *Server {
	return &Server{Cfg: cfg, DB: database, Auth: authSvc, Hub: hub}
}
```

---

## 认证中间件

```go
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := bearerToken(r)
		if tok == "" {
			if c, err := r.Cookie("access_token"); err == nil {
				tok = c.Value
			}
		}
		if tok == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing token")
			return
		}
		claims, err := s.Auth.ParseAccessToken(tok)
		if err != nil {
			if errors.Is(err, auth.ErrTokenExpired) {
				writeError(w, http.StatusUnauthorized, "token_expired", "access token expired")
				return
			}
			writeError(w, http.StatusUnauthorized, "token_invalid", "access token invalid")
			return
		}
		u, err := s.DB.GetUserByID(r.Context(), claims.UserID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(w, http.StatusUnauthorized, "user_not_found", "user does not exist")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeyUser, u)
		ctx = context.WithValue(ctx, ctxKeyToken, tok)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
```

**依赖链:** `bearerToken → r.Cookie → Auth.ParseAccessToken → DB.GetUserByID → context.WithValue`

**条件分支:**
- `bearerToken(r) == "" && r.Cookie("access_token") == nil` → `401 {"error":"unauthorized","message":"missing token"}`
- `errors.Is(err, auth.ErrTokenExpired)` → `401 {"error":"token_expired","message":"access token expired"}`
- `err != nil` (其他 JWT 错误) → `401 {"error":"token_invalid","message":"access token invalid"}`
- `errors.Is(err, db.ErrNotFound)` → `401 {"error":"user_not_found","message":"user does not exist"}`
- `err != nil` (DB 错误) → `500 {"error":"internal","message":err.Error()}`

---

## `issueSession(w, r, userID)`

**目的:** 签发完整 session（Access Token + Refresh Token），设置 cookie 并返回响应体。由 `Register`、`Login`、`Refresh` 调用。

**基本方法:** `issueSession(w, r, userID)` — 签发 token → 创建 refresh token → 设置 cookie → 返回 JSON。

```go
type sessionResp struct {
	User        any   `json:"user"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int64 `json:"expires_in"`
}

func (s *Server) issueSession(w http.ResponseWriter, r *http.Request, userID string) {
	access, exp, err := s.Auth.IssueAccessToken(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	raw, hash := auth.GenerateRefreshToken()
	if _, err := s.DB.CreateRefreshToken(r.Context(), userID, hash, s.Cfg.RefreshTokenTTL); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	u, err := s.DB.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	setAuthCookie(w, r, "access_token", access, "/", s.Cfg.AccessTokenTTL)
	setRefreshCookie(w, r, raw, s.Cfg.RefreshTokenTTL)
	_ = exp
	writeJSON(w, http.StatusOK, sessionResp{
		User:        u,
		AccessToken: access,
		ExpiresIn:   int64(s.Cfg.AccessTokenTTL.Seconds()),
	})
}
```

**依赖链:** `Auth.IssueAccessToken → auth.GenerateRefreshToken → DB.CreateRefreshToken → DB.GetUserByID → setAuthCookie + setRefreshCookie → writeJSON`

**条件分支:**
- `Auth.IssueAccessToken` 失败 → `500 {\"error\":\"internal\",\"message\":err.Error()}`
- `DB.CreateRefreshToken` 失败 → `500 {\"error\":\"internal\",\"message\":err.Error()}`
- `DB.GetUserByID` 失败 → `500 {\"error\":\"internal\",\"message\":err.Error()}`

---

## 辅助方法

### `bearerToken(r *http.Request) string`

```go
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	if t := r.URL.Query().Get("access_token"); t != "" {
		return t
	}
	return ""
}
```

**依赖链:** `r.Header.Get → strings.HasPrefix → r.URL.Query.Get`

**条件分支:**
- `strings.HasPrefix(h, "Bearer ")` → 返回 `strings.TrimSpace(h[7:])`
- `r.URL.Query().Get("access_token") != ""` → 返回 query 值
- 都失败 → 返回 `""`

### `writeJSON(w, status, body)`

```go
func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}
```

**条件分支:**
- `body == nil` → 只写 status 和 Content-Type，不写 body

### `writeError(w, status, code, message)`

```go
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": code, "message": message})
}
```

**依赖链:** `writeJSON → json.NewEncoder`

### `decodeJSON(r, into)`

```go
func decodeJSON(r *http.Request, into interface{}) error {
	if r.Body == nil {
		return errors.New("empty body")
	}
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(into)
}
```

**条件分支:**
- `r.Body == nil` → `return error("empty body")`

### `userFrom(ctx) *models.User`

```go
func userFrom(ctx context.Context) *models.User {
	v, _ := ctx.Value(ctxKeyUser).(*models.User)
	return v
}
```

### `tokenFrom(ctx) string`

```go
func tokenFrom(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyToken).(string)
	return v
}
```

---

## 公开路由

---

### GET /healthz

**目的:** 健康检查。返回排序后的请求头回显，用于调试连接状态。

**基本方法:** `GET /healthz` — 无认证。

```go
r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
    om := orderedmap.New()
    om.Set("status", "ok")
    echo := orderedmap.New()
    for k, v := range r.Header {
        echo.Set(k, strings.Join(v, ", "))
    }
    echo.SortKeys(func(keys []string) { sort.Strings(keys) })
    om.Set("echo", echo)
    om.SortKeys(func(keys []string) { sort.Strings(keys) })
    writeJSON(w, http.StatusOK, om)
})
```

**依赖链:** `orderedmap.New → strings.Join → sort.Strings → writeJSON`

**Response 200:**
```json
{"echo":{"Accept-Encoding":"gzip","User-Agent":"Go-http-client/1.1"},"status":"ok"}
```

---

### POST /api/auth/register

**目的:** 注册新用户。创建账号并直接签发 session。

**基本方法:** `POST /api/auth/register` — JSON body: `{email, username, password}`。返回 `sessionResp` + Set-Cookie。

```go
type registerReq struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	email := auth.NormalizeEmail(req.Email)
	username, err := auth.ValidateUsername(req.Username)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_username", err.Error())
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, "weak_password", err.Error())
		return
	}
	u, err := s.DB.CreateUser(r.Context(), email, username, hash)
	if err != nil {
		if errors.Is(err, db.ErrConflict) {
			writeError(w, http.StatusConflict, "already_taken", "email or username already taken")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.issueSession(w, r, u.ID)
}
```

**依赖链:** `decodeJSON → auth.NormalizeEmail → auth.ValidateUsername → auth.HashPassword → DB.CreateUser → issueSession`

**条件分支:**
- `decodeJSON` 失败 → `400 {"error":"bad_request","message":err.Error()}`
- `auth.ValidateUsername` 失败 → `400 {"error":"invalid_username","message":err.Error()}`
- `auth.HashPassword` 失败 → `400 {"error":"weak_password","message":err.Error()}`
- `errors.Is(err, db.ErrConflict)` → `409 {"error":"already_taken","message":"email or username already taken"}`
- `err != nil` (其他) → `500 {"error":"internal","message":err.Error()}`

**Response 200:**
```json
{"user":"models.User","access_token":"string (JWT)","expires_in":"int64 (seconds)"}
```

---

### POST /api/auth/login

**目的:** 用户登录。验证凭据并签发 session。

**基本方法:** `POST /api/auth/login` — JSON body: `{email, password}`。返回 `sessionResp` + Set-Cookie。

```go
type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	email := auth.NormalizeEmail(req.Email)
	u, hash, err := s.DB.GetUserByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if err := auth.VerifyPassword(hash, req.Password); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}
	s.issueSession(w, r, u.ID)
}
```

**依赖链:** `decodeJSON → auth.NormalizeEmail → DB.GetUserByEmail → auth.VerifyPassword → issueSession`

**条件分支:**
- `decodeJSON` 失败 → `400 {"error":"bad_request","message":err.Error()}`
- `errors.Is(err, db.ErrNotFound)` → `401 {"error":"invalid_credentials","message":"invalid email or password"}`
- `auth.VerifyPassword` 失败 → `401 {"error":"invalid_credentials","message":"invalid email or password"}`
- `err != nil` (其他) → `500 {"error":"internal","message":err.Error()}`

**Response 200:** same `sessionResp` as register

---

### POST /api/auth/refresh

**目的:** 刷新 Access Token。使用 Refresh Token（httpOnly cookie）换取新 session，单次使用。

**基本方法:** `POST /api/auth/refresh` — 从 cookie `refresh_token` 读取。返回 `sessionResp`。

```go
func (s *Server) Refresh(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("refresh_token")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "refresh token missing")
		return
	}
	s.refreshMu.Lock()
	hash := auth.HashRefreshToken(c.Value)
	rt, err := s.DB.FindRefreshToken(r.Context(), hash)
	if err != nil {
		s.refreshMu.Unlock()
		writeError(w, http.StatusUnauthorized, "refresh_invalid", "invalid refresh token")
		return
	}
	if rt.ExpiresAt.Before(timeNow()) {
		_ = s.DB.DeleteRefreshToken(r.Context(), rt.ID)
		s.refreshMu.Unlock()
		writeError(w, http.StatusUnauthorized, "refresh_expired", "refresh token expired")
		return
	}
	if err := s.DB.DeleteRefreshToken(r.Context(), rt.ID); err != nil {
		s.refreshMu.Unlock()
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.refreshMu.Unlock()
	s.issueSession(w, r, rt.UserID)
}
```

**依赖链:** `r.Cookie → auth.HashRefreshToken → DB.FindRefreshToken → timeNow → DB.DeleteRefreshToken → issueSession`

**条件分支:**
- `r.Cookie("refresh_token")` 失败 → `400 {"error":"bad_request","message":"refresh token missing"}`
- `DB.FindRefreshToken` 失败 → `401 {"error":"refresh_invalid","message":"invalid refresh token"}`
- `rt.ExpiresAt.Before(timeNow())` → 删除旧 token → `401 {"error":"refresh_expired","message":"refresh token expired"}`
- `DB.DeleteRefreshToken` 失败 → `500 {"error":"internal","message":err.Error()}`

**Response 200:** same `sessionResp`

---

### POST /api/auth/logout

**目的:** 登出。清除 refresh cookie 并吊销所有 refresh token。

**基本方法:** `POST /api/auth/logout` — 需认证。无 body。返回 `{"ok":true}`。

```go
func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing user")
		return
	}
	clearRefreshCookie(w, r)
	_ = s.DB.DeleteUserRefreshTokens(r.Context(), u.ID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
```

**依赖链:** `userFrom → clearRefreshCookie → DB.DeleteUserRefreshTokens → writeJSON`

**条件分支:**
- `userFrom(ctx) == nil` → `401 {"error":"unauthorized","message":"missing user"}`

**Response 200:** `{"ok":true}`

---

## 认证路由

---

### GET /api/users/me

**目的:** 获取当前登录用户信息。

**基本方法:** `GET /api/users/me` — 需认证。返回 `models.User`。

```go
func (s *Server) Me(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	writeJSON(w, http.StatusOK, u)
}
```

**依赖链:** `userFrom → writeJSON`

**条件分支:**
- `userFrom(ctx) == nil` → `401 {"error":"unauthorized"}`

**Response 200:** `models.User`

---

### PATCH /api/users/me

**目的:** 更新当前用户资料（用户名、头像颜色、头像 URL）。

**基本方法:** `PATCH /api/users/me` — 需认证。JSON body: `{username, avatar_color, avatar_url}`。返回 updated `models.User`。

```go
type updateProfileReq struct {
	Username    string `json:"username"`
	AvatarColor string `json:"avatar_color"`
	AvatarURL   string `json:"avatar_url"`
}

func (s *Server) UpdateMe(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	var req updateProfileReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var name string
	var err error
	if req.Username != "" {
		name, err = auth.ValidateUsername(req.Username)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_username", err.Error())
			return
		}
	} else {
		name = u.Username
	}
	if req.AvatarColor == "" {
		req.AvatarColor = u.AvatarColor
	}
	updated, err := s.DB.UpdateUserProfile(r.Context(), u.ID, name, req.AvatarColor, req.AvatarURL)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "user disappeared")
			return
		}
		if errors.Is(err, db.ErrConflict) {
			writeError(w, http.StatusConflict, "username_taken", "username already taken")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.BroadcastUserUpdate(updated)
	}
	writeJSON(w, http.StatusOK, updated)
}
```

**依赖链:** `userFrom → decodeJSON → auth.ValidateUsername → DB.UpdateUserProfile → Hub.BroadcastUserUpdate → writeJSON`

**条件分支:**
- `userFrom(ctx) == nil` → `401 {"error":"unauthorized"}`
- `decodeJSON` 失败 → `400 {"error":"bad_request","message":err.Error()}`
- `req.Username == ""` → 沿用 `u.Username`（跳过校验）
- `req.AvatarColor == ""` → 沿用 `u.AvatarColor`
- `errors.Is(err, db.ErrNotFound)` → `404 {"error":"not_found","message":"user disappeared"}`
- `errors.Is(err, db.ErrConflict)` → `409 {"error":"username_taken","message":"username already taken"}`
- `err != nil` (其他) → `500 {"error":"internal","message":err.Error()}`
- `s.Hub != nil` → `Hub.BroadcastUserUpdate(updated)`

**Response 200:** updated `models.User`

---

### GET /api/users

**目的:** 搜索用户。按用户名模糊搜索或完整 UUID 精确搜索，排除自身。不允许搜索 email。

**基本方法:** `GET /api/users?q=<string>` — 需认证。`q` 至少 1 字符，按 username LIKE 模糊匹配或完整 UUID 精确匹配。返回 `{\"users\":[...]}`。

```go
func (s *Server) SearchUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if len(q) < 1 {
		writeJSON(w, http.StatusOK, map[string]any{"users": []any{}})
		return
	}
	users, err := s.DB.SearchUsers(r.Context(), q, 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	me := userFrom(r.Context())
	filtered := users[:0]
	for _, u := range users {
		if me != nil && u.ID == me.ID {
			continue
		}
		filtered = append(filtered, u)
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": filtered})
}
```

**依赖链:** `r.URL.Query.Get → DB.SearchUsers → userFrom → writeJSON`

**条件分支:**
- `len(q) < 1` → 返回 `{"users":[]}`
- `DB.SearchUsers` 失败 → `500 {"error":"internal","message":err.Error()}`
- `me != nil && u.ID == me.ID` → 跳过自己（不返回自身）

**Response 200:** `{"users":["models.User"]}`

---

### GET /api/chats/my

**目的:** 获取当前用户的聊天列表。

**基本方法:** `GET /api/chats/my` — 需认证。返回 `{"chats":[...]}`。

```go
func (s *Server) ListChats(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chats, err := s.DB.ListUserChats(r.Context(), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"chats": chats})
}
```

**依赖链:** `userFrom → DB.ListUserChats → writeJSON`

**条件分支:**
- `DB.ListUserChats` 失败 → `500 {"error":"internal","message":err.Error()}`

**Response 200:** `{"chats":["models.Chat"]}`

---

### GET /api/chats/public

**目的:** 获取公开聊天列表（可发现的所有公开群组）。

**基本方法:** `GET /api/chats/public` — 需认证。返回 `{"chats":[...]}`。

```go
func (s *Server) ListPublicChats(w http.ResponseWriter, r *http.Request) {
	chats, err := s.DB.ListPublicChats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"chats": chats})
}
```

**依赖链:** `DB.ListPublicChats → writeJSON`

**条件分支:**
- `DB.ListPublicChats` 失败 → `500 {"error":"internal","message":err.Error()}`

**Response 200:** `{"chats":["models.Chat"]}`

---

### POST /api/chats

**目的:** 创建群聊。指定类型、名称、可见性和成员。

**基本方法:** `POST /api/chats` — 需认证。JSON body: `{type, name, visibility, member_ids}`。返回 201 `models.Chat`。

```go
type createChatReq struct {
	Type       string   `json:"type"`
	Name       string   `json:"name"`
	Visibility string   `json:"visibility"`
	MemberIDs  []string `json:"member_ids"`
}

func (s *Server) CreateChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	var req createChatReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if req.Type != "group" && req.Type != "dm" {
		writeError(w, http.StatusBadRequest, "bad_request", "type must be group or dm")
		return
	}
	if req.Type == "dm" {
		writeError(w, http.StatusBadRequest, "bad_request", "use POST /api/dms for direct messages")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "name required")
		return
	}
	members := req.MemberIDs
	hasMe := false
	for _, m := range members {
		if m == u.ID {
			hasMe = true
			break
		}
	}
	if !hasMe {
		members = append(members, u.ID)
	}
	chat, err := s.DB.CreateChat(r.Context(), "group", req.Name, req.Visibility, u.ID, members)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.BroadcastChatCreated(chat)
	}
	writeJSON(w, http.StatusCreated, chat)
}
```

**依赖链:** `userFrom → decodeJSON → strings.TrimSpace → DB.CreateChat → Hub.BroadcastChatCreated → writeJSON`

**条件分支:**
- `decodeJSON` 失败 → `400 {"error":"bad_request","message":err.Error()}`
- `req.Type != "group" && req.Type != "dm"` → `400 {"error":"bad_request","message":"type must be group or dm"}`
- `req.Type == "dm"` → `400 {"error":"bad_request","message":"use POST /api/dms for direct messages"}`
- `strings.TrimSpace(req.Name) == ""` → `400 {"error":"bad_request","message":"name required"}`
- `m` 列表中不含 `u.ID` → 自动追加当前用户
- `DB.CreateChat` 失败 → `400 {"error":"bad_request","message":err.Error()}`
- `s.Hub != nil` → `Hub.BroadcastChatCreated(chat)`

**Response 201:** `models.Chat`

---

### POST /api/dms \(Deprecated\)

**目的:** 创建或查找私聊（DM）。若已有 DM 则直接返回，否则创建。已废弃。

**基本方法:** `POST /api/dms` — 需认证。JSON body: `{user_id}`。返回 `models.Chat`。

```go
type createDMReq struct {
	UserID string `json:"user_id"`
}

func (s *Server) CreateOrGetDM(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	var req createDMReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if req.UserID == "" || req.UserID == u.ID {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid user_id")
		return
	}
	other, err := s.DB.GetUserByID(r.Context(), req.UserID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user_not_found", "")
		return
	}
	if dm, err := s.DB.FindDMBetween(r.Context(), u.ID, other.ID); err == nil {
		writeJSON(w, http.StatusOK, dm)
		return
	}
	chat, err := s.DB.CreateChat(r.Context(), "dm", "", "", "", []string{u.ID, other.ID})
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.BroadcastChatCreated(chat)
	}
	writeJSON(w, http.StatusCreated, chat)
}
```

**依赖链:** `userFrom → decodeJSON → DB.GetUserByID → DB.FindDMBetween → DB.CreateChat → Hub.BroadcastChatCreated → writeJSON`

**条件分支:**
- `decodeJSON` 失败 → `400`
- `req.UserID == "" || req.UserID == u.ID` → `400 {"error":"bad_request","message":"invalid user_id"}`
- `DB.GetUserByID` 失败 → `404 {"error":"user_not_found"}`
- `DB.FindDMBetween` 成功 → `200 dm`（已有 DM，直接返回）
- `DB.FindDMBetween` 失败 → 创建新 DM → `201 chat`

---

### GET /api/chats/\{chatID\}

**目的:** 获取单个聊天的详情（需是成员）。

**基本方法:** `GET /api/chats/{chatID}` — 需认证。返回 `models.Chat`。

```go
func (s *Server) GetChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	ok, err := s.DB.IsChatMember(r.Context(), id, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "not a member")
		return
	}
	c, err := s.DB.GetChat(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	writeJSON(w, http.StatusOK, c)
}
```

**依赖链:** `userFrom → chi.URLParam → DB.IsChatMember → DB.GetChat → writeJSON`

**条件分支:**
- `DB.IsChatMember` 失败 → `500`
- `!ok` (非成员) → `403 {"error":"forbidden","message":"not a member"}`
- `DB.GetChat` 失败 → `404 {"error":"not_found"}`

**Response 200:** `models.Chat`

---

### PATCH /api/chats/\{chatID\}

**目的:** 重命名群聊。仅 owner 可操作，DM 不可重命名。

**基本方法:** `PATCH /api/chats/{chatID}` — 需认证。JSON body: `{name}`。返回 updated `models.Chat`。

```go
type renameChatReq struct {
	Name string `json:"name"`
}

func (s *Server) RenameChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	c, err := s.DB.GetChat(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if c.Type == "dm" {
		writeError(w, http.StatusBadRequest, "bad_request", "cannot rename dm")
		return
	}
	if c.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "only owner can rename")
		return
	}
	var req renameChatReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := s.DB.RenameChat(r.Context(), id, req.Name); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	updated, _ := s.DB.GetChat(r.Context(), id)
	if s.Hub != nil && updated != nil {
		s.Hub.BroadcastChatUpdated(updated)
	}
	writeJSON(w, http.StatusOK, updated)
}
```

**依赖链:** `userFrom → chi.URLParam → DB.GetChat → decodeJSON → DB.RenameChat → DB.GetChat → Hub.BroadcastChatUpdated → writeJSON`

**条件分支:**
- `DB.GetChat` 失败 → `404 {"error":"not_found"}`
- `c.Type == "dm"` → `400 {"error":"bad_request","message":"cannot rename dm"}`
- `c.OwnerID != u.ID` → `403 {"error":"forbidden","message":"only owner can rename"}`
- `decodeJSON` 失败 → `400`
- `DB.RenameChat` 失败 → `400`
- `s.Hub != nil && updated != nil` → `Hub.BroadcastChatUpdated(updated)`

**Response 200:** updated `models.Chat`

---

### DELETE /api/chats/\{chatID\}

**目的:** 删除群聊。仅 owner 可操作，DM 不可删除。

**基本方法:** `DELETE /api/chats/{chatID}` — 需认证。返回 `{"ok":true}`。

```go
func (s *Server) DeleteChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	c, err := s.DB.GetChat(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if c.Type == "dm" {
		writeError(w, http.StatusBadRequest, "bad_request", "cannot delete dm; leave instead")
		return
	}
	if c.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "only owner can delete")
		return
	}
	if err := s.DB.DeleteChat(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.BroadcastChatDeleted(c, id)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
```

**依赖链:** `userFrom → chi.URLParam → DB.GetChat → DB.DeleteChat → Hub.BroadcastChatDeleted → writeJSON`

**条件分支:**
- `DB.GetChat` 失败 → `404`
- `c.Type == "dm"` → `400 {"error":"bad_request","message":"cannot delete dm; leave instead"}`
- `c.OwnerID != u.ID` → `403 {"error":"forbidden","message":"only owner can delete"}`
- `DB.DeleteChat` 失败 → `500`
- `s.Hub != nil` → `Hub.BroadcastChatDeleted(c, id)`

**Response 200:** `{"ok":true}`

---

### GET /api/chats/\{chatID\}/members

**目的:** 获取聊天成员列表（需是成员）。

**基本方法:** `GET /api/chats/{chatID}/members` — 需认证。返回 `{"members":[...]}`。

```go
func (s *Server) ListMembers(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	ok, err := s.DB.IsChatMember(r.Context(), id, u.ID)
	if err != nil || !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	members, err := s.DB.GetChatMembers(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}
```

**依赖链:** `userFrom → chi.URLParam → DB.IsChatMember → DB.GetChatMembers → writeJSON`

**条件分支:**
- `err != nil || !ok` (非成员或查询失败) → `403 {"error":"forbidden"}`
- `DB.GetChatMembers` 失败 → `500`

**Response 200:** `{"members":["models.ChatMember"]}`

---

### POST /api/chats/\{chatID\}/members

**目的:** 添加成员到群聊。操作者需已是成员，DM 不可添加。

**基本方法:** `POST /api/chats/{chatID}/members` — 需认证。JSON body: `{user_id}`。返回 updated `models.Chat`。

```go
type addMemberReq struct {
	UserID string `json:"user_id"`
}

func (s *Server) AddMember(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	c, err := s.DB.GetChat(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if c.Type == "dm" {
		writeError(w, http.StatusBadRequest, "bad_request", "cannot add to dm")
		return
	}
	ok, _ := s.DB.IsChatMember(r.Context(), id, u.ID)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	var req addMemberReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if _, err := s.DB.GetUserByID(r.Context(), req.UserID); err != nil {
		writeError(w, http.StatusNotFound, "user_not_found", "")
		return
	}
	if err := s.DB.AddChatMember(r.Context(), id, req.UserID); err != nil {
		if errors.Is(err, db.ErrConflict) {
			writeError(w, http.StatusConflict, "already_member", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	updated, _ := s.DB.GetChat(r.Context(), id)
	if s.Hub != nil && updated != nil {
		s.Hub.BroadcastChatUpdated(updated)
		s.Hub.NotifyUserNewChat(req.UserID, updated)
	}
	writeJSON(w, http.StatusOK, updated)
}
```

**依赖链:** `userFrom → chi.URLParam → DB.GetChat → DB.IsChatMember → decodeJSON → DB.GetUserByID → DB.AddChatMember → DB.GetChat → Hub.BroadcastChatUpdated + Hub.NotifyUserNewChat → writeJSON`

**条件分支:**
- `DB.GetChat` 失败 → `404`
- `c.Type == "dm"` → `400 {"error":"bad_request","message":"cannot add to dm"}`
- `!ok` (操作者非成员) → `403 {"error":"forbidden"}`
- `decodeJSON` 失败 → `400`
- `DB.GetUserByID` 失败 → `404 {"error":"user_not_found"}`
- `errors.Is(err, db.ErrConflict)` → `409 {"error":"already_member"}`
- `err != nil` (其他) → `500`
- `s.Hub != nil && updated != nil` → 广播 `ChatUpdated` + 通知目标用户 `NotifyUserNewChat`

**Response 200:** updated `models.Chat`

---

### DELETE /api/chats/\{chatID\}/members/\{userID\}

**目的:** 移除聊天成员。踢自己 = 退出；踢他人需 owner 或 admin；不可踢 owner。

**基本方法:** `DELETE /api/chats/{chatID}/members/{userID}` — 需认证。返回 `{"ok":true}`。

```go
func (s *Server) RemoveMember(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	target := chi.URLParam(r, "userID")
	c, err := s.DB.GetChat(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if c.Type == "dm" {
		writeError(w, http.StatusBadRequest, "bad_request", "cannot remove from dm")
		return
	}
	if target == c.OwnerID && target != u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "cannot kick owner")
		return
	}
	if target != u.ID {
		if err := s.requireOwnerOrAdmin(r.Context(), id, u.ID); err != nil {
			writeError(w, http.StatusForbidden, "forbidden", "only owner or admin can kick others")
			return
		}
	}
	if err := s.DB.RemoveChatMember(r.Context(), id, target); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.NotifyUserLeftChat(target, id)
		if updated, _ := s.DB.GetChat(r.Context(), id); updated != nil {
			s.Hub.BroadcastChatUpdated(updated)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
```

**依赖链:** `userFrom → chi.URLParam → DB.GetChat → requireOwnerOrAdmin → DB.RemoveChatMember → Hub.NotifyUserLeftChat + Hub.BroadcastChatUpdated → writeJSON`

**条件分支:**
- `DB.GetChat` 失败 → `404`
- `c.Type == "dm"` → `400 {"error":"bad_request","message":"cannot remove from dm"}`
- `target == c.OwnerID && target != u.ID` → `403 {"error":"forbidden","message":"cannot kick owner"}`
- `target != u.ID && requireOwnerOrAdmin` 失败 → `403 {"error":"forbidden","message":"only owner or admin can kick others"}`
- `DB.RemoveChatMember` 失败 → `500`

**Response 200:** `{"ok":true}`

---

### POST /api/chats/\{chatID\}/read \(Deprecated\)

**目的:** 标记已读消息。更新最后已读指针。已废弃。

**基本方法:** `POST /api/chats/{chatID}/read` — 需认证。JSON body: `{message_id}`。返回 `{"ok":true}`。

```go
type readReq struct {
	MessageID string `json:"message_id"`
}

func (s *Server) MarkRead(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	ok, _ := s.DB.IsChatMember(r.Context(), id, u.ID)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	var req readReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if req.MessageID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "message_id required")
		return
	}
	if err := s.DB.UpdateLastRead(r.Context(), id, u.ID, req.MessageID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
```

**依赖链:** `userFrom → chi.URLParam → DB.IsChatMember → decodeJSON → DB.UpdateLastRead → writeJSON`

**条件分支:**
- `!ok` → `403 {"error":"forbidden"}`
- `decodeJSON` 失败 → `400`
- `req.MessageID == ""` → `400 {"error":"bad_request","message":"message_id required"}`
- `DB.UpdateLastRead` 失败 → `500`

**Response 200:** `{"ok":true}`

---

### GET /api/chats/\{chatID\}/messages

**目的:** 获取聊天消息列表。支持游标分页和 member 详情。

**基本方法:** `GET /api/chats/{chatID}/messages[?limit=&before=&details=]` — 需认证。返回 `{"messages":[...]}`。

```go
func (s *Server) ListMessages(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	ok, _ := s.DB.IsChatMember(r.Context(), id, u.ID)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	before := r.URL.Query().Get("before")
	details := r.URL.Query().Get("details") == "true"
	msgs, err := s.DB.GetMessages(r.Context(), id, u.ID, before, limit, details)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}
```

**依赖链:** `userFrom → chi.URLParam → DB.IsChatMember → strconv.Atoi → DB.GetMessages → writeJSON`

**条件分支:**
- `!ok` → `403 {"error":"forbidden"}`
- `DB.GetMessages` 失败 → `500`

**Query Parameters:**
- `limit`: `int` 可选，默认 50
- `before`: `string` 可选，游标分页（取此 messageID 之前的消息）
- `details`: `bool` 可选，`true` 时返回 member 详情

**Response 200:** `{"messages":["models.Message"]}`

---

### POST /api/chats/\{chatID\}/messages

**目的:** 发送消息到聊天。支持文本、附件、@提及。

**基本方法:** `POST /api/chats/{chatID}/messages` — 需认证。JSON body: `{content, attachments}`。返回 201 `models.Message`。

```go
type sendMsgReq struct {
	Content     string              `json:"content"`
	Attachments []models.Attachment `json:"attachments"`
}

var mentionRegex = regexp.MustCompile(`<@([a-f0-9-]{36})>`)

func extractMentions(content string) []string {
	matches := mentionRegex.FindAllStringSubmatch(content, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

func (s *Server) SendMessage(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	ok, _ := s.DB.IsChatMember(r.Context(), id, u.ID)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	var req sendMsgReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	for i, a := range req.Attachments {
		if a.URL == "" || a.Filename == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "attachment missing url/filename")
			return
		}
		if !strings.HasPrefix(a.URL, "https://upload.moonchan.xyz/") {
			writeError(w, http.StatusBadRequest, "bad_request", "attachment url must be on upload.moonchan.xyz")
			return
		}
		if a.MimeType == "" {
			req.Attachments[i].MimeType = "application/octet-stream"
		}
	}
	mentions := extractMentions(req.Content)
	msg, err := s.DB.CreateMessage(r.Context(), id, u.ID, req.Content, mentions, req.Attachments)
	if err != nil {
		if strings.Contains(err.Error(), "content too long") {
			writeError(w, http.StatusForbidden, "content_too_long", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.BroadcastMessageCreate(msg)
	}
	writeJSON(w, http.StatusCreated, msg)
}
```

**依赖链:** `userFrom → chi.URLParam → DB.IsChatMember → decodeJSON → strings.HasPrefix → extractMentions → DB.CreateMessage → Hub.BroadcastMessageCreate → writeJSON`

**条件分支:**
- `!ok` → `403`
- `decodeJSON` 失败 → `400`
- `a.URL == "" || a.Filename == ""` → `400 {"error":"bad_request","message":"attachment missing url/filename"}`
- `!strings.HasPrefix(a.URL, "https://upload.moonchan.xyz/")` → `400 {"error":"bad_request","message":"attachment url must be on upload.moonchan.xyz"}`
- `a.MimeType == ""` → 默认 `"application/octet-stream"`
- `err` 包含 `"content too long"` → `403 {"error":"content_too_long","message":err.Error()}`
- `err != nil` (其他) → `400`
- `s.Hub != nil` → `Hub.BroadcastMessageCreate(msg)`

**Response 201:** `models.Message`

---

### PATCH /api/chats/\{chatID\}/messages/\{messageID\}

**目的:** 编辑自己的消息。仅作者可编辑。

**基本方法:** `PATCH /api/chats/{chatID}/messages/{messageID}` — 需认证。JSON body: `{content}`。返回 updated `models.Message`。

```go
type editMsgReq struct {
	Content string `json:"content"`
}

func (s *Server) EditMessage(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chatID := chi.URLParam(r, "chatID")
	id := chi.URLParam(r, "messageID")
	ok, _ := s.DB.IsChatMember(r.Context(), chatID, u.ID)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	var req editMsgReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	msg, err := s.DB.UpdateMessage(r.Context(), id, u.ID, req.Content)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "")
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if msg.ChatID != chatID {
		writeError(w, http.StatusBadRequest, "bad_request", "chat mismatch")
		return
	}
	if s.Hub != nil {
		s.Hub.BroadcastMessageUpdate(msg)
	}
	writeJSON(w, http.StatusOK, msg)
}
```

**依赖链:** `userFrom → chi.URLParam → DB.IsChatMember → decodeJSON → DB.UpdateMessage → Hub.BroadcastMessageUpdate → writeJSON`

**条件分支:**
- `!ok` → `403`
- `decodeJSON` 失败 → `400`
- `errors.Is(err, db.ErrNotFound)` → `404 {"error":"not_found"}`
- `err != nil` (其他) → `400`
- `msg.ChatID != chatID` → `400 {"error":"bad_request","message":"chat mismatch"}`
- `s.Hub != nil` → `Hub.BroadcastMessageUpdate(msg)`

**Response 200:** updated `models.Message`

---

### DELETE /api/chats/\{chatID\}/messages/\{messageID\}

**目的:** 删除消息。作者可删自己的消息，owner 或 admin 可删任意消息。

**基本方法:** `DELETE /api/chats/{chatID}/messages/{messageID}` — 需认证。返回 `{"ok":true}`。

```go
func (s *Server) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chatID := chi.URLParam(r, "chatID")
	id := chi.URLParam(r, "messageID")
	ok, _ := s.DB.IsChatMember(r.Context(), chatID, u.ID)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	existing, err := s.DB.GetMessage(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if existing.ChatID != chatID {
		writeError(w, http.StatusBadRequest, "bad_request", "chat mismatch")
		return
	}
	chat, _ := s.DB.GetChat(r.Context(), chatID)
	canDeleteAny := chat != nil && (chat.OwnerID == u.ID || s.requireOwnerOrAdmin(r.Context(), chatID, u.ID) == nil)
	if existing.UserID != u.ID && !canDeleteAny {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	if err := s.DB.DeleteMessage(r.Context(), id, u.ID, canDeleteAny); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.BroadcastMessageDelete(chatID, id)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
```

**依赖链:** `userFrom → chi.URLParam → DB.IsChatMember → DB.GetMessage → DB.GetChat → requireOwnerOrAdmin → DB.DeleteMessage → Hub.BroadcastMessageDelete → writeJSON`

**条件分支:**
- `!ok` → `403`
- `DB.GetMessage` 失败 → `404 {"error":"not_found"}`
- `existing.ChatID != chatID` → `400 {"error":"bad_request","message":"chat mismatch"}`
- `chat != nil && (chat.OwnerID == u.ID || requireOwnerOrAdmin == nil)` → `canDeleteAny = true`（owner/admin 可删他人消息）
- `existing.UserID != u.ID && !canDeleteAny` → `403 {"error":"forbidden"}`
- `DB.DeleteMessage` 失败 → `500`
- `s.Hub != nil` → `Hub.BroadcastMessageDelete(chatID, id)`

**Response 200:** `{"ok":true}`

---

### PUT /api/chats/\{chatID\}/messages/\{messageID\}/reactions/\{emoji\}

**目的:** 为消息添加 emoji 反应。

**基本方法:** `PUT .../reactions/{emoji}` — 需认证。Path 参数 `emoji` 需 URL 编码。返回 updated `models.Message`。

```go
func (s *Server) AddReaction(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chatID := chi.URLParam(r, "chatID")
	msgID := chi.URLParam(r, "messageID")
	emojiRaw := chi.URLParam(r, "emoji")
	emoji, err := url.PathUnescape(emojiRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "bad emoji encoding")
		return
	}
	ok, _ := s.DB.IsChatMember(r.Context(), chatID, u.ID)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	msg, err := s.DB.GetMessage(r.Context(), msgID)
	if err != nil || msg.ChatID != chatID {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err := s.DB.AddReaction(r.Context(), msgID, u.ID, emoji); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	updated, _ := s.DB.GetMessage(r.Context(), msgID)
	if updated != nil {
		enrichReactions(updated, u.ID)
	}
	if s.Hub != nil {
		s.Hub.BroadcastReaction(chatID, msgID, emoji, u.ID, true)
	}
	writeJSON(w, http.StatusOK, updated)
}
```

**依赖链:** `userFrom → chi.URLParam → url.PathUnescape → DB.IsChatMember → DB.GetMessage → DB.AddReaction → DB.GetMessage → enrichReactions → Hub.BroadcastReaction → writeJSON`

**条件分支:**
- `url.PathUnescape` 失败 → `400 {"error":"bad_request","message":"bad emoji encoding"}`
- `!ok` → `403`
- `DB.GetMessage` 失败 或 `msg.ChatID != chatID` → `404 {"error":"not_found"}`
- `DB.AddReaction` 失败 → `400`
- `s.Hub != nil` → `Hub.BroadcastReaction(added=true)`

**Response 200:** updated `models.Message`

---

### DELETE /api/chats/\{chatID\}/messages/\{messageID\}/reactions/\{emoji\}

**目的:** 移除自己的 emoji 反应。

**基本方法:** `DELETE .../reactions/{emoji}` — 需认证。Path 参数 `emoji` 需 URL 编码。返回 updated `models.Message`。

```go
func (s *Server) RemoveReaction(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chatID := chi.URLParam(r, "chatID")
	msgID := chi.URLParam(r, "messageID")
	emojiRaw := chi.URLParam(r, "emoji")
	emoji, err := url.PathUnescape(emojiRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "bad emoji encoding")
		return
	}
	ok, _ := s.DB.IsChatMember(r.Context(), chatID, u.ID)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	if err := s.DB.RemoveReaction(r.Context(), msgID, u.ID, emoji); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.BroadcastReaction(chatID, msgID, emoji, u.ID, false)
	}
	updated, _ := s.DB.GetMessage(r.Context(), msgID)
	if updated != nil {
		enrichReactions(updated, u.ID)
	}
	writeJSON(w, http.StatusOK, updated)
}
```

**依赖链:** `userFrom → chi.URLParam → url.PathUnescape → DB.IsChatMember → DB.RemoveReaction → Hub.BroadcastReaction → DB.GetMessage → enrichReactions → writeJSON`

**条件分支:**
- `url.PathUnescape` 失败 → `400`
- `!ok` → `403`
- `DB.RemoveReaction` 失败 → `500`
- `s.Hub != nil` → `Hub.BroadcastReaction(added=false)`

**Response 200:** updated `models.Message`

---

### POST /api/chats/\{chatID\}/join

**目的:** 加入公开聊天。仅 `public` / `unlisted` 聊天可加入。

**基本方法:** `POST /api/chats/{chatID}/join` — 需认证。无 body。返回 `{"ok":true}`。

```go
func (s *Server) JoinChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	if err := s.DB.JoinChatByID(r.Context(), id, u.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	chat, _ := s.DB.GetChat(r.Context(), id)
	if s.Hub != nil && chat != nil {
		s.Hub.NotifyUserNewChat(u.ID, chat)
		s.Hub.BroadcastChatUpdated(chat)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
```

**依赖链:** `userFrom → chi.URLParam → DB.JoinChatByID → DB.GetChat → Hub.NotifyUserNewChat + Hub.BroadcastChatUpdated → writeJSON`

**条件分支:**
- `userFrom(ctx) == nil` → `401 {"error":"unauthorized"}`
- `DB.JoinChatByID` 失败 → `500`（内部限制仅 `public` / `unlisted` 可加入）
- `s.Hub != nil && chat != nil` → 通知目标用户 + 广播更新

**Response 200:** `{"ok":true}`

---

### POST /api/chats/\{chatID\}/pin

**目的:** 设置置顶消息。仅 owner，需 ≥3 成员。

**基本方法:** `POST /api/chats/{chatID}/pin` — 需认证。JSON body: `{content}`。返回 `{"ok":true}`。

```go
type pinContentReq struct {
	Content string `json:"content"`
}

func (s *Server) requireOwnerOrAdmin(ctx context.Context, chatID, userID string) error {
	c, err := s.DB.GetChat(ctx, chatID)
	if err != nil {
		return err
	}
	if c.OwnerID == userID {
		return nil
	}
	role, err := s.DB.GetChatMemberRole(ctx, chatID, userID)
	if err != nil {
		return err
	}
	if role == "admin" {
		return nil
	}
	return errors.New("forbidden")
}

func (s *Server) PinChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	c, err := s.DB.GetChat(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if c.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	n, err := s.DB.ChatMemberCount(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if n < 3 {
		writeError(w, http.StatusBadRequest, "bad_request", "need at least 3 members to pin")
		return
	}
	var req pinContentReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := s.DB.SetPinnedMessage(r.Context(), id, req.Content); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if s.Hub != nil {
		updated, _ := s.DB.GetChat(r.Context(), id)
		if updated != nil {
			s.Hub.BroadcastChatUpdated(updated)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
```

**依赖链:** `userFrom → chi.URLParam → DB.GetChat → DB.ChatMemberCount → decodeJSON → DB.SetPinnedMessage → DB.GetChat → Hub.BroadcastChatUpdated → writeJSON`

**`requireOwnerOrAdmin` 依赖链:** `DB.GetChat → DB.GetChatMemberRole`

**条件分支:**
- `userFrom(ctx) == nil` → `401`
- `DB.GetChat` 失败 → `404`
- `c.OwnerID != u.ID` → `403 {\"error\":\"forbidden\"}`
- `DB.ChatMemberCount` 失败 → `500`
- `n < 3` → `400 {"error":"bad_request","message":"need at least 3 members to pin"}`
- `decodeJSON` 失败 → `400`
- `DB.SetPinnedMessage` 失败 → `500`
- `s.Hub != nil` → `DB.GetChat` → `Hub.BroadcastChatUpdated(updated)`

**`requireOwnerOrAdmin` 条件分支:**
- `DB.GetChat` 失败 → 透传 err
- `c.OwnerID == userID` → `return nil`（owner 直接通过）
- `DB.GetChatMemberRole` 失败 → 透传 err
- `role == "admin"` → `return nil`
- 其他 → `return error("forbidden")`

**Response 200:** `{"ok":true}`

---

### PATCH /api/chats/\{chatID\}/pin

**目的:** 更新置顶消息。委托给 `PinChat`。

**基本方法:** `PATCH /api/chats/{chatID}/pin` — 需认证。JSON body: `{content}`。返回 `{"ok":true}`。

直接委托给 `PinChat`。

```go
func (s *Server) UpdatePinnedChat(w http.ResponseWriter, r *http.Request) {
	s.PinChat(w, r)
}
```

**依赖链:** `PinChat`（同 POST /pin）

---

### DELETE /api/chats/\{chatID\}/pin

**目的:** 清除置顶消息。需 owner 或 admin 权限。

**基本方法:** `DELETE /api/chats/{chatID}/pin` — 需认证。返回 `{"ok":true}`。

```go
func (s *Server) DeletePinnedChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	if err := s.requireOwnerOrAdmin(r.Context(), id, u.ID); err != nil {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	if err := s.DB.ClearPinnedMessage(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if s.Hub != nil {
		updated, _ := s.DB.GetChat(r.Context(), id)
		if updated != nil {
			s.Hub.BroadcastChatUpdated(updated)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
```

**依赖链:** `userFrom → chi.URLParam → requireOwnerOrAdmin → DB.ClearPinnedMessage → DB.GetChat → Hub.BroadcastChatUpdated → writeJSON`

**条件分支:**
- `userFrom(ctx) == nil` → `401`
- `requireOwnerOrAdmin` 失败 → `403`
- `DB.ClearPinnedMessage` 失败 → `500`
- `s.Hub != nil` → `DB.GetChat` → `Hub.BroadcastChatUpdated(updated)`

**Response 200:** `{"ok":true}`

---

### POST /api/uploads \(Deprecated\)

**目的:** 上传文件到本地存储。限制大小和 MIME 类型。已废弃（前端直传 upload.moonchan.xyz）。

**基本方法:** `POST /api/uploads` — 需认证。`multipart/form-data`，field `file`。返回 `{id, url, filename, mime_type, size}`。

```go
var allowedMime = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true,
	"video/mp4": true, "video/webm": true,
	"audio/mpeg": true, "audio/ogg": true, "audio/wav": true, "audio/webm": true,
	"application/pdf": true, "text/plain": true, "application/zip": true,
	"application/octet-stream": true,
}

func randomKey(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "_")
	if len(name) > 200 {
		name = name[len(name)-200:]
	}
	return name
}

func (s *Server) Upload(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.Cfg.MaxUploadBytes+1<<20)
	if err := r.ParseMultipartForm(s.Cfg.MaxUploadBytes); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large",
			fmt.Sprintf("max %d bytes", s.Cfg.MaxUploadBytes))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "file field missing")
		return
	}
	defer file.Close()
	if header.Size > s.Cfg.MaxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", "")
		return
	}
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(header.Filename))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	mimeType = strings.SplitN(mimeType, ";", 2)[0]
	if !allowedMime[mimeType] {
		writeError(w, http.StatusUnsupportedMediaType, "bad_mime",
			"unsupported mime type "+mimeType)
		return
	}
	if err := os.MkdirAll(s.Cfg.UploadDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	ext := filepath.Ext(header.Filename)
	key := randomKey(16) + ext
	target := filepath.Join(s.Cfg.UploadDir, key)
	dst, err := os.Create(target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	written, err := io.Copy(dst, file)
	dst.Close()
	if err != nil {
		os.Remove(target)
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":        key,
		"url":       "/uploads/" + key,
		"filename":  sanitizeFilename(header.Filename),
		"mime_type": mimeType,
		"size":      written,
	})
}
```

**依赖链:** `userFrom → http.MaxBytesReader → r.ParseMultipartForm → r.FormFile → allowedMime → os.MkdirAll → os.Create → io.Copy → writeJSON`

**条件分支:**
- `userFrom(ctx) == nil` → `401`
- `r.ParseMultipartForm` 失败 → `413 {"error":"too_large","message":"max N bytes"}`
- `r.FormFile("file")` 失败 → `400 {"error":"bad_request","message":"file field missing"}`
- `header.Size > Config.MaxUploadBytes` → `413 {"error":"too_large"}`
- `mimeType == ""` → 通过扩展名推断 → 仍为空则 `"application/octet-stream"`
- `!allowedMime[mimeType]` → `415 {"error":"bad_mime","message":"unsupported mime type "+mimeType}`
- `os.MkdirAll` 失败 → `500`
- `os.Create` 失败 → `500`
- `io.Copy` 失败 → `os.Remove(target)` + `500`

**Response 201:**
```json
{"id":"string","url":"/uploads/string","filename":"string","mime_type":"string","size":"int64"}
```

---

## 额外路由（不经过认证中间件）

---

### GET /ws

**目的:** WebSocket 连接入口。由 ws.Gateway 处理升级和消息路由。

**基本方法:** `GET /ws` — 无认证（认证在 WS 协议内处理）。

```go
if gateway != nil {
    r.Get("/ws", gateway.ServeHTTP)
}
```

由 `ws.Gateway.ServeHTTP` 处理。详见 `ws-architecture-spec`。

---

### GET /api/events

**目的:** SSE 事件流。接收实时事件（消息、通知、在线状态）。

**基本方法:** `GET /api/events?access_token=<string>` — 手动验证 token。返回 `text/event-stream`。

```go
func (s *Server) SSE(w http.ResponseWriter, r *http.Request) {
	tok := bearerToken(r)
	if tok == "" {
		tok = r.URL.Query().Get("access_token")
	}
	if tok == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing token")
		return
	}
	claims, err := s.Auth.ParseAccessToken(tok)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid token")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)

	userID := claims.UserID
	user, err := s.DB.GetUserByID(r.Context(), userID)
	if err != nil {
		return
	}

	chats, _ := s.DB.ListUserChats(r.Context(), userID)
	ready, _ := json.Marshal(map[string]any{
		"user": user, "chats": chats,
		"online_user_ids": s.Hub.OnlineUserIDs(),
	})
	fmt.Fprintf(w, "id: 0\nevent: ready\ndata: %s\n\n", ready)
	flusher.Flush()

	ch := make(chan []byte, 64)
	s.Hub.SSERegister(userID, ch)
	defer s.Hub.SSEUnregister(userID)

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case data, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
```

**依赖链:** `bearerToken → Auth.ParseAccessToken → DB.GetUserByID → DB.ListUserChats → Hub.OnlineUserIDs → Hub.SSERegister → for-select → Hub.SSEUnregister`

**条件分支:**
- `bearerToken(r) == "" && r.URL.Query().Get("access_token") == ""` → `401 {"error":"unauthorized","message":"missing token"}`
- `Auth.ParseAccessToken` 失败 → `401 {"error":"unauthorized","message":"invalid token"}`
- `w.(http.Flusher)` 失败 → `500 "SSE not supported"`
- `DB.GetUserByID` 失败 → 静默关闭连接
- `<-notify` (客户端断开) → `Hub.SSEUnregister` + return
- `ok == false` (channel 关闭) → return

---

### GET /swagger/\*

**目的:** Swagger API 文档页面。

**基本方法:** `GET /swagger/*` — 无认证。由 httpSwagger 提供服务。

```go
r.Get("/swagger/*", httpSwagger.Handler(
    httpSwagger.URL("https://wsl-8080.moonchan.xyz/swagger/doc.json"),
))
```

由 `httpSwagger.Handler` 提供 Swagger UI。

---

### GET /uploads/\* \(Deprecated\)

**目的:** 提供上传文件的静态访问。已废弃。

**基本方法:** `GET /uploads/*` — 无认证。返回静态文件。

```go
func (s *Server) serveUpload(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/uploads/")
	if rel == "" || strings.Contains(rel, "..") {
		http.NotFound(w, r)
		return
	}
	p := filepath.Join(s.Cfg.UploadDir, rel)
	info, err := os.Stat(p)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=2592000")
	http.ServeFile(w, r, p)
}
```

**条件分支:**
- `rel == "" || strings.Contains(rel, "..")` → `404`
- `os.Stat` 失败 或 `info.IsDir()` → `404`

---

### GET /\* \(SPA 兜底\)

**目的:** SPA 静态文件兜底路由。返回匹配的静态文件或 index.html。

**基本方法:** `GET /*` — 无认证。API 路径返回 404。

```go
func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws") {
		http.NotFound(w, r)
		return
	}
	if s.Cfg.StaticDir == "" {
		http.NotFound(w, r)
		return
	}
	clean := filepath.Clean("/" + r.URL.Path)
	rel := strings.TrimPrefix(clean, "/")
	if rel == "" {
		rel = "index.html"
	}
	p := filepath.Join(s.Cfg.StaticDir, rel)
	if !strings.HasPrefix(p, s.Cfg.StaticDir) {
		http.NotFound(w, r)
		return
	}
	if info, err := os.Stat(p); err == nil && !info.IsDir() {
		http.ServeFile(w, r, p)
		return
	}
	idx := filepath.Join(s.Cfg.StaticDir, "index.html")
	if _, err := os.Stat(idx); err == nil {
		http.ServeFile(w, r, idx)
		return
	}
	http.NotFound(w, r)
}
```

**条件分支:**
- `strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws")` → `404`（API 路径不走 SPA）
- `s.Cfg.StaticDir == ""` → `404`
- `!strings.HasPrefix(p, s.Cfg.StaticDir)` → `404`（路径穿越防护）
- `os.Stat(p) 成功 && !info.IsDir()` → 返回该文件
- `os.Stat(index.html) 成功` → 回退到 `index.html`
- 以上都不满足 → `404`

---

## 辅助方法（Cookie 操作）

### `setAuthCookie(w, r, name, value, path, ttl)`

**目的:** 设置 Access Token Cookie。HttpOnly=true，SameSite=Lax。

**基本方法:** `setAuthCookie(w, r, name, value, path, ttl)` — 调用 `http.SetCookie`。

```go
func setAuthCookie(w http.ResponseWriter, r *http.Request, name, value string, path string, ttl time.Duration) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
}
```

**条件分支:**
- `r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"` → `Secure = true`

### `setRefreshCookie(w, r, value, ttl)`

**目的:** 设置 Refresh Token Cookie。HttpOnly=true，Path=/api/auth/refresh，SameSite=Lax。

**基本方法:** `setRefreshCookie(w, r, value, ttl)` — 调用 `http.SetCookie`。

```go
func setRefreshCookie(w http.ResponseWriter, r *http.Request, raw string, ttl time.Duration) {
    secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
    http.SetCookie(w, &http.Cookie{
        Name:     "refresh_token",
        Value:    raw,
        Path:     "/api/auth/refresh",
        HttpOnly: true,
        Secure:   secure,
        SameSite: http.SameSiteLaxMode,
        MaxAge:   int(ttl.Seconds()),
    })
}
```

**条件分支:** 同 `setAuthCookie` — `r.TLS != nil || X-Forwarded-Proto == https` → `Secure = true`

### `clearRefreshCookie(w, r)`

**目的:** 清除 Refresh Token Cookie。MaxAge=-1。

**基本方法:** `clearRefreshCookie(w, r)` — 调用 `http.SetCookie` 设置空值 + 过期。

```go
func clearRefreshCookie(w http.ResponseWriter, r *http.Request) {
    secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
    http.SetCookie(w, &http.Cookie{
        Name:     "refresh_token",
        Value:    "",
        Path:     "/api/auth/refresh",
        HttpOnly: true,
        Secure:   secure,
        SameSite: http.SameSiteLaxMode,
        MaxAge:   -1,
    })
}
```

**条件分支:** 同 `setAuthCookie` + `MaxAge: -1` 使浏览器立即删除 cookie

### `timeNow()`

**目的:** 返回当前 UTC 时间，用于 token 过期比较。

**基本方法:** `timeNow()` — 调用 `time.Now().UTC()`。返回 `time.Time`。

```go
func timeNow() time.Time { return time.Now().UTC() }
```