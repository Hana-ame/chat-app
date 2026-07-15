# 修正记录
> 文件：`frontend-logic-spec-20260710.md`、`frontend-ui-spec-20260710.md`
> 日期：2026-07-10

## `frontend-logic-spec-20260710.md`

| # | 所在节 | 上下文 | 修改 |
|---|--------|--------|------|
| 1 | 文档头部 > 元信息块 `>` | 依赖声明行，被全文引用，无平行项 | `React 18、Zustand、React Router v6` → `React 19、Zustand、React Router v7` |
| 2 | 目录 / mock.js 节标题 / 函数分类表 / 概述段 | 全文 4 处重复声明 mock 函数总数，读者依此判断覆盖率 | 全部 `28 个` → `29 个`（分类表 5+2+10+2+5+2+2+1=29，`mockTogglePin` 未被计数） |
| 3 | Chats 类 > `listChats` 行 | API 方法表（HTTP 路径列），被 `mockListChats` 实现引用 | `GET /api/chats/my` → `GET /api/chats`（前端实际请求路径，Go 后端才是 `/api/chats/my`） |
| 4 | Chats 类 > 方法表末尾 | API 方法表，平行于 register / login / listChats / sendMessage 等行 | 补充 `togglePin` 行：`POST /api/chats/{id}/pin` → `mockTogglePin` |
| 5 | mock.js 节 > 标题行 | 文件名括号内的行数标注，平行于 auth.js(92)/chat.js(361) | `447 行` → `456 行` |
| 6 | 核心状态 > `data` 变量注释 | `let data = null` 行内注释，被 `ensureData()` 引用 | `// { users, chats, messages, reactions, chatMembers }` → `// { chats, messages }`（后三者代码库中不存在） |
| 7 | 差异表（与 Go API 对比）| 审计结论表第 4 行，平行于"不校验密码/无 attachment 校验/MarkRead 无校验/无 presence" | `AI 回复 50% 触发 / ⚠️ 预期行为` → `AI 回复 100% 触发 / ⚠️ mock专属` |
| 8 | `mockSendMessage` > AI 回复代码块 | `mockSendMessage` 函数源码展示段，被差异表和注意框引用 | 删除伪造的 `if (Math.random() < 0.5)` 分支；`AI_RESPONSES[Math.floor(Math.random() * AI_RESPONSES.length)]` → `AI_RESPONSES[0]`；`500 + Math.random() * 800` → `500` |
| 9 | `mockSendMessage` 下方 | 独立的 ### 子节，被差异表#4 和代码块后注释引用 | 删除「双数据源问题」整节（`d.chatMembers` 和 `buildChatResponse` 代码库中不存在） |
| 10 | Store 层 > auth.js > `refreshAuth` 方法 | auth store 方法描述，被"页面刷新恢复登录态"引用 | `调用 refreshAuth() 验证有效性` → `refreshAuth() 已定义但未被调用，不验证有效性（localStorage 直读）` |
| 11 | Store 层 > auth.js > 初始状态表 | Zustand state 结构表，被 login/register/mockLogin 动态修改引用 | 删除伪造的 `accessToken` 初始行（初始 state = `{ user, loading, error, debugMode }`，无 accessToken） |
| 12 | Dev 层 > mock-ws.js > `presence_update` | WebSocket 事件处理段，平行于 message_create / typing 等 | 新增警告：`store.onlineUserIds = payload` 直接赋值跳过 Zustand `set()`，不触发重渲染 |
| 13 | Store 层 > chat.js > `connectWS` 方法 | 连接管理子节，被 store 初始化流程引用 | `connectWS() 检测 mock 模式后调用 mockWebSocketConnect()` → `connectWS() 始终创建真实 WebSocket，mock-ws.js 未集成到 store` |
| 14 | Dev 层 > mock-ws.js > 导出表 | 函数导出列表，平行于 mockWebSocketConnect / mockWebSocketDisconnect / simulateEvent / chatEvents | 补充 `resetMockWs` 行 |
| 15 | Dev 层 > dummy.js > 群组说明 | 命名群组列表段，平行于 DM 数据说明 | `8 个命名群组` → `9 个`（补充 Pet Lovers） |
| 16 | Dev 层 > dummy.js > 特殊消息索引 | 消息特殊属性分配规则段，平行于附件/E2E 说明 | 拆分为 DM 模式和群组模式（DM: `mi===2` deleted, `mi===5` edited, 末 15 条附件, `mi>10&&%5===0` reactions；群组: `mi===1` deleted, `mi===3` edited, `mi===4` 附件, `mi>5&&%3===0` reactions） |
| 17 | 路由层 > App.jsx > 路由表 | 路由注册顺序表，平行于 login/register/`/` 路由 | 修正顺序：`/*` 在 `/g/:chatId` 之前（原表顺序相反） |
| 18 | 路由层 > ChatPage > `useEffect` 段 | ChatPage 两个 effect 描述，被消息加载/URL 同步逻辑引用 | 补全 `accessToken` 守卫条件 + effect 依赖声明 |
| 19 | 路由层 > ChatPage > `isMobile` 段 | 响应式布局检测段，被 sidebar/chat 视图切换引用 | `useState(() => ...)` 惰性初始化器 → `useState(...)` 直接表达式 + 补充 resize 事件监听 `useEffect` |
| 20 | 路由层 > LoginPage / RegisterPage | 登录/注册页 UI 描述段，被 `mockLogin` 和 debug 流程引用 | 删除 Debug 复选框 + Mock API 切换开关描述，改为实际单一 Quick Enter 按钮（内部调用 `api.enableMock()` + `setDebugMode(true)` + `mockLogin()` + `nav('/')`） |
| 21 | 跨模块问题汇总 > #4 | 问题表第 4 行，平行于 #1-#10 其他跨模块风险项 | 标记为 `❌ 幻觉已删除`（原内容 `d.chatMembers` 不存在） |

