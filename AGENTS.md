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

## Deployment
- Frontend (Cloudflare Pages): `https://chat.moonchan.xyz`
- Backend API: `https://chat.moonchan.xyz` (same domain, proxied)
- API version endpoint: `GET /api/version`

## Release (Tag & Build)
- `git tag build-$(git rev-parse --short HEAD)` — tag with short commit hash
- `git push --tags` — triggers CI (go-build + release job only runs on `main` or tags)
- After push, run `gh run list --branch <tag> --limit 3` to confirm CI passes

## Workflow (每次修改后)
1. `git add` + `git commit`
2. `git push` (push to remote)
3. `git tag build-$(git rev-parse --short HEAD)` — tag with short commit hash
4. `git push --tags`
5. `gh run list --branch <tag> --limit 3` — confirm CI passes

## Version Bump (bump version 时)
需同步以下三处，确保版本号一致：
- `client/package.json` — `"version": "x.y.z"`
- `server/internal/handlers/swagger.json` — `"version": "x.y.z"`
- git tag — `build-<sha>`（Workflow step 3 自动完成）
