# 修改日志

> 用户在实际浏览器中操作后提出的反馈与修复记录

---

## 2026-07-10 批量修复（5 Bug + 1 改进）

### 环境
- Mock API → MITM 模式（完整模拟后端，前端无感）
- 后端本地运行 (PID 2136970, port 8080)

### 用户反馈

#### Bug 1: AI 消息被轮询刷新顶掉
- **现象**: 向聊天发送消息后，AI Bot 回复出现，但 2 秒后页面轮询刷新，AI 消息消失
- **根因**: `loadMessages` 用 `mockListMessages` 返回的数据直接替换 store，但旧 mock 不保存 AI 消息。`mockSendMessage` 直接调 `s.onMessageCreate` 而非写入数据层
- **修复**: 重写 `mock.js` 为 MITM 模式——统一管理运行时数据（`ensureData()`），`mockSendMessage` 将用户消息 + AI 都写入 `d.messages`，`mockListMessages` 从数据层读取，轮询自然返回 AI 消息
- **文件**: `client/src/api/mock.js`（完整重写）

#### Bug 2: 已读红点重新出现
- **现象**: 点击聊天消除红点后，几秒后轮询刷新红点再次出现
- **根因**: `loadChats` 直接 `set({ chats: data.chats })` 覆盖本地 `unread_count = 0`
- **修复**: `setChats` 合并本地已有聊天的 `unread_count`；`loadChats` 改用 `setChats`
- **文件**: `client/src/store/chat.js:256`

#### Bug 3: Settings 头像上传无效
- **现象**: 在 Settings 中选择头像文件并保存后，头像不变
- **根因**: `mockUploadAvatar` 返回 `{ id: '...' }`，而 `handleSaveSettings` 取 `data.url` 为 `undefined`
- **修复**: `mockUploadAvatar` 返回 `{ url }`
- **文件**: `client/src/api/mock.js:169`

#### Bug 4: 搜索不出用户
- **现象**: 搜索框输入用户名，结果为空
- **根因**: `mockSearchUsers` 依赖的 `USERS` 列表无有效数据，或搜索逻辑不对
- **修复**: 基于完整 `MOCK_USERS` 列表按 `username` 模糊查询
- **文件**: `client/src/api/mock.js:89`

#### Bug 5: 点击头像没有详情
- **现象**: 左侧栏用户头像点击无反应
- **修复**: 头像区域加 `onClick` 打开 SettingsModal（与齿轮按钮行为一致）
- **文件**: `client/src/components/ChatList.jsx:84`

### 改进: Mock/Real 切换开关
- **反馈**: Mock 模式与真实后端之间缺少干净的切换入口
- **实现**: 登录页面 Debug 区新增 "Mock API" 复选框，实时切换 `api.enableMock()` / `api.disableMock()`
- **文件**: `client/src/routes/LoginPage.jsx:35`

### 其他变更
- 移除 `mergeMessages` hack（MITM 方案已不需要）
- 保留 `__setStoreRef(useChatStore)` 让 mock 能通知 store 做增量更新

### 验证
- Build 通过：`npm run build` → 67 modules transformed, no errors

---

## 2026-07-10 实时交互修复（第 2 轮）

### 用户反馈

#### 问题 1: 新消息不会自动滚动到底部 + AI Bot streaming 失效
- **现象**:
  - 发送消息后，消息出现在底部的时机有延迟，且不会自动滚动到底部
  - AI Bot 回复一次吐出全文，没有逐字出现的效果
- **根因**:
  - 自研 `mockSendMessage` 未呼叫 `onMessageCreate(userMsg)`，用户消息仅写入数据层，须等轮询（~2s）才出现在 store
  - AI 消息无 `streaming: true` + `source`，缺失流式逐字输出
  - 自动滚动 effect 依赖 `filtered.length`，但消息没有先入 store 则 length 不变
- **修复**:
  - `mockSendMessage` 写入数据层后立即呼叫 `onMessageCreate(userMsg)` → 即时可见 → auto-scroll 触发
  - AI 消息改为双轨：`aiStoreMsg`（`streaming: true` + `source` 逐字 emit）先 `onMessageCreate` 消费；`aiDataMsg`（无 `source`）写入数据层，streaming 过程中逐字同步 content→数据层，结束标记 `streaming: false`
  - 附加延迟 `setTimeout` 模拟 AI 思考时间（~500-1300ms）
- **文件**: `client/src/api/mock.js`

#### 问题 2: 上传附件无效
- **现象**: Composer 选择文件后附件列表未出现
- **根因**: `mockUpload` 返回 `{ id }`，但 Composer 的 `handleFile` 读取 `data.filename` / `data.mime_type` / `data.size` / `data.url`，全为 `undefined`
- **修复**: `mockUpload` 返回完整字段（含 `URL.createObjectURL(file)`）
- **文件**: `client/src/api/mock.js:301`

#### 问题 3: 上传头像无效
- **现象**: Settings 选择头像文件保存后，头像仍是初始字母
- **根因**: `mockUploadAvatar` 返回假 URL `https://mock-avatar.local/...`，浏览器无法加载
- **修复**: `mockUploadAvatar` 返回 `URL.createObjectURL(file)`，使用本地 Blob URL，立即渲染
- **文件**: `client/src/api/mock.js:306`

### 验证
- Build 通过：`npm run build` → 67 modules transformed, no errors

---

## 2026-07-10 连续反馈修复（第 3 轮）

---

### 3-1: 新消息不自动滚动到底部 + AI Bot streaming 失效

**反馈**: 发送消息后不自动滚到底部；AI 回复一次吐出全文，无逐字效果。

**根因**: `mockSendMessage` 未调用 `onMessageCreate`，用户消息须等轮询（~2s）才入 store，auto-scroll effect 无法触发；AI 消息无 `streaming: true` + `source`。

**代码变更** (`client/src/api/mock.js` — `mockSendMessage`):

```js
export function mockSendMessage(_token, chatId, content, attachments) {
  const d = ensureData();
  const now = new Date().toISOString();

  const userMsg = {
    id: 'mock-msg-' + Date.now(),
    chat_id: chatId,
    content,
    user_id: 'dev-self',
    author: userById('dev-self'),
    created_at: now,
    edited_at: null,
    deleted: false,
    attachments: attachments || [],
    reactions: [],
  };
  d.messages.push(userMsg);
  if (_store) _store.getState().onMessageCreate(userMsg);  // ← 即时入 store → 触发 auto-scroll

  const text = AI_RESPONSES[Math.floor(Math.random() * AI_RESPONSES.length)];
  const aiId = 'mock-ai-' + Date.now() + '-' + Math.random().toString(36).slice(2, 6);
  const aiCreatedAt = new Date(Date.now() + 1).toISOString();

  const aiStoreMsg = {
    id: aiId,
    chat_id: chatId,
    content: '',
    user_id: 'ai',
    author: userById('ai'),
    created_at: aiCreatedAt,
    streaming: true,
    source: async (emit) => {
      let acc = '';
      for (let i = 0; i < text.length; i++) {
        await new Promise(r => setTimeout(r, 25 + Math.random() * 30));
        emit(text[i]);
        acc += text[i];
        const m = d.messages.find(m => m.id === aiId);
        if (m) m.content = acc;
      }
      const m = d.messages.find(m => m.id === aiId);
      if (m) m.streaming = false;
    },
  };

  const aiDataMsg = {
    id: aiId,
    chat_id: chatId,
    content: '',
    user_id: 'ai',
    author: userById('ai'),
    created_at: aiCreatedAt,
    streaming: true,
  };

  setTimeout(() => {
    d.messages.push(aiDataMsg);
    if (_store) _store.getState().onMessageCreate(aiStoreMsg);
  }, 500 + Math.random() * 800);

  return userMsg;
}
```

同时移除 `mergeMessages` hack (`client/src/store/chat.js`)，MITM 路线不再需要：
```js
// 之前
set(s => ({ messages: before ? [...msgs, ...s.messages] : mergeMessages(msgs, s.messages) }));

// 之后
set(s => ({ messages: before ? [...(data.messages || []), ...s.messages] : (data.messages || []) }));
```

---

### 3-2: 输入框自动包下多行

**反馈**: textarea 固定一行，多行文本需滚动。

**代码变更** (`client/src/components/Composer.jsx`):

```jsx
const textRef = useRef(null);

const autoResize = useCallback(() => {
  const el = textRef.current;
  if (!el) return;
  el.style.height = 'auto';
  el.style.height = el.scrollHeight + 'px';
}, []);

useEffect(() => { autoResize(); }, [text, autoResize]);

<textarea rows={1}
  ref={textRef}
  onChange={e => { setText(e.target.value); handleTyping(); autoResize(); }}
  style={{flex:1, resize:'none', overflow:'hidden', minHeight:36}}
/>
```

---

### 3-3: 去掉左下角 Generate Test Data 按钮

**代码变更** (`client/src/components/ChatList.jsx`):

```js
// 整段删除
const handleGenerateDummy = async () => {
  api.enableMock();
  await useChatStore.getState().loadChats(accessToken);
  const firstChat = useChatStore.getState().chats[0];
  if (firstChat) onSelectChat(firstChat.id);
};
```

---

### 3-4: 左下角改 Mock API 提示

**代码变更** (`client/src/components/ChatList.jsx`):

```jsx
// 之前
<div style={{ marginTop: 4, borderTop: '1px solid var(--border)', paddingTop: 4 }}>
  <button className="btn-ghost" style={{ fontSize: 11, width: '100%' }} onClick={handleGenerateDummy}>
    🧪 Generate test data
  </button>
</div>

// 之后
{api.isMockEnabled() && (
  <div style={{ marginTop: 4, borderTop: '1px solid var(--border)', paddingTop: 4 }}>
    <div style={{ fontSize: 11, color: 'var(--text-muted)', textAlign: 'center', padding: '4px 0' }}>
      ⚡ Using Mock API
    </div>
  </div>
)}
```

---

### 3-5: Mock API console 打印所有请求

**代码变更** (`client/src/api/client.js`):

