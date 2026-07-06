# Chat 可见性（Visibility）定义

三种级别，适用于 `type='group'` 的聊天：

| 级别 | DB 值 | 搜索公开频道 | 用 ID 加入 | 只能拉人 |
|------|-------|------------|-----------|---------|
| Public | `'public'` | ✅ 可搜到 | ✅ | ❌ |
| Unlisted | `'unlisted'` | ❌ 搜不到 | ✅ | ❌ |
| Private | `'private'` | ❌ | ❌ | ✅ |

## 规则

- **Public**: 出现在 `GET /api/chats/public` 结果中，任何用户可通过搜索发现并加入
- **Unlisted**: 不出现在公开列表，但知道 chat ID 的用户可通过 `POST /api/chats/{id}/join` 加入
- **Private**: 不接受直接加入，只能由已有成员通过 `POST /api/chats/{id}/members` 拉入
- DM 的 `visibility` 固定为空字符串 `""`，不适用此分级

## 后端校验

`server/internal/db/chats_ext.go:38-56` — `JoinChatByID` 执行时检查：

```go
if visibility == "private" {
    return errors.New("chat is private, invitation required")
}
```

## 前端入口

`client/src/components/ChatList.jsx`:
- **Public 搜索**: 搜索栏切换到 🌐 Public tab，输入关键词搜索
- **ID 加入**: 搜索栏下方的 "Join by chat ID..." 输入框
- **建群**: "Create Group" 弹窗的三个 radio 选项
