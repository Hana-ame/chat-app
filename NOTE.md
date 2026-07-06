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
│       ├── components/  # ChatList, ChatView, MessageItem, Composer, MemberPanel
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

## 已知主要问题

- `Load older messages` 功能不可用 (store 未合并)
- Tab 顺序因 `column-reverse` 倒置
- ChatList 组件过大 (313 行)

详见 `docs/todo-20260706.md`。