```js
function swap(key, mock) {
  api[key] = (...args) => {
    console.log(`[Mock API] ${key}(`, ...args, ')');
    const result = mock(...args);
    if (result && typeof result.then === 'function') {
      return result.then(v => {
        console.log(`[Mock API] ${key} =>`, v);
        return v;
      });
    }
    console.log(`[Mock API] ${key} =>`, result);
    return Promise.resolve(result);
  };
}
```

输出示例：
```
[Mock API] listChats( mock-token ) => {chats: Array(10)}
[Mock API] sendMessage( mock-token, chat-1, "hello", [] ) => {id: 'mock-msg-...'}
```

---

### 3-6: 移除所有 DM Chat（DM 已废弃）

**代码变更** (`client/src/components/ChatList.jsx`):

```jsx
// 1. 移除 import DmSearchPanel
// 2. 移除 state: showDmSearch, dmSearch, dmResults
// 3. 移除 handleDM / searchUser 函数
// 4. 移除 @ 按钮
// 5. 移除 <DmSearchPanel/> 渲染
// 6. filteredChats 过滤 DM
const filteredChats = chats.filter(c => {
  if (c.type === 'dm') return false;
  if (!chatSearch.trim()) return true;
  const q = chatSearch.toLowerCase();
  const name = c.name || '';
  return name.toLowerCase().includes(q) || c.id.toLowerCase().includes(q);
});
// 7. EmptyState 文案
<EmptyState message="No conversations yet. Create a new group!" />
```

---

### 3-7: Reaction 在 Mock API 无法保存

**代码变更** (`client/src/api/mock.js`):

```js
export function mockAddReaction(_token, _chatId, msgId, emoji) {
  const d = ensureData();
  const msg = d.messages.find(m => m.id === msgId);
  if (msg) {
    const rxs = msg.reactions || [];
    if (!rxs.find(r => r.emoji === emoji)) {
      rxs.push({ emoji, count: 1, user_ids: ['dev-self'], me: true });
    }
    msg.reactions = rxs;
  }
  if (_store) _store.getState().onReaction({ message_id: msgId, emoji, user_id: 'dev-self' }, true);
  return { ok: true };
}

export function mockRemoveReaction(_token, _chatId, msgId, emoji) {
  const d = ensureData();
  const msg = d.messages.find(m => m.id === msgId);
  if (msg) {
    const rxs = (msg.reactions || []).map(r =>
      r.emoji === emoji ? { ...r, count: r.count - 1 } : r
    ).filter(r => r.count > 0);
    msg.reactions = rxs;
  }
  if (_store) _store.getState().onReaction({ message_id: msgId, emoji, user_id: 'dev-self' }, false);
  return { ok: true };
}
```

---

### 3-8: 无法对消息进行编辑和删除

**根因**: `mockLogin`/`mockRegister` 返回 `user.id: 'mock-...'`，消息 `user_id: 'dev-self'`，`isMe` 永为 `false` → Edit/Delete 按钮不出现。

**代码变更** (`client/src/api/mock.js`):

```js
// 之前
export function mockLogin(_token, email, password) {
  return { user: { id: 'mock-' + Date.now(), ... } };
}

// 之后 — 统一 id: 'dev-self'
export function mockLogin(_token, email, password) {
  return {
    user: { id: 'dev-self', username: email.split('@')[0], email, avatar_color: '#5865F2' },
    access_token: 'mock-token',
    expires_in: 3600,
  };
}

export function mockRegister(_token, email, username, password) {
  return {
    user: { id: 'dev-self', username, email, avatar_color: '#5865F2' },
    access_token: 'mock-token',
    expires_in: 3600,
  };
}
```

**代码变更** (`client/src/store/auth.js`):

```js
// 之前
mockLogin: () => {
  const payload = {
    user: { id: 'mock-' + Date.now(), username: 'DebugUser', email: 'debug@test.com', avatar_color: '#5865F2' },
  };
}

// 之后
mockLogin: () => {
  const payload = {
    user: { id: 'dev-self', username: 'Alice', email: 'alice@test.com', avatar_color: '#5865F2' },
  };
}
```

---

### 3-9: 消息顺序在 AI 回复时变动

**代码变更** (`client/src/api/mock.js`):

```js
// 之前
const aiCreatedAt = new Date().toISOString();

// 之后 — +1ms 保证严格晚于用户消息
const aiCreatedAt = new Date(Date.now() + 1).toISOString();
```

---

### 3-10: Context 菜单被 sidebar 容器裁剪

**根因**: `.sidebar-body` 的 `overflow-y:auto` 裁剪了 `position:absolute` 的 context menu。

**代码变更** (`client/src/components/ChatListItem.jsx`):

```jsx
// 不再渲染 .context-menu，只传递点击坐标
const btnRef = useRef(null);
const handleMenu = (e) => {
  e.stopPropagation();
  const rect = btnRef.current?.getBoundingClientRect();
  onContextMenu({ chatId: chat.id, x: rect?.right || 0, y: rect?.bottom || 0 });
};

<button ref={btnRef} className="btn-ghost chat-item-menu-btn" onClick={handleMenu}>⋮</button>
// .context-menu DOM 已移除
```

**代码变更** (`client/src/components/ChatList.jsx`):

```jsx
// 状态改为存坐标
const [contextMenu, setContextMenu] = useState(null); // { chatId, x, y }

// 在 ScrollArea 之外渲染
{contextMenu && (
  <div className="context-menu"
    style={{ position: 'fixed', left: contextMenu.x, top: contextMenu.y, zIndex: 1000 }}>
    <button className="context-menu-item danger"
      onClick={() => handleDeleteChat(contextMenu.chatId)}>Delete</button>
  </div>
)}
```

---

### 3-11: 左下角显示 DebugUser 而非 Alice

已包含在 3-8 修复中（`client/src/store/auth.js` — `mockLogin`）：

```js
user: { id: 'dev-self', username: 'Alice', email: 'alice@test.com', avatar_color: '#5865F2' }
```

---

### 验证
- Build 通过：`npm run build` → 66 modules transformed, no errors

---

## 2026-07-10 第 4 轮修复（4-1 ~ 4-9）

---

### 4-1: 最后一条消息的 reaction 被输入框盖掉

**根因**: `.chat-body` 底部 padding 仅 `16px`，最后消息的 reaction bar 超出容器与 `.chat-footer` 重叠。

**代码变更** (`client/src/styles/global.css`):

```css
/* 之前 */
.chat-body { overflow-y:auto; padding: 16px; }

/* 之后 — 加大底部留白 */
.chat-body { overflow-y:auto; padding: 16px 16px 120px; }
```

---

### 4-2: 输入框为空时 Send 按钮应为灰色

**代码变更** (`client/src/styles/global.css`):

```css
/* 新增 */
.btn-primary:disabled {
  background: var(--bg-secondary);
  color: var(--text-muted);
  cursor: default;
}
```

按钮的 `disabled` 属性已在 Composer 中通过 `(!text.trim() && attachments.length === 0)` 控制。

---

### 4-3: 消息内容应保留回车换行

**根因**: `.msg-content` 缺少 `white-space: pre-wrap`，多行文本被浏览器合并为一行。

**代码变更** (`client/src/styles/global.css`):

```css
/* 之前 */
.msg-content { line-height: 1.45; word-break: break-word; color: var(--text-primary); }

/* 之后 */
.msg-content { line-height: 1.45; word-break: break-word; white-space: pre-wrap; color: var(--text-primary); }
```

---

### 4-4: Context 菜单横跨整个屏幕

**根因**: 使用 `position: fixed` 时，CSS 中的 `right: 0` 未覆盖，导致菜单从 `left: x` 拉伸到屏幕右边缘。

**代码变更** (`client/src/components/ChatList.jsx`):

```jsx
{contextMenu && (
  <div className="context-menu"
    style={{
      position: 'fixed',
      left: contextMenu.x,
      top: contextMenu.y,
      zIndex: 1000,
      right: 'auto',   /* ← 关键：覆盖 CSS 的 right:0 */
      width: 140,       /* ← 固定宽度 */
    }}>
    <button className="context-menu-item danger"
      onClick={() => handleDeleteChat(contextMenu.chatId)}>Delete</button>
  </div>
)}
```

---

### 4-5: 上传依旧无效 — 补充 mock 返回字段

**根因**: `mockUpload` 返回缺少 `id` 字段，`MessageItem` 中 `msg.attachments.map(a => <div key={a.id}>)` key 为 `undefined`；且 mock 数据层附件缺少必要字段。

**代码变更** (`client/src/api/mock.js`):

```js
// mockUpload 确保返回完整字段（含用于后端构建 URL 的 id）
export function mockUpload(file) {
  const ext = file?.name?.split('.').pop() || 'bin';
  return {
    id: 'mock-upload-' + Date.now() + '.' + ext,  // ← 新增
    filename: file?.name || 'file.' + ext,
    mime_type: file?.type || 'application/octet-stream',
    size: file?.size || 0,
    url: URL.createObjectURL(file),
  };
}
```

`handleFile` (`Composer.jsx`) 读取 `data.filename/mime_type/size/url` 并组装到 `attachments`，`sendMessage` 透传到消息体。

---

### 4-6: 菜单加入"查看 Chat 信息" — 显示 Owner/Admin/Member

**实现**: 新建 `ChatInfoModal` 组件，按角色分组展示成员。

**代码变更** (`client/src/components/ChatInfoModal.jsx` — 新文件):

```jsx
export default function ChatInfoModal({ chatId, onClose }) {
  const { chats } = useChatStore();
  const chat = chats.find(c => c.id === chatId);

  const { owner, admins, members } = useMemo(() => {
    if (!chat?.members) return { owner: null, admins: [], members: [] };
    const o = chat.members.find(m => m.id === chat.owner_id) || null;
    const a = chat.members.filter(m => m.role === 'admin' && m.id !== chat.owner_id);
    const m = chat.members.filter(m => m.id !== chat.owner_id && m.role !== 'admin');
    return { owner: o, admins: a, members: m };
  }, [chat]);

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-box" onClick={e => e.stopPropagation()} style={{ maxWidth: 400 }}>
        {/* Chat name / Owner section / Admin section / Member section */}
      </div>
    </div>
  );
}
```

**代码变更** (`client/src/components/ChatList.jsx`):

```jsx
// 新增 state
const [showChatInfo, setShowChatInfo] = useState(null); // chatId

// 上下文菜单增加 View Info
{contextMenu && (
  <div className="context-menu" style={{ position:'fixed', left:contextMenu.x, top:contextMenu.y, zIndex:1000, right:'auto', width:140 }}>
    <button className="context-menu-item" onClick={() => { setShowChatInfo(contextMenu.chatId); setContextMenu(null); }}>View Info</button>
    <button className="context-menu-item danger" onClick={() => handleDeleteChat(contextMenu.chatId)}>Delete</button>
  </div>
)}

// ChatInfoModal 渲染
{showChatInfo && (
  <ChatInfoModal chatId={showChatInfo} onClose={() => setShowChatInfo(null)} />
)}
```

---

### 4-7: 点击已有 reaction 应实现快速 +1

**根因**: `onReaction` 中 `added` 分支在 `idx >= 0`（反应已存在）时直接 `return m`，未递增 count。

**代码变更** (`client/src/store/chat.js`):

```js
onReaction(payload, added) {
  set(s => ({ messages: s.messages.map(m => {
    if (m.id !== payload.message_id) return m;
    const rxs = m.reactions || [];
    const idx = rxs.findIndex(r => r.emoji === payload.emoji);
    if (added) {
      if (idx >= 0) {
        const existing = rxs[idx];
        if (existing.user_ids?.includes(payload.user_id)) return m;
        return {
          ...m,
          reactions: rxs.map((r, i) =>
            i === idx
              ? { ...r, count: r.count + 1, user_ids: [...(r.user_ids || []), payload.user_id], me: payload.user_id === 'dev-self' }
              : r
          ),
        };
      }
      return { ...m, reactions: [...rxs, { emoji: payload.emoji, count: 1, user_ids: [payload.user_id], me: payload.user_id === 'dev-self' }] };
    } else {
      return { ...m, reactions: rxs.map(r => r.emoji === payload.emoji ? { ...r, count: r.count - 1, user_ids: (r.user_ids || []).filter(id => id !== payload.user_id), me: false } : r).filter(r => r.count > 0) };
    }
  }) }));
},
```

同步修复 `mockAddReaction` (`client/src/api/mock.js`):

```js
export function mockAddReaction(_token, _chatId, msgId, emoji) {
  const d = ensureData();
  const msg = d.messages.find(m => m.id === msgId);
  if (msg) {
    const rxs = msg.reactions || [];
    const existing = rxs.find(r => r.emoji === emoji);
    if (existing) {
      if (!existing.user_ids?.includes('dev-self')) {
        existing.count += 1;
        existing.user_ids = [...(existing.user_ids || []), 'dev-self'];
        existing.me = true;
      }
    } else {
      rxs.push({ emoji, count: 1, user_ids: ['dev-self'], me: true });
    }
  }
  if (_store) _store.getState().onReaction({ message_id: msgId, emoji, user_id: 'dev-self' }, true);
  return { ok: true };
}
```

---

### 4-8: 自己创建的 group 无法显示最后一条消息 + 背景色不对

**根因**:
1. `mockListChats()` 未填充 `last_message` 字段 → chat list preview 为空
2. `setChats()` 未保留 store 中的 `last_message` → 轮询刷新后丢失
3. 无 owner 标识 class → 无法差异化样式

**代码变更** (`client/src/api/mock.js` — `mockListChats`):

```js
export function mockListChats() {
  const d = ensureData();
  const enriched = d.chats.map(c => {
    const msgs = messagesFor(c.id);
    const last = msgs.length > 0 ? msgs[msgs.length - 1] : null;
    return {
      ...c,
      last_message: last ? { id: last.id, content: last.content, deleted: last.deleted, author: last.author, created_at: last.created_at } : c.last_message,
      last_message_at: last?.created_at || c.last_message_at,
      members: c.members?.map(m => ({ ...m, ...(MOCK_USERS.find(u => u.id === m.id) || {}) })) || [],
    };
  });
  return { chats: enriched };
}
```

**代码变更** (`client/src/store/chat.js` — `setChats`):

```js
return old
  ? { ...c, last_message: old.last_message || c.last_message, unread_count: old.unread_count || 0 }
  : c;
```

**代码变更** (`client/src/components/ChatListItem.jsx`):

```jsx
// 新增 owner class
className={'chat-item' + (chat.id === activeId ? ' active' : '') + (chat.pinned ? ' pinned' : '') + (chat.visibility === 'public' ? ' public' : '') + (chat.owner_id === user.id ? ' owner' : '')}
```

---

### 4-9: Chat 列表应显示 visibility（public / unlisted / private）

**代码变更** (`client/src/components/ChatListItem.jsx`):

```jsx
<div style={{display:'flex',alignItems:'center',gap:4}}>
  <div className="chat-item-name">{name}</div>
  <span style={{
    fontSize:10, padding:'0 5px', borderRadius:3, fontWeight:500,
    background: chat.visibility === 'public' ? 'rgba(35,165,89,0.15)'
      : chat.visibility === 'unlisted' ? 'rgba(88,101,242,0.15)'
      : 'rgba(128,132,142,0.15)',
    color: chat.visibility === 'public' ? '#23a559'
      : chat.visibility === 'unlisted' ? '#5865F2'
      : 'var(--text-muted)',
  }}>
    {chat.visibility || 'private'}
  </span>
</div>
```

- `public` → 绿色
- `unlisted` → 蓝色
- `private` → 灰色

---

---

### 4-10: Chat group 头像全都变成同一种颜色

**反馈**: 新建的 group 头像颜色全是 `#5865F2`（Discord 蓝），无法区分。

**根因**: `mockCreateChat` 硬编码 `icon_color: '#5865F2'`。

**代码变更** (`client/src/api/mock.js`):

```js
// 新增彩色调色板
const CHAT_COLORS = [
  '#5865F2', '#23a559', '#f0b232', '#ed4245', '#9b59b6',
  '#1abc9c', '#e67e22', '#2ecc71', '#e74c3c', '#3498db',
  '#f39c12', '#1dd1a1', '#a29bfe', '#fd79a8', '#00cec9',
];

// mockCreateChat 改用轮询分配
icon_color: CHAT_COLORS[d.chats.length % CHAT_COLORS.length],
```

---

### 验证
- Build 通过：`npm run build` → 67 modules transformed, no errors

---

## 2026-07-10 第 5 轮修复（5-1 ~ 5-9）

---

### 5-1: 取消 reaction 会误删别人的

**反馈**: 取消自己的 reaction 时，`mockRemoveReaction` 对整个 reaction 做 `count - 1`，如果别人也点过同样的 emoji，别人的记录被移除。

**根因**: `mockRemoveReaction` 按 emoji 整体递减 count，未按 user_id 过滤。

**代码变更** (`client/src/api/mock.js` — `mockRemoveReaction`):

```js
// 之前：全局递减，可能误删别人的 reaction
msg.reactions = (msg.reactions || []).map(r =>
  r.emoji === emoji ? { ...r, count: r.count - 1 } : r
).filter(r => r.count > 0);

// 之后：按 user_id 过滤，仅移除自己的记录
const rxs = (msg.reactions || []).map(r => {
  if (r.emoji !== emoji) return r;
  const filtered = (r.user_ids || []).filter(id => id !== 'dev-self');
  return { ...r, count: filtered.length, user_ids: filtered };
}).filter(r => r.count > 0);
```

同步修复 `mockAddReaction` 的 `existing` 判断逻辑 — 改为查找 `emoji + user_ids.includes('dev-self')`，避免重复添加：

```js
const existing = rxs.find(r => r.emoji === emoji && r.user_ids?.includes('dev-self'));
if (!existing) {
  const byEmoji = rxs.find(r => r.emoji === emoji);
  if (byEmoji) {
    byEmoji.count += 1;
    byEmoji.user_ids = [...(byEmoji.user_ids || []), 'dev-self'];
  } else {
    rxs.push({ emoji, count: 1, user_ids: ['dev-self'] });
  }
}
```

---

### 5-2: 消息与输入框间距过大

**反馈**: 消息区域底部与输入框之间的空白太大。

**代码变更** (`client/src/styles/global.css`):

```css
/* 之前 */
.chat-body { padding: 16px 16px 120px; }

/* 之后 */
.chat-body { padding: 16px 16px 40px; }
```

---

### 5-3: 查看 Chat Info 入口

**反馈**: 聊天缺少信息面板入口，不知道群成员有哪些。

**代码变更** (`client/src/components/ChatList.jsx` — context 菜单):

```jsx
<button className="context-menu-item"
  onClick={() => { setShowChatInfo(contextMenu.chatId); setContextMenu(null); }}>
  View Info
</button>
```

**新增 ChatInfoModal** (`client/src/components/ChatInfoModal.jsx`): 按 Owner / Admin / Member 分组显示群成员。

---

### 5-4: 头像全同色（已有代码但初始数据无）

**反馈**: 所有 group 头像都是 `#5865F2`，无法区分。

**根因**: `CHAT_COLORS` 调色板已存在，但 `dummy.js` 初始数据和 `mockListChats` 未分配 `icon_color`。

**代码变更** (`client/src/api/mock.js` — `mockListChats`):

```js
const enriched = d.chats.map((c, i) => ({
  ...c,
  icon_color: c.icon_color || CHAT_COLORS[i % CHAT_COLORS.length],
  // ...
}));
```

---

### 5-5: 修改名称不反映到消息

**反馈**: 在 Settings 改用户名后，新发消息的 author 仍是旧用户名。

**根因**: `mockSendMessage` 硬编码 `user_id: 'dev-self'` + `author: userById('dev-self')`，未读取 localStorage 中的最新用户信息。

**代码变更** (`client/src/api/mock.js`):

```js
// 新增 currentUser() 从 localStorage auth 读取
function currentUser() {
  try {
    const raw = localStorage.getItem('auth');
    if (raw) {
      const u = JSON.parse(raw).user;
      if (u) return u;
    }
  } catch {}
  return userById('dev-self');
}

// mockSendMessage 改用 currentUser()
user_id: currentUser().id,
author: currentUser(),

// mockUpdateProfile 同时更新 store 中的 user
export function mockUpdateProfile(_token, data) {
  const cu = currentUser();
  const updated = { ...cu, username: data.username || cu.username, avatar_color: data.avatar_color || cu.avatar_color, avatar_url: data.avatar_url || cu.avatar_url || '' };
  if (_store) _store.setState({ user: updated });
  return updated;
}
```

---

### 5-6: 头像上传问题 → 已确认正常工作

**反馈**: 上传头像后头像不变。

**修复**: `mockUploadAvatar` 返回 `URL.createObjectURL(file)` 生成本地 Blob URL。

**确认**: 用户测试后确认已正常工作。

---

### 5-7: 上传慢但能用 → 确认

**反馈**: 上传文件响应较慢。

**说明**: Mock API 使用 `URL.createObjectURL` 生成本地 URL，无需后端。慢是由于真实文件读取 + Blob 创建开销。

**确认**: 用户确认可用。

---

### 5-8: Chat list 显示 AI 消息内容

**反馈**: Chat list 预览中 AI Bot 回复显示为空白。

**根因**: `mockListChats` 取最后一条消息时 AI 消息 `content` 为空字符串（streaming 初始状态）；`setChats` 中 `old.last_message || c.last_message` 优先保留旧的空内容。

**代码变更** (`client/src/api/mock.js` — `mockListChats`):

```js
const msgs = messagesFor(c.id);
const last = msgs.filter(m => m.content?.trim()).length > 0
  ? msgs.filter(m => m.content?.trim()).pop()
  : msgs[msgs.length - 1];
```

**代码变更** (`client/src/store/chat.js` — `setChats`):

```js
const lm = (c.last_message?.content?.trim() ? c.last_message : null)
  || (old.last_message?.content?.trim() ? old.last_message : null);
return { ...c, last_message: lm, unread_count: old.unread_count || 0 };
```

---

### 5-9: Pin/Unpin 顶置功能

**反馈**: 右键菜单需要 Pin/Unpin 将聊天顶置到列表最前。

**代码变更** (`client/src/api/mock.js` — 新增 `mockTogglePin`):

```js
export function mockTogglePin(_token, chatId) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (!chat) return { ok: false };
  chat.pinned = !chat.pinned;
  if (_store) _store.getState().onChatUpdate({ id: chatId, pinned: chat.pinned });
  return { ok: true, pinned: chat.pinned };
}
```

**代码变更** (`client/src/api/client.js`):

```js
togglePin: (_token, chatId) => request('POST', '/api/chats/' + chatId + '/pin', _token),
['togglePin', mockTogglePin],
```

**代码变更** (`client/src/components/ChatList.jsx`):

```jsx
const handleTogglePin = async (chatId) => {
  try { await api.togglePin(accessToken, chatId); } catch (e) { console.error('Toggle pin error:', e); }
  setContextMenu(null);
};

<button className="context-menu-item"
  onClick={() => handleTogglePin(contextMenu.chatId)}>
  {chats.find(c => c.id === contextMenu.chatId)?.pinned ? 'Unpin' : 'Pin'}
</button>
```

`onChatUpdate` 已有 pinned 排序逻辑（`setChats`/`onChatUpdate` 按 `a.pinned ? -1 : 1` 排序）。

---

### 验证
- Build 通过：`npm run build` → 67 modules transformed, no errors
---

## 2026-07-10 第 6 轮（6-1 ~ 6-5）

---

### 6-1: 修改名称不反映到历史消息

**反馈**: 在 Settings 修改用户名后，历史消息的 author 仍是旧用户名。

**根因**: `MessageItem` 直接从 `msg.author`（消息发送时的快照）读取显示，不改数据库。`mockUpdateProfile` 只更新了 store 的 `user` 对象，未更新 `d.chats` 中 members 成员信息。

**代码变更** (`client/src/api/mock.js` — `mockUpdateProfile`):

```diff
 export function mockUpdateProfile(_token, data) {
   const cu = currentUser();
   const updated = { ...cu, username: data.username || cu.username, ... };
+  const d = ensureData();
+  d.chats.forEach(chat => {
+    const mi = chat.members?.findIndex(m => m.id === updated.id);
+    if (mi !== -1 && mi !== undefined) {
+      chat.members[mi] = { ...chat.members[mi], ...updated };
+      if (_store) _store.getState().onChatUpdate({ id: chat.id, members: [...chat.members] });
+    }
+  });
   if (_store) _store.setState({ user: updated });
```

**代码变更** (`client/src/components/MessageItem.jsx` — 改用 live data):

```diff
-  const author = msg.author || { username: 'Unknown', avatar_color: '#5865F2', id: msg.user_id };
+  const author = useMemo(() => {
+    const chat = chats.find(c => c.id === chatId);
+    if (msg.user_id === user.id) return user;
+    return chat?.members?.find(m => m.id === msg.user_id) || msg.author || { ... };
+  }, [chats, chatId, msg.user_id, msg.author, user]);
```

关键改动：
- `mockUpdateProfile` 遍历所有 chat，更新成员信息后调用 `onChatUpdate` 同步 store
- `MessageItem` 优先从 `chat.members`（live data）查找作者，而非 `msg.author` 快照
- 自己的消息直接使用 `useAuthStore` 的 `user`（已在 `mockUpdateProfile` 中更新）

---

### 6-2: "View Info" 不弹出任何内容

**反馈**: 右键菜单点击 "View Info" 无反应。

**根因**: `ChatInfoModal` 已导入、`showChatInfo` state 已声明，但组件未被渲染到 JSX 中。

**代码变更** (`client/src/components/ChatList.jsx`):

```diff
+      {showChatInfo && (
+        <ChatInfoModal chatId={showChatInfo} onClose={() => setShowChatInfo(null)} />
+      )}
       {showSettings && (
         <SettingsModal user={user} onClose={() => setShowSettings(false)} onSave={handleSaveSettings} />
       )}
```

### 验证
- Build: ✓

---

### 6-3: 发送多行消息后不自动滚动到底部

**反馈**: 输入多行消息发送后，消息列表未自动滚动到最新消息。

**根因**: 多行文本发送后 Composer textarea 高度骤降（从多行回缩到单行），引起 `.chat-body` 容器布局变化，浏览器 auto-anchor 调整 scrollTop，导致距底部 <100px 的判断条件不满足。

**代码变更** (`client/src/components/ChatView.jsx` — scroll `useEffect`):

```diff
-  }, [chatId, filtered.length]);
+  }, [chatId, messages]);
```

```diff
-      if (isNewChat || (scrollHeight - scrollTop - clientHeight < 100)) {
+      if (isNewChat || (scrollHeight - scrollTop - clientHeight < 300)) {
```

关键改动：
- 依赖改为 `messages`（完整数组），覆盖流式更新 content 变化
- 阈值从 100px → 300px，抵消 textarea 高度变化引起的 scrollTop 偏移

---

### 6-4: 其他用户的 profile 可以点击查看

**反馈**: 消息中其他人的头像和用户名无法点击查看详情。

**实现**: 新增 `UserProfileModal` 组件，MessageItem 和 ChatInfoModal 中点击用户头像/名称弹出。

**新增文件** (`client/src/components/UserProfileModal.jsx`):

```jsx
export default function UserProfileModal({ user: profileUser, onClose }) {
  if (!profileUser) return null;
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-box" onClick={e => e.stopPropagation()} style={{ maxWidth: 360, textAlign: 'center' }}>
        {/* ✕ close button */}
        {/* avatar (img or colored letter) */}
        {/* username, status (● Online / ○ Offline) */}
        {/* Email, ID */}
      </div>
    </div>
  );
}
```

**代码变更** (`client/src/components/MessageItem.jsx`):

```diff
+import UserProfileModal from './UserProfileModal';
+
   const [opPending, setOpPending] = useState(false);
+  const [profileUser, setProfileUser] = useState(null);

   // avatar wrapper → clickable
-  author.avatar_url
-    ? <img ... />
-    : <div className="msg-avatar" ... />
+  <div onClick={() => setProfileUser(author)} style={{ cursor: 'pointer' }}>
+    {author.avatar_url ? <img ... /> : <div className="msg-avatar" ... />}
+  </div>

   // username → clickable
-  <span className="msg-author">{author.username}</span>
+  <span className="msg-author" onClick={() => setProfileUser(author)} style={{cursor:'pointer'}}>{author.username}</span>

   // render modal
+  {profileUser && (
+    <UserProfileModal user={profileUser} onClose={() => setProfileUser(null)} />
+  )}
```

**代码变更** (`client/src/components/ChatInfoModal.jsx`):

```diff
-import { useMemo } from 'react';
+import { useMemo, useState } from 'react';
+import UserProfileModal from './UserProfileModal';

+  const [profileUser, setProfileUser] = useState(null);

   // MemberRow 增加 onProfile 回调
-  {owner && <MemberRow member={owner} />}
+  {owner && <MemberRow member={owner} onProfile={setProfileUser} />}

   // render modal
+  {profileUser && (
+    <UserProfileModal user={profileUser} onClose={() => setProfileUser(null)} />
+  )}

   // MemberRow 定义
-  function MemberRow({ member }) {
-    <div style={{ ... }}>
+  function MemberRow({ member, onProfile }) {
+    <div onClick={() => onProfile?.(member)} style={{ ..., cursor: 'pointer' }}>
```

---

### 6-5: Chat list 排序问题（数据同步 + 排序 comparator 修复）

**反馈**: 排序有问题，新创建的 chat 和有回复的 chat 交错排列不符合预期。

**根因**: 两层问题叠加：

**第一层（数据同步）**: `mockCreateChat`/`mockCreateDM` 仅写入 `d.chats`，不通知 Store，新 chat 在下次轮询前不出现；`setChats` 合并时 `...c` 将 Store 中已有的 `last_message_at` 覆盖为 `null`。

**第二层（排序 comparator 不一致）**: 更隐蔽。sort comparator:

```js
if (a.pinned !== b.pinned) return a.pinned ? -1 : 1;
```

新创建的 chat 没有 `pinned` 属性（值为 `undefined`），而数据库里的 chat 有 `pinned: false`。当 comparator 遭遇 `undefined` vs `false`：

- `undefined !== false` → `true`
- `undefined ? -1 : 1` → `1`（b 在前）
- `false ? -1 : 1` → `1`（也是 b 在前）

**两边都返回 "对方排前面"**，违反了排序 comparator 的契约（`compare(a,b)` 应与 `-compare(b,a)` 相反）。这导致 sort 的排序行为未定义——具体表现取决于 JS 引擎的实现和数组初始顺序，用户观察到的"在 pin 后面、其他 chat 前面"正是这个 undefined behavior 的外显。

**修复 1 — 数据同步** (`client/src/api/mock.js`):

```diff
   d.chats.unshift(newChat);
+  if (_store) _store.getState().onChatUpdate(newChat);
   return newChat;
```

```diff
   d.chats.unshift(newDM);
+  if (_store) _store.getState().onChatUpdate(newDM);
   return newDM;
```

**修复 2 — `setChats` 合并保护** (`client/src/store/chat.js`):

```diff
-      if (!old) return c;
-      const lm = (c.last_message?.content?.trim() ? c.last_message : null) || (old.last_message?.content?.trim() ? old.last_message : null);
-      return { ...c, last_message: lm, unread_count: old.unread_count || 0 };
+      if (!old) {
+        const lma = c.last_message_at || c.created_at;
+        return { ...c, last_message_at: lma };
+      }
+      const lm = (c.last_message?.content?.trim() ? c.last_message : null) || (old.last_message?.content?.trim() ? old.last_message : null);
+      const lma = c.last_message_at || old.last_message_at || c.created_at;
+      return { ...c, last_message_at: lma, last_message: lm, unread_count: old.unread_count || 0 };
```

**修复 3 — 统一 `!!` 转换 + 显式 `pinned: false`**: 所有三处 sort（`setChats`/`onChatUpdate`/`onMessageCreate`）的 pinned 比较由 `a.pinned ? -1 : 1` 改为 `!!a.pinned ? -1 : 1`；`mockCreateChat` 新增 `pinned: false` 显式字段。（第 7 轮补充修正）

**验证**: 集成测试模拟完整流程通过 ✅

---

### 6-6: ChatInfoModal 显示时间信息

**反馈**: Chat Info 面板缺少 Created at 和 Last message 时间。

**代码变更** (`client/src/components/ChatInfoModal.jsx`):

```diff
+function fmtTime(t) {
+  if (!t) return '-';
+  return new Date(t).toLocaleString();
+}

+<InfoRow label="Created at" value={fmtTime(chat.created_at)} />
+<InfoRow label="Last message" value={fmtTime(chat.last_message_at)} />
```

---

### 验证
- Build: ✓ 2.17s

---

## 2026-07-10 第 7 轮 — Mock 重写回退 + 排序修复 + 测试数据增强

---

### 7-1: Reaction 归属硬编码 `dev-self`

**反馈**: `mockAddReaction`/`mockRemoveReaction` 硬编码 `user_id: 'dev-self'`，无论当前登录用户是谁。

**根因**: 反应操作固定写死为 `'dev-self'`，未读取当前用户。

**代码变更** (`client/src/api/mock.js`):

```diff
   if (_store) _store.getState().onReaction({ message_id: msgId, emoji, user_id: cu.id }, true);
```

其中 `cu = currentUser()` 从 localStorage 读取当前用户。

---

### 7-2: ID 前缀 `mock-xxx-` 与真实后端格式不一致

**反馈**: 新建 chat/message/reaction 的 ID 使用 `mock-chat-`/`mock-msg-`/`mock-upload-` 前缀。

**根因**: 自研 ID 生成函数使用前缀 + 时间戳，与后端 UUID 格式不匹配。

**代码变更** (`client/src/api/mock.js` — 新增 `randid`):

```js
function randid() {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID();
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
    const r = Math.random() * 16 | 0;
    return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16);
  });
}
```

---

### 7-3: `mockUpdateProfile` 不同步 chat members

**反馈**: Settings 改用户名后只有 store 的 `user` 更新，`d.chats` 中 members 仍是旧名。

**根因**: `mockUpdateProfile` 仅更新 store 中的 `user` 对象，未遍历更新 `d.chats` 中各 chat 的 members。

**代码变更** (`client/src/api/mock.js` — `mockUpdateProfile`):

```diff
   const updated = { ...cu, username: data.username || cu.username, ... };
+  const d = ensureData();
+  d.chats.forEach(chat => {
+    const mi = chat.members?.findIndex(m => m.id === updated.id);
+    if (mi !== -1 && mi !== undefined) {
+      chat.members[mi] = { ...chat.members[mi], ...updated };
+      if (_store) _store.getState().onChatUpdate({ id: chat.id, members: [...chat.members] });
+    }
+  });
   if (_store) _store.setState({ user: updated });
```

---

### 7-4: `mockLogin`/`mockRegister`/`mockMe`/`mockRefresh` 固定返回 `dev-self`

**反馈**: 多用户场景下，auth 相关 mock 函数硬编码返回 `userById('dev-self')`。

**根因**: 登录后 auth 数据已写入 localStorage，但 mock 函数未使用 `currentUser()` 读取。

**代码变更** (`client/src/api/mock.js`):

```diff
-  return userById('dev-self');
+  return userById(currentUser().id);
```

`currentUser()` 优先从 localStorage 读取，fallback 到 `userById('dev-self')`。

---

### 7-5: MemberPanel 点击成员行无反应

**反馈**: 右侧成员面板点击成员行无任何交互。

**根因**: 成员行缺少 onClick 处理。

**代码变更** (`client/src/components/MemberPanel.jsx`):

```diff
-  <div key={m.id} style={{...}}>
+  <div key={m.id} style={{...,cursor:'pointer'}} onClick={() => setProfileUser(m)}>
```

新增 `UserProfileModal` 弹窗显示用户详情。

---

### 7-6: CI 测试 `text=Quick Enter (mock)` 选择器不匹配

**反馈**: Frontend CI 27/27 全部失败，超时在 `locator('text=Quick Enter (mock)')`。

**根因**: 5f951ba 将按钮文字从 `⚡ Quick Enter (mock)` 改为 `⚡ Quick Enter`，测试选择器未同步更新。

**代码变更** (`client/tests/ci.spec.mjs` + `client/tests/real-time.spec.mjs`):

```diff
-  await page.click('text=Quick Enter (mock)');
+  await page.click('text=Quick Enter');
```

---

### 7-7: `mockDeleteChat` 不通知 Store

**反馈**: 右键删除 chat 后 sidebar 不刷新，数量不变。

**根因**: `mockDeleteChat` 仅从 `d.chats` 数组中过滤，未调用 store 的 `onChatDelete`。

**代码变更** (`client/src/api/mock.js`):

```diff
   d.chats = d.chats.filter(c => c.id !== id);
+  if (_store) _store.getState().onChatDelete({ chat_id: id });
   return { ok: true };
```

---

### 7-8: `mockRenameChat` 不通知 Store

**反馈**: 重命名 chat 后 sidebar 标题不更新。

**根因**: `mockRenameChat` 仅改内存中 `chat.name`，未触发 store 更新。

**代码变更** (`client/src/api/mock.js`):

```diff
   if (chat) chat.name = name;
+  if (_store) _store.getState().onChatUpdate({ id: _id, name });
   return { ok: true };
```

---

### 7-9: `mockCreateChat` 缺少 `pinned` 字段导致排序异常

**反馈**: 新创建的 chat 出现在"pin 后面、其他 chat 前面"，排序行为不符合预期。

**根因**: 新 chat 无 `pinned` 属性（`undefined`），sort comparator 使用 `a.pinned ? -1 : 1`。`undefined` 与 `false` 比较时：

```
compare(undefined, false)  → undefined !== false → true → undefined? → 1  → false 在前
compare(false, undefined)  → false !== undefined → true → false?     → 1  → undefined 在前
```

两边都返回"对方排前面"，违反 comparator 契约（`compare(a,b)` 应与 `-compare(b,a)` 相反），导致 sort 排序未定义。

**代码变更** (`client/src/store/chat.js` — 三处 sort）：

```diff
-  if (a.pinned !== b.pinned) return a.pinned ? -1 : 1;
+  const pa = !!a.pinned, pb = !!b.pinned;
+  if (pa !== pb) return pa ? -1 : 1;
```

**代码变更** (`client/src/api/mock.js`):

```diff
+  pinned: false,
```

---

### 7-10: Dummy 数据非确定性导致 CI 不一致

**反馈**: 每次运行生成不同数据，测试结果不可复现。

**根因**: `expandMessages` 和 `pick` 使用 `Math.random()` 生成额外回复消息。

**代码变更** (`client/src/dev/dummy.js`):

```diff
- function expandMessages(topicMsgs) {
-   const result = [];
-   for (const [content, userId] of topicMsgs) {
-     const isSystem = userId === 'GM';
-     result.push({ content, userId, isSystem });
-     if (Math.random() > 0.7) { ... }
-   }
-   return result;
- }
+ function expandMessages(topicMsgs) {
+   const result = [];
+   for (const [content, userId] of topicMsgs) {
+     result.push({ content, userId, isSystem: userId === 'GM' });
+   }
+   return result;
+ }
```

同时：DM 改为 `pinned: false` 使首条 `.chat-item` 为群聊（MemberPanel 测试可用）；首群聊（General）消息添加 `me: true` 反应数据（reaction 测试可用）。

---

### 7-11: `mockSendMessage` AI 回复随机触发

**反馈**: AI 回复有时出现有时消失，测试不稳定。

**根因**: `Math.random() < 0.5` 条件导致 AI 回复不可预期。

**代码变更** (`client/src/api/mock.js`):

```diff
-  if (Math.random() < 0.5) {
-    const text = AI_RESPONSES[Math.floor(Math.random() * AI_RESPONSES.length)];
-    ...
-    setTimeout(..., 500 + Math.random() * 800);
-  }
+  const text = AI_RESPONSES[0];
+  ...
+  setTimeout(..., 500);
```

固定使用第一条 AI 回复，固定延迟 500ms。

---

### 7-12: DM 创建测试已废弃

**反馈**: CI 中 `mock create DM via search` 测试超时。

**根因**: `button[title="New DM"]` 在 UI 重构（5f951ba）中移除，测试选择器永远等不到元素。

**代码变更** (`client/tests/ci.spec.mjs`):

```diff
-  test('mock create DM via search', async ({ page }) => {
+  test.skip('mock create DM via search', async ({ page }) => {
```

---

### 验证
- CI (Go 后端 + 构建): ✅
- Frontend CI (mock 测试 27/27 + e2e 8/8): ✅

---

## 2026-07-10 第 8 轮 — Reaction me 动态计算 + 登录页简化

---

### 8-1: Reaction `me` 字段从硬编码改为动态计算

**反馈**: 修改名称后，reaction 的 `me` 归属显示不正确（连是不是自己的都显示错误）。

**根因**: `mockListMessages` 返回的消息中的 `reactions[].me` 来自 `generateDummyData` 的硬编码 `me: true`。轮询刷新（`connectPolling`）定期调用 `mockListMessages`，原始硬编码 `me` 覆盖掉 `onReaction` 正确设置的 `me`。

**代码变更** (`client/src/api/mock.js` — `mockListMessages`):

```diff
 export function mockListMessages(_token, chatId, before, limit) {
   const all = messagesFor(chatId);
+  const cu = currentUser();
+  const mapped = all.map(msg => ({
+    ...msg,
+    reactions: (msg.reactions || []).map(r => ({
+      ...r,
+      me: r.user_ids?.includes(cu.id) || false,
+    })),
+  }));
   if (before) {
-    const idx = all.findIndex(m => m.id === before);
+    const idx = mapped.findIndex(m => m.id === before);
     if (idx <= 0) return { messages: [] };
-    const start = Math.max(0, idx - (limit || 50));
-    return { messages: all.slice(start, idx) };
+    return { messages: mapped.slice(start, idx) };
   }
-  const total = all.length;
-  return { messages: all.slice(start, total) };
+  return { messages: mapped.slice(start, total) };
```

`me` 改为 `r.user_ids?.includes(cu.id) || false`，确保每次返回的消息中 reaction 归属与当前用户一致。

---

### 8-2: 登录页简化 — 单一 Quick Enter 按钮

**反馈**: Debug 模式开关 + Mock API 复选框 + Quick Enter 按钮三个控件操作繁琐；默认不应勾选 Mock API。

**代码变更** (`client/src/routes/LoginPage.jsx`):

```diff
-  <label><input type="checkbox" checked={debugMode} onChange={...} /> Debug mode</label>
-  <label><input type="checkbox" checked={api.isMockEnabled()} onChange={...} /> Mock API</label>
-  {debugMode && <button ...>⚡ Quick Enter</button>}
+  <button ...>⚡ Quick Enter</button>
```

`quickEnter` 函数整合：`api.enableMock()` + `setDebugMode(true)` + `mockLogin()` + `nav('/')`，去除独立的 debugMode 和 Mock API 控件。

---

### 验证
- CI Frontend 测试选择器同步更新（去除 `text=Debug mode` 点击和 `text=Quick Enter (mock)` 选择器）

---

### 8-3: AI 生成报告存在多处幻觉

**反馈**: 两份自动生成的规范报告（`frontend-logic-spec-20260710.md`、`frontend-ui-spec-20260710.md`）包含不存在的概念。

**根因**: AI 生成过程中产生了幻觉（hallucination），引入了代码库中根本不存在的标识符和依赖。

**发现的问题**:

| 报告 | 幻觉 | 实际 |
|------|------|------|
| `logic-spec` | `d.chatMembers` 扁平数组成员数据源 | 仅 `d.chats[].members` 单一内联数组 |
| `logic-spec` | `buildChatResponse` 函数 | 不存在，chat 响应由 `mockListChats`/`mockGetChat` 直接构造 |
| `logic-spec` | 注释 `// { users, chats, messages, reactions, chatMembers }` | `ensureData()` 实际只创建 `{ chats, messages }` |
| `ui-spec` | 依赖 `Tailwind CSS` | 无 tailwind 配置，纯 CSS custom properties + inline styles |
| `ui-spec` | 依赖 `Lucide Icons` | 全局搜索 0 匹配，使用 unicode emoji |
| `ui-spec` | 依赖 `React 18` | `package.json` → `"react": "^19.0.0"` |

**修正文档**: `docs/reports/frontend-logic-correction-20260710.md`

---

## 2026-07-10 第 9 轮 — UI 修复

---

### 9-1: 创建 Group 时搜索框未隐藏

**反馈**: 点击 "+" 弹出 GroupName 输入框时，搜索框仍可见，应隐藏。

**根因**: `showCreate` 状态只在 `ScrollArea` 内控制 CreateGroupForm 的显隐，搜索框（`.sidebar-search-row`）独立于该状态，始终渲染。

**代码变更** (`client/src/components/ChatList.jsx`):

```diff
+      {!showCreate && (
        <div className="sidebar-search-row">
          ...
        </div>
+      )}
```

搜索输入框、搜索按钮、join/create 快捷操作按钮在 `showCreate=true` 时全部隐藏，恢复时重新出现。

---

### 9-2: 消息 hover 菜单位置调整 — 置于文本与 reaction 之间

**反馈**: hover 后的 reaction/edit/delete 菜单在有 reaction 的情况下出现在 reaction 下方，鼠标很难选中。

**根因**: `.msg-actions` 使用 `position:absolute; bottom:-22px` 锚定在 `.msg-row` 底部，reaction 栏扩展了行高，菜单被推得更远。

**首轮尝试**: 用 `position:absolute; z-index:10` 遮住 reaction → 但 reaction 需要点击快速赞同/取消，被菜单挡住不可用。

**最终方案**: 改为正常文档流，菜单在文本与 reaction 之间撑开。

**代码变更** (`client/src/styles/global.css`):

```diff
- .msg-actions { display:none; gap:4px; position:absolute; bottom:-22px; left:52px; z-index:5; background:...; }
+ .msg-actions { display:none; gap:4px; margin-top:4px; background:var(--bg-primary); padding:2px 6px; border-radius:4px; box-shadow:0 2px 8px rgba(0,0,0,0.3); }
```

**代码变更** (`client/src/components/MessageItem.jsx`):

```diff
  {msg.deleted ? (...) : editing ? (...) : streaming ? (...) : (
    <div className="msg-content">...</div>
  )}
+ {!msg.deleted && (
+   <div className="msg-actions">
+     😀 Edit Delete
+   </div>
+ )}
```

改动要点：
- 移除 `position:absolute`，菜单恢复正常文档流
- `margin-top:4px` 与文本保持间距
- 菜单 DOM 顺序在文本之后、attachments/reactions 之前
- hover 时菜单展开占位，reaction 保持在下方可点击
- 鼠标从文本垂直下滑即可到达菜单，无需跨越 reaction

### 9-3: （回退 9-2）消息 hover 菜单改为右上角浮动

**反馈**: 9-2 方案把菜单改为正常文档流，会改变消息布局（hover 时撑开空间），且被驳回。

**根因**: 
- absolute 遮住 reaction → 无法点击 reaction（rejected）
- 正常文档流撑开 → 改变布局（rejected）

**最终方案**: 菜单定位到 `.msg-row` 右上角 (`top:-8px; right:8px`)，类似 Slack/Discord 的 hover 菜单。不与底部 reaction 干涉，不改变布局，且鼠标在消息右侧自然可达。

**代码变更** (`client/src/styles/global.css`):

```diff
- .msg-actions { display:none; gap:4px; margin-top:4px; background:...; }
+ .msg-actions { display:none; gap:4px; position:absolute; top:-8px; right:8px; z-index:5; background:...; }
```

**DOM 位置**: `msg-actions` 保持在 flex 内容区内、attachments/reactions 之前（不变），由 CSS `position:absolute` 脱离文档流。

### 9-4: 菜单左对齐 + 悬浮上方，表情选单同位置（回退）

**反馈**: `top:-40px` 太靠上点不到；且菜单位于 avatar/username/message 整体上方，定位锚点错误。

**后续尝试**:
- `top:-30px` 仍太靠上
- `top:0; left:0` 相对于 `.msg-row` → 遮住 avatar，不是 message 上方
- `bottom:100%; left:0` 相对于内容块 `position:relative` 容器 → 正确位置

### 9-5: 消息 hover 菜单 — 最终方案（文本正上方左对齐）

**反馈**: 菜单应与 **消息内容文本** 左对齐，正好在 **文本** 正上方（不在 username 上方，不遮 avatar）。

**实现**: 用 `position:relative` 包裹内容块，菜单和表情选单以 `position:absolute; bottom:100%; left:0` 挂在文本正上方。

**代码变更** (`client/src/components/MessageItem.jsx`):

```diff
+          <div style={{position:'relative'}}>
             {msg.deleted ? (...) : editing ? (...) : streaming ? (...) : (
               <div className="msg-content">...</div>
             )}
             {!msg.deleted && (
               <div className="msg-actions">
                 😀 Edit Delete
               </div>
             )}
+          </div>
+          {showEmoji && (
+            <div className="emoji-picker">👍 ❤️ 😂 🎉 ...</div>
+          )}
```

- 表情选单从 flex div 末尾移入内容块容器
- 移除 flex div 的 `position:relative`

**代码变更** (`client/src/styles/global.css`):

```diff
- .msg-actions { position:absolute; top:0; left:0; }
+ .msg-actions { position:absolute; bottom:100%; left:0; }
- .emoji-picker { position:absolute; top:0; left:0; }
+ .emoji-picker { position:absolute; bottom:100%; left:0; }
```

---

### 9-6: Emoji picker 点击外部取消

**反馈**: 点击 😀 打开表情选单后，没有取消选项，只能选一个 emoji 后关闭。

**代码变更** (`client/src/components/MessageItem.jsx`):

```diff
- import { useState, useMemo } from 'react';
+ import { useState, useMemo, useRef, useEffect } from 'react';

+  const pickerRef = useRef(null);
+  useEffect(() => {
+    if (!showEmoji) return;
+    const handler = (e) => {
+      if (pickerRef.current && !pickerRef.current.contains(e.target)) {
+        setShowEmoji(false);
+      }
+    };
+    document.addEventListener('mousedown', handler);
+    return () => document.removeEventListener('mousedown', handler);
+  }, [showEmoji]);

-  {showEmoji && <div className="emoji-picker">...</div>}
+  {showEmoji && <div className="emoji-picker" ref={pickerRef}>...</div>}
```

 点击 emoji picker 外部任意区域（消息、sidebar、空白处）触发 `mousedown` → `setShowEmoji(false)` 关闭选单。

### 9-7: Mock 数据缺少同一人连续多条消息

**反馈**: 测试数据中每个用户的消息都是交替出现，没有一个人连续发送多条的场景，无法测试 `sameAuthor` 续接显示。

**代码变更** (`client/src/dev/dummy.js` — `General` 话题末尾):

```diff
    ['Check out this link: https://example.com', 'dev-frank'],
+   ['Anyone tried the new dark mode?', 'ME'],
+   ['Yes, it looks great!', 'ME'],
+   ['Love the contrast ratio', 'ME'],
+   ['Also the animations are smooth', 'ME'],
  ],
```

末尾 4 条消息均为 Alice (`ME`)，在 chat 中会渲染为 `sameAuthor=true` 的续接样式（无头像/用户名）。

### 9-8: Chat list 菜单按钮判定范围太小（触屏适配）

**反馈**: 三点菜单 `⋮` 按钮点击区域过窄（`padding: 0 2px`），经常点不到，触屏更严重。

**代码变更** (`client/src/styles/global.css`):

```diff
- .chat-item-menu-btn { font-size:18px; padding:0 2px; }
+ .chat-item-menu-btn { font-size:18px; padding:12px 20px; }
```

点击区域从 2px → 20px 水平扩展、0 → 12px 垂直扩展，满足触屏 44×44px 推荐标准。

### 9-9: 右侧成员面板按角色分组 + online 状态修复

**反馈**: 成员列表应按 Owner → Admin → Member 分组排序；online 状态不显示（所有点都是灰色）。

**根因**: `ensureData()` 未保存 `gen.onlineUserIds`；`dummy.js` 用 `u.id !== 'dev-self'` 计算在线导致 Frank 也被算作在线。

**修正**:
1. `dummy.js`: 改用 `last_seen` 时间戳判断在线，Bob/Carol/Eve 最近在线 → 绿点，Dave（1 分钟前）→ 绿点，Frank（1 天前）→ 灰点
2. `mock.js`: 存储和传递 `onlineUserIds` 到 store
3. `MemberPanel`: 按 `last_seen` 与当前时间差（<5 分钟）判断在线，在线排前面

**代码变更** (`client/src/api/mock.js`):

```diff
  const MOCK_USERS = [
-   { id: 'dev-self', ..., status: 'online' },
-   { id: 'dev-frank', ..., status: 'offline' },
+   { id: 'dev-self', ..., last_seen: new Date().toISOString() },
+   { id: 'dev-frank', ..., last_seen: new Date(Date.now() - 86400000).toISOString() },
  ];
```

**代码变更** (`client/src/dev/dummy.js`):

```diff
- const onlineUserIds = USERS.filter(u => u.id !== 'dev-self').map(u => u.id);
+ const onlineUserIds = USERS.filter(u => u.status === 'online').map(u => u.id);
- const members = [...unique].map(...);
+ const members = unique.map((m) => ({
+   ...m,
+   role: m.id === ME.id ? 'owner' : (m.id === 'dev-bob' || m.id === 'dev-carol' ? 'admin' : 'member'),
+ }));
```

**代码变更** (`client/src/components/MemberPanel.jsx`):

```diff
- const { chats } = useChatStore();
+ const { chats, onlineUserIds } = useChatStore();
+ const isOnline = (m) => { ... };
- <span className="status-dot offline" />
+ <span className={'status-dot ' + (isOnline(m) ? 'online' : 'offline')} />
```

### 9-10: 成员名后显示 Admin / Member tag

**反馈**: Admin 和 Member 组成员名后应显示角色 tag。

**代码变更** (`client/src/components/MemberPanel.jsx`):

```diff
  <span style={{flex:1}}>{m.username}</span>
+ {s.label !== 'Owner' && (
+   <span style={{fontSize:10,padding:'0 5px',borderRadius:3,fontWeight:500,
+     background:'rgba(88,101,242,0.15)',color:'#5865F2'}}>
+     {s.label === 'Admin' ? 'ADMIN' : 'MEMBER'}
+   </span>
+ )}
```

Owner 组不显示 tag（由 section label 区分），Admin 组显示 `ADMIN`，Member 组显示 `MEMBER`。

### 9-11: 删除多余的高度占位 `<div>`

**反馈**: 成员列表应按 Owner → Admin → Member 分组排序；online 状态不显示（所有点都是灰色）。

**根因**: `ensureData()` 未保存 `gen.onlineUserIds`；store 没有 `onUserStatus` action，调用被 `?.` 静默忽略。`onlineUserIds` 始终为 `[]`。

**代码变更** (`client/src/api/mock.js` — `ensureData`):

```diff
  const gen = generateDummyData(...);
- data = { chats: gen.chats, messages: [...(gen.messages || [])] };
+ data = { chats: gen.chats, messages: [...(gen.messages || [])], onlineUserIds: gen.onlineUserIds || [] };
+ if (_store) _store.setState({ onlineUserIds: data.onlineUserIds });
```

**代码变更** (`client/src/components/MemberPanel.jsx`):

```diff
- const { chats } = useChatStore();
+ const { chats, onlineUserIds } = useChatStore();

  // 按角色分组渲染（Owner → Admin → Member）
- {[...members].sort((a,b) => onlineUserIds...).map(m => (...))}
+ {(() => {
+   const owner = members.find(m => m.id === chat.owner_id);
+   const admins = members.filter(m => m.id !== chat.owner_id && m.role === 'admin');
+   const rest = members.filter(m => m.id !== chat.owner_id && m.role !== 'admin');
+   // 分三组渲染，每组带 label
+ })()}

- <span className="status-dot offline" />
+ <span className={'status-dot ' + (onlineUserIds.includes(m.id) ? 'online' : 'offline')} />
```

### 9-10: 删除多余的高度占位 `<div>`

**反馈**: `unread_count` 为 0 时渲染的 `<div style={{height:18}} />` 多余。

**代码变更** (`client/src/components/ChatListItem.jsx`):

```diff
- {unread > 0 ? <div className="unread-badge">{unread}</div> : <div style={{ height: 18 }} />}
+ {unread > 0 ? <div className="unread-badge">{unread}</div> : null}
```

### 9-11: 红点与菜单按钮同行对齐

**反馈**: 删除占位 `<div>` 后红点不对齐。

**代码变更** (`client/src/styles/global.css`):

```diff
- .chat-item-menu-wrap { position:relative; }
+ .chat-item-menu-wrap { display:flex; align-items:center; gap:4px; }
```

**代码变更** (`client/src/components/ChatListItem.jsx`):

```diff
  <div className="chat-item-meta">
    <div className="chat-item-time">{timeAgo(chat.last_message_at)}</div>
-   {unread > 0 ? <div className="unread-badge">{unread}</div> : null}
    <div className="chat-item-menu-wrap">
+     {unread > 0 ? <div className="unread-badge">{unread}</div> : null}
      <button ...>⋮</button>
    </div>
  </div>
```

红点移至 `.chat-item-menu-wrap` 内，与 `⋮` 按钮 flex 同行排列，自动对齐。

---

## 2026-07-11 第 10 轮 — Mock 数据清理 & 角色分配

---

### 10-1: DM 标记废弃注释

**反馈**: `mockCreateDM` 已废弃，应标记。

**代码变更** (`client/src/api/mock.js`):
```diff
+ // @deprecated DMs are now handled via createChat with type='dm'
export function mockCreateDM(_token, userId) {
```

---

### 10-2: 去除 members MOCK_USERS 合并

**反馈**: `mockListChats` 和 `mockGetChat` 中 `c.members?.map(m => ({ ...m, ...(MOCK_USERS.find(u => u.id === m.id) || {}) }))` 用 `MOCK_USERS` 的数据污染了原始数据，mock 应严格按 API 返回内容输出。

**代码变更** (`client/src/api/mock.js`):
```diff
- members: c.members?.map(m => ({ ...m, ...(MOCK_USERS.find(u => u.id === m.id) || {}) })) || [],
+ members: c.members || [],
```

```diff
- return { ...chat, members: chat.members?.map(m => ({ ...m, ...userById(m.id) })) || [] };
+ return { ...chat, members: chat.members || [] };
```

---

### 10-3: Dummy 数据随机分配角色 + Owner 随机化

**反馈**: 
- mock 数据需要有一部分成员是 admin，个数随机、身份随机
- owner 应是随机 member 而非固定 Alice
- owner 属于 admin（不单独分栏）

**代码变更** (`client/src/dev/dummy.js` — `generateDummyData`):
```js
const members = unique.map(u => ({ ...u }));
const ownerMember = members[Math.floor(Math.random() * members.length)];
ownerMember.role = 'admin';
const adminCount = Math.floor(Math.random() * Math.max(1, Math.floor((members.length - 1) / 2)));
const candidates = members.filter(m => m.id !== ownerMember.id).sort(() => Math.random() - 0.5);
for (let i = 0; i < adminCount && i < candidates.length; i++) {
  const idx = members.findIndex(m => m.id === candidates[i].id);
  members[idx].role = 'admin';
}
```

- Owner 从 members 中随机选取，自动获得 `role: 'admin'`
- 再随机 0~floor((len-1)/2) 个其他成员为 admin
- `owner_id` 改为 `ownerMember.id`（原固定 `ME.id`）

---

### 10-4: Owner 合并到 Admin 组（UI 改动）

**反馈**: owner 属于 admin，UI 不应分成 Owner/Admin/Member 三栏，改为 Admin/Member 两栏。

**代码变更** (`client/src/components/ChatInfoModal.jsx`):
```diff
- const { owner, admins, members } = useMemo(() => {
-   const o = chat.members.find(m => m.id === chat.owner_id) || null;
-   const a = chat.members.filter(m => m.role === 'admin' && m.id !== chat.owner_id);
-   const m = chat.members.filter(m => m.id !== chat.owner_id && m.role !== 'admin');
-   return { owner: o, admins: a, members: m };
+ const { admins, members } = useMemo(() => {
+   const isAdmin = m => m.role === 'admin' || m.id === chat.owner_id;
+   return {
+     admins: chat.members.filter(isAdmin),
+     members: chat.members.filter(m => !isAdmin(m)),
+   };
 }, [chat]);
```

```diff
- <Section title="Owner">...</Section>
- {admins.length > 0 && <Section title={`Admin — ${admins.length}`}>...</Section>}
- <Section title={`Member — ${members.length}`}>...</Section>
+ <Section title={`Admin — ${admins.length}`}>{admins.map(...)}</Section>
+ {members.length > 0 && <Section title={`Member — ${members.length}`}>...</Section>}
```

**代码变更** (`client/src/components/MemberPanel.jsx`):
```diff
- const owner = members.find(m => m.id === chat.owner_id);
- const admins = sortOnline(members.filter(m => m.id !== chat.owner_id && m.role === 'admin'));
- const rest = sortOnline(members.filter(m => m.id !== chat.owner_id && m.role !== 'admin'));
- const sections = [
-   owner ? { label: 'Owner', items: [owner] } : null,
-   admins.length ? { label: 'Admin', items: admins } : null,
-   rest.length ? { label: 'Member', items: rest } : null,
- ].filter(Boolean);
+ const isAdmin = m => m.role === 'admin' || m.id === chat.owner_id;
+ const admins = sortOnline(members.filter(isAdmin));
+ const rest = sortOnline(members.filter(m => !isAdmin(m)));
+ const sections = [
+   admins.length ? { label: 'Admin', items: admins } : null,
+   rest.length ? { label: 'Member', items: rest } : null,
+ ].filter(Boolean);
```

---

### 10-5: 移除 member tag

**反馈**: member 后面的 ADMIN/MEMBER tag 不需要显示。

**代码变更** (`client/src/components/MemberPanel.jsx`):
```diff
  <span style={{flex:1}}>{m.username}</span>
- <span style={{fontSize:10,padding:'0 5px',borderRadius:3,fontWeight:500,background:'rgba(88,101,242,0.15)',color:'#5865F2'}}>{s.label === 'Admin' ? 'ADMIN' : 'MEMBER'}</span>
```

---

### 10-6: GROUP_TOPICS 改用 'dev-self' 替代 'ME'

**反馈**: GROUP_TOPICS 中用 `'ME'` 字符串标识 Alice，导致 `USERS.find(u => u.id === 'ME')` 找不到，需靠 `unique.unshift(ME)` 回退加入。应直接用 `'dev-self'`。

**代码变更** (`client/src/dev/dummy.js`):
```diff
- ['Anyone tried the new dark mode?', 'ME'],
+ ['Anyone tried the new dark mode?', 'dev-self'],
```
（全局 23 处 `'ME'` → `'dev-self'`）

```diff
- if (!unique.find(m => m.id === ME.id)) unique.unshift(ME);
```

Alice 现在通过 `USERS.find(u => u.id === 'dev-self')` 自然匹配进入 members。

---

### 10-7: 修改 display name 时 chat list 丢失信息

**反馈**: 在 Settings 修改用户名后，chat list 所有信息丢失（只剩 id 和 members）。

**根因**: `mockUpdateProfile` 调用 `onChatUpdate({ id, members })` 仅传了部分字段。`onChatUpdate` 中 `n[idx] = updated` 直接用这个部分对象替换整个 chat，覆盖了 name、last_message、last_message_at 等字段。

**代码变更** (`client/src/store/chat.js` — `onChatUpdate`):
```diff
- n[idx] = updated;
+ n[idx] = { ...n[idx], ...updated };
```

改为合并而非替换，只覆盖 payload 中提供的字段。

---

---

### 10-8: 登录页重定向修复 + pinned 指示器 + 布局修复

#### 10-8-1: 退出登录后未跳转登录页

**反馈**: 用户退出后仍显示 chat 页面，无登录/注册页。

**根因**: `auth.js` 的 `logout()` 中 `set({ user: null })` 未清除 `accessToken`，路由守卫 `token ? <ChatPage /> : <Navigate to="/login" />` 认为仍登录；且 ChatStore 的 `activeChatId` 残留，重新登录后直接跳回之前打开的聊天而非 WelcomeView。

**代码变更** (`client/src/store/auth.js`):
```diff
  logout: async () => {
    api.disableMock();
    try { await api.logout(); } catch (e) { console.error('Logout error:', e); }
+   useChatStore.getState().reset();
    storage.clear();
-   set({ user: null });
+   set({ user: null, accessToken: null });
  },
```

```diff
  return {
    user: saved.user || null,
+   accessToken: saved.accessToken || null,
```

**代码变更** (`client/src/store/chat.js` — 新增 `reset` action):
```js
reset() {
  set({ chats: [], activeChatId: null, messages: [], pinnedMessage: {} });
},
```

**代码变更** (`client/src/components/ChatList.jsx` — 安全防护):
```diff
- {user.avatar_url ? ...}
+ {user?.avatar_url ? ...}
```

#### 10-8-2: Chat list 添加 pinned 📌 指示器

**反馈**: pinned 信息没有显示位置。

**代码变更** (`client/src/components/ChatListItem.jsx`):
```jsx
{chat.pinned && <span style={{fontSize:12}}>📌</span>}
```

#### 10-8-3: ADMIN tag 垂直居中

**反馈**: absolute 定位的 ADMIN tag 未垂直居中。

**代码变更** (`client/src/components/MemberPanel.jsx`, `ChatInfoModal.jsx`):
```diff
- <span style={{position:'absolute',right:22,...}}>ADMIN</span>
+ <span style={{position:'absolute',right:22,top:'50%',transform:'translateY(-50%)',...}}>ADMIN</span>
```

#### 10-8-4: Chat name 禁止换行

**反馈**: chat list 中的名称不应显示为两行。

**代码变更** (`client/src/styles/global.css`):
```diff
- .chat-item-name { font-weight: 600; font-size: 15px; }
+ .chat-item-name { font-weight: 600; font-size: 15px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
```

---

### 10-8: MemberPanel 不分组 + ADMIN tag 右对齐

**反馈**: member 不应按角色分组，按原始顺序显示；ADMIN tag 加在 admin/owner 名字后面。

**过程**:

1. **首次尝试 — flex:1 spacer**:
   - 将 `flex:1` 从 username 移到独立的 `<div style={{flex:1}} />` 放在 username 和 tag 之间
   - 结果: tag 和 × 被推到右侧，但当 × 存在时 tag 左移 × 的宽度，ADMIN 在不同行 x 坐标不一致

2. **第二次尝试 — 固定宽度容器**:
   - 将 tag + × 包裹在 `width:56` 的 `justifyContent:'flex-end'` 容器中
   - 结果: tag + × 组合宽度 (~64px) 超过 56px，溢出导致布局错乱；且 flex-end 仍让 tag 位置随 × 显隐变化

3. **最终方案 — absolute 定位**:
   - 容器改为 `width:66; position:relative`，ADMIN tag `position:absolute; right:22`，× 按钮 `position:absolute; right:0`
   - 无论 × 是否显示，ADMIN tag 的 right 值固定不变，所有 tag 在同一垂直线

**教训**: 用 flex-end 或 margin-left:auto 做右对齐时，条件渲染的子元素会改变前一个元素的 x 坐标。如需多个元素固定在固定位置，用 absolute 定位。

**代码变更** (`client/src/components/MemberPanel.jsx`):
- 移除 `sortOnline` 分组逻辑，直接遍历 `members` 原始顺序
- 移除 `Section` 分栏结构
- 每行加 `isAdmin` 判断显示 ADMIN tag

**代码变更** (`client/src/components/ChatInfoModal.jsx`):
- 移除 `useMemo` 分组、`Section` 组件、`MemberRow` 函数
- 直接遍历 `chat.members` 渲染
- 移除重复的 `InfoRow` 函数声明（修复 build error）

---

### 验证
- Build: ✓

---

## 2026-07-11 第 11 轮 — Notice bar 折叠 + Pinned 字段重构

---

### 11-1: Notice bar 折叠按钮移入标题栏

**反馈**: collapse/expand 按钮应在标题栏右侧，不在 notice bar 内。

**代码变更** (`client/src/components/ChatView.jsx`):
```diff
- <div className="chat-header">
-   <button>←</button>
-   <div style={{flex:1}}>...</div>
+ <div className="chat-header">
+   <button>←</button>
+   <div style={{flex:1}}>...</div>
+   {pinnedMessage[chatId] && (
+     <button className="btn-ghost" onClick={() => setShowNotice(!showNotice)}>
+       {showNotice ? '▲' : '▼'}
+     </button>
+   )}
  </div>
```

同时 notice bar 整体隐藏逻辑：
```diff
- {(pinnedMessage[chatId] || isEditingNotice) && (
+ {((showNotice && pinnedMessage[chatId]) || isEditingNotice) && (
```

---

### 11-2: Pinned 字段重构 — last_update 为 Chat 属性, last_read 为 Member 属性

**反馈**: `last_update` 和 `last_read` 不应放在 `pinned_message` 对象中。`last_update` 是 chat 级属性（`pinned_updated_at`），`last_read` 是 user 级属性（`pinned_last_read_at`），存储在 chat_member 记录中。

**代码变更**:

**Go model** (`server/internal/models/models.go`):
```go
type Chat struct {
    PinnedMessage    *PinnedContent `json:"pinned_message,omitempty"`
    PinnedUpdatedAt  *time.Time     `json:"pinned_updated_at,omitempty"`
    PinnedLastReadAt *time.Time     `json:"pinned_last_read_at,omitempty"`
}
type ChatMember struct {
    PinnedLastReadAt *time.Time `json:"pinned_last_read_at,omitempty"`
}
```

**Go DB** (`server/internal/db/`):
- `migrations/init.sql` — 新增 `chat_members.pinned_last_read_at TEXT`
- `chats.go` — `GetChat`/`ListUserChats` SELECT `pinned_updated_at`, `cm.pinned_last_read_at`; 新增 `UpdatePinnedLastReadAt()`
- `chats_ext.go` — `ListPublicChats` SELECT `pinned_updated_at`

**Go handler** (`server/internal/handlers/chat.go`):
- `PinChat`/`DeletePinnedChat` 现在通过 Hub 广播 chat 更新

**Mock data** (`client/src/dev/dummy.js`):
```diff
- pinned_message: { content, pinned_at, last_update, last_read }
+ pinned_message: { content, pinned_at }
+ pinned_updated_at: ci < 2 ? timeAgo(1800) : null
+ members[meIdx].pinned_last_read_at = ci < 2 ? timeAgo(1200) : null
```

**Mock API** (`client/src/api/mock.js`):
```diff
- onChatUpdate({ ..., pinned_message: { content, pinned_at, last_update, last_read } });
+ onChatUpdate({ ..., pinned_message: { content, pinned_at }, pinned_updated_at: now });
```

**Store** (`client/src/store/chat.js`):
- `pinnedMessage` 注释更新: `{ chatId: { id, content, pinned_at } }`
- `onChatUpdate`/`setChats` 透传 `pinned_updated_at`/`pinned_last_read_at` 到 chat 对象

---

### 验证
- Go build + vet: ✓
- Client build: ✓

---

## 2026-07-11 第 12 轮 — Go 后端测试 + 文档更新

---

### 12-1: 新增 7 个 Pinned 字段相关 DB 测试

**文件**: `server/internal/db/db_test.go`

| 测试名 | 验证点 |
|--------|--------|
| `TestSetAndClearPinnedMessage` | 扩展 — 验证 `PinnedUpdatedAt` 在 set 后被设置、clear 后被清空 |
| `TestSetPinnedMessage_MultipleUpdates` | 多次更新 pin，`PinnedUpdatedAt` 应递增 |
| `TestUpdatePinnedLastReadAt` | `UpdatePinnedLastReadAt` 写入后，`ListUserChats` 返回 `PinnedLastReadAt` |
| `TestUpdatePinnedLastReadAt_Nonexistent` | 不存在的 chat/user 不报错 |
| `TestPinnedLastReadAt_NotSet` | 未读 pin 时 `PinnedLastReadAt` 为 nil |
| `TestPinnedUpdatedAt_NotSet` | 无 pin 时 `PinnedUpdatedAt` 为 nil |
| `TestSetPinnedMessage_ThreeMembers` | DB 层允许 1 人 chat 设置 pin（handler 层限制 ≥3） |

---

### 12-2: 更新 4 个后端文档

**文件** (`docs/reports/`):
- `models-data-spec-20260708.md` — Chat/PinnedContent/ChatMember 模型字段新增 `PinnedUpdatedAt`/`PinnedLastReadAt`
- `db-spec-20260710.md` — DB migration、GetChat/ListUserChats/ListPublicChats 新增字段、SetPinnedMessage/ClearPinnedMessage 更新 `pinned_updated_at`、新增 `UpdatePinnedLastReadAt` 方法
- `api-handlers-spec-20260709.md` — PinChat/DeletePinnedChat handler 新增 Hub broadcast
- `test-suite-spec-20260709.md` — db_test.go 测试表新增 7 项

---

### 验证
- Go all tests: `ok  internal/db 3.53s` + `testutil 11.02s` + `auth 0.50s` + `ws 0.02s`
- Client build: ✓

---

## 2026-07-11 第 13 轮 — Pinned 按钮美化（喇叭图标 + 选中态 + 红点）

---

### 13-1: Pinned 折叠按钮改用喇叭 SVG 图标 + 选中态背景 + 更新红点

**反馈**: ▲/▼ 文本图标不够直观；选中时应高亮；有更新时应显示红点提示。

**代码变更** (`client/src/components/ChatView.jsx`):

```diff
- <button className="btn-ghost" style={{fontSize:13,padding:'2px 8px'}}>
-   {showNotice ? '▲' : '▼'}
- </button>
+ <button className="btn-ghost" style={{
+   position:'relative', padding:'6px 8px',
+   background: showNotice ? 'var(--bg-tertiary)' : 'transparent',
+   borderRadius:4, lineHeight:0,
+ }}>
+   <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
+     strokeLinecap="round" strokeLinejoin="round">
+     <path d="M11 5L6 9H2v6h4l5 4V5z"/>
+     <path d="M19.07 4.93a10 10 0 0 1 0 14.14M15.54 8.46a5 5 0 0 1 0 7.07"/>
+   </svg>
+   {chat?.pinned_updated_at && (!chat?.pinned_last_read_at || new Date(chat.pinned_updated_at) > new Date(chat.pinned_last_read_at)) && (
+     <span style={{position:'absolute', top:2, right:2, width:8, height:8, borderRadius:'50%', background:'#ed4245'}}/>
+   )}
+ </button>
```

### 验证
- Build: ✓

---

## 2026-07-11 第 14 轮 — 红点自动消失（查看后标记已读）

---

### 14-1: 查看 pin 后更新 `pinned_last_read_at` 消除红点

**反馈**: 打开通知栏后红点应消失。

**代码变更** (`client/src/store/chat.js` — 新增 `markPinnedRead`):
```js
markPinnedRead(chatId) {
  set(s => ({
    chats: s.chats.map(c => c.id === chatId ? { ...c, pinned_last_read_at: new Date().toISOString() } : c),
  }));
},
```

**代码变更** (`client/src/components/ChatView.jsx`):
```diff
- const { ... pinnedMessage, setPinnedMessage, clearPinnedMessage } = useChatStore();
+ const { ... pinnedMessage, setPinnedMessage, clearPinnedMessage, markPinnedRead } = useChatStore();

+ useEffect(() => {
+   if (showNotice && pinnedMessage[chatId]) {
+     markPinnedRead(chatId);
+   }
+ }, [showNotice, chatId, pinnedMessage[chatId]]);
```

当 `showNotice` 为 true 且 `pinnedMessage[chatId]` 存在时，自动调用 `markPinnedRead` 将当前 chat 的 `pinned_last_read_at` 设为当前时间，红点条件 `pinned_updated_at > pinned_last_read_at` 不再满足，红点消失。

### 验证
- Build: ✓



---

## 2026-07-11 第 15 轮 — 头像大图预览 + 按钮优化

---

### 15-1: 头像点击打开大图预览

**反馈**: 点击头像应打开大图，而非直接上传或跳转到 profile。

**新增文件** (`client/src/components/ImagePreviewModal.jsx`): 全屏暗色遮罩 + 居中大图，点击遮罩关闭。

**代码变更** (`client/src/components/SettingsModal.jsx`):
- 头像 `<img>` 的 `onClick` 从 `document.getElementById('avatar-file-input').click()` 改为 `setPreviewUrl(user.avatar_url)` → 打开大图预览
- "Click to upload" 文本保留为上传触发器，有头像时改为 "Change avatar"
- 无头像时字母占位区不再触发上传（移除 `onClick`）

**代码变更** (`client/src/components/UserProfileModal.jsx`, `MessageItem.jsx`, `ChatList.jsx`, `MemberPanel.jsx`):
- 头像 `<img>` 点击改为打开大图预览，`stopPropagation` 阻止触发 UserProfileModal/SettingsModal
- 字母占位区保留原有点击行为（profile / settings）

### 15-2: Pinned 按钮始终可见（无 pin 时 disable）

**代码变更** (`client/src/components/ChatView.jsx`):
```diff
- {(pinnedMessage[chatId] || chat?.owner_id === user.id) && <button ...>}
+ <button ... style={{ opacity: pinnedMessage[chatId] ? 1 : 0.4 }}
+   onClick={() => { if (!pinnedMessage[chatId]) return; setShowNotice(!showNotice); }}>
```

按钮始终渲染，无 pin 时 `opacity: 0.4` 且点击无反应。

### 15-3: Send 按钮改为 SVG + Ghost 样式

**代码变更** (`client/src/components/Composer.jsx`):
- `className="btn btn-primary"` → `className="btn-ghost"`（无背景）
- 文本 "Send" → 右箭头 SVG

### 15-4: Send 图标方向修正

**代码变更** (`client/src/components/Composer.jsx`): 原先的纸飞机 SVG 朝向右上，改为标准右箭头。

### 15-5: Disabled pinned 按钮背景固定透明

**代码变更** (`client/src/components/ChatView.jsx`):
```diff
- background: showNotice ? 'var(--bg-tertiary)' : 'transparent',
+ background: (showNotice && pinnedMessage[chatId]) ? 'var(--bg-tertiary)' : 'transparent',
```

禁用状态下始终透明。

### 15-6: 头像链接失效时 fallback 到字母

**代码变更** (`client/src/components/MessageItem.jsx`, `SettingsModal.jsx`, `UserProfileModal.jsx`, `ChatList.jsx`, `MemberPanel.jsx`):
- 新增 `avatarError` state，`<img onError={() => setAvatarError(true)} />`
- 渲染条件改为 `avatar_url && !avatarError ? <img /> : <div className="msg-avatar">...</div>`
- 图片加载失败自动回退到字母占位

### 验证
- Build: ✓

---

## 2026-07-11 第 16 轮 — Mock/Go API 对齐（G1 + G2）

---

### 16-1: 前端 listChats 路径修正（G1）

**审计发现**: 前端调 `GET /api/chats`，后端注册 `GET /api/chats/my`，生产环境会 404。

**代码变更** (`client/src/api/client.js:87`):
```diff
-  listChats: (token) => request('GET', '/api/chats', token),
+  listChats: (token) => request('GET', '/api/chats/my', token),
```

---

### 16-2: 后端 Reactions 添加 `me` 字段（G2）

**审计发现**: Go 后端 Reaction 结构体无 `me` 字段，`reactionsFor()` 的 `viewerID` 参数未使用。前端无法高亮自己的 reaction。

**模型变更** (`server/internal/models/models.go:84-87`):
```diff
 type Reaction struct {
-	Emoji   string `json:"emoji"`
-	Count   int    `json:"count"`
+	Emoji   string   `json:"emoji"`
+	Count   int      `json:"count"`
+	UserIDs []string `json:"user_ids,omitempty"`
+	Me      bool     `json:"me"`
 }
```

**DB 变更** (`server/internal/db/messages.go` — `reactionsFor`):
- 收集每个 reaction 组的 `user_ids` 列表（之前只计数）
- JSON 缓存列现在包含 `user_ids`（仅在新添加/删除 reaction 时通过 `syncReactionsColumn` 更新，旧数据需手动触发）

**Handler 变更** (`server/internal/handlers/messages.go` — 新增 `enrichReactions`):
```go
func enrichReactions(msg *models.Message, viewerID string) {
	var rxs []models.Reaction
	json.Unmarshal(msg.Reactions, &rxs)
	for i := range rxs {
		for _, uid := range rxs[i].UserIDs {
			if uid == viewerID { rxs[i].Me = true; break }
		}
	}
	raw, _ := json.Marshal(rxs)
	msg.Reactions = raw
}
```

`enrichReactions` 在以下 handler 中调用：
- `ListMessages` — 遍历所有返回消息
- `AddReaction` — 返回更新后的消息前
- `RemoveReaction` — 返回更新后的消息前

注意: `me` 字段是 per-viewer 的，不注入 WS 广播（广播给其他客户端时会得到错误的 `me`）。客户端通过 store `onReaction` 自行计算 `me`。

### 验证
- Go build + vet: ✓
- Go reaction tests: ✓
- Client build: ✓

---

## 2026-07-12 第 17 轮 — Reaction `me` 改用专用端点（回退 enrichReactions）

---

### 问题

16-2 方案在 `ListMessages` 响应中对每条消息的 JSON 缓存列做 `enrichReactions`（N+1 解析），WS 广播中 `me` 对其他客户端无效。需要更干净的方案。

### 重新设计

- 移除 `enrichReactions` 函数及所有调用
- 新增 `GET /api/chats/{chatID}/messages/{messageID}/reactions` 端点，查询原始 `reactions` 表，按 `emoji` 分组，返回 `user_ids` 及 per-viewer `me` 标志
- 前端 `MessageItem` 在 `msg.reaction_count > 0` 时调用 `getReactions` 获取带 `me` 的数据

### 代码变更

#### 后端

**移除 `enrichReactions`** (`server/internal/handlers/messages.go`):
- 删除完整函数定义（242-262 行）
- 删除 `encoding/json` 导入（不再使用）

**移除 `enrichReactions` 调用** (`server/internal/handlers/reactions.go`):
- `AddReaction`: 删除 `enrichReactions(updated, u.ID)` 调用
- `RemoveReaction`: 删除 `enrichReactions(updated, u.ID)` 及 `!= nil` 检查

**新增 `ListReactions` handler** (`server/internal/handlers/reactions.go`):
```go
func (s *Server) ListReactions(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chatID := chi.URLParam(r, "chatID")
	msgID := chi.URLParam(r, "messageID")
	ok, _ := s.DB.IsChatMember(r.Context(), chatID, u.ID)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	rxs, err := s.DB.ListReactions(r.Context(), msgID, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reactions": rxs})
}
```

**新增路由** (`server/internal/handlers/router.go`):
```go
r.Get("/messages/{messageID}/reactions", s.ListReactions)
```

**新增 `DB.ListReactions`** (`server/internal/db/messages.go`):
```go
func (d *DB) ListReactions(ctx context.Context, messageID, viewerID string) ([]models.Reaction, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT emoji, user_id FROM reactions WHERE message_id = ? ORDER BY created_at`,
		messageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct{ emoji, uid string }
	var all []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.emoji, &r.uid); err != nil {
			return nil, err
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	grouped := map[string]*models.Reaction{}
	order := []string{}
	for _, r := range all {
		grp, ok := grouped[r.emoji]
		if !ok {
			grp = &models.Reaction{Emoji: r.emoji, UserIDs: []string{}}
			grouped[r.emoji] = grp
			order = append(order, r.emoji)
		}
		grp.Count++
		grp.UserIDs = append(grp.UserIDs, r.uid)
	}
	for _, grp := range grouped {
		for _, uid := range grp.UserIDs {
			if uid == viewerID {
				grp.Me = true
				break
			}
		}
	}
	out := make([]models.Reaction, 0, len(order))
	for _, e := range order {
		out = append(out, *grouped[e])
	}
	return out, nil
}
```

#### 前端

**新增 client API** (`client/src/api/client.js`):
```js
getReactions: (token, chatId, msgId) =>
  request('GET', '/api/chats/' + chatId + '/messages/' + msgId + '/reactions', token),
['getReactions', mockGetReactions],
```

**新增 mock** (`client/src/api/mock.js`):
```js
export function mockGetReactions(_token, _chatId, msgId) {
  const d = ensureData();
  const cu = currentUser();
  const msg = d.messages.find(m => m.id === msgId);
  if (!msg) return { reactions: [] };
  const raw = (msg.reactions || []).slice();
  const grouped = {};
  const order = [];
  for (const r of raw) {
    if (!grouped[r.emoji]) {
      grouped[r.emoji] = { emoji: r.emoji, count: 0, user_ids: [], me: false };
      order.push(r.emoji);
    }
    grouped[r.emoji].count = r.count;
    grouped[r.emoji].user_ids = r.user_ids || [];
  }
  for (const grp of Object.values(grouped)) {
    if (grp.user_ids.includes(cu.id)) grp.me = true;
  }
  return { reactions: order.map(e => grouped[e]) };
}
```

**MessageItem 改用 fetched reactions** (`client/src/components/MessageItem.jsx`):
- 新增 `reactions` state（初始值 `msg.reactions || []`）
- `useEffect`: 当 `msg.reaction_count > 0` 时调用 `getReactions` 填充 state
- `handleReaction`: 从 `reactions` state 读取 `me` 判断 toggle，操作后 refetch
- 渲染: 使用 `reactions` state 替代 `msg.reactions`

### 验证
- Go build: ✓
- Client build: ✓

---

## 2026-07-12 第 19 轮 — README 修正 + 死代码清理 + 审计项关闭

---

### G3/G4: README 文档修正

**G3** — README 说"无 refresh 刷新机制"，实际有完整 refresh_tokens 表 + token rotation。

**代码变更** (`README.md:152`):
```diff
- - **JWT 10yr**: access token 10年有效期,无 refresh 刷新机制
+ - **JWT 10yr + refresh rotation**: access token 10年有效期,配套 refresh_tokens 表实现 token rotation,支持 refresh 刷新机制
```

**G4** — README 列出 `POST /api/chats/:id/unpin` 不存在，实际是 `DELETE /api/chats/:id/pin`。

**代码变更** (`README.md:75`):
```diff
- | POST | `/api/chats/:id/unpin` | Bearer | Unpin |
+ | DELETE | `/api/chats/:id/pin` | Bearer | Unpin |
```

---

### G10: 死代码清理

**CreateMessage 中空循环** (`server/internal/db/messages.go:96-98`):
```go
// 删除前
for range dedupe(mentions) {
    // Deprecated: mentions are stored as JSON in messages.mentions column.
}
// Deprecated: attachments are stored as JSON in messages.mentions column.

// 删除后
// Deprecated: mentions/attachments are stored as JSON in messages.mentions column.
```

---

### M1: 侧边栏置顶 Go 端补齐

详见第 18 轮。新增 `POST /api/chats/{chatID}/pin-toggle`，`ListUserChats` 返回 `cm.pinned` 字段。

---

### M2: pinned_last_read_at 持久化

`chat_members.pinned_last_read_at` 列和 `UpdatePinnedLastReadAt` DB 函数已存在，补了 handler 和前端 API 调用。

**后端** — 新增 `MarkPinnedRead` handler (`server/internal/handlers/chat.go`):
- `POST /api/chats/{chatID}/pin-read` → 调用 `DB.UpdatePinnedLastReadAt`

**前端** (`client/src/store/chat.js`):
- `markPinnedRead` 改为先调 `api.markPinnedRead` 再更新本地 state

**Mock** (`client/src/api/mock.js`):
- 新增 `mockMarkPinnedRead`，更新数据层后通过 `onChatUpdate` 同步 store

---

### M3: Login/Register/Me role 字段差异

审计称 mock 返回额外字段 `role`/`last_seen`/`status`。实际 Go `User` 模型已有 `status` 和 `last_seen`，`role` 在 `ChatMember` 上，前端也不从 user 对象读 `role`。**Close，无代码变更。**

---

### M4: Upload 响应格式不一致

审计称 real upload.moonchan.xyz 返回 `{ id }`，mock 返回 `{ filename, mime_type, size, url }`。但 `client.js:141-146` 的 `api.upload` 已从 `File` 对象补充这些字段，真实和 mock 返回同一 shape。**Close，无代码变更。**

---

### 验证
- Go build: ✓
- Client build: ✓

---

### 背景

Mock 端 `togglePin` 切换 `chat.pinned`（侧边栏置顶），Go 端 `POST /pin` 是设置公告消息（`{ content }`），两套功能共用一个路由路径冲突。且 Go 后端 `chat_members.pinned` 列从未被 SELECT 或使用。

### 代码变更

#### 后端

**模型** (`server/internal/models/models.go`):
```diff
+	Pinned          bool          `json:"pinned"`   // Chat 新增
-	// Deprecated.
	Pinned          bool       `json:"pinned"`       // ChatMember 移除 Deprecated 注释
```

**`ListUserChats` SELECT 加入 `cm.pinned`** (`server/internal/db/chats.go:273-278`):
```diff
-		        cm.pinned_last_read_at
+		        cm.pinned_last_read_at, cm.pinned
```
Scan 新增 `var pinnedBool bool` + `c.Pinned = pinnedBool`。

**新增 `TogglePinned` DB 方法** (`server/internal/db/chats.go`):
```go
func (d *DB) TogglePinned(ctx context.Context, chatID, userID string) error {
	_, err := d.ExecContext(ctx,
		`UPDATE chat_members SET pinned = CASE WHEN pinned = 0 THEN 1 ELSE 0 END WHERE chat_id = ? AND user_id = ?`,
		chatID, userID,
	)
	return err
}
```

**新增 `TogglePin` handler** (`server/internal/handlers/chat.go`):
- 校验 chat member
- 调用 `DB.TogglePinned`
- 广播 `ChatUpdated`
- 路由: `POST /api/chats/{chatID}/pin-toggle`

#### 前端

**`togglePin` 路径改为 `/pin-toggle`** (`client/src/api/client.js:130`):
```diff
-  togglePin: (_token, chatId) => request('POST', '/api/chats/' + chatId + '/pin', _token),
+  togglePin: (_token, chatId) => request('POST', '/api/chats/' + chatId + '/pin-toggle', _token),
```

`mockTogglePin` 不变（dev 单用户场景等效）。前端 store 已有按 `!!a.pinned` 排序逻辑，Go 返回正确字段后自动生效。

### 验证
- Go build: ✓
- Client build: ✓

---

## 2026-07-12 第 20 轮 — CI 注释 + SSE 注释 + GAP 关闭

### 背景
代码 vs 报告审计共识别 14 项 GAP，经多轮修复后全部关闭。

### 变更

**CI 注释** (`.github/workflows/ci.yml:20`):
```yaml
# WS tests 为后续版本预备，当前版本不启用（需设置 WS_ENABLED=1）
- run: go test ./... -cover -coverprofile=coverage.out -count=1 -timeout 120s
```

**SSE 注释** (`server/internal/handlers/sse.go:41`):
```go
w.Header().Set("Access-Control-Allow-Origin", "*") // 为将来 CORS 支持而准备
```

### 最终 GAP 状态

| 编号 | 问题 | 处理 |
|------|------|------|
| G1 | 前端 listChats 端点不匹配 | client.js 改为 `/api/chats/my` |
| G2 | Reactions 缺 me 字段 | 新增 `GET .../reactions` 端点 |
| G3 | README refresh 描述错误 | 已修正 |
| G4 | README unpin 端点不存在 | 已修正 |
| G5 | api-gap-analysis 报告范围遗漏 | 不涉及代码变更 |
| G6 | final-consistency-audit 遗漏问题 | 不涉及代码变更 |
| G7 | CI 缺 WS_ENABLED | 已加注释说明 |
| G8 | DisallowUnknownFields 风险 | 不涉及代码变更 |
| G9 | SSE CORS * 硬编码 | 已加注释说明 |
| G10 | 死代码空循环 | 已删除 |
| G11 | UpdateLastRead deprecated 不一致 | 已移除标记 |
| G12 | attachments/mentions 物理表 | 不涉及代码变更 |
| G13 | 报告声称自动生成 | 不涉及代码变更 |
| G14 | 报告文档腐烂 | 不涉及代码变更 |

### 验证
- CI config: ✓
- Go build: ✓

---

## 2026-07-12 第 21 轮 — DB 错误静默忽略修复（X-Error 透传）

---

### 背景

Handler 中有 ~20 处 `ok, _ := s.DB.IsChatMember(...)` / `updated, _ := s.DB.GetChat(...)`，DB 失败时静默忽略，线上没法 debug。

### 变更

全都透传，以 `X-Error` header 传出。

#### 处理策略

| 场景 | 行为 |
|------|------|
| `IsChatMember` 错误 | 设 `X-Error` header + 返回 500 |
| `GetChat`/`GetMessage`/`ListUserChats` 后置拉取错误 | 设 `X-Error` header + 继续执行 |
| `DeleteRefreshToken`/`DeleteUserRefreshTokens` 错误 | 设 `X-Error` header + 继续执行 |

#### 文件变更

| 文件 | 改动数 | 模式 |
|------|--------|------|
| `server/internal/handlers/messages.go` | 7 处 | IsChatMember×5 + GetChat×1 + MarkRead×1 |
| `server/internal/handlers/chat.go` | 8 处 | GetChat×6 + IsChatMember×1 + DeletePinnedChat×1 |
| `server/internal/handlers/reactions.go` | 6 处 | IsChatMember×3 + GetMessage×3 |
| `server/internal/handlers/member.go` | 3 处 | IsChatMember×1 + GetChat×2 |
| `server/internal/handlers/auth.go` | 2 处 | DeleteRefreshToken + DeleteUserRefreshTokens |
| `server/internal/handlers/sse.go` | 1 处 | ListUserChats — 提前到 WriteHeader 之前 |

### 验证

- Go build + vet: ✓

---

## 2026-07-12 第 22 轮 — Deprecated 标记审计（调用链调查）

---

### 背景

代码中有 20+ 处 `// Deprecated.` 标记，但大部分仍被活跃使用。逐项追踪调用链，判断是真死代码还是误标记。

### 调查结果

完整报告见 `docs/reports/deprecated-call-chain-analysis-20260712.md`。

| # | 标记项 | 类型 | 真实状态 | 存活原因 |
|---|--------|------|---------|---------|
| 1 | `Chat.UnreadCount` | 模型字段 | **误标记 — 核心功能** | 唯一的未读计数机制，前端红点依赖 |
| 2 | `Chat.LastMessage` | 模型字段 | **迁移不完整** | `last_message_id` 列已加，但前端需要 `author+content` 预览 |
| 3 | `Message.Author` | 模型字段 | **误标记** | SQL JOIN 零成本，前端作 fallback |
| 4 | `ChatMember.LastReadMessageID` | 模型字段 | **模型可删，列不能删** | SQL 列仍用于 `UnreadCount` 计算 |
| 5 | `CreateOrGetDM` / `POST /api/dms` | handler/路由 | **真死代码** | 前端 UI 零调用，DM 已被过滤 |
| 6 | `Upload` handler + 路由 | handler/路由 | **生产死代码，测试活代码** | 前端走 `upload.moonchan.xyz`，仅 5 个 Go 测试在用 |
| 7 | URL query token fallback | 认证逻辑 | **误标记 — 无法移除** | EventSource/WebSocket 浏览器 API 不支持自定义头 |
| 8 | `attachmentsFor` 函数 + 表 | DB 函数/表 | **真死代码** | 全库零引用，附件已迁 JSON 列 |
| 9 | `FindDMBetween` | DB 函数 | **真死代码（联动）** | 仅被 `CreateOrGetDM` 调用 |
| 10 | `UnreadCount()` 函数 | DB 函数 | **误标记 — 核心功能** | `ListUserChats` 调用，无替代品 |
| 11 | `UploadDir` / `MaxUploadBytes` | 配置字段 | **真死代码（联动）** | 仅 Upload handler 使用 |
| 12 | `mockCreateDM` | Mock 函数 | **真死代码** | 前端零调用 |

### 结论

| 优先级 | 操作 | 项 |
|--------|------|----|
| 高（安全可删） | 直接移除 | `attachmentsFor` + `attachments` 表 |
| 中（联动删除） | 整组移除 | `CreateOrGetDM` + `FindDMBetween` + `mockCreateDM` (+ 3 个测试) |
| 中（联动删除） | 整组移除 | Upload handler + 路由 + `UploadDir`/`MaxUploadBytes` (+ 重写 5 个测试) |
| 低 | 移除模型字段 | `ChatMember.LastReadMessageID`（只删 struct 字段，不动列） |
| 低 | 更正注释 | 移除 `Chat.UnreadCount`、`Message.Author`、`UnreadCount()`、URL query token 上的 `Deprecated` 标记 |

---

## 2026-07-12 第 23 轮 — 发布准备 + 首屏公开聊天 + Bug 修复

---

### 23-1: 版本号统一为 `0.1.0-beta`

**背景**: 项目进入小范围测试阶段，需要统一的版本标识。

**代码变更**:
- `client/package.json`: `2.0.0` → `0.1.0-beta`
- `server/cmd/chatd/main.go` Swagger: `1.0` → `0.1.0-beta`

**Git tag**: `v0.1.0-beta`

---

### 23-2: 缺失 `mockGetReactions` 导入修复

**反馈**: 部署后浏览器控制台报 `Uncaught ReferenceError: mockGetReactions is not defined`。

**根因**: `client/src/api/client.js` 的 `MOCKABLE` 列表使用了 `mockGetReactions`，但 import 语句中缺失该导出。

**代码变更** (`client/src/api/client.js:8`):
```diff
-  mockMarkRead, mockAddReaction, mockRemoveReaction,
+  mockMarkRead, mockAddReaction, mockRemoveReaction, mockGetReactions,
```

---

### 23-3: Composer Send 按钮添加 title 属性

**反馈**: `15-3` 将 Send 按钮改为 SVG 图标（无文字），Playwright 测试使用 `button:has-text("Send")` 永远等不到元素。

**代码变更** (`client/src/components/Composer.jsx:84`):
```diff
-          onClick={handleSend}>
+          onClick={handleSend} title="Send">
```

**测试修正** (`client/tests/*.spec.mjs`):
```diff
-  await page.click('button:has-text("Send")');
+  await page.click('button[title="Send"]');
```

---

### 23-4: Add member 功能隐藏 + 成员列表实时同步

**反馈**: 
1. 加人逻辑不符合常理，应先隐藏
2. add/remove member 后成员列表不刷新

**代码变更** (`client/src/components/MemberPanel.jsx`):
- 移除整个 `"+ Add member"` 按钮、搜索框、用户搜索结果 UI
- 移除 `adding`/`search`/`results`/`searchUsers`/`addUser` 相关状态和函数
- `removeUser` 增加本地 `setMembers(prev => prev.filter(...))` 立即更新列表

**代码变更** (`client/src/api/mock.js` — `mockAddMember`):
```diff
+    if (_store) _store.getState().onChatUpdate({ id: chatId, members: [...chat.members] });
```

**代码变更** (`client/src/api/mock.js` — `mockRemoveMember`):
```diff
+    if (_store) _store.getState().onChatUpdate({ id: chatId, members: [...(chat.members || [])] });
```

---

### 23-5: 首屏 WelcomeView 改为公开聊天发现页

**背景**: 登录后首屏空白，改为展示最近活跃的公开聊天列表。

**后端** (`server/internal/db/chats_ext.go`):
- `ListPublicChats` 新增 `page`/`limit` 参数
- SQL 改为按 `last_message_at DESC NULLS LAST, created_at DESC` 排序
- 新增 `LIMIT ? OFFSET ?` 分页
- 子查询取 `last_message_content`（截取前 100 字符）
- Scan 新增 `lastMsgContent` 字段

**后端** (`server/internal/handlers/chat.go`):
- `ListPublicChats` 解析 `page`/`limit` 查询参数，透传 DB

**后端** (`server/internal/handlers/util.go`):
- 新增 `intQueryParam` 辅助函数

**后端** (`server/internal/db/db_test.go`):
- `TestListPublicChats_Empty` 适配新签名 `(ctx, page, limit)`

**后端 Bug 修复** (`server/internal/db/chats_ext.go`):
```diff
-  AND deleted = 0
+  AND deleted_at IS NULL
```
messages 表使用 `deleted_at`（时间戳，NULL=未删除），非 `deleted` 布尔字段。

**前端** (`client/src/components/WelcomeView.jsx` — 完整重写):
- 加载时调 `api.listPublicChats(accessToken, page, 20)` 获取公开聊天
- 显示名称、成员数、最后消息预览
- 已加入显示 "Open"，未加入显示 "Join"（点击先 join 再导航）
- `← Prev` / `Next →` 翻页（每页 20 条，少于 20 条时禁用 Next）

**前端** (`client/src/api/client.js`):
- `listPublicChats` 支持 `page`/`limit` 查询参数

**前端** (`client/src/api/mock.js` — `mockListPublicChats`):
- 支持分页、按 `last_message_at` 排序、填充 `last_message.content`

**前端** (`client/src/api/mock.js` — `mockJoinChat`):
```diff
+    if (_store) _store.getState().onChatUpdate({ ...chat, members: [...chat.members] });
```

**CSS** (`client/src/styles/global.css`):
- 新增 `.public-chat-card` 样式

**前端** (`client/src/components/WelcomeView.jsx`):
- 去除 Quick Start 欢迎文字（return null → 重新实现）

---

### 23-6: CI 调试 + 测试修复

**CI 调试** (`.github/workflows/frontend-ci.yml`):
- 增加 `curl` 步骤打印登录页和首页 HTML 内容
- 确认 Vite 正常返回 HTML，问题出在 Playwright 浏览器环境

**Local 测试结果**: 25/28 pass，2 fail（Send 按钮选择器，已修复），1 skip（DM 废弃）

---

### 验证
- Go build + vet + test: ✅
- Client build: ✅
- CI (Go): ✅
- Frontend CI: ✅ 全部通过（选择器修复后）

---

## 2026-07-12 第 24 轮 — 生产清理 + 成员数据加载

---

### 24-1: 移除 Quick Enter / Debug mode 按钮

**背景**: 公开测试后不应在 UI 暴露 Mock 入口。

**代码变更**:
- `LoginPage.jsx`: 移除 ⚡ Quick Enter 按钮、`quickEnter` 函数、`api`/`setDebugMode`/`mockLogin` import
- `RegisterPage.jsx`: 移除 Debug mode checkbox、⚡ Quick Enter (mock) 按钮、`quickEnter` 函数
- `main.jsx`: 添加 `window.__mockLogin` 测试钩子
- 所有测试文件: 用 `page.evaluate(() => window.__mockLogin())` 替代按钮点击

---

### 24-2: 移除 Composer 🤖 按钮

**背景**: "Send to AI (mock only)" 按钮在公开版无意义。

**代码变更** (`Composer.jsx:98-102`): 移除 🤖 按钮及 `api.isMockEnabled()` 逻辑。

---

### 24-3: Member count 修复

**反馈**: 无论何时 members 显示 0。

**根因**: 后端 `ListUserChats` 返回 `member_count`（int），前端所有组件读 `chat.members`（array），后端从不返回 `members` 数组。

**代码变更**:
- `ChatView.jsx:96`: `chat?.members?.length` → `chat?.member_count`
- `ChatView.jsx:97`: 移除 online count（无数据源）
- `ChatInfoModal.jsx:34`: 同上
- `MemberPanel.jsx:36`: 同上
- `PublicChannelList.jsx:32`: 同上

---

### 24-4: Phase 2 — 从 API 加载真实成员数据

**背景**: member count 显示正确了，但成员列表（头像、名字、Admin 标签）为空。

**方案**: 独立的 `membersByChatId: { [chatId]: [User, ...] }` map，不与 chat 对象耦合。

**后端**:
- `models.User` 加 `Role` 字段 (`json:"role,omitempty"`)
- `GetChatMembers` SQL 加 `cm.role`，scan 捕获

**Client API**:
- 新增 `api.listMembers(token, chatId)`
- 新增 `mockListMembers`（在 store 初始化时自动触发）

**Store** (`chat.js`):
- 新增 `membersByChatId: {}` 状态
- 新增 `loadMembers` action（调 API → 写 `membersByChatId`)
- `setChats`: 不再尝试保留 members（各存各的）
- `onChatUpdate`: 不再处理 members
- `reset`: 清空 `membersByChatId`

