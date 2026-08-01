# Chat App

实时聊天应用：Go 后端（chi + SQLite + WebSocket/SSE）+ React 前端（Vite + Zustand）。

## 功能

- 群组聊天（公开 / 未列出 / 私密）、私聊（DM）、系统通知聊天
- 实时消息：WebSocket / SSE / 轮询 三种传输模式（前端可切换）
- 消息编辑、删除、回复、表情反应、提及、附件上传（图片/文件，本机存储）
- 群组管理：成员增删、置顶公告、聊天置顶、头像/横幅/背景图
- AI 流式补全（`POST /api/chats/{id}/messages` type=stream）
- 未读计数、浏览器通知、消息搜索与公开频道发现

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go（chi 路由、SQLite/WAL、gorilla/websocket、SSE） |
| 前端 | React 19、Vite 6、Zustand、React Router 7 |
| 测试 | Go 单测（12 包）、vitest（55 例）、Playwright（mock 模式 34 例 + e2e） |
| 部署 | 后端二进制 + 静态托管（Cloudflare Pages） |

## 快速开始

```bash
# 一键构建 + 启动（编译后端/前端，写日志到 server.log）
python scripts/deploy_local.py all

# 或手动：
cd server && go build -o chatd.exe ./cmd/chatd/ && ./chatd.exe   # 需先配 .env
cd client && npm ci && npm run build && npx vite --port 5173     # 前端 dev
```

详见 [docs/guide/quickstart.md](docs/guide/quickstart.md)。

## 目录结构

```
server/                    # Go 后端
  cmd/chatd/               # 入口
  internal/
    handlers/              # HTTP 处理器 + 路由（router.go）+ swagger.json
    service/               # 业务逻辑、权限、广播
    db/                    # 数据访问 + migrations/
    ws/                    # WebSocket hub + client
    ai/                    # AI 流式（SSRF 防护 ValidateEndpoint）
    config/                # 环境变量配置
client/                    # React 前端
  src/api/                 # API 客户端（client.ts 代理 + mock.js）
  src/store/               # Zustand：auth / chat / notification
  src/realtime/            # 实时协调器 + 传输（ws/sse/poll/mock）
  src/components/          # UI 组件
  src/routes/              # 页面（Login / Register / ChatPage）
  tests/                   # Playwright（ci / real-time / e2e）
scripts/                   # 部署脚本（deploy_local.py 等）
docs/                      # 文档（见下）
```

## 文档

- [docs/README.md](docs/README.md) — 文档导航首页
- 指南：快速开始、生产部署与发布、开发工作流
- 架构：整体 / 后端 / 前端 / 数据库 / 实时协议
- API：端点参考、错误码、限速
- 旧版文档已归档至 `docs/archive/legacy-20260731/`

## 线上环境

- 前端 + API: `https://chat.moonchan.xyz`（同一域名，反向代理）
- 版本: `GET /api/version`（OpenAPI: `/swagger/`）
