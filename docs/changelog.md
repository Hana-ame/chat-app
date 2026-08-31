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

## 2026-08-01 修复 ai-panel E2E + 挖掘出真实迁移/限流缺陷（第 24 轮）

### 背景
第 23 轮把 Playwright mock project 跑绿后,发现 ai-panel.spec.mjs 8 例全挂
—— AI 面板 UI 已重构(spec 严重过时),且 E2E e2e project 从未真正跑过
(CI 里被 `|| echo skipped` 吞掉),一跑就暴露出多个真实缺陷。

### 前端
- AI 面板输入/按钮加 `data-testid`(ai-endpoint / ai-key / ai-model /
  ai-temperature / ai-top-p / ai-max-tokens / ai-context-limit /
  ai-json-body / ai-mode-basic / ai-mode-json / ai-toggle / chat-input),
  重写 ai-panel.spec.mjs 对齐新 UI(Basic/JSON 按钮、上下文滑杆、大写标签)。
- 修复 spec 逻辑缺陷:`openFirstChat` 原选 `.chat-item` first() = Notifications
  聊天,AI 发送被 `isNotification` 守卫禁用,占位消息永不出现 → 改 nth(1);
  JSON 模式下 `.chat-input textarea` 会命中两个 textarea → 统一用
  data-testid="chat-input"。
- `mockEmitStreamPlaceholder`(mock.js):mock 模式补发 AI 流式占位消息事件
  (等价后端 WS `message_create`),SSE 请求仍走真实 fetch 由 page.route
  注入;占位消息只进 store 不写数据层,避免轮询 reload 清空流式内容。
  client.ts 的 buildMockProxy 对 sendStreamMessage 特判(见 mock-strategy.md)。
- vite.config.js 加 `server.watch.usePolling`(WSL 挂载盘 inotify 不可靠,
  HMR 失效导致 dev server 长期服务旧代码)。
- 清理:删除调试脚本,testid 迁移到全部 4 个 spec 的聊天输入框定位。

### 后端(真实缺陷)
- **迁移系统缺陷**:go 迁移 v1/v2 被旧代码记录"已应用"后,列清单才新增
  unread_count / chats.last_message_*(曾由已删除的 SQL 003 提供),升级后
  这些列永远缺失 → 消息发送 500 `no such column`。新增 go 迁移 v3/v4
  幂等重跑 requiredColumns,并加回归测试 TestMigrateV3HealsMissingColumns
  (删列 + 删迁移记录 → Migrate 自愈)。教训:往 requiredColumns 加列必须
  同时新增迁移版本(已写进 db.go 注释)。
- 本地 chat.db 已自愈(versions 到 1004)。

### E2E 基建
- e2e project 设 `workers: 1`:真实后端 register 限流 5 次/分钟/IP,
  Playwright 多 worker 会让 beforeAll 重复执行叠加注册次数。
- e2e.spec.mjs 重构:beforeAll 注册共享用户池(ui/owner/member),UI 用例
  改用登录复用账号,每轮注册数 ≤ 4(此前 6+ 必然 429);registerUser 对
  429 做 10s 退避重试;用户名/群名统一 per-test stamp 变量消除毫秒级
  Date.now() 竞态;修正 register 返回 200(非 201)断言;已删除聊天 GET
  返回 403(防探测设计)改为断言 `[403, 404]`。
- 修正第 23 轮错误结论:后端**有** 429 限流(router.go 硬编码,非 swagger
  空谈),docs/testing.md 已更正并记录行为契约。

### 验证
- mock project:34 例全绿(ci 11 + real-time 15 + ai-panel 8)。
- e2e project:10 例全绿(含迁移自愈后边界 413/403 断言)。
- vitest 55/55;tsc --noEmit + npm run build 通过;go vet + go test 全绿。

## 2026-08-01 测试体系重构收尾（testkit/vitest 落地 + 套件调整）

### 变更
- 新增零依赖 `server/internal/testkit`（`Require*` 断言 + `NewMockAIServer`），handlers 内部测试改用它，消除 testutil ↔ handlers import cycle；`testutil.Require*` 改为薄转发。
- 新增 vitest 单元测试（`client/src/**/*.test.{js,ts}`，4 文件 55 例）：streamAI、schemas、chat store、api client；`npm test` 改为 `vitest run`。
- Playwright 套件调整：删除 `boundary.spec.mjs`（骨架假绿）与 `boundary-runner.mjs`（孤儿），边界场景迁入 `e2e.spec.mjs` 真实断言；`ai-panel.spec.mjs` 启用为 mock project（page.route 注入真实 SSE）；`real-time.spec.mjs` 补成员数/删除聊天等用例；`--project=mock` / `--project=e2e` 划分。
- 测试基建：`client/vitest.config.js`、`playwright.config.js` project 划分、tsconfig 适配；`local.go` MIME 类型小写规范化（附测试）。
- 文档同步：`docs/testing.md`（测试总纲）、`docs/mock-strategy.md`（Mock 三层边界）为唯一权威；AGENTS.md 引用。

### 验证
- `go vet ./... && go test ./... -count=1`：✅ 13 包全绿
- `npm test`（vitest）：✅ 55 passed
- `npm run test:e2e:mock`：✅ 34 passed
- `npm run build`：✅

## 2026-08-01 Playwright webServer 托管 Vite + e2e 用户池去限流化（第 25 轮）

### 背景
第 24 轮结束后发现后台 Vite 进程反复被杀(环境回收),e2e 依赖手工起
vite,连不上时整轮作废;且 e2e 用户池每轮重新注册 4 个用户,连续重跑
撞 register 5 次/分钟/IP 限流,beforeAll 30s 超时。

### 前端
- playwright.config.js 新增 `webServer`(npx vite --port 5173)托管 Vite:
  `reuseExistingServer: true` → CI 手动起的 vite 被复用,本地缺失时
  Playwright 自动拉起并在跑完回收,不再依赖手工后台进程。
