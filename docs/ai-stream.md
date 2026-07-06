# AI 打字机消息机制

## 架构

基于 **promise 风格的 stream 源头**。`streamFn` 是一个接收 `emit(chunk)` 回调的 async 函数，`emit` 每次被调用时 store 即时追加内容，渲染层直接展示。

## Stream 源头抽象

`client/src/dev/stream-source.js`

```js
createStreamSource(async (emit) => {
  // 调用 emit(chunk) 逐块推送
  // 函数 resolve 时 stream 结束
})
```

返回 `{ onChunk(cb), done: Promise }`。

## Store 层

`store/chat.js:276-305`

```js
startStreamingInChat(chatId, streamFn) {
  // 1. 创建空消息 (content: '', streaming: true)
  // 2. 注入到 messages
  // 3. 调用 createStreamSource(streamFn)
  //    .onChunk(chunk => 追加到 msg.content)
  //    .done  → finishStreaming(msgId)
}
```

关键变化：不再预置内容，而是通过 `emit(chunk)` 实时追加。

## 渲染层

`MessageItem.jsx` — 移除旧的 `setInterval` + `visibleLen` 动画，直接渲染 `msg.content`：

```jsx
{msg.streaming ? (
  <div className="msg-content" style={{whiteSpace:'pre-wrap',wordBreak:'break-word'}}>
    {msg.content}<span className="stream-cursor" />
  </div>
) : (
  <ReactMarkdown ...>{msg.content}</ReactMarkdown>
)}
```

## 使用示例

### 模拟流 (demo)

```js
startStreamingInChat(chatId, async (emit) => {
  const text = 'Hello world!';
  for (const char of text) {
    await new Promise(r => setTimeout(r, 40));
    emit(char);
  }
});
```

### 真实 SSE

```js
startStreamingInChat(chatId, async (emit) => {
  const res = await fetch('/api/ai/chat', { ... });
  const reader = res.body.getReader();
  const dec = new TextDecoder();
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    emit(dec.decode(value, { stream: true }));
  }
});
```

## 数据流

```
Composer 🤖 点击
  → store.startStreamingInChat(chatId, async (emit) => { ... })
    → 插入 { content: '', streaming: true }
    → createStreamSource(streamFn)
      → streamFn 执行中，emit(chunk) 被调用
        → onChunk: store 追加 chunk 到 msg.content
          → MessageItem 重渲染，显示新内容
      → streamFn resolve
        → .done.then → finishStreaming(msgId)
          → streaming: false → 切换 Markdown 渲染
```

## 关键文件

| 文件 | 说明 |
|------|------|
| `src/dev/stream-source.js` | Stream 源头抽象 |
| `src/store/chat.js:276-305` | `startStreamingInChat` |
| `src/components/MessageItem.jsx` | 流式/完成两种渲染 |
| `src/components/Composer.jsx:74` | 🤖 触发按钮 |

