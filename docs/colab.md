# colab.md — Session State

## Agent: ren

AI coding assistant specialized in software engineering. Maintains session continuity across conversations.

### Responsibilities
- Implement features and fix bugs across Go backend and React frontend
- Maintain and debug CI/CD pipeline (GitHub Actions, deploy scripts)
- Manage releases: tag, build, deploy
- Write and update documentation (changelog, deployment guides, session state)
- Refactor code, improve architecture, ensure security best practices
- Test via Go unit tests, Playwright E2E, and manual verification
- Keep AGENTS.md and colab.md updated for agent handover

## Agent: rin
AI coding assistant specialized in software engineering. Maintains session continuity across conversations.

### Responsibilities
- Implement features and fix bugs across Go backend and React frontend
- Maintain and debug CI/CD pipeline (GitHub Actions, deploy scripts)
- Manage releases: tag, build, deploy
- Write and update documentation (changelog, deployment guides, session state)
- Refactor code, improve architecture, ensure security best practices
- Test via Go unit tests, Playwright E2E, and manual verification
- Keep AGENTS.md and colab.md updated for agent handover

## Current State
- Local dev server running (PID 14948, `chatd-windows-amd64.exe`, started 14:54)
- Version: v0.8.15 (`/api/version` returns `{"version":"v0.8.15"}`)
- Frontend built at `client/dist/`
- Built from source, not downloaded from release (GitHub download too slow on this network)

## How to Build & Run
```powershell
# Build frontend
cd client; npm ci; npm run build; cd ..

# Build backend
cd server; go build -ldflags="-s -w -X main.Version=dev" -o ../chatd.exe ./cmd/chatd/; cd ..

# Run (requires .env configured)
./chatd.exe

# Kill existing
taskkill /f /im chatd-windows-amd64.exe
```

## Key Decisions Made
- Local builds for debugging instead of CI → deploy cycle
- GH proxy (`gh-proxy.com`) unreliable on this network — should fall back to direct download or local build
- AI stream uses `context.Background()` with 15min hard timeout (not context from HTTP request)
- AI stream routes in separate `ReadTimeout` router group (not `APITimeout` group)
- CI version verification strips quotes: `tr -d '"'` on `go version -m` output

## CI/CD Pipeline (for production release)
1. Modify code → `git add` + `git commit`
2. Sync version: `client/package.json` + `server/internal/handlers/swagger.json`
3. `git tag v<version>`
4. `git push && git push --tags`
5. `gh run watch <run-id> --exit-status`
6. On server: `python scripts/deploy_win.py download` then restart

## Troubleshooting
- If binary won't start, check `.env` has all required vars
- If `chatd.exe` fails with "not a valid Win32 app", it's a corrupted proxy download — build locally instead
- Version embedded via ldflags: `-X main.Version=v0.8.15`
