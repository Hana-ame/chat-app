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

## Changelog Rules
- Always append new entries to the **end** of `docs/changelog.md`.
- When appending, anchor `edit`'s `oldString` on the **last section heading** (e.g., `## 2026-... 统一前端错误通知通道（第 21 轮）`) to guarantee a unique match — never match a generic line like `- Client build: ✅` that appears multiple times.

## Release (Tag & Build)
- `git tag build-$(git rev-parse --short HEAD)` — tag with short commit hash
- `git push --tags` — triggers CI (go-build + release job only runs on `main` or tags)
