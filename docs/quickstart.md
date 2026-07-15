# Quick Start

## Prerequisites

- Go 1.26+
- Node.js 20+
- Make

## Backend

```bash
cd server
cp .env.example .env    # edit JWT secret
go run ./cmd/chatd/
```

Server starts on `:8080` (configurable via `CHAT_ADDR`).

## Frontend

```bash
cd client
npm install
npm run dev
```

Dev server at `:5173`, proxies `/api` and `/uploads` to `:8080`.

## Build & Run (一体化)

```bash
make build   # builds both frontend + backend
make run     # runs the combined binary
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `CHAT_ADDR` | `:8080` | Listen address |
| `CHAT_DB_PATH` | `chat.db` | SQLite database path |
| `CHAT_JWT_SECRET` | (auto-generated) | HMAC secret for JWT |
| `CHAT_ACCESS_TTL` | `30m` | Access token lifetime |
| `CHAT_REFRESH_TTL` | `8760h` (1 year) | Refresh token lifetime |
| `CHAT_STATIC_DIR` | `../client/dist` | Frontend build output (empty = no SPA serving) |
| `CHAT_BASE_URL` | `""` | Base URL for CORS/cookie |
| `CHAT_UPLOAD_DIR` | `./uploads` | File upload directory |
| `CHAT_MAX_UPLOAD_BYTES` | `20971520` (20 MiB) | Max upload size |
| `LOG_LEVEL` | `INFO` | Log level: `DEBUG`, `INFO`, `WARN`, `ERROR` |
| `WS_ENABLED` | `false` | Enable WebSocket (otherwise SSE only) |

## Test

```bash
# Backend
cd server && go test ./...

# Frontend E2E (requires dev server running)
cd client && npx playwright test
```
