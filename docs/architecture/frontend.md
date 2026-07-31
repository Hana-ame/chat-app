# 前端架构

## 目录

```
client/src/
  api/          client.ts（API 方法定义 + Mock Proxy）· mock.js（mock 数据）· schemas.ts
  store/        auth.js（用户/登录态）· chat.js（聊天/消息/实时状态）· notification.js
  realtime/     coordinator.js（传输调度）· fetchStream.js（流式读取）· transports/{ws,sse,poll,mock}.js
  components/   ChatList · ChatView · MessageList · MessageItem · Composer · MemberList ·
                MemberPanel · ChatInfoModal · UserProfileModal · ImagePreviewModal ·
                CreateGroupForm · PublicChannelList · SidebarFooter · UserAvatar · Toast · ScrollArea · EmptyState · WelcomeView
  routes/       ChatPage（布局+路由）· LoginPage · RegisterPage
  hooks/        useEscapeKey · useMembers
  utils/        ai.js（AI 流式组装）· notifyMessage.js · browserNotify.js
  dev/          dummy.js（dev 模式假数据）· stream-source.js
```

技术栈：React 19 + Vite 6 + Zustand 5 + React Router 7（全部 ESM）。

## 状态管理（Zustand）

| store | 状态 | 关键动作 |
|---|---|---|
| `auth` | `user`、`accessToken`、登录态 | `login`、`register`、`refresh`、`logout`、`mockLogin` |
| `chat` | `chats`、`messages`、`activeChatId`、`mode`（ws/sse/poll/mock）、`wsReady` 等 | `setMode`、`connect`、`loadChats`、`loadMessages`、`sendMessage`、`onMessageCreate`…（事件 handler 族） |
| `notification` | 通知消息列表、未读 | `loadNotifications`、`markRead` |

组件通过 hook 订阅 store 切片；**事件更新统一走 `coord.setHandlers(...)` 注册的 handler**（onMessageCreate / onMessageUpdate / onMessageDelete / onChatUpdate / onChatRemove / onUserUpdate），避免组件各自拼事件逻辑。

## 实时协调器（realtime/coordinator.js）

- `coordinator` 持有当前传输实例，提供 `connect(mode, token)` / `disconnect()` / `setHandlers()`
- `connect` 总是先 teardown 旧传输再建立新传输（支持运行中切换模式）；旧传输回调由 `_gen` 计数器门控丢弃
- transports 统一接口：`start(coord, token)` + `stop()` + 回调 `onReady / onMessage(s) / onEvent / onChatUpdate / onPresence`
- **模式切换竞态**：`poll`/`sse` 传输在回调中带 `chatId`，`coordinator` 校验 `getActiveChatId()` 后丢弃过期聊天的事件（防止切聊天时串台）
- 主动拉取类动作（`loadChats`、`loadMessages`、`markRead`）由 store 直接调 API，不依赖传输

## 三传输 + Mock

| 传输 | 实现 | 细节 |
|---|---|---|
| `ws` | `transports/ws.js` | 原生 WebSocket → `/ws?token=`，收到信封分发给 `onEvent` |
| `sse` | `transports/sse.js` | `EventSource('/api/events')`（带 token），按 `data:` 解析 |
| `poll` | `transports/poll.js` | 定时轮询 `GET /api/chats/my` + `GET /api/events/...`，增量合并 |
| `mock` | `transports/mock.js` | 定时从 mock store 拉取并派发事件（CI 测试用） |

## API 层与 Mock 机制

`api/client.ts` 导出 `api` 对象（Proxy 包装）：

1. 正常模式：方法直通 HTTP（`request("GET", path, token, body)`），上传带 `Authorization` + `credentials: 'include'`
2. `api.enableMock()`（由 `window.__mockLogin()` 调用）后：Proxy `get` 拦截 ——
   - `notifications` 等**特殊分支**返回 Promise 包装的 mock handlers（**必须 `Promise.resolve(...)`，同步返回会崩调用方**）
   - 其余属性命中 `mockHandlers` 表 → 调用 `mock.js` 对应函数
   - 未命中 → 回退真实实现

Mock 数据（`mock.js`）：`ensureData()` 惰性生成用户/群组/消息；`resetMockData()` 在每次 `enableMock` 时重建，保证测试间隔离。测试通过 `window.__mockLogin()`（挂在 login 页）进入 mock 模式，无需后端。

## 关键 UI 细节

- **Notifications 聊天固定置顶**：`ChatList` 在搜索框为空时把 `type=notify` 聊天渲染在列表首位（产品行为，非排序结果）
- 排序：置顶聊天 → `last_message_at`（无消息则 `created_at`）降序
- 消息渲染：`renderContent.jsx` 支持 markdown 子集 + 图片预览 + 文件附件
- 移动端：ChatPage 自动跳转 `/g/notifications` 等路由，无侧栏