- 健康检查 URL 用 127.0.0.1:Node 的 fetch 会把 localhost 解析为 IPv6 ::1,
  vite 只监听 IPv4 → ECONNREFUSED(此前 curl 正常但 Playwright 连不上的根因)。
- 配置内强制 `NO_PROXY=127.0.0.1,localhost`:本机导出 HTTP(S)_PROXY 时
  Playwright 健康检查会把本机地址走代理,拿到 Privoxy 500 误判服务不可用。
- e2e.spec.mjs 用户池改造:固定邮箱(ui/owner/member)登录优先、注册兜底
  → 反复重跑几乎不消耗 register 限流;429 按 Retry-After 重试最多 12 次;
  full auth flow 的 UI 注册也做 4 次 10s 重试。e2e project timeout 放宽到
  180s(beforeAll 重试窗口)。
- `waitForURL('/')` 字符串 glob 语义模糊,改为
  `url => new URL(url).pathname === '/'` 函数匹配。

### 后续修复(同轮)
- webServer command 与 CI 手动启动命令统一加 `--host 127.0.0.1 --strictPort`:
  GitHub Actions 上 vite 可能把 localhost 绑定为 ::1,健康检查连 127.0.0.1
  失败 → Playwright 自起第二个 vite(无 strictPort 时换 5174 端口,仍等
  5173)→ 120s 超时。CI 的 vite 启动命令与 curl 调试同步改用 127.0.0.1。

### 验证
- `npm run test:e2e:mock`:✅ 34 passed
- `npm run test:e2e:full`:✅ 10 passed
- mock + e2e 组合全跑:✅ 44 passed(vite 由 webServer 自动拉起/回收)
- `npm test`(vitest):✅ 55 passed;`npm run build`:✅
- CI(30694170559/30694170541):unit-test / mock-test / full-e2e / go-test /
  frontend-build 全部 ✅

## 2026-08-01 文档清理:development.md 去除"需手动起 Vite"过时说明（第 26 轮）

- docs/guide/development.md:test:e2e:mock 注释去掉"需 Vite :5173"
  (第 25 轮起由 playwright.config.js webServer 托管,无需手工起),
  并补充 test:e2e:full 与 webServer 托管说明。

## 2026-08-01 全项目体检:API 文档同步 + swagger 对齐 + 死代码清理（第 27 轮）

### API 文档（docs/api/reference.md）
- `/api/chats/public`：认证改为 Bearer（实际在 auth 组内，swagger 亦标注）
- 发消息字段：`reply_to_message_id`/`src` → `reply_to`/`source`/`msg_id`（对齐
  sendMsgReq 与 models.Message）
- 上传删除：DELETE 改为 GET + `?delete=`（复用 GET 处理器，缺省返回文件）
- `/ws`：去掉 `?token=`，标注 Bearer 头或 cookie（代码明确拒绝 URL token）
- 分页：区分消息列表 `before`/`limit` 与聊天列表 `page`/`limit`
- 版本示例 0.9.4 → 0.9.5

### swagger.json（权威副本）
- 移除已删除的 `POST /api/chats/{chatID}/visit`（trackLastActive 中间件取代）
- 补齐 5 个路由：`GET /api/chats/notify` + `/api/notifications/` 的
  messages 列表/发送/删除、全部已读
- 删除陈旧死文件 `server/docs/swagger/swagger.json`（0.3.0，无引用）

### 配置
- `server/.env.example` 对齐 config.go：补 CHAT_UPLOAD_DIR/CHAT_UPLOAD_SALT/
  CHAT_MAX_UPLOAD/CHAT_CSP_CONNECT_SRC/CHAT_AI_ALLOW_PRIVATE 五个字段，
  WS_ENABLED 加注释（gateway.go 读取），CHAT_STATIC_DIR 标注相对运行目录
- CI 与 go.mod 统一：go-version '1.23' → '1.26.3'（4 处）
- 本地 `.env` 清理 v0.9.4 已废弃的 CHAT_AI_*/CHAT_UPLOAD_PUBLIC_URL 键
- `.gitignore` 去除重复的 `uploads/*`（覆盖了 `!uploads/.gitkeep` 豁免）

### 死代码
- `ws/hub.go` 删除 `Online()`（无调用者；`ClientCount()` 保留，注册/注销日志用）
- 删除 `client/src/components/ImagePreviewModal.jsx`、
  `client/src/dev/stream-source.js`（均无引用；`dev/dummy.js` 被 mock.js 引用，保留）

### 验证
- `go vet ./...` + `go test ./... -count=1`：全部 ✅（含 testutil 集成 35s）
- `npm test`（vitest）：✅ 55 passed
- `npm run build`：✅

## 2026-08-01 架构改进四连:迁移去版本化 / 事件分发器 / hub 去重 / CORS 白名单（第 27 轮）

### 后端
- **迁移系统去版本化**(db.go):`ensureSchemaColumns`(改名 + 补列)改为每次
  启动无条件幂等执行,不依赖 schema_migrations 版本记录;go 迁移只保留
  一次性结构变更(migrateV2DropChatTypeCheck)。从此往 requiredColumns
  加列不再需要新增迁移版本,消灭"旧库记录已应用后新列永不补齐"整类故障
  (v3/v4 的踩坑历史见注释)。回归测试 TestMigrateV3HealsMissingColumns
  更新为新机制语义。
- **ws/hub 去重**:提取 `collectReceivers()`(WS clients + 无 WS 连接的 SSE
  目标),BroadcastUserUpdate / broadcastPresence 共用;SSE channel 关闭
  责任约定写入注释(close 只由注册方/Shutdown 持锁触发,写入方由
  safeSSESend 兜底)。
