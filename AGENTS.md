# AGENTS.md — AI 代理项目上下文

## 项目
Go 后端 + React 前端的聊天应用。

## 关键路径
- `server/` — Go 后端（chi 路由、SQLite、WebSocket）
- `client/` — React 前端（Vite、Zustand）
- 文档：`README.md` + `docs/`（导航入口 `docs/README.md`；旧文档在 `docs/archive/legacy-20260731/`，只进不出）

## 架构
- `server/internal/handlers/` — 处理器，仅 HTTP 层
- `server/internal/service/` — 业务逻辑、权限、广播
- `server/internal/db/` — 数据访问
- `server/internal/ws/` — WebSocket hub + client

## 说明
- 本文件仅提供初始化上下文。会话日志和修改日志放在 `docs/changelog.md`。

## 本地构建与调试
- 一键全流程: `python scripts/deploy_local.py all`
- 单独编译: `python scripts/deploy_local.py build`
- 单独启动（捕获日志到 server.log）: `python scripts/deploy_local.py start`
- 单独杀掉进程: `python scripts/deploy_local.py kill`
- kill + start（不编译）: `python scripts/deploy_local.py restart`
- 手动编译前端: `cd client && npm ci && npm run build`
- 手动编译后端: `cd server && go build -ldflags="-s -w -X main.Version=dev" -o ../chatd.exe ./cmd/chatd/`
- 手动启动: `./chatd.exe`（需先配置 `.env`，参考 `docs/guide/quickstart.md`）
- 日志: `server.log`（启动后自动写入）

## 测试（权威文档: docs/testing.md）
- 金字塔: Go 包内单元 / testutil 集成(真实 SQLite + httptest) / vitest 单测 / Playwright E2E。
- 提交前必跑: `cd server && go vet ./... && go test ./... -count=1` + `cd client && npm test && npm run build`。
- Go 断言一律 `testkit.Require*`(handlers 内部测试)或 `testutil.Require*`;禁止手写 `if x != y { t.Fatalf }`。
- 新增导出函数必须带测试;改 UI 元素/API 字段必须同步测试(否则会变成死用例)。
- WS 测试默认启用;`WS_ENABLED` 仅是生产关闭开关。
- Mock 三层边界见 `docs/mock-strategy.md`:DB 永不 mock;单测用 vi.mock;应用内建 mock 只服务 dev/E2E。

## 通用原则
- **不要假设字段/配置没用。** 如果一个配置字段（如 `CHAT_BASE_URL` / `CHAT_CSP_CONNECT_SRC`）存在，就有人有理由放它在那。删除或改动前先理解完整数据流。
- **清理前追踪全链路。** 任何对 API 响应字段的"清理"都必须追溯该字段从生产者（handler）经传输格式到每一个消费者（HTML 页面、前端组件、其他服务、脚本）。漏掉一个消费者 = 功能损坏。
- **先 grep 再动手。** 改动任何 API 响应字段前，grep 所有消费者：`client/src/`、`*.html`、`docs/api/reference.md`、`scripts/`。常有消费者在直接代码路径之外。
- **配置优于魔法。** 如果一个值可以从配置（如 `CHAT_BASE_URL`）获得，优先使用配置，而不是通过请求头临时拼凑。配置是显式的，请求头是隐式的且可能随部署拓扑变化。

## API 响应约定
- **上传接口返回的 `url` 字段必须包含绝对 URL**（scheme + host），使用 `CHAT_BASE_URL` 或从请求（`X-Forwarded-Proto` + `Host`）推导。其他 API 响应中的 URL 字段可视情况使用相对路径。
- 改动任何 API 响应字段前，grep 所有消费者（前端 `.jsx`、`.ts`、`.html`、其他服务）。
- 这条规则的存在是因为 v0.8.15 去掉了上传 `url` 响应中的 host，导致 upload.html 的"复制所有链接"功能和其他消费者被破坏。

## 修改日志规则
- 始终在 `docs/changelog.md` 的**末尾**追加新条目。
- 追加时，`edit` 的 `oldString` 锚定在**最后一个章节标题**（例如 `## 2026-... 统一前端错误通知通道（第 21 轮）`）以保证唯一匹配——永远不要匹配像 `- Client build: ✅` 这样出现多次的通用行。

## 部署
- 前端（Cloudflare Pages）: `https://chat.moonchan.xyz`
- 后端 API: `https://chat.moonchan.xyz`（同一域名，反向代理）
- API 版本端点: `GET /api/version`

## CI/CD 工作流（生产发布）
1. 修改代码 → `git add` + `git commit`
2. 如需 bump version，先同步两处版本号：
   - `client/package.json` — `"version": "x.y.z"`
   - `server/internal/handlers/swagger.json` — `"version": "x.y.z"`
3. `git tag v<version>` — 创建版本标签
4. `git push && git push --tags`
5. `gh run watch <run-id> --exit-status` — 等 CI 通过（run ID 从 push 输出获取）
