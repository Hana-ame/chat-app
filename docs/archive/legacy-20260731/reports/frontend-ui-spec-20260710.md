# 前端 UI 层规范

> 审计日期：2026-07-10
> 源码：`client/src/components/`、`client/src/styles/`
> 依赖：React 19、CSS custom properties (global.css)

---

## 目录

- [组件架构概览](#组件架构概览)
- [顶层组件](#顶层组件)
  - [ChatPage.jsx — 三栏布局骨架](#chatpagejsx--三栏布局骨架)
- [Sidebar 组件](#sidebar-组件)
  - [ChatList.jsx — 聊天列表](#chatlistjsx--聊天列表)
  - [ChatListItem.jsx — 单条聊天项](#chatlistitemjsx--单条聊天项)
  - [CreateGroupForm.jsx — 新建群组表单](#creategroupformjsx--新建群组表单)
  - [PublicChannelList.jsx — 公开频道列表](#publicchannellistjsx--公开频道列表)
  - [EmptyState.jsx — 空状态占位](#emptystatejsx--空状态占位)
- [ChatView 组件](#chatview-组件)
  - [ChatView.jsx — 聊天主视图](#chatviewjsx--聊天主视图)
  - [MessageItem.jsx — 单条消息](#messageitemjsx--单条消息)
  - [Composer.jsx — 输入框](#composerjsx--输入框)
  - [WelcomeView.jsx — 欢迎视图](#welcomeviewjsx--欢迎视图)
- [MemberPanel 组件](#memberpanel-组件)
  - [MemberPanel.jsx — 成员面板](#memberpaneljsx--成员面板)
  - [DmSearchPanel.jsx — DM 搜索（已废弃）](#dmsearchpaneljsx--dm-搜索已废弃)
- [Modal 组件](#modal-组件)
  - [ChatInfoModal.jsx — 聊天信息](#chatinfomodaljsx--聊天信息)
  - [SettingsModal.jsx — 设置](#settingsmodaljsx--设置)
  - [UserProfileModal.jsx — 用户资料](#userprofilemodaljsx--用户资料)
- [通用组件](#通用组件)
  - [ScrollArea.jsx — 自定义滚动容器](#scrollareajsx--自定义滚动容器)
  - [renderContent.jsx — 消息内容渲染器](#rendercontentjsx--消息内容渲染器)
- [样式系统](#样式系统)
  - [icon_color — 头像颜色](#icon_color--头像颜色)
  - [CSS 设计体系](#css-设计体系)
- [组件关系图](#组件关系图)

---

## 组件架构概览

```
client/src/components/
├── ChatList.jsx            ← 侧边栏：聊天列表 + 搜索
├── ChatListItem.jsx        ← 单条聊天项（名称、头像、unread badge）
├── ChatView.jsx            ← 主视图：消息列表 + 输入框 + 已读条
├── MessageItem.jsx         ← 单条消息（气泡、头像、reaction、编辑、pin）
├── Composer.jsx            ← 富文本输入框 + Markdown 预览 + 附件
├── MemberPanel.jsx         ← 右栏：成员列表 + 公开频道
├── ChatInfoModal.jsx       ← 聊天详情弹窗（信息 + 成员管理）
├── SettingsModal.jsx       ← 设置弹窗（用户名、头像、登出）
├── UserProfileModal.jsx    ← 用户资料弹窗
├── CreateGroupForm.jsx     ← 新建群组表单（嵌入 ChatList）
├── PublicChannelList.jsx   ← 公开频道列表（嵌入 ChatList）
├── DmSearchPanel.jsx       ← DM 搜索面板（⚠️ 已废弃/未被引用）
├── EmptyState.jsx          ← 无聊天时的占位提示
├── WelcomeView.jsx         ← 首次访问欢迎提示
├── ScrollArea.jsx          ← 自定义滚动区域（无外部依赖）
└── renderContent.jsx       ← 消息内容解析器（@mention + url 自动链接）
```

**关键模式：** 组件之间通过 Zustand store 通信，无 props drilling（除少数受控模式）。

---

## 顶层组件

---

### `ChatPage.jsx` — 三栏布局骨架

**位置：** 整个页面骨架，三栏布局容器（sidebar 300px / chat flex-1 / member-panel 240px），全屏高度
**父组件：** 无（路由入口，由 React Router 加载）
**子组件：** ChatList、ChatView、WelcomeView、MemberPanel
**逻辑层引用：** `useAuthStore` → 获取 accessToken（管理连接生命周期）；`useChatStore.setActiveChat()` → 同步 URL 路由与活跃聊天状态；连接管理层（WS/SSE/Polling）→ 建立/断开实时通信

> 位于 routes/ 但承担核心布局职责，故归入 UI 层。

**功能概述：** ChatPage 是登录后的主页面，负责三栏布局（sidebar / chat / member-panel）、实时连接生命周期管理、URL 路由与 Store 状态同步、移动端视图切换。

**使用场景：** 用户登录后跳转到此页面，自动建立实时连接并加载聊天列表；点击聊天时 URL 变为 `/g/:chatId`，加载该聊天的消息；发送消息后自动滚动到最新消息；切换聊天时断开旧连接、建立新连接、加载新消息；移动端侧边栏占满屏幕，点击聊天后切换到消息视图。

**职责：**
1. 三栏布局：`sidebar (300px) | chat (flex-1) | member-panel (240px)`
2. 管理实时连接生命周期（连接/断开/重连）
3. URL 路由 ↔ store 状态同步（`/g/:chatId`）
4. 移动端视图切换（list ↔ chat）

**函数：**

| 函数 | 类型 | 职责 |
|------|------|------|
| `useEffect([accessToken])` | 生命周期 | 连接生命周期：进入页面时建立 WS/SSE/Polling 连接并加载聊天列表，离开时断开连接 |
| `useEffect([urlChatId, accessToken])` | 生命周期 | URL → Store 同步：URL 中的 `:chatId` 变化时调用 `setActiveChat()` 切换活跃聊天 |
| `useEffect([activeChatId, accessToken])` | 生命周期 | 消息加载 + 已读标记：切换聊天时加载历史消息，标记最后一条消息为已读 |
| `setMobileView(view)` | 状态更新 | 移动端视图切换：`'list'` 显示侧边栏，`'chat'` 显示消息视图 |

**移动端适配：**
```javascript
const [mobileView, setMobileView] = useState('list'); // 'list' | 'chat'
const [isMobile, setIsMobile] = useState(window.innerWidth < 768);

useEffect(() => {
  const onResize = () => setIsMobile(window.innerWidth < 768);
  window.addEventListener('resize', onResize);
  return () => window.removeEventListener('resize', onResize);
}, []);
```

**布局结构：**
```html
<div className={appClass}>  <!-- appClass = 'shell' + mobile modifiers -->
  <ChatList onSelectChat={...} activeId={...} onLogout={...} />
  {activeChatId ? <ChatView chatId={...} onBack={...} /> : isMobile ? null : <WelcomeView />}
  {!isMobile && activeChatId && <MemberPanel chatId={...} />}
</div>
```

`shell` class 使用 Flexbox 布局（`display:flex; height:100%`），三栏通过 `sidebar` / `main` / `member` CSS 类分配宽度。移动端添加 `mobile-list`（显示侧边栏）或 `mobile-chat`（显示消息视图）修饰 class。

---

## Sidebar 组件

---

### `ChatList.jsx` — 聊天列表

**位置：** 左侧栏（sidebar），固定宽度 300px，全屏高度；内部包含聊天列表、搜索框、新建群组表单、公开频道搜索结果、右键菜单、设置弹窗入口
**父组件：** ChatPage
**子组件：** ChatListItem、CreateGroupForm、PublicChannelList、EmptyState、ChatInfoModal、SettingsModal
**逻辑层引用：** `useAuthStore` → 获取 user、accessToken；`useChatStore` → 获取聊天列表、切换实时模式（setMode）；`api.createChat()` → 新建群组；`api.deleteChat()` → 删除聊天；`api.togglePin()` → 切换置顶；`api.listPublicChats()` → 搜索公开频道；`api.joinChat()` → 加入频道；`api.updateProfile()` → 保存设置；`api.upload()` → 上传头像

**功能概述：** ChatList 是侧边栏的核心组件，负责展示用户加入的聊天列表、搜索过滤、新建群组、右键菜单操作（Pin/Unpin、View Info、Delete）、用户设置入口。它集成了 CreateGroupForm、PublicChannelList、SettingsModal、ChatInfoModal 四个子组件/弹窗。

**使用场景：** 用户登录后看到所有已加入的聊天；输入搜索词过滤聊天名称或输入 `join #id` / `create name` 快速操作；点击「+」按钮展开新建群组表单；点击 WS/SSE/Poll 按钮切换实时模式；右键聊天项弹出上下文菜单进行 Pin/查看信息/删除操作；点击底部用户头像打开设置弹窗。

**状态：**

| 状态 | 类型 | 用途 |
|------|------|------|
| `showCreate` | boolean | 控制新建群组表单的展开/收起 |
| `newChatName` | string | 新建群组的名称输入 |
| `newChatVisibility` | string | 新建群组的可见性（private/public/unlisted） |
| `chatSearch` | string | 搜索框输入内容 |
| `publicResults` | array \| null | 公开频道搜索结果 |
| `publicSearching` | boolean | 公开频道搜索加载状态 |
| `contextMenu` | object \| null | 右键菜单坐标 `{ chatId, x, y }` |
| `showSettings` | boolean | 控制设置弹窗的显示/隐藏 |
| `showChatInfo` | string \| null | 控制聊天信息弹窗（值为 chatId） |
| `buildCount` | number | 构建计数器（localStorage 持久化，显示在侧边栏标题 `+#{n}`） |

**派生值：**

| 值 | 类型 | 用途 |
|------|------|------|
| `joinAction` | 'join' \| 'create' \| null | 根据 `chatSearch` 内容自动判断用户意图：匹配 `join #id` 或纯数字 → 'join'；匹配 `create name` → 'create'；否则 null |

**函数：**

| 函数 | 职责 |
|------|------|
| `closeContextMenu()` | useCallback 包装的关闭右键菜单函数，避免每次渲染创建新引用 |
| `useEffect([contextMenu])` | 监听全局 click 事件，点击菜单外部时自动关闭右键菜单 |
| `handleCreate()` | 新建群组：校验名称非空 → 调用 `api.createChat()` → 关闭表单 → 选中新 chat。失败时 `alert()` 提示 |
| `handleDeleteChat(chatId)` | 删除聊天：弹出 confirm 确认 → 调用 `api.deleteChat()` → 关闭菜单。失败时 `console.error` |
| `handleTogglePin(chatId)` | 切换置顶：调用 `api.togglePin()` → 关闭菜单。Store 的 `onChatUpdate` 自动处理排序。失败时 `console.error` |
| `searchPublic(q)` | 搜索公开频道：调用 `api.listPublicChats()` 获取全部公开 chat → 按名称/ID 过滤 → 设置结果。失败时 `console.error` |
| `handleJoinPublic(chatId)` | 加入公开频道：调用 `api.joinChat()` → 刷新聊天列表 → 选中新 chat。失败时 `alert()` 提示 |
| `joinChatByID(chatId)` | 通过 ID 加入聊天：调用 `api.joinChat()` → 清空搜索 → 刷新聊天列表。失败时 `alert()` 提示 |
| `handleSaveSettings(name)` | 保存设置：检查头像文件 → 上传头像（如有）→ 调用 `api.updateProfile()` → 更新 Store 用户 → 关闭弹窗 |
| `filteredChats` | 派生数据：排除 DM 类型 → 按搜索词匹配名称/ID → 返回过滤后的聊天列表 |

**依赖：** `useAuthStore`（user, accessToken）、`useChatStore`（chats, mode, setMode）、`api`（全部 API 方法）。

---

### `ChatListItem.jsx` — 单条聊天项

**位置：** 左侧栏（sidebar）内的聊天列表项，作为 ChatList 的子条目渲染，每行显示头像、名称、最后消息预览、时间、未读 badge
**父组件：** ChatList
**子组件：** 无
**逻辑层引用：** 无直接引用（纯展示组件，数据通过 props 传入）

**功能概述：** ChatListItem 渲染单条聊天的头像、名称、可见性标签、最后消息预览、时间、未读 badge 和右键菜单按钮。它是 ChatList 的列表项子组件，每个聊天对应一个实例。

**使用场景：** 被 ChatList 的 `filteredChats.map()` 渲染，点击切换活跃聊天，右键弹出上下文菜单。

**Props：**

| Prop | 类型 | 用途 |
|------|------|------|
| `chat` | Chat | 聊天对象 |
| `activeId` | string | 当前活跃聊天 ID，用于高亮 |
| `onSelectChat` | (chatId) => void | 点击回调 |
| `onContextMenu` | ({chatId, x, y}) => void | 右键菜单回调（传递按钮坐标） |

**函数：**

| 函数 | 职责 |
|------|------|
| `timeAgo(t)` | 时间格式化：now（<1分钟）、Xm（<1小时）、Xh（<24小时）、`toLocaleDateString()`（其他，格式因 locale 而异） |
| `handleMenu(e)` | 右键菜单：阻止冒泡 → 获取按钮位置 → 调用 `onContextMenu` 传递坐标 |

**⚠️ 已知问题：** 头像颜色用 `chat.icon_color`（mock 的 `CHAT_COLORS` 轮询分配），不考虑用户自定义 `avatar_color`。

---

### `CreateGroupForm.jsx` — 新建群组表单

**位置：** 左侧栏（sidebar）内，ChatList 顶部「+」按钮点击后展开显示
**父组件：** ChatList
**子组件：** 无
**逻辑层引用：** 无直接引用（受控组件，数据与回调通过 props 传入）

**功能概述：** CreateGroupForm 是嵌入 ChatList 的表单组件，用于创建新的群组聊天。包含群组名称输入、可见性选择（private/public/unlisted）、创建和取消按钮。

**使用场景：** 用户点击 ChatList 顶部的「+」按钮展开此表单，输入名称后按 Enter 或点击 Create 创建群组。

**状态：** 无独立状态——名称和可见性由父组件 ChatList 通过 props 控制。

**Props：**

| Prop | 类型 | 用途 |
|------|------|------|
| `name` | string | 群组名称 |
| `visibility` | string | 可见性 |
| `onVisibilityChange` | (e) => void | 可见性变更回调 |
| `onNameChange` | (e) => void | 名称输入变更回调 |
| `onNameKeyDown` | (e) => void | 键盘事件（Enter 创建） |
| `onCreate` | () => void | 创建按钮回调 |
| `onCancel` | () => void | 取消按钮回调 |

---

### `PublicChannelList.jsx` — 公开频道列表

**位置：** 左侧栏（sidebar）内，ChatList 搜索框下方，搜索公开频道后显示结果列表
**父组件：** ChatList
**子组件：** 无
**逻辑层引用：** 无直接引用（受控组件，数据与回调通过 props 传入）

**功能概述：** PublicChannelList 展示搜索结果中的公开频道列表。列表项无加入状态标识，统一显示名称和成员数。仅在 ChatList 中使用。

**使用场景：** 在 ChatList 中搜索公开频道后显示结果列表，供用户点击加入。

**Props：**

| Prop | 类型 | 用途 |
|------|------|------|
| `results` | array \| null | 搜索结果（null 表示未搜索） |
| `searching` | boolean | 搜索加载状态 |
| `onJoin` | (chatId) => void | 加入频道回调 |

**⚠️ 注意：** 中英文混用（英文标题 "Public Channels" + 中文提示 "搜索中..." / "无结果"）。

---

### `EmptyState.jsx` — 空状态占位

**位置：** 左侧栏（sidebar）内，聊天列表为空（chats.length === 0）时显示
**父组件：** ChatList
**子组件：** 无
**逻辑层引用：** 无直接引用（纯展示组件）

**功能概述：** 纯展示组件，无状态。在聊天列表为空时显示提示文案，可选显示图标。无操作按钮。

**Props：**

| Prop | 类型 | 用途 |
|------|------|------|
| `message` | string | 提示文案 |
| `icon` | ReactNode | 可选图标 |

---

## ChatView 组件

---

### `ChatView.jsx` — 聊天主视图

**位置：** 中间主区域（flex-1），左侧栏与右侧成员面板之间；顶部显示聊天标题和 Notice Board，中间为消息列表，底部为 Composer 输入框
**父组件：** ChatPage
**子组件：** MessageItem、Composer
**逻辑层引用：** `useChatStore` → 获取消息列表、聊天数据；`api.listMessages()` → 分页加载历史消息；`api.setPinnedMessage()` → 保存 Notice Board；`api.clearPinnedMessage()` → 清除 Notice Board；WS 频道订阅 → 接收实时消息

**功能概述：** ChatView 是聊天区域的核心组件，负责消息列表渲染、自动滚动、加载更多历史消息、Notice Board（Pin 固定消息）管理、消息编辑/删除。它是最复杂的组件之一，包含滚动锚点逻辑、分页加载、实时消息追加。

**使用场景：** 用户切换聊天后，ChatView 加载并渲染该聊天的所有消息；用户滚动到顶部时点击「Load older messages」加载更多；Owner 可设置/编辑/清除 Notice Board；新消息到达时自动滚动到底部（除非用户正在向上浏览）。

**状态：**

| 状态 | 类型 | 用途 |
|------|------|------|
| `loading` | boolean | 加载更多历史消息的 loading 状态 |
| `hasMore` | boolean | 是否还有更多历史消息可加载 |
| `noticeInput` | string | Notice Board 编辑输入框的内容 |
| `isEditingNotice` | boolean | Notice Board 是否处于编辑模式 |

**Ref：**

| Ref | 用途 |
|-----|------|
| `bodyRef` | 消息容器 DOM 引用，用于滚动控制 |
| `loadingMoreRef` | 标记是否正在加载更多（防止自动滚动干扰） |
| `prevChatIdRef` | 记录上一次的 chatId，用于判断是否切换了聊天 |

**函数：**

| 函数 | 职责 |
|------|------|
| `getDMName(chat, currentUserId)` | 工具函数：DM 聊天取对方用户名（DM 已废弃，函数仅兼容遗留数据） |
| `useEffect([chatId, accessToken])` | 聊天切换：订阅 WS 频道 → 加载消息 → 重置 hasMore |
| `loadMore()` | 分页加载：记录当前滚动位置 → 调用 `api.listMessages(before)` → 前插消息 → 恢复滚动位置。429 时弹 alert |
| `useEffect([chatId, messages])` | 自动滚动：新消息到达或切换聊天时，若距底部 <300px 则滚动到底部 |
| `handleSaveNotice()` | 保存 Notice：校验非空 → 调用 `setPinnedMessage()` → 退出编辑模式。429 时弹 alert，其他错误 console.error |
| `handleClearNotice()` | 清除 Notice：调用 `clearPinnedMessage()` → 清空输入 → 退出编辑模式。429 时弹 alert，其他错误 console.error |

**自动滚动逻辑：**
```javascript
// 距底部 < 300px 时自动滚动（抵消 textarea 高度变化引起的 scrollTop 偏移）
if (isNewChat || (scrollHeight - scrollTop - clientHeight < 300)) {
  bodyRef.current.scrollTop = bodyRef.current.scrollHeight;
}
```

**⚠️ 已知问题：**
- `filtered` 过滤所有消息（`m.chat_id === chatId`），无虚拟列表，150+ 消息时性能差。
- `markRead` 在 ChatPage.jsx 中调用（非 ChatView），失败被静默吞掉。

---

### `MessageItem.jsx` — 单条消息

**位置：** 中间主区域（ChatView）的消息列表内，按时间顺序纵向排列，每条消息占一行
**父组件：** ChatView
**子组件：** UserProfileModal
**逻辑层引用：** `useAuthStore` → 获取当前用户（判断消息作者身份）；`api.editMessage()` → 编辑消息；`api.deleteMessage()` → 删除消息；`api.addReaction()` → 添加表情反应；`api.removeReaction()` → 移除表情反应

**功能概述：** MessageItem 渲染单条消息的完整内容：头像、用户名、时间戳、消息气泡（支持 @mention + URL 自动链接）、附件、Reaction 列表、编辑/删除操作、流式输出指示器。点击头像或用户名可弹出 UserProfileModal。每个消息对应一个实例。

**使用场景：** 被 ChatView 的 `filtered.map()` 渲染；用户 hover 消息时显示操作按钮；点击 reaction 可快速 toggle；点击 emoji 按钮打开 emoji 选择器；自己的消息可编辑/删除。

**Props：**

| Prop | 类型 | 用途 |
|------|------|------|
| `msg` | Message | 消息对象 |
| `sameAuthor` | boolean | 是否与上一条消息同作者（用于合并头像） |
| `chatId` | string | 当前聊天 ID |

**状态：**

| 状态 | 类型 | 用途 |
|------|------|------|
| `showEmoji` | boolean | emoji 选择器的显示/隐藏 |
| `editing` | boolean | 编辑模式的开关 |
| `editText` | string | 编辑输入框的内容 |
| `opPending` | boolean | 操作（编辑/删除）的 loading 状态 |
| `profileUser` | object \| null | UserProfileModal 显示的用户对象 |

**函数：**

| 函数 | 职责 |
|------|------|
| `timeFormat(t)` | 工具函数：将 ISO 时间戳格式化为 locale 时间（`toLocaleTimeString`，en-US 下为 `h:mm AM` 格式） |
| `author` (useMemo) | 派生数据：优先从 `chat.members`（live data）查找作者，fallback 到 `msg.author` 快照。自己的消息直接用 `useAuthStore().user` |
| `userMap` (useMemo) | 派生数据：构建 `{ userId: username }` 映射，供 `renderContent` 解析 `@mention` |
| `handleReaction(emoji)` | Reaction toggle：检测是否已有自己的 reaction → 调用 `addReaction` 或 `removeReaction` → 关闭 emoji 选择器 |
| `handleEdit()` | 编辑消息：校验非空 → 调用 `api.editMessage()` → 退出编辑模式 |
| `handleDelete()` | 删除消息：弹出 confirm → 调用 `api.deleteMessage()` |

**条件分支：**
```
msg.deleted     → 显示 "(message deleted)"
msg.streaming   → 显示内容 + 流式光标（无 renderContent）
editing         → 显示编辑输入框 + Save/Cancel
否则            → renderContent 解析 @mention + URL
```

**⚠️ 已知问题：**
- 编辑/删除操作用 `confirm()` 阻塞 UI。
- `COMMON_EMOJI` 只有 10 个预设 emoji，无自定义输入。

---

### `Composer.jsx` — 输入框

**位置：** 中间主区域（ChatView）底部，消息列表下方，固定于聊天区域底部
**父组件：** ChatView
**子组件：** 无
**逻辑层引用：** `api.upload()` → 上传附件；`sendMessage()` → 发送消息；`sendTyping()` → 发送打字状态通知

**功能概述：** Composer 是消息输入区域，包含多行文本框、附件上传、发送按钮、AI 快捷发送按钮。支持 Enter 发送、Shift+Enter 换行、自动高度调整、打字状态通知。

**使用场景：** 用户在 ChatView 底部输入消息；按 Enter 发送，Shift+Enter 换行；点击 📎 上传文件；点击 🤖 快捷发送消息给 AI（仅 Mock 模式可用，非 Mock 模式弹 alert 提示）；输入时自动通知其他人「正在输入」。

**状态：**

| 状态 | 类型 | 用途 |
|------|------|------|
| `text` | string | 输入框内容 |
| `uploading` | boolean | 文件上传的 loading 状态 |
| `attachments` | array | 待发送的附件列表 |

**Ref：**

| Ref | 用途 |
|-----|------|
| `fileInput` | 隐藏的 `<input type="file">` DOM 引用 |
| `typingTimer` | 打字通知的防抖 timer |
| `textRef` | textarea DOM 引用，用于自动高度调整 |

**函数：**

| 函数 | 职责 |
|------|------|
| `autoResize()` | useCallback：重置 textarea 高度为 auto → 设为 scrollHeight，实现多行自动扩展 |
| `useEffect([text])` | 文本变化时触发 autoResize |
| `handleTyping()` | 立即调用 `sendTyping(chatId)` 通知 Store → 清除旧 timer → 设新 2 秒 timer（回调为空函数，实际不发送停止输入信号） |
| `handleSend()` | 发送消息：校验内容或附件非空 → 调用 `sendMessage()` → 清空输入和附件 |
| `handleKey(e)` | 键盘事件：Enter（非 Shift）→ 阻止默认 + 发送；其他键 → 触发 typing 通知 |
| `handleFile(e)` | 文件上传：遍历选中文件 → 逐个调用 `api.upload()` → 追加到 attachments → 清空 input。失败时 `alert()` 提示 |

**附件处理：**
```javascript
// 逐个上传，收集结果
for (const f of files) {
  const data = await api.upload(f);
  results.push({ filename: data.filename, mime_type: data.mime_type, size: data.size, url: data.url });
}
```

**⚠️ 已知问题：**
- 无上传进度指示。
- `api.upload` 失败用 `alert()` 提示。

---

### `WelcomeView.jsx` — 欢迎视图

**位置：** 中间主区域（flex-1），未选择任何聊天时显示，桌面端独占中间栏
**父组件：** ChatPage
**子组件：** 无
**逻辑层引用：** 无直接引用（纯展示组件）

**功能概述：** 纯展示组件，无状态。在未选择任何聊天时显示 💬 图标、欢迎标题和说明段落，无操作按钮。

---

## MemberPanel 组件

---

### `MemberPanel.jsx` — 成员面板

**位置：** 右侧栏，固定宽度 240px，全屏高度；有活跃聊天且非移动端时显示，位于 ChatView 右侧
**父组件：** ChatPage
**子组件：** UserProfileModal
**逻辑层引用：** `useAuthStore` → 获取当前用户；`useChatStore` → 获取聊天数据、在线用户列表（onlineUserIds）；`api.searchUsers()` → 搜索用户；`api.addMember()` → 添加成员；`api.removeMember()` → 移除成员

**功能概述：** MemberPanel 是右侧栏组件，展示当前聊天的成员列表、在线状态指示、添加/移除成员功能。点击成员行可弹出 UserProfileModal 查看详细资料。

**使用场景：** 用户选择聊天后，右栏显示该聊天的所有成员；Owner 可搜索并添加新成员，或移除现有成员；点击成员行弹出用户资料卡。

**状态：**

| 状态 | 类型 | 用途 |
|------|------|------|
| `members` | array | 当前聊天的成员列表（从 chat.members 同步） |
| `adding` | boolean | 添加成员模式的开关 |
| `search` | string | 用户搜索输入 |
| `results` | array | 用户搜索结果 |
| `profileUser` | object \| null | UserProfileModal 显示的用户对象 |

**函数：**

| 函数 | 职责 |
|------|------|
| `useEffect([chat])` | 同步成员列表：chat 变化时从 `chat.members` 更新本地 state |
| `searchUsers(q)` | 搜索用户：调用 `api.searchUsers()` → 过滤已是成员的用户 → 设置结果 |
| `addUser(userId)` | 添加成员：调用 `api.addMember()` → 关闭添加模式 → 清空搜索 |
| `removeUser(userId)` | 移除成员：弹出 confirm → 调用 `api.removeMember()` |
| `isOnline(uid)` | 工具函数：检查用户 ID 是否在 `onlineUserIds` 列表中 |

**依赖：** `useChatStore`（chats, onlineUserIds）、`useAuthStore`（user, accessToken）、`api`（searchUsers, addMember, removeMember）。

---

### `DmSearchPanel.jsx` — DM 搜索（已废弃）

**位置：** 右侧栏（已废弃，未在任何组件中引用）
**父组件：** 无（已废弃，未被引用）
**子组件：** 无
**逻辑层引用：** 无（已废弃，未被引用）

**功能概述：** DM 搜索面板，搜索用户并创建 DM 聊天。DM 功能已废弃，该组件为死代码。

**⚠️ 未被引用：** 任何组件都没有 import 此文件。

---

## Modal 组件

---

### `ChatInfoModal.jsx` — 聊天信息

**位置：** 居中弹窗（Modal），覆盖在整个页面上层（modal-overlay），由 ChatList 的右键菜单「View Info」触发
**父组件：** ChatList
**子组件：** UserProfileModal
**逻辑层引用：** `useChatStore` → 获取聊天数据（成员列表、创建时间、所有者等元信息）

**功能概述：** ChatInfoModal 是聊天信息弹窗，展示聊天的基本信息（名称、创建时间、最后消息时间）和成员列表（按 Owner / Admin / Member 分组）。点击成员行可弹出 UserProfileModal。

**使用场景：** 用户右键聊天项 → 点击「View Info」→ 弹出此弹窗；查看聊天的创建时间、成员角色分组；点击成员行查看用户资料。

**Props：**

| Prop | 类型 | 用途 |
|------|------|------|
| `chatId` | string | 聊天 ID |
| `onClose` | () => void | 关闭弹窗回调 |

**状态：**

| 状态 | 类型 | 用途 |
|------|------|------|
| `profileUser` | object \| null | UserProfileModal 显示的用户对象 |

**函数：**

| 函数 | 职责 |
|------|------|
| `fmtTime(t)` | 工具函数：将 ISO 时间戳格式化为本地日期时间字符串 |
| `owner, admins, members` (useMemo) | 派生数据：将 `chat.members` 按角色分组为 Owner、Admin、Member 三个数组 |
| `Section` | 子组件：渲染带标题的分组区域（uppercase 小标题 + 子内容） |
| `MemberRow` | 子组件：渲染单个成员行（头像 + 用户名），点击触发 `onProfile` 回调 |
| `InfoRow` | 子组件：渲染键值对信息行（label + value，左右分布） |

**成员分组逻辑：**
```javascript
const o = chat.members.find(m => m.id === chat.owner_id);      // Owner
const a = chat.members.filter(m => m.role === 'admin' && m.id !== chat.owner_id);  // Admin
const m = chat.members.filter(m => m.id !== chat.owner_id && m.role !== 'admin');  // Member
```

---

### `SettingsModal.jsx` — 设置

**位置：** 居中弹窗（Modal），覆盖在整个页面上层，由 ChatList 底部用户头像/齿轮按钮触发
**父组件：** ChatList
**子组件：** 无
**逻辑层引用：** 无直接引用（受控组件，通过 `onSave` props 回调调用父组件的逻辑层操作）

**功能概述：** SettingsModal 是用户设置弹窗，允许修改用户名和上传头像。修改后点击 Save 调用父组件的 `onSave` 回调。

**使用场景：** 用户点击侧边栏底部头像或齿轮按钮 → 弹出此弹窗；修改用户名 → 点击 Save 保存；点击头像区域上传新头像。

**Props：**

| Prop | 类型 | 用途 |
|------|------|------|
| `user` | User | 当前用户对象 |
| `onClose` | () => void | 关闭弹窗回调 |
| `onSave` | (name) => Promise | 保存回调（父组件处理上传和 API 调用） |

**状态：**

| 状态 | 类型 | 用途 |
|------|------|------|
| `name` | string | 用户名输入框的内容 |
| `saving` | boolean | 保存操作的 loading 状态 |

**函数：**

| 函数 | 职责 |
|------|------|
| `handleSave()` | 保存设置：设 saving → 调用 `onSave(name)` → 清除 saving |

**⚠️ 已知问题：**
- 头像上传用 `document.getElementById('avatar-file-input')` DOM 耦合。
- Enter 键直接保存，无确认。

---

### `UserProfileModal.jsx` — 用户资料

**位置：** 居中弹窗（Modal），覆盖在整个页面上层，由 MessageItem 中点击头像/用户名、MemberPanel 或 ChatInfoModal 中点击成员行触发
**父组件：** MessageItem、MemberPanel、ChatInfoModal
**子组件：** 无
**逻辑层引用：** 无直接引用（纯展示组件，用户数据通过 props 传入）

**功能概述：** UserProfileModal 展示用户资料卡，包含头像、用户名、在线状态、邮箱。

**使用场景：** 在 MessageItem 中点击头像/用户名、在 MemberPanel 中点击成员行、在 ChatInfoModal 中点击成员行，均弹出此弹窗。

**Props：**

| Prop | 类型 | 用途 |
|------|------|------|
| `user` | User | 要展示的用户对象 |
| `onClose` | () => void | 关闭弹窗回调 |

---

## 通用组件

---

### `ScrollArea.jsx` — 自定义滚动容器

**位置：** 通用组件，包裹需要自定义滚动条的区域，无固定屏幕位置；在 sidebar、ChatView 消息列表等需要纵向滚动的容器中使用
**父组件：** ChatList、ChatView 等需要纵向滚动的容器
**子组件：** 无
**逻辑层引用：** 无直接引用（通用 UI 组件，仅 DOM 操作）

**功能概述：** ScrollArea 封装一个带自定义滚动条样式的 flex 滚动容器，通过 `style` prop 覆盖默认样式。

**Props：**

| Prop | 类型 | 用途 |
|------|------|------|
| `children` | ReactNode | 子内容 |
| `style` | object | 可选样式覆盖（默认 `flex:1, overflowY:auto, minHeight:0`） |
| `className` | string | 额外 CSS 类名 |

**⚠️ 已知问题：**
- 无虚拟列表支持。
- 无平滑滚动 API。

---

### `renderContent.jsx` — 消息内容渲染器

**位置：** 通用组件，在 MessageItem 的消息气泡和 ChatView 的 Notice Board 内部调用，无固定屏幕位置
**父组件：** MessageItem、ChatView（Notice Board）
**子组件：** 无
**逻辑层引用：** 无直接引用（纯渲染函数，仅处理字符串到 React 元素的转换，不调用任何 API 或 Store）

**功能概述：** renderContent 将纯文本消息解析为 React 元素，仅处理两种模式：(1) `<@uuid>` 格式的 @mention 通过 userMap 映射为 `<span class="mention">@用户名</span>`，(2) `https?://` URL 自动链接化为 `<a>` 标签。无 markdown 或 emoji 解析。

**使用场景：** MessageItem 和 ChatView 的 Notice Board 中调用 `renderContent(msg.content, userMap)` 渲染消息内容。

**解析流程：**
```
文本 → 按 MENTION_RE (/(<@[a-f0-9-]{36}>)/) 分割：
  - 匹配 <@uuid> → 查 userMap → <span class="mention">@用户名</span>
  - 未匹配段 → 按 URL_RE (/(https?:\/\/[^\s<>[\]{}|\\^`]+)/g) 分割：
    - 匹配 URL → <a href="..." target="_blank">
    - 未匹配 → 纯文本
```

---

## 样式系统

---

### `icon_color` — 头像颜色

ChatListItem 的头像背景色来自 `chat.icon_color`，回退为 `'#5865F2'`。颜色值由 `mock.js` 中的 `CHAT_COLORS` 数组（15 色）在 mock 数据中轮询分配：

```javascript
const CHAT_COLORS = [
  '#5865F2', '#23a559', '#f0b232', '#ed4245', '#9b59b6',
  '#1abc9c', '#e67e22', '#2ecc71', '#e74c3c', '#3498db',
  '#f39c12', '#1dd1a1', '#a29bfe', '#fd79a8', '#00cec9',
];
```

ChatInfoModal 的 `MemberRow` 使用 `member.avatar_color || '#5865F2'`。

---

### CSS 设计体系

项目不使用 Tailwind CSS。所有样式通过 `client/src/styles/global.css` 定义，基于 CSS custom properties 和手写 class。

**设计标记（`:root` 变量）：**
```css
--bg-primary: #313338;   --bg-secondary: #2b2d31;   --bg-tertiary: #1e1f22;
--text-primary: #f2f3f5; --text-secondary: #b5bac1; --text-muted: #949ba4;
--accent: #5865F2;       --danger: #da373c;         --success: #23a559;
--border: #3f4147;       --radius: 8px;
```

**常用 class 模式：**
- `.chat-item`, `.chat-item.active`, `.chat-item.pinned` — 聊天列表项
- `.msg-row`, `.msg-continuation`, `.msg-content`, `.msg-actions` — 消息气泡
- `.reaction-chip`, `.reaction-chip.me` — Reaction 按钮
- `.btn-primary`, `.btn-ghost`, `.btn-danger` — 按钮
- `.modal-overlay`, `.modal-box` — 弹窗
- `.input-field`, `.form-label`, `.form-error`, `.form-box` — 表单
- `.sidebar`, `.chat-body`, `.chat-footer` — 布局

---

## 组件关系图

```
ChatPage (routes/ChatPage.jsx)
├── ChatList (sidebar)
│   ├── ChatListItem × N
│   ├── CreateGroupForm (conditional)
│   ├── PublicChannelList (embedded)
│   ├── EmptyState (conditional, when chats.length === 0)
│   ├── ChatInfoModal (conditional)
│   └── SettingsModal (conditional)
├── ChatView (main area, when activeChatId set)
│   ├── MessageItem × N
│   │   └── UserProfileModal (conditional)
│   └── Composer
├── WelcomeView (when no activeChatId on desktop)
└── MemberPanel (right panel, when activeChatId && !isMobile)
    └── UserProfileModal (conditional)
```

---

## 跨组件问题汇总

| # | 问题 | 位置 | 严重性 |
|---|------|------|--------|
| 1 | `DmSearchPanel.jsx` 未被引用（DM 已废弃） | DmSearchPanel | 低 |
| 2 | `UserProfileModal.jsx` 被 3 个组件引用（MessageItem、MemberPanel、ChatInfoModal），纯展示组件无自身状态 | UserProfileModal | 低 |
| 3 | 无虚拟列表，大量消息时性能差 | MessageItem, ChatListItem | 中 |
| 4 | `alert()`/`confirm()` 阻塞 UI | MessageItem | 中 |
| 5 | `#avatar-file-input` DOM 耦合（SettingsModal 定义，ChatList 读取） | SettingsModal, ChatList | 低 |
| 6 | `renderContent` 功能过于有限（仅 @mention + URL，无 markdown/emoji 解析） | renderContent | 中 |
| 7 | 中英文混用 | PublicChannelList | 低 |
| 8 | 头像颜色未考虑用户自定义 `avatar_color` | ChatListItem | 低 |
| 9 | 附件上传无进度指示 | Composer | 中 |
| 10 | 无错误边界 | 全局 | 高 |
