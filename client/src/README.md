# client/src — 前端源码

## 目录速查（改代码前先定位）

| 目录 | 职责 | 关键文件 |
|---|---|---|
| `api/` | HTTP 请求层 + 类型 + mock 实现 | `client.ts`（方法 + Mock Proxy）· `mock.js`（内存 mock 数据）· `schemas.ts`（类型/校验） |
| `store/` | Zustand 状态 | `auth.js` · `chat.js`（chats/messages/mode + 事件 handler）· `notification.js` |
| `realtime/` | 实时传输协调 | `coordinator.js`（调度）· `opHandlers.js`（事件 handler）· `transports/`（ws/sse/poll/mock） |
| `components/` | UI 组件（19 个） | 见下方组件表 |
| `routes/` | 页面 | `LoginPage.jsx` · `RegisterPage.jsx` · `ChatPage.jsx`（/ 与 /g/:chatId） |
| `hooks/` | 共享 hooks | `useEscapeKey.js` · `useMembers.js` |
| `utils/` | 工具 | `ai.js`（流式组装）· `notifyMessage.js` · `browserNotify.js` |
| `dev/` | 假数据 | `dummy.js`（mock 聊天/消息数据） |

## API 层

- `client.ts` 定义全部 API 方法与类型；内部 `request()` 封装 fetch（JSON、Bearer token、credentials）
- `enableMock()` 后 Proxy 拦截方法调用 → `mock.js` 内存实现（登录、聊天、消息、通知、上传全覆盖）
- mock 分支返回**必须**包 `Promise.resolve(...)`（调用方 `.then()` 依赖）
- 进入 mock 模式：dev 页执行 `window.__mockLogin()`（详见 [docs/guide/development.md](../../docs/guide/development.md)）
- **改 API 响应字段时必须同步**：`schemas.ts`（类型）+ `mock.js`/`dev/dummy.js`（mock 数据）+ 后端 `models`（JSON 契约）

## 实时层

- `coordinator.js` 统一管理 ws/sse/poll/mock 四种传输；模式由 `store/chat.js` 的 `mode` 驱动
- 事件 handler 注册在 `coord.setHandlers()`，store 内实现（onMessageCreate 等）
- 协议见 [docs/architecture/realtime.md](../../docs/architecture/realtime.md)

## 组件表

| 组件 | 职责 |
|---|---|
| `ChatList.jsx` / `ChatListItem.jsx` | 侧栏：列表、搜索、建群、公共频道 |
| `ChatView.jsx` | 中间：消息列表 + Composer |
| `MessageList.jsx` / `MessageItem.jsx` | 消息渲染（滚动加载、编辑/删除/反应/回复） |
| `Composer.jsx` | 输入区：文本 + 附件 + AI 流式 |
| `MemberPanel.jsx` / `MemberList.jsx` | 右侧栏：成员管理 |
| `ChatInfoModal.jsx` | 聊天设置（改名/头像/横幅/置顶等） |
| `UserProfileModal.jsx` / `UserAvatar.jsx` | 用户资料/头像 |
| `CreateGroupForm.jsx` / `PublicChannelList.jsx` | 建群/发现公开频道 |
| `WelcomeView.jsx` / `EmptyState.jsx` | 空态/引导 |
| `SidebarFooter.jsx` | 侧栏底部：用户信息/设置入口 |
| `Toast.jsx` / `ScrollArea.jsx` / `renderContent.jsx` | 通用（提示/滚动/markdown 渲染） |

## 测试

- 单元测试与源码同目录：`<file>.test.js` / `schemas.test.ts`（vitest）
- 浏览器 E2E 在 `tests/`，规则见 [tests/README.md](../tests/README.md)
