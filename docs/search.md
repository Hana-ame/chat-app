# Search 机制

## 搜索栏（单输入框）

`ChatList.jsx` — 一个 input + onChange 触发两种搜索：

### 1. 本地聊天过滤

输入时实时过滤 `chats` 数组：

```js
chats.filter(c => {
  if (!chatSearch.trim()) return true;
  const q = chatSearch.toLowerCase();
  const name = c.type === 'dm' ? getDMName(c, user.id) : c.name || '';
  return name.toLowerCase().includes(q) || c.id.toLowerCase().includes(q);
})
```

匹配字段：`chat.name` / `chat.id` / DM 对方 username，大小写不敏感。

### 2. 公开频道搜索

每次输入变化调用 `searchPublic()`：

```js
const searchPublic = async (q) => {
  setPublicSearching(true);
  const data = await api.listPublicChats(accessToken);
  const all = data.chats || [];
  const matched = all.filter(c =>
    c.name?.toLowerCase().includes(lower) || c.id.toLowerCase().includes(lower)
  );
  setPublicResults(matched);
  setPublicSearching(false);
};
```

- 调用 `GET /api/chats/public` 获取所有公开频道
- 客户端按 name/ID 过滤匹配
- 结果显示在 **Public Channels** 分组下，可点击加入
- 状态：**搜索中...** → **结果列表** / **无结果**

### 3. 数字操作按钮

输入纯数字（`/^\d+$/` 或 `x-y` 格式）时，自动显示操作按钮：
- **Join #{id}** — 调用 `POST /api/chats/{id}/join` 加入
- **Create "name"** — 弹出建群面板，预填 name

## 关键文件

| 层 | 文件 |
|----|------|
| 搜索 UI + 逻辑 | `client/src/components/ChatList.jsx` |
| 公开频道 API | `client/src/api/client.js:50` (`listPublicChats`) |
| 后端公开列表 | `server/internal/handlers/chats_v2.go:9-16` |
| DB 查询 | `server/internal/db/chats_ext.go:9-36` |
