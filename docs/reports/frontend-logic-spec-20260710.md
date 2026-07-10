# 前端逻辑层规范

> 审计日期：2026-07-10
> 源码：`client/src/api/`、`client/src/store/`、`client/src/dev/`、`client/src/routes/`
> 依赖：React 18、Zustand、React Router v6

---

## 目录

- [架构概览](#架构概览)
- [API 层](#api-层)
  - [client.js — HTTP 请求与 Mock 切换](#clientjs--http-请求与-mock-切换)
  - [mock.js — 内存态 API 实现](#mockjs--内存态-api-实现)
- [Store 层](#store-层)
  - [auth.js — 认证状态](#authjs--认证状态)
  - [chat.js — 聊天状态与实时通信](#chatjs--聊天状态与实时通信)
- [Dev 层](#dev-层)
  - [stream-source.js — 异步流封装](#stream-sourcejs--异步流封装)
  - [mock-ws.js — 模拟 WebSocket](#mock-wsjs--模拟-websocket)
  - [dummy.js — 假数据生成器](#dummyjs--假数据生成器)
- [路由层](#路由层)
  - [App.jsx — 根路由](#appjsx--根路由)
  - [ChatPage.jsx — 主页面](#chatpagejsx--主页面)
  - [LoginPage.jsx / RegisterPage.jsx](#loginpagejsx--registerpagejsx)
- [跨模块问题汇总](#跨模块问题汇总)

---

## 架构概览

```
client/src/
├── api/
│   ├── client.js          ← HTTP 请求、401 刷新、Mock 注入
│   └── mock.js            ← 28 个 mock 函数（内存态）
├── store/
│   ├── auth.js            ← Zustand：用户身份、登录/登出
│   └── chat.js            ← Zustand：聊天列表、消息、WS/SSE/Poll
├── dev/
│   ├── stream-source.js   ← 异步流封装（onChunk + done promise）
│   ├── mock-ws.js         ← 模拟 WS 连接（polling 模式）
│   └── dummy.js           ← 假数据生成（10 chat × 65 msg）
├── routes/
│   ├── ChatPage.jsx       ← 三栏布局 + 实时连接生命周期
│   ├── LoginPage.jsx      ← 登录表单 + debug 快捷入口
│   └── RegisterPage.jsx   ← 注册表单 + debug 快捷入口
├── App.jsx                ← 路由定义 + auth:unauthorized 监听
└── main.jsx               ← React DOM 挂载
```

**数据流：** `API层 → Store层 → 组件层 → 用户交互 → Store → API`

**依赖链：**
```
main.jsx → App.jsx → routes/* → store/* → api/*
                                    ↑
                              dev/mock-ws.js (mock 模式)
```

---

## API 层

> **功能概述：** API 层封装前端与后端之间的所有 HTTP 通信。它负责请求构造、响应解析、token 自动刷新、错误统一处理，以及 Mock 模式的无缝切换。整个前端（Store 层、组件层）只通过此层访问后端，不允许直接 `fetch`。
>
> **使用场景：** 用户登录、发送消息、上传文件、搜索用户、切换聊天——所有需要与服务器交互的操作都经过 API 层。Mock 模式下，API 层将所有调用重定向到内存态实现，无需后端即可完整运行前端。

---

### `client.js` — HTTP 请求与 Mock 切换

**位置：** `client/src/api/client.js`（240 行）

**功能概述：** `client.js` 是整个前端的 HTTP 通信中枢。它对外暴露一个 `api` 对象，包含所有 API 方法（login、listChats、sendMessage 等）。每个方法内部统一调用 `request()`，由 `request()` 处理 fetch 构造、JSON 解析、401 自动刷新、429 限流提示、错误抛出。此外，它提供 `enableMock()`/`disableMock()` 机制，在运行时将所有 API 方法替换为 `mock.js` 中的内存态实现，使前端无需后端即可运行。

**使用场景：**
- 开发环境：Vite proxy 将 `/api/*` 转发到 `localhost:8080`，`client.js` 直接请求相对路径
- 生产环境（Cloudflare Pages）：检测 `*.pages.dev` 域名，请求走 `wsl-8080.moonchan.xyz`
- Mock 模式：登录页点击「Quick Enter」或勾选「Mock API」，`client.js` 将 28 个方法替换为 mock 函数，所有请求走内存态

**核心方法：** `request(method, path, token, body)` — 所有 API 调用的统一入口。

#### 环境检测

```javascript
const IS_PAGES = typeof window !== 'undefined' && window.location.hostname.endsWith('pages.dev');
const API_BASE = IS_PAGES ? 'https://wsl-8080.moonchan.xyz' : '';
const UPLOAD_BASE = 'https://upload.moonchan.xyz';
```

**条件分支：**
- `IS_PAGES = true` → 生产环境，请求走 `wsl-8080.moonchan.xyz`
- `IS_PAGES = false` → 本地开发，走 Vite proxy → `localhost:8080`

#### `request()` — 自动刷新机制

```javascript
async function request(method, path, token, body) {
  const opts = { method, headers: {}, credentials: 'include' };
  if (body) { opts.headers['Content-Type'] = 'application/json'; opts.body = JSON.stringify(body); }
  const res = await fetch(API_BASE + path, opts);
  const data = await res.json().catch(() => ({}));

  if (res.status === 401 && path !== '/api/auth/refresh') {
    if (!_refreshing) {
      _refreshing = true;
      // 尝试 refresh → 成功则 retry 原请求 → 失败则 auth:unauthorized
    }
  }
  if (res.status === 429) throw { status: 429, error: 'too_many_requests', ... };
  if (!res.ok) throw { status: res.status, ...data };
  return data;
}
```

**依赖链：** `fetch → res.json() → 401? → refresh → retry → throw`

**条件分支：**
- `res.status === 401 && path !== '/api/auth/refresh'` → 尝试自动刷新
  - `_refreshing === true` → 跳过（防止并发刷新风暴）
  - refresh 成功 → 更新 `localStorage.auth` + `useAuthStore` → retry 原请求
  - refresh 失败 → `window.dispatchEvent('auth:unauthorized')` → App.jsx 处理登出
- `res.status === 429` → 抛出 `too_many_requests`
- 其他非 200 → 抛出错误体

**关键状态：** `_refreshing`（模块级 bool）— 防止并发刷新。

#### API 方法列表

| 方法 | HTTP | 路径 | Token | Body | 职责 |
|------|------|------|-------|------|------|
| `register` | POST | `/api/auth/register` | — | `{email,username,password}` | 注册新用户，返回 user + token |
| `login` | POST | `/api/auth/login` | — | `{email,password}` | 登录，返回 user + token |
| `refresh` | POST | `/api/auth/refresh` | cookie | — | 刷新 access token |
| `logout` | POST | `/api/auth/logout` | token | — | 登出，服务端清除 refresh token |
| `me` | GET | `/api/users/me` | token | — | 获取当前用户信息 |
| `updateProfile` | PATCH | `/api/users/me` | token | `{username,avatar_color,avatar_url}` | 修改用户名/头像/颜色 |
| `searchUsers` | GET | `/api/users?q=` | token | — | 按用户名搜索用户 |
| `listChats` | GET | `/api/chats/my` | token | — | 获取当前用户加入的所有 chat |
| `listPublicChats` | GET | `/api/chats/public` | token | — | 获取所有公开 chat |
| `createChat` | POST | `/api/chats` | token | `{type,name,member_ids,visibility}` | 创建群组聊天 |
| `getChat` | GET | `/api/chats/{id}` | token | — | 获取单个 chat 详情（含 members） |
| `deleteChat` | DELETE | `/api/chats/{id}` | token | — | 删除聊天（仅 owner） |
| `renameChat` | PATCH | `/api/chats/{id}` | token | `{name}` | 重命名聊天 |
| `createDM` | POST | `/api/dms` | token | `{user_id}` | 创建或获取与指定用户的 DM |
| `joinChat` | POST | `/api/chats/{id}/join` | token | — | 加入公开/unlisted chat |
| `setPinnedMessage` | POST | `/api/chats/{id}/pin` | token | `{content}` | 设置 Notice Board 固定消息 |
| `clearPinnedMessage` | DELETE | `/api/chats/{id}/pin` | token | — | 清除 Notice Board |
| `addMember` | POST | `/api/chats/{id}/members` | token | `{user_id}` | 添加成员到群组 |
| `removeMember` | DELETE | `/api/chats/{id}/members/{userId}` | token | — | 从群组移除成员 |
| `listMessages` | GET | `/api/chats/{id}/messages?limit=&before=` | token | — | 分页加载消息（before 为游标） |
| `sendMessage` | POST | `/api/chats/{id}/messages` | token | `{content,attachments}` | 发送消息 |
| `editMessage` | PATCH | `/api/chats/{id}/messages/{msgId}` | token | `{content}` | 编辑消息内容 |
| `deleteMessage` | DELETE | `/api/chats/{id}/messages/{msgId}` | token | — | 删除消息 |
| `markRead` | POST | `/api/chats/{id}/read` | token | `{message_id}` | 标记消息已读 |
| `addReaction` | PUT | `/api/chats/{id}/messages/{msgId}/reactions/{emoji}` | token | — | 添加 reaction |
| `removeReaction` | DELETE | `/api/chats/{id}/messages/{msgId}/reactions/{emoji}` | token | — | 移除 reaction |
| `upload` | PUT | `https://upload.moonchan.xyz/api/upload` | — | Raw binary | 上传文件，返回 URL |
| `uploadAvatar` | — | 委托 `api.upload()` | — | — | 上传头像（特殊 URL） |

#### Mock 注入机制

```javascript
let _mockEnabled = false;
const _originals = {};

api.enableMock = () => {
  _mockEnabled = true;
  resetMockData();
  for (const [key, mock] of MOCKABLE) { save(key, api[key]); swap(key, mock); }
};

api.disableMock = () => {
  _mockEnabled = false;
  for (const [key] of MOCKABLE) { api[key] = _originals[key]; }
};
```

**机制：** `save()` 备份原始方法 → `swap()` 替换为 mock 函数（包装了 console.log + Promise.resolve）→ `disableMock()` 恢复。

**⚠️ 已知问题：** `swap()` 中 `console.log` 对每个调用都输出，mock 模式下控制台噪声大。

---

### `mock.js` — 内存态 API 实现

**位置：** `client/src/api/mock.js`（447 行）

**功能概述：** `mock.js` 是 API 层的 Mock 实现，提供 28 个函数，完整模拟后端所有接口的行为。它在内存中维护一套假数据（用户、聊天、消息、reactions），所有读写操作都在这套数据上进行。关键设计是「MITM 模式」：mock 函数写入内存数据层后，同时调用 Store 的事件处理函数（`onMessageCreate`、`onChatUpdate` 等），使 UI 即时响应，无需等待轮询。此外，`mockSendMessage` 内置 AI 自动回复机制，模拟 AI Bot 的流式打字效果。

**使用场景：**
- 开发调试：无需启动 Go 后端，前端即可完整运行（登录、发消息、切换聊天、上传文件）
- CI 测试：Playwright 测试在 Mock 模式下运行，验证前端逻辑正确性
- 演示展示：前端可独立演示给利益相关者，不依赖后端服务

**核心状态：**
```javascript
let data = null;   // { users, chats, messages, reactions, chatMembers }
let _store = null; // useChatStore 引用（由 __setStoreRef 注入）
```

**初始化：** `ensureData()` → `generateDummyData({ chatCount: 10, msgPerChat: 150 })` 生成假数据。

#### 与 Go API 的关键差异（审计结论）

| 差异 | 严重性 | 说明 |
|------|--------|------|
| 不校验密码 | ❌ 重大 | 任意密码可登录 |
| 无 attachment URL 校验 | ❌ 重大 | Go 强制 `upload.moonchan.xyz` |
| AI 回复 50% 触发 | ⚠️ 预期行为 | Go 无此逻辑 |
| MarkRead 无校验 | ❌ 重大 | 缺 membership/message_id 校验 |
| user_update 逐 chat 广播 | ⚠️ 差异 | Go 全局广播 |
| 无 presence | ❌ 缺失 | Go 有 online/offline |
| SearchUsers 不排序 | ⚠️ 差异 | Go 按 username 排序 |

#### Mock 函数分类

| 类别 | 函数 | 数量 |
|------|------|------|
| Auth | `mockRegister`, `mockLogin`, `mockRefresh`, `mockLogout`, `mockMe` | 5 |
| Users | `mockUpdateProfile`, `mockSearchUsers` | 2 |
| Chats | `mockListChats`, `mockListPublicChats`, `mockCreateChat`, `mockGetChat`, `mockDeleteChat`, `mockRenameChat`, `mockCreateDM`, `mockJoinChat`, `mockSetPinnedMessage`, `mockClearPinnedMessage` | 10 |
| Members | `mockAddMember`, `mockRemoveMember` | 2 |
| Messages | `mockListMessages`, `mockSendMessage`, `mockEditMessage`, `mockDeleteMessage`, `mockMarkRead` | 5 |
| Reactions | `mockAddReaction`, `mockRemoveReaction` | 2 |
| Uploads | `mockUpload`, `mockUploadAvatar` | 2 |
| Legacy | `mockTogglePin` | 1 |

#### Mock 函数详细说明

**Auth 类：**

| 函数 | 职责 | 依赖 |
|------|------|------|
| `mockRegister(_token, email, username, password)` | 校验邮箱唯一性 → 从 MOCK_USERS 分配颜色 → 返回 `{user, access_token, expires_in}` | `MOCK_USERS` |
| `mockLogin(_token, email, password)` | 按邮箱查找 MOCK_USERS → 匹配则返回用户，不校验密码 → 未找到则抛 401 | `MOCK_USERS` |
| `mockRefresh()` | 从 localStorage 读取 currentUser → 返回用户信息 + 新 token | `currentUser()` |
| `mockLogout()` | 直接返回 `{ok: true}`，无实际操作 | — |
| `mockMe()` | 从 localStorage 读取 currentUser → 返回用户对象 | `currentUser()` |

**Users 类：**

| 函数 | 职责 | Store 通知 |
|------|------|-----------|
| `mockUpdateProfile(_token, data)` | 合并用户名/头像/颜色 → 遍历所有 chat 更新 members 中的当前用户 → 通知每个 chat 的 `onChatUpdate` → 更新 Store 的 `user` | `onChatUpdate` × N + `setState({user})` |
| `mockSearchUsers(_token, q)` | 从 MOCK_USERS 按 username 模糊匹配 → 排除当前用户 → 返回前 20 条 | — |

**Chats 类：**

| 函数 | 职责 | Store 通知 |
|------|------|-----------|
| `mockListChats()` | 遍历 d.chats → 填充 last_message（跳过空内容的 AI 消息）→ 合并 members 的 MOCK_USERS 数据 → 分配 icon_color | — |
| `mockListPublicChats()` | 过滤 `visibility === 'public'` 的 chat → 返回 | — |
| `mockCreateChat(_token, name, memberIds, visibility)` | 生成 UUID → 构建 chat 对象（含 owner + members）→ unshift 到 d.chats | `onChatUpdate(newChat)` |
| `mockGetChat(_token, id)` | 按 ID 查找 chat → 合并 members 的 MOCK_USERS 数据 → 返回 | — |
| `mockDeleteChat(_token, id)` | 从 d.chats 过滤掉 → 从 d.messages 过滤掉该 chat 的消息 | `onChatDelete({chat_id})` |
| `mockRenameChat(_token, _id, name)` | 查找 chat → 修改 name | `onChatUpdate({id, name})` |
| `mockCreateDM(_token, userId)` | 查找已有 DM → 存在则返回 → 不存在则创建新 DM | `onChatUpdate(newDM)` |
| `mockJoinChat(_token, chatId)` | 查找 chat → 检查可见性 → 添加当前用户为 member | — |
| `mockSetPinnedMessage(_token, chatId, content)` | 设置固定消息内容 | `onChatUpdate({id, pinned_message})` |
| `mockClearPinnedMessage(_token, chatId)` | 清除固定消息 | `onChatUpdate({id, pinned_message: null})` |

**Members 类：**

| 函数 | 职责 |
|------|------|
| `mockAddMember(_token, chatId, userId)` | 查找 chat → 检查是否已是成员 → 添加 `{id, ...userById, role:'member'}` |
| `mockRemoveMember(_token, chatId, userId)` | 查找 chat → 从 members 过滤掉指定用户 |

**Messages 类：**

| 函数 | 职责 | Store 通知 |
|------|------|-----------|
| `mockListMessages(_token, chatId, before, limit)` | 按 chatId 过滤 → 支持 before 游标分页 → 默认返回最新 50 条 | — |
| `mockSendMessage(_token, chatId, content, attachments)` | 创建 userMsg → 写入 d.messages → 通知 `onMessageCreate` → 50% 概率触发 AI 回复（双轨：storeMsg + dataMsg） | `onMessageCreate` × 2 |
| `mockEditMessage(_token, _chatId, msgId, content)` | 查找 msg → 更新 content + edited_at | `onMessageUpdate({id, content, edited_at})` |
| `mockDeleteMessage(_token, _chatId, msgId)` | 从 d.messages 过滤掉 | `onMessageDelete({message_id, chat_id})` |
| `mockMarkRead()` | 直接返回 `{ok: true}`，无实际操作 | — |

**Reactions 类：**

| 函数 | 职责 | Store 通知 |
|------|------|-----------|
| `mockAddReaction(_token, _chatId, msgId, emoji)` | 查找 msg → 检查当前用户是否已有该 emoji → 无则新增/递增 → 有则跳过 | `onReaction({message_id, emoji, user_id}, true)` |
| `mockRemoveReaction(_token, _chatId, msgId, emoji)` | 查找 msg → 按 user_id 过滤掉当前用户的记录 → 递减 count → 移除 count=0 的条目 | `onReaction({message_id, emoji, user_id}, false)` |

**Uploads 类：**

| 函数 | 职责 |
|------|------|
| `mockUpload(file)` | 用 `URL.createObjectURL(file)` 生成本地 Blob URL → 返回 `{filename, mime_type, size, url}` |
| `mockUploadAvatar(_token, file)` | 同上，返回 `{url}` |

**Legacy 类：**

| 函数 | 职责 | Store 通知 |
|------|------|-----------|
| `mockTogglePin(_token, chatId)` | 查找 chat → 取反 pinned 字段 | `onChatUpdate({id, pinned})` |

#### `mockSendMessage` — AI 自动回复

```javascript
export function mockSendMessage(_token, chatId, content, attachments) {
  // ... 创建 userMsg ...
  d.messages.push(userMsg);
  if (_store) _store.getState().onMessageCreate(userMsg);

  if (Math.random() < 0.5) {  // ← 50% 概率触发 AI
    const text = AI_RESPONSES[Math.floor(Math.random() * AI_RESPONSES.length)];
    const aiId = randid();
    const aiStoreMsg = { id: aiId, chat_id: chatId, content: '', user_id: 'ai', author: userById('ai'),
      streaming: true, source: async (emit) => { /* 逐字符 emit */ } };
    setTimeout(() => { d.messages.push(aiDataMsg); _store.getState().onMessageCreate(aiStoreMsg); }, 500 + Math.random() * 800);
  }
  return userMsg;
}
```

**⚠️ 注意：** AI 回复的 `source` 闭包包含 `asyncFn`，通过 `createStreamSource` 流式推送到 store。

#### 双数据源问题

Mock 中 chat 成员同时存在于：
1. `c.members`（chat 对象上的数组）— `buildChatResponse` 用它算 `member_count`
2. `d.chatMembers`（扁平数组）— `mockListChats`/`mockAddMember`/`mockRemoveMember` 用它

**风险：** 两处靠手写同步，若某路径漏改会导致 member_count 不一致。Go 侧只有 `chat_members` 一张表，无此风险。

---

## Store 层

> **功能概述：** Store 层是前端的状态管理中心，使用 Zustand 实现。它分为两个独立的 store：`auth.js` 管理用户身份（登录态、token、当前用户信息），`chat.js` 管理聊天数据（聊天列表、消息、实时连接）。Store 层是 API 层和 UI 组件之间的桥梁——API 层获取数据后写入 Store，组件从 Store 读取数据渲染，用户交互后组件调用 Store 方法触发 API 调用。
>
> **使用场景：** 用户登录后，`auth.js` 持久化 token 到 localStorage；`chat.js` 建立 WebSocket/SSE/Polling 连接接收实时消息；用户发送消息时，`chat.js` 调用 API 层发送，成功后将消息追加到本地状态；其他用户发来消息时，`chat.js` 的事件处理函数即时更新 Store，组件自动重渲染。

---

### `auth.js` — 认证状态

**位置：** `client/src/store/auth.js`（92 行）

**功能概述：** `auth.js` 管理用户认证状态，包括当前用户对象、access token、登录/登出/注册逻辑。它将认证信息持久化到 `localStorage.auth`，页面刷新后自动恢复登录态。如果 localStorage 中存在 `mock-token`，初始化时自动启用 Mock 模式。它还提供 `mockLogin()` 快捷入口，一键注入假用户并切换到 polling 模式。

**使用场景：**
- 登录/注册：调用 API 层的 `login()`/`register()`，成功后写入 localStorage + 更新 store
- 页面刷新：从 localStorage 读取 token，调用 `refreshAuth()` 验证有效性
- 登出：清除 localStorage，调用 API 层 `logout()`，重置 store
- Mock 调试：点击「Quick Enter」直接调用 `mockLogin()`，跳过真实认证

**状态结构：**
```typescript
{
  user: User | null,
  loading: boolean,
  error: string | null,
  accessToken: string,         // 实际存储在 localStorage.auth.accessToken
  debugMode: boolean,
}
```

#### 函数说明

| 函数 | 职责 | 副作用 |
|------|------|--------|
| `storage.get()` | 从 localStorage 读取 auth 数据（user + accessToken） | — |
| `storage.set(v)` | 将 auth 数据写入 localStorage | — |
| `storage.clear()` | 清除 localStorage 中的 auth 数据 | — |
| `login(email, password)` | 调用 `api.login()` → 写入 localStorage → 更新 Store（user, accessToken, loading） | `storage.set()` |
| `register(email, username, password)` | 调用 `api.register()` → 写入 localStorage → 更新 Store | `storage.set()` |
| `logout()` | 调用 `api.logout()` → 清除 localStorage → 禁用 Mock → 重置 Store（user=null） | `storage.clear()`, `api.disableMock()` |
| `refreshAuth()` | 调用 `api.refresh()` → 成功则更新 localStorage + Store → 失败则清除 + 重置 | `storage.set()` 或 `storage.clear()` |
| `setUser(user)` | 外部更新 user 对象（用于 mockUpdateProfile 同步） | `set({user})` |
| `mockLogin()` | 一键 Mock 登录：启用 Mock → 切到 poll 模式 → 注入 dev-self → 写入 localStorage | `api.enableMock()`, `setMode('poll')` |

**初始化逻辑：**
```javascript
const saved = storage.get();
if (saved.accessToken === 'mock-token') api.enableMock();  // 自动恢复 Mock 模式
// 从 saved 恢复 user + accessToken 到 Store
```

**条件分支：**
- `saved.accessToken === 'mock-token'` → 初始化时自动 `api.enableMock()`
- 登录/注册成功 → `storage.set({ user, accessToken })` → `set(state)`
- 登出 → `api.disableMock()` + `storage.clear()` + `set({ user: null })`
- `refreshAuth()` 失败 → `storage.clear()` + `set({ user: null })`

#### `mockLogin()` — 快捷调试入口

```javascript
mockLogin: () => {
  api.enableMock();
  useChatStore.getState().setMode('poll');  // ← mock 模式强制 polling
  const payload = { user: { id: 'dev-self', ... }, accessToken: 'mock-token' };
  storage.set(payload);
  set({ ...payload, loading: false, error: null });
};
```

**逻辑：** 启用 mock → 切到 poll 模式 → 注入 dev-self 用户 → 写入 localStorage。

---

### `chat.js` — 聊天状态与实时通信

**位置：** `client/src/store/chat.js`（361 行）

**功能概述：** `chat.js` 是整个前端最核心的 Store，管理聊天列表、当前消息、实时连接（WS/SSE/Polling）、用户在线状态。它是实时通信的中枢——建立连接、解析事件、分发到对应的处理函数、更新状态、触发组件重渲染。所有实时事件（新消息、编辑、删除、reaction、聊天更新、成员变动、在线状态）都由它处理。此外，它负责消息的流式消费（AI 打字效果）和自动滚动触发。

**使用场景：**
- 页面加载：`connectWS()`/`connectSSE()`/`connectPolling()` 建立实时连接，接收初始数据
- 切换聊天：`setActiveChat()` 清零未读计数，`loadMessages()` 加载历史消息
- 发送消息：`sendMessage()` 调用 API 层，成功后 `onMessageCreate()` 追加到本地
- 接收消息：WebSocket 收到 `message_create` 事件，`onMessageCreate()` 即时更新
- React 操作：`onReaction()` 处理表情添加/移除，维护 `user_ids` 和 `count`
- 流式输出：AI 回复到达时，`startConsumingStream()` 逐字符消费并更新 store

**状态结构：**
```typescript
{
  chats: Chat[],
  activeChatId: string | null,
  messages: Message[],
  pinnedMessage: Record<string, string>,  // { chatId: content }
  onlineUserIds: string[],

  mode: 'ws' | 'sse' | 'poll',
  ws: WebSocket | null,
  wsReady: boolean,
  sse: EventSource | null,
  sseReady: boolean,
  pollTimer: ReturnType<typeof setTimeout> | null,
  _lastToken: string,
}
```

#### 函数说明

| 函数 | 职责 | 副作用 |
|------|------|--------|
| `setMode(mode)` | 切换实时模式：断开旧连接 → 设置 mode → 按新模式建立连接 | `disconnect()`, `connectWS/SSE/Poll()` |
| `connectWS(token)` | 建立 WebSocket 连接 → 解析事件 → 分发到处理函数 → 3 秒自动重连 | `set({ws, wsReady})` |
| `connectSSE(token)` | 建立 EventSource 连接 → 同 WS 事件处理逻辑 → 3 秒自动重连 | `set({sse, sseReady})` |
| `connectPolling(token)` | 每 2 秒轮询 `api.listChats` + `api.listMessages` → 更新 Store | `set({pollTimer})` |
| `disconnect()` | 断开所有连接：先清 ws.onclose 防重连 → close ws → close sse → clearTimeout | `set({ws:null, sse:null, pollTimer:null})` |
| `setChats(chats)` | 合并新旧 chat 列表：保留 unread_count + last_message → 按 pinned → last_message_at 排序 | `set({chats})` |
| `onChatUpdate(chat)` | 新增或更新单个 chat → 更新 pinnedMessage | `set({chats, pinnedMessage})` |
| `onChatDelete(payload)` | 移除 chat → 若是 activeChat 则清空消息 | `set({chats, messages, activeChatId})` |
| `onChatRemove(payload)` | 同 onChatDelete（被踢出群组时触发） | `set({chats, messages, activeChatId})` |
| `onMessageCreate(msg)` | 追加消息到 messages → 更新 chat.last_message + unread_count → 若 streaming 则启动流式消费 | `set({messages, chats})` |
| `onMessageUpdate(msg)` | 替换 messages 中对应消息的内容 | `set({messages})` |
| `onMessageDelete(payload)` | 软删除：设 content='' + deleted=true（不从数组移除） | `set({messages})` |
| `onReaction(payload, added)` | added=true: 已有则递增 count + 加 user_ids，无则新建；added=false: 按 user_id 过滤 + 递减 → 移除 count=0 | `set({messages})` |
| `setActiveChat(chatId)` | 设置 activeChatId → 清零该 chat 的 unread_count | `set({activeChatId, chats})` |
| `startConsumingStream(msg)` | 调用 `api.startStreaming(msg.source)` → onChunk 追加内容 → done 时 finishStreaming | 流式更新 messages |
| `finishStreaming(msgId)` | 设 streaming: false | `set({messages})` |
| `loadChats(token)` | 调用 `api.listChats()` → 用 setChats 合并 | `setChats()` |
| `loadMessages(token, chatId, before)` | 调用 `api.listMessages()` → before 则前插，否则替换 | `set({messages})` |
| `sendMessage(token, chatId, content, attachments)` | 委托 `api.sendMessage()` → mock 模式下自动触发 onMessageCreate | API 调用 |
| `sendTyping(chatId)` | 通过 WebSocket 发送 typing 事件（仅 WS 模式有效） | `ws.send()` |
| `subscribe(chatId)` | 通过 WebSocket 发送 subscribe 事件（仅 WS 模式有效） | `ws.send()` |
| `setPinnedMessage(token, chatId, content)` | 调用 `api.setPinnedMessage()` → 更新 pinnedMessage | `set({pinnedMessage})` |
| `clearPinnedMessage(token, chatId)` | 调用 `api.clearPinnedMessage()` → 删除 pinnedMessage 条目 | `set({pinnedMessage})` |

#### 实时连接 — 三模式

**`connectWS(token)`：**
```javascript
connectWS(token) {
  get().disconnect();
  const url = proto + '://' + host + '/ws?access_token=' + token;
  const ws = new WebSocket(url);
  ws.onmessage = (e) => {
    const env = JSON.parse(e.data);
    switch (env.op) {
      case 'ready': set({ onlineUserIds, wsReady: true }); setChats(p.chats); break;
      case 'message_create': onMessageCreate(env.payload); break;
      case 'message_update': onMessageUpdate(env.payload); break;
      case 'message_delete': onMessageDelete(env.payload); break;
      case 'reaction_add'/'reaction_remove': onReaction(env.payload, added); break;
      case 'chat_create'/'chat_update': onChatUpdate(env.payload); break;
      case 'chat_delete': onChatDelete(env.payload); break;
      case 'chat_remove': onChatRemove(env.payload); break;
      case 'user_update': 更新所有 chat 的 members 数组; break;
      case 'presence_update': 更新 onlineUserIds; break;
    }
  };
  ws.onclose = () => { setTimeout(() => reconnect, 3000); };  // ← 3 秒重连
}
```

**`connectSSE(token)`：** 同 WS 逻辑，走 `EventSource`，`ready` 事件为自定义事件。

**`connectPolling(token)`：** 每 2 秒轮询 `api.listChats` + `api.listMessages`。

#### `onReaction` — 完整逻辑

```javascript
onReaction(payload, added) {
  const myId = useAuthStore.getState().user?.id;
  set(s => ({ messages: s.messages.map(m => {
    if (m.id !== payload.message_id) return m;
    const rxs = m.reactions || [];
    const idx = rxs.findIndex(r => r.emoji === payload.emoji);
    if (added) {
      if (idx >= 0) { /* 增 count, 加 user_ids */ }
      else { /* 新建 reaction 条目 */ }
    } else {
      return { ...m, reactions: rxs.map(...).filter(r => r.count > 0) };
    }
  }) }));
}
```

**依赖：** `useAuthStore.getState().user?.id`（跨 store 读取）。

#### `disconnect()` — 清理

```javascript
disconnect() {
  if (s.ws) { s.ws.onclose = null; s.ws.close(); }  // ← 先清 onclose 防重连
  if (s.sse) { s.sse.close(); }
  if (s.pollTimer) { clearTimeout(s.pollTimer); }
  set({ ws: null, wsReady: false, sse: null, sseReady: false, pollTimer: null });
}
```

**⚠️ 关键：** 先设 `ws.onclose = null` 再 `close()`，防止触发 `onclose` 中的自动重连。

---

## Dev 层

> **功能概述：** Dev 层是前端的开发辅助工具集，包含三个模块：假数据生成器（`dummy.js`）为 Mock 模式提供初始数据；异步流封装（`stream-source.js`）为 AI 打字效果提供流式输出接口；模拟 WebSocket（`mock-ws.js`）在 Mock 模式下模拟实时连接的 `ready` 事件和消息轮询。这些模块仅在开发/Mock 模式下使用，生产环境不加载。
>
> **使用场景：** 开发者在登录页点击「Quick Enter」，`dummy.js` 生成 10 个聊天（1 DM + 9 群组）× 65 条消息的假数据；用户发送消息后，`mockSendMessage` 通过 `stream-source.js` 流式输出 AI 回复；`mock-ws.js` 模拟 WebSocket 的 `ready` 事件，使聊天列表即时加载。

---

### `stream-source.js` — 异步流封装

**位置：** `client/src/dev/stream-source.js`（21 行）

**功能概述：** `stream-source.js` 封装一个异步推送函数为 `{ onChunk, done }` 接口。调用方传入一个 `asyncFn(emit)` 函数，该函数在执行过程中调 `emit(chunk)` 推送数据块；外部通过 `.onChunk(callback)` 注册回调接收数据块；`.done` 是一个 Promise，在 `asyncFn` 执行完毕后 resolve。这个模式被两处使用：Mock 模式的 AI 流式回复（逐字符推送）和 API 层的 SSE 流式源。

**使用场景：**
- Mock AI 回复：`mockSendMessage` 创建 `aiStoreMsg`，其 `source` 是一个 `asyncFn`，通过 `emit` 逐字符推送 AI 文本
- 流式消费：`chat.js` 的 `startConsumingStream()` 调用 `api.startStreaming(msg.source)`，注册 `onChunk` 回调更新消息内容

```javascript
export function createStreamSource(asyncFn) {
  let onChunk = null;
  const emit = (chunk) => { if (onChunk) onChunk(chunk); };
  const promise = new Promise((resolve, reject) => {
    onDone = resolve; onError = reject;
  });
  asyncFn(emit).then(() => onDone()).catch(err => onError(err));
  return { onChunk(cb) { onChunk = cb; return this; }, done: promise };
}
```

**用途：** 封装异步推送函数为 `{ onChunk, done }` 接口。被 `mockSendMessage`（AI 流式回复）和 `api.startStreaming`（SSE 源）使用。

**依赖链：** `asyncFn(emit)` → `onChunk` 回调 → `done` promise resolve。

---

### `mock-ws.js` — 模拟 WebSocket

**位置：** `client/src/dev/mock-ws.js`（99 行）

**功能概述：** `mock-ws.js` 在 Mock 模式下模拟 WebSocket 连接的行为。由于 Mock API 是纯内存态，没有真实的 WebSocket 服务器，此模块在连接时直接 fire `ready` 事件（携带初始聊天列表和在线用户），并在后续以固定间隔轮询 `api.listChats()` 和 `api.listMessages()` 模拟实时更新。它还暴露 `simulateEvent()` 函数，允许开发者在浏览器控制台手动触发任意事件（如 `message_create`、`reaction_add`），用于调试 Store 的事件处理逻辑。

**使用场景：**
- Mock 模式连接：`chat.js` 的 `connectWS()` 检测到 mock 模式后调用 `mockWebSocketConnect()`，模拟 `ready` 事件
- 手动调试：在控制台调用 `simulateEvent('reaction_add', { message_id: '...', emoji: '👍' })` 测试 reaction 逻辑
- 事件列表：`chatEvents` 导出所有可模拟的 op 名称，供调试工具使用

**核心函数：**

| 函数 | 职责 |
|------|------|
| `mockWebSocketConnect(token, mode)` | 切到 poll 模式，50ms 后 fire `ready` 事件，每 500ms 轮询 |
| `mockWebSocketDisconnect()` | 清除 interval |
| `simulateEvent(op, payload)` | 手动触发 store 事件（调试用） |
| `chatEvents` | 导出所有可模拟的事件 op 名称 |

**`simulateEvent` 依赖链：** `useChatStore.getState()` → 根据 op 分发到对应 handler。

---

### `dummy.js` — 假数据生成器

**位置：** `client/src/dev/dummy.js`（352 行）

**功能概述：** `dummy.js` 为 Mock 模式提供完整的初始数据集。`generateDummyData()` 函数生成一套贴近真实使用场景的假数据：1 个 DM 聊天 + 9 个命名群组（General、Random、Dev Team、Gaming 等），每个群组预设 18-20 条符合该主题的对话内容。数据包含已删除消息、已编辑消息、附件消息、reactions 等边界情况，确保 Mock 模式下能覆盖各种 UI 状态。ID 生成使用固定起始值（`seqId = 1`），保证每次运行生成相同的数据，CI 测试结果可复现。

**使用场景：**
- Mock 模式初始化：`mock.js` 的 `ensureData()` 调用 `generateDummyData()` 生成初始数据
- CI 测试：Playwright 测试依赖确定性的假数据（固定 ID、固定消息内容），验证前端渲染逻辑
- 开发调试：开发者登录后立即看到丰富的聊天列表和消息，无需手动创建测试数据

**`generateDummyData({ chatCount, msgPerChat })` 返回：**
```javascript
{ chats, messages, activeChatId, onlineUserIds }
```

**数据规模：** 默认 10 chat（1 DM + 9 group）× 65 条消息 = ~650 条消息。

**群组预设：** 8 个命名群组（General, Random, Dev Team, Gaming, Music Club, Movie Night, Food & Cooking, Travel Pics），每个群组有 18-20 条预设对话。

**特殊消息：**
- `mi === 2` → deleted（空内容）
- `mi === 5` → edited（设 edited_at）
- `mi === 4` → attachments（2 个文件）
- `mi > 5 && mi % 3 === 0` → reactions

---

## 路由层

> **功能概述：** 路由层使用 React Router v6 管理页面导航和权限控制。`App.jsx` 定义路由表并处理全局认证事件（token 过期时强制登出）；`ChatPage.jsx` 是主页面，管理三栏布局和实时连接的生命周期；`LoginPage.jsx` 和 `RegisterPage.jsx` 提供认证表单和开发者快捷入口。路由层还负责 URL 与 Store 状态的同步（如 `/g/:chatId` 对应 `activeChatId`）。
>
> **使用场景：** 用户访问 `/login` 看到登录表单，登录成功后跳转到 `/`（ChatPage）；用户点击聊天，URL 变为 `/g/:chatId`，ChatPage 加载对应聊天的消息；token 过期时，`client.js` 触发 `auth:unauthorized` 事件，App.jsx 捕获后强制跳转回 `/login`。

---

### `App.jsx` — 根路由

**位置：** `client/src/App.jsx`

**功能概述：** `App.jsx` 是前端的根组件，负责三件事：（1）定义路由表，将 URL 路径映射到页面组件；（2）处理全局认证事件——监听 `auth:unauthorized`，token 过期时强制登出并跳转登录页；（3）根据登录状态控制路由访问——未登录用户只能访问 `/login` 和 `/register`，其他路径一律重定向到 `/login`。

**使用场景：**
- 首次访问：未登录 → 重定向到 `/login`
- 登录后：跳转到 `/`，渲染 `ChatPage`
- 直接访问 `/g/:chatId`：已登录则渲染 `ChatPage`，未登录则重定向到 `/login`
- Token 过期：`auth:unauthorized` 触发 → 登出 → 跳转 `/login`

**路由表：**
| 路径 | 组件 | 条件 |
|------|------|------|
| `/login` | `LoginPage` | 未登录 |
| `/register` | `RegisterPage` | 未登录 |
| `/` | `ChatPage` | 已登录，否则 → `/login` |
| `/g/:chatId` | `ChatPage` | 已登录，否则 → `/login` |
| `/*` | `ChatPage` | 已登录，否则 → `/login` |

**⚠️ 路由问题：** `/*` 在 `/g/:chatId` 之前匹配，导致 `/g/:chatId` 实际被 `/*` 捕获。功能不受影响（都渲染 ChatPage），但意图不清晰。

**全局监听：**
```javascript
window.addEventListener('auth:unauthorized', () => {
  if (!loggingOut.current) {
    loggingOut.current = true;
    logout();
    navigate('/login');
    setTimeout(() => { loggingOut.current = false; }, 500);
  }
});
```

**条件分支：**
- `auth:unauthorized` 事件触发 → 登出 + 跳转 `/login`
- `loggingOut.current === true` → 跳过（500ms 防抖）

---

### `ChatPage.jsx` — 主页面

**位置：** `client/src/routes/ChatPage.jsx`

**功能概述：** `ChatPage.jsx` 是前端的主页面，承担四大职责：（1）三栏布局骨架——左侧 sidebar（聊天列表）、中间 chat（消息视图）、右侧 member-panel（成员面板）；（2）实时连接生命周期管理——进入页面时建立 WS/SSE/Polling 连接，离开时断开；（3）URL 路由与 Store 状态同步——`/g/:chatId` 对应 `activeChatId`；（4）移动端适配——小于 768px 时切换为 list/chat 两视图。

**使用场景：**
- 用户登录后跳转到此页面，自动建立实时连接并加载聊天列表
- 点击聊天时，URL 变为 `/g/:chatId`，加载该聊天的消息
- 发送消息后，自动滚动到最新消息
- 切换聊天时，断开旧连接、建立新连接、加载新消息
- 移动端：侧边栏占满屏幕，点击聊天后切换到消息视图

**职责：**
1. 三栏布局（sidebar / chat / member panel）
2. 实时连接生命周期管理
3. URL 路由 ↔ store 状态同步
4. 移动端视图切换

**关键逻辑：**

```javascript
// 连接生命周期
useEffect(() => {
  if (!accessToken) return;
  const mode = chatStore.mode;
  if (mode === 'ws') chatStore.connectWS(accessToken);
  else if (mode === 'sse') chatStore.connectSSE(accessToken);
  else chatStore.connectPolling(accessToken);
  chatStore.loadChats(accessToken);
  return () => chatStore.disconnect();
}, [accessToken]);

// URL → store 同步
useEffect(() => {
  if (urlChatId) { chatStore.setActiveChat(urlChatId); setMobileView('chat'); }
}, [urlChatId]);

// 消息加载 + 已读标记
useEffect(() => {
  if (activeChatId) {
    if (messages.length === 0) chatStore.loadMessages(accessToken, activeChatId);
    api.markRead(accessToken, activeChatId, lastMsg.id).catch(() => {});
  }
}, [activeChatId]);
```

**⚠️ 已知问题：**
- `messages.length === 0` 检查的是全局 messages 数组（所有 chat），不是当前 chat。若其他 chat 有消息，会跳过加载。
- `api.markRead` 失败被静默吞掉。

**移动端适配：**
```javascript
const [mobileView, setMobileView] = useState('list'); // 'list' | 'chat'
const [isMobile] = useState(() => window.innerWidth < 768);
```

---

### `LoginPage.jsx` / `RegisterPage.jsx`

**功能概述：** 登录和注册页面，提供用户认证表单。`LoginPage.jsx` 额外包含开发者调试区——「Debug mode」复选框展开后显示「Quick Enter」按钮（一键 Mock 登录）和「Mock API」开关（实时切换 Mock/真实后端）。`RegisterPage.jsx` 无 Mock 开关（需先注册才能 Mock）。

**使用场景：**
- 新用户：访问 `/register`，填写邮箱/用户名/密码注册，成功后自动登录跳转到 `/`
- 老用户：访问 `/login`，输入邮箱/密码登录
- 开发者：点击「Debug mode」→「Quick Enter」，一键注入 `dev-self` 用户并切换到 polling 模式
- Mock 切换：勾选「Mock API」启用/禁用 Mock 模式，无需重启应用

**共同点：**
- 调用 `useAuthStore` 的 `login()`/`register()`
- 成功后 `navigate('/')`
- 支持 `debugMode` 快捷入口（`mockLogin()`）

**LoginPage 额外：** Mock API 开关（直接调用 `api.enableMock()`/`api.disableMock()`）。

**RegisterPage 额外：** 无 mock 开关（需要先注册才能 mock）。

---

## 跨模块问题汇总

| # | 问题 | 位置 | 严重性 | 状态 |
|---|------|------|--------|------|
| 1 | `/g/:chatId` 路由被 `/*` 吞掉 | App.jsx | 低 | ⏭️ 功能正常 |
| 2 | `messages.length === 0` 检查全局而非当前 chat | ChatPage.jsx | 中 | ⏭️ 首次加载有影响 |
| 3 | `api.markRead` 失败静默吞掉 | ChatPage.jsx | 低 | ⏭️ 可接受 |
| 4 | Mock 双数据源（c.members vs d.chatMembers） | mock.js | 中 | ⏭️ mock 模式特有 |
| 5 | `DmSearchPanel` 未被引用 | DmSearchPanel.jsx | 低 | ⏭️ 死代码 |
| 6 | 无顶层 Error Boundary | main.jsx | 高 | ⏭️ 生产风险 |
| 7 | HTTP 429 用 `alert()` 处理 | ChatView.jsx | 中 | ⏭️ 体验差 |
| 8 | `#avatar-file-input` DOM 耦合 | SettingsModal / ChatList | 低 | ⏭️ 可用 |
| 9 | Mock 模式无 presence | mock.js | 低 | ⏭️ 预期行为 |
| 10 | 无 i18n，中英文混用 | PublicChannelList | 低 | ⏭️ |
