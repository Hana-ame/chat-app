# 前端测试评估报告 (Frontend Test Assessment)

审查日期：2026-07-10
审查范围：`client/tests/` + `client/src/api/mock.js` + `client/src/store/` + `server/internal/testutil/`

---

## 一、本项目的测试全貌

### 1.1 测试分层架构

```
┌─────────────────────────────────────────────────────────────┐
│                    生产环境（手动测试）                        │
│  无法自动化的：视觉回归、响应式布局细节、真实网络场景            │
├─────────────────────────────────────────────────────────────┤
│                    前端 E2E（8 个测试）                       │
│  真实后端 + 真实浏览器，覆盖注册→发消息→公告栏完整用户旅程      │
│  运行在 CI full-e2e job（需 Go 后端）                        │
├─────────────────────────────────────────────────────────────┤
│                 前端 Mock CI（15 个测试）                      │
│  28/28 Mock API 全覆盖，无后端依赖，CI 最先执行的 Job          │
│  覆盖：登录、发消息、公告栏、群聊管理、成员管理、文件上传        │
├─────────────────────────────────────────────────────────────┤
│                 后端集成测试（~68 个测试）                     │
│  真实 SQLite + 完整 Chi 路由，覆盖 27/29 HTTP 端点             │
│  HTTP 请求/响应管道全链路，含权限检查、输入验证、错误码          │
├─────────────────────────────────────────────────────────────┤
│                 后端单元测试（~66 个测试）                     │
│  36 个 DAO 导出函数全覆盖，无 mock，每测试独立 SQLite 文件      │
│  覆盖：ErrNotFound ×10、ErrConflict ×4、空列表 ×5 等边界       │
└─────────────────────────────────────────────────────────────┘
```

### 1.2 测试数量统计

| 层 | 测试数 | 文件 | 运行时间 | 依赖 |
|---|--------|------|---------|------|
| 前端 Mock CI | 15 | `ci.spec.mjs` | ~30s | 无 |
| 前端 E2E | 8 | `e2e.spec.mjs` | ~60s | Go 后端 |
| 前端合计 | 23 | 2 文件 | ~90s | |
| 后端 DB 单元测试 | ~66 | `db_test.go` + `messages_test.go` | ~1.7s | 无 |
| 后端集成测试 | ~68 | `handler_test.go` + `auth_flow_test.go` + `integration_test.go` | ~9s | 无 |
| 后端 WS 测试 | ~6 | `ws_test.go` | ~0.01s | 无 |
| 后端合计 | ~134 | 5 文件 | ~11s | |
| **总计** | **~157** | **7 文件** | **~100s** | |

---

## 二、每个测试在测什么（详细）

### 2.1 前端 Mock CI 测试（15 个）

| # | 测试名 | 它实际在验证什么 | 覆盖的前端逻辑 |
|---|--------|----------------|-------------|
| 1 | `debug mode toggle shows mock button` | Debug mode 复选框 → Quick Enter 按钮出现 | RegisterPage Debug 模式 UI 切换 |
| 2 | `mock login shows sidebar` | 点 Quick Enter → URL 变成 `/` → 侧边栏渲染 | `auth.js` mockLogin → `enableMock()` → React Router 跳转 → ChatPage 挂载 |
| 3 | `mock login shows chat items` | 聊天列表从 Mock API 加载并渲染 | `useChatStore.loadChats` → `mockListChats` → `.chat-item` DOM |
| 4 | `mock send message` | 输入文本 → Send → 消息出现在列表中 | `Composer.handleSend` → `mockSendMessage` → AI 回复 → `msg-content` 渲染 |
| 5 | `mock notice board: set, edit, clear` | 公告栏 Set → Edit → Clear 完整 CRUD | `ChatView` 公告栏 UI + `setPinnedMessage` / `clearPinnedMessage` |
| 6 | `logout returns to login` | Logout → URL `/login` → h1 正确 | `auth.js` logout → `disableMock()` → 清除 localStorage → 路由跳转 |
| 7 | `mock create and rename group chat` | Create Group → 改名完整流程 | `ChatList` CreateGroupForm + `createChat` + `renameChat` |
| 8 | `mock create DM via search` | 搜索用户 → 创建 DM | `DmSearchPanel` + `searchUsers` + `createDM` |
| 9 | `mock edit and delete message` | 编辑消息内容 + 删除消息 | `MessageItem` 内联编辑/删除按钮 + `editMessage` / `deleteMessage` |
| 10 | `mock delete chat from context menu` | 右键菜单删除聊天 | `ChatListItem` 右键菜单 + `deleteChat` |
| 11 | `mock member panel interaction` | 成员面板打开 → 搜索用户 | `MemberPanel` + `addMember` |
| 12 | `mock public channels search` | 公开频道 UI | `PublicChannelList` + `joinChat` |
| 13 | `mock open settings and close` | 设置页打开 → 关闭 | `SettingsModal` 渲染/关闭 |
| 14 | `mock upload file to composer` | 📎 按钮 → 选文件 → 附件显示 | `Composer.handleFile` → `api.upload` → `.file-attach` DOM |
| 15 | `mock upload avatar in settings` | 点头像 → 选文件 → Save | `SettingsModal` 头像上传 → `api.uploadAvatar` |