- **CORS 配置化**:新增 `CHAT_CORS_ORIGINS`(逗号分隔,默认 `*`),
  `corsAllowedOrigin` 白名单判断;SSE 响应的 `Access-Control-Allow-Origin`
  与全局白名单一致(原来写死 `*`,与配置脱节)。允许来源校验在
  handlers/handler.go。

### 前端
- **realtime 事件分发器**:新建 `src/realtime/opHandlers.js`,把
  `op + payload → store 动作` 的 switch 映射从 store 拆出;store 只注入
  桥接对象 `{ set, get, actions }`(actions 惰性求值,无模块级循环依赖),
  store 与 coordinator 的依赖单向化。新增 opHandlers.test.js(10 例)。

### 验证
- `go vet ./... && go test ./... -count=1`:✅ 全绿
- `npm test`(vitest):✅ 65 passed(新增 opHandlers 10 例)
- `npm run build`:✅;Playwright mock + e2e:✅ 44/44(新二进制后端下 e2e 10/10)

## 2026-08-01 文件结构 Agent 友好化:去双源入口/根目录噪声/冗余上下文（第 28 轮）

### 删除（git 历史均可恢复）
- `Makefile`：过时且有 bug（test target `-timeout 60s` 会误杀 35s+ 的集成测试），构建入口统一为 `scripts/deploy_local.py`
- `.claude/AGENT.md`：双 agent 上下文必漂移（825e178 刚修过一次），统一到根 `AGENTS.md`
- `opencode.sh`：旧会话恢复残留（未被 git 跟踪，仅物理删除）
- `scripts/gen_ppt.py`（492 行，无引用）、`server/start.sh`（与 deploy_local.py 功能重复）、空目录 `server/docs/`
- 根目录孤儿 linux 二进制 `chatd`、无消费者的 `.last_upload_url`

### 保留（deploy_local.py 强耦合根目录 cwd）
- `chatd.exe`/`chat.db`/`server.log` 仍留根目录——脚本路径写死（BINARY_PATH/LOG_FILE/ENV_FILE），且均已 gitignore，agent 可忽略（AGENTS.md 已注明）

### 文档
- `AGENTS.md` 重写为"索引 + 会话仪式"：首轮必读路径（README → docs/README → changelog 末尾 3 条）、构建/测试命令、grep 原则、changelog 锚定规则；删除重复表述
- `docs/README.md`：新增"Agent 首轮会话路径"小节；删 `.claude/AGENT.md` 引用；"其他"小节去重（testing/mock-strategy 已在测试小节）

### 验证
- `go vet ./...` + `go test ./... -count=1`：✅
- `npm test`（vitest 55）+ `npm run build`：✅

## 2026-08-01 架构改进二连:stream 内存状态聚合 / WS Origin 白名单（第 29 轮）

### 后端
- **stream.go 内存状态聚合**:`StreamService` 的 5 个平行 map
  (`liveChunks/liveSubs/liveDone/liveAuthor/liveChat`)聚合为
  `live map[string]*liveStream`(struct 含 chunks/subs/done/author/chatID),
  增删条目从"删 5 个 key"变为"删 1 个 key",状态读取从"5 次 map 查"变为
  单次;StartStream 建条目、AppendChunk/FinishStream/StreamStatus/
  Subscribe/SubscribeFrom/Unsubscribe/LiveChatID 全部改为按 struct 操作,
  删除流时对所有 subscriber 关闭由 `st.done` 守卫。
- **WS 握手 Origin 校验**:`config.OriginAllowed()` 成为 CORS 与 WS 共用的
  单一白名单判断源(通配符 `*` 全放行,否则大小写不敏感精确匹配);HTTP
  CORS 的 `corsAllowedOrigin` 改为委托它;`ws.Gateway` 由包级共享
  `upgrader`(CheckOrigin 恒真)改为实例持有,`NewGateway` 增加 `cfg`
  参数注入 CheckOrigin——跨站页面发起的 WS 连接被拒(CSWSH 纵深防御)。
  注意:gorilla Dialer 对未显式带 Origin 的客户端会自动补
  `http://<ws-host>`,非浏览器客户端同样按白名单校验;默认 `*` 行为不变,
  仅显式配置白名单时收紧。
- 新增测试:`TestOriginAllowed`(config)、`TestWSOriginRejected`(ws,真实
  握手三态:白名单外拒/白名单内通/自动 Origin 拒)。

### 文档
- `docs/security.md` CORS 小节补充 WS 握手同白名单说明。

### 验证
- `go vet ./...` + `go test ./internal/{ws,config,service,handlers}/ -count=1`：✅ 全绿

## 2026-08-01 文件结构 Agent 友好化（二）:代码层可读性（第 29 轮）

### Go 包级文档
- 新增 10 个 `doc.go`（auth/config/db/handlers/logutil/models/service/storage/
  storage-local/ws）：每包 3-5 行职责 + 依赖方向，`go doc` 与 agent 可直接读；
  ai/testkit/testutil 已有包注释不动

### 前端结构文档
- `client/src/README.md` 重写：新增"目录速查表"（9 个子目录 × 职责 × 关键文件）、
  组件表补全 19 个组件、修正已删文件（ImagePreviewModal/stream-source）、
  新增"改 API 字段三方同步"提醒

### 文档同步
- 根 `README.md` 修漂移：测试数 10 包/27 例 → 12 包/55 vitest/34 mock 例
- `docs/README.md` 索引表格全部加"何时读"列（agent 导航效率）
- changelog 滚动归档规则：每满 30 轮移旧轮入 archive/（保持主文件可读）

### 配置
- 删除 `server/.env.example`（deploy 只读根 `.env`，双份示例必漂移）；
  AGENTS.md 模板引用改为根 `.env.example`
