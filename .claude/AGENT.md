# Chat App — Agent Quick Guide

## What This Is

Discord-style real-time chat app. React 19 frontend + Go backend, SQLite storage, WebSocket/SSE real-time.

## Tech Stack

| Layer | Tech |
|---|---|
| Frontend | React 19, Vite 6, Zustand 5, React Router 7, Playwright |
| Backend | Go 1.26, chi v5, gorilla/websocket, golang-jwt v5, modernc.org/sqlite |
| Auth | bcrypt + JWT (access + refresh tokens), httpOnly cookies |
| Real-time | WebSocket (primary) / SSE (fallback) |
| Database | SQLite (WAL mode, TEXT timestamps, UUID v4 IDs) |

## Project Structure

```
chat-app/
├── server/
│   ├── cmd/chatd/main.go     # Entry point
│   ├── internal/
│   │   ├── handlers/         # HTTP handlers (router, auth, chat, messages, members, reactions, uploads, sse, ratelimit)
│   │   ├── ws/               # WebSocket hub/client/gateway
│   │   ├── db/               # SQLite DAO + migrations
│   │   ├── auth/             # JWT + bcrypt
│   │   ├── config/           # Env config
│   │   ├── models/           # Data models
│   │   ├── logutil/          # Logging (DEBUG/INFO/WARN/ERROR, LOG_LEVEL env)
│   │   ├── orderedmap/       # Ordered JSON map (for /healthz)
│   │   └── testutil/         # Integration test fixtures
│   └── .env.example
├── client/
│   ├── src/
│   │   ├── api/client.js     # HTTP + mock system
│   │   ├── api/mock.js       # In-memory mock (MITM mode)
│   │   ├── components/       # 18 React components
│   │   ├── routes/           # LoginPage, RegisterPage, ChatPage
│   │   ├── store/            # auth.js + chat.js (Zustand)
│   │   ├── dev/              # stream-source.js, dummy.js, mock-ws.js
│   │   └── styles/           # Discord dark CSS
│   └── playwright.config.js
├── docs/
│   ├── README.md             # Documentation index
│   ├── quickstart.md         # Setup / env vars / run
│   ├── changelog.md          # Modification log
│   ├── features/             # Feature specs + API docs
│   ├── reports/              # Architecture specs (14 files)
│   └── reference/            # Cross-cutting refs (security, rate limit, protocol, errors, DB schema)
└── Makefile
```

## Key Commands

```bash
# Backend
cd server && cp .env.example .env && go run ./cmd/chatd/

# Frontend
cd client && npm install && npm run dev

# Build all
make build

# Test backend
cd server && go test ./...

# Test frontend (requires dev server)
cd client && npx playwright test
```

## Architecture Notes

- **Router**: chi v5, routes defined in `server/internal/handlers/router.go`
- **Auth middleware**: Bearer token → cookie → query param (fallback chain)
- **Rate limiting**: Global 120/min, login 10/min, register 5/min, search/send 30/min/user
- **Real-time**: WS primary (`/ws`), SSE fallback (`/api/events`). Same Envelope format for both.
- **Mock**: `api.enableMock()` replaces all HTTP methods with in-memory JS implementations. MITM mode: mock writes to `data` object, reads from it, so polling sees mock-generated AI replies.
- **DB migration**: Single `000__init.sql` (all-in-one, no incremental migrations)
- **Uploads**: Frontend uploads directly to `upload.moonchan.xyz` via PUT. Go `/api/uploads` deprecated.
- **MarkRead**: No longer reads `message_id` body; updates `last_active_at` instead.
- **Deployment**: Cloudflare Pages (frontend) + WSL tunnel (Go backend), or single-binary deployment.

## Documentation Quick Links

- [Quick Start](docs/quickstart.md)
- [API Routes](docs/features/go-api-routes.md)
- [API Models](docs/features/go-api-models.md)
- [Frontend API Endpoints](docs/features/api-endpoints.md)
- [Real-time Protocol](docs/reference/realtime-protocol.md)
- [Error Codes](docs/reference/error-codes.md)
- [Database Schema](docs/reference/database.md)
- [Security](docs/reference/security.md)
- [Rate Limiting](docs/reference/rate-limiting.md)

## Before Editing

1. Read relevant docs first (features/ for behavior, reports/ for deep specs)
2. Check existing patterns in neighboring files
3. Run `go test ./...` or `npm run build` after changes
4. Update `docs/changelog.md` after functional changes or bug fixes
