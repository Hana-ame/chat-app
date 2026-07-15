# AGENTS.md — Project Context for AI Agents

## Project
Chat application with Go backend + React frontend.

## Key Paths
- `server/` — Go backend (chi router, SQLite, WebSocket)
- `client/` — React frontend (Vite, Zustand)

## Commands
- Server: `cd server && go build ./... && go test ./... && go vet ./...`
- Client: `cd client && npm run build`

## Architecture
- Handlers in `server/internal/handlers/` — HTTP layer only
- Service in `server/internal/service/` — business logic, permissions, broadcasts
- DB in `server/internal/db/` — data access
- WS in `server/internal/ws/` — WebSocket hub + client

## Notes
- This file is for initialization context only. Session logs and changelogs go in `docs/changelog.md`.
