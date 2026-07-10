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
| `GET` | `/api/chats/public` | — | `token` | — |
| `POST` | `/api/chats` | — | `token` | `{ type: "group", name, member_ids[], visibility: "public" \| "unlisted" \| "private" }` |
| `GET` | `/api/chats/{id}` | `id`: chat ID | `token` | — |
| `DELETE` | `/api/chats/{id}` | `id`: chat ID | `token` | — |
| `PATCH` | `/api/chats/{id}` | `id`: chat ID | `token` | `{ name }` |
| `POST` | `/api/dms` | — | `token` | `{ user_id }` | ⚠️ Deprecated |
| `POST` | `/api/chats/{id}/join` | `id`: chat ID | `token` | — |
| `POST` | `/api/chats/{id}/pin` | `id`: chat ID | `token` | `{ content: "..." }` |
| `PATCH` | `/api/chats/{id}/pin` | `id`: chat ID | `token` | `{ content: "..." }` |
| `DELETE` | `/api/chats/{id}/pin` | `id`: chat ID | `token` | — |

### Members

| Method | Endpoint | Parameters | Token | Body |
|---|---|---|---|---|
| `POST` | `/api/chats/{chatId}/members` | `chatId` | `token` | `{ user_id }` |
| `DELETE` | `/api/chats/{chatId}/members/{userId}` | `chatId`, `userId` | `token` | — |

### Messages

| Method | Endpoint | Parameters | Token | Body |
|---|---|---|---|---|
| `GET` | `/api/chats/{chatId}/messages` | `limit` (default 50), `before` (cursor), `details` (bool) | `token` | — |
| `POST` | `/api/chats/{chatId}/messages` | `chatId` | `token` | `{ content, attachments[] }` ⚠️ 附件 URL 必须在 `upload.moonchan.xyz`，否则 400；返回 **201** |
| `PATCH` | `/api/chats/{chatId}/messages/{msgId}` | `chatId`, `msgId` | `token` | `{ content }` |
| `DELETE` | `/api/chats/{chatId}/messages/{msgId}` | `chatId`, `msgId` | `token` | — |
| `POST` | `/api/chats/{chatId}/read` | `chatId` | `token` | `{ message_id }` ⚠️ Deprecated |

### Reactions

| Method | Endpoint | Parameters | Token | Body |
|---|---|---|---|---|
| `PUT` | `/api/chats/{chatId}/messages/{msgId}/reactions/{emoji}` | `chatId`, `msgId`, `emoji` | `token` | — |
| `DELETE` | `/api/chats/{chatId}/messages/{msgId}/reactions/{emoji}` | `chatId`, `msgId`, `emoji` | `token` | — |

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

When mock is enabled, three methods are overridden:

| Original | Mock Replacement |
|---|---|
| `api.listChats` | `mockListChats()` — returns `{ chats: [...] }` from dummy data |
| `api.listMessages` | `mockListMessages(token, chatId, before?, limit?)` — paginated from dummy data |
| `api.sendMessage` | `mockSendMessage(token, chatId, content, attachments?)` — creates user msg + delayed AI reply via `source.type: 'mock'` |

Helpers: `api.enableMock()`, `api.disableMock()`, `api.isMockEnabled()`
