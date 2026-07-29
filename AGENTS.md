# AGENTS.md — Project Context for AI Agents

## Project
Chat application with Go backend + React frontend.

## Key Paths
- `server/` — Go backend (chi router, SQLite, WebSocket)
- `client/` — React frontend (Vite, Zustand)

## Architecture
- Handlers in `server/internal/handlers/` — HTTP layer only
- Service in `server/internal/service/` — business logic, permissions, broadcasts
- DB in `server/internal/db/` — data access
- WS in `server/internal/ws/` — WebSocket hub + client

## Notes
- This file is for initialization context only. Session logs and changelogs go in `docs/changelog.md`.

## Local Build & Debug
- 一键全流程: `python scripts/deploy_local.py all`
- 单独编译: `python scripts/deploy_local.py build`
- 单独启动（捕获日志到 server.log）: `python scripts/deploy_local.py start`
- 单独杀掉进程: `python scripts/deploy_local.py kill`
- kill + start（不编译）: `python scripts/deploy_local.py restart`
- 手动编译前端: `cd client && npm ci && npm run build`
- 手动编译后端: `cd server && go build -ldflags="-s -w -X main.Version=dev" -o ../chatd.exe ./cmd/chatd/`
- 手动启动: `./chatd.exe`（需先配置 `.env`，参考 `LOCAL_DEPLOYMENT.md`）
- 日志: `server.log`（启动后自动写入）

## Changelog Rules
- Always append new entries to the **end** of `docs/changelog.md`.
- When appending, anchor `edit`'s `oldString` on the **last section heading** (e.g., `## 2026-... 统一前端错误通知通道（第 21 轮）`) to guarantee a unique match — never match a generic line like `- Client build: ✅` that appears multiple times.

## Deployment
- Frontend (Cloudflare Pages): `https://chat.moonchan.xyz`
- Backend API: `https://chat.moonchan.xyz` (same domain, proxied)
- API version endpoint: `GET /api/version`

## CI/CD Workflow (production release)
1. 修改代码 → `git add` + `git commit`
2. 如需 bump version，先同步两处版本号：
   - `client/package.json` — `"version": "x.y.z"`
   - `server/internal/handlers/swagger.json` — `"version": "x.y.z"`
3. `git tag v<version>` — 创建版本标签
4. `git push && git push --tags`
5. `gh run watch <run-id> --exit-status` — 等 CI 通过（run ID 从 push 输出获取）
