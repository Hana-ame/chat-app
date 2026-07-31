# api — API 层

**文件**: `client.ts`（方法定义 + Mock Proxy）、`mock.js`（内存 mock 数据）、`schemas.ts`（类型）。

后端通信统一经此模块的 `request()` 封装：

- JSON body 序列化 / 响应解析
- 自动注入 `Authorization: Bearer <token>`（`credentials: 'include'`）
- 全局 `401` 事件派发（`auth:unauthorized` → 自动跳登录）

## 方法分组

- **Auth**：`register / login / refresh / logout / me / updateProfile / searchUsers`
- **Chats**：`listChats / listPublicChats / createChat / createOrGetDM / getChat / deleteChat / renameChat / joinChat / getNotificationsChat / pinChat / unpinChat / updateChatAvatar|Banner|Background|Notify / setAnnouncement / markPinnedRead`
- **Members**：`listMembers / addMember / removeMember`
- **Messages**：`listMessages / sendMessage / editMessage / deleteMessage / markRead / addReaction / removeReaction / listReactions`
- **Notifications**：`notifications.listMessages / sendMessage / deleteMessage / markRead`
- **Uploads**：`upload(file)`（PUT `/api/upload`）、`uploadAvatar`、`uploadBanner`、`uploadBackground`
- **Realtime**：`sseUrl()`（`/api/events`，token 走 Cookie）

完整端点与契约以 [docs/api/reference.md](../../docs/api/reference.md) 为准（此处不重复列表，避免漂移）。

## Mock 模式

`api.enableMock()` 后 Proxy 拦截全部方法 → `mock.js` 内存实现：

- 特殊分支：`notifications` 等返回 **Promise 包装**的 mock handlers（同步返回值会导致调用方 `.then()` 崩溃）
- 其余属性命中 `mockHandlers` 表 → 调 `mock.js` 对应函数
- 进入方式：`window.__mockLogin()`（dev 环境挂载），详见 [docs/guide/development.md](../../docs/guide/development.md)
- mock 数据隔离：`enableMock()` 调用 `resetMockData()` 重建内存数据

## 错误处理

- 非 2xx：thrown error 携带 `{ status, ...body }`
- 401 触发 `auth:unauthorized` 事件；`App.jsx` 防重入（500ms 窗口）后 logout 跳 `/login`
