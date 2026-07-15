# Go Server — API Routes

## Framework

- **Router:** `github.com/go-chi/chi/v5`
- **Routes defined in:** `internal/handlers/router.go`

## Middleware Chain

| Middleware | Scope |
|---|---|
| `chi.middleware.RealIP` | Global |
| `chi.middleware.RequestID` | Global |
| `chi.middleware.Recoverer` | Global |
| Request logging (method, path, status, duration, user) | Global |
| CSP security headers | Global |
| `github.com/go-chi/cors` (credentialed, all origins) | Global |
| `chi.middleware.Timeout(30s)` | `/api` group |
| `httprate.LimitByIP(120, 1m)` | `/api` group |
| `httprate.LimitByIP(10, 1m)` | `POST /api/auth/login` |
| `httprate.LimitByIP(5, 1m)` | `POST /api/auth/register` |
| `rateLimitByUser(30, 1m)` | `GET /api/users` |
| `rateLimitByUser(30, 1m)` | `POST /api/chats/{chatID}/messages` |
| `s.authMiddleware` (JWT) | `/api` group (excl. auth/register, auth/login, auth/refresh) |

### Security Headers (CSP)

```
Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline';
  style-src 'self' 'unsafe-inline'; img-src 'self' https://upload.moonchan.xyz data:;
  connect-src 'self' wss://wsl-8080.moonchan.xyz https://upload.moonchan.xyz;
  font-src 'self' data:;
X-Content-Type-Options: nosniff
```

## Authentication

- **Header:** `Authorization: Bearer <token>` (primary)
- **Cookie fallback:** `access_token` httpOnly cookie
- **Query fallback:** `?access_token=<token>` (for SSE/WS; deprecated, will be removed)
- **Algorithm:** HS256 (`github.com/golang-jwt/jwt/v5`)
- **Context injection:** `*models.User` under key `"user"`, raw token under key `"token"`

## Complete Route Table

### Public (No Auth)

| # | Method | Path | Handler | Notes |
|---|--------|------|---------|-------|
| 1 | `GET` | `/healthz` | inline | Returns `{"status":"ok","echo":{headers}}` |
| 2 | `GET` | `/swagger/*` | `httpSwagger.Handler` | Swagger UI (points to `wsl-8080.moonchan.xyz`) |
| 3 | `GET` | `/uploads/*` | `s.serveUpload` | ⚠️ Deprecated — static file server |
| 4 | `GET` | `/*` | `s.serveStatic` | SPA fallback (only when `Cfg.StaticDir != ""`) |

### Auth

| # | Method | Path | Handler | Rate Limit | Request Body |
|---|--------|------|---------|------------|-------------|
| 5 | `POST` | `/api/auth/register` | `s.Register` | 5/min/IP | `registerReq` |
| 6 | `POST` | `/api/auth/login` | `s.Login` | 10/min/IP + login attempt rate limit | `loginReq` |
| 7 | `POST` | `/api/auth/refresh` | `s.Refresh` | — | (reads httpOnly cookie) |
| 8 | `POST` | `/api/auth/logout` | `s.Logout` | — | (optional body) |

### Real-time

| # | Method | Path | Handler | Auth | Notes |
|---|--------|------|---------|------|-------|
| 9 | `GET` | `/ws` | `gateway.ServeHTTP` | `?access_token=` | WebSocket, gorilla/websocket |
| 10 | `GET` | `/api/events` | `s.SSE` | Bearer or `?access_token=` | SSE stream |

### Users

| # | Method | Path | Handler | Rate Limit | Query Params |
|---|--------|------|---------|------------|-------------|
| 11 | `GET` | `/api/users/me` | `s.Me` | — | — |
| 12 | `PATCH` | `/api/users/me` | `s.UpdateMe` | — | — |
| 13 | `GET` | `/api/users` | `s.SearchUsers` | 30/min/user | `q` (min 1 char) |

### Chats