### 2.2 前端 E2E 测试（8 个）

| # | 测试名 | 实际验证 | 覆盖的后端端点 |
|---|--------|---------|-------------|
| 1 | `home redirects to login` | 无 accessToken → React Router 重定向 | 无（纯前端路由） |
| 2 | `login form renders correctly` | 登录页有 h1 + inputs + button | 无（纯 UI） |
| 3 | `register form renders correctly` | 注册页有 h1 + inputs | 无（纯 UI） |
| 4 | `full auth flow` | 真实注册 → 后端返回 token → 跳转到 `/` | `POST /api/auth/register` |
| 5 | `create group chat` | 注册 → 创建群聊 → header 显示群名 | `register` + `POST /api/chats` |
| 6 | `send and receive message` | 注册 → 建群 → 发消息 → 消息出现 | `register` + `chats` + `*/messages` |
| 7 | `responsive layout on mobile` | 375px 视口下表单可见 | 无（纯 CSS） |
| 8 | `notice board as owner` | 注册 → 建群 → Set/Edit/Clear 公告栏 | `register` + `chats` + `*/pin` |

### 2.3 后端测试（~134 个）

**DB 层单元测试（66 个）：**
- 每个 DAO 函数的 happy path + error path
- `CreateUser` → 重复 email → `ErrConflict`
- `FindDMBetween` → 不存在 → `ErrNotFound`
- `AddReaction` → 重复 emoji → no-op
- `GetMessages` → 空聊天 → 空列表
- `DeleteChat` → 不存在的聊天 → no-op

**Handler 集成测试（68 个）：**
- 每个 HTTP 端点的 200 + 400 + 403 + 401 + 404 + 409 + 413 + 415
- 权限矩阵：non-member 403、non-author 404、踢群主 403
- 输入验证：空内容 400、超长 400、JSON 格式错误 400
- 刷新 token 轮换：并发刷新仅一个成功

**WebSocket 测试（6 个）：**
- `ready` 事件：连接后收到 user + chats
- `ping/pong`：心跳互答
- `message_created`：Alice 发消息 → Bob 收到广播
- `typing`：成员收到 typing 事件
- `presence`：连接/断开 → onlineUserIds 变化

---

## 三、测试结果可靠吗

### ✅ 可靠的方面

| 方面 | 可靠性 | 理由 |
|------|--------|------|
| Mock API 覆盖 | **高** | 28/28 方法全部有 Mock 实现并在 CI 中触发 |
| 后端 DB 层 | **高** | 每测试独立 SQLite 文件，无状态泄漏，66 个测试覆盖所有边界 |
| 后端集成层 | **高** | 真实 SQLite + 真实 HTTP server，~68 个测试覆盖权限矩阵 |
| 前端 Mock 测试 | **中高** | 无后端依赖，15 个测试稳定通过（CI 中多次验证） |
| CI/CD 一致性 | **高** | 用 `npm ci` 锁定依赖，双流水线全绿 |

### ⚠️ 不可靠/风险较高的方面

