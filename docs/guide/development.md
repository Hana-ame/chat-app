# 开发工作流

## 测试

### 后端（Go）

```bash
cd server && go test ./...        # 10 个包
go vet ./...                      # 静态检查
```

覆盖范围：config、db（含迁移）、handlers、ws、ai（SSRF 校验）、service（权限/广播/流式）。

### 前端（Playwright）

```bash
cd client && npm test             # = ci.spec.mjs + real-time.spec.mjs（mock 模式，无需后端）
npx playwright test tests/e2e.spec.mjs   # 需要本地后端（真实 API）
npm run build                     # tsc --noEmit + vite build（构建前必跑）
```

| 套件 | 依赖 | 内容 |
|---|---|---|
| `ci.spec.mjs` | 仅 Mock | 登录、聊天列表、发送/编辑/删除消息、通知、设置、上传头像 |
| `real-time.spec.mjs` | 仅 Mock | WS/SSE/Poll 三传输的事件同步 |
| `e2e.spec.mjs` | 真实后端 | 全链路 E2E |
| `boundary.spec.mjs` / `ai-panel.spec.mjs` | — | 早期遗留弱断言套件，不作为 CI 目标 |

### Mock 模式（关键机制）

前端测试依赖 `client/src/api/mock.js`：`window.__mockLogin()` 启用 `api.enableMock()` 后，`client.ts` 的 Proxy 把所有 API 调用拦截到内存 mock 数据（聊天、消息、通知、上传等），不发起真实网络请求。注意：

- **mock 分支返回必须包 `Promise.resolve(...)`**（如 `notifications` 特殊分支），否则调用方 `.then()` 崩溃
- mock 数据为模块级，每个测试页面独立；`localStorage.clear()` 用于重置登录态
- Mock 模式下 WebSocket 传输由 `realtime/transports/mock.js` 模拟（定时事件）

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
