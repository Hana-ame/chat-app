# Create Group 机制

## 完整流程

### 1. 前端触发

`ChatList.jsx` — 点 **+** 按钮弹出创建面板：

```jsx
const handleCreate = async () => {
  const data = await api.createChat(accessToken, newChatName, [], newChatVisibility);
  onSelectChat(data.id);
};
```

三个输入：
- **Group name** — 必填
- **Visibility** — radio: `private` / `unlisted` / `public`
- (不选成员，创建者自己自动成为第一个成员)

### 2. API 调用

`client/src/api/client.js:42-43`

```js
createChat: (token, name, memberIds, visibility) =>
  request('POST', '/api/chats', token, { type: 'group', name, member_ids: memberIds, visibility }),
```

### 3. 后端 handler

`server/internal/handlers/chats.go:45-84`

- 校验: `type === "group"`、name 非空
- `member_ids` 如果没有自己则自动追加当前用户
- 调 `DB.CreateChat()`

### 4. 数据库层

`server/internal/db/chats.go:76-146`

1. 生成 UUID (`NewID()` 即 `uuid.NewString()`)
2. 从 name 算出一个 icon color (`PickColor`)
3. **Visibility 归一化**:
   ```go
   if visibility != "public" && visibility != "unlisted" {
       visibility = "private"
   }
   ```
4. INSERT `chats` 表
5. INSERT 每个成员到 `chat_members` 表
6. 重新 `GetChat` 读取完整数据返回

### 5. 实时广播

创建成功后，`s.Hub.BroadcastChatCreated(chat)` 通过 WebSocket 向所有在线用户广播 `chat_create` 事件。

### 6. 前端收尾

`handleCreate` 拿到返回的 chat 数据后调用 `onSelectChat(data.id)` 切换到新群组。

## 关键文件

| 层 | 文件 |
|----|------|
| UI 触发 | `client/src/components/ChatList.jsx:55-63` |
| API 方法 | `client/src/api/client.js:42-43` |
| Handler | `server/internal/handlers/chats.go:45-84` |
| DB 逻辑 | `server/internal/db/chats.go:76-146` |
| ID 生成 | `server/internal/db/users.go:17` |
| 可见性规则 | `docs/visibility.md` |
