# API Endpoints & Architecture

This document describes how the frontend communicates with the backend and lists all API endpoints.

## 🌐 Base URL & Environment Handling

```javascript
const IS_PAGES = (typeof window !== 'undefined' && window.location.hostname.endsWith('pages.dev'));
const API_BASE = IS_PAGES ? 'https://wsl-8080.moonchan.xyz' : '';
```

- **Production (Cloudflare Pages)**: `https://wsl-8080.moonchan.xyz`
- **Local Development**: empty string, uses Vite proxy to `http://localhost:8080`

## 🛠 Development Proxy (Vite)

| Prefix | Target |
|---|---|
| `/api` | `http://localhost:8080` |
| `/uploads` | `http://localhost:8080` |

## 📤 External Upload Service

- **Base**: `https://upload.moonchan.xyz`
- **Method**: `PUT`
- **Payload**: Raw binary stream
- **Return URL**: `https://upload.moonchan.xyz/api/{id}/{filename}`

## 📡 实时通信

**主协议:** WebSocket (`ws://` / `wss://`) — `GET /ws?access_token={token}`
**降级协议:** SSE — `api.sseUrl(token)` → `API_BASE + '/api/events?access_token={token}'`

> 前端优先使用 WebSocket；仅在 mock 模式或 WS 不可用时降级到 SSE。

---

## 📋 Complete API Reference

All standard API calls pass through the `request(method, path, token, body?)` helper. All responses expect JSON. `401` triggers the `auth:unauthorized` custom event.

### Auth

| Method | Endpoint | Parameters | Token | Body |
|---|---|---|---|---|
| `POST` | `/api/auth/register` | — | — | `{ email, username, password }` |
| `POST` | `/api/auth/login` | — | — | `{ email, password }` |
| `POST` | `/api/auth/refresh` | — | — | (读取 httpOnly cookie) |
| `POST` | `/api/auth/logout` | — | `token` | — |

### Users

| Method | Endpoint | Parameters | Token | Body |
|---|---|---|---|---|
| `GET` | `/api/users/me` | — | `token` | — |
| `PATCH` | `/api/users/me` | — | `token` | `{...data}` |
| `GET` | `/api/users?q={query}` | `q`: search keyword | `token` | — |

### Chats

| Method | Endpoint | Parameters | Token | Body |
|---|---|---|---|---|
| `GET` | `/api/chats/my` | — | `token` | — |
| `GET` | `/api/chats/public` | `page` (int, default 1), `limit` (int, default 20) | `token` | — |
| `POST` | `/api/chats` | — | `token` | `{ type: "group", name, member_ids[], visibility: "public" \| "unlisted" \| "private" }` |
| `GET` | `/api/chats/{id}` | `id`: chat ID | `token` | — |
| `DELETE` | `/api/chats/{id}` | `id`: chat ID | `token` | — |
| `PATCH` | `/api/chats/{id}` | `id`: chat ID | `token` | `{ name }` |
| `POST` | `/api/dms` | — | `token` | `{ user_id }` | ⚠️ Deprecated |
| `POST` | `/api/chats/{id}/join` | `id`: chat ID | `token` | — |
| `POST` | `/api/chats/{id}/announcement` | `id`: chat ID | `token` | `{ content: "..." }` |
| `PATCH` | `/api/chats/{id}/announcement` | `id`: chat ID | `token` | `{ content: "..." }` |
| `DELETE` | `/api/chats/{id}/announcement` | `id`: chat ID | `token` | — |
| `POST` | `/api/chats/{id}/announcement/read` | `id`: chat ID | `token` | — |
| `POST` | `/api/chats/{id}/pin` | `id`: chat ID | `token` | — |
| `POST` | `/api/chats/{id}/unpin` | `id`: chat ID | `token` | — |
| `POST` | `/api/chats/{id}/visit` | `id`: chat ID | `token` | — |

### Members

