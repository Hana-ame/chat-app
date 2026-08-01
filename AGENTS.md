# AGENTS.md — AI 代理项目上下文

Go 后端（chi + SQLite + WS/SSE）+ React 前端（Vite + Zustand）的实时聊天应用。

## 首轮会话仪式（新会话必做）

1. 读 `README.md`（产品概览）
2. 读 `docs/README.md`（文档树索引，唯一权威入口）
3. 读 `docs/changelog.md` **末尾 3 条**（最近改了什么，避免重做）
4. 需要时按索引深入 `docs/architecture/`、`docs/api/`、`docs/testing.md`、`docs/mock-strategy.md`

## 关键路径

- `server/` — Go 后端（`internal/handlers` HTTP 层 → `internal/service` 业务/权限/广播 → `internal/db` 数据访问 → `internal/ws` hub+gateway；另有 `ai`/`auth`/`config`/`storage`/`models`/`logutil`）
- 路由注册真相在 `server/internal/handlers/router.go`；改 API 端点必须同步 `server/internal/handlers/swagger.json`
- 前端字段契约：后端 `models`（JSON）↔ `client/src/api/schemas.ts`（zod 校验）↔ `client/src/api/mock.js` + `dev/dummy.js`（mock 数据）三方同步
- `client/` — React 前端（`src/api` 请求层 + zod schema → `src/store` Zustand → `src/components` → `src/realtime` 四传输协调器）
- 文档：`README.md` + `docs/`（索引 `docs/README.md`；旧文档 `docs/archive/legacy-20260731/`，只进不出）
- 根目录运行产物（`chatd.exe`/`chat.db`/`server.log` 等）已 gitignore，可忽略

## 构建与调试

- 一键全流程: `python scripts/deploy_local.py all`（另: build/start/kill/restart）
- 手动前端: `cd client && npm ci && npm run build`；手动后端: `cd server && go build -ldflags="-s -w -X main.Version=dev" -o ../chatd.exe ./cmd/chatd/`
- 启动: `./chatd.exe`（需 `.env`，模板 `.env.example`，以 `server/internal/config/config.go` 为准）
- 日志: `server.log`

## 测试（权威: docs/testing.md）

- 金字塔: Go 包内单测 / testutil 集成(真实 SQLite + httptest) / vitest / Playwright E2E（mock 与 full 两 project，webServer 托管 vite）
- **验证策略:不用本地测试,以 GitHub Actions 为准**——本地环境(WSL 挂载盘、
  后台进程回收、代理)跑 Playwright 易误判;代码改动提交并 push 后,用
  `gh run watch <run-id> --exit-status` 盯 CI(CI = Go 全量测试 + 前端构建 +
  vitest + mock/full E2E)直到 success。本地只允许用于编译/快速反馈
  (`go build`、`npx tsc --noEmit`),不做全量测试验证
- Go 断言一律 `testkit.Require*` 或 `testutil.Require*`，禁手写 `if x != y { t.Fatalf }`（存量增量迁移中）
- 新增导出函数必须带测试；改 UI 元素/API 字段必须同步测试
- Mock 三层边界: `docs/mock-strategy.md`

## 通用原则

- **不要轻信 subagent 的结论**:Task 工具派生的 subagent 输出可能过时或
  想当然,涉及关键决策/修改时必须亲自读代码验证(subagent 只用来缩小
  搜索范围,不当事实来源)
- **先 grep 再动手**：改动任何 API 响应字段/配置前，grep 全部消费者（`client/src/`、`*.html`、`docs/api/reference.md`、`scripts/`）
- **配置优于魔法**：能用 `CHAT_BASE_URL` 等配置就不用请求头临时拼凑
- **上传 `url` 字段必须绝对 URL**（`CHAT_BASE_URL` 或 `X-Forwarded-Proto`+`Host` 推导）——v0.8.15 曾因去掉 host 破坏 upload.html
- **不要假设字段/配置没用**：删除前先理解完整数据流

## 修改日志规则

- 代码改动后必须在 `docs/changelog.md` **末尾**追加条目
- 追加时 `edit` 的 `oldString` 锚定在**最后一个章节标题**（如 `## 2026-...`）保证唯一匹配，不要匹配通用行
