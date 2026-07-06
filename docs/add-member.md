# Add Member 机制

## 完整流程

### 1. 前端触发

`MemberPanel.jsx` — 点 **+ Add member** → 搜索用户 → 点击结果：

```jsx
const addUser = async (userId) => {
  await api.addMember(accessToken, chatId, userId);
};
```

- 搜索时调用 `api.searchUsers(q)` 查所有用户
- 搜索结果自动过滤掉已在群中的成员
- 只有 `type !== 'dm'` 的群聊显示 Add member 按钮

### 2. API 调用

`client/src/api/client.js:55-56`

```js
addMember: (token, chatId, userId) =>
  request('POST', '/api/chats/' + chatId + '/members', token, { user_id: userId }),
```

### 3. 后端 handler

`server/internal/handlers/chats.go:211-251`

校验链：
1. 聊天必须存在
2. 不能是 DM
3. 当前用户必须是该群已有成员（否则 403）
4. 目标用户必须存在（否则 404）
5. 调 `DB.AddChatMember()`

### 4. 数据库层

`server/internal/db/chats.go:319-332`

```go
INSERT OR IGNORE INTO chat_members (chat_id, user_id) VALUES (?,?)
```

- 成功插入返回 nil
- 如果用户已在群中（影响 0 行），返回 `ErrConflict` → 前端收到 409 `already_member`

### 5. 实时广播

添加成功后：
- `Hub.BroadcastChatUpdated(updated)` — 所有在线用户更新群信息
- `Hub.NotifyUserNewChat(req.UserID, updated)` — 被拉入的用户收到 `chat_create` 事件

### 6. 前端收尾

`MemberPanel.jsx` 关闭添加模式、清空搜索。`chat_id` 对应的群数据会通过 WebSocket `chat_update` 事件自动更新（含新的 members 列表），无需手动刷新。

## Remove Member

`server/internal/handlers/chats.go:253-280`

- 不能是 DM
- 踢自己：无需 owner 权限
- 踢别人：仅 owner 可以
- 不能踢 owner
- 删除后广播 `chat_update`

## 关键文件

| 层 | 文件 |
|----|------|
| 成员面板 UI | `client/src/components/MemberPanel.jsx` |
| API 方法 | `client/src/api/client.js:55-58` |
| AddMember handler | `server/internal/handlers/chats.go:211-251` |
| AddChatMember DB | `server/internal/db/chats.go:319-332` |
| RemoveMember handler | `server/internal/handlers/chats.go:253-280` |
| 路由注册 | `server/internal/handlers/router.go:56-57` |