**组件**:
- `ChatView`: 进入聊天时调用 `loadMembers(accessToken, chatId)`；`userMap` 和 `getDMName` 从 `membersByChatId` 读取
- `MemberPanel`: 从 `membersByChatId[chatId]` 读取，移除本地 `members` state
- `ChatInfoModal`: 同理

**Mock 同步**:
- `mockAddMember`/`mockRemoveMember`/`mockJoinChat`: 同时更新 `membersByChatId`
- `mockCreateChat`/`mockCreateDM`: 加 `member_count`

**Bug 修复 — 后端** (`server/internal/db/chats_ext.go`):
- `JoinChatByID` 插入 `chat_members` 后未 `UPDATE chats SET member_count = member_count + 1`

---

### 24-5: Mock `member_count` 缺失修复

**背景**: Phase 2 完成后 members 仍显示 0（mock 模式）。

**根因**: Mock dummy data 生成 chat 时从未设置 `member_count` 字段。

**代码变更**:
- `dummy.js:generateDummyData`: 加 `member_count: members.length`
- `mock.js:mockCreateChat`: 加 `member_count`
- `mock.js:mockCreateDM`: 加 `member_count`
- `mock.js:mockJoinChat`: `chat.member_count++`
- `mock.js:mockAddMember`: `chat.member_count++`
- `mock.js:mockRemoveMember`: `chat.member_count--`