| Method | Endpoint | Parameters | Token | Body |
|---|---|---|---|---|
| `GET` | `/api/chats/{chatId}/members` | `chatId` | `token` | — |
| `POST` | `/api/chats/{chatId}/members` | `chatId` | `token` | `{ user_id }` |
| `DELETE` | `/api/chats/{chatId}/members/{userId}` | `chatId`, `userId` | `token` | — |

### Messages

| Method | Endpoint | Parameters | Token | Body |
|---|---|---|---|---|
| `GET` | `/api/chats/{chatId}/messages` | `limit` (int, default 50, max 100), `before` (cursor) | `token` | — |
| `POST` | `/api/chats/{chatId}/messages` | `chatId` | `token` | `{ content, attachments[] }` ⚠️ 附件 URL 必须在 `upload.moonchan.xyz`，否则 400；返回 **201** |
| `PATCH` | `/api/chats/{chatId}/messages/{msgId}` | `chatId`, `msgId` | `token` | `{ content }` |
| `DELETE` | `/api/chats/{chatId}/messages/{msgId}` | `chatId`, `msgId` | `token` | — |
| `POST` | `/api/chats/{chatId}/read` | `chatId` | `token` | `{}` ⚠️ Deprecated（不再读 body，改为更新 `last_active_at`） |

### Reactions

| Method | Endpoint | Parameters | Token | Body |
|---|---|---|---|---|
| `PUT` | `/api/chats/{chatId}/messages/{msgId}/reactions/{emoji}` | `chatId`, `msgId`, `emoji` | `token` | — |
| `DELETE` | `/api/chats/{chatId}/messages/{msgId}/reactions/{emoji}` | `chatId`, `msgId`, `emoji` | `token` | — |
| `GET` | `/api/chats/{chatId}/messages/{msgId}/reactions` | `chatId`, `msgId` | `token` | — |

### Uploads

| Method | Endpoint | Parameters | Token | Body |
|---|---|---|---|---|
| `PUT` | `https://upload.moonchan.xyz/api/upload` | — | — | Raw binary (`file`) |
| `POST` | `/api/uploads` | — | `token` | multipart `file` ⚠️ Deprecated（Go 端保留，前端不再调用） |

### Application API (non-HTTP)

The `api.startStreaming(source)` method handles streaming message sources:

```typescript
startStreaming(source: 
  | (emit: (chunk: string) => void) => Promise<void>    // custom function
  | { type: 'mock', fn: ... }                           // mock AI reply
  | { type: 'sse', url: string }                        // Server-Sent Events
)
```

---

## 🔄 Mock System

When mock is enabled, API 方法被替换为内存态实现（`client/src/api/mock.js`）。Mock 模式下所有 handler 绕过 HTTP，直接操作 `data` 内存对象。

**已知与 Go API 的差异（详见 `docs/mock-vs-go-api-report.md`）：**
- Mock 不校验密码（任意密码可登录）
- Mock 无 attachment URL 校验（Go 强制 `upload.moonchan.xyz`）
- Mock `SendMessage` 50% 概率触发 AI 自动回复（Go 无此逻辑）
- Mock 无 presence 在线状态广播
- Mock `MarkRead` 无 membership/message_id 校验

| Original | Mock Replacement |
|---|---|
| `api.listChats` | `mockListChats()` — returns `{ chats: [...] }` from dummy data |
| `api.listMessages` | `mockListMessages(token, chatId, before?, limit?)` — paginated from dummy data |
| `api.sendMessage` | `mockSendMessage(token, chatId, content, attachments?)` — creates user msg, 50% chance AI reply |

**新增 mock 函数（与 Go 对齐）：** `mockTogglePin`, `mockMarkPinnedRead`, `mockGetReactions`, `mockUploadAvatar`, `mockVisitChat`。

Helpers: `api.enableMock()`, `api.disableMock()`, `api.isMockEnabled()`, `resetMockData()`
