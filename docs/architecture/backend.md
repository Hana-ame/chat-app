# 后端架构

## 分层职责

```
handlers（HTTP 层）        service（业务层）          db（数据访问）
  chi 路由 / 中间件          权限检查（authz）          SQL 封装
  参数校验 / 响应序列化      业务编排 / 广播            事务
  → 调用 service 一层        → 调用 db 一层
```

- `handlers` 不写业务规则，只做 HTTP 层的事；`service` 不感知 HTTP
- `ws.Hub` 是独立的连接管理器，广播由 `service` 发起（调用 `hub.SendToChat`）
- `db` 返回领域模型（`models`），不带 `*sql.DB` 细节

## 路由与中间件（router.go）

中间件链（全局）：`RealIP` → `RequestID` → `Recoverer` → 访问日志 → **CSP** → **CORS**

按路由分组：

| 组 | 中间件 | 路由 |
|---|---|---|
| 上传 | `Timeout(5m)` + `httprate 60/min/IP` | `/api/upload`（GET/PUT/POST）、`/api/local/*`（无认证） |
| 常规 API | `Timeout(10s)` + `httprate 120/min/IP` | `/api/version`、`/api/auth/*`、`/api/users/*`、`/api/chats/*`（登录/注册另有更严限速） |
| 认证组 | 以上 + `authMiddleware` | 聊天、消息、通知、置顶、头像等全部业务端点 |
| SSE | `Timeout(10m)`（非 10s） | `GET /api/events` |
| AI 流式 | 认证 + `Timeout(10m)` | `POST /api/chats/{id}/messages`（type=stream）、`GET /api/chats/{id}/messages/{mid}/stream` |

其他：`/ws`（WebSocket 网关）、`/healthz`、`/swagger/*`（内嵌 swagger.json）、`/favicon.ico`、静态目录回退（`serveStatic`，SPA 路由回退 index.html）。

## 认证（auth）

- **Access Token**：JWT，`CHAT_ACCESS_TTL`（默认 30m），`Authorization: Bearer <token>` 或 `access_token` Cookie
- **Refresh Token**：不透明 token，哈希存 `refresh_tokens` 表，`CHAT_REFRESH_TTL`（默认 1y）
- 流程：`POST /api/auth/login` → 返回 `{access_token, refresh_token}`；`POST /api/auth/refresh` 轮换；`POST /api/auth/logout` 吊销
- 校验失败错误码：`unauthorized`（缺失）/ `token_expired` / `token_invalid`

## 权限模型（service/authz.go）

| 动作 | 规则 |
|---|---|
| 读聊天 | 成员，或公开（visibility=public），或私密（unlisted 仅成员+owner） |
| 发消息 | 必须是成员 |
| 编辑/删除消息 | 本人（删除：owner/admin 可删他人） |
| 管理成员 | chat owner 或 admin |
| 置顶/改名 | owner |
| AI 流式 | 成员 |

错误码见 [api/error-codes.md](../api/error-codes.md)。

## 实时广播

- `ws.Hub`：房间 = chat_id；成员连接加入房间；`hub.SendToChat(chatID, envelope)` 推给该聊天在线成员
- 事件信封：`{"op": "message_create", "req_id": N, "payload": {...}}`
- 单连接（ws.Client）：读/写泵 + 心跳（ping/pong）+ 消息大小限制（`CHAT_WS_MAX_MSG_SIZE`）
- 离线成员在下次连接时通过 `last_read_message_id` 等机制拿到增量（未读计数）

## AI 流式（service/stream.go + ai/stream.go）

1. 客户端发 `type=stream` 消息，body 内嵌 `src`（`endpoint`/`auth_key`/`body` 等）
2. `service.StreamService.StartStream` 先做 **SSRF 校验**：`ai.ValidateEndpoint(endpoint, AIAllowPrivateIPs)` —— 只允许 http/https，默认拒绝私有/回环/链路本地 IP（`CHAT_AI_ALLOW_PRIVATE=1` 可放行，如本地 ollama）
3. 流式 fetch 上游 → 分块经 `POST /api/chats/{id}/messages`（同 type=stream）持久化，`GET .../messages/{mid}/stream` 供消费
4. 30s 无新块自动结束；`liveDone`/`liveChat` 记录用于重放与去重

## 配置

见 [guide/deployment.md](../guide/deployment.md#环境变量)（`config/config.go` 为唯一真源）。