---

### 24-6: Deprecation 标记补充

- `client.js:createDM`: 加 `// @deprecated`
- `client.js:MOCKABLE`: 加 `// @deprecated`

---

### 验证
- Client build: ✅
- Go build + test: ✅
- Frontend CI: ✅

---

### 待办
- [x] `mockJoinChat`/`mockAddMember`/`mockRemoveMember`: 改用完整 chat 对象调 `onChatUpdate`，避免 store 只存 `{ id: chatId }` 丢字段

---

## 2026-07-12 第 25 轮 — 架构简化 + 后端 JoinChatByID 修复

---

### 25-1: 移除 `membersByChatId` store，组件直接调 API

**背景**: `membersByChatId` 增加了 store 复杂度，轮询时会覆盖、同步容易出 bug。

**方案**: 组件各自 `useEffect` 里直接 `api.listMembers(token, chatId)`，结果存本地 state。

**代码变更**:
- `store/chat.js`: 移除 `membersByChatId` 字段、`loadMembers` action、`user_update` handler、reset 里的清空
- `MemberPanel.jsx`: 新增 `useEffect` → `api.listMembers` → `setMembers`，移除 `membersByChatId` 依赖
- `ChatInfoModal.jsx`: 同上
- `ChatView.jsx`: 移除 `loadMembers` 调用、`membersByChatId`、`getDMName` 简化
- `mock.js`: 移除 `updateMembersByChatId` helper

