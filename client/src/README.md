# client/src — 前端源码

## 目录结构

```
src/
├── api/client.js          # HTTP 请求封装（register, login, upload, messages 等）
├── components/            # UI 组件
│   ├── ChatList.jsx       # 左侧栏：聊天列表、创建群、搜索用户、设置
│   ├── ChatView.jsx       # 中间：消息列表 + Composer
│   ├── Composer.jsx       # 输入区：文本输入 + 文件上传
│   ├── MemberPanel.jsx    # 右侧栏：群组成员列表
│   └── MessageItem.jsx    # 单条消息：渲染、编辑、删除、Emoji 反应
├── routes/
│   ├── LoginPage.jsx      # /login
│   ├── RegisterPage.jsx   # /register
│   └── ChatPage.jsx       # / 和 /g/:chatId — 主聊天页
├── store/
│   ├── auth.js            # Zustand — 认证状态（token、user、login/logout）
│   └── chat.js            # Zustand — 聊天状态（chats、messages、webSocket/SSE/polling）
├── dev/
│   └── dummy.js           # 生成测试数据（chats/messages），用于 UI 调试
├── hooks/                 # 自定义 hooks（当前为空）
├── styles/
│   └── global.css         # 全局样式变量和布局
├── App.jsx                # 路由分发、401 自动跳转登录
└── main.jsx               # 入口：ReactDOM + BrowserRouter
```

---

## 组件组合

```
App → ChatPage
        ├─ ChatList     (sidebar — 聊天列表, 创建, DM, 设置, ContextMenu)
        ├─ ChatView     (中间面板)
        │   ├─ MessageItem  (单条消息: 渲染/编辑/反应)
        │   └─ Composer     (文本输入 + 文件上传)
        └─ MemberPanel  (右侧成员列表/添加/踢出)
```

详细 review 见 [`COMPOSITION_REVIEW.md`](./COMPOSITION_REVIEW.md)。

## 架构约定

| 层 | 说明 |
|----|------|
| `store/` | Zustand store，所有共享状态放这里。禁止组件内 useState 传递跨组件数据。 |
| `api/client.js` | 唯一网络层。所有 HTTP 请求走这里，统一处理 auth header、JSON parse、401 事件。 |
| `components/` | 纯 UI 组件，可从 store/ 读取状态，但不直接发 API（调用 api.* 可以）。 |
| `routes/` | 页面级组件，组合 components/ 完成布局。 |
| `styles/global.css` | 使用 CSS 变量，主题色统一，组件内可 inline style 覆盖。 |

---

## 状态管理 (Zustand)

### `useAuthStore`

```js
user, accessToken, loading, error
register(email, username, password)
login(email, password)
logout()
refreshAuth()
setUser(user)
```

Token 持久化到 `localStorage['auth']`。

### `useChatStore`

```js
chats, activeChatId, messages, onlineUserIds
mode: 'ws' | 'sse' | 'poll'
connectWS(token) / connectSSE(token) / connectPolling(token)
disconnect()
sendMessage(token, chatId, content, attachments)
loadChats(token)
loadMessages(token, chatId, before?)
setActiveChat(chatId)
pinChat / unpinChat
```

---

## 实时连接

支持三种模式，在 ChatList 顶部的 WS/SSE/Poll 按钮切换：

| 模式 | 原理 | 重连 |
|------|------|------|
| **WS** | WebSocket | 断线 3s 后自动重连 |
| **SSE** | EventSource (Server-Sent Events) | 断线 3s 后自动重连 |
| **Poll** | `setInterval(2000)` 轮询 | 无断线检测，始终轮询 |

---

## 上传

| 用途 | 端点 | 方法 | Auth |
|------|------|------|------|
| 附件（消息内） | `upload.moonchan.xyz/api/upload` | PUT multipart | 无 |
| 头像 | `upload.moonchan.xyz/api/upload` | PUT multipart | 无 |

上传后前端拼接 URL 为 `https://upload.moonchan.xyz/api/{id}/{filename}`。

---

## 测试数据（dev）

侧栏底部有 `🧪 Generate test data` 按钮，调用 `dev/dummy.js` 生成模拟聊天和消息。

```js
generateDummyData({ chatCount: 5, msgPerChat: 40 })
// → { chats, messages, activeChatId }
```

生成的数据直接注入 Zustand store，不经过后端 API。用于验证：
- 聊天列表为空 / 少量 / 大量时的 UI 表现
- 消息滚动（`flex-direction: column-reverse`）
- 各种消息内容（Markdown、附件、删除、编辑、Reaction）

---

## 滚动行为

`.chat-body` 使用 `display: flex; flex-direction: column-reverse`：
- 最新消息始终在可视区域底部
- 消息少时，空白区域出现在顶部（视觉合理）
- 消息多时可滚动查看历史

已知问题：
- Tab 顺序被反转（第一个消息在 DOM 末尾），键盘导航会异常 —— 见 `COMPOSITION_REVIEW.md`
- `ChatView.loadMore` 目前直接调用 `api.listMessages` 但未将结果合并到 store，`load older messages` 功能不可用（需改用 `useChatStore.loadMessages(token, chatId, before)`）

---

## 路由

| Path | 组件 | 说明 |
|------|------|------|
| `/login` | `LoginPage` | 未认证时显示 |
| `/register` | `RegisterPage` | 未认证时显示 |
| `/` | `ChatPage` | 已认证，显示左侧列表 + 空中间区域 |
| `/g/:chatId` | `ChatPage` | 已认证，自动打开指定聊天 |

---

## UI 风格

- 暗色主题（Discord-like），CSS 变量定义在 `:root`
- 桌面三栏（sidebar / main / members），移动端两栏全屏切换
- 响应式断点 `768px`
- 组件内避免重复 CSS class，多用 `style={}` 快速调整

---

## 修改记录

| 日期 | 变更 |
|------|------|
| 2026-07-06 | 创建 `README.md` |
| 2026-07-06 | 创建 `COMPOSITION_REVIEW.md`，组件组合分析及重构建议 |
| 2026-07-06 | 添加 `dev/dummy.js`，侧栏「Generate test data」按钮，更新滚动行为说明 |
