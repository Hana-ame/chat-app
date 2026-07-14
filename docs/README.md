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
Developer reports and architectural analysis:
- [`components-signature.md`](./reports/components-signature.md): Component interface definitions.
- [`frontend-update-report-20260707.md`](./reports/frontend-update-report-20260707.md): Summary of recent UI/UX improvements.
- [`member-presence-analysis.md`](./reports/member-presence-analysis.md): Analysis of the real-time presence system.

### 📦 [Archive](./archive/)
Session logs and legacy todo lists:
- [`conversation-20260707.md`](./archive/conversation-20260707.md): Chat transcripts with the AI agent.
- [`session-20260706.md`](./archive/session-20260706.md): Session summaries.
- [`todo-20260706.md`](./archive/todo-20260706.md): Historical task lists.
- [`todo-uiux.md`](./archive/todo-uiux.md): UI/UX specific task lists.