| 方面 | 风险 | 原因 |
|------|------|------|
| E2E 测试 | **中** | 依赖后端在 CI 中编译+启动，偶发时序问题。每次 commit 都会重新编译 Go（~30s） |
| 前端 Mock API 与真实 API 的一致性 | **中** | Mock 是手工维护的，真实后端 API 变更后 Mock 可能不同步。无自动对比机制 |
| AI 回复验证 | **低** | Mock 的 AI 回复是固定文本，不验证真正的 AI 集成 |
| WebSocket 实时同步 | **高** | **完全未测试。** Mock 模式用 polling，E2E 用 HTTP 代理，没有测试 WebSocket 连接和实时事件推送 |
| 流式消息（streaming） | **高** | **完全未测试。** AI 打字效果通过 `startConsumingStream` 实现，但没有测试验证流式内容逐步渲染 |
| 同时多用户场景 | **高** | **完全未测试。** 所有测试都是单用户操作，没有测试两个用户同时在线时的消息同步、Presence 更新 |
| 重连/断线恢复 | **高** | **完全未测试。** WebSocket 断线后 3s 自动重连，但没有测试验证重连后状态一致性 |
| 并发/竞态条件 | **高** | **完全未测试。** 两个用户同时发消息、同时编辑同一条消息、token 刷新与请求并发 |
| 大规模数据 | **中** | Mock 只有 10 个聊天 × 150 条消息，没有测试 1000+ 聊天/10000+ 消息时的性能 |
| 429 限流 | **中** | 后端 API 有 30/min 限流，前端 `request` 函数能捕获 429 并 alert，但没有测试验证限流触发时的行为 |
| 401 自动刷新 | **中** | `request` 函数有 401 自动 refresh 逻辑，但没有测试验证 refresh 成功后重试原请求 |

---

## 四、Playwright 做不到的事情

### 4.1 架构性限制（Playwright 本身）

| 做不到的事 | 为什么 Playwright 做不到 | 替代方案 |
|-----------|----------------------|---------|
| **WebSocket 消息断言** | Playwright 可以监听 WebSocket 事件（`page.waitForEvent('websocket')`），但不能可靠地断言特定消息被发送到特定客户端 | 后端 WS 测试（`ws_test.go` 6 个测试）+ 手动测试 |
| **多用户/多浏览器同步** | Playwright 单个 `page` 实例模拟一个用户。多用户需要多个 `context` 或 `browser`，但时序协调极其复杂 | 后端集成测试（多 token 模拟多用户）+ Zulip 风格的 `verify_action` |
| **流式渲染逐帧验证** | `toContainText` 是最终断言，不能逐字符验证 AI 打字效果 | 后端测试 + 手动测试 |
| **视觉回归** | `toHaveScreenshot()` 存在但 CI 环境字体/抗锯齿不同导致假阳性 | 人工 QA |
| **真实移动设备** | Playwright 可以设 viewport 但不能模拟真机触摸、性能、网络切换 | 真机测试（BrowserStack） |
| **网络条件模拟** | Playwright 可以 `page.route` 拦截但无法模拟真实丢包/高延迟 | 后端 chaos testing |
| **性能/负载测试** | Playwright 单浏览器单线程 | k6 / Artillery |
| **安全检查（XSS/SQLi）** | Playwright 可以输入恶意 payload 但无法检测后端 SQL 注入 | 后端安全扫描（SQLMap, ZAP） |

### 4.2 本项目未覆盖但关键的功能