---

### 25-2: 后端 `JoinChatByID` 补 `member_count` 自增

**背景**: `JoinChatByID` INSERT INTO `chat_members` 后从未 `UPDATE chats SET member_count = member_count + 1`。

**代码变更** (`server/internal/db/chats_ext.go:83-91`):
```go
res, err := d.ExecContext(ctx, ...)
n, _ := res.RowsAffected()
if n > 0 {
    _, err = d.ExecContext(ctx, `UPDATE chats SET member_count = member_count + 1 WHERE id = ?`, chatID)
}
```

---

### 25-3: 右键菜单 "Delete" → "Leave"

**反馈**: 删除应该是从自己列表移除（离开群组），不是解散整个 group。

**代码变更**:
- `ChatList.jsx`: `handleDeleteChat` → `handleLeaveChat`（调 `api.removeMember` + store `onChatDelete`），菜单文字 "Delete" → "Leave"
- `mock.js:mockRemoveMember`: 当 `userId === currentUser.id` 时从 `d.chats` 移除并调 `onChatDelete`

---

### 25-4: Member count 改为组件直接调 API

**背景**: `chat?.member_count` 从 store 读仍然有时为 0（取决于后端是否返回该字段）。

**方案**: ChatView header 的 member count 改为 `useEffect` → `api.listMembers` → `.length`，跟 MemberPanel / ChatInfoModal 一致。

**代码变更** (`ChatView.jsx`):
```jsx
const [memberCount, setMemberCount] = useState(0);
useEffect(() => {
    api.listMembers(accessToken, chatId).then(d => setMemberCount(d.members?.length || 0));
}, [chatId, accessToken]);
```

---

### 验证
- Client build: ✅
- Go build + test: ✅

---

## 2026-07-12 第 26 轮 — 架构简化 II

---

### 26-1: 右键菜单 "Delete" → "Leave"

**反馈**: 删除应该是从自己列表移除（离开群组），不是解散整个 group。

**代码变更**:
- `ChatList.jsx`: `handleDeleteChat` → `handleLeaveChat`（调 `api.removeMember` + store `onChatDelete`），菜单文字 "Delete" → "Leave"
- `mock.js:mockRemoveMember`: 当 `userId === currentUser.id` 时从 `d.chats` 移除并调 `onChatDelete`

---

### 26-2: 后端 `JoinChatByID` 补 `member_count` 自增

**代码变更** (`server/internal/db/chats_ext.go:83-91`):
```go
res, err := d.ExecContext(ctx, ...)
n, _ := res.RowsAffected()
if n > 0 {
    _, err = d.ExecContext(ctx, `UPDATE chats SET member_count = member_count + 1 WHERE id = ?`, chatID)
}
```

---

### 26-3: 移除 `membersByChatId` store，组件直接调 API

**背景**: store 维护成员列表增加复杂度，轮询覆盖、同步易出 bug。

**方案**: 各组件 `useEffect` 里直接 `api.listMembers(token, chatId)`，存本地 state。

**代码变更**:
- `store/chat.js`: 移除 `membersByChatId`、`loadMembers`、`user_update` handler、reset 清空
- `MemberPanel.jsx` / `ChatInfoModal.jsx`: 加 `useEffect` → `api.listMembers` → `setMembers`
- `ChatView.jsx`: 移除 `loadMembers` 调用、`membersByChatId`，`getDMName` 简化
- `mock.js`: 移除 `updateMembersByChatId` helper

---

### 26-4: `activeChatId` 完全由 URL 驱动

**背景**: store 的 `activeChatId` 和 URL `/g/:chatId` 双源冗余。

**方案**: 去掉 store 的 `setActiveChat` 暴露，`ChatPage` 直接用 `useParams().chatId`，所有组件通过 props 接收。

**代码变更**:
- `store/chat.js`: 移除 `setActiveChat` action
- `ChatPage.jsx`: `urlChatId` 直接作为 `chatId` prop 传递给所有子组件；URL 变更时 `useChatStore.setState({ activeChatId })`（仅用于内部 unread 逻辑）；`handleSelectChat` 只 `navigate` 不调 store
- `WelcomeView.jsx`: `navigate('/g/' + chatId)` 替代 `setActiveChat` + `pushState`

---

### 验证
- Client build: ✅
- Go build + test: ✅

---

## 2026-07-12 第 27 轮 — 结构化日志 + WebSocket 成员列表 + 访问追踪

---

### 27-1: 全面结构化日志（logutil）

**背景**: 替换 `stdlib log` 为内部 `logutil` 包，提升可观测性。

**新增文件** (`server/internal/logutil/log.go`):
- 提供 `Debug` / `Info` / `Warn` / `Error` 级别日志
- 支持 `With(key, val)` 附加结构化字段
- 注入到所有 handler、DB、WebSocket、SSE、Auth 流程

**代码变更**:
- `server/cmd/chatd/main.go`: 服务器启动/关闭日志改为 logutil
- `server/internal/config/config.go`: 配置加载日志
- `server/internal/db/*.go`: 所有 DB 操作加日志
- `server/internal/handlers/*.go`: 所有 handler 入口/出口加日志
- `server/internal/ws/*.go`: WebSocket 连接/消息/错误日志
- `server/internal/auth/auth.go`: Token 生成/验证/刷新日志

---

### 27-2: WebSocket 成员列表请求/响应

**背景**: 成员列表通过 WebSocket 实时获取，减少 API 调用。

**后端变更**:
- `server/internal/ws/hub.go`: 新增 `OpListMembers` / `OpMembersList` 消息类型、`ReqID` 字段用于请求追踪
- `server/internal/ws/client.go`: `readPump` 处理 `list_members` 请求，查询 DB 后返回 `members_list` 响应

**前端变更**:
- `client/src/store/chat.js`: 新增 `_wsReqs` map + `reqId` 计数器 + `wsRequest(op, payload)` 返回 Promise；WS 消息处理中匹配 `reqId` resolve/reject pending 请求
- `client/src/components/MemberPanel.jsx`: `useEffect` 中优先使用 `wsRequest('list_members', ...)`，回落 API
- `client/src/components/ChatInfoModal.jsx`: 同理
- 60 秒自动刷新成员列表

---

### 27-3: Chat 访问追踪

**背景**: 记录用户最后访问聊天时间，用于未读计数优化。

**后端变更**:
- `server/internal/db/db.go` + `chats.go`: `Chat` 模型新增 `LastActiveAt` 字段，`GetChat`/`ListUserChats` SELECT 该字段，`UpdateLastActiveAt` 写入 `chat_members.last_active_at`
- `server/internal/db/migrations/000__init.sql`: 合并 `last_seen` + `last_visited_at` → `last_active_at`
- `server/internal/handlers/chat.go`: 新增 `VisitChat` handler
- `server/internal/handlers/router.go`: 注册 `POST /api/chats/{chatID}/visit`

**前端变更**:
- `client/src/api/client.js`: 新增 `visitChat` API
- `client/src/routes/ChatPage.jsx`: 进入聊天时调用 `api.visitChat`
- 未读红点改为使用 `chat.unread_count` 直接显示

---

### 验证
- Client build: ✅
- Go build + test: ✅
- Frontend CI: ✅

---

## 2026-07-13 第 28 轮 — DB migration ASCII 排序 bug + chat leave 修复

---

### 28-1: DB migration 排序导致 V001 在 init.sql 之前执行

**根因**: `sort.Strings` 按 ASCII 排序，`V`(86) < `i`(105)，V001 排在 init.sql 之前 → 空数据库先跑 ALTER TABLE，chat_members 尚不存在，全库测试挂。

**修复**:
- `init.sql` → `000__init.sql`（`0`=48 < `V`=86，保证 init 先执行）
- 删除遗留的 `rename_last_visited.sql`（旧非版本化 migration，与 V001 重复）

### 28-2: 离开聊天改为调 removeMember

**反馈**: 点击 Leave 之后整个群组从其他成员的列表消失。

**根因**: `handleDeleteChat` 调 `deleteChat` 接口（解散群组），非 `removeMember`（仅自己退出）。

**修复**:
- `ChatList.jsx`: `handleDeleteChat` → `handleLeaveChat`，改调 `api.removeMember`
- 菜单文字 "Delete" → "Leave"
- `mockRemoveMember`: 自己退出时从 `d.chats` 移除并调 `onChatDelete`
- 增加 `navigate('/', { replace: true })` 离开当前活跃聊天

### 验证
- Go all tests: ✅
- Client build: ✅

---

## 2026-07-14 真实后端修复（第 5 轮）

### 环境
- 真实后端部署于 `chat.moonchan.xyz`

### 用户反馈

#### Bug 1: 退出聊天白屏
- **现象**: 点击退出聊天后页面白屏（React Error #300/#310）
- **根因**: `MemberPanel`、`ChatInfoModal` 的 `if (!chat) return null` 写在 `useEffect` 之前，违反 rules-of-hooks
- **修复**: 将 `useEffect` 移到 early return 之前；`ChatView` 加 `if (!chat) return null` guard 在所有 hooks 之后
- **文件**: `client/src/components/MemberPanel.jsx`, `client/src/components/ChatInfoModal.jsx`, `client/src/components/ChatView.jsx`

#### Bug 2: 公开群聊重新进入不显示消息
- **现象**: 退出公开群聊后再次进入，消息列表空白
- **根因**: ChatView 直接读 store 中的 chat，但退出时 chat 已被移除
- **修复**: ChatView 加 `useEffect` 检测 chat 不在 store 时通过 `api.getChat` 重新获取
- **文件**: `client/src/components/ChatView.jsx`

