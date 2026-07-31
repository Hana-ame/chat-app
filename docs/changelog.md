# 修改日志

从 v0.9.4 起。旧日志见 `docs/archive/legacy-20260731/changelog-20260731.md`。

规则：**始终在末尾追加新条目**，不修改历史。

## 2026-08-01 文档系统翻新（v0.9.4）

### 变更
- 旧文档全部归档至 `docs/archive/legacy-20260731/`（README/NOTE/LOCAL_DEPLOYMENT/api.md/changelog/features/reference/reports/colab/todo/ppt 等 60+ 文件）。
- 新文档结构：
  - `README.md` — 项目总览（功能/技术栈/目录/快速开始）
  - `docs/guide/` — quickstart（快速开始）、deployment（生产部署+发布）、development（开发工作流）
  - `docs/architecture/` — overview / backend / frontend / database / realtime
  - `docs/api/` — reference（端点全表）/ error-codes / rate-limiting
  - `docs/security.md` — CSP/CORS/JWT/上传/SSRF/IP
  - `docs/changelog.md` — 新日志（从本条目起）
- 修正旧文档中已失效的信息：
  - 删除已不存在的 `UploadPublicURL`、`CHAT_AI_*`（AI 端点改由消息 `src` 携带，服务端仅 SSRF 校验）
  - 修正 `/api/chats` → `/api/chats/my`、`POST /api/uploads` → `/api/upload`、`/uploads/*` → `/api/local/*`
  - 修正 access token 时长（15 分钟 → 默认 30m）、上传响应（201 → 200，含 `path`/`url`/`delete_url`）
  - 修正 SSE 行格式（仅 ready 带 `event:`/`id:`，其余裸 `data:`）
  - 补全限速表（upload 组 60/min）、数据库动态补列（ensureColumn）、迁移 001-004
  - 前端架构更新为现状（api 层 TS 化、realtime/ 协调器、Mock 模式、Notifications 置顶）
- `.env.example` 与 `config.go` 对齐（移除 AI_*/UPLOAD_PUBLIC_URL，新增 CHAT_AI_ALLOW_PRIVATE/CHAT_CSP_CONNECT_SRC/CHAT_UPLOAD_SALT/CHAT_MAX_UPLOAD）。

### 验证
- 文档链接交叉验证（README ↔ docs 导航 ↔ 各文档）：✅
- 内容与代码核对（router.go / config.go / migrations / sse.go / hub.go / local_upload.go）：✅

## 2026-08-01 测试体系大规模翻新（第 23 轮）

### 结构体系
- 新建 `internal/testkit/` 零依赖叶子包:断言助手 `Require*`(assert.go) +
  可复用假 AI 上游 `NewMockAIServer`(mockai.go);`testutil` 薄转发保持
  `testutil.Require*` 兼容,handlers 内部测试改直接 import testkit 消除 import cycle。
- 新增权威文档 `docs/testing.md`(金字塔/命令/命名/断言/注释规范/同步审计)与
  `docs/mock-strategy.md`(四层 mock 边界);`docs/README.md` 增加"测试"导航分区;
  `guide/development.md` 测试章节改为指向新文档;`AGENTS.md` 补"测试"小节。

### Go 侧
- 补缺口:新增 `storage/local` 测试(14 例:Put/Get/Head/Delete/路径穿越/ETag/
  SafeContentType 白名单)、`models` 测试(12 例:JSON 契约/omitempty/指针/
  TokenHash 不泄露)、`handlers` 函数级单测(14 例:上传净化/类型校验/hash/
  BaseURL 推导/VersionHandler/中间件 401)。
- **修复 SafeContentType 大小写敏感**(`IMAGE/PNG` 之前会被错误降级为
  octet-stream,现按 MIME 规范小写归一化)。
- **WS 测试解锁**:去掉 `WS_ENABLED` 门控默认启用;`dialWS` 改走
  Authorization 头;补全 TestWSPresence/TestWSTyping 的真实断言。
