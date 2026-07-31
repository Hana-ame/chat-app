# 前端架构规范 (Frontend Architecture Spec)

---

## 目录

- [技术栈](#技术栈)
- [项目结构](#项目结构)
- [API 通信层](#api-通信层)
  - [`request(method, path, token, body)`](#requestmethod-path-token-body)
  - [`api` 对象](#api-对象)
  - [`MOCKABLE` 系统](#mockable-系统)
  - [`enableMock()` / `disableMock()`](#enablemock--disablemock)
- [状态管理层](#状态管理层)
  - [`useAuthStore`](#useauthstore)
  - [`useChatStore`](#usechatstore)
- [路由层](#路由层)
  - [`App.jsx` 路由](#appjsx-路由)
  - [`ChatPage.jsx` 页面](#chatpagejsx-页面)
- [UI 组件层](#ui-组件层)
  - [`ChatView`](#chatview)
  - [`ChatListItem`](#chatlistitem)
  - [`MessageItem`](#messageitem)
  - [`Composer`](#composer)
- [Mock 模拟层](#mock-模拟层)
- [测试](#测试)
- [CI/CD](#cicd)

---

## 技术栈

```json
{
  "framework": "React 19",
  "bundler": "Vite 6",
  "state": "Zustand 5",
  "router": "React Router 7",
  "test": "Playwright 1.50",
  "real-time": "WebSocket (primary) / SSE (secondary) / Polling (fallback)"
}
```

---

## 项目结构

```
client/
├── src/
│   ├── api/
│   │   ├── client.js      # HTTP 请求封装 + API 方法定义 + Mock 开关
│   │   └── mock.js         # Mock 函数实现（与真实 API 方法一一对应）
│   ├── dev/
│   │   ├── dummy.js        # Mock 数据生成器（10 个聊天 × 150 条消息）
│   │   └── stream-source.js # 流式输出模拟（AI 打字效果）
│   ├── store/
│   │   ├── auth.js         # 认证状态：login/register/logout/mockLogin
│   │   └── chat.js         # 聊天状态：chats/messages/pinnedMessage/实时同步
│   ├── routes/
│   │   ├── LoginPage.jsx   # 登录页
│   │   ├── RegisterPage.jsx # 注册页
│   │   └── ChatPage.jsx    # 聊天主页面（列表 + 对话 + 成员面板）
│   ├── components/
│   │   ├── ChatView.jsx    # 消息流 + 公告栏
│   │   ├── ChatListItem.jsx # 聊天列表项
│   │   ├── ChatList.jsx    # 聊天列表容器
│   │   ├── Composer.jsx    # 消息输入框 + 附件上传
│   │   ├── MessageItem.jsx # 单条消息渲染 + 操作
│   │   ├── MemberPanel.jsx # 成员面板
│   │   ├── WelcomeView.jsx # 空状态欢迎页
│   │   └── renderContent.js # Markdown / @提及 渲染
│   └── App.jsx             # 根组件 + 路由配置
└── tests/
    └── e2e.spec.js         # Playwright E2E 测试
```

---

## API 通信层

### `request(method, path, token, body)`

**目的:** 统一的 HTTP 请求封装。处理鉴权、Cookie、429/401 错误。

**基本方法:** 接收 method、path、可选的 token 和 body，返回解析后的 JSON 数据。

```js
async function request(method, path, token, body) {
  const opts = { method, headers: {}, credentials: 'include' };
  if (body) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(API_BASE + path, opts);
  const data = await res.json().catch(() => ({}));
  // ... 401 auto-refresh, 429 handling
  if (!res.ok) throw { status: res.status, ...data };
  return data;
}
```

**依赖链:** `fetch → res.json → (optional refresh) → return`

**条件分支:**
- `res.status === 401 && path !== '/api/auth/refresh'`：触发自动 refresh → 重试 → 失败则 dispatch `auth:unauthorized`
- `res.status === 429`：抛出 `{ status: 429, error: 'too_many_requests', message: 'Too many requests, please try again later' }`
- `!res.ok`：抛出 `{ status: res.status, ...data }`

### `api` 对象

**目的:** 集中定义所有后端 API 方法。每个方法对应一个后端端点。

**基本方法:**

```js
export const api = {
  // ── Auth ──
  register: (email, username, password) =>
    request('POST', '/api/auth/register', null, { email, username, password }),
  login: (email, password) =>
    request('POST', '/api/auth/login', null, { email, password }),
  refresh: () =>
    fetch(API_BASE + '/api/auth/refresh', { method: 'POST', credentials: 'include' }).then(r => { if (!r.ok) throw r; return r.json(); }),
  logout: (token) =>
    request('POST', '/api/auth/logout', token),
  me: (token) => request('GET', '/api/users/me', token),
  updateProfile: (token, data) => request('PATCH', '/api/users/me', token, data),
  searchUsers: (token, q) => request('GET', '/api/users?q=' + encodeURIComponent(q), token),

  // ── Chats ──
  listChats: (token) => request('GET', '/api/chats', token),
  listPublicChats: (token) => request('GET', '/api/chats/public', token),
  createChat: (token, name, memberIds, visibility) =>
    request('POST', '/api/chats', token, { type: 'group', name, member_ids: memberIds, visibility: visibility || 'private' }),
  getChat: (token, id) => request('GET', '/api/chats/' + id, token),
  deleteChat: (token, id) => request('DELETE', '/api/chats/' + id, token),
  renameChat: (token, id, name) =>
    request('PATCH', '/api/chats/' + id, token, { name }),
  createDM: (token, userId) =>
    request('POST', '/api/dms', token, { user_id: userId }),
  joinChat: (token, chatId) => request('POST', '/api/chats/' + chatId + '/join', token),
  setPinnedMessage: (token, chatId, content) =>
    request('POST', '/api/chats/' + chatId + '/pin', token, { content }),
  clearPinnedMessage: (token, chatId) =>
    request('DELETE', '/api/chats/' + chatId + '/pin', token),

  // ── Members ──
  addMember: (token, chatId, userId) =>
    request('POST', '/api/chats/' + chatId + '/members', token, { user_id: userId }),
  removeMember: (token, chatId, userId) =>
    request('DELETE', '/api/chats/' + chatId + '/members/' + userId, token),

  // ── Messages ──
  listMessages: (token, chatId, before, limit) => {
    let url = '/api/chats/' + chatId + '/messages?limit=' + (limit || 50);
    if (before) url += '&before=' + before;
    return request('GET', url, token);
  },
  sendMessage: (token, chatId, content, attachments) =>
    request('POST', '/api/chats/' + chatId + '/messages', token, { content, attachments: attachments || [] }),
  editMessage: (token, chatId, msgId, content) =>
    request('PATCH', '/api/chats/' + chatId + '/messages/' + msgId, token, { content }),
  deleteMessage: (token, chatId, msgId) =>
    request('DELETE', '/api/chats/' + chatId + '/messages/' + msgId, token),
  markRead: (token, chatId, messageId) =>
    request('POST', '/api/chats/' + chatId + '/read', token, { message_id: messageId }),

  // ── Reactions ──
  addReaction: (token, chatId, msgId, emoji) =>
    request('PUT', '/api/chats/' + chatId + '/messages/' + msgId + '/reactions/' + encodeURIComponent(emoji), token),
  removeReaction: (token, chatId, msgId, emoji) =>
    request('DELETE', '/api/chats/' + chatId + '/messages/' + msgId + '/reactions/' + encodeURIComponent(emoji), token),

  // ── Uploads ──
  upload: async (file) => { /* ... */ },
  uploadAvatar: async (_token, file) => { /* ... */ },

  // ── Misc ──
  sseUrl: (token) => API_BASE + '/api/events?access_token=' + encodeURIComponent(token),
};
```

**依赖链:** 每个方法 `→ request` / `fetch` → 后端

### `MOCKABLE` 系统

**目的:** 维护一个数组，枚举 `api` 对象上所有可被 Mock 替换的方法，确保 Mock API 与真实 API 一一对应。

```js
const MOCKABLE = [
  ['register', mockRegister],
  ['login', mockLogin],
  ['refresh', mockRefresh],
  ['logout', mockLogout],
  ['me', mockMe],
  // ... 共 28 个方法
];
```

**条件分支:**
- 切换 Mock 时，遍历 `MOCKABLE`，将 `api[key]` 替换为 `mock` 或恢复 `_originals[key]`

### `enableMock()` / `disableMock()`

**目的:** 开启/关闭 Mock 模式。将 `api` 上的所有方法替换为 Mock 实现。

**基本方法:**
```js
api.enableMock = () => {
  if (_mockEnabled) return;
  _mockEnabled = true;
  resetMockData();
  for (const [key, mock] of MOCKABLE) {
    save(key, api[key]);   // 保存原函数
    swap(key, mock);       // 替换为 mock
  }
};

api.disableMock = () => {
  if (!_mockEnabled) return;
  _mockEnabled = false;
  for (const [key] of MOCKABLE) {
    api[key] = _originals[key]; // 恢复原函数
  }
};
```

**条件分支:**
- 已启用 → 直接 return
- 启用时：保存所有原函数，替换为 mock，调用 `resetMockData()`
- 禁用时：从 `_originals` 恢复所有原函数

---

## 状态管理层

### `useAuthStore`

**目的:** 管理用户认证状态（user、accessToken）、登录/注册/登出/刷新操作、Debug Mode 和 Mock Login。

**依赖链:** `api/client.js → localStorage`

**状态定义:**
| 字段 | 类型 | 默认值 | 说明 |
| :--- | :--- | :--- | :--- |
| `user` | `object|null` | `saved.user` | 当前登录用户 |
| `accessToken` | `string\|undefined` | `saved.accessToken` | JWT Token，用于路由判断 |
| `loading` | `boolean` | `false` | 请求中 |
| `error` | `string\|null` | `null` | 错误信息 |
| `debugMode` | `boolean` | `saved.debugMode` | Debug 模式开关，持久化到 localStorage |

**条件分支:**
- 初始化时 `saved.accessToken === 'mock-token'` → 自动调用 `api.enableMock()`
- `register` / `login` 成功 → 保存 `{ user, accessToken }` 到 store + localStorage
- `logout` → 调用 `api.disableMock()`，清除 localStorage，重置 store
- `mockLogin` → 调用 `api.enableMock()`，`setMode('poll')`，写入 mock user + `accessToken: 'mock-token'`

### `useChatStore`

**目的:** 管理聊天列表、消息缓存、公告栏、WebSocket/SSE/Polling 三种实时同步模式。

**依赖链:** `api/client.js → api/mock.js`

**状态定义:**
| 字段 | 类型 | 默认值 | 说明 |
| :--- | :--- | :--- | :--- |
| `chats` | `array` | `[]` | 聊天列表，支持 `pinned` 排序 |
| `activeChatId` | `string\|null` | `null` | 当前激活聊天 ID |
| `messages` | `array` | `[]` | 当前聊天消息缓存 |
| `pinnedMessage` | `object` | `{}` | `{ chatId: content }` 公告栏 |
| `onlineUserIds` | `array` | `[]` | 在线用户 ID 列表 |
| `mode` | `'ws'\|'sse'\|'poll'` | `'ws'` | 实时同步模式 |

**三种同步模式:**
| 模式 | 连接方式 | 适用场景 |
| :--- | :--- | :--- |
| `ws` | WebSocket | 生产环境（默认），双向实时 |
| `sse` | EventSource | 生产环境备选，单向实时 |
| `poll` | `setInterval` 2s 轮询 | Mock 模式 / 降级方案 |

**关键动作:**
- `connectWS(token)` → 建立 WebSocket → 接收 `ready`, `message_create`, `chat_update`, `presence_update` 等事件
- `connectPolling(token)` → 每 2 秒调用 `api.listChats` 和 `api.listMessages`
- `onChatUpdate(chat)` → 更新聊天列表 + 同步 `pinnedMessage[chat.id]`
- `setPinnedMessage(token, chatId, content)` → 调用 API + 更新 store
- `clearPinnedMessage(token, chatId)` → 调用 API + 删除 store 中的公告

---

## 路由层

### `App.jsx` 路由

**目的:** 根组件。根据 `accessToken` 判断登录状态，控制页面路由。

```jsx
<Routes>
  <Route path="/login" element={token ? <Navigate to="/" /> : <LoginPage />} />
  <Route path="/register" element={token ? <Navigate to="/" /> : <RegisterPage />} />
  <Route path="/*" element={token ? <ChatPage /> : <Navigate to="/login" />} />
  <Route path="/g/:chatId" element={token ? <ChatPage /> : <Navigate to="/login" />} />
</Routes>
```

**条件分支:**
- `token` 真值 → 渲染 `ChatPage`（已登录）
- `token` 假值 → 重定向到 `/login`
- 监听 `auth:unauthorized` 事件 → 自动 logout + 跳转 `/login`

### `ChatPage.jsx` 页面

**目的:** 聊天主页面。组合侧边栏聊天列表、中间聊天视图、右侧成员面板。

**加载流程:**
```
mount
  ├── accessToken 存在
  │   ├── mode === 'ws'   → connectWS(accessToken)
  │   ├── mode === 'sse'  → connectSSE(accessToken)
  │   └── mode === 'poll' → connectPolling(accessToken)
  │
  ├── loadChats(accessToken)
  │
  ├── urlChatId 存在 → setActiveChat(urlChatId)
  │   └── messages 为空 → loadMessages(accessToken, chatId)
  │       └── 有消息 → api.markRead(accessToken, chatId, lastMsgId)
  │
  └── unmount → disconnect()
```

---

## UI 组件层

### `ChatView`

**目的:** 核心聊天视图。包含消息流、公告栏、加载更多。

**关键特性:**
| 特性 | 实现 |
| :--- | :--- |
| 消息列表 | 游标分页，`loadMore` 加载旧消息 |
| 自动滚动 | 新消息自动滚到底部，加载旧消息保持位置 |
| 公告栏 | 显示 `pinnedMessage[chatId]`，Owner 可管理 |
| 429 错误 | 加载更多时捕获 429 并 alert |

**条件分支:**
- `pinnedMessage[chatId] || isEditingNotice` → 渲染公告栏
- `chat.owner_id === user.id` → 显示 Edit / Clear / + Set Notice 按钮
- `filtered.length === 0` → 显示空状态
- `loading` → 显示 Loading...

### `ChatListItem`

**目的:** 聊天列表中的单个条目。显示头像、名称、最后消息、未读数。

**条件分支:**
- `chat.type === 'dm'` → 显示对方用户名
- `unread > 0` → 显示未读徽标
- `chat.owner_id === user.id` → 右键菜单显示 Delete 按钮

### `MessageItem`

**目的:** 单条消息的完整渲染与操作。

**操作清单:**
| 操作 | 条件 | 说明 |
| :--- | :--- | :--- |
| 😀 Reactions | 任何人 | 显示/选择常用 emoji |
| Edit | 仅作者 | 内联编辑消息内容 |
| Delete | 仅作者 | 确认后删除 |
| Deleted 状态 | 任何 | 显示 `(message deleted)` |

### `Composer`

**目的:** 消息输入框 + 附件上传。

**条件分支:**
- 附件 URL 必须指向 `https://upload.moonchan.xyz/`

---

## Mock 模拟层

### 文件: `api/mock.js`

**目的:** 提供与真实 API 方法签名完全一致的 Mock 实现。28 个方法一一对应。

**Mock 数据:**
```
generateDummyData({ chatCount: 10, msgPerChat: 150 })
  ├── chats: 10 个聊天（群组 + DM + 公开）
  └── messages: 1500 条消息（带 @提及、Markdown、code block）
```

**Mock 状态同步:**
- `mockSendMessage` → 调用 `store.onMessageCreate` 触发实时更新 + AI 自动回复
- `mockAddReaction` → 调用 `store.onReaction`
- `mockSetPinnedMessage` → 调用 `store.onChatUpdate`
- `mockEditMessage` → 调用 `store.onMessageUpdate`
- `mockDeleteMessage` → 调用 `store.onMessageDelete`

### 切换机制

```
mockLogin()
  ├── api.enableMock()
  │   ├── 保存 28 个原函数到 _originals
  │   └── 替换为 mock 函数
  ├── setMode('poll')
  └── 写入 localStorage + store

logout()
  ├── api.disableMock()
  │   └── 从 _originals 恢复 28 个原函数
  └── 清除 localStorage + store
```

---

## 测试

### 文件: `tests/e2e.spec.js`

**目的:** Playwright E2E 测试，覆盖核心用户流程。

**测试用例:**
| 测试 | 覆盖内容 |
| :--- | :--- |
| `home redirects to login` | 未登录重定向 |
| `login form renders correctly` | 登录页 UI |
| `register form renders correctly` | 注册页 UI |
| `full auth flow` | 注册 → 自动登录 → 进入主界面 |
| `create group chat` | 创建群组 → 验证名称 |
| `send and receive message` | 发送消息 → 验证消息出现 |
| `responsive layout on mobile` | 375px 视口 → 验证表单可见 |
| `notice board functionality as owner` | 设置/编辑/清除公告 |

---

## CI/CD

### 文件: `.github/workflows/frontend-ci.yml`

**目的:** 自动化回归测试流水线。

**步骤:**
```
Trigger: push / pull_request → main
  ├── Setup Node.js 20
  ├── npm install (client/)
  ├── npx playwright install --with-deps
  ├── npm run build
  ├── go build + start server (后台)
  └── npm test (Playwright)
```

**条件分支:**
- `push` 到 `main` → 执行全量测试
- `pull_request` 到 `main` → 执行全量测试