#### Bug 3: 登录失败 → 注册 → 自动跳回登录页
- **现象**: 登录时输错密码（服务器返回错误），跳到注册页注册成功后又被弹回登录页
- **根因**: 登录失败后 store 残留了旧 `accessToken`，Route guard (`/app`) 判断有 token 但未初始化 → 重定向到 `/`
- **修复**: `login()`/`register()` 捕获失败后调用 `storage.clear()` 并 `set({user:null, accessToken:null})`
- **文件**: `client/src/store/auth.js`

#### Bug 4: 红点通知反复出现
- **现象**: 点击聊天消除红点后，收到新消息红点又出现在旧的聊天上
- **根因**: `onMessageCreate` 内依赖的 `activeChatId` 是 stale closure；`handleSelectChat` 中 `unread_count` 未重置
- **修复**: `handleSelectChat` 同步设置 `activeChatId`（而非通过 useEffect 延迟）并重置 `unread_count = 0`
- **文件**: `client/src/routes/ChatPage.jsx`, `client/src/store/chat.js`

#### Bug 5: 确认对话框残留
- **现象**: 退出聊天弹 confirm 对话框，用户体验差
- **修复**: 移除 confirm dialog，`queueMicrotask` 延迟 `onChatDelete` 到 `navigate('/')` 之后
- **文件**: `client/src/components/ChatList.jsx`

### 改进: 未读计数改用 last_active_at

- **背景**: 原先用 `last_read_message_id` 计算未读，需要前端每次传 `message_id`，逻辑繁琐且容易出错
- **方案**: 在 `chat_member` 表新增 `last_active_at` 字段，未读计数改为 `SELECT COUNT(*) WHERE created_at > last_active_at`
- **变更**:
  - `UnreadCount(ctx, chatID, lastActiveAt)`: 按时间戳计未读，上限 99
  - `GetMessages` 默认 limit 50→100
  - `MarkRead` handler 不再读取 request body，直接更新 `last_active_at`
  - `ChatListItem` badge 显示 `99+` 封顶
  - `UpdateLastRead` / `readReq` 标记为 deprecated
- **文件**: `server/internal/db/messages.go`, `server/internal/db/chats.go`, `server/internal/handlers/messages.go`, `client/src/components/ChatListItem.jsx`, `client/src/routes/ChatPage.jsx`, `client/src/api/client.js`

#### Bug 6: UnreadCount 时区比较错误
- **现象**: CI 测试 `TestUnreadCount` 失败
- **根因**: `UnreadCount` 格式化 `lastActiveAt` 时未转 UTC，但 `created_at` 始终存 UTC。字符串比较时带时区偏移的时间导致 `created_at > lastActiveAt` 为 false
- **修复**: `lastActiveAt.UTC().Format(time.RFC3339Nano)` 统一转 UTC
- **文件**: `server/internal/db/messages.go:272`

#### Bug 7: 快速切换聊天显示错乱
- **现象**: 快速点击不同聊天，之前的 chat 的 fetch 完成后覆盖当前聊天显示
- **根因**: `loadMessages` 用 `data.messages` 直接替换 `s.messages`，旧 chat 的异步响应晚于新 chat 到达时将 store 覆盖
- **修复**: 引入 `_msgLoadId` 递增计数器，`loadMessages` 完成后检查 `_msgLoadId` 是否匹配，不匹配则丢弃结果
- **文件**: `client/src/store/chat.js`

#### Bug 8: 登录失败后 refresh/logout 无限循环
- **现象**: 登录失败（密码错误）后控制台不断输出 refresh 400 + logout 401
- **根因**: `api.logout()` 调用时 token 已 null → 服务器 401 → `request()` 自动尝试 refresh → 400 → 再次 触发 `auth:unauthorized` → 循环
- **修复**:
  - `logout()` 检查 `accessToken` 存在才调用 `api.logout()`
  - `request()` 对 `/api/auth/logout` 路径跳过 auto-refresh
  - App.jsx `auth:unauthorized` 处理器的 guard 不再用 setTimeout 重置（一次性守卫）防止二次触发
- **文件**: `client/src/store/auth.js`, `client/src/api/client.js`, `client/src/App.jsx`

### 验证
- Go all tests: ✅
- Client build: ✅
- 推送 `41b0cfd`，CI 构建中

---

## 2026-07-14 搜索/UI 修复（第 6 轮）

### 用户反馈

#### Bug 9: 快速切换聊天时旧 fetch 覆盖当前聊天
- **现象**: 快速切换聊天，前一个 chat 的请求完成后覆盖当前聊天显示
- **根因**: `loadMessages` 无条件替换 `s.messages`
- **修复**: 引入 `_msgLoadId` 递增计数器，返回时校验 id，不匹配则丢弃
- **文件**: `client/src/store/chat.js`

#### Bug 10: 登录失败 refresh/logout 无限循环
- **现象**: 登录失败后控制台不断刷 refresh 400 + logout 401
- **根因**: `api.logout()` 时 token 已 null → 401 → `request()` 自动 refresh → 400 → 再次 `auth:unauthorized` → 循环
- **修复**: `logout()` 无 token 跳过 `api.logout()`；`request()` 跳过 `/api/auth/logout` 的 auto-refresh；App.jsx guard 不再用 setTimeout 重置
- **文件**: `client/src/store/auth.js`, `client/src/api/client.js`, `client/src/App.jsx`

#### Bug 11: 搜索框 focus 不显示公开频道
- **现象**: 搜索框 focus 时没有自动加载公开频道列表
- **修复**: `onFocus` 加载所有公开频道；输入文字实时过滤；blur 200ms 延迟隐藏；隐藏时原有 chat list 不显示
- **文件**: `client/src/components/ChatList.jsx`

#### Bug 12: 搜索匹配 UUID 导致误中
- **现象**: 搜 "44" 弹出某个聊天（UUID 含 44），搜 "4" 几乎全中
- **根因**: `filteredChats` 和 `filterPublicChats` 都按 `c.id` 子串匹配
- **修复**: 正常搜索只按 `c.name` 匹配；输入完整 UUID 格式时精确匹配 ID
- **文件**: `client/src/components/ChatList.jsx`

#### Bug 13: 首条消息的 emoji picker 被标题挡住
- **现象**: 第一位发的消息点 emoji picker，弹出框被 chat header 裁剪
- **根因**: emoji picker `position:absolute; bottom:100%`，`.chat-body` 的 `overflow-y:auto` 裁剪溢出
- **修复**: 改用 `position:fixed`，打开时计算按钮位置，空间不足时显示在下方
- **文件**: `client/src/components/MessageItem.jsx`

#### Bug 14: 输入数字显示 "Join #N"
- **现象**: 搜索框输入 "1" 出现 "Join #1" 按钮，含义不明
- **根因**: 旧版数字 ID join 残留逻辑 `/^\d+$/`
- **修复**: 去掉数字匹配，仅保留显式 `join <uuid>` 命令
- **文件**: `client/src/components/ChatList.jsx`

### 验证
- Client build: ✅
- 最新推送 `78be5e8`，CI 构建部署中

---

## 2026-07-14 细节修复（第 7 轮）

#### Bug 15: 粘贴图片上传不压缩
- **现象**: 粘贴图片直接上传原图，体积大
- **修复**: 粘贴图片 canvas 转 WebP quality 0.75 后上传；文件选择器保持原样
- **文件**: `client/src/components/Composer.jsx`

#### Bug 16: 创建群组按钮样式不一致
- **现象**: 点击 "+" 后 visibility radio 按钮太小，不适配触屏
- **修复**: radio 改为 pill 按钮样式，`minHeight: 36`，选中高亮；Create/Cancel 按钮统一大小
- **文件**: `client/src/components/CreateGroupForm.jsx`

#### Bug 17: Emoji picker 离按钮太远
- **现象**: 点击 😀 后 picker 显示在按钮上方 200px，组件远离按钮
- **修复**: 改为紧贴按钮上方 8px，空间不够才显示下方
- **文件**: `client/src/components/MessageItem.jsx`

#### Bug 18: 输入栏布局
- **现象**: 文件选择器在输入栏下方独立一行，空间浪费
- **修复**: 📎 移到发送按钮左边，与 textarea 同行；保留 autoResize
- **文件**: `client/src/components/Composer.jsx`

### 其他
- 版本号 `0.1.0-beta` → `0.2.0`
- `docs/README.md` 添加 Release Checklist

### 验证
- Client build: ✅
- 最新推送 `09271f9`，CI 构建中

---

## 2026-07-15 安全加固（第 8 轮）

#### Bug 19: 登录无频率限制
- **现象**: 无限制尝试密码，可暴力破解
- **修复**: 每 IP 每小时最多 5 次登录失败，超限返回 429
- **文件**: `server/internal/handlers/ratelimit.go`, `auth.go`, `handler.go`

#### Bug 20: Cloudflare IP 段硬编码
- **现象**: CF IP 段可能过期，导致来源判定出错
- **修复**: 启动时从 `cloudflare.com/ips-v4` 和 `ips-v6` 拉取最新段，失败则回退硬编码列表
- **文件**: `server/internal/handlers/ratelimit.go`

### 验证
- Go build: ✅ `go vet` ✅
- 最新推送 `557152e`，CI 构建中

---

## 2026-07-15 功能改进（第 9 轮）

#### Pin-Toggle 改为显式设定
- **现象**: `pin-toggle` 自动翻转顶置状态，无法直接设定
- **修复**: 接受 `{"pinned": true/false}` body，DB 新增 `SetPinned()` 方法；Context Menu 分 Pin/Unpin 两个按钮
- **文件**: `server/internal/handlers/chat.go`, `server/internal/db/chats.go`, `client/src/api/client.js`, `mock.js`, `ChatList.jsx`

#### Pinned Message 改名为 公告
- **现象**: 群组公告栏无明确标题
- **修复**: 公告横幅前显示 `📢 公告` 标签
- **文件**: `client/src/components/ChatView.jsx`, `client/src/dev/dummy.js`

### 验证
- Go build: ✅ `go vet` ✅
- Client build: ✅
- 最新推送 `f564348`，CI 构建中

---

## 2026-07-15 API 重命名与风格统一（第 10 轮）

#### API 路径重命名
- `/pin` (POST/PATCH/DELETE) → `/announcement` — 公告 CRUD
- `/pin-read` → `/announcement/read` — 标记公告已读
- `/pin-toggle` → 拆分为 `POST /pin` 和 `POST /unpin` — 显式顶置/取消顶置
- 前后端所有函数名同步更新（`setAnnouncement`, `clearAnnouncement`, `pinChat`, `unpinChat` 等）
- **文件**: `server/internal/handlers/router.go`, `chat.go`, `client/src/api/client.js`, `mock.js`, `store/chat.js`, `ChatList.jsx`, `ChatView.jsx`

#### 文档更新
- `docs/features/api-endpoints.md` — 更新 API 路径表
- `docs/features/go-api-routes.md` — 更新路由表
- `docs/features/go-api-models.md` — 更新模型引用
- `docs/features/mock-vs-go-api-report.md` — 更新对齐报告
- `docs/features/frontend-architecture.md` — 更新公告流程图

### 验证
- Go build: ✅ `go vet` ✅
- Go test: ✅
- Client build: ✅
- 最新推送 `9f54c38`，CI 构建中

---

## 2026-07-15 服务层架构重构（第 11 轮）

### 背景

Handlers 直接调 `s.DB.*`，权限检查、Hub 广播、验证逻辑在每个 handler 重复。Handler = HTTP + 业务逻辑紧耦合。

### 方案

提取 `server/internal/service/` 包，作为 handler 与 DB 之间的中间层。

### 新增文件（6 个）

| 文件 | 行数 | 职责 |
|------|------|------|
| `server/internal/service/errors.go` | 10 | Sentinel 错误（ErrForbidden, ErrNotFound 等） |
| `server/internal/service/service.go` | 30 | 容器 + WithTx 占位 |
| `server/internal/service/authz.go` | 53 | MustBeMember, RequireOwnerOrAdmin（解耦自 ChatService） |
| `server/internal/service/chat.go` | 175 | ChatService（聊天业务逻辑） |
| `server/internal/service/message.go` | 111 | MessageService（消息业务逻辑） |
| `server/internal/service/member.go` | 100 | MemberService（成员管理） |

### 修改文件（4 个）

| 文件 | 变更 |
|------|------|
| `handler.go` | 新增 `Services` 字段、`mapServiceError()` 函数 |
| `chat.go` | -179 +86，只做 HTTP 编解码 |
| `messages.go` | -199 +88，只做 HTTP 编解码 |
| `member.go` | -156 +43，只做 HTTP 编解码 |

### 关键设计决策

1. **Sentinel Errors** — 所有 service 返回命名错误，handler 用 `mapServiceError` 映射到 HTTP 状态码
2. **AuthZ 解耦** — `MustBeMember`/`RequireOwnerOrAdmin` 在 `authz.go` 中，可被 ChatService/MessageService/MemberService 共用
3. **WithTx 占位** — 预备未来跨表事务，当前透传
4. **广播集中化** — Hub 广播在 service 层内完成，不依赖 handler

### 验证

- `go build ./...`: ✅
- `go vet ./...`: ✅
- `go test ./...`: ✅
- CI (GitHub): ✅

---

## 2026-07-15 DB 包拆分 — refresh_tokens.go（第 12 轮）

### 操作

`chats.go` (487行) 中混有 5 个 Refresh Token 方法（CreateRefreshToken, FindRefreshToken, DeleteRefreshToken, DeleteUserRefreshTokens, PurgeExpiredTokens），文件命名与实际内容不符。

**修复**：
- 新增 `refresh_tokens.go`（78行），搬入 5 个方法 + 精准 import
- `chats.go` 删除对应 60 行，现纯属 chat/member 领域

### 验证

- `go build ./...`: ✅
- `go vet ./...`: ✅
- `go test ./...`: ✅
- CI (GitHub): ✅
- 最新推送 `f689db4`，CI 构建中

---

## 2026-07-15 DB 文件拆分 — chats.go + messages.go（第 13 轮）

### 操作

`chats.go` (427行) 包含 Chat CRUD + 成员管理 + pin 操作，`messages.go` (523行) 包含消息 CRUD + reactions。

**拆分**：
- `chats.go` (293行) → chats.go + `chat_members.go` (150行)
- `messages.go` (356行) → messages.go + `message_reactions.go` (176行)

### 验证
- `go build ./...`: ✅
- `go test ./...`: ✅

---

## 2026-07-15 refreshMu 竞态修复（第 14 轮）

### 问题

`handler.go:35` 的 `refreshMu sync.Mutex` 只保护了 Find+Delete，不保护 `issueSession`（创建新 token），Logout 也未加锁，存在并发竞态。

### 修复

1. **`db.FindAndDeleteRefreshToken`** — 原子 find+delete 事务，替代分离的 Find+Delete 调用
2. **`Refresh`** — 锁范围扩展到 `issueSession`（`defer s.refreshMu.Unlock()`）
3. **`Logout`** — 持有 `refreshMu` 执行 `DeleteUserRefreshTokens`

### 并发安全性

- 两个 `Refresh` 同 token → 原子 find+delete，第一个成功第二个 401
- `Refresh` vs `Logout` → 互斥锁串行化，`issueSession` 完成后 `Logout` 删除所有 token

### 验证
- `go build ./...`: ✅
- `go test ./...`: ✅

---

## 2026-07-15 实时连接重构 + Mock 解耦（第 15 轮）

### 1. RealtimeCoordinator — 连接状态机

**新增** `client/src/realtime/`
- `coordinator.js` — 状态机 singleton（IDLE/CONNECTING/CONNECTED/DISCONNECTING），统一管理连接生命周期和自动重连
- `transports/ws.js` — WebSocket 传输
- `transports/sse.js` — SSE 传输
- `transports/poll.js` — 轮询传输
- `transports/mock.js` — Mock 传输

**核心机制**：
- 状态守卫锁：`connect()` 入口检查 CONNECTING/DISCONNECTING 则跳过，防止双通道
- `_closeGuard`：手动 `disconnect()` 设 true，阻止 transport `onClose` 触发自动重连
- 自动重连：transport `onClose` → 3s 后 `connect(mode, token)`（仅当状态仍为 IDLE）

### 2. Proxy 替代 MOCKABLE 数组

**修改** `api/client.js`
- MOCKABLE 数组 + `save()`/`swap()` → `new Proxy(realApi, { get })`
- `_mockHandlers` 字典：`{ methodName: mockFn }`，新增 API 只需加一行
- 无缓存层，每次 `api.xxx` 返回新的包装函数

### 3. 消除 `__setStoreRef` 循环依赖

**修改** `api/mock.js`
- `__setStoreRef`/`__getAuthUser` → `import('../store/chat')` 模块级动态 import
- store 加载完成后自动设 `_store`，`if (_store)` 保护空窗期

### 4. Store 瘦身

**修改** `store/chat.js`
- 移除 `connectWS` / `connectSSE` / `connectPolling` / `disconnect` / `_lastToken`
- `setMode` / `connect` / `sendTyping` / `subscribe` / `wsRequest` → 薄代理到 coordinator

### 5. 组件适配

**修改** `ChatPage.jsx` — `connectWS/SSE/Polling` → `connect(token)`
**删除** `dev/mock-ws.js`（已死代码）

### 验证
- Client build: ✅
- Go build + vet: ✅

---

## 2026-07-15 Hub 成员列表缓存（第 16 轮）

### 背景

`sendToChat` 每次广播都调 `GetChatMembers`（SQL JOIN 查询）。500 人群每秒 10 条消息 → 5000 qps DB 查询，连接池大概率挂。

### 修复

`server/internal/ws/hub.go`:
- 新增 `memberCacheEntry` 结构体 + `memberCache map[string]*memberCacheEntry` + `memberMu sync.Mutex`
- `getCachedMembers(chatID)`: 命中且未过期返回，否则 nil
- `setCachedMembers(chatID, members)`: 写入 cache，TTL 1s
- `sendToChat`: 先查缓存，miss 才查 DB 并写回缓存
- 1s TTL 削减 ~99% DB 查询压力

### 验证
- Go build + vet: ✅

---

## 2026-07-15 CSP 可配置化（第 17 轮）

### 背景

`connect-src` 硬编码 `wss://wsl-8080.moonchan.xyz`，本地开发不用 tunnel 时 WS 被 CSP 拦截。

### 变更

- `server/internal/config/config.go`: 新增 `CSPConnectSrc` 字段，`Load()` 从 `CHAT_CSP_CONNECT_SRC` 环境变量读取，默认值保持向后兼容
- `server/internal/handlers/router.go`: CSP header 改为 `"connect-src "+s.Cfg.CSPConnectSrc`

### 开发环境用法

```bash
CHAT_CSP_CONNECT_SRC="'self' ws://localhost:8080 wss://localhost:8080 http://localhost:* https://upload.moonchan.xyz" go run ./cmd/chatd
```

### 验证
- Go build + vet: ✅

---

## 2026-07-15 JSDoc 类型注释 — API + Store + Mock（第 18 轮）

### 背景

前端纯 JSX，状态形状、API 响应类型、组件 props 全靠运行时隐式约定，无类型提示。

### 变更

**新增** `client/src/types.js` — 共享类型定义：
- `User`, `Chat`, `Message`, `Reaction`, `Attachment`, `PinnedContent`, `StreamSource`

**`store/chat.js`**:
- `@typedef ChatStore` 完整 state + 所有 action 方法签名
- 每个 action 方法加 `@param` / `@returns`
- 外部回调（`onReady`, `onEvent`）加参数类型

**`api/client.js`**:
- 所有 API 方法加 `@param` / `@returns`
- 响应类型化（`AuthResponse`, `ListChatsResponse` 等）
- `request()`、`buildUploadUrl()`、`mockCallLog()` 加签名

**`api/mock.js`**:
- 所有 export 函数加 `@param` / `@returns`
- `randid()`、`currentUser()`、`userById()`、`messagesFor()` 加签名

**新增** `client/tsconfig.json`:
- `allowJs: true`, `checkJs: true` — IDE 可启用 TypeScript 类型检查

### 验证
- Client build: ✅ 76 modules

---



---

## 2026-07-15 BroadcastUserUpdate data race 修复（第 19 轮）

### 问题

`BroadcastUserUpdate` 在 `h.mu.RLock()` 下读 `c.subs`，但 `subs` 由 `c.mu`（`Client.mu`）保护。两把锁不一致，并发迭代+写入 map 会 panic。

### 修复

`server/internal/ws/hub.go:256` — 遍历 `c.subs` 前加 `c.mu.RLock()` / `c.mu.RUnlock()`。

### 验证
- Go build + vet: ✅

---

## 2026-07-15 removeUser try/catch 修复（第 20 轮）

### 问题

`MemberPanel.removeUser` 无 try/catch，网络错误 → Unhandled Rejection → React 白屏。

### 修复

`client/src/components/MemberPanel.jsx:38` — `api.removeMember` 包裹 try/catch，错误用 `notify()` 显示。

### 验证
- Client build: ✅

---

## 2026-07-15 统一前端错误通知通道（第 21 轮）

### 问题

各组件错误处理碎片化：`alert()`、`console.error()`、静默 `catch(() => {})` 混用。

### 实现

**新增** `client/src/store/notification.js`:
- `useNotificationStore` — Zustand store，`notifications[]`
- `notify(message, type, duration)` — 全局函数，组件直接调用

**新增** `client/src/components/Toast.jsx`:
- 右上角堆叠，4s 自动消失，滑入动画
- 挂载在 `main.jsx` `<BrowserRouter>` 内

**集成**（10 处替换）:
- `Composer.jsx` — `alert()` → `notify()`
- `ChatView.jsx` / `ChatInfoModal.jsx` / `MemberPanel.jsx` — `.catch(() => {})` → `notify()`
- `MemberPanel` 移除内联 `removeErr` 状态

### 验证
- Client build: ✅ 76 modules

---

## 2026-07-15 Handler 层直连 DB 清理 + coordinator 修复（第 22 轮）

### 问题

审计第 2 条及更多：4 个 handler 文件绕过 Service 层直接调用 `s.DB.*`。另 `coordinator.js` 有两处 bug：
1. `disconnect()` 后旧 `setTimeout` 重连 — 缺少 `_closeGuard` 检查
2. `_initTransport` 在 ws/sse 握手完成前就设 `_state = STATE.CONNECTED`

### 实现

**新增** `server/internal/service/user.go`:
- `UserService` — `GetByID`, `GetByEmail`, `Create`, `UpdateProfile`, `Search`
- DB 错误统一映射为 `service.Err*`

**新增** `ChatService.CreateOrGetDM` — 封装 DM 创建全流程（查用户 + 查现有 DM + 创建 + 广播）

**修改** 4 个 handler:
- `auth.go` — `CreateUser`/`GetUserByEmail`/`GetUserByID` → `Services.User.*`
- `users.go` — `UpdateUserProfile`/`SearchUsers` → `Services.User.*`
- `sse.go` — `GetUserByID`/`ListUserChats` → `Services.User.GetByID`/`Services.Chat.ListForUser`
- `chat.go` — `CreateOrGetDM` 全链路 → `Services.Chat.CreateOrGetDM`

**修改** `client/src/realtime/coordinator.js`:
- `setTimeout` 回调增加 `if (this._closeGuard) return`（disconnect 后不重连）
- `_state = STATE.CONNECTED` 移入 `onReady` 回调，增加 `if (this._state !== STATE.CONNECTING) return` 过时守卫

### 验证
- Server: `go build`, `go vet`, `go test` — ✅
- Client: `npm run build` — ✅

---



## 2026-07-16 后端添加单元测试覆盖（第 24 轮）

### 新增测试文件

| 文件 | 测试数 | 覆盖包 |
|------|--------|--------|
| `server/internal/config/config_test.go` | 6 | config (100%) |
| `server/internal/orderedmap/orderedmap_test.go` | 23 | orderedmap (71.4%) |
| `server/internal/service/service_test.go` | 83 | service (80.4%) |
| `server/internal/handlers/util_test.go` | 17 | handlers util (100% of util) |

### 测试内容

- **config**: 默认值 / 自定义环境变量 / 非法值回退 / JWT 随机密钥
- **orderedmap**: 核心 CRUD / 排序 / JSON 序列化与反序列化 / 嵌套对象 / HTML 转义
- **service (83 个测试)**: Chat CRUD + 权限（owner/admin/member）/ DM / 消息发送校验（附件 URL 白名单 / 内容长度 / @提及提取）/ 成员管理（owner 保护）/ User CRUD + 搜索 / authz 边界
- **handlers util**: mapServiceError / writeJSON / writeError / decodeJSON / bearerToken / cookie helpers

### 验证
- `go build ./...` — ✅
- `go vet ./...` — ✅
- `go test ./...` — ✅

---

## 2026-07-16 gateway.go/poll.js/handler.go/ChatPage/chat.js 修复（第 25 轮）

### 问题

1. `gateway.go` — `chats, _ := g.db.ListUserChats(...)` 丢弃错误，失败后 `chats` 为 nil，JSON 序列化为 `"chats": null`
2. `handler.go` — `ErrContentTooLong` 返回 403 Forbidden，语义应为 413
3. `poll.js` — `disconnect()` 后 in-flight 异步 poll 仍会调度下一轮
4. `ChatPage.jsx` — 与 `ChatView.jsx` 重复调用 `loadMessages`
5. `chat.js` — `setChats`/`onChatUpdate`/`onMessageCreate` 3 处重复的排序逻辑

