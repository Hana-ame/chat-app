# 前端功能实现报告

报告日期：2026-07-10
版本：v1.1
审查范围：`client/src/` (components, routes, store, api)

---

## 1. 界面功能模块

### 1.1 身份验证与入口 (`/login`, `/register`)
提供完整的用户准入界面。

- **注册页面**: `client/src/routes/RegisterPage.jsx:5` 支持输入用户名、邮箱、密码创建账号。
- **登录页面**: `client/src/routes/LoginPage.jsx:5`
  - 标准登录表单。
  - **Debug 模式**: `client/src/routes/LoginPage.jsx:36-41` 专为开发/CI 设计的快捷入口，可通过 "Quick Enter (mock)" 绕过真实认证直接进入主页。
- **会话状态**: `client/src/api/client.js:37-63` 自动处理 401 错误并尝试使用 `refresh_token` 刷新会话。

### 1.2 核心聊天布局 (`/`)
采用经典的“侧边栏 + 主视图”架构。

#### 1.2.1 侧边栏 (Sidebar)
- **聊天列表**: `client/src/components/ChatList.jsx:24` 实时显示参与的所有会话。
- **会话操作**: 
  - 右键上下文菜单 $\rightarrow$ 删除会话: `client/src/components/ChatList.jsx:44-49`
  - 点击切换当前活跃聊天室: `client/src/components/ChatList.jsx:204`
- **快捷入口**: 
  - `Create Group`: `client/src/components/ChatList.jsx:153`
  - `New DM`: `client/src/components/ChatList.jsx:154`
  - `Public Channels`: `client/src/components/ChatList.jsx:201`
- **个人设置**: `client/src/components/ChatList.jsx:225` 点击头像进入 Settings 模态框。

#### 1.2.2 聊天主视图 (Chat View)
- **消息流**: `client/src/components/ChatView.jsx:15`
  - 渲染文本、附件链接: `client/src/components/MessageItem.jsx` (通过 `ChatView.jsx:199` 渲染)
  - 支持消息编辑、删除操作: `client/src/components/MessageItem.jsx`
  - 实时显示 Emoji Reactions: `client/src/components/MessageItem.jsx`
- **顶部状态栏 (Header)**: `client/src/components/ChatView.jsx:124`
  - 显示聊天室名称及成员数量: `client/src/components/ChatView.jsx:127-132`
  - **公告栏 (Notice Board)**: `client/src/components/ChatView.jsx:136-178` 支持所有者/管理员进行 `设置` $\rightarrow$ `编辑` $\rightarrow$ `清除` 的完整链路。
  - **成员面板**: `client/src/components/ChatView.jsx:133` (通过 `MemberPanel` 渲染，见 `client/src/components/MemberPanel.jsx`)
- **输入区域 (Composer)**: `client/src/components/Composer.jsx:6`
  - 多行文本输入: `client/src/components/Composer.jsx:68`
  - **文件附件**: `client/src/components/Composer.jsx:39-53` 支持点击附件按钮触发文件选择，在发送前预览已选择的文件 (`client/src/components/Composer.jsx:57-65`)。

### 1.3 用户个人设置 (Settings)
通过模态框实现，无需页面跳转。

- **头像管理**: `client/src/components/SettingsModal.jsx:23-31` 支持点击头像上传新图片，实时预览并保存。
- **资料修改**: `client/src/components/SettingsModal.jsx:34-35` 更新用户名、自定义头像颜色。
- **账户操作**: `client/src/components/ChatList.jsx:226` 一键退出登录 (Logout)。

---

## 2. 实时通信与状态管理

### 2.1 通信协议适配
前端实现了高度灵活的通信协议切换机制，用户可在界面上动态选择：`client/src/components/ChatList.jsx:150-152`

- **WS (WebSocket)**: `client/src/api/client.js:148` (SSE URL 示例，WS 逻辑在 Store 中实现)
- **SSE (Server-Sent Events)**: `client/src/api/client.js:148`
- **POLL (Polling)**: `client/src/api/client.js:86` (通过 `listChats` 定时请求实现)

### 2.2 实时更新逻辑
通过全局 Store (Zustand) 监听服务端事件，实现以下 UI 的无刷新更新：

- **消息实时追加**: `client/src/store/chat.js` (处理 `onMessageCreate` 事件)
- **消息实时变更**: `client/src/store/chat.js` (处理 `onMessageUpdate/Delete` 事件)
- **聊天列表同步**: `client/src/store/chat.js` (处理 `onChatCreate` 等事件)
- **公告同步**: `client/src/store/chat.js` (更新 `pinnedMessage` 状态)

---

## 3. 开发与测试增强 (DX)

- **Mock API 模式**: `client/src/api/client.js:208-226`
  - 内置 `mock.js` 拦截层，允许在没有后端的情况下运行完整的前端 UI 逻辑。
  - 用于 CI 环境下的 Playwright 测试，确保 UI 逻辑的稳定性。
- **响应式设计**: `client/src/styles/global.css`
- **错误处理**: `client/src/api/client.js:64-66` 对 429 Too Many Requests 提供标准处理。