## `frontend-ui-spec-20260710.md`

| # | 所在节 | 上下文 | 修改 |
|---|--------|--------|------|
| 1 | 文档头部 > 元信息块 `>` | 依赖声明行，被全文引用，无平行项 | `React 18、Lucide Icons、Tailwind CSS` → `React 19、CSS custom properties (global.css)` |
| 2 | 目录 | 目录项，链接锚点指向 `#tailwind-工具类`，被读者点击导航 | `Tailwind 工具类` → `CSS 设计体系`（锚点同步） |
| 3 | 组件架构概览 > 布局参数 | 核心尺寸声明，被 sidebar/member-panel 宽度的所有 CSS 引用 | sidebar=280px → 300px（`--sidebar-w`），member-panel=280px → 240px（`--member-w`） |
| 4 | 路由层 > ChatPage > effect 依赖段 | URL sync + message loading 两个 `useEffect` 描述，被状态初始化流程引用 | 补充 `accessToken` 作为第二个依赖项；`isMobile` 改为直接表达式 + `setIsMobile` 解构 + resize 事件监听 `useEffect` |
| 5 | 组件架构概览 > ChatPage 布局 | 三栏布局骨架段，被所有子组件引用 | 删除伪造的 `<nav>` 和 `h-screen/flex-col/bg-dark-900/text-gray-100` class，改为实际 `shell` flex 布局（ChatList / ChatView｜WelcomeView / MemberPanel） |
| 6 | PublicChannelList > 功能概述 | 列表项外观描述，被 join/leave 操作引用 | 删除 ✅ / "Join" 按钮描述（实际无加入状态标识，列表项统一显示名称和成员数） |
| 7 | PublicChannelList > 使用场景 | 嵌入位置声明，被 ChatList / MemberPanel 引用 | 删除 MemberPanel（PublicChannelList 仅由 ChatList 导入渲染） |
| 8 | PublicChannelList > i18n 示例 | 中英文混用举例，被跨模块#10 引用 | `"暂无公开频道"` / `"加入成功"` → `"搜索中..."` / `"无结果"` |
| 9 | EmptyState > props 表 | 组件参数表，被 ChatList / MemberPanel 调用引用 | 补充 `icon: ReactNode` 行（原表只列了 `message: string`） |
| 10 | MessageItem > 消息内容渲染描述 | renderContent 调用处说明，被 renderContent 节引用 | `支持 markdown` → `支持 @mention + URL 自动链接` |
| 11 | MessageItem > `timeFormat` 工具函数 | 时间格式化描述，被多条消息的时间戳渲染引用 | `HH:mm` 固定格式 → locale 依赖输出（`toLocaleTimeString`，en-US 为 "2:30 PM"） |
| 12 | Composer > `handleTyping` 方法 | 输入框去抖机制描述，被 typing indicator 流程引用 | `2 秒无输入后停止` → `setTimeout 回调为空函数，实际不发送停止信号` |
| 13 | Composer > 已知问题 | 跨组件 DOM 耦合描述，被 `#avatar-file-input` 引用 | 删除 Composer 的 `document.getElementById` 错误描述（Composer 使用 React `useRef`） |
| 14 | WelcomeView > 组件概述 | 欢迎页 UI 说明，被空聊天状态引用 | 删除"创建按钮"（实际只有 icon + heading + 描述段落，无操作按钮） |
| 15 | ScrollArea > props 表 | 滚动容器参数表，被 ChatList 调用引用 | `height: string\|number` → `style: React.CSSProperties` |
| 16 | renderContent > 功能概述 + 解析流程表 | 全文最核心的幻觉段，被 MessageItem / 消息渲染流程引用 | 全面重写：删除 bold/italic/code/emoji 四种伪造解析，改为实际仅两项——`<@uuid>` mention 匹配（→ `<span class="mention">@username</span>`）和 `https?://` URL 自动链接 |
| 17 | 样式系统 > avatarColors | 头像颜色分配段，被 ChatListItem / ChatInfoModal 引用 | 删除伪造的 8 色 `avatarColors` 数组，改为 `CHAT_COLORS`（mock.js 15 色）+ `chat.icon_color \|\| '#5865F2'` 回退机制 |
| 18 | 样式系统 > ### 子节 | 完整的 CSS 框架声明段，被所有样式引用描述 | `Tailwind 工具类`（伪造的 `dark-900` / `bg-indigo-500/10` / `rounded-xl` 等）→ `CSS 设计体系`（实际的 `:root` 变量 + `.chat-*` / `.msg-*` / `.btn-*` / `.modal-*` class 体系） |
| 19 | 组件关系图 | 树形结构 ASCII 图，被全文架构概览引用 | ChatView 子节点删除 `ScrollArea`；MemberPanel 子节点删除 `PublicChannelList` |
| 20 | 跨组件问题汇总 > #2 | 问题表第 2 行，平行于 #1-#10 其他问题 | `UserProfileModal 未被引用` → `实际被 3 组件引用（MessageItem / MemberPanel / ChatInfoModal），问题不成立` |
| 21 | 跨组件问题汇总 > #4 | 问题表第 4 行，平行于 #1-#10 其他问题 | 删除 ChatInfoModal（该组件无任何 `alert()` / `confirm()` 调用） |
| 22 | 跨组件问题汇总 > #5 | 问题表第 5 行，平行于 #1-#10 其他问题 | `#avatar-color-input` / `#file-input` → `#avatar-file-input`；Composer → ChatList |
| 23 | 跨组件问题汇总 > #6 | 问题表第 6 行，平行于 #1-#10 其他问题 | `不支持多行代码块` → `功能过于有限（仅 @mention + URL）` |