| # | Method | Path | Handler | Path Params |
|---|--------|------|---------|-------------|
| 14 | `GET` | `/api/chats/my` | `s.ListChats` | — |
| 15 | `GET` | `/api/chats/public` | `s.ListPublicChats` | — |
| 16 | `POST` | `/api/chats` | `s.CreateChat` | — |
| 17 | `POST` | `/api/dms` | `s.CreateOrGetDM` | ⚠️ Deprecated |
| 18 | `GET` | `/api/chats/{chatID}` | `s.GetChat` | `chatID` |
| 19 | `PATCH` | `/api/chats/{chatID}` | `s.RenameChat` | `chatID` |
| 20 | `DELETE` | `/api/chats/{chatID}` | `s.DeleteChat` | `chatID` |
| 21 | `POST` | `/api/chats/{chatID}/join` | `s.JoinChat` | `chatID` |
| 22 | `POST` | `/api/chats/{chatID}/pin` | `s.PinChat` | `chatID` |
| 22a | `PATCH` | `/api/chats/{chatID}/pin` | `s.UpdatePinnedChat` | `chatID` |
| 22b | `DELETE` | `/api/chats/{chatID}/pin` | `s.DeletePinnedChat` | `chatID` |
| 23 | `POST` | `/api/chats/{chatID}/pin-toggle` | `s.TogglePin` | `chatID` |
| 24 | `POST` | `/api/chats/{chatID}/pin-read` | `s.MarkPinnedRead` | `chatID` |
| 25 | `POST` | `/api/chats/{chatID}/visit` | `s.VisitChat` | `chatID` |

### Members

| # | Method | Path | Handler | Path Params |
|---|--------|------|---------|-------------|
| 26 | `GET` | `/api/chats/{chatID}/members` | `s.ListMembers` | `chatID` |
| 27 | `POST` | `/api/chats/{chatID}/members` | `s.AddMember` | `chatID` |
| 28 | `DELETE` | `/api/chats/{chatID}/members/{userID}` | `s.RemoveMember` | `chatID`, `userID` |

### Messages

| # | Method | Path | Handler | Path Params | Query Params |
|---|--------|------|---------|-------------|-------------|
| 29 | `GET` | `/api/chats/{chatID}/messages` | `s.ListMessages` | `chatID` | `limit` (int, default 50), `before` (cursor) |
| 30 | `POST` | `/api/chats/{chatID}/messages` | `s.SendMessage` | `chatID` | — |
| 31 | `PATCH` | `/api/chats/{chatID}/messages/{messageID}` | `s.EditMessage` | `chatID`, `messageID` | — |
| 32 | `DELETE` | `/api/chats/{chatID}/messages/{messageID}` | `s.DeleteMessage` | `chatID`, `messageID` | — |
| 33 | `POST` | `/api/chats/{chatID}/read` | `s.MarkRead` | `chatID` | ⚠️ Deprecated (no longer reads body, updates `last_active_at`) |

### Reactions

| # | Method | Path | Handler | Path Params |
|---|--------|------|---------|-------------|
| 34 | `PUT` | `/api/chats/{chatID}/messages/{messageID}/reactions/{emoji}` | `s.AddReaction` | `chatID`, `messageID`, `emoji` |
| 35 | `DELETE` | `/api/chats/{chatID}/messages/{messageID}/reactions/{emoji}` | `s.RemoveReaction` | `chatID`, `messageID`, `emoji` |
| 36 | `GET` | `/api/chats/{chatID}/messages/{messageID}/reactions` | `s.ListReactions` | `chatID`, `messageID` |

### Uploads

| # | Method | Path | Handler | Auth | Input |
|---|--------|------|---------|------|-------|
| 37 | `POST` | `/api/uploads` | `s.Upload` | Required | ⚠️ Deprecated（前端直传 `upload.moonchan.xyz`） |

### Misc

| # | Method | Path | Handler | Notes |
|---|--------|------|---------|-------|
| 38 | `GET` | `/api/version` | `s.VersionHandler` | Returns `{"version":"..."}` |

Upload constraints: PNG, JPEG, GIF, WebP, MP4, WebM, MP3, OGG, WAV, PDF, plain text, ZIP, octet-stream.
