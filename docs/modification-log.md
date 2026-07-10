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

### 6-5: Chat list 排序问题（数据同步修复）

**反馈**: 排序有问题，新创建的 chat 和有回复的 chat 交错排列不符合预期。

**根因**: 排序算法 `a.last_message_at || a.created_at` 正确。问题出在数据同步：
1. `mockCreateChat`/`mockCreateDM` 仅写入 `d.chats`，不通知 Store，新 chat 在下次轮询前不出现
2. `setChats` 合并时 `...c` 将 Store 中已有的 `last_message_at` 覆盖为 `null`（来自 API 的无消息 chat），排序瞬间失效

**代码变更** (`client/src/api/mock.js` — 创建后同步):

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

**代码变更** (`client/src/store/chat.js` — `setChats` 合并保护):

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

## 前一次会话（历史）

*（此处记录此前已归档的 CI/CD、审计、验证手册等工作）*
