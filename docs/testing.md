# 测试体系总纲

本文是**唯一权威**的测试文档。测试分层、运行命令、命名与注释规范、
Mock 策略(另见 [mock-strategy.md](mock-strategy.md))都在这里定义。
修改测试体系前先读本文;新增测试时必须遵循本文约定。

## 测试金字塔

```
        E2E(Playwright, 真实后端)      少量 · 核心用户旅程
      集成(testutil 黑盒 HTTP + WS)   中等 · 业务全链路与权限
     单元(vitest / Go 包内测试)       大量 · 纯逻辑与边界
```

| 层 | 位置 | 依赖 | 典型内容 |
|---|---|---|---|
| 单元(Go) | `server/internal/<pkg>/<subject>_test.go` | 无(或 t.TempDir 临时 SQLite) | 工具函数、纯逻辑、DB CRUD、SSE 解析 |
| 集成(Go) | `server/internal/testutil/*_test.go` | testutil.Fixture(真实 SQLite + httptest) | 认证流程、HTTP 全链路、WS 端到端、AI 流回放 |
| 单元(JS) | `client/src/**/*.test.{js,ts}`(与源码同目录) | vitest | streamAI、schemas、store reducer、api request |
| E2E(JS) | `client/tests/*.spec.mjs` | Playwright + Vite(;e2e project 另需后端) | 登录/建群/发消息/公告/边界 |

## 运行命令

### Go 后端

```bash
cd server
go vet ./...                 # 静态检查(必跑)
go test ./... -count=1       # 全量测试(含 WS,默认启用)
go test ./internal/ws/ -v    # 单包
```

### 前端

```bash
cd client
npm test                     # vitest 单元测试
npm run test:e2e:mock        # Playwright mock 模式(无需后端)
npm run test:e2e:full        # Playwright 真实后端(需后端 :8080)
npm run test:e2e             # 全部 E2E
npm run build                # tsc --noEmit + vite build(构建前必跑)
```

Vite dev server 由 `playwright.config.js` 的 `webServer` 托管:
`reuseExistingServer: true` → 已有 vite 则复用,缺失则 Playwright 自动拉起
并在跑完后回收(无需手工起 vite,也不用担心后台进程被杀后连不上)。
CI 仍手动起 vite,配置与之兼容。另外配置内已强制 `NO_PROXY` 包含
`127.0.0.1,localhost`:本机若导出 HTTP(S)_PROXY,Node 的 fetch 会把本机
地址也代理出去,导致 webServer 健康检查拿到代理 500 而误判。

CI 中每个 job 只跑自己负责的层,详见 `.github/workflows/`(ci.yml =
Go 测试 + 前端构建 + vitest;frontend-ci.yml = 单测 / mock E2E / full E2E)。

## 命名与文件布局

**Go:**
- 单元测试:`<subject>_test.go` 与源码同目录同包(需要测私有函数时用内部包名,
  否则用 `<pkg>_test` 外部包名走公共 API)。
- 集成测试:统一放 `server/internal/testutil/`,按领域拆文件
  (`auth_flow_test.go` / `handler_test.go` / `ai_stream_test.go`),不用内部包名。
- 假上游/假服务:可复用的放 `internal/testkit/`(零依赖叶子包,见下),
  一次性场景放测试文件内,命名 `mockXxx` 前缀。
- **testkit 与 testutil 的分工**:
  - `internal/testkit/` — 只依赖标准库,任何包都能 import(handlers 内部
    测试用这个,避免 import cycle):断言助手 `Require*`、`NewMockAIServer`。
  - `internal/testutil/` — 依赖 handlers/service 等业务包,提供 `Fixture`
    装配 + HTTP client(`Register/Login/Do/WSURL`),并把 testkit 断言
    薄转发为 `testutil.Require*`。

**JS:**
- 单元测试:与被测文件同目录、同名 `<file>.test.js`。
- E2E:全部在 `client/tests/`,由 `playwright.config.js` 的 project 划分
  (ci/real-time/ai-panel → mock;e2e → e2e)。
- 不要创建 `tests/` 目录之外的 spec 文件;不要往 `src/` 里放 E2E。

## 断言规范

**Go(强制):** 一律使用 `testkit.Require*`(或经 `testutil.Require*` 转发),
禁止手写 `if x != y { t.Fatalf(...) }`。新增断言助手进 `testkit/assert.go`。

**JS:** vitest 的 `expect`;断言意图要带上下文描述(`expect(x).toBe(y)`
不足以说明时用 message 参数)。

## 文件头注释规范(Go 测试文件)

每个测试文件第一行必须是块注释,包含:

```go
// Package xxx 覆盖 <被测模块>:<一句话范围>。
//
// 运行方式: cd server && go test ./internal/xxx/
// 说明:<依赖/mock 策略/特殊约定>。
package xxx_test
```

JS 测试文件同理(第一行块注释写范围 + 运行命令)。

## Mock 策略

三套 mock 各有边界,见 [mock-strategy.md](mock-strategy.md) 全文。速记:

| Mock 手段 | 用途 | 禁止 |
|---|---|---|
| Go httptest 假上游(testkit.NewMockAIServer) | AI 流等外部依赖 | 不 mock DB(用真实 SQLite) |
| JS 应用内建 Mock(mock.js + Proxy) | 开发模式 / E2E mock project | 单元测试不得依赖它 |
| vitest `vi.mock` / `vi.stubGlobal` | 单元测试 | 不 mock 被测模块自身 |
| Playwright `page.route` | 特殊 SSE 注入 | 常规请求拦截 |

## 新增/修改代码时的测试义务

1. 新增导出函数 → 必须带测试(除非是 `main` 入口)。
2. 修改 API 响应字段 → 先 grep 消费者(`client/src/`、`tests/`、`docs/api/reference.md`),
   再改对应测试契约(models_test.go 的 JSON 断言、schemas.test.ts、e2e 断言)。
3. 删除/重命名 UI 元素或按钮 → 同步清理/重写引用它的 spec(历史教训:
   `button[title="New DM"]` 删除后测试只 skip 不删,变成死用例)。
4. 提交前:本地必须 `go test ./...` + `npm test` + `npm run build` 全绿;
   E2E 至少 mock 模式绿。
5. 每轮大改动后做一次"同步审计":逐包对照源码导出符号与测试引用,
   掉队即补(见 AGENTS.md 通用原则)。

## 已知边界

- 后端有硬编码 IP 限流(router.go):`/auth/register` 5 次/分钟、`/auth/login`
  10 次/分钟、全局 120 次/分钟。e2e 因此用 beforeAll 建立**固定邮箱**用户池
  (ui/owner/member):先登录复用、不存在才注册 → 反复重跑几乎不再消耗
  register 窗口;429 仍按 Retry-After 自动重试(最多 12 次)。429 路径本身
  不做断言(触发它会耗尽限流窗口,连累其他用例)。
- 固定邮箱在 CI 每次全新后端上首次跑时会注册,重复跑时登录复用,幂等。
- `/auth/register` 返回 200(而非 201);已删除聊天的 `GET /chats/{id}`
  返回 403(防探测设计:MustBeMember 先于存在性检查)。e2e 断言据此编写。
- `e2e.spec.mjs` 用 Playwright `request` fixture 直连 API 断言错误码,
   比 UI 级断言稳定;UI 级断言仅用于正常流程。
- WS 测试默认启用(已去 WS_ENABLED 门控);`WS_ENABLED` 环境变量仅作
   生产环境显式关闭 WS 的开关(gateway.go)。