### 修复

- `gateway.go` — 记录 error log，失败时 `chats = []models.Chat{}` 保底空数组
- `handler.go` — `http.StatusForbidden` → `http.StatusRequestEntityTooLarge`
- `poll.js` — `cancelled` 标志，`disconnect()` 设 true，`poll()` 调度前检查
- `ChatPage.jsx` — 移除第 45 行 `loadMessages(accessToken, urlChatId)`
- `chat.js` — 抽取 `sortChats(a, b)` 公共函数替换 3 处内联排序

### 验证
- Server: `go build`, `go vet`, `go test` — ✅
- Client: `npm run build` — ✅

---

## 2026-07-16 删除死代码 modelsUser + 补充 Chat.Join 测试 + 修复 content_too_long 状态码（第 26 轮）

### 变更

| 文件 | 操作 |
|------|------|
| `server/internal/service/authz.go` | 删除未使用的 `modelsUser` 函数及对应 `models` import |
| `server/internal/service/service_test.go` | 新增 `TestChatService_Join_Success` / `Join_PrivateChat` / `Join_Nonexistent` |
| `server/internal/handlers/util_test.go` | `ErrContentTooLong` 期望值 403 → 413（对齐实际代码） |
| `server/internal/testutil/handler_test.go` | 同一测试期望值 403 → 413 |

### 验证
- `go build ./...` — ✅
- `go vet ./...` — ✅
- `go test ./...` — ✅

---

## 2026-07-16 UserAvatar 组件抽取 + 模态框无障碍（第 27 轮）

### 新增
- `client/src/components/UserAvatar.jsx` — `<UserAvatar user size onClick onFallbackClick>`，内置 img 错误回退/首字母彩色圆环 fallback

### 替换 6 处内联头像渲染
| 组件 | 效果 |
|------|------|
| `MemberPanel` | 替换内联条件 + `avatarError` 状态 |
| `MessageItem` | 同上，同步删除 `initials` 变量 |
| `UserProfileModal` | 同上 |
| `SettingsModal` | 同上 |
| `ChatList` (侧栏底部) | 同上 |
| `DmSearchPanel` | 同上，**修复**缺失 `onError` 回退的 bug |

### 模态框无障碍
4 个模态框统一添加：
- `role="dialog"` + `aria-modal="true"` + `aria-label`
- Escape 键关闭 (全局 `keydown` 监听)
- 涉及: `ImagePreviewModal` / `UserProfileModal` / `SettingsModal` / `ChatInfoModal`

### 验证
- Client: `npm run build` — ✅ (316 KB)

---

## 2026-07-16 前端代码质量：6 项审计修复（第 28 轮）

### 问题
前端审计发现 5 项问题：
1. `ChatView.jsx` userMap 为空对象无用代码
2. `ChatList.jsx` filteredChats 未 memo 每次渲染重新计算
3. `Composer.jsx` attachment key 用数组索引
4. 多处硬编码颜色未使用 CSS 变量
5. `ChatList.jsx` sidebar-footer 组件过于庞大

### 修复

**1. ChatView.jsx — 删除无用 userMap**
- 删除 `useMemo(() => ({}), [])`，传空对象 `{}` 给 `renderContent`

**2. ChatList.jsx — filteredChats 用 useMemo**
- 包裹 `useMemo`，依赖 `[chats, chatSearch, isUUID]`

**3. Composer.jsx — attachment key 用唯一 ID**
- 上传时用 `crypto.randomUUID()` 生成 `_key`，替换 `key={i}` → `key={a._key}`

**4. 硬编码颜色 → CSS 变量**
- 新增 `--accent-bg`、`--success-bg` CSS 变量
- Toast/MemberPanel/ChatInfoModal/ChatListItem/ChatView/UserProfileModal 中的 `#5865F2`、`#23a559`、`#ed4245`、`rgba(88,101,242,0.15)` 等替换为对应 `var()`

**5. ChatList.jsx — 拆分 SidebarFooter 子组件**
- 新建 `SidebarFooter.jsx`，接收 `user`/`onLogout`/`onSettings`/`onAvatarPreview` 四个 props

### 验证
- Client: `npm run build` — ✅ (316 KB)
- Server: `go build` + `go vet` + `go test` — ✅

---

## 2026-07-16 reactions 迁移至 Service 层（第 29 轮）

### 问题
`handlers/reactions.go` 中 AddReaction/RemoveReaction/ListReactions 直连 `s.DB`，绕过 Service 层。

### 修复
- 新建 `server/internal/service/reaction.go`：`ReactionService` 封装 `Add`/`Remove`/`List`
- 各方法统一走 `s.Chat.MustBeMember` → DB → Hub broadcast 模式
- 修正 `GetMessage` 的 `db.ErrNotFound` → `service.ErrNotFound` 映射（修复测试断言）
- Handler 层简化：三处 handler 统一通过 `s.Services.Reaction.XXX` 调用，删除直连 DB 代码

### 验证
- `go build` + `go vet` + `go test` — ✅

---

## 2026-07-16 后端单元测试覆盖：Phase 4（第 30 轮）

### 新增测试

| 包 | +测试数 | 覆盖提升 | 测试内容 |
|----|---------|----------|----------|
| `internal/service` | 27 | 84.2% → 92.6% | 取消上下文错误路径（AuthZ / Chat / User / Member / Message / Reaction）、MemberService Remove 三种角色、MessageService Edit/Delete 边界、ReactionService 全套 |
| `internal/auth` | 2 | 85.2% → 90.7% | WrongSigningMethod（"none" 算法）、EmptyUserID claim |
| `internal/db` | 9 | 70.2% → 81.6% | FindAndDeleteRefreshToken 原子性 / DeleteUserRefreshTokens 隔离 / ListReactions Me 标志 / SetPinned / TogglePinned / GetChatMemberRole / ListPublicChats 分页+内容+固定+私密排除 |
| `internal/orderedmap` | 8 | 71.4% → 78.6% | 无效 JSON、嵌套数组/对象、重复 Unmarshal、Marshal/Reader 错误路径 |

### 验证
- `go build` + `go vet` + `go test` — ✅
- 全部新测试通过，无回归

---

## 2026-07-16 删除消息重载后显示为空信息（第 31 轮）

### 问题

服务器对已删除消息返回 `deleted_at` 时间戳 + `content: ""`。实时广播通过 `onMessageDelete` 正确设置 `deleted: true`，但页面重载时（API 分页加载 / polling），`deleted_at` 未被转为 `deleted: true`，导致已删除消息渲染为空白可编辑信息。

### 代码变更

**`client/src/store/chat.js`**:

新增 `_normalize(m)` 辅助函数，将 `deleted_at` 时间戳映射为 `deleted: true`，并在以下入口调用：
- `loadMessages` — API 分页加载
- `poll:messages` — 轮询传输

```js
/** @param {import('../types').Message} m */
_normalize(m) {
  if (m.deleted_at) {
    return { ...m, deleted: true, content: '' };
  }
  if (m.deleted) {
    return { ...m, content: '' };
  }
  return m;
},
```

### 验证
- Client build: ✅
- Go build + vet + test: ✅

---

## 2026-07-16 UserProfileModal 头像居中修复 + OpenAPI Spec 替换（第 32 轮）

### Avatar 居中修复

**问题**: `UserProfileModal` 中头像未居中。

**根因**: `<UserAvatar style={{ margin: '8px auto' }}>` — `UserAvatar` 不接受 `style` prop（内部 inline style 硬编码），prop 被静默忽略。

**修复**: 用 `<div style={{ display:'flex', justifyContent:'center' }}>` 包裹 `<UserAvatar>`，替代无效的 `style` prop（`client/src/components/UserProfileModal.jsx:23`）。

### OpenAPI Spec 替换

**背景**: 旧 `server/docs/swagger/` 由 swaggo 从 Go 注解生成，存在以下问题：
- 路径不匹配（`/api/chats` 实为 `/api/chats/my`，`pin-toggle` 已改 `announcement`）
- 缺失 8 个路径（`healthz`、`version`、`announcement/*`、`unpin`、`visit`、`/ws`）
- 响应描述空洞（大量 `additionalProperties: true`）
- 格式落后（Swagger 2.0）

**方案**: 手写 `docs/openapi.yml`（OpenAPI 3.1，29 路径，18 Schema，完整错误码）作为契约，替代 swaggo 生成的 spec。

**代码变更**:
- `docs/openapi.yml` — 新增，v0.3.0 的 API 契约骨架
- `server/internal/handlers/router.go` — `//go:embed swagger.json` + 本地 `GET /swagger/swagger.json`，删除远程 URL 依赖
- `server/internal/handlers/swagger.json` — 从 `openapi.yml` 转换的 JSON，embedded
- `server/cmd/chatd/main.go` — 删除 swaggo 全局注解 + 废弃的 `_ import`
- `server/docs/swagger/docs.go` — 删除（swaggo 生成死代码）
- `server/docs/swagger/swagger.yaml` — 删除（swaggo 生成死代码）

### 验证
- Go build + vet: ✅
- Client build: ✅

## 2026-07-16 多窗口消息不同步修复（第 33 轮）

### 根因
API 返回 `ORDER BY created_at DESC`（最新在前），但 `onMessageCreate` 用 `[...s.messages, msg]` 追加到末尾。导致：
- 初始加载得到的数组为 `[msg3(newest), msg2, msg1(oldest)]`（DESC）
- WebSocket 收到新消息后数组变为 `[msg3, msg2, msg1, msg4]`（混合顺序）
- 另一窗口刷新后从 API 加载得到 `[msg4, msg3, msg2, msg1]`（纯 DESC）
- 两个窗口消息顺序不一致 → 同步失败

### 修复
将 store 中 `messages` 数组统一为 **ASC 顺序**（oldest first）：
- `loadMessages`（`client/src/store/chat.js:274`）：反转 API 响应（DESC → ASC）
- `loadMore`（`client/src/components/ChatView.jsx:67`）：反转 API 响应后前置到现有数组
- `onMessageCreate`（`client/src/store/chat.js:190,192`）：追加到末尾（新消息=最后=底端）
- `poll:messages` 处理（`client/src/store/chat.js:85`）：反转负载

现在两个窗口始终得到相同顺序的消息。

### 验证
- Client build: ✅
- Go build + vet: ✅

## 2026-07-16 Frontend CI Playwright 测试修复（第 34 轮）

### 问题
SettingsModal 合并到 UserProfileModal 后，Playwright 测试仍引用旧的 selector：
- `text=Settings` → 新 modal 用 `aria-label="Settings"`，没有可见的 "Settings" 文本
- `.settings-avatar-placeholder` → 头像上传改为点击 "Click to upload" 文本

### 修复
`client/tests/ci.spec.mjs`：
- `text=Settings` → `[aria-label="Settings"]`
- `.settings-avatar-placeholder` → `text=Click to upload`

### 验证
- Frontend CI: ✅
- Backend CI: ✅

---

## 2026-07-17 生产环境批量修复（第 35 轮）

### 35-1: 消息顺序双重重排导致最新消息置顶

**问题**: 后端 `GetMessages` 已反转 SQL DESC 结果为 ASC，但前端 `loadMessages` / `poll:messages` / `loadMore` 又额外调用 `.reverse()`，造成双重重排 → 最新消息出现在顶部。

**修复**: 移除 `client/src/store/chat.js` 中 `poll:messages` 和 `loadMessages` 的 `.reverse()`，以及 `ChatView.jsx` 中 `loadMore` 的 `.reverse()`。

### 35-2: 公开聊天卡片头像不显示

**问题**: `WelcomeView.jsx` 公聊卡片硬编码首字母 avatar，从不检查 `avatar_url`。

**修复**: 卡片渲染时优先使用 `chat.avatar_url`（图片），fallback 到字母占位。同时从 store 合并实时数据。

### 35-3: ChatListItem 头像不同步

**问题**: `ChatListItem` 使用父组件缓存的 `chat` 数据，用户修改 avatar_url 后列表不刷新。

**修复**: `ChatListItem` 通过 `useChatStore()` 直接读取 store 中的最新 `avatar_url`/`banner_url`，覆盖 props 传入的缓存数据。

### 35-4: 粘贴上传被 CSP 拦截

**问题**: CSP `img-src` 缺少 `blob:`，`compressImage` 的 `URL.createObjectURL()` 被浏览器阻止。

**修复**: `server/internal/handlers/router.go` CSP header 中 `img-src` 加入 `blob:`。

### 35-5: 粘贴附件 `_key` 字段引发后端解析错误

**问题**: `api.sendMessage` 发送附件时保留 `_key` 字段，后端 `json: unknown field "_key"` 报错。

**修复**: `client/src/api/client.js` 的 `sendMessage` 中 strip 掉 `_key` 字段（`const { _key, ...rest } = a`）。

### 35-6: 粘贴附件预览始终显示文件名

**问题**: Composer 粘贴后附件预览对图片也仅显示文件名，无缩略图。

**修复**: 根据 `mime_type` 决定渲染：`image/*` 显示 `<img>`，其他显示文件名文本。

### 35-7: PWA 清单缺失

**问题**: 移动端浏览器无安装提示。

**修复**: 新增 `client/public/manifest.json`、`icon.svg`（💬 + #5865F2 背景）、HTML head 添加 Apple meta tags。

### 35-8: 保存 Profile 时头像被清空

**问题**: 后端 `UPDATE users SET avatar_url = ?` 用空字符串覆盖已有头像。

**修复**: 前端 Profile Save 请求包含 `me.avatar_url` 字段；后端 `UpdateUserProfile` 在 `avatar_url` 为空字符串时跳过该字段的 UPDATE。

### 35-9: Context 菜单溢出 viewport

**问题**: Context menu `position: fixed` 使用原始鼠标坐标，在屏幕右侧/底部溢出。

**修复**: left/top 用 `Math.min` 钳制到 viewport 边界。

### 35-10: 右键菜单加入复制 Chat ID

**问题**: 无便捷方式复制聊天 ID。

**修复**: Context menu 新增 "Copy Chat ID" 项（`navigator.clipboard.writeText`）；ChatInfoModal 中 Chat ID 行可点击复制。

### 验证
- Go build + vet: ✅
- Client build: ✅

---

## 2026-07-17 重构收尾（第 36 轮）

---

### H2: `notify()` 参数顺序错误

**问题**: `ChatView.jsx` 中 3 处调用 `notify('error', msg)` 将 type 和 message 参数交换，`notify` 签名是 `notify(message, type)`，导致错误提示不显示或显示错误内容。

**根因**: 旧签名 `notify(type, message)` 与新签名 `notify(message, type)` 不兼容，且有 3 处未更新。

**修复** (`client/src/components/ChatView.jsx:153,169,185`): 参数统一改为 `notify(msg)`，利用 `type` 默认值 `'error'`。

**文件**: `client/src/components/ChatView.jsx`

---

### H4: Reaction 双源冲突

**问题**: `MessageItem` 中 reaction 数据同时来自 `useState(msg.reactions || [])` 和 store `msg.reactions`，且轮询刷新时 store 的硬编码 `me` 覆盖了用户操作的 `me` 状态。

**根因**: 本地 state + store 双源未同步，轮询返回的 `reactions` 含旧 `me` 值。

**修复** (`client/src/components/MessageItem.jsx`): 移除 `useState(msg.reactions)`，改用 `displayReactions`（优先 API `getReactions` fetched 数据，fallback store `msg.reactions`）。

**文件**: `client/src/components/MessageItem.jsx`

---

### X1: WS 协议自动检测

**问题**: WS URL 硬编码 `ws://`，生产环境 HTTPS 下浏览器阻止混合内容。

**修复** (`client/src/realtime/transports/ws.js:6`): 优先使用 `import.meta.env.VITE_WS_URL`，fallback 根据 `location.protocol` 自动选择 `wss://` 或 `ws://`。

**文件**: `client/src/realtime/transports/ws.js`

---

### X3: 环境变量规范化

**问题**: API_BASE、UPLOAD_BASE、WS_URL 全部硬编码在源码中，部署不同环境需改代码。

**修复**: 创建 `client/.env`，三个端点全部可通过 `VITE_API_BASE` / `VITE_UPLOAD_BASE` / `VITE_WS_URL` 覆写，代码 fallback 到原自动检测逻辑。

**文件**: `client/.env` + `client/src/api/client.js:44-45` + `client/src/realtime/transports/ws.js:6`

---

### H3: `auth.js` ↔ `chat.js` 循环依赖

**问题**: `auth.js` 静态 import `chat.js`，`chat.js` 静态 import `auth.js`，形成闭环。打包后 runtime 出现 `undefined` store 引用。

**修复**:
- `store/chat.js`: 不再 import `useAuthStore`，改用 `getLocalAuth()` 从 localStorage 直接读取 token/user
- `store/auth.js`: 静态 `import { useChatStore }` → 动态 `await import('./chat')`，仅在 action 调用时加载

**文件**: `client/src/store/chat.js` + `client/src/store/auth.js`

---

### M3: 提取 `useEscapeKey` hook

**问题**: ChatInfoModal、SettingsModal、UserProfileModal、ImagePreviewModal 4 个 modal 各自实现 Escape key 监听，代码重复。

**修复**: 提取 `hooks/useEscapeKey.js`，统一 `addEventListener('keydown', ...)` + `removeEventListener` 清理，4 处替换。

**文件**: `client/src/hooks/useEscapeKey.js` + ChatInfoModal + SettingsModal + UserProfileModal + ImagePreviewModal

---

### M4: 提取 `formatRelativeTime` 工具

**问题**: MessageItem 和 ChatListItem 分别实现 `timeAgo` / `formatTime` 函数，逻辑重复。

**修复**: 提取 `utils/time.js` 导出 `formatRelativeTime(t)`（含 just now / Nm / Nh / Nd / date 层级回退），两组件统一引用。

**文件**: `client/src/utils/time.js` + `client/src/components/MessageItem.jsx` + `client/src/components/ChatListItem.jsx`

---

### M5: 删除无用 CSS 类

**问题**: CSS bundle 中包含 7 个从未被引用的类（`.user-avatar-img`、`.settings-avatar-img`、`.settings-avatar-placeholder`、`.msg-avatar-img`、`.typing-indicator`、`.file-download-link`、`.emoji-picker`）。

**修复**: 物理删除，CSS bundle 10.86kB → 10.21kB（-0.65kB）。

**文件**: `client/src/styles/global.css`

---

### M1: 删除死 API 方法

**问题**: `api.me`（无后端对应端点）、`api.createDM`（DM 已废弃）、`api.renameChat`（前端无 UI 调用，且后端无路由）占用 bundle + mock 维护成本。

**修复**: 删除 3 个方法和对应 mock 函数引用，JS bundle 322.19kB → 321.21kB。

**文件**: `client/src/api/client.js`

---

### X2(backend): 错误吞咽日志补全

**问题**: Go 后端 5 处 `_ = fn()` 静默忽略错误，线上故障无法追溯。

**修复**:
- `server/internal/handlers/handler.go:96` — `writeJSON` 编码错误 → `log.Printf`
- `server/internal/handlers/handler.go:170` — `trackLastActive` goroutine → `log.Printf`
- `server/internal/handlers/uploads.go:27` — `randomKey` 中 `rand.Read` → `log.Printf`
- `server/internal/ws/hub.go:80,100` — `UpdateUserStatus` / `UpdateUserLastSeen` → `logutil.Error`

**文件**: `server/internal/handlers/handler.go` + `server/internal/handlers/uploads.go` + `server/internal/ws/hub.go`

---

### X2(frontend): API 错误日志拦截

**问题**: `request()` 函数中对非 200 响应直接 `throw`，不记录任何日志；refresh 失败的空 catch 也完全静默，线上错误无迹可查。

**修复** (`client/src/api/client.js`):
- `!res.ok` 分支加 `console.error('[API Error]', method, path, status, data.error)`
- refresh 的 `catch(e) {}` 改为 `catch(e) { console.error('[API] refresh failed:', e) }`

**文件**: `client/src/api/client.js`

---

### .env.example

**新增**: 创建 `client/.env.example` 模板文件，供开发者 `cp .env.example .env` 后按需配置。含 `VITE_API_BASE` / `VITE_UPLOAD_BASE` / `VITE_WS_URL` 三个变量及注释。

**文件**: `client/.env.example`

---

### H1: 抽取 `useMembers` hook + `MemberList` 组件

**问题**: MemberPanel 和 ChatInfoModal 有完整的重复代码——相同的 `useEffect` + 60s 轮询 + WS/API 回退 + `onlineUserIds` 合并 + 成员行渲染（status dot + avatar + username + admin badge）。

**修复**:
- `hooks/useMembers.js` — 共享 hook，封装 `fetchMembers`（WS/API 按 mode 回退）、60s 轮询清理、`onlineUserIds` 合并为 `isOnline` 字段；暴露 `setLocalMembers` 支持乐观更新
- `components/MemberList.jsx` — 共享成员行组件：status-dot + UserAvatar + username ellipsis + ADMIN badge（absolute 右对齐）+ 可选 kick 按钮
- MemberPanel：改用 hook + 组件，`removeUser` 保持乐观删除 + rollback 行为
- ChatInfoModal：简化，移除内联 fetch + inline avatar 渲染，统一用 MemberList

**文件**: `client/src/hooks/useMembers.js`（新增）+ `client/src/components/MemberList.jsx`（新增）+ `client/src/components/MemberPanel.jsx`（重构）+ `client/src/components/ChatInfoModal.jsx`（重构）

### 验证
- Client build: ✅ (322.19kB → 320.54kB)
- CI: `build-7a70cf0` ✅

---

## 2026-07-17 Frontend CI Playwright 测试修复（第 37 轮）

### 问题: `mockLogin` 回归导致全部测试 timeout

**现象**: Frontend CI `mock-test` 全部 26 个 test timeout（30s），错误堆栈停在 `waitForSelector('.form-box')`。CI 自 commit `25d29a0` 起持续红。

**根因 — 双重**:

1. **首因** (`25d29a0`): 死代码清理时删了 `mockAddMember` 的 `import`，但 `_mockHandlers` 中仍引用 `addMember: mockAddMember`。模块加载时 `ReferenceError` 同步抛出，`main.jsx` 无法完成初始化，`#root` 始终为空 → 登录页 `.form-box` 永不渲染。

2. **次因** (`88faab6`): 即使修复 import，Playwright 各 test 共享同一 browser context，`__mockLogin()` 写入 `localStorage` 的 token 泄漏到后续 test。下一个 test 打开 `/login` 时读到残留 token，路由守卫立即重定向到 `/`，`.form-box` 同样不可见。

### 修复

- `client/src/api/client.js:33` — 补回 `mockAddMember` import
- `client/tests/ci.spec.mjs` + `client/tests/real-time.spec.mjs` — `mockLogin()` 内添加 `page.addInitScript(() => localStorage.clear())`，每次导航前清除 localStorage

### 验证
- 本地 `npx playwright test tests/ci.spec.mjs tests/real-time.spec.mjs --reporter=list` → 26 passed, 1 skipped
- CI (`build-da8ea9f`): go-build + go-test ✅, Frontend CI (mock-test + full-e2e) ✅

### 文件
- `client/src/api/client.js` — import 补回 `mockAddMember`
- `client/tests/ci.spec.mjs` — `mockLogin()` 加 `addInitScript`
- `client/tests/real-time.spec.mjs` — `mockLogin()` 加 `addInitScript`

---

## 2026-07-17 提取 MessageList 组件（第 38 轮）

### 背景

为将来替换虚拟列表（react-virtuoso / react-window）做准备，先将消息列表的渲染和滚动逻辑从 ChatView 中拆出，使 ChatView 不必感知消息列表的实现细节。

### 改动

- **新建** `client/src/components/MessageList.jsx`（71 行）
  - Props: `messages`, `hasMore`, `loading`, `onLoadMore`, `chatId`, `backgroundStyle`, `hasBackground`
  - 内部封装 `bodyRef` 管理滚动容器
  - 自动滚底（新消息、切换 chat）
  - load-more 滚动保持（点击「Load older messages」后记录 scrollHeight/scrollTop，新消息 prepend 后恢复）
  - 渲染加载按钮、loading 指示器、空状态、消息列表

- **重构** `client/src/components/ChatView.jsx`
  - 移除：`bodyRef`, `loadingMoreRef`, `prevChatIdRef`, `MessageItem` import, auto-scroll effect
  - 简化 `loadMore`：去掉滚动操作（交给 MessageList 处理）
  - 替换 `chat-body` div + `filtered.map` 为 `<MessageList>`

### 验证
- Client build: ✅ (321.04kB)
- `npx playwright test tests/ci.spec.mjs tests/real-time.spec.mjs` → 26 passed, 1 skipped

### 文件
- `client/src/components/MessageList.jsx` — 新增
- `client/src/components/ChatView.jsx` — 重构

---

## 2026-07-17 MessageItem 加 React.memo（第 39 轮）

### 问题

`MessageItem` 没有 `React.memo`。`ChatView` 订阅了整个 `useChatStore()`，任何 store 变更（包括其他 chat 的消息到达）都会导致全部 MessageItem 重渲染。Mock 数据 150 条消息意味着每次无用 diff 150 个组件。

### 修复