- **发现并修复测试协议 bug**:旧测试 typing/subscribe 把 `chat_id` 发在
  Envelope 顶层,服务端要求嵌套 `payload`,字段被静默丢弃——typing 从未
  真正发送过(旧测试无断言所以假绿)。
- **顺带修复生产 bug**:`client/src/realtime/transports/ws.js` 的
  `sendTyping`/`subscribe` 同样把 `chat_id` 放顶层,typing 指示器在生产
  环境从未生效;已按 `{op, payload:{chat_id}}` 格式修正。
- 全部既有测试文件补齐文件头 doc 注释(测什么/怎么跑/依赖什么)。
- service_test.go 的 `startMockAIStream` 委托给 `testkit.NewMockAIServer`。

### JS 侧
- 引入 vitest(vitest.config.js,只扫 `src/**/*.test.{js,ts}`,与 E2E 隔离),
  npm scripts 重排:`test`=vitest、`test:e2e:mock`/`test:e2e:full` 走
  playwright projects。
- 新增单测 55 例:utils/ai.js(10:SSE 解析/chunk 跨边界/Abort/cancel)、
  schemas.ts(7:zod 契约)、store/chat.js(26:排序/乐观消息/reaction/
  合并/回滚)、api/client.ts(12:401→refresh 重试/并发防重入/429/
  upload URL 兜底)。
- **修复 buildUploadUrl 边缘 bug**:path 前导斜杠产生双斜杠 `//`、path
  缺失拼出 `"undefined"` 字符串;现去前导斜杠并兜底空 path。
- E2E 清理:删除 `boundary.spec.mjs`(5 例空断言假绿)与孤儿
  `boundary-runner.mjs`;超长消息(413 content_too_long)与越权(非 owner
  改公告/删群 403)两个边界场景迁入 `e2e.spec.mjs` 用真实后端 API 断言;
  删除 `ci.spec.mjs` 的 `test.skip` 死用例;`playwright.config.js` 落地
  projects(mock/e2e)并清除过期注释;重写 `tests/README.md`。

### CI
- ci.yml:go-test 启用 WS 测试并提 timeout 至 180s;frontend-build 增加
  vitest 步骤,移除依赖不存在 project 且吞失败的
  `npx playwright test --project=chromium || echo "E2E skipped"`。
- frontend-ci.yml:新增独立 `unit-test` job(vitest);mock-test 与 full-e2e
  改用新 npm scripts。

### 验证
- `cd server && go vet ./... && go test ./... -count=1`:14 包全绿(含 WS)。
- `cd client && npx vitest run`:4 文件 55 例全绿。
- `npm run build`(tsc --noEmit + vite build):通过。
- 遗留:400+ 存量 Go 断言尚未全部迁移到 Require*(增量迁移,新代码强制)。

## 2026-08-01 剩余文档翻新：AGENT 指南 / 前端源码 README / 索引补全

### 变更
- 重写 `.claude/AGENT.md`（英文快速指南）：对齐新文档结构（docs/guide|architecture|api）、当前代码（api 层 TS 化、realtime/、testkit/testutil 分工、AI 流式 src 机制、上传契约），删除已不存在的 Makefile/upload.moonchan.xyz/old-path 引用。
- 重写 `client/src/README.md`：目录结构更新为现状（api/client.ts + mock.js + schemas.ts、realtime/coordinator + transports、store 三件套、18 个组件、hooks/utils），补充 API 层与实时层说明。
- 归档 `client/src/COMPOSITION_REVIEW.md`（2026-07 组件评审快照，组件清单已过时）→ `docs/archive/legacy-20260731/`。
- `docs/README.md` 索引纳入 `testing.md` / `mock-strategy.md`，新增"Agent 上下文"一节（AGENTS.md / .claude/AGENT.md）。
- `.github/README.md` 头部注明当前工作流文件（ci.yml + frontend-ci.yml），明确其为历史踩坑记录。

### 验证
- 文档链接交叉验证（.claude/AGENT.md ↔ docs/ 各文件）：✅
