# 开发工作流

## 测试

测试体系总纲见 [testing.md](../testing.md)(金字塔、运行命令、断言/命名/
注释规范),Mock 策略见 [mock-strategy.md](../mock-strategy.md)。速记:

```bash
cd server && go vet ./... && go test ./... -count=1   # Go 全量(含 WS)
cd client && npm test                                  # vitest 单元测试
cd client && npm run test:e2e:mock                     # Playwright mock 模式(无需后端)
cd client && npm run test:e2e:full                     # 真实后端(需后端 :8080)
cd client && npm run build                             # tsc --noEmit + vite build
```

Vite dev server 由 playwright.config.js 的 `webServer` 托管(已有则复用、
缺失自动拉起并回收),无需手工起 vite;e2e 项目需要真实 Go 后端 :8080。

- Go 断言一律用 `testkit.Require*`(handlers 内部测试)或 `testutil.Require*`(转发)。
- 单元测试(Go)放包内 `*_test.go`;集成测试放 `internal/testutil/`;
  JS 单测与源码同目录 `*.test.js`;E2E 全在 `client/tests/`。
- **提交前**:`go test ./...` + `npm test` + `npm run build` 必须通过。

### Mock 模式（关键机制）

前端开发/E2E mock 依赖 `client/src/api/mock.js`:`window.__mockLogin()`
启用 `api.enableMock()` 后,`client.ts` 的 Proxy 把所有 API 调用拦截到
内存 mock 数据,不发起真实网络请求。注意:

- **mock 分支返回必须包 `Promise.resolve(...)`**（如 `notifications` 特殊分支），否则调用方 `.then()` 崩溃
- mock 数据为模块级，每个测试页面独立；`localStorage.clear()` 用于重置登录态
- Mock 模式下 WebSocket 传输由 `realtime/transports/mock.js` 模拟（定时事件）
- 这是**运行时代码**;单元测试禁止依赖它(用 vitest `vi.mock`),详见 mock-strategy.md

## CI 结构

见 [deployment.md](deployment.md#发布流程cicd)。本地可用 `scripts/deploy_local.py`：

```bash
python scripts/deploy_local.py build    # 编译前后端
python scripts/deploy_local.py start    # 启动（日志 → server.log）
python scripts/deploy_local.py kill     # 停止
python scripts/deploy_local.py restart  # kill + start
python scripts/deploy_local.py all      # 构建 + 启动
```

## 代码约定

- **后端分层**：`handlers`（HTTP 层）→ `service`（业务/权限/广播）→ `db`（数据访问）；`ws` 仅负责连接管理
- **前端状态**：Zustand store 持有状态，`realtime/coordinator` 统一管理传输层，组件不直接碰 transport
- **API 响应约定**：上传接口的 `url` 字段必须为绝对 URL（`CHAT_BASE_URL` 或 `X-Forwarded-Proto` + `Host` 推导）
- **改 API 字段先 grep 消费者**：`client/src/`、`docs/`、`scripts/` 中常有直接代码路径之外的消费者
- **配置优于魔法**：能从配置（`CHAT_*`）获取的值，不要从请求头临时拼凑
- **提交前**：`go test ./...` + `npm run build` 必须通过

## 修改日志

每次修改在 `docs/changelog.md` **末尾**追加条目（不要覆盖历史）。归档目录 `docs/archive/` 只进不出。
