# frontend-ui-spec 逐行审计修正报告

> 审计日期：2026-07-10
> 审计方法：将 `frontend-ui-spec-20260710.md` 每一条声明与 16 个源文件逐行对照
> 源文件：ChatList.jsx (229L), ChatListItem.jsx (55L), ChatView.jsx (206L), MessageItem.jsx (153L), Composer.jsx (100L), MemberPanel.jsx (90L), ChatInfoModal.jsx (92L), SettingsModal.jsx (45L), UserProfileModal.jsx (40L), renderContent.jsx (36L), ScrollArea.jsx (12L), EmptyState.jsx (12L), WelcomeView.jsx (37L), CreateGroupForm.jsx (22L), DmSearchPanel.jsx, ChatPage.jsx (80L)

---

## 统计

| 严重性 | 数量 |
|--------|------|
| CRITICAL — 完全虚构 | 4 |
| HIGH — 显著行为差异 | 5 |
| MEDIUM — 描述不完整/有误导性 | 9 |
| LOW — 轻微不准确 | 7 |
| **合计** | **25** |

---

## CRITICAL — 完全虚构

### C1. 组件关系图编造不存在的组件（spec lines 647-669）

**声明：** 图中包含 `ChatArea`、`MemberArea` 包装组件，`ChatView` 包含 `WelcomeView`，有独立 `(streaming indicator)` 组件。
**实际：** 不存在 `ChatArea` 或 `MemberArea` 组件。`WelcomeView` 由 `ChatPage.jsx:75` 直接渲染（非 ChatView）。流式输出在 `MessageItem` 内部处理，无独立组件。
**修正：** 重写组件关系图，以 `ChatPage` 为根节点，直接列出各子组件的实际嵌套关系。

### C2. UserProfileModal "用 useEffect 模拟生命周期" 虚构（spec line 679）

**声明：** "UserProfileModal.jsx 依赖 3 个组件但自身用 `useEffect` 模拟生命周期"
**实际（UserProfileModal.jsx）：** 纯展示组件，40 行，无 useEffect、无 useState、无任何 hook。仅接收 `user` 和 `onClose` props 渲染 UI。
**修正：** 改为"被 3 个组件引用（MessageItem、MemberPanel、ChatInfoModal），纯展示组件无自身状态"。

### C3. EmptyState "创建按钮" 虚构（spec line 243）

**声明：** "在聊天列表为空时显示提示文案和创建按钮"
**实际（EmptyState.jsx）：** 纯展示组件，仅渲染 `message` 文本和可选 `icon`，无任何按钮。
**修正：** 改为"显示提示文案，可选显示图标。无操作按钮"。

### C4. WelcomeView 嵌套位置错误（spec line 657）

**声明：** 组件图显示 `ChatView → WelcomeView (no active chat)`
**实际（ChatPage.jsx:75）：** `WelcomeView` 由 `ChatPage` 渲染，当 `!activeChatId && !isMobile` 时显示。`ChatView` 不导入也不渲染 `WelcomeView`。
**修正：** 在组件图中将 `WelcomeView` 移到 `ChatPage` 下，与 `ChatView` 同级。

---

## HIGH — 显著行为差异

### H1. filteredChats DM 过滤描述有歧义（spec line 161）

**声明：** "过滤 DM 类型 → 按搜索词匹配名称/ID"
**实际（ChatList.jsx:117-118）：** `if (c.type === 'dm') return false;` — 排除 DM（不是过滤到 DM）。
**修正：** 改为"排除 DM 类型"。

### H2. ChatList 缺少 joinAction 快捷操作描述（spec lines 148-161）

**声明：** 函数表无 `joinAction` 相关描述
**实际（ChatList.jsx:32-35）：** `joinAction` 根据 `chatSearch` 内容自动判断用户意图（join/create），渲染快捷操作按钮"Join #id"/"Create name"。
**修正：** 在状态/派生值表中添加 `joinAction`，在使用场景中描述快捷操作。

### H3. ChatList 缺少 buildCount 状态（spec lines 134-146）

**声明：** 状态表无 `buildCount`
**实际（ChatList.jsx:111-115）：** `buildCount` 从 localStorage 读取/递增，显示在侧边栏标题 `+#{n}`。
**修正：** 在状态表中添加 `buildCount`。

### H4. ChatList 缺少模式切换按钮（spec lines 148-161）

**声明：** 无 WS/SSE/Poll 切换功能描述
**实际（ChatList.jsx:129-133）：** 有按钮循环切换 WS→SSE→Poll 模式。
**修正：** 在使用场景中添加"点击 WS/SSE/Poll 按钮切换实时模式"。

