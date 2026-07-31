# tests — Playwright E2E 测试目录

本目录只放 **浏览器级 E2E** 测试(Playwright)。前端单元测试不在
这里,一律放在 `client/src/**/*.test.{js,ts}` 下由 vitest 运行(见
`client/vitest.config.js`)。

## 文件说明

| 文件 | project | 依赖 | 用途 |
|------|---------|------|------|
| `ci.spec.mjs` | mock | 无后端 | Mock API 模式核心回归:登录、发消息、公告、建群、删除、成员面板、上传 |
| `real-time.spec.mjs` | mock | 无后端 | Mock 传输层的实时事件驱动:消息增删改、反应、聊天增删、未读数 |
| `ai-panel.spec.mjs` | mock | 无后端 | AI 面板 UI 与 SSE 流式请求(用 page.route 拦截真实 SSE) |
| `e2e.spec.mjs` | e2e | Go 后端 | 真实后端全流程:认证、建群、收发消息、公告、边界(超长消息/越权) |
| `upload_test.sh` | — | 外部服务 | 上传服务 `upload.moonchan.xyz` 可用性检查(curl) |

已删除:`boundary.spec.mjs`(骨架假绿,边界场景迁入 `e2e.spec.mjs` 真实
断言)、`boundary-runner.mjs`(孤儿文件,无任何引用)。

## 运行方式

所有 Playwright 测试要求 **Vite dev server 运行在 5173**(baseURL),
CI 中手动 `npx vite --port 5173 &` 启动;e2e 项目另需 Go 后端
(`:8080`,`/api` 由 Vite proxy 转发)。

```bash
cd client
npm install
npx playwright install chromium   # 首次

npm run test:e2e:mock   # 仅 mock 模式(不依赖后端)
npm run test:e2e:full   # 仅真实后端(先起后端)
npm run test:e2e        # 全部
```

单测(不需要浏览器):

```bash
npm test              # vitest run
npm run test:watch    # 监听模式
```

上传服务检查(依赖外部服务可用):

```bash
./client/tests/upload_test.sh
```

## Mock 分层说明(与测试的关系)

前端有三层 mock,测试各用各的:

1. **应用内建 Mock API**(`src/api/mock.js` + `client.ts` 的 Proxy):
   mock project 的 spec 通过 `window.__mockLogin()` 进入,覆盖 API 层
   与 mock transport(500ms 轮询)。详见 `docs/mock-strategy.md`。
2. **page.route 拦截**:仅 `ai-panel.spec.mjs` 用来注入真实格式的 SSE
   响应,绕过应用 mock 直接测流式渲染。
3. **vitest 单元 mock**(`vi.mock` / `vi.stubGlobal`):只出现在
   `src/**/*.test.js`,与 E2E 完全隔离。

## 注意事项

- mock project 不依赖后端,CI 必须保证它零外部依赖(若失败说明
  mock API 与源码脱节,属 bug 而非环境问题)。
- e2e 的边界测试用 Playwright `request` fixture 直连后端 API 断言
  错误码(403/413),比 UI 级断言更稳定。
- 新增 spec 文件时按 `playwright.config.js` 的 project `testMatch`
  规则命名(ci/real-time/ai-panel → mock;e2e → e2e)。

## 修改记录

| 日期 | 变更 |
|------|------|
| 2026-07-06 | 创建 `upload_test.sh` 与 `README.md` |
| 2026-08-01 | 大翻新:引入 vitest 单测、删除 boundary 骨架与孤儿 runner、边界场景迁入 e2e.spec.mjs、playwright.config.js 落地 projects 机制、重写本 README |