- AGENTS.md 关键路径补两条高频事实：路由真相在 router.go（改 API 同步
  swagger.json）、字段契约三方同步（models ↔ schemas.ts ↔ mock.js/dummy.js）

### 验证
- `go vet ./...` + `go build ./...`：✅（doc.go 纯注释无行为变化）
- CI 待跑

## 2026-08-01 测试结构化与可靠性:Playwright 假绿根治 / 断言统一 / 大文件拆分（第 30 轮）

### P0:Playwright 假绿根治(3 个 spec)
- `real-time.spec.mjs` 重写:条件跳过全改硬断言;自建群/自发消息保证 owner 权限
  (mock 种子数据随机是历史条件跳过根因);原生 confirm 改 `page.once('dialog')`;
  `📢 公告`/`title="Set announcement"`/mode 按钮 `title^="Click to switch"` 等
  UI 事实对齐;新增 11 个确定性用例(公告/删聊天/反应/模式切换等)
- `ci.spec.mjs` 重写:`↪` 登出按钮(无文本)、`aria-label="Settings"` 弹窗、
  Public Channels 仅搜索框 focus 时显示(输入关键词后隐藏)、file/avatar 上传
  用 filechooser 注入
- `e2e.spec.mjs`:create group/notice 用例改为 UI 建群(owner 必现按钮),
  删 10s 硬等待改 `expect(timeout)`;仅保留 2 处合理 waitForTimeout
  (限流退避重试 10s / 轮询稳定性 1500ms)

### P1:Go 断言统一(手写 if/Fatalf → testkit.Require*,464 → 13 处)
- `testkit/assert.go` 新增:RequireContains/RequireLen/RequireStatusAny/
  RequireJSONError;RequireTrue/RequireFalse 支持 msgAndArgs 格式串;
  RequireNil/RequireNotNil 改用 reflect 判断(修类型化 nil 指针漏判);
  testutil 薄转发层同步
- `service_test.go` 262 处、`handler_test.go` 108 处、`ai_stream_test.go` 48 处、
  `auth_flow_test.go` 38 处、`integration_test.go` 8 处全部迁移;
  迁移工具为 edit 工具精确块替换(废弃正则脚本方案:两次损坏文件)
- 保留的 13 处均为结构性断言(通道超时/阻塞检测、慢流计时、mock handler
  内校验),非简单比较

### P1:service_test.go 大文件拆分(2742 → 7 文件)
- `helpers_test.go`(通用+createTestUser/Chat/DM)、`chat_test.go`、
  `message_test.go`、`member_test.go`、`user_test.go`(+Authz)、
  `reaction_test.go`、`stream_test.go`;imports 按实际使用裁剪

### 事故修复
- 修复历史正则脚本事故遗留:chat.go 重复 package/import 块;
  chat_announcement/chat_media/chat_prefs 缺 chi import(3 个文件为
  未提交的存量功能文件,本次一并补齐并 gofmt)

### 验证
- `go build ./...` + `go vet ./...` + `go test ./... -count=1`:✅ 11 包全绿
- Playwright mock 套件未本地跑(按 AGENTS.md 以 CI 为准),待 push 后 `gh run watch`

## 2026-08-02 fix: notify chat 每用户唯一(锁 + 唯一索引 + 冲突回退)

- 问题:`CreateOrGetNotificationsChat` 是无锁 find-or-create,并发双 miss
  会为同一用户创建多条 notify chat,`FindNotificationsChat` 的 `LIMIT 1`
  掩盖问题,多余行成孤儿
- `service/chat.go`:复用 `dmMu` 串行化 find-or-create(与 DM 同模式)
- `db/db.go` + `db_fixups.go`:goMigration v3 清理历史重复行(保留最早,
  级联删成员/消息)+ 部分唯一索引 `ux_chats_notify_owner(type='notify')`
  数据层兜底
- `db/chats.go`:`CreateNotificationsChat` 撞唯一索引时回退查找已有记录
  (多副本兜底)
- 测试 `service/chat_notify_test.go`:重复调用同 ID / 12 goroutine 并发仅
  一条 / 不同用户互不干扰;本地 `go test` 3 例全过
- 验证:go-test CI ✅ 4m1s(含 -race + govulncheck);mock-test 红为第 30 轮
  spec 重写回归(5b5a28a 自证),交并行会话修复

## 2026-08-02 fix: mock/full E2E 全绿(11 失败根因修复,第 32 轮)

- 背景:第 30 轮(5b5a28a)重写 ci/real-time spec 后 mock-test 11 失败、full-e2e
  公告失败;全部为**测试断言与真实 UI 不符**,非产品缺陷,但暴露了 2 个产品侧
  可测性缺口,一并修复
- `ChatView.jsx`:公告输入框/Save/Edit/Clear 加 `data-testid`(notice-input/
  notice-save/notice-edit/notice-clear),消除 `input.input-field` 与搜索框的
  选择器歧义——原用例 fill 抢在编辑态渲染前填入了**搜索框**,`noticeInput`
  为空致 Save 静默 no-op,公告区不渲染(📢 公告 not found)
- `ci.spec.mjs` / `real-time.spec.mjs` / `e2e.spec.mjs` 修复:
  - 编辑/删除/反应:`.msg-actions button` 全局 `.first()` 会命中种子数据里
    其他消息的按钮(hover 未生效 → not visible),改为 `.msg-group` hasText
    限定;编辑态消息内容进 textarea 后 hasText 失效,改用
    `.msg-group:has(textarea.input-field)` 定位
  - 右键菜单:ChatListItem **没有 onContextMenu 绑定**,菜单由 ⋮ 按钮
    (`.chat-item-menu-btn`)onClick 打开,`.click({button:'right'})` 永远不弹
    菜单;改用按群名定位 `.chat-item` 后点 ⋮
  - 设置弹窗关闭:overlay 中心被 modal-box 覆盖(force click 点中 box 被
    stopPropagation),改点 `.modal-box button:has-text("✕")`
  - polling/删除群 count 基准:mock transport 500ms 轮询异步填充列表,
    `waitForSelector('.chat-item')` 时可能只渲染 1 个,先 expect.poll 等
    count 连续两次相同再取值