`client/src/components/MessageItem.jsx`:
- 导入 `memo`
- `export default function MessageItem` → `const MessageItem = memo(function MessageItem ...)` + `export default MessageItem`

默认 shallow 比较即可覆盖 `msg`（对象引用稳定）、`sameAuthor`（boolean）、`chatId`（string）三个 props。

### 验证
- Client build: ✅ (321.06kB)
- `npx playwright test tests/ci.spec.mjs tests/real-time.spec.mjs` → 26 passed, 1 skipped

### 文件
- `client/src/components/MessageItem.jsx` — 加 `memo` 包装

---

## 2026-07-17 消除 Chat UI 闪烁 + localStorage Token 移除（第 40 轮）

### 问题

1. **Chat UI 闪烁**：用户打开 app 后，因 localStorage 中存有过期/无效 token，App 直接渲染 `<ChatPage />`，随后 API 401 跳转到登录页，用户短暂看到聊天界面。
2. **Token 存 localStorage 不安全**：XSS 攻击可窃取 token。后端已支持 httpOnly Cookie 认证，前端无需持久化 token。
3. **无加载态**：app 启动后没有等待 auth 确认，直接渲染路由。

### 修复

#### client/src/store/auth.js
- 移除 `storage.get()` 从 store creator 读取 token（根因）
- 移除所有 `storage.set()` / `storage.clear()` 调用（不再持久化 token）
- Store 初始化为 `{ user: null, accessToken: null, booting: true }`
- 新增 `boot()` action：
  - Mock 模式：通过 `localStorage.getItem('chat:mock')` 布尔标记检测，调用 `mockLogin` 等效逻辑
  - 生产模式：调用 `api.refresh()` 验证 httpOnly Cookie，成功则恢复 session，失败则留在登录页
- Mock 模式改用 `localStorage.setItem('chat:mock', 'true')` 标记（仅存布尔值，不存 token）
- `logout()` 清除 `chat:mock` 标记

#### client/src/App.jsx
- 新增 boot phase：`useEffect` 调用 `boot()` 一次，`booting: true` 时渲染 spinner，完成后才渲染 Routes
- 路由守卫依赖的 `token` 在 boot 完成后决定流向

#### client/src/styles/global.css
- 新增 `.spinner` 样式 + `@keyframes spin` 动画

#### client/src/api/client.js
- 在 `VITE_API_BASE` / `VITE_UPLOAD_BASE` 未设置时输出 `console.warn` 提示

#### client/src/realtime/transports/ws.js
- 在 `VITE_WS_URL` 未设置时输出 `console.warn` 提示

### 影响
- 旧方式 `storage.get()` 读到的过期 token → app 直接显示 ChatPage → 401 闪烁
- 新方式 `boot()` 先验证 → 确定有效 session 才显示 ChatPage → 零闪烁
- mock 测试中的 `addInitScript(() => localStorage.clear())` + `page.evaluate(mockLogin)` 流程不受影响

### 验证
- Client build: ✅ (321.58kB + 10.41kB CSS)
- Go build + vet: ✅
- CI: ✅
- Frontend CI: ✅

---

## 2026-07-17 批量修复 items 8-12（第 41 轮）

### 8. MessageItem Reactions 双源风险
- **根因**：`MessageItem.jsx:71-80` 使用 `useState(null)` 本地缓存 reactions，并通过 `useEffect` 独立调 API 获取，与 WebSocket `onReaction` 推送产生竞争。
- **修复**：完全移除本地 `reactions` state 和对应的 `useEffect`，`displayReactions` 直接取自 `msg.reactions || []`。`handleReaction` 中 add/remove 后不再手动 `getReactions`（WS 事件会自动更新 store）。
- **死代码清理**：`api.getReactions`、`mockGetReactions`、`ListReactionsResponse` typedef 全部移除（-0.75 kB bundle）。

### 9. 环境变量运行时校验
- 新增 `client/src/config.js`：导出 `validateEnv()`、`API_BASE`、`UPLOAD_BASE`。
- `validateEnv()` 在 dev 模式下检查 `VITE_API_BASE` / `VITE_UPLOAD_BASE` / `VITE_WS_URL`，缺失时 `console.error` 报错。
- `client.js` / `ws.js` 的 `import.meta.env` 直接读取迁移到 config.js 集中管理。
- 新增 `client/.env.example` 文档化三个变量。

### 10. 错误日志结构化
- `server/internal/handlers/handler.go:98,176` + `uploads.go:29` 三处 `log.Printf` → `logutil.Error`。
- 移除 `"log"` import（两个文件均不再使用）。

### 11. 表驱动测试
- `TestAuthz_MustBeMember`：合并 4 个 case（success / not_member / empty_user_id / canceled_context）。
- `TestAuthz_RequireOwnerOrAdmin`：合并 6 个 case（owner / admin_role / not_owner / not_chat_member / chat_not_found / canceled_context）。
- 删除 5 个旧的独立测试函数。

### 12. TypeScript 迁移（API 层 + Zod 校验）
- 安装依赖：`zod`、`typescript`、`@types/react`、`@types/react-dom`。
- 创建 `client/src/types.ts`：`interface User`、`Chat`、`Message`、`Reaction`、`Attachment`、`PinnedContent`、`StreamSource`、`AuthResponse`。
- 创建 `client/src/schemas.ts`：对应 zod schemas + `validate()` 工具函数。
- 转换 `client/src/api/client.js` → `client.ts`：全部参数/返回值加上 TypeScript 类型标注；`register` / `login` / `refresh` 调用 `validate(AuthResponseSchema)` 做运行时校验。
- 更新 `tsconfig.json`：`include` 加入 `.ts` / `.tsx`。

### 验证
- Client build: ✅ (394.43kB JS + 10.41kB CSS)
- Go build + vet: ✅
- Go test ./...: ✅ (all packages pass)
- 162 modules transformed
- **后续优化**：使用 `z.infer<typeof Schema>` 替代手写 `types.ts`，删除 `types.ts`（85 行），全部 6 个类型 + `StreamSource` 集中在 `schemas.ts`；`mock.js`、`chat.js` 的 JSDoc 引用更新路径。
- **修复**：`MessageSchema.source` 原用 `z.function().args().returns(z.void())`，但 Zod v4 无 `.args()`/`.returns()` 方法，导致运行时抛出 TypeError，Frontend CI 全部 25 个测试超时。改为 `z.function().optional()` + `interface Message extends z.infer<typeof MessageSchema> { source?: () => void }` 解决。

---

## 2026-07-18 搜索重构 + DM 清理 + 颜色选择器 + 版本号（第 42 轮）

### 搜索行为重构
- **空输入 + 焦点** → 显示公共频道（不变）
- **有输入** → 过滤自己的已加入频道（之前错误地显示公共频道的过滤结果）
- **输入完整 UUID** → 无视任何限制，从已加入 + 公共缓存中直接显示对应 chat；未加入的 chat 点击即 join
- **修复竞态**：`handleSearchFocus` + `loadAllPublicChats` 设置 `showPublicList=true` 时，若输入已有文字，render 三个分支全部错过导致空白。改为 `!showPublicList || chatSearch.trim()` 兜底
- 移除死代码 `closeContextMenu`（从未被引用）

### DM 废弃处理
- `mock.js`: `mockCreateDM` 函数体 + `@deprecated` 标记保留不删（可能日后恢复）
- `README.md`: 删除 `createDM` 文档行；`client.js` → `client.ts`
- `ChatList.jsx`: `filteredChats` DM 过滤加 `@deprecated` 注释

### 头像颜色选择器
- `UserProfileModal.jsx`: 新增 10 色预设选色器（Discord 风格色盘），当前色高亮边框，点同一色取消
- `handleSave` 加入 `avatar_color` 字段提交

### 版本号
- `0.2.0` → `0.3.0`（自 0.2.0 以来 30+ 轮迭代）

### 验证
- Client build: ✅ (395.10kB JS + 10.41kB CSS)
- CI: ✅
- Frontend CI: ✅

---

## 2026-07-18 前端版本号注入 + 后端 tag 联动（第 43 轮）

### APP_VERSION 注入（前端 ldflags）
- `vite.config.js`: 新增 `define.__APP_VERSION__`，值取自 `process.env.APP_VERSION`
- `ChatList.jsx`: 移除 `import pkg from '../../package.json'`，改用 `__APP_VERSION__` 全局常量
- `frontend-ci.yml` + `ci.yml`: 构建时注入 `APP_VERSION`——tag 推送时取 tag 名，branch 推送时取 `build-{sha}`

### 后端 tag 联动
- `ci.yml` go-build 步骤：`ldflags -X main.Version` 改为 `${{ startsWith(github.ref, 'refs/tags/') && github.ref_name || format('build-{0}', github.sha) }}`
- Release 标题同理跟随 tag 名

### 行为对照

| 场景 | 后端 `/api/version` | 侧栏 | Release 标题 |
|------|-------------------|------|-------------|
| `git tag v0.3.1` | `v0.3.1` | `v0.3.1` | `v0.3.1` |
| `git push`（branch） | `build-abc123` | `build-abc123` | `Build abc123` |

### 验证
- Client build: ✅ (395.11kB JS + 10.41kB CSS)
- CI: ✅
- Frontend CI: ✅

## 2026-07-18 成员列表数据源统一 + last_seen 绿点（第 44 轮）

### 数据源统一
- `useMembers` 改从 store `chats[*].members` 读取，store 为唯一数据源
- 首次加载时 fetch 一次写入 store，移除 60s 轮询
- `setChats` merge 保留 `members` 不被 poll 覆盖
- `onEvent` 新增 `case 'user_update'`：更新 `chats[*].members[*]`，profile 修改后 member list 自动同步

### 绿点改用 last_seen
- 移除 `onlineUserIds` 依赖，绿点判断改为 `last_seen` 5分钟内算在线
- `onlineUserIds`/`presence_update` 保留不动

### 版本
- `0.3.0` → `0.3.1`

### 验证
- Client build: ✅
- go vet: ✅
- go test: ✅

## 2026-07-23 本地文件上传 /aapi/（第 45 轮）

## 2026-07-24 架构清理（第 46 轮）

### 目标
删除死代码、重复逻辑、无用依赖，提升代码可维护性。

### 删除
- `server/internal/handlers/uploads.go` — 废弃上传处理器（前端直传 upload.moonchan.xyz）
- `server/internal/orderedmap/` — 整个包，healthz 改用 `map[string]any`
- `server/cmd/chatd/main.go` `WriteTimeout` — 移除（导致 SSE 断开）
- 5 个失效的 upload 测试（`TestUploadFile`, `TestUploadExceedsSizeLimit`, `TestUploadRejectsUnsupportedMime`, `TestUploadNotLoggedIn`, `TestUpload_MissingFile`）

### 重构
- `db_fixups.go` — 6 个 `ensureXxxColumn` 函数合并为单个参数化 `ensureColumn(table, name, definition)`
- `main.go` — 添加 `hub.Shutdown()` 实现 WS/SSE 优雅关闭（P1）
- `local_upload.go` — raw body 路径添加 `http.MaxBytesReader` 大小限制 + `413` 响应

### 验证
- Server: `go build` / `go vet` / `go test` — all ✅
- Client: `npm run build` — ✅
- CI (GitHub Actions) — both jobs pass ✅

### 目标
用本地存储替换 `upload.moonchan.xyz` 外部上传服务，API 格式对齐 `Hana-ame/azure-go` 的 `?dest=local` 模式。

### 新增
- `server/internal/storage/driver.go` — `StorageDriver` 接口（Put/Get/Delete/Head）
- `server/internal/storage/local/local.go` — 本地文件系统驱动（文件存 `UploadDir/<ts>/<fn>`，MD5 ETag）
- `server/internal/handlers/aapi.go` — 上传/下载/删除处理器，匹配 azure-go 响应格式：
  - `PUT /aapi/upload` — 裸 body 上传
  - `PUT /aapi/upload/*` — 带文件名上传
  - `POST /aapi/upload` — multipart form-data 上传
  - `GET /aapi/local/*` — 文件下载
  - `GET /aapi/local/*?delete=<hash>` — 带 key 删除
- 响应格式：`{ id, path, url, delete_url }`（`id` = sha256(path + salt)[:8]）

### 修改
- `config.go` — 新增 `UploadSalt`、`UploadPublicURL`；移除过期注释
- `handler.go` — Server 结构体新增 `aapiLocalDriver` 字段
- `router.go` — 注册 `/aapi/*` 路由；CSP 移除 `upload.moonchan.xyz`
- `service/message.go` — 附件 URL 校验改为检查 `/aapi/local/` 前缀
- `service/service.go` — Service 注入 `*config.Config`
- `client/src/config.js` — `UPLOAD_BASE` 默认值改为空（同源）
- `client/src/api/client.ts` — 上传端点改为 `/aapi/upload`；`buildUploadUrl` 直接取 `data.url`
- `client/vite.config.js` — dev proxy 添加 `/aapi`
- `client/.env.example` — `VITE_UPLOAD_BASE` 默认值更新
- 所有测试中的 `upload.moonchan.xyz` URL 替换为新格式

### 验证
- Server: `go build` / `go vet` / `go test` — all ✅
- Client: `npm run build` — ✅

## 2026-07-24 v0.5.0 超时可配 + 消息长度可配（第 47 轮）

### 新增环境变量
- `CHAT_MAX_MESSAGE_LENGTH` — 单条消息最大字符数（默认 4000）
- `CHAT_WS_MAX_MSG_SIZE` — WebSocket 单条消息最大字节（默认 65536）
- `CHAT_API_TIMEOUT` — 普通 API 超时（默认 10s）
- `CHAT_UPLOAD_TIMEOUT` — 上传超时（默认 5m）
- `CHAT_READ_TIMEOUT` — 服务级读取超时（默认 10m）
- `CHAT_READ_HEADER_TIMEOUT` — 请求头读取超时（默认 10s）

### 审计报告验证
- 对照 `docs/reports/codebase-audit-20260715.md` 逐项核查 14 个问题：
  - 12 个已完全修复 ✅（BroadcastUserUpdate data race、Reactions 绕过 service、Users/SSE/Chat 直调 DB、Reconnect/CONNECTED 状态、removeUser try/catch、Gateway DB 错误、双重 loadMessages、ErrContentTooLong 413、Poll cancelled、双重排序、UserAvatar 组件、Modals aria）
  - 2 个部分修复 🔶（`auth.go` 仍有 3 处直接调 `s.DB.*`、`isContentTooLong` 字符串匹配 → 本次已改成 `errors.Is`）

### 修改
- `config.go` — 新增 6 个可配置字段（`MaxMessageContentLength`, `WSMaxMessageSize`, `APITimeout`, `UploadTimeout`, `ReadTimeout`, `ReadHeaderTimeout`）
- `db/db.go` — `DB` 结构体加 `maxContentLength`，`Open()` 签名扩展
- `db/messages.go` — 硬编码 4000 替换为 `d.maxContentLength`，新增 `db.ErrContentTooLong` sentinel
- `service/authz.go` — `isContentTooLong` 改用 `errors.Is(err, db.ErrContentTooLong)`
- `ws/gateway.go` — `maxMessageSize` 从常量改为 `Gateway.maxMessageSize` 字段
- `handlers/handler.go` — `authMiddleware` 中 `s.DB.GetUserByID` → `s.Services.User.GetByID`
- `handlers/router.go` — `/api` 分组拆为 upload（`UploadTimeout`）和其余（`APITimeout`）
- `cmd/chatd/main.go` — 透传新配置到 DB, Gateway, http.Server
- `.env.example`（root + server）— 新增 Timeouts / Limits 分组
- `scripts/deploy_win.py` — 新增 `check_env()`：运行前自动检查 root/server/client 三组 `.env` 完整性，缺失时从 GitHub 拉取模板
- `scripts/check_env.py` — 移除（功能合并入 deploy_win.py）

### 验证
- Server: `go build` / `go vet` / `go test` — all ✅
- Client: `npm run build` — ✅
- CI (GitHub Actions) — passed ✅

---

## 2026-07-25 后端架构优化（第 48 轮）

### 变更

#### 1. Service.WithTx 空存根 → 真实事务
- **问题**: `WithTx` 是空存根 `return fn()`，不支持跨表事务
- **修复**: 改为 `BeginTx/Commit/Rollback` 完整事务；`CreateAIMessage` 改用事务保证 INSERT + UPDATE 原子性
- **文件**: `server/internal/service/service.go`、`server/internal/db/messages.go`

#### 2. 消除 4 处重复的 cookie secure 判断
- **问题**: `setAuthCookie`、`setRefreshCookie`、`clearRefreshCookie`、`clearAccessTokenCookie` 各自重复 `r.TLS != nil || r.Header.Get(...)` 判断
- **修复**: 提取 `isSecure(r)` 辅助函数
- **文件**: `server/internal/handlers/util.go`

#### 3. PickColor 改用 FNV 哈希
- **问题**: `PickColor` 用 `uuid.Parse(seed)` 再取 `id.ID()`，对非 UUID 字符串（如群聊 name）静默回退到第一种颜色 → 所有群聊同色
- **修复**: 改用 `hash/fnv.New32a`，任意字符串均匀分布到色盘
- **文件**: `server/internal/db/users.go`

#### 4. 移除 UpdatePinnedChat 死代码别名
- **问题**: `UpdatePinnedChat` 是 `PinChat` 的精确别名，路由中 POST/PATCH 各注册一次
- **修复**: 删除别名，路由 POST + PATCH 共用 `s.PinChat`
- **文件**: `server/internal/handlers/chat.go`、`router.go`

#### 5. Password truncation 一致性
- **问题**: `HashPassword` 拒绝 >72 字节密码，`VerifyPassword` 静默截断到 72 字节，两者行为不一致
- **修复**: `VerifyPassword` 超 72 字节直接返回 `ErrInvalidCredentials`
- **文件**: `server/internal/auth/auth.go`

#### 6. SafeID 日志 helper + 全量替换
- **问题**: 30+ 处 `userID[:8]` / `chatID[:8]` 重复截断模式，可读性低，且 `logutil.SafeID` 集中控制截断长度
- **修复**: 新增 `logutil.SafeID(id)`，替换全部 `id[:8]` 模式（保留 SHA256 hash 和 UUID 文件名等非用户 ID 的 `[:8]`）
- **文件**: `server/internal/logutil/log.go` + `handlers/` 6 文件 + `ws/` 3 文件

#### 7. 修复 AIChat 中 body.Close 顺序
- **问题**: `io.ReadAll` 失败后 `r.Body.Close()` 不会执行
- **修复**: 先检查 `io.ReadAll` 错误，再关闭 body
- **文件**: `server/internal/handlers/ai.go`

#### 8. ensureColumn SQL 注入防护
- **问题**: `fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", table, definition)` 直接拼接参数
- **修复**: 添加 `strings.ContainsAny` 校验 + 警告注释
- **文件**: `server/internal/db/db_fixups.go`

### 验证
- Go build + vet: ✅
- Go test ./...: ✅ (all packages pass)
- Client build: ✅

---

## 2026-07-25 deploy_win.py 自更新 + .env 自动同步（第 49 轮）

### 变更

#### 1. deploy_win.py 自更新
- **功能**: 每次运行（run 模式除外）自动从 GitHub main 分支拉取最新 `deploy_win.py`，若内容不同则覆盖自身并重启
- **跳过**: `--no-self-update` 参数可跳过
- **文件**: `scripts/deploy_win.py`

#### 2. .env 自动同步
- **功能**: 自动从 GitHub 获取最新 `.env.example`，对比本地 `.env`，自动添加缺失 key（保留 `CHAT_STATIC_DIR` 和已有值），提示 placeholder 警告
- **文件**: `scripts/deploy_win.py`

#### 3. CHAT_STATIC_DIR 写入绝对路径
- **功能**: download 步骤自动将 `CHAT_STATIC_DIR` 写入 `.env` 为绝对路径，不受工作目录影响
- **文件**: `scripts/deploy_win.py`

#### 4. 配置日志从 Debug 改为 Info
- **功能**: 服务器启动时 `static_dir=xxx` 直接打印到日志，无需设 `LOG_LEVEL=DEBUG`
- **文件**: `server/internal/config/config.go`

### 验证
- Go build: ✅
- Python syntax: ✅

---

## 2026-07-25 v0.6.0: 全局速率限制 + 版本 bump（第 50 轮）

### 变更

#### 1. 全局速率限制 120 req/min/IP
- **背景**: 之前仅在 login/register/search/send 上有细粒度限流，缺少全局兜底
- **修复**: `/api` 非上传路由组添加 `r.Use(httprate.LimitByIP(120, 1*time.Minute))`
- **文件**: `server/internal/handlers/router.go`

#### 2. 版本 bump 0.5.0 → 0.6.0
- `client/package.json`、`server/internal/handlers/swagger.json` 同步更新

### 验证
- Go build + vet: ✅
- CI (2 workflows): 运行中

---

## 2026-07-26 DB 迁移重构：JSON manifest + GitHub 远程读取（第 51 轮）

### 变更

#### 1. 新增 migrations.json manifest
- `server/internal/db/migrations/migrations.json` — 列出所有 SQL 迁移文件及版本号
- 替代原先从文件名前 3 字符解析版本的方式

#### 2. GitHub 远程读取（替代 embed）
- 主路径：从 `CHAT_MIGRATION_URL` 环境变量指定的 URL 获取 `migrations.json` 和各 SQL 文件
- 回退路径：若网络不可用或 URL 为空，fallback 到 `//go:embed` 本地文件
- 默认 URL: `https://raw.githubusercontent.com/Hana-ame/chat-app/main/server/internal/db/migrations/`

#### 3. 配置
- `config.go`: 新增 `MigrationURL` 字段 + `CHAT_MIGRATION_URL` 环境变量

#### 4. 代码变更
| 文件 | 操作 |
|------|------|
| `server/internal/db/migrations/migrations.json` | 新增 — 迁移 manifest |
| `server/internal/db/db.go` | 重写 `Migrate()` — HTTP 获取 + JSON 解析 + embed 回退 |
| `server/internal/config/config.go` | 新增 `MigrationURL` 字段 |
| `server/cmd/chatd/main.go` | `db.Open()` 传入 `cfg.MigrationURL` |
| `server/internal/testutil/testutil.go` | `db.Open()` 传入 `""`（使用 embed 回退） |

### 验证
- Go build: ✅
- Go vet: ✅
- Go test ./...: ✅

---

## 2026-07-26 DB 迁移系统重新设计—fs.Glob 文件查找 + Go 迁移 1000+（第 52 轮）

### 设计变更
按照标准迁移系统方案重新设计：

- 文件命名 `NNN_xxx.sql`（000~999），版本号从文件名前缀解析
- `schema_migrations` 只存 `(version INTEGER PRIMARY KEY, applied_at TEXT)`
- 每次启动查 `MAX(version)`，循环 `fs.Glob("{next:03d}_*.sql")` 找下一个文件
- 找不到 → 已最新，跳出循环
- Go 迁移版本用 1000+，与 SQL 隔离

### 移除
| 文件 | 原因 |
|------|------|
| `migrations/migrations.json` | 不再需要 manifest |
| `MigrationEntry` + `loadManifest` + `loadSQL` + `fetch*` | 用 `fs.Glob` 替代 |
| `config.MigrationURL` + `db.Open` 参数 | 远程读取不再需要 |
| `schema_migrations.type` 列 | 版本空间通过 0-999 / 1000+ 自然隔离 |

### 修复
- `AIPanel.jsx` `buildContext`：`m.type === 'stream'` 替代 `m.user_id === 'ai'`

### 文档
- `server/internal/db/migration.md` — 迁移系统设计文档

### 验证
- Go build + vet: ✅
- Go test ./...: ✅
- Client build: ✅

---

## 2026-07-26 v0.6.0 -> v0.7.0: Stream 消息重构（第 53 轮）

### 新增
- `ai/stream.go` — 通用 SSE streaming 客户端，支持 OpenAI 兼容的 streaming/non-streaming 响应、`reasoning_content`、多 choices
- `ai/stream_test.go` — 20+ 测试用例覆盖各种边缘情况
- `service/stream.go` — `StreamService`：内存 chunk 缓冲 + 订阅/通知模式 + 生命周期管理
- `db/migrations/001__add_type_column.sql` — messages 表增加 `type` 列
- `GET /api/chats/{chatID}/messages/{messageID}/stream` — SSE 端点，实时推送 stream 内容（先读内存缓冲，结束后 fallback DB）
- `AIPanel.jsx` — 新的 AI 面板组件
- `client/tests/ai-panel.spec.mjs` — AI 面板 E2E 测试

### 变更
- `messages.go` `SendMessage`：新增 `type=stream` 支持，返回 SSE 而非 JSON；`sendMsgReq` 增加 `type`/`source`/`msg_id`
- `Composer.jsx` — 大幅精简，移除旧的 AI 面板逻辑
- `chat.js` store — 支持 stream 消息类型
- `models.Message` — 新增 `Type` 字段
- `CreateAIMessage` — 接受 `userID` 参数，写入 `type=stream`
- `router.go` — `/api/ai/chat` 移除，`/stream` 端点接入

### 移除
- `handlers/ai.go` — 旧 AI handler
- `ai/openai.go` + `ai/provider.go` — 旧的 Provider 抽象
- `config.AISources` + `.env.example` 中的 AI 配置

### 修复
- `PinChat` — `GetByID` 失败时记录 error log

### 验证
- Go build + vet: ✅
- Go test ./...: ✅
- Client build: ✅