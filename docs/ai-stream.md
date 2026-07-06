# AI 打字机消息机制

## 概述

完全前端模拟，不涉及任何 AI API 调用。只是一个打字机效果的 UI 动画。

## 触发入口

`Composer.jsx` — 🤖 按钮：

```jsx
<button onClick={() => startStreamingInChat(chatId, text.trim() || '默认消息...')}>
  🤖
</button>
```

## Store 层

`store/chat.js:276-291`

```js
startStreamingInChat(chatId, content) {
  const msg = {
    id: 'stream-' + Date.now(),
    chat_id: chatId,
    content,                          // 完整文本
    user_id: 'ai',
    author: { id: 'ai', username: 'AI Bot', avatar_color: '#10a37f' },
    created_at: new Date().toISOString(),
    streaming: true,                  // 标记为流式
    deleted: false,
    attachments: [],
    reactions: [],
  };
  set(s => ({ messages: [...s.messages, msg] }));
}
```

`finishStreaming(msgId)` — 将 `streaming` 设为 `false`，触发渲染切换。

## 渲染层

`MessageItem.jsx:29-45` — 打字机动画：

```jsx
useEffect(() => {
  if (!msg.streaming) return;
  setVisibleLen(0);
  const speed = Math.max(20, Math.min(80, 4000 / msg.content.length));
  streamingRef.current = setInterval(() => {
    setVisibleLen(prev => {
      if (prev >= msg.content.length) {
        clearInterval(streamingRef.current);
        chatStore.finishStreaming(msg.id);   // 完成后关闭流状态
        return msg.content.length;
      }
      return prev + 1;                       // 逐字递增
    });
  }, speed);
  return () => { if (streamingRef.current) clearInterval(streamingRef.current); };
}, [msg.streaming, msg.content, msg.id]);
```

- 速度自适应: `4000ms / content.length`，限定在 20-80ms 之间
- 完成后 `finishStreaming` → 切换为普通 Markdown 渲染

### 两种渲染状态

| 状态 | 渲染方式 |
|------|---------|
| `msg.streaming === true` | 纯文本 `slice(0, visibleLen)` + 闪烁光标 `.stream-cursor` |
| `msg.streaming === false` | 完整 ReactMarkdown 渲染（含 GFM、链接、图片等） |

`stream-cursor` CSS (`global.css`):

```css
.stream-cursor::after {
  content: '▊';
  animation: blink 0.8s step-end infinite;
}
@keyframes blink { 50% { opacity: 0; } }
```

## 数据流

```
Composer 🤖 点击
  → store.startStreamingInChat(chatId, content)
    → 插入 streaming:true 的消息到 messages 数组
    → MessageItem 检测到 msg.streaming
      → useEffect 启动 setInterval 逐字 reveal
      → 每 tick setVisibleLen(prev + 1)
      → 渲染 msg.content.slice(0, visibleLen) + 光标
      → 全部 reveal 后调用 finishStreaming(msgId)
        → streaming:false
        → 切换为 ReactMarkdown 渲染
```

## 注意

- 不调用任何后端 API
- 消息只存在于当前前端 store，刷新即消失
- AI Bot 用户是虚拟的（`id: 'ai'`），无对应后端用户