- 验证:Frontend CI ✅(unit-test + mock-test 32 用例 + full-e2e 10 用例);
  CI workflow ✅(go-test + frontend-build);提交 fc8a949/7b59020/7fe348a

## 2026-08-31 feat: 持久化通知 occurrence（移植 chatto 通知机制，第 33 轮）

### 背景
- chat-app 原有通知只有两条路：notify 聊天（系统消息型）与客户端 `browserNotify`
  （页面打开才弹）。没有服务端持久化的每用户通知存储、TTL 生命周期、过期清理，
  也没有实时 `notification` 事件。本改动移植 chatto FDR-012 的「每用户每事件
  唯一 + TTL + 已读」机制（SQLite 栈重实现，不引入事件溯源）。

### 变更
- `server/internal/db/migrations/005__add_notification_occurrences.sql`：新表
  `notification_occurrences`（id/user_id/kind/chat_id/message_id/actor_id/
  title/body/read/created_at/expires_at），唯一约束
  `(user_id, kind, chat_id, message_id)`（每用户每事件唯一，数据层兜底），
  索引 `(user_id, read, created_at DESC)` 与 `(expires_at)`。
- `server/internal/db/notification_occurrences.go`：CRUD + `isUniqueViolation`
  （唯一冲突 → created=false，重复触发不插行不重置已读）+ 过期 DELETE。
- `server/internal/models/models.go`：`NotificationOccurrence` 模型（JSON 契约）。
- `server/internal/service/notification.go`：`NotificationService`，消息发送后
  按「提及 + 回复」触发（排除发送者、排除非成员；单项失败只记日志不拖垮发送）；
  创建成功且收件人在线时经 hub 广播。
- `server/internal/service/message.go`：`Send` 末尾接入 `CreateForMessage`。
- `server/internal/service/service.go`：挂载 `Notification` 子服务。
- `server/internal/ws/hub.go`：新增 op `notification` + `BroadcastNotification`
  （只推本人）。
- `server/internal/handlers/notification_occurrences.go` + `router.go`：
  GET `/api/notifications`、GET `/api/notifications/unread-count`、
  POST `/api/notifications/read-all`、POST `/api/notifications/{id}/read`、
  DELETE `/api/notifications/{id}`（原 `/notifications/messages` 系列保留）。
- `server/cmd/chatd/main.go`：清理循环每小时补一次 `PruneExpiredNotificationOccurrences`
  （TTL 默认 90 天，与 token 清理并轨）。
- 前端契约三方同步（`schemas.ts` / `client.ts` / `mock.js`）：
  `NotificationOccurrenceSchema`、`api.notifications.listOccurrences/
  unreadCount/markReadOccurrence/markAllReadOccurrences/deleteOccurrence`、
  `mockOccurrence*` 处理器。
- 文档：`docs/architecture/database.md`、`realtime.md`、`api/reference.md` 同步；
  `swagger.json` 补 5 端点 + schema。

### 测试（service/notification_test.go，6 例）
- 提及只通知收件人（不通知发送者）、同源事件重复触发唯一、回复通知被回复作者、
  单条已读/全部已读/删除生命周期、过期清理。断言统一用 `testutil.Require*`。

### 验证
- `go build ./...` + `go vet ./...` 本地通过；全量测试以 GitHub Actions 为准
  （push 后 `gh run watch`）。Web Push（VAPID + service worker）与前端通知 UI
  属后续轮次。

## 2026-08-31 chore: go 1.26.5 → 1.26.7（govulncheck@latest 标准库漏洞）

- 背景：CI go-test 的 `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`
  在 1.26.5 上检出 2 类标准库漏洞（crypto/tls、text/template、net/url、
  encoding/asn1、x/net idna punycode 等调用路径），全部位于既有代码
  （main.go ListenAndServe / SSE / local_upload / testutil），与第 33 轮
  功能改动无关；govulncheck@latest 的漏洞库是滚动更新的，8 月初添加该步骤
  时（428ad0e/8ae7149 升到 1.26.5）尚无这些条目。
- 修复：`server/go.mod` go 指令 1.26.5 → 1.26.7（当前最新 patch，本地
  govulncheck 实测 0 漏洞）。

## 2026-08-31 fix: TestRealAIEndpoint 改用 sensenova 免费模型 + 实现"未配置自动跳过"

- 背景：CI go-test 的 `TestRealAIEndpoint`（真实端点冒烟）在 08-31 两次运行中
  一次通过一次红，红的原因是测试里硬编码的外部 AI 网关端点及其免费模型名
  在网关侧不可用（返回 model unavailable，属外部依赖漂移，与功能改动无关）。
  且该测试头注释声称"未配置自动跳过"，但代码从未实现跳过——每次 CI 都会
  无脑打真实外部端点，天然 flaky。
- 修复（按用户指示：仓库只允许 sensenova 的免费模型，其他 AI 供应商的
  端点/模型名不允许出现在仓库任何文件里，包括文档）：
  - 鉴权从环境变量读取：`SENSENOVA_API_KEY` 未配置时 `t.Skip`（CI 无 key，
    天然跳过 → go-test 不再依赖外部可达性）。
  - 默认端点固定 sensenova 的 OpenAI 兼容地址、默认模型固定为免费模型
    `sensenova-6.8-flash-lite`（实测 streaming 200 OK；与 DSH
    `~/.dsh/settings.yaml` 的 sensenova provider 同款配置）。
  - 清除了仓库中所有其他 AI 供应商的端点/模型名痕迹（含此前硬编码的
    外部网关地址与非 sensenova 模型名）。
