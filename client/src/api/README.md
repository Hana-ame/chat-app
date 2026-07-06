# api — HTTP 请求封装

**文件**: `client.js`

所有后端通信统一经过此模块，由 `request()` 函数封装：

- 自动拼接 `API_BASE`（pages.dev 环境指向 `wsl-8080.moonchan.xyz`，本地开发为空）
- 自动注入 `Authorization: Bearer <token>`
- JSON body 序列化 / 响应解析
- 全局 `401` 事件派发（触发 `auth:unauthorized` → 自动跳登录）

---

## 方法分组

### Auth

| 方法 | 路径 | 说明 |
|------|------|------|
| `register(email, username, password)` | POST `/api/auth/register` | 注册 |
| `login(email, password)` | POST `/api/auth/login` | 登录 |
| `refresh(refreshToken)` | POST `/api/auth/refresh` | 刷新 token |
| `logout(token, refreshToken)` | POST `/api/auth/logout` | 登出 |
| `me(token)` | GET `/api/users/me` | 当前用户信息 |
| `updateProfile(token, data)` | PATCH `/api/users/me` | 修改资料（username / avatar_url） |
| `searchUsers(token, q)` | GET `/api/users?q=` | 搜索用户 |

### Chats

| 方法 | 路径 | 说明 |
|------|------|------|
| `listChats(token)` | GET `/api/chats` | 我的聊天列表 |
| `listPublicChats(token)` | GET `/api/chats/public` | 公开群组列表 |
| `createChat(token, name, memberIds, visibility)` | POST `/api/chats` | 创建群组 |
| `getChat(token, id)` | GET `/api/chats/:id` | 聊天详情 |
| `deleteChat(token, id)` | DELETE `/api/chats/:id` | 删除聊天 |
| `renameChat(token, id, name)` | PATCH `/api/chats/:id` | 重命名 |
| `createDM(token, userId)` | POST `/api/dms` | 发起私聊 |
| `joinChat(token, chatId)` | POST `/api/chats/:id/join` | 加入公开群 |
| `pinChat(token, chatId)` / `unpinChat` | POST `/api/chats/:id/pin\|unpin` | 置顶 / 取消 |

### Members

| 方法 | 路径 | 说明 |
|------|------|------|
| `addMember(token, chatId, userId)` | POST `/api/chats/:id/members` | 添加成员 |
| `removeMember(token, chatId, userId)` | DELETE `/api/chats/:id/members/:userId` | 踢出成员 |

### Messages

| 方法 | 路径 | 说明 |
|------|------|------|
| `listMessages(token, chatId, before?, limit?)` | GET `/api/chats/:id/messages` | 历史消息 |
| `sendMessage(token, chatId, content, attachments?)` | POST `/api/chats/:id/messages` | 发送消息 |
| `editMessage(token, chatId, msgId, content)` | PATCH `/api/chats/:id/messages/:msgId` | 编辑 |
| `deleteMessage(token, chatId, msgId)` | DELETE `/api/chats/:id/messages/:msgId` | 删除 |
| `markRead(token, chatId, messageId)` | POST `/api/chats/:id/read` | 标记已读 |

### Reactions

| 方法 | 路径 |
|------|------|
| `addReaction(token, chatId, msgId, emoji)` | PUT `/api/chats/:id/messages/:msgId/reactions/:emoji` |
| `removeReaction(token, chatId, msgId, emoji)` | DELETE `/api/chats/:id/messages/:msgId/reactions/:emoji` |

### Uploads

所有上传走外部服务 `upload.moonchan.xyz`，方法为 `PUT /api/upload`，multipart form field `file`。

| 方法 | 返回 |
|------|------|
| `upload(file)` | `{ filename, mime_type, size, url }` |
| `uploadAvatar(_token, file)` | `{ url }` — 内部委托 `upload()` |

### Misc

| 方法 | 说明 |
|------|------|
| `sseUrl(token)` | 拼接 SSE 事件流 URL（含 `access_token` 参数） |

---

## 错误处理

- 响应非 2xx 时，thrown error 包含 `{ status, ...responseBody }`
- 401 自动触发 `window.dispatchEvent(new CustomEvent('auth:unauthorized'))`，`App.jsx` 收到后执行 logout → 跳转 `/login`
- 防重入：`App.jsx` 使用 `useRef` 在 500ms 窗口内忽略重复 401

---

## 修改记录

| 日期 | 变更 |
|------|------|
| 2026-07-06 | 创建 `README.md`；提取 `buildUploadUrl` 辅助函数；`uploadAvatar` 委托 `upload` |
