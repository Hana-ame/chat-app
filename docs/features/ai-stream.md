# AI 打字机消息机制

## 架构

消息自带流入口。每条 `streaming: true` 的消息携带 `source` 字段，store 收到后自动消费。

## Source 规范

`source` 是一个对象，支持两种类型：

| 类型 | 字段 | 说明 |
|------|------|------|
| `mock` | `fn(emit)` | 模拟流：async 函数逐块调用 `emit(chunk)` |
| `sse` | `url` | 真实 SSE：fetch 该 URL 并解析 `data:` 行 |

## Store 层

`store/chat.js` — `onMessageCreate` 自动消费：

```js
// 收到消息后，若含 source 则自动消费
onMessageCreate(msg) {
  set(s => { /* 追加到 messages */ });
  if (msg.streaming && msg.source) {
    get().startConsumingStream(msg);
  }
}

// 消费逻辑
startConsumingStream(msg) {
  api.startStreaming(msg.source)
    .onChunk(chunk => 追加到 msg.content)
    .done.then(() => finishStreaming(msg.id))
}
```

不再需要 `startStreamingInChat`。

## API 层

`api/client.js` — 统一入口适配多种 source：

```js
api.startStreaming = (source) => {
  if (typeof source === 'function') return createStreamSource(source);
  if (source.type === 'mock') return createStreamSource(source.fn);
  if (source.type === 'sse') return createStreamSource(async (emit) => {
    const res = await fetch(source.url);
    const reader = res.body.getReader();
    // 逐行解析 data: 事件 → emit
  });
};
```

## Mock 层

`api/mock.js` — `mockSendMessage` 创建 AI 消息时嵌入 `source`：

```js
const aiMsg = {
  id: 'mock-ai-' + Date.now(),
  content: '',
  user_id: 'ai',
  streaming: true,
  source: {
    type: 'mock',
    fn: async (emit) => {
      for (const char of text) {
        await new Promise(r => setTimeout(r, 30));
        emit(char);
      }
    },
  },
};
cur.onMessageCreate(aiMsg);
```

消息一旦入 store，自动触发消费 → chunk 填充 → 渲染。

## Mock 实现详解

### 启动

`Composer 🤖` → 点击后先检查 `api.isMockEnabled()`，未启用则 alert。启用后调 `sendMessage`。

### API 替换机制

`api/client.js` 提供三个开关方法：

```js
// 保存原始实现
const _real = { listChats: api.listChats, listMessages: api.listMessages, sendMessage: api.sendMessage };

api.enableMock = () => {
  api.listChats = mockListChats;
  api.listMessages = mockListMessages;
  api.sendMessage = mockSendMessage;
  _mockEnabled = true;
};

api.disableMock = () => {
  api.listChats = _real.listChats;
  api.listMessages = _real.listMessages;
  api.sendMessage = _real.sendMessage;
  _mockEnabled = false;
};
```

调用 `api.enableMock()` 后，所有后续 `api.sendMessage()` 调用自动走 `mockSendMessage`。

### AI 消息生成

`mockSendMessage` 接收 `(token, chatId, content, attachments)`，执行：

1. **创建用户消息** — 将用户输入包装为 Alice 消息，调 `onMessageCreate` 写入 store

```js
const userMsg = {
  id: 'mock-msg-' + Date.now(),
  chat_id: chatId,
  content,
  user_id: 'dev-self',
  author: { id: 'dev-self', username: 'Alice', avatar_color: '#5865F2' },
  created_at: now, deleted: false, attachments: attachments || [], reactions: [],
};
s.onMessageCreate(userMsg);
```

2. **延迟后创建 AI 消息 + source** — 随机等待 500–1700ms，从 `AI_RESPONSES` 选一句

```js
const delay = 500 + Math.random() * 1200;
setTimeout(() => {
  const text = AI_RESPONSES[Math.floor(Math.random() * AI_RESPONSES.length)];
  const aiMsg = {
    id: 'mock-ai-' + Date.now(),
    chat_id: chatId,
    content: '',
    user_id: 'ai',
    author: { id: 'ai', username: 'AI Bot', avatar_color: '#10a37f' },
    created_at: new Date().toISOString(),
    streaming: true,
    deleted: false, attachments: [], reactions: [],
    source: {                                          // ← 关键：流入口
      type: 'mock',
      fn: async (emit) => {
        for (const char of text) {
          await new Promise(r => setTimeout(r, 25 + Math.random() * 20));
          emit(char);
        }
      },
    },
  };
  cur.onMessageCreate(aiMsg);  // 入 store → 自动消费 source → 流式渲染
}, delay);
```

3. **`onMessageCreate` 自动消费** — store 检测到 `streaming && source`，调 `startConsumingStream` → `api.startStreaming` → `createStreamSource(fn)` → 逐字填充 `msg.content` → React 重渲染

### AI_RESPONSES

```js
const AI_RESPONSES = [
  "That's an interesting question! Let me think about it...",
  "I've analyzed the data and here's what I found.",
  "Great point! I'd like to add some context.",
  // ... 共 10 条，随机选取
];
```

### 打字机参数

| 参数 | 值 | 说明 |
|------|-----|------|
| AI 延迟 | 500–1700ms | 模拟思考时间 |
| 字符间隔 | 25–45ms | 模拟打字速度 |

## 渲染层

`MessageItem.jsx` — 渲染 `msg.content`：

```jsx
{msg.streaming ? (
  <div className="msg-content" style={{whiteSpace:'pre-wrap',wordBreak:'break-word'}}>
    {msg.content}<span className="stream-cursor" />
  </div>
) : (
  <ReactMarkdown ...>{msg.content}</ReactMarkdown>
)}
```

## 数据流

```
🤖 点击
  → sendMessage → mockSendMessage
    → onMessageCreate(userMsg)
    → setTimeout → onMessageCreate(aiMsg { source })
      → store.startConsumingStream(aiMsg)
        → api.startStreaming(aiMsg.source)
          → .onChunk: 追加到 msg.content
            → MessageItem 重渲染
          → stream resolve
            → finishStreaming → streaming: false → Markdown
```

## Source 分离的意义

- **消息附带 source**：流入口是消息的一个属性，不再依赖外部调用
- **mock 用 promise，真实用 SSE**：同一接口，不同实现
- **生产流程**：后端创建 streaming 消息（含 SSE URL）→ store 自动消费
- **纯组件渲染**：MessageItem 只关心 `msg.content`，不关心流逻辑

## 关键文件

| 文件 | 说明 |
|------|------|
| `src/dev/stream-source.js` | 底层 `createStreamSource` 抽象 |
| `src/api/client.js` | `api.startStreaming` — 统一入口 |
| `src/api/mock.js` | `mockSendMessage` — AI 消息 + source 构建 |
| `src/store/chat.js` | `onMessageCreate` + `startConsumingStream` |
| `src/components/MessageItem.jsx` | 流式/完成两种渲染 |
| `src/components/Composer.jsx:75` | 🤖 触发按钮 |