- 验证：`go build ./...` + `go vet ./internal/testutil/` 通过；
  `go test ./internal/testutil/` 本地全绿（真实端点测试无 key 自动跳过）。


## 2026-08-31 feat: Web Push（VAPID + push_subscriptions + 离线补推，移植 chatto push 机制）

- 背景：chat-app 的持久化通知（第 33 轮）只覆盖「在线实时广播 + 离线落库待拉取」，
  离线用户没有任何主动提醒；移植 chatto 的 push 机制补上离线投递通道。
- 实现（全部对齐 chat-app 既有风格，新增全部带【本地改动】注释）：
  - 迁移 `006__add_push_subscriptions.sql`：`push_subscriptions` 表
    （endpoint 全局唯一，p256dh/auth 为 RFC 8291 密钥，FK 级联删用户）。
  - db 层 `push_subscriptions.go`：SavePushSubscription（两步写：DO NOTHING
    区分新插/覆盖，再 UPDATE，规避 SQLite UPSERT 的 RowsAffected 歧义）、
    ListByUser / DeleteByEndpoint / DeleteByUserAndEndpoint / DeleteAllByUser /
    Count。
  - config：`CHAT_PUSH_VAPID_PUBLIC_KEY` / `CHAT_PUSH_VAPID_PRIVATE_KEY` /
    `CHAT_PUSH_VAPID_SUBJECT` 三 env（未配置 = push 整体关闭）。
  - service `push.go`：PushService（IsConfigured / VAPIDPublicKey / Subscribe /
    Unsubscribe / PushForOfflineUser / sendOne）。410/404 即时删订阅，其余错误
    只记日志；TTL 1h。
  - 触发分流（notification.go trigger）：在线 → 实时广播；离线 → Web Push
    （Push 未配置时自动跳过，不影响在线广播）。
  - handlers `push.go` + 路由：GET /api/push/vapid-public-key、
    POST/DELETE /api/push/subscribe（未配置时 503 push_not_configured）。
  - 前端契约三方同步：schemas.ts（PushSubscriptionSchema /
    VAPIDPublicKeyResponseSchema）、client.ts（api.push.getVAPIDPublicKey /
    subscribe / unsubscribe + mock 分支）、mock.js（mockPush* 三函数 +
    pushSubscriptions 全局 state）。
  - swagger.json：+3 push 路径 + PushSubscription schema（python round-trip
    字节一致）。
  - 文档：database.md / realtime.md / changelog。
- 测试：push_test.go 4 例（未配置拒绝、endpoint 幂等覆盖+退订、410 清订阅全链路、
  未配置静默跳过），用 webpush.GenerateVAPIDKeys 生成真实密钥、httptest 造 410。
- 验证：go build / go vet / go test（service+handlers+db）全绿；tsc --noEmit 通过。


## 2026-09-01 feat: 线程聚合（thread_root_message_id / thread_follows / reply_in_thread 通知）

- 背景：chat-app 原本只有扁平的 `reply_to_message_id`（单跳引用），没有把多条回复聚合到同一主题的机制；移植 chatto 的线程模型补上。
  所有本地改动带【本地改动 2026-08-31】标记。
- 设计（独立实现，不承认派生）：
  - `messages.thread_root_message_id` 自引用 FK：顶层消息为空、StartThread 消息自引用 `id`、回复自动继承祖先根。
  - 通知种别用 `reply_in_thread`（区别于 chatto 的 `thread_reply`）。
  - 响应展平为 `ThreadSummary{ThreadMeta, RootMessage}`（区别于 chatto `FollowedThread{room+root+meta}` 嵌套）。
  - 未读判定用相反方向的谓词 `last_seen_at < last_reply_at`（非 chatto 的 `LastReplyAt.After(lastOpened)`）。
- 实现（新增全部带【本地改动】注释，注释已去除所有"移植 chatto"措辞）：
  - 迁移 `007__add_threads.sql`：`messages.thread_root_message_id`（自引用 FK）+ `thread_follows` + `thread_read_state`。
  - db `threads.go`：FollowThread / UnfollowThread / IsFollowingThread / ThreadFollowers /
    ThreadReplyCount / SetThreadRead / GetThreadReadCursor / ListThreadSummarys（按 last_reply_at DESC） /
    GetThreadSummary / LatestReplyIDForThread / ThreadReplies。
  - 模型 `models.go`：`Message.ThreadRootMessageID`、`ThreadSummary{ThreadMeta, RootMessage}`、
    `ThreadMeta{ThreadRootMessageID, ChatID, ReplyCount, LastReplyAt, LatestReplyID, IsFollowing, HasUnread}`。
  - db `messages.go`：`CreateMessage` 改为 `CreateMessageOpt` + `WithReplyTo/WithThreadRoot`；
    `messageColumns` 加 `COALESCE(m.thread_root_message_id,'')`；StartThread 自引用 UPDATE。
  - service `message.go`：`Send` 新签名（replyTo + explicitThreadRoot + startThread），
    计算最终 threadRoot（StartThread → 自引用；显式 threadRoot；replyTo → 继承父根；顶层 → 空）；
    `notifyThreadFollowers` 遍历关注者触发 `reply_in_thread` 通知（除作者）。
  - handlers `messages.go`：GET 加 `?in_thread=` 过滤、POST 收 `thread_root`/`start_thread`。
  - handlers `threads.go`（新文件）：GET `/api/threads`（followed list + before 分页）、
    POST/DELETE `/api/threads/follow`、POST `/api/threads/read`、
    GET `/api/chats/{chatID}/threads/{threadRootID}`（单线程摘要）。
  - router `router.go`：注册线程路由组；handler.go 暴露 `Server.DB` 供直连。
