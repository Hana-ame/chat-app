# NOTE — 快速掌握仓库

## 项目结构

```
chat-app/
├── server/              # Go 后端 (chi + SQLite + WS/SSE)
│   ├── cmd/chatd/       # 入口
│   ├── internal/        # auth, db, handlers, ws, config, testutil
│   └── ...
├── client/              # 前端 (React 19 + Vite + Zustand)
│   └── src/
│       ├── api/         # HTTP 请求封装
│       ├── components/  # ChatList, ChatListItem, ChatView, MessageItem, Composer, MemberPanel, CreateGroupForm, DmSearchPanel, PublicChannelList, SettingsModal
│       ├── store/       # Zustand (auth / chat)
│       ├── routes/      # LoginPage, RegisterPage, ChatPage
│       ├── dev/         # 测试数据生成器
│       └── styles/      # 全局 CSS (暗色 Discord 风格)
├── docs/                # 会话摘要、Todo
├── .github/
│   ├── workflows/ci.yml # CI: test → build → release
│   └── README.md        # CI 踩坑记录
└── Makefile
```

## 核心命令

```bash
# 后端 (Go 1.23)
cd server && CHAT_JWT_SECRET=xxx go run ./cmd/chatd &

# 前端 (Node 22)
cd client && npm run dev     # 开发 → localhost:5173
cd client && npm run build   # 构建 → dist/

# 全部 Go 测试
cd server && go test ./... -count=1 -timeout 120s

# Playwright E2E
cd client && npx playwright test

# 上传服务测试
./client/tests/upload_test.sh
```

## 架构要点

| 层级 | 说明 |
|------|------|
| **后端** | Go chi router, JWT (10yr), SQLite, WS/SSE/Poll 三种实时模式 |
| **前端** | SPA, Zustand 全局状态, 暗色 Discord 风格 |
| **上传** | 头像/附件统一走外部 `upload.moonchan.xyz` (PUT multipart) |
| **实时** | WebSocket 为主, SSE/Poll 备选 |

## CI 流水线

每次 push 到 main：
1. `go-test` — lint + unit tests
2. `frontend-build` — npm ci + build + Playwright
3. `go-build` — 交叉编译 linux/amd64, linux/arm64, windows/amd64
4. `release` — 创建 GitHub Release `build-<shortsha>` 附带三个二进制

## 铁律

**每次改代码必须同步更新相关 README 文档。** 包括新增功能、改 API、修 bug、改结构等。

**每次 docs/ 有增删改必须同步更新 `docs/README.md` 目录。**

**所有交互元素必须同时适配触控（手机/平板）和鼠标（PC）操作。** 按钮/输入框等至少 44px 触控区域，`hover` 效果不能作为唯一操作提示。

**UI 重构类 PR 必须附带三态截图：空数据 / 少量数据 / 大量数据（100+）。** 参见下方 Flexbox 陷阱。

**分页/列表数据加载后必须更新 Store/State，不能只在局部变量中持有。**

## Flexbox 溢出陷阱 — 避坑指南

### 症状
- `overflow-y: auto` 设置了但不生效
- 内容撑破父容器，导致外层滚动而非内层滚动
- 空状态不占满剩余空间

### 根因
flex 子项默认 `min-height: auto`，意味着它的高度"至少等于所有子元素的内容高度之和"。当这个 flex 子项自身也是一个 flex 容器时，它的 `min-height: auto` 会阻止父 flex 容器的约束传递下去，导致内部 `overflow-y` 永远不会触发。

### Flexbox 优先级检查清单
```
✅ 父容器 display: flex
✅ 子项 flex-grow / flex-shrink / flex-basis
🔥 子项 min-height: 0 / min-width: 0（最容易忘！）
✅ 子项 overflow-y: auto
```

### 修复示例
```css
/* 错误：只给内层加 min-height:0 */
.sidebar-body { min-height: 0; overflow-y: auto; }

/* 正确：外层（flex 子项）也需要 min-height:0 */
.sidebar {
  display: flex; flex-direction: column;
  min-height: 0;         /* ← 关键 */
  height: 100%; overflow: hidden;
}
.sidebar-body {
  flex: 1; overflow-y: auto; min-height: 0;
  display: flex; flex-direction: column;
}
```

### 经验教训
1. **"只改目标元素"的直觉是错的** — 问题常出在外层约束链断裂
2. `min-height: auto` 是 flex 子项看不见的默认值，逐层检查每一级 flex 子项
3. 拆分组件时注意包装层是否打断了 flex 父子约束链
4. 使用 `<ScrollArea>` 组件替代手写 `.sidebar-body` 样式，一次踩坑全局受益

## 搜索栏行为

- 单输入框搜索
- 输入即搜索本地聊天（name/ID）+ 自动检索公开频道（Public Channels 分组显示）
- 输入纯数字时触发 Join/Create 操作按钮
**每次对话的最终回复都调用file-uploader skill 上传**

## 已知主要问题

- `Load older messages` → 已修复
- Tab 顺序倒置 → 已修复（改为普通 column 布局）
- ChatList 组件过大 → 已拆为 5 个子组件

## 已修复 Bug 记录

详见 [Sidebar Scroll Bug 排查报告](https://page.moonchan.xyz/?url=https://upload.moonchan.xyz/api/01LLWEUU6SSRBRJKKOFBA3FSOZTXG4ZEVM/sidebar-scroll-report.md.gz#markdown-parser) (Board 666)。

修复列表：
1. **侧边栏不滚动** — 给 `.sidebar` 加 `min-height: 0`
2. **空状态不撑满** — `display: flex` on `.sidebar-body` + `<EmptyState>` 组件
3. **loadMore 不合并数据** — 将 API 返回直接写入 store（`useChatStore.setState`）
4. **Tab 键顺序颠倒** — 去掉 `column-reverse`，改用普通 column

详见 `docs/todo-20260706.md`。
