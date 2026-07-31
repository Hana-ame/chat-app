# Real-time Protocol Reference

## Transport

Two transport modes, both deliver the same event format:

- **WebSocket** (`GET /ws`): Primary, bidirectional
- **SSE** (`GET /api/events`): Fallback, unidirectional

两者认证方式相同：`Authorization: Bearer <access_token>` 请求头，或 `access_token` Cookie（浏览器自动携带）。不接受 URL 查询参数传 token（防访问日志 / Referer 泄漏）。

SSE delivers the same events as WS but does not receive client-to-server ops (typing, subscribe, etc.).

## Envelope Format

All messages use the same JSON envelope:

```json
{"op": "message_create", "req_id": 0, "payload": {...}}
```

| Field | Type | Description |
|---|---|---|
| `op` | string | Event type |
| `req_id` | int | Client request ID (echoed for client ops) |
| `payload` | object | Event-specific data |

## Server → Client Events

| Op | Trigger | Payload |
|---|---|---|
| `ready` | Connection established | `{user, chats, online_user_ids}` |
| `message_create` | New message sent | Full `Message` object |
| `message_update` | Message edited | Full `Message` object |
| `message_delete` | Message deleted | `{chat_id, message_id}` |
| `reaction_add` | Reaction added | `{chat_id, message_id, emoji, user_id}` |
| `reaction_remove` | Reaction removed | `{chat_id, message_id, emoji, user_id}` |
| `chat_create` | New chat created | Full `Chat` object |
| `chat_update` | Chat updated | Full `Chat` object |
| `chat_delete` | Chat deleted | `{chat_id}` |
| `chat_remove` | User removed from chat | `{chat_id}` |
| `user_update` | User profile changed | Full `User` object |
| `presence_update` | User online/offline | `{user_id, status}` ("online" \| "offline") |
| `typing` | User typing | `{chat_id, user_id, timestamp}` |
| `error` | Error occurred | `{message}` |

## Client → Server Ops (WebSocket only)

| Op | Payload | Description |
|---|---|---|
| `ping` | — | Keepalive ping |
| `pong` | — | Pong response |
| `subscribe` | `{chat_id}` | Subscribe to chat events |
| `unsubscribe` | `{chat_id}` | Unsubscribe from chat events |
| `typing` | `{chat_id}` | Typing indicator |

## SSE Event Format

```text
id: 0
event: ready
data: {"user":{...},"chats":[...],"online_user_ids":[...]}

id: 1
event: message_create
data: {"op":"message_create","payload":{...}}
```

- `id`: Incremental event ID
- `event`: Always matches the `op` field from the envelope
- `data`: The full envelope JSON
- The `ready` event uses `data` directly (not wrapped in envelope)