| 功能 | 是否有测试 | 风险等级 | 建议 |
|------|----------|---------|------|
| WebSocket 连接建立 | ❌ 无 | **高** | 添加 WS 连接 E2E 测试或后端 WS 集成测试 |
| 实时消息广播（A 发→B 收） | ❌ 无 | **高** | 后端 `ws_test.go` 有 1 个测试，前端无验证 |
| 断线重连 | ❌ 无 | **高** | `chat.js` 第 88 行 `setTimeout(() => reconnect, 3000)` 从未被测试 |
| 多设备登录 | ❌ 无 | **中** | 后端 `TestMultiDeviceRefreshIsolation` 测试了 token 隔离，前端无 |
| 消息未读计数 | ❌ 无 | **中** | `unread_count` 在 store 中维护但无测试验证增减逻辑 |
| 成员在线状态（Presence） | ❌ 无 | **中** | `onlineUserIds` store + WS `presence_update` 未测试 |
| 输入指示器（Typing） | ❌ 无 | **中** | `sendTyping` 通过 WS 发送但未验证 |
| Markdown 渲染 | ❌ 无 | **中** | `renderContent.jsx` 用 `react-markdown` 但无测试验证渲染结果 |
| @提及解析 | ❌ 无 | **中** | 同上，未测试 @提及的链接化 |
| XSS 防护 | ❌ 无 | **中** | CSP header + HTML 转义存在但未验证绕过 |
| 附件上传验证 | ⚠️ 部分 | **低** | Mock 测试验证了 UI，未验证真正上传到 upload.moonchan.xyz |
| 长消息/超长内容 | ❌ 无 | **低** | 后端有 4000 字符上限测试，前端无 |
| 空消息/仅附件 | ❌ 无 | **低** | 后端有测试，前端无 |
| 表情选择器交互 | ⚠️ 部分 | **低** | Mock 测试验证按钮存在，未验证 emoji 点击→reaction 出现 |
| 搜索功能 | ⚠️ 部分 | **低** | Mock 测试验证搜索 UI 出现，未验证搜索结果准确性 |
| 深色模式切换 | ❌ 无 | **低** | 项目可能支持，未测试 |

---

## 五、与成熟 Chat App 对比

### 5.1 Zulip（最成熟的 Open Source Chat App）

| 方面 | Zulip | 本项目 | 差距 |
|------|-------|--------|------|
| 后端覆盖率 | **~98%**（强制 CI 门槛） | ~100% DAO + ~93% handler（27/29 端点） | 接近 |
| 前端测试策略 | Node 单元测试优先，E2E 最小化 | Playwright E2E + Mock CI（15 个） | 本项目更依赖 E2E |
| 实时事件测试 | `verify_action()` 系统——模拟 initial fetch 和 event delivery 的竞态 | **无** | **最大差距** |
| SQL 性能测试 | `queries_captured()` 断言查询次数防 N+1 | **无** | 中等差距 |
| 安全测试 | 集中式 `access_*_by_*` 函数，每条件一行 | 分散在各 handler 测试中 | 功能上覆盖但可维护性差 |
| 外部网络 | **禁用**——monkey-patch HTTP 库抛异常 | **不限制**——upload 测试会上传真实文件 | 本项目更脆弱 |
| 视觉测试 | 承认 "CSS 唯一方法是手动查看" | 无截图对比 | 一致 |
| API 契约测试 | Schema checker + OpenAPI 自动验证 | **无** | 中等差距 |
| 测试维护成本 | Puppeteer 测试 "slow, costly, flaky" | Playwright 15 个 Mock 测试稳定 | 本项目 Mock 模式优于 Puppeteer |
| CI 时间 | Django 项目，~10-15 分钟 | **~3 分钟** | 本项目更快 |

### 5.2 Rocket.Chat

| 方面 | Rocket.Chat | 本项目 |
|------|-------------|--------|
| 前端测试 | Jest + Playwright | Playwright |
| E2E 稳定性 | 活跃地修复 flaky 测试 | 15/15 稳定 |
| 实时测试 | 有 WebSocket mock | **无** |

### 5.3 Mattermost

| 方面 | Mattermost | 本项目 |
|------|-----------|--------|
| 后端 | Go + 集成测试 | Go + 集成测试（相似） |
| 前端 | Cypress E2E | Playwright |
| CI | Jenkins + Docker | GitHub Actions |

---

## 六、测试差距的根因

```
缺失测试                   根因
──────────────────────────────────────────────────
WebSocket 实时同步    ←  Mock 模式用 polling，跳过 WS
多用户场景            ←  单个 Playwright page 限制
流式消息              ←  Mock AI 回复固定文本
断线重连              ←  无网络条件模拟
未读计数              ←  Store 逻辑无单元测试
Presence              ←  无多用户 WS 测试
Typing                ←  无 WS 消息断言
Markdown 渲染         ←  无组件单元测试
XSS 验证              ←  无安全扫描集成
API 契约一致性        ←  Mock 手工维护
```

---

## 七、是否可以投入生产

### ✅ 可以投入生产的理由

