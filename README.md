# Chat App

A lightweight instant-messaging server with bot support. Built with FastAPI + SQLite + React.

Inspired by QQ / Line / WeChat / Discord — room-based chat, real-time messaging, and an easy bot API.

## Architecture

```
chat-app/
├── server/                  # Python backend
│   ├── main.py              # FastAPI app, routes
│   ├── db.py                # Async SQLite (aiosqlite), write batching
│   ├── room_manager.py      # WebSocket + Long-Poll room manager
│   ├── schema.sql           # Database schema + seed data
│   ├── run.sh               # Start script
│   ├── requirements.txt
│   └── test_server.py       # Pytest tests
├── client/                  # React SPA (Vite)
│   ├── src/
│   │   ├── components/      # Login, Chat, MessageItem
│   │   ├── hooks/           # useChat — WS + polling hybrid
│   │   └── index.css
│   ├── package.json
│   └── vite.config.js
├── build_client.sh          # npm install && npm run build
├── guide.md                 # Bot development guide
└── README.md
```

## Quick Start

### 1. Install dependencies

```bash
pip install -r server/requirements.txt
```

### 2. Build the client (one time)

```bash
bash build_client.sh
```

### 3. Run the server

```bash
bash server/run.sh
```

Open **http://localhost:8000** in your browser.

## Features

- **Room-based chat** — create custom rooms, default "Lobby" included
- **WebSocket** — real-time messaging with auto-reconnect and TCP keepalive
- **Long-poll fallback** — keeps working through proxies and restrictive networks
- **Bot API** — permanent API tokens for building chat bots (see [guide.md](guide.md))
- **Simple auth** — username + password, first login auto-registers
- **SPA** — React frontend served from the same process

## Testing

```bash
pytest server/test_server.py -v
```

Tests run against a temporary SQLite database so they don't affect your data.

## Bot System

1. Log in or register to get a **session token**
2. `POST /api/bot/create` to generate a **permanent bot token**
3. Use the bot token to send/receive messages via REST or WebSocket

Full details and code examples in [guide.md](guide.md).

## API Overview

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/login` | — | Login / auto-register |
| GET | `/api/rooms` | token | List rooms |
| POST | `/api/rooms` | token | Create room |
| GET | `/api/history/{id}` | token | Message history |
| POST | `/api/msg` | token | Send message |
| GET | `/api/poll` | token | Long-poll for messages |
| WS | `/ws/{room_id}` | token | WebSocket real-time |
| POST | `/api/bot/create` | token | Create bot |
| GET | `/api/bot/list` | token | List your bots |
| DELETE | `/api/bot/{id}` | token | Delete a bot |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CHAT_DB_PATH` | `server/chat.db` | SQLite database file path |

## Deployment

The server listens on `0.0.0.0:8000`. For production, put it behind nginx or a Cloudflare Tunnel.

Build the React client before deploying:

```bash
bash build_client.sh
```

The static files are served by FastAPI from `client/dist/`.
