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
| `github.com/go-chi/cors` | Global |
| `chi.middleware.Timeout(30s)` | `/api` group |
| `s.authMiddleware` (JWT) | `/api` group (excl. auth/register, auth/login, auth/refresh) |

## Authentication

- **Header:** `Authorization: Bearer <token>` (primary)
- **Cookie fallback:** `access_token` httpOnly cookie
- **Query fallback:** `?access_token=<token>` (for SSE/WS)
- **Algorithm:** HS256 (`github.com/golang-jwt/jwt/v5`)
- **Context injection:** `*models.User` under key `"user"`, raw token under key `"token"`

## Complete Route Table

### Public (No Auth)

| # | Method | Path | Handler | Notes |
|---|--------|------|---------|-------|
| 1 | `GET` | `/healthz` | inline | Returns `{"status":"ok"}` |
| 2 | `GET` | `/uploads/*` | `s.serveUpload` | Static file server |
| 3 | `GET` | `/*` | `s.serveStatic` | SPA fallback (only when `Cfg.StaticDir != ""`) |

### Auth

| # | Method | Path | Handler | Request Body |
|---|--------|------|---------|-------------|
| 4 | `POST` | `/api/auth/register` | `s.Register` | `registerReq` |
| 5 | `POST` | `/api/auth/login` | `s.Login` | `loginReq` |
| 6 | `POST` | `/api/auth/refresh` | `s.Refresh` | `refreshReq` |
| 7 | `POST` | `/api/auth/logout` | `s.Logout` | `refreshReq` (optional) |

### Real-time

| # | Method | Path | Handler | Auth | Notes |
|---|--------|------|---------|------|-------|
| 8 | `GET` | `/ws` | `gateway.ServeHTTP` | `?access_token=` | WebSocket, gorilla/websocket |
| 9 | `GET` | `/api/events` | `s.SSE` | Bearer or `?access_token=` | SSE stream |

### Users

| # | Method | Path | Handler | Query Params |
|---|--------|------|---------|-------------|
| 10 | `GET` | `/api/users/me` | `s.Me` | — |
| 11 | `PATCH` | `/api/users/me` | `s.UpdateMe` | — |
| 12 | `GET` | `/api/users` | `s.SearchUsers` | `q` (min 1 char) |

### Chats

| # | Method | Path | Handler | Path Params |
|---|--------|------|---------|-------------|
| 13 | `GET` | `/api/chats/my` | `s.ListChats` | — |
| 14 | `GET` | `/api/chats/public` | `s.ListPublicChats` | — |
| 15 | `POST` | `/api/chats` | `s.CreateChat` | — |
| 16 | `POST` | `/api/dms` | `s.CreateOrGetDM` | ⚠️ Deprecated |
| 17 | `GET` | `/api/chats/{chatID}` | `s.GetChat` | `chatID` |
| 18 | `PATCH` | `/api/chats/{chatID}` | `s.RenameChat` | `chatID` |
| 19 | `DELETE` | `/api/chats/{chatID}` | `s.DeleteChat` | `chatID` |
| 20 | `POST` | `/api/chats/{chatID}/join` | `s.JoinChat` | `chatID` |
| 21 | `POST` | `/api/chats/{chatID}/pin` | `s.PinChat` | `chatID` |
| 21a | `PATCH` | `/api/chats/{chatID}/pin` | `s.UpdatePinnedChat` | `chatID` |
| 21b | `DELETE` | `/api/chats/{chatID}/pin` | `s.DeletePinnedChat` | `chatID` |


### Members

| # | Method | Path | Handler | Path Params |
|---|--------|------|---------|-------------|
| 23 | `GET` | `/api/chats/{chatID}/members` | `s.ListMembers` | `chatID` |
| 24 | `POST` | `/api/chats/{chatID}/members` | `s.AddMember` | `chatID` |
| 25 | `DELETE` | `/api/chats/{chatID}/members/{userID}` | `s.RemoveMember` | `chatID`, `userID` |

### Messages

| # | Method | Path | Handler | Path Params | Query Params |
|---|--------|------|---------|-------------|-------------|
| 26 | `GET` | `/api/chats/{chatID}/messages` | `s.ListMessages` | `chatID` | `limit` (int), `before` (cursor) |
| 27 | `POST` | `/api/chats/{chatID}/messages` | `s.SendMessage` | `chatID` | — |
| 28 | `PATCH` | `/api/chats/{chatID}/messages/{messageID}` | `s.EditMessage` | `chatID`, `messageID` | — |
| 29 | `DELETE` | `/api/chats/{chatID}/messages/{messageID}` | `s.DeleteMessage` | `chatID`, `messageID` | — |
| 30 | `POST` | `/api/chats/{chatID}/read` | `s.MarkRead` | `chatID` | ⚠️ Deprecated |

### Reactions

| # | Method | Path | Handler | Path Params |
|---|--------|------|---------|-------------|
| 31 | `PUT` | `/api/chats/{chatID}/messages/{messageID}/reactions/{emoji}` | `s.AddReaction` | `chatID`, `messageID`, `emoji` |
| 32 | `DELETE` | `/api/chats/{chatID}/messages/{messageID}/reactions/{emoji}` | `s.RemoveReaction` | `chatID`, `messageID`, `emoji` |

### Uploads

| # | Method | Path | Handler | Auth | Input |
|---|--------|------|---------|------|-------|
| 33 | `POST` | `/api/uploads` | `s.Upload` | Required | ⚠️ Deprecated（前端直传 `upload.moonchan.xyz`） |

Upload constraints: PNG, JPEG, GIF, WebP, MP4, WebM, MP3, OGG, WAV, PDF, plain text, ZIP, octet-stream.