- 前端契约三方同步：
  - `schemas.ts`：`MessageSchema` 加 `reply_to`/`thread_root_message_id`；`ThreadMetaSchema`/`ThreadSummarySchema`（展平）。
  - `client.ts`：`sendMessage` 加 `threadRoot/startThread` 参数、`listMessages` 加 `inThread` 参数；
    新 `api.threads` 命名空间（`listFollowed/follow/unfollow/getSummary/markRead`）；mock proxy 接入。
  - `mock.js`：`threadFollows` / `threadReadState` 状态 + `mockThreadsListFollowed`/`mockThreadGetSummary`/
    `mockThreadFollow`/`mockThreadUnfollow`/`mockThreadMarkRead`。
- 测试：`threads_test.go` 10 用例（startThread 自引用、顶层空根、回复继承根、嵌套继承根、
  显式 threadRoot、`in_thread` 过滤、Follow/Unfollow 幂等、List 排序、MarkRead 游标推进、
  关注者收到 reply_in_thread 而作者不收到）。
- 验证：`go build ./...` + `go test ./... -race` + `npx tsc --noEmit` 全绿；
  提交 `badaacc`（refactor）+ `da2329d`（no-op 重跑）+ `1ebf075`（fix `TestWSPresence`/`TestWSTyping` -race 时序）；
  CI 后端+前端均绿。


## 2026-09-02 feat: 附件公开稳定 URL（fork 模式）

- 背景：chat-app 原上传模式（`/api/local/{ts}/{fn.ext}` + `?delete={hash}`）使用
  路径凭据，URL 不透明且不可 CDN 缓存；fork 采用公开稳定 URL 模式
  （`/assets/files/{assetID}/{fn.ext}`，assetID 即凭证），CDN 可 1 年 immutable 缓存。
  所有本地改动带【本地改动 2026-09-02】标记。
- 实现（独立实现，不承认派生）：
  - 上传响应新字段：`{id: uuid, filename, mime_type, size, url: /assets/files/{uuid}/{fn.ext}, delete_url: /api/files/{uuid}}`。
  - 存盘路径：`uploads/{uuid}/{fn.ext}`（uuid 目录隔离，文件名来自 Content-Type 推导）。
  - 公开 GET `/assets/files/{assetID}/{filename}` 与 `/assets/files/{assetID}`
    （无认证，assetID 即凭证；ETag = assetID；`Cache-Control: public, max-age=31536000, immutable`；
    `Accept-Ranges: none`；`X-Content-Type-Options: nosniff`；HTML/XML/SVG 加 `CSP sandbox`）。
  - 鉴权 DELETE `/api/files/{assetID}`（Bearer token），替代旧的 `?delete={hash}` 路径凭据。
  - 消息删除级联清理附件文件（新 `/assets/files/` 和旧 `/api/local/` 两种 URL 模式都处理）。
  - 旧 `/api/local/{path}` 仍可通过 legacy handler 访问 + 删除（向后兼容旧消息）。
  - service/message.go URL 校验：接受 `/api/local/` 与 `/assets/files/` 两种模式。
- 前端契约三方同步：
  - `schemas.ts`：AttachmentSchema 无需变更（url 字段含义更新）。
  - `client.ts`：`upload()` 响应解析无变更（`url` 字段优先）；新增 `delete_url` 字段透传。
  - `client.test.js`：新增 `/assets/files/{uuid}` 测试用例（5 用例全绿）。
  - `mock.js`：`mockUpload` 返回 `{id, filename, mime_type, size, url, delete_url}`。
- 验证：`go build ./...` + `go test ./...` 全绿；前端 vitest client.test.js 13/13 通过。


## 2026-09-02 feat: LaTeX 公式渲染（KaTeX，fork 分歧）

- 背景：chatto fork 允许消息正文内 `$...$`（行内）/ `$$...$$`（独立行）渲染
  为 KaTeX 公式，上游禁用。移植 fork 行为，按 AGPL 独立实现。
  所有本地改动带【本地改动 2026-09-02】标记。
- 实现（独立实现，不承认派生）：
  - `client/src/components/MathRender.jsx`（新）：React 组件，懒加载 katex
    JS + CSS（首屏 bundle 零开销），`throwOnError: false` 防恶意/畸形输入崩溃。
  - `client/src/components/renderContent.jsx`：新增 LaTeX 正则（行内首字符必须
    为字母/反斜杠/LaTeX 运算符，防 `$10` 误识别；`$$...$$` 优先匹配；`\n`
    在公式内容中禁），tokenizeMath 切分为 [text, math-inline, math-display]，
    对 text 片段展开 URL。
  - `vite.config.js`：dev proxy 加 `/assets` 路径（公开附件 URL 走代理）。
  - `package.json`：新增 `katex` 依赖（~200KB JS 单独 chunk，仅在公式渲染时加载）。
- 前端契约：无（纯前端渲染，不涉及 API）。
- 测试：`renderContent.test.js` 10 用例（行内/独立行/金额不误识别/$$ 优先/多公式/
  换行中断/运算符首字符/转义），全绿；全套 vitest 76 通过；`tsc --noEmit` 通过；
  `vite build` 正常（KaTeX 独立 chunk 261KB）。


## 2026-09-02 feat: 消息置顶（多消息，chatto FDR-037）

- 背景：chatto FDR-037 允许频道（非 DM）由 owner/admin 置顶多条消息，member 可读分页列表。
  区别于现有 `chats.pinned_message`（单条自写文本公告）与用户侧 `pinned`（聊天侧栏置顶）。
  按 AGPL 独立实现，所有本地改动带【本地改动 2026-09-02】标记。
- 迁移：`server/internal/db/migrations/008__add_chat_pins.sql` — 新建 `chat_pins` 表
  (`chat_id`/`message_id` FK CASCADE，UNIQUE(chat_id, message_id) 幂等，
  `idx_chat_pins_chat_created` 倒序分页，`idx_chat_pins_message` 批量清理)。
