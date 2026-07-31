# Chat App — Agent Quick Guide

## What This Is

Discord-style real-time chat app. React 19 frontend + Go backend, SQLite storage, WebSocket/SSE/poll real-time.

## Tech Stack

| Layer | Tech |
|---|---|
| Frontend | React 19, Vite 6, Zustand 5, React Router 7, Playwright, vitest |
| Backend | Go 1.26, chi v5, gorilla/websocket, golang-jwt v5, modernc.org/sqlite |
| Auth | bcrypt + JWT (access 30m + refresh 1y rotation) |
| Real-time | WebSocket `/ws` / SSE `/api/events` / poll fallback（前端可切换） |
| Database | SQLite (WAL, TEXT timestamps, UUID v4) |

## Project Structure

```
chat-app/
├── server/
│   ├── cmd/chatd/main.go        # Entry point
│   ├── internal/
│   │   ├── handlers/            # HTTP handlers + router.go + swagger.json (embedded)
│   │   ├── service/             # Business logic, authz, broadcast, AI streaming
│   │   ├── ws/                  # WebSocket hub/client/gateway
│   │   ├── db/                  # SQLite DAO + migrations/ (000 init + 001-004)
│   │   ├── ai/                  # AI stream client + SSRF guard (ValidateEndpoint)
│   │   ├── auth/                # JWT + bcrypt
│   │   ├── config/              # Env config (config.go = single source of truth)
│   │   ├── models/ · logutil/ · storage/local/
│   │   ├── testkit/             # Zero-dep test helpers (Require*, NewMockAIServer)
│   │   └── testutil/            # Integration fixture (real SQLite + httptest)
│   └── .env.example
├── client/
│   ├── src/
│   │   ├── api/                 # client.ts (methods + mock Proxy) · mock.js · schemas.ts
│   │   ├── realtime/            # coordinator.js + transports/{ws,sse,poll,mock}.js
│   │   ├── store/               # auth.js · chat.js · notification.js (Zustand)
│   │   ├── components/ · routes/ · hooks/ · utils/ · dev/ · styles/
│   ├── tests/                   # Playwright E2E (mock project + e2e project)
│   └── vitest.config.js         # Unit tests live next to source (src/**/*.test.*)
├── scripts/                     # deploy_local.py (build/start/kill/restart/all)
└── docs/                        # Documentation (see below)
```

## Key Commands

```bash
# Backend
cd server && cp .env.example .env && go run ./cmd/chatd/

# Frontend dev
cd client && npm ci && npx vite --port 5173

# Build all (one-command local flow)
python scripts/deploy_local.py all

# Tests (see docs/testing.md for the full policy)
cd server && go vet ./... && go test ./... -count=1
cd client && npm test && npm run build          # vitest + tsc + vite build
cd client && npx playwright test                # E2E (needs vite :5173; e2e project needs backend :8080)
```

## Architecture Notes

- **Layers**: handlers (HTTP only) → service (business/authz/broadcast) → db (data access); ws hub is a separate connection manager.
- **Auth middleware**: `Authorization: Bearer` or `access_token` cookie.
- **Rate limiting**: global 120/min/IP; upload 60/min; login 10, register 5; users/search & send 30/min/user.
- **Real-time envelope**: `{"op": ..., "req_id": N, "payload": ...}` shared by WS and SSE. SSE ready event carries `event:`/`id:`, all later events are bare `data:` lines.
- **Mock**: `window.__mockLogin()` → `api.enableMock()` (Proxy interception) + `setMode('mock')`. In-memory data in `mock.js`; used for dev without backend and CI mock tests. DB is never mocked in Go tests (real temp SQLite).
- **AI streaming**: client sends `type=stream` message with `src {endpoint, auth_key, body}`; server validates endpoint (SSRF guard, `CHAT_AI_ALLOW_PRIVATE` opt-out).
- **Uploads**: PUT/POST `/api/upload` (no auth, delete via `?delete=<hash>`); files served at `/api/local/*`; response `url`/`delete_url` are absolute.
- **Upload response contract**: `url` MUST be absolute (CHAT_BASE_URL or derived from X-Forwarded-Proto + Host).
- **Deployment**: Cloudflare Pages (frontend) + reverse proxy to Go backend, single domain `chat.moonchan.xyz`.

## Documentation Quick Links

- [docs/README.md](docs/README.md) — index
- [docs/guide/quickstart.md](docs/guide/quickstart.md) — setup / run
- [docs/guide/deployment.md](docs/guide/deployment.md) — env vars / deployment / release workflow
- [docs/guide/development.md](docs/guide/development.md) — dev workflow / mock mode
- [docs/architecture/overview.md](docs/architecture/overview.md) — system overview
- [docs/architecture/backend.md](docs/architecture/backend.md) — backend layers / middleware
- [docs/architecture/frontend.md](docs/architecture/frontend.md) — frontend state / realtime coordinator
- [docs/architecture/database.md](docs/architecture/database.md) — DB schema
- [docs/architecture/realtime.md](docs/architecture/realtime.md) — WS/SSE/poll protocol
- [docs/api/reference.md](docs/api/reference.md) — API reference
- [docs/api/error-codes.md](docs/api/error-codes.md) · [docs/api/rate-limiting.md](docs/api/rate-limiting.md)
- [docs/security.md](docs/security.md)
- [docs/testing.md](docs/testing.md) · [docs/mock-strategy.md](docs/mock-strategy.md)

Archived (do not edit): `docs/archive/legacy-20260731/`.

## Before Editing

1. Read the relevant docs first (guide/ for workflow, architecture/ for design, api/ for contracts).
2. Check existing patterns in neighboring files.
3. After changes: `go test ./...` or `npm test && npm run build` must pass.
4. Append a new entry to `docs/changelog.md` after functional changes or bug fixes.
