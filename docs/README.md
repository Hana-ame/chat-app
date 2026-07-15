# Documentation

This directory contains technical specifications, feature breakdowns, and development reports for the WebChat App.

## ⚠️ Release Checklist (必读)
每次推送前检查：
1. **版本号** — `client/package.json` 的 `version` 是否已递增？有功能性变更或修复时需 bump
2. **构建** — `npm run build` 是否通过
3. **后端测试** — `go test ./...` 是否通过
4. **修改日志** — `docs/modification-log.md` 是否已追加本次变更

## 📁 Structure

### 🚀 [Features](./features/)
Detailed technical implementation of core app functionalities:
- [`ai-stream.md`](./features/ai-stream.md): AI typing effect and promise-based streaming architecture.
- [`chat-list.md`](./features/chat-list.md): Sidebar sorting, styling, and logic.
- [`create-group.md`](./features/create-group.md): Full end-to-end flow for group creation.
- [`search.md`](./features/search.md): Search mechanism (local filter + public channels + ID-based join).
- [`visibility.md`](./features/visibility.md): Definition of visibility levels (public / unlisted / private).
- [`add-member.md`](./features/add-member.md): Member addition and removal mechanism.

### 📊 [Reports](./reports/)
Developer reports and architectural analysis (see [`index.md`](./reports/index.md) for full list):

### 📦 [Archive](./archive/)
Stale, session-specific, and duplicate documents:
- [`handoff.md`](./archive/handoff.md): Previous session handoff (outdated).
- [`log.md`](./archive/log.md): Short change log (superseded by `modification-log.md`).
- [`upload_summary.md`](./archive/upload_summary.md): Temporary upload index (no longer relevant).
- [`conversation-20260707.md`](./archive/conversation-20260707.md): Chat transcripts with the AI agent.
- [`session-20260706.md`](./archive/session-20260706.md): Session summaries.
- [`todo-20260706.md`](./archive/todo-20260706.md): Historical task lists.
- [`todo-uiux.md`](./archive/todo-uiux.md): UI/UX specific task lists.
- **Reports archived:**
  - `db-base-spec-20260709.md` / `db-dao-api-reference-20260708.md`: Superseded by `db-spec-20260710.md`.
  - `frontend-logic-spec-20260710.delta.md`: Redundant summary of corrections.
  - `frontend-update-report-20260707.md`: Earliest report, absorbed into later specs.
  - `upload-urls-20260710.md`: Operational URL list, not substantive.
  - `ws-implementation-spec.md` / `sse-implementation-spec.md`: Undated drafts duplicated by dated specs.
