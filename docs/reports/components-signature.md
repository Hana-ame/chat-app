# Components 签名

| 组件 | 导出 | Props | 说明 |
|------|------|-------|------|
| `ChatList` | `default` | `{ onSelectChat, activeId, onLogout }` | 侧栏：聊天列表、搜索、建群、设置 |
| `ChatView` | `default` | `{ chatId, onBack }` | 主区域：消息列表 + Composer |
| `MessageItem` | `default` | `{ msg, sameAuthor, chatId }` | 单条消息：文本/Markdown/流式/附件/reaction/编辑 |
| `Composer` | `default` | `{ chatId }` | 输入框：文本、附件上传、AI 流式触发 |
| `MemberPanel` | `default` | `{ chatId }` | 成员面板：列表、搜索添加、踢人 |

## 详细 Props

### ChatList

```jsx
ChatList({
  onSelectChat: (chatId: string) => void,  // 选中聊天
  activeId: string | null,                  // 当前高亮 chat id
  onLogout: () => void,                     // 登出回调
})
```

### ChatView

```jsx
ChatView({
  chatId: string,           // 当前聊天 id
  onBack?: () => void,      // 返回按钮（移动端用）
})
```

### MessageItem

```jsx
MessageItem({
  msg: {                    // 消息对象
    id: string,
    chat_id: string,
    content: string,
    user_id: string,
    author: { id, username, avatar_color, avatar_url? },
    created_at: string,
    edited_at?: string,
    deleted: boolean,
    streaming?: boolean,
    attachments: Array<{ id, filename, mime_type, size, url }>,
    reactions: Array<{ emoji, count, me }>,
  },
  sameAuthor: boolean,      // 与上一条消息同作者（隐藏头像）
  chatId: string,
})
```

### Composer

```jsx
Composer({
  chatId: string,
})
```

### MemberPanel

```jsx
MemberPanel({
  chatId: string,
})
```