### H5. ChatView markRead 位置描述错误（spec line 304）

**声明：** "api.markRead 失败被静默吞掉"（暗示在 ChatView 中调用）
**实际：** `markRead` 在 `ChatPage.jsx:48` 调用，非 ChatView。ChatView 从 store 解构了 `markRead` 但未使用。
**修正：** 改为"markRead 在 ChatPage.jsx 中调用（非 ChatView）"。

---

## MEDIUM — 描述不完整/有误导性

### M1. ChatListItem 缺少 onContextMenu prop（spec lines 176-183）

**声明：** Props 表列出 `chat`, `activeId`, `onSelectChat`，无 `onContextMenu`
**实际（ChatListItem.jsx:15）：** 接收 4 个 props：`chat`, `activeId`, `onSelectChat`, `onContextMenu`
**修正：** 补充 `onContextMenu` prop。

### M2. handleCreate 错误处理遗漏（spec line 154）

**声明：** "校验名称非空 → 调用 `api.createChat()` → 关闭表单 → 选中新 chat"
**实际（ChatList.jsx:53）：** 失败时 `alert(e.message)`
**修正：** 补充"失败时 alert() 提示"。

### M3. handleJoinPublic 错误处理遗漏（spec line 158）

**声明：** 无错误处理描述
**实际（ChatList.jsx:86）：** 失败时 `alert(e.message)`
**修正：** 补充"失败时 alert() 提示"。

### M4. joinChatByID 错误处理遗漏（spec line 159）

**声明：** 无错误处理描述
**实际（ChatList.jsx:96）：** 失败时 `alert(e.message)`
**修正：** 补充"失败时 alert() 提示"。

### M5. handleDeleteChat 错误处理遗漏（spec line 155）

**声明：** 无错误处理描述
**实际（ChatList.jsx:58）：** 失败时 `console.error`
**修正：** 补充"失败时 console.error"。

### M6. handleTogglePin 错误处理遗漏（spec line 156）

**声明：** 无错误处理描述
**实际（ChatList.jsx:63）：** 失败时 `console.error`
**修正：** 补充"失败时 console.error"。

### M7. searchPublic 错误处理遗漏（spec line 157）

**声明：** 无错误处理描述
**实际（ChatList.jsx:76）：** 失败时 `console.error`
**修正：** 补充"失败时 console.error"。

### M8. handleTyping 描述不准确（spec line 389）

**声明：** "通知 Store 发送 typing 事件 → 防抖清除旧 timer"
**实际（Composer.jsx:25-29）：** `sendTyping(chatId)` 立即调用（非防抖），仅 timer 是防抖的。
**修正：** 改为"立即调用 sendTyping(chatId) → 清除旧 timer → 设新 2 秒 timer"。

### M9. Composer handleFile 错误处理遗漏（spec line 392）

**声明：** 无错误处理描述
**实际（Composer.jsx:60）：** 失败时 `alert(err.message || 'Upload failed')`
**修正：** 补充"失败时 alert() 提示"。

---

## LOW — 轻微不准确

### L1. timeAgo "月/日" 描述不准确（spec line 188）

**声明：** "月/日（其他）"
**实际（ChatListItem.jsx:12）：** `d.toLocaleDateString()` 格式因 locale 而异，不一定是"月/日"。
**修正：** 改为"`toLocaleDateString()`（格式因 locale 而异）"。

### L2. ChatPage 移动端 class 名称未说明（spec line 118）

**声明：** 仅提到 "shell class 使用 Flexbox 布局"
**实际（ChatPage.jsx:65）：** 移动端添加 `mobile-list` 或 `mobile-chat` 修饰 class。
**修正：** 补充移动端 class 名称说明。

### L3. handleSaveNotice 错误处理遗漏（spec line 291）

**声明：** 无错误处理描述
**实际（ChatView.jsx:105-108）：** 429 时弹 alert，其他错误 console.error。
**修正：** 补充错误处理说明。

### L4. handleClearNotice 错误处理遗漏（spec line 292）

**声明：** 无错误处理描述
**实际（ChatView.jsx:116-119）：** 429 时弹 alert，其他错误 console.error。
**修正：** 补充错误处理说明。

### L5. Composer AI 按钮 "Mock 模式" 描述不够明确（spec line 365）

**声明：** "点击 🤖 快捷发送消息给 AI（Mock 模式）"
**实际（Composer.jsx:91-95）：** 非 Mock 模式下弹 alert 提示"Enable mock first"。
**修正：** 改为"仅 Mock 模式可用，非 Mock 模式弹 alert 提示"。

