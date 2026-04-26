# Architecture & Technical Documentation

## Overview

Chat App is an IM-style chat server — room-based, real-time, with a bot API. Two frontends share the same backend:

| Frontend | Entry | Tech |
|----------|-------|------|
| SPA (standalone) | `/` | React + Vite |
| Inline widget | `<script src=".../static/loader.js">` | Preact + CDN Babel |

Backend: **FastAPI** + **aiosqlite** (async SQLite with write batching). One Python process handles REST, WebSocket, and serves static files — no nginx required for dev.

## Directory Structure

```
chat-app/
├── server/
│   ├── main.py           # FastAPI app, all routes, auth
│   ├── db.py             # Database layer (aiosqlite, write batching)
│   ├── room_manager.py   # WebSocket room & long-poll manager
│   ├── schema.sql        # DDL + seed data
│   ├── run.sh            # Start script
│   ├── requirements.txt
│   ├── test_server.py    # Pytest tests (17 cases)
│   ├── static/           # ⬅ NEW: inline widget files
│   │   ├── loader.js     #   Widget bootstrap (CDN loader + JSX transform)
│   │   └── widget.jsx    #   Chat widget React component
│   └── chat.db           # SQLite database (gitignored)
├── client/               # React SPA
│   ├── src/
│   │   ├── App.jsx
│   │   ├── components/   # Login.jsx, Chat.jsx, MessageItem.jsx
│   │   └── hooks/        # useChat.js (WS + polling hybrid)
│   ├── dist/             # Vite build output
│   └── vite.config.js    # Dev proxy to :8000
├── build_client.sh
├── guide.md              # Bot development guide
├── README.md
└── ARCHITECTURE.md       # This file
```

## Database (SQLite)

```
users        ─── id, username, password, avatar_color, created_at
rooms        ─── id, name, description, created_at
messages     ─── id, room_id→rooms, user_id→users, content, msg_type, created_at
bot_tokens   ─── id, user_id→users, name, token, created_at
```

Key design choices:
- **WAL mode** — high-concurrency reads (multiple readers, one writer)
- **Write batching** — `INSERT` statements are queued and flushed every 200ms in batches of up to 50, giving ~5000 writes/sec
- **Foreign keys** — enforced, rooms/messages/users linked
- **Seed data** — room "大厅" (Lobby) and user "系统通知" (System) created at first start

## Backend Architecture

### 1. Server Lifecycle (`lifespan`)

```
STARTUP:
  db.init()           → open SQLite, run schema.sql, start flush loop
  load_bot_cache()    → load all bot tokens into memory
  _cleanup_task()     → background task: delete expired sessions every 10 min

SHUTDOWN:
  _cleanup_task.cancel()
  db.close()          → drain write queue, commit, close
```

### 2. Authentication

**Regular users**: `POST /api/login` → returns `token` (UUID hex). Stored in `active_tokens` dict with 24h TTL.

**Bots**: `POST /api/bot/create` → returns `token` prefixed with `bot_`. Stored in `bot_tokens_cache` dict (loaded from DB at startup).

`get_user(token)` checks both dictionaries:
```python
def get_user(token):
    u = active_tokens.get(token)        # session token
    if u: return {**u, "is_bot": False}
    b = bot_tokens_cache.get(token)     # bot token
    if b: return b                      # has is_bot=True
    return None
```

### 3. Message Flow

```
Client sends message:
  ┌─ REST  → POST /api/msg       → room_manager.send_message()
  └─ WS    → {"type":"message"}  → room_manager.send_message()
                                    │
                                    ├─ db.save_message() → SQLite INSERT
                                    │
                                    └─ room._broadcast()
                                       ├─ all WS clients    → send_bytes()
                                       └─ all poll clients  → event.set()
```

### 4. Connection Modes

**WebSocket** (primary):
- Connect: `ws://host:8000/ws/{room_id}?token=...`
- TCP keepalive: idle=60s, interval=10s, probes=3
- 5-minute inactivity timeout → disconnect
- Heartbeat: client sends `{"type":"ping"}`, server replies `{"type":"pong"}`

**Long Poll** (fallback):
- `GET /api/poll?room_id=&token=&after_id=&timeout=30`
- Blocks up to 30s waiting for new messages
- Uses `asyncio.Event` for instant wake on new messages
- Returns empty list on timeout

### 5. Room Manager

```
RoomManager
  ├── rooms: dict[int, Room]
  │
  └── Room (per room_id)
      ├── ws_connections: dict[WebSocket, WSConnection]
      ├── poll_clients: dict[str, PollClient]
      ├── last_msg_id: int
      └── online_count: len(ws) + len(poll)
```

