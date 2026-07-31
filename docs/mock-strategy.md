# Mock 策略(三层体系与边界)

项目横跨 Go 后端与 JS 前端,历史上 mock 手段散落且互不说明,导致
"删了按钮忘了删测试"这类脱节。本文是 mock 的唯一权威说明。

## 总览

| 层 | Mock 手段 | 归属 | 用途 | 数据流 |
|---|---|---|---|---|
| 1 | Go `httptest` 假上游 | 后端测试 | 模拟 AI 上游等外部依赖 | 服务进程内,不进网络 |
| 2 | JS 应用内建 Mock API | 前端运行时代码 | 开发模式 / E2E mock project | 浏览器内存数据 |
| 3 | vitest `vi.mock` / `vi.stubGlobal` | JS 单元测试 | 隔离被测模块的依赖 | 进程内,不进浏览器 |
| 4 | Playwright `page.route` | E2E | 注入特殊响应(如真实 SSE) | 浏览器网络层拦截 |

**核心原则:**
- 每层只在自己的边界内工作,禁止跨界。
- **DB 永不 mock**(后端):测试一律用真实 SQLite 临时库(`t.TempDir()`),
  只有 AI 上游这类"外部服务"才用假实现。
- 单元测试不得依赖第 2 层(应用内建 mock);E2E 不得依赖第 3 层。

## 第 1 层:Go httptest 假上游

- 统一入口:`testkit.NewMockAIServer(t, chunks...)`(OpenAI 风格 SSE,
  校验 POST,chunk 逐个下发后跟 `[DONE]`,自动 Close)。
- 一次性场景(mock 错误响应、超时、非流式)写在测试文件内,
  命名 `mockXxx`,带注释说明行为。
- 不要引入 mock 生成器(gomock/testify):零第三方测试依赖是当前
  架构的优点,接口 mock 用真实实现 + 错误注入代替。

## 第 2 层:应用内建 Mock API(前端)

- 文件:`client/src/api/mock.js`(内存数据层)+ `client/src/api/client.ts`
  的 `buildMockProxy`(Proxy 拦截)+ `client/src/realtime/transports/mock.js`
  (500ms 轮询事件)。
- 入口:`window.__mockLogin()`(仅 DEV 环境挂载)→ localStorage 标记
  `chat:mock` → `api.enableMock()` → `setMode('mock')`。
- **边界:这是运行时代码,不是测试代码**。它服务两类消费者:
  1. 开发者本地无后端调试;
  2. E2E mock project(ci/real-time/ai-panel spec 走它)。
- 注意:`enableMock()` 里调用 `resetMockData()`(数据重置);mock 分支
  返回必须包 `Promise.resolve(...)`,否则调用方 `.then()` 崩溃。
- **禁止**:单元测试 `vi.mock('../api/client')` 时引用 mock.js 内部实现。

## 第 3 层:vitest 单元 mock

- 只存在于 `client/src/**/*.test.{js,ts}`。
- 手段:`vi.mock(模块, 工厂)`(工厂里可引用 `vi.hoisted` 共享状态)、
  `vi.stubGlobal('fetch'/'localStorage'/'window', ...)`。
- **禁止**:mock 被测模块自身;只 mock 它的依赖。
- 全局打桩用 `vi.unstubAllGlobals()`(beforeEach)清理。

## 第 4 层:Playwright page.route

- 只在 `tests/ai-panel.spec.mjs` 使用:拦截 `/api/chats/*/messages`
  返回真实格式 SSE,验证前端流式渲染与请求体构造。
- 这是唯一允许的 route 拦截场景(应用内建 mock 覆盖不了"真实 SSE 格式")。
- **禁止**:用它模拟常规 CRUD 场景(应用内建 mock 已覆盖)。

## 历史与迁移说明

- `server/internal/testkit/mockai.go` 统一了 service_test.go 与
  testutil/ai_stream_test.go 各自手写的假 AI 上游。
- 曾被删除:`client/tests/boundary.spec.mjs`(mock 模式下用户恒为 owner,
  无法测错误路径,空断言假绿)与 `boundary-runner.mjs`(孤儿文件)。
  其边界场景已迁入 `e2e.spec.mjs` 用真实后端断言(403/413)。
