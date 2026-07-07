# ChatList 侧栏逻辑

## 数据来源

`chats` 数组来自 `useChatStore`，通过 WebSocket `ready` 事件或 `loadChats()` 填充。

## 排序规则（`store/chat.js:157-165`）

```js
const sorted = (chats || []).sort((a, b) => {
  if (a.pinned !== b.pinned) return a.pinned ? -1 : 1;
  const da = a.last_message_at || a.created_at;
  const db = b.last_message_at || b.created_at;
  return new Date(db) - new Date(da);
});
```

1. **Pinned 优先**: 置顶聊天排在最前
2. **最后活动时间倒序**: 按 `last_message_at`（或 `created_at`）最近的在前面

## 聊天类型

| 类型 | `type` 字段 | 显示名称 | 头像颜色 |
|------|------------|---------|---------|
| 私聊 DM | `'dm'` | 对方 `username` | 对方 `avatar_color` |
| 群组 Group | `'group'` | `chat.name` | `chat.icon_color` |

判断方式 (`ChatList.jsx:227-228`):

```js
const name = c.type === 'dm' ? getDMName(c, user.id) : c.name;
const avatar = c.type === 'dm'
  ? (c.members?.find(m => m.id !== user.id)?.avatar_color || c.icon_color)
  : c.icon_color;
```

## 样式决定（`ChatList.jsx:231`）

```jsx
className={'chat-item'
  + (c.id === activeId ? ' active' : '')
  + (c.pinned ? ' pinned' : '')
  + (c.visibility === 'public' ? ' public' : '')}
```

- `.active` — 当前选中高亮
- `.pinned` — 左边 accent 色竖线
- `.public` — 浅绿色背景
- `hover` — 灰底
- `···` 菜单按钮 — 默认隐藏，hover 渐现

## 附加元素

- **未读 badge**: `c.unread_count > 0` 时显示红色圆角计数
- **最后消息预览**: `author: content` 单行省略，deleted 消息显示 `(message deleted)`
- **时间戳**: 相对时间（now / 5m / 3h / 日期）
- **右键菜单 (⋮)**: Pin/Unpin + Delete（仅 owner 可见）

## 筛选搜索（2026-07-06 新增）

搜索框根据输入匹配 `chat.name` / `chat.id` / DM 对方用户名，大小写无关。
