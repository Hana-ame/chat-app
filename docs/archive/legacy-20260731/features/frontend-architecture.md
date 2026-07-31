# 前端技术架构文档 (Frontend Architecture Document)

## 1. 概览 (Overview)

本项目前端是一个基于 React 的单页应用 (SPA)，旨在提供一个实时、流畅的聊天体验。系统采用“状态驱动”的架构，通过 Zustand 管理全局状态，并结合 WebSocket/SSE 实现实时数据同步。

### 1.1 技术栈 (Tech Stack)
- **框架**: React 19
- **构建工具**: Vite 6
- **状态管理**: Zustand (轻量级、高性能)
- **路由**: React Router 7
- **通信**:
  - HTTP/JSON: 基础 API 调用
  - WebSocket: 实时消息推送、在线状态同步
  - SSE (Server-Sent Events): 可选的实时事件流
- **测试**: Playwright (E2E 测试)

---

## 2. 项目层级与结构 (Project Structure)

```text
client/
├── src/
│   ├── api/                # API 通信层
│   │   ├── client.js       # 核心请求封装与 API 定义
│   │   └── mock.js         # 模拟数据 (开发/演示用)
│   ├── components/         # UI 组件层
│   │   ├── ChatView.jsx    # 聊天主界面 (消息流、公告栏)
│   │   ├── ChatList.jsx    # 聊天列表容器
│   │   ├── ChatListItem.jsx # 聊天列表项
│   │   ├── Composer.jsx    # 消息输入框与附件上传
│   │   ├── MessageItem.jsx  # 单条消息渲染与操作
│   │   ├── renderContent.jsx # Markdown/提及渲染逻辑
│   │   ├── ChatInfoModal.jsx # 聊天信息弹窗
│   │   ├── MemberPanel.jsx  # 成员面板
│   │   ├── WelcomeView.jsx  # 欢迎页
│   │   ├── UserProfileModal.jsx # 用户资料弹窗
│   │   ├── SettingsModal.jsx # 设置弹窗
│   │   ├── ScrollArea.jsx  # 虚拟滚动容器
│   │   ├── PublicChannelList.jsx # 公开频道列表
│   │   ├── DmSearchPanel.jsx # DM 搜索面板
│   │   ├── CreateGroupForm.jsx # 创建群组表单
│   │   ├── ImagePreviewModal.jsx # 图片预览弹窗
│   │   └── EmptyState.jsx  # 空状态占位
│   ├── routes/             # 页面级路由组件
│   │   ├── LoginPage.jsx   # 登录页
│   │   ├── RegisterPage.jsx # 注册页
│   │   └── ChatPage.jsx    # 聊天主页
│   ├── store/              # 全局状态层
│   │   ├── auth.js         # 用户认证、Session 状态
│   │   └── chat.js         # 聊天列表、消息缓存、实时同步逻辑
│   ├── dev/                # 开发辅助工具
│   │   └── stream-source.js # 流式输出模拟 (AI/Typing 效果)
│   ├── hooks/              # 自定义 React Hooks
│   ├── styles/             # 样式文件 (Discord 暗色主题 CSS)
│   ├── App.jsx             # 根组件与路由配置
│   └── main.jsx            # 应用入口
└── tests/                  # 测试用例
    └── e2e.spec.js         # Playwright 全链路测试
```

---

## 3. 核心模块详细设计 (Detailed Module Design)

### 3.1 API 通信层 (`api/client.js`)
采用单一入口模式，通过 `request` 函数统一处理请求。
- **拦截机制**：
  - **401 处理**：自动触发 `/api/auth/refresh` 尝试续期 Token，失败则触发 `auth:unauthorized` 事件。
  - **429 处理**：识别 `too_many_requests` 错误并抛出特定异常。
- **凭据管理**：使用 `credentials: 'include'` 支持 HttpOnly Cookie 传输。
- **API 封装**：将所有端点（Auth, Chats, Members, Messages, Reactions）封装为具名函数，解耦 UI 与 URL 路径。

### 3.2 状态管理层 (`store/chat.js`)
使用 Zustand 实现响应式状态。
- **状态定义**：
  - `chats`: 当前用户的聊天列表（包含 `pinned` 排序）。
  - `messages`: 当前激活聊天的消息缓存。
  - `pinnedMessage`: 聊天室公告内容 `{ chatId: content }`。
- **同步策略 (Triple-Mode)**：
  - **WS (Primary)**：通过 WebSocket `onmessage` 实时更新状态（`message_create`, `chat_update` 等）。
  - **SSE (Secondary)**：通过 `EventSource` 实现单向实时更新。
  - **Polling (Fallback)**：通过定时 `listChats`/`listMessages` 轮询数据。

### 3.3 UI 组件层
- **`ChatView` (容器组件)**：
  - **消息流管理**：实现游标分页加载 (`loadMore`)，并处理滚动条在加载旧消息后的位置修正。
  - **公告栏 (Notice Board)**：实现 Owner 专属的公告设置、编辑与清除界面。
- **`Composer` (交互组件)**：
  - 处理附件上传到外部存储 (`upload.moonchan.xyz`) $\rightarrow$ 获取 URL $\rightarrow$ 发送给服务器。
- **`MessageItem` (叶子组件)**：
  - 复杂内容渲染 $\rightarrow$ 使用 `renderContent` 处理 `@提及`。
  - 交互动作 $\rightarrow$ 集成表情反应 (Reactions) 和编辑/删除操作。

---

## 4. 关键业务流程 (Key Workflows)

### 4.1 认证流 (Auth Flow)
`Login/Register` $\rightarrow$ `Server (Set-Cookie)` $\rightarrow$ `localStorage (Save AccessToken)` $\rightarrow$ `AuthStore (Update User)` $\rightarrow$ `Router (Redirect to /)`.

### 4.2 实时更新流 (Real-time Sync)
`Server Event` $\rightarrow$ `WS/SSE Client` $\rightarrow$ `ChatStore.onMessageCreate` $\rightarrow$ `Sate Update` $\rightarrow$ `ChatView Re-render` $\rightarrow$ `Scroll to Bottom`.

### 4.3 置顶公告流 (Notice Flow)
`Owner Click Edit` $\rightarrow$ `Input Content` $\rightarrow$ `api.setAnnouncement` $\rightarrow$ `Server DB Update` $\rightarrow$ `WS Broadcast ChatUpdate` $\rightarrow$ `All Clients Update pinnedMessage state`.

---

## 5. 性能与优化 (Optimization)

- **渲染优化**：使用 `useMemo` 缓存 `userMap` 等计算密集型数据，减少不必要的重绘。
- **流量控制**：
  - `listMessages` 采用分页加载 (Limit 50)。
  - 消息列表通过 `filtered` 动态计算，避免重复存储多套消息集。
- **内存管理**：在 `disconnect` 时清理所有 Timer 和 Socket 连接。