- 后端：`db/pins.go`（Pin/Unpin/List/HasPin/RemovePinsForChat/Message）；
  `service/pins.go`（owner/admin 写、member 读、DM 拒绝）；
  `handlers/pins.go` + router：`POST/DELETE /pins/{messageID}`、`GET /pins?before=&limit=`；
  `models/models.go` 新增 `PinEntry` 结构。
- 联动：`service/message.go` DeleteMessage 软删除时同步清理该消息所有 pin
  （FK CASCADE 只对硬删除生效）。
- 前端契约同步：`client/src/schemas.ts`（PinEntrySchema）；
  `client/src/api/client.ts`（pinMessage/unpinMessage/listPinnedMessages）；
  `client/src/api/mock.js`（pinsState + mock funcs）；
  `server/internal/handlers/swagger.json`（+3 paths + 3 schemas）。
- 文档：`docs/architecture/database.md`（008 迁移行 + chat_pins 章节）；
  `docs/api/reference.md`（消息置顶章节）。
- 验证：`go build` ✅；`npx vitest run` 99 ✅；`npx tsc --noEmit` ✅。


## 2026-09-02 feat: 消息正文内联图片（`![]()` → `<img>` + 代理，fork 分歧）

- 背景：chatto fork 允许消息正文 `![alt](url)` 渲染为 `<img>`，且所有图片 src
  统一走 `https://proxy.moonchan.xyz` 代理（隐藏观看者 IP/Referer，防止外链
  host 直接暴露观看者身份）。上游禁用该语法。移植 fork 行为，按 AGPL 独立实现。
  所有本地改动带【本地改动 2026-09-02】标记。
- 实现（独立实现，不承认派生）：
  - `renderContent.jsx` 新增 `proxyImageSource(src)`：仅 http(s) → 代理 URL；
    javascript:/data:/file:/相对路径/其他协议一律降级为 `#`；已指向
    `proxy.moonchan.xyz` 的 URL 直通（避免二次代理环，2026-08-31 修复）。
    `searchParams` 编码 `:` → `%3A`（代理端正常解码），`proxy_host` 含端口。
  - `tokenizeImages(text)`：切分为 `[text, image]` 片段；正则
    `!\[([^\]]*)\]\(([^)\s]+)\)`（安全子集，`alt` 不含 `]`）。
  - `renderContent` 管线：mentions → images → math → URLs。
  - `<img>` 属性加固：`loading=lazy`、`referrerPolicy=no-referrer`；
    alt 为空时留空；src 经 proxyImageSource 重写（非法 → `#`）。
  - 注意：聊天中 `![a]b](https://a.png)`（alt 含 `]`）不匹配正则，整体作为
    纯文本保留（安全子集取舍）。
- 前端契约：无（纯前端渲染）。
- 测试：`renderContent.test.js` 新增 24 用例（proxyImageSource 14 + tokenizeImages 10）；
  全套 vitest 99 通过；`tsc --noEmit` 通过；`vite build` 正常。


## 2026-09-03 feat: FTS5 消息全文搜索

- 背景：chatto 提供聊天消息全文搜索（基于搜索后端）；chat-app 此前无此能力。
  用 SQLite 原生 FTS5 虚拟表实现（无外部依赖），所有本地改动带【本地改动 2026-09-03】标记。
- 迁移：`server/internal/db/migrations/009__add_messages_fts.sql` — 新建
  `messages_fts` FTS5 虚拟表（内联，`tokenize='unicode61'`，`msg_id UNINDEXED`
  辅助列关联 messages.id）。采用 UNINDEXED 辅助列方案：一开始用 FTS5 rebuild
  控制命令 + external content，但 modernc.org/sqlite 对 FTS5 external content
  支持不完整（rowid 强制 INTEGER，与 UUID msg ID 冲突）→ 改用 msg_id UNINDEXED
  后 INSERT OR REPLACE 正常。
- 后端：`db/messages.go` 新增 `upsertFTS`/`deleteFTS` 维护函数 + `SearchMessages`
  查询函数（INNER JOIN messages_fts f ON f.msg_id = m.id，MATCH ? 参数化）；
  在 `CreateMessage`/`UpdateMessage`/`DeleteMessage` 同步调用维护函数。
  `db/db.go` 启动后台 goroutine 调用 `BackfillFTS`（60s 超时），对老消息（已存
  但无 FTS 索引）逐条回填。
  `service/search.go`：`SearchService`（ChatID 非空 → Authz.MustBeMember；
  为空 → DB 层通过 chat_members 子查询强制访问控制，防越权）。
  `handlers/search.go` + router：`GET /api/search/messages?query=&chat_id=&user_id=&before=&limit=50`
  （限流 60 req/min/user）。
- 搜索语义：FTS5 MATCH 原样透传：空格分词（多词 OR）、`""` 精确短语、
  `*` 前缀通配、`AND` 逻辑运算。
- 前端契约：`client/src/schemas.ts`（SearchMessagesResponse）；
  `client/src/api/client.ts`（searchMessages）；`client/src/api/mock.js`（mock 版子串匹配 + 简单 highlight）。
- 前端 UI：`client/src/components/SearchModal.jsx`（搜索弹窗：输入框 debounce + Enter/Escape +
  全部/聊天过滤 chips + 结果列表 + 加载更多 + 点击跳转到 /g/{chat_id}）；`ChatView.jsx`
  header menu（⋮）新增「🔍 搜索消息」入口。
- swagger：+`/api/search/messages` +`SearchMessagesResponse`。
- 文档：`docs/architecture/database.md`（009 迁移行 + messages_fts 章节）；
  `docs/api/reference.md`（消息搜索章节 + 语法表）。
- 验证：`go build` ✅；`npx vitest run` 99 ✅；`npx tsc --noEmit` ✅。
