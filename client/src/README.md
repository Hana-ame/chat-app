# client/src — 前端源码

## 目录结构

```
src/
├── api/                    # API 层（见下方"API 层"）
│   ├── client.ts           # 方法定义 + Mock Proxy（buildMockProxy）
│   ├── mock.js             # 内存 mock 数据（dev/E2E mock 模式用）
│   ├── schemas.ts          # 前端类型定义 + 校验
│   └── README.md
├── realtime/               # 实时层
│   ├── coordinator.js      # 传输调度（connect/disconnect/setHandlers）
│   ├── fetchStream.js      # SSE 流式读取
│   └── transports/         # ws.js / sse.js / poll.js / mock.js
├── store/                  # Zustand 状态
│   ├── auth.js             # 用户、登录态、token
│   ├── chat.js             # chats/messages/activeChatId/mode + 事件 handler
│   └── notification.js     # 通知消息
├── components/             # UI 组件
│   ├── ChatList.jsx        # 侧栏：列表、搜索、建群、公共频道
│   ├── ChatView.jsx        # 中间：消息列表 + Composer
│   ├── MessageList.jsx     # 消息列表（滚动加载、占位态）
│   ├── MessageItem.jsx     # 单条消息：渲染/编辑/删除/反应/回复
│   ├── Composer.jsx        # 输入区：文本 + 附件 + AI 流式
│   ├── MemberPanel.jsx / MemberList.jsx   # 右侧栏：成员管理
│   ├── ChatInfoModal.jsx   # 聊天设置（改名/头像/横幅/置顶等）
│   ├── UserProfileModal.jsx / UserAvatar.jsx / ImagePreviewModal.jsx
│   ├── CreateGroupForm.jsx / PublicChannelList.jsx / WelcomeView.jsx / EmptyState.jsx
│   ├── SidebarFooter.jsx   # 侧栏底部：用户信息/设置入口
│   ├── Toast.jsx / ScrollArea.jsx / renderContent.jsx
├── routes/                 # 页面
│   ├── LoginPage.jsx       # /login
│   ├── RegisterPage.jsx    # /register
│   └── ChatPage.jsx        # / 与 /g/:chatId — 主页面（含通知页路由）
├── hooks/                  # useEscapeKey.js · useMembers.js
├── utils/                  # ai.js（流式组装）· notifyMessage.js · browserNotify.js
├── dev/                    # dummy.js（假数据）· stream-source.js（AI 流源）
├── styles/global.css       # 全局样式变量和布局
├── App.jsx                 # 路由分发 + 401 处理
└── main.jsx                # 入口：ReactDOM + BrowserRouter
```

## API 层

- `client.ts` 定义全部 API 方法与类型；内部 `request()` 封装 fetch（JSON、Bearer token、credentials）
- `enableMock()` 后 Proxy 拦截方法调用 → `mock.js` 内存实现（登录、聊天、消息、通知、上传全覆盖）
- mock 分支返回**必须**包 `Promise.resolve(...)`（调用方 `.then()` 依赖）
- 进入 mock 模式：dev 页执行 `window.__mockLogin()`（详见 [docs/guide/development.md](../../docs/guide/development.md)）

## 实时层

- `coordinator.js` 统一管理 ws/sse/poll/mock 四种传输；模式由 `store/chat.js` 的 `mode` 驱动
- 事件 handler 注册在 `coord.setHandlers()`，store 内实现（onMessageCreate 等）
- 协议见 [docs/architecture/realtime.md](../../docs/architecture/realtime.md)

## 测试

- 单元测试与源码同目录：`<file>.test.js` / `schemas.test.ts`（vitest）
- 浏览器 E2E 在 `tests/`，规则见 [tests/README.md](../tests/README.md)
