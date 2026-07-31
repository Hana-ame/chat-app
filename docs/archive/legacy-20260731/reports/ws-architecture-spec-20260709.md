# WebSocket 实时架构规范 (WS Architecture Spec)

> 原始来源：`server/internal/ws/`
> 文件：`hub.go`、`client.go`、`gateway.go`

---

## 一、架构总览

```
客户端 ──WS 握手──→ Gateway ──认证──→ Client 注册到 Hub ──→ 收发消息
                    ↑                          ↓
                    └── 认证：ParseAccessToken ──┘
```

### 核心组件

| 组件 | 文件 | 职责 |
|------|------|---|
| `Gateway` | `gateway.go` | WebSocket 握手入口：验证 token、加载用户、初始化 Client、注册到 Hub |
| `Hub` | `hub.go` | 全局连接管理器：维护在线用户集合、消息广播、SSE 回退 |
| `Client` | `client.go` | 单个 WS 连接：读写泵、订阅/退订聊天、心跳 Ping/Pong |

---

## 二、连接生命周期

### 1. 握手（Gateway）

1. 客户端通过 `GET /ws?access_token=...` 发起连接。
2. `Gateway.ServeHTTP` 校验 `access_token`（JWT）。
3. 通过 `ParseAccessToken` 认证，通过 `GetUserByID` 加载用户信息。
4. 升级 WS 连接（`websocket.Upgrader`）。
5. 创建 `Client`，初始化 `subs`（订阅列表，填充该用户的所有聊天）。
6. 发送 `OpReady` 信封（含用户信息、聊天列表、在线用户列表）。
7. 启动 `writePump` 和 `readPump` goroutines。

### 2. 读泵（readPump）

- 设置 PongHandler（`pongWait = 60s`）。
- 循环读取客户端消息，解析为 `Envelope`。
- 处理客户端操作（`OpPing`、`OpSubscribe`、`OpUnsubscribe`、`OpTyping`）。
- 连接断开或读取错误时：`unregister` → `close`。

### 3. 写泵（writePump）

- 从 `send` channel 读取消息写入 WS 连接。
- 心跳：每 `50s` 发送 PingMessage。
- 写超时：`10s`。

---

## 三、消息协议 (Envelope)

### 3.1 通用格式

```json
{"op": "<operation>", "payload": {...}}
```

### 3.2 服务端发出（Op 定义）

| Op | 方向 | 含义 | Payload |
|---|---|---|---|
| `ready` | S→C | 登录成功，初始化数据 | `{user, chats, online_user_ids}` |
| `message_create` | S→C | 新消息 | `Message` |
| `message_update` | S→C | 消息编辑 | `Message` |
| `message_delete` | S→C | 消息删除 | `{chat_id, message_id}` |
| `reaction_add` | S→C | 添加反应 | `{chat_id, message_id, emoji, user_id}` |
| `reaction_remove` | S→C | 移除反应 | `{chat_id, message_id, emoji, user_id}` |
| `chat_create` | S→C | 新聊天 | `Chat` |
| `chat_update` | S→C | 聊天更新 | `Chat` |
| `chat_delete` | S→C | 聊天删除 | `{chat_id}` |
| `chat_remove` | S→C | 用户被移除 | `{chat_id}` |
| `user_update` | S→C | 用户资料更新 | `User` |
| `presence_update` | S→C | 在线状态变更 | `{user_id, status}` |
| `typing` | S→C | 输入中提示 | `{chat_id, user_id, timestamp}` |
| `error` | S→C | 错误信息 | `{message}` |

### 3.3 客户端发送（Op 定义）

| Op | 方向 | 含义 | Payload |
|---|---|---|---|
| `ping` | C→S | 心跳探测 | 无 |
| `pong` | S→C | 心跳响应 | 无（服务端自动回复 `pong`） |
| `subscribe` | C→S | 订阅聊天 | `{chat_id}` |
| `unsubscribe` | C→S | 退订聊天 | `{chat_id}` |
| `typing` | C→S | 输入中通知 | `{chat_id}` |

---

## 四、约束汇总

| 约束 | 说明 |
|------|------|
| 传输 | `gorilla/websocket`，WSS |
| 认证 | `access_token` URL 参数 |
| 订阅 | 连接时自动订阅用户所有聊天 |
| 读超时 | 60s 无消息断开 |
| Ping | 服务端每 50s 发 ping |
| 背压 | `send` channel buffer=64，满则关闭 |
| 并发 | `sync.Once` 确保 close 一次，`sync.RWMutex` 保护 subs |
| 禁用开关 | 环境变量 `WS_ENABLED=false` 可禁用 |

## 四、广播机制（Hub）

### 4.1 发送到用户（sendToUser）

```
Hub.sendToUser(userID, env)
  → 遍历该用户所有 Client 实例 → queue(env)
  → SSE 回退 → sseSend(userID, b)
```

### 4.2 发送到聊天（sendToChat）

```
Hub.sendToChat(chatID, env, exceptUser)
  → DB.GetChatMembers → 遍历所有成员
  → 跳过 exceptUser
  → 对每个成员：sendToUser
```

### 4.3 在线状态管理

| 事件 | 动作 |
|------|------|
| Client 注册（第一个连接） | `UpdateUserStatus("online")` + `broadcastPresence` |
| Client 注销（最后一个断开） | `UpdateUserStatus("offline")` + `broadcastPresence` |

---

## 五、SSE 回退机制

当 WebSocket 不可用时，Hub 自动通过 SSE 通道发送相同数据：

- `SSERegister` / `SSEUnregister`：管理 SSE 通道。
- `sseSend`：向用户的所有 SSE channel 推送数据（非阻塞）。
- 所有 `sendToUser` / `sendToChat` 操作自动同步写入 SSE。

---

## 六、常规定义

| 参数 | 值 |
|---|---|
| ReadBufferSize | 4096 |
| WriteBufferSize | 4096 |
| writeWait | 10s |
| pongWait | 60s |
| pingPeriod | 50s |
| maxMessageSize | 65536 (1<<16) |
| sendQueueSize | 64 |