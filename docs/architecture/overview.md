# 系统总览

## 架构图

```
┌───────────────────────────── 浏览器 ─────────────────────────────┐
│  React SPA（client/src）                                         │
│  ├─ routes/    ChatPage · LoginPage · RegisterPage               │
│  ├─ store/     auth · chat · notification（Zustand）             │
│  ├─ realtime/  coordinator ─→ transports/{ws,sse,poll,mock}      │
│  └─ api/       client.ts（Proxy + mock.js）                      │
└──────────────────────────────────────────────────────────────────┘
        │ HTTP (JSON)          │ WS / SSE           │ Poll
        ▼                      ▼                    ▼
┌──────────────────────────────────────────────────────────────────┐
│  Go 后端（server/internal）                                       │
│  handlers/router.go — 中间件（CORS/CSP/限速/认证/超时）            │
│    ├─ handlers/   HTTP 处理（参数校验 → 调 service）               │
│    ├─ service/    业务逻辑、权限（authz）、广播、AI 流式            │
│    ├─ ws/         WebSocket hub + client（连接/心跳/事件投递）     │
│    ├─ db/         SQLite 访问（WAL）+ migrations/                 │
│    └─ ai/         流式补全客户端（SSRF 校验）                     │
└──────────────────────────────────────────────────────────────────┘
        │                          │
        ▼                          ▼
   SQLite（chat.db）        本地磁盘（uploads/）
```

## 三种实时传输

同一份事件协议（`op` + `payload`），三种载体，前端可切换（侧栏 `MOCK/WS/SSE/POLL` 按钮）：

| 传输 | 载体 | 说明 |
|---|---|---|
| `ws` | WebSocket `/ws` | 双向，全量事件，心跳 ping/pong |
| `sse` | GET `/api/events` | 服务端推送，仅下行 |
| `poll` | HTTP 轮询 | 前端降级方案，定时拉取 |

Mock 模式（`mock`）下由 `realtime/transports/mock.js` 模拟定时事件，供 CI 测试。

## 一次发送消息的链路

1. 前端 `Composer` → `POST /api/chats/{id}/messages`（携带 `content`、`attachments`；AI 流式时带 `type=stream` + `src`）
2. `handlers/messages.go` 校验 → `service.MessageService.Create`（写入 SQLite，附消息 `messages.attachments` 等冗余字段）
3. 广播：`service` 调 `ws.Hub.SendToChat` → 该聊天成员的所有连接收到 `message_create` 事件（WS/SSE 推送；poll 客户端下次轮询可见）
4. 前端 `store/chat.js onMessageCreate` 更新消息列表 + 聊天列表最后一条 + 未读数

## 目录结构

```
server/
  cmd/chatd/             # main：装配 config/db/hub/server
  internal/
    handlers/            # router.go（全部路由）+ 各端点 handler + swagger.json
    service/             # Service（User/Chat/Member/Message/Reaction/Stream/Authz）
    db/                  # DB 封装 + migrations/*.sql + db_fixups.go
    ws/                  # Hub（房间广播）+ Client（单连接）
    ai/                  # Source 解析 + 流式 fetch + ValidateEndpoint
    config/              # Config.Load() 环境变量
    testutil/            # 测试工具（httptest server 等）
    storage/local/       # 本地上传存储驱动
client/
  src/api/               # client.ts（方法定义）+ mock.js（mock 数据）+ schemas.ts
  src/store/             # auth.js · chat.js · notification.js
  src/realtime/          # coordinator.js + fetchStream.js + transports/
  src/components/        # 17 个 UI 组件（ChatList/ChatView/MessageList/...）
  src/routes/            # ChatPage / LoginPage / RegisterPage
  src/utils/             # ai.js · notifyMessage.js · browserNotify.js
  src/dev/               # dummy 数据（dev 模式）
scripts/                 # deploy_local.py · deploy_win.py
docs/                    # 本文档树
```

## 关键设计决策

- **SQLite + WAL**：单机部署零运维；计数字段（`reaction_count` 等）与 `reactions` JSON 缓存列避免读侧 N+1
- **消息冗余字段**：`messages` 行内直接存 `reactions`/`attachments`/`mentions` JSON，广播时无需 join
- **服务端时间为准**：所有时间戳 UTC RFC3339，前端不信任本地时钟
- **Mock 与真实 API 同构**：前端 Proxy 拦截保证 mock 分支与真实分支签名一致（详见 [frontend.md](frontend.md)）
