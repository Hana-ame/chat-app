# 全系统 Payload 功能-格式 对照表 (Global Payload Comparison Table)
日期: 2026-07-10

## 1. REST API Payload 对照表 (HTTP)
所有 REST API 请求均采用 `application/json` 格式。

| 功能 | 方法 | 路径 | Payload 格式 (JSON) | 说明 |
| :--- | :--- | :--- | :--- | :--- |
| **用户注册** | `POST` | `/api/auth/register` | `{"email": "...", "username": "...", "password": "..."}` | 创建账号并登录 |
| **用户登录** | `POST` | `/api/auth/login` | `{"email": "...", "password": "..."}` | 验证凭据并登录 |
| **Token 刷新** | `POST` | `/api/auth/refresh` | *(无 Body, 依赖 refresh_token cookie)* | 换取新 access_token |
| **资料更新** | `PATCH` | `/api/users/me` | `{"username": "...", "avatar_color": "...", "avatar_url": "..."}` | 部分字段可选更新 |
| **创建群聊** | `POST` | `/api/chats` | `{"type": "group", "name": "...", "visibility": "...", "member_ids": ["..."]}` | `type` 必须为 `group` |
| **创建私聊** | `POST` | `/api/dms` | `{"user_id": "..."}` | (Deprecated) 查找或创建 DM |
| **重命名群聊** | `PATCH` | `/api/chats/{id}` | `{"name": "..."}` | 仅 Owner 可操作 |
| **添加成员** | `POST` | `/api/chats/{id}/members` | `{"user_id": "..."}` | 添加用户至群聊 |
| **发送消息** | `POST` | `/api/chats/{id}/messages` | `{"content": "...", "attachments": [{"url": "...", "filename": "...", "mime_type": "..."}]}` | 支持附件数组 |
| **编辑消息** | `PATCH` | `/api/chats/{id}/messages/{mid}` | `{"content": "..."}` | 仅作者可操作 |
| **标记已读** | `POST` | `/api/chats/{id}/read` | `{"message_id": "..."}` | (Deprecated) 更新已读指针 |
| **设置置顶** | `POST` | `/api/chats/{id}/pin` | `{"content": "..."}` | 仅 Owner 可操作, 需 $\ge 3$ 人 |

---

## 2. WebSocket Payload 对照表 (WS)
统一信封格式: `{"op": "操作码", "payload": { ... }}`

### 2.1 服务端 $\rightarrow$ 客户端 (S $\rightarrow$ C)
| 操作码 (Op) | 含义 | Payload 结构 (JSON) | 说明 |
| :--- | :--- | :--- | :--- |
| `ready` | 登录就绪 | `{"user": User, "chats": [Chat], "online_user_ids": [string]}` | 初始化数据快照 |
| `message_create` | 新消息 | `Message` 对象 | 完整消息内容 |
| `message_update` | 消息编辑 | `Message` 对象 | 更新后的完整消息 |
| `message_delete` | 消息删除 | `{"chat_id": string, "message_id": string}` | 移除消息通知 |
| `reaction_add` | 添加反应 | `{"chat_id": string, "message_id": string, "emoji": string, "user_id": string}` | Emoji 添加 |
| `reaction_remove` | 移除反应 | `{"chat_id": string, "message_id": string, "emoji": string, "user_id": string}` | Emoji 撤回 |
| `chat_create` | 聊天创建 | `Chat` 对象 | 新聊天详情 |
| `chat_update` | 聊天更新 | `Chat` 对象 | 属性变更 |
| `chat_delete` | 聊天删除 | `{"chat_id": string}` | 聊天被删除 |
| `chat_remove` | 成员被移 | `{"chat_id": string}` | 用户退出/被踢 |
| `user_update` | 用户更新 | `User` 对象 | 资料变更广播 |
| `presence_update` | 在线状态 | `{"user_id": string, "status": "online"$\vert$"offline"}` | 上下线通知 |
| `typing` | 输入状态 | `{"chat_id": string, "user_id": string, "timestamp": string}` | 输入中提示 |
| `error` | 错误通知 | `{"message": string}` | 协议层错误 |
| `pong` | 心跳响应 | *无 (Empty)* | 响应 `ping` |

### 2.2 客户端 $\rightarrow$ 服务端 (C $\rightarrow$ S)
| 操作码 (Op) | 含义 | Payload 结构 (JSON) | 说明 |
| :--- | :--- | :--- | :--- |
| `ping` | 心跳探测 | *无 (Empty)* | 维持连接 |
| `subscribe` | 订阅聊天 | `{"chat_id": "..."}` | 开启特定聊天推送 |
| `unsubscribe` | 退订聊天 | `{"chat_id": "..."}` | 关闭特定聊天推送 |
| `typing` | 输入通知 | `{"chat_id": "..."}` | 触发输入状态广播 |

---

## 3. SSE Payload 对照表 (SSE)
格式: `event: <name>\ndata: <json>\n\n`

| 功能 | 事件名 (`event`) | 数据内容 (`data`) | 说明 |
| :--- | :--- | :--- | :--- |
| **初始化就绪** | `ready` | `{"user": User, "chats": [Chat], "online_user_ids": [string]}` | 连接建立立即发送 |
| **实时广播** | *(默认/无)* | `{"op": "WS操作码", "data": { WS对应的Payload }}` | 完全复用 WS 的 S $\rightarrow$ C 操作码 |

---
*报告由 opencode 自动生成*