### L6. ChatView "已知问题" 中 markRead 位置（spec line 304）

**声明：** 暗示 markRead 在 ChatView 中
**实际：** markRead 在 ChatPage 中
**修正：** 已在 H5 中修正。

### L7. Component tree 图中 Streaming 处理位置（spec line 660）

**声明：** `(streaming indicator)` 作为独立节点
**实际：** 流式输出在 MessageItem 内部通过 `msg.streaming` 和 `<span className="stream-cursor" />` 处理
**修正：** 已在 C1 中修正组件图。

---

## 修正前后对比（关键段落）

### 组件关系图

```diff
- ChatPage
- ├── Sidebar
- │   ├── ChatList
- │   │   ├── ChatListItem × N
- │   │   ├── CreateGroupForm (conditional)
- │   │   └── PublicChannelList (embedded)
- │   └── EmptyState (conditional)
- ├── ChatArea
- │   ├── ChatView
- │   │   ├── WelcomeView (no active chat)
- │   │   ├── MessageItem × N
- │   │   └── Composer
- │   └── (streaming indicator)
- └── MemberArea
-     └── MemberPanel
-         ├── member-item × N
-         └── UserProfileModal (conditional)
+ ChatPage (routes/ChatPage.jsx)
+ ├── ChatList (sidebar)
+ │   ├── ChatListItem × N
+ │   ├── CreateGroupForm (conditional)
+ │   ├── PublicChannelList (embedded)
+ │   ├── EmptyState (conditional, when chats.length === 0)
+ │   ├── ChatInfoModal (conditional)
+ │   └── SettingsModal (conditional)
+ ├── ChatView (main area, when activeChatId set)
+ │   ├── MessageItem × N
+ │   │   └── UserProfileModal (conditional)
+ │   └── Composer
+ ├── WelcomeView (when no activeChatId on desktop)
+ └── MemberPanel (right panel, when activeChatId && !isMobile)
+     └── UserProfileModal (conditional)
```

### EmptyState 功能概述

```diff
- 纯展示组件，无状态。在聊天列表为空时显示提示文案和创建按钮。
+ 纯展示组件，无状态。在聊天列表为空时显示提示文案，可选显示图标。无操作按钮。
```

### UserProfileModal 跨组件问题

```diff
- | 2 | UserProfileModal.jsx 依赖 3 个组件但自身用 useEffect 模拟生命周期 | UserProfileModal | 低 |
+ | 2 | UserProfileModal.jsx 被 3 个组件引用（MessageItem、MemberPanel、ChatInfoModal），纯展示组件无自身状态 | UserProfileModal | 低 |
```

### filteredChats

```diff
- | filteredChats | 派生数据：过滤 DM 类型 → 按搜索词匹配名称/ID → 返回过滤后的聊天列表 |
+ | filteredChats | 派生数据：排除 DM 类型 → 按搜索词匹配名称/ID → 返回过滤后的聊天列表 |
```

### handleTyping

```diff
- | handleTyping() | 通知 Store 发送 typing 事件 → 防抖清除旧 timer → 设新 timer（setTimeout 回调为空函数，实际不发送停止输入信号） |
+ | handleTyping() | 立即调用 sendTyping(chatId) 通知 Store → 清除旧 timer → 设新 2 秒 timer（回调为空函数，实际不发送停止输入信号） |
```

---

## 教训总结

1. **组件关系图是最容易编造的部分。** AI 倾向于生成"看起来合理"的层级结构（Sidebar/ChatArea/MemberArea），但实际代码没有这些包装组件。ChatPage 直接渲染 ChatList、ChatView、MemberPanel。

2. **纯展示组件容易被误认为有状态。** UserProfileModal 被 3 个组件引用，AI 就推断它"依赖 3 个组件"并"用 useEffect 模拟生命周期"，但它实际上是一个 40 行的纯函数组件。

3. **错误处理模式容易被忽略。** ChatList 中 8 个函数都有错误处理（alert 或 console.error），但 spec 只描述了成功路径。

4. **"立即"和"防抖"要区分清楚。** `sendTyping` 是立即调用的，只有 timer 是防抖的。Spec 说"防抖发送 typing 事件"是不准确的。

5. **嵌套关系要从渲染方确认。** WelcomeView 看起来"应该"在 ChatView 内部，但实际是 ChatPage 直接渲染的。必须从 `import` 和 JSX 使用处确认。