- `ws_join()` / `ws_leave()` — broadcast system messages (join/leave), update online count
- `send_message()` — save to DB, set `last_msg_id`, broadcast to all peers
- Dead WebSocket connections are cleaned up lazily during broadcast

### 6. Bot System

Bots are API keys tied to user accounts:

```python
# Create
POST /api/bot/create  {"token": "<user_token>", "name": "MyBot"}
→ {"id": 1, "name": "MyBot", "token": "bot_<hex>"}

# List
GET /api/bot/list?token=<user_token>
→ {"bots": [...]}

# Delete
DELETE /api/bot/{id}?token=<user_token>
→ {"ok": true}

# Use (send message)
POST /api/msg {"token": "bot_<hex>", "content": "hello"}
→ {..., "is_bot": true, "username": "MyBot"}
```

Key properties:
- Bot tokens are **permanent** (persisted in DB, cached in memory)
- Bots display with robot emoji (SPA) or `[Bot]` prefix (system messages)
- Bots share the user's `avatar_color`
- Bots **cannot** create sub-bots (recursive prevention)
- Deleted bot tokens are removed from cache immediately

### 7. Inline Widget System

Reference: https://github.com/Hana-ame/inline-chat-room

The widget lets any website add a chat room with one `<script>` tag:

```html
<script src="https://wsl-8000.moonchan.xyz/static/loader.js"></script>
```

**How it works:**

```
1. Browser loads <script src=".../loader.js">
2. loader.js extracts its own origin → API_HOST
3. Loads React + ReactDOM + Babel from unpkg CDN
4. Fetches widget.jsx from API_HOST/static/widget.jsx
5. Babel transforms JSX → JS in-browser
6. Renders floating bubble + chat window as React
7. Widget uses API_HOST for all REST/poll requests
```

**Widget features:**
- Draggable floating ball (click to open/close)
- Login with username (auto-register), token stored in localStorage
- Default room: "大厅" (Lobby)
- Messages polled every 2s via `/api/poll`
- Bot detection (robot emoji)
- Mobile-responsive (full-screen mode)
- Load history on scroll-to-top

**CORS requirement**: All `/api/*` endpoints must return `Access-Control-Allow-Origin: *` for the widget to work cross-origin. Added via middleware.

## API Reference

All endpoints require `token` (except `/api/login`). `token` can be passed as query param or JSON body field.

| Method | Path | Auth | Body/Params | Response |
|--------|------|------|-------------|----------|
| POST | `/api/login` | — | `{username, password}` | `{token, user_id, username, avatar_color}` |
| GET | `/api/rooms` | token | `?token=` | `{rooms: [{id, name, description}]}` |
| POST | `/api/rooms` | token | `{token, name, description?}` | `{id, name, description}` |
| GET | `/api/history/{room_id}` | token | `?token=&after_id=&limit=` | `{messages: [...]}` |
| POST | `/api/msg` | token | `{token, room_id?, content}` | `{id, room_id, user_id, username, content, is_bot, created_at}` |
| GET | `/api/poll` | token | `?room_id=&token=&after_id=&timeout=` | `{messages, last_id}` |
| WS | `/ws/{room_id}` | token | `?token=` | Real-time message stream |
| POST | `/api/bot/create` | token | `{token, name}` | `{id, name, token}` |
| GET | `/api/bot/list` | token | `?token=` | `{bots: [{id, name, token, created_at}]}` |
| DELETE | `/api/bot/{id}` | token | `?token=` | `{ok: true}` |

## Testing

```bash
pytest server/test_server.py -v    # 17 tests
```

Test infrastructure:
- **TestClient** from Starlette (sync, handles lifespan)
- **Session-scoped fixture** — one DB init for all tests
- **CHAT_DB_PATH** env var → temp file (isolation from production DB)
- Coverage: login/register, auth errors, rooms CRUD, message send/history, bot CRUD, bot auth, bot send, bot delete+cache eviction, long poll timeout

## Deployment

### Development
```bash
pip install -r server/requirements.txt
bash build_client.sh
bash server/run.sh          # → http://0.0.0.0:8000
```

### Production

```bash
# Build frontend
bash build_client.sh

# Run behind nginx or Cloudflare Tunnel
# The server serves client/dist/ as static files
# Set environment:
#   CHAT_DB_PATH=/data/chat.db   (default: server/chat.db)
```

### Cloudflare Pages Deployment

`其他设置.txt` mentions: push to deploy at `https://chat-app-fastapi.pages.dev/`

The SPA is served from `client/dist/`. Cloudflare Pages auto-deploys on push.
The Python backend runs separately (not on Pages — only static files).

### Reverse Proxy

```nginx
# Example nginx config for wsl-8000.moonchan.xyz
server {
    location / {
        proxy_pass http://127.0.0.1:8000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

The `Upgrade` + `Connection` headers enable WebSocket passthrough.