1. **后端核心逻辑全覆盖** —— 36 个 DAO 函数 + 27/29 handler 端点 + WebSocket 基础流程
2. **认证安全经过验证** —— JWT 签发/解析、bcrypt 哈希、refresh token 轮换、并发刷新互斥
3. **前端基本流程正常** —— 注册、登录、发消息、建群、公告栏、文件上传
4. **CI/CD 自动化** —— 每次 push 自动跑 157 个测试
5. **错误路径覆盖** —— ErrNotFound、ErrConflict、403/401/400/409 全覆盖

### ⚠️ 投入生产前必须修复的差距

| 优先级 | 差距 | 影响 | 修复工作量 |
|--------|------|------|----------|
| **P0** | WebSocket 实时消息未测试 | 核心功能——聊天没有消息推送等于不可用 | 2-3 天（建立 WS mock 或真实 WS E2E） |
| **P0** | 无多用户场景验证 | 无法保证 A 发消息 B 能看到 | 1-2 天（多 browser context 测试） |
| **P1** | Mock 与真实 API 的一致性 | Mock 通过但真实后端 API 改了，前端崩溃 | 1 天（API 契约测试或自动对比） |
| **P1** | 断线重连未测试 | 用户网络波动后数据丢失 | 1 天 |
| **P2** | Markdown/@提及 未测试 | 显示异常不会被发现 | 0.5 天 |
| **P2** | 未读计数未测试 | 使用体验问题 | 0.5 天 |
| **P3** | XSS 安全验证 | 安全风险 | 1 天（安全扫描工具） |
| **P3** | 视觉回归 | UI 样式异常 | 1 天（Playwright 截图 + CI 对比） |

---

## 八、建议的测试改进路线图

### Phase 1（1-2 天）：WebSocket + 多用户测试

```javascript
// 思路：用 Playwright 的多个 browser context 模拟多用户
test('Alice sends message, Bob receives it via WS', async ({ browser }) => {
  const aliceCtx = await browser.newContext();
  const bobCtx = await browser.newContext();
  const alicePage = await aliceCtx.newPage();
  const bobPage = await bobCtx.newPage();
  
  // 分别在两个 context 中注册/登录
  // Alice 发消息 → Bob 的聊天列表应该更新
});
```

### Phase 2（1 天）：API 契约测试

```
为每个 API 端点定义 JSON Schema：
- POST /api/auth/register → 返回 { user: {...}, access_token: "..." }
- GET /api/chats/my → 返回 { chats: [...] }

Mock 实现必须通过 schema 验证，否则 CI 失败。
```

### Phase 3（0.5 天）：Store 单元测试

```javascript
// Zustand store 可以脱离 React 测试
import { useChatStore } from './store/chat';

test('onChatUpdate sets pinnedMessage correctly', () => {
  useChatStore.getState().onChatUpdate({
    id: 'chat-1',
    pinned_message: { content: 'notice!' }
  });
  expect(useChatStore.getState().pinnedMessage['chat-1']).toBe('notice!');
});
```

### Phase 4（1 天）：Playwright 截图对比

```javascript
test('login page matches design', async ({ page }) => {
  await page.goto('/login');
  await expect(page).toHaveScreenshot('login-page.png');
});
```

---

## 九、总结

| 维度 | 评分 | 说明 |
|------|------|------|
| 后端测试完整性 | ⭐⭐⭐⭐⭐ | 36/36 DAO + 27/29 handler，错误路径全覆盖 |
| 前端 Mock 覆盖 | ⭐⭐⭐⭐⭐ | 28/28 Mock API，15 个 CI 测试，100% 方法覆盖 |
| 前端 E2E 覆盖 | ⭐⭐⭐ | 基本用户旅程覆盖，但缺实时和多用户 |
| 实时功能测试 | ⭐ | WebSocket、流式、多用户、重连均未测试 |
| 安全测试 | ⭐⭐⭐ | CSP header + XSS 防护存在未验证 |
| 性能测试 | ⭐ | 无 |
| 视觉测试 | ⭐ | 无截图对比 |
| 测试可靠性 | ⭐⭐⭐⭐ | Mock 测试稳定，E2E 偶发失败 |
| CI/CD 自动化 | ⭐⭐⭐⭐⭐ | 双流水线，~3 分钟，自动 Release |

**结论：基本功能可以投入生产，但 WebSocket 实时功能和多用户场景存在高风险。建议在 Phase 1（1-2 天）解决这两个 P0 差距后再正式上线。**