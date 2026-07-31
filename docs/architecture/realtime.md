# 实时协议（WS / SSE / Poll）

三种载体共用同一事件信封：`{"op": "<op>", "req_id": N, "payload": {...}}`。

## 事件类型（op）

| op | 触发 | payload |
|---|---|---|
| `message_create` | 发送消息（含 AI 流式分块） | 完整 Message 对象 |
| `message_update` | 编辑消息 | 完整 Message 对象 |
| `message_delete` | 删除消息 | `{chat_id, message_id}` |
| `reaction_add` | 添加表情反应 | `{chat_id, message_id, emoji, user_id}` |
| `reaction_remove` | 移除表情反应 | `{chat_id, message_id, emoji, user_id}` |
| `chat_create` | 创建聊天（含 DM/通知聊天） | 完整 Chat 对象 |
| `chat_update` | 改名/成员变化/置顶/头像等 | 完整 Chat 对象 |
| `chat_remove` | 被移出聊天 | `{chat_id}` |
| `user_update` | 用户资料变化 | User 对象（**email 已脱敏为空**） |
| `presence_update` | 上线/下线 | `{user_id, status}` |
| `ping` / `pong` | WS 心跳 | 空 |
| `error` | 错误 | `{message}` |

Message 对象关键字段：`id`、`chat_id`、`user_id`、`content`、`created_at`、`edited_at`、`deleted`、`attachments`（数组）、`reactions`（数组）、`type`、`reply_to_message_id`。

## WebSocket（/ws）

- 地址：`/ws?token=<access_token>`（或 Cookie）
- 帧：文本 JSON 信封
- 心跳：服务端定时 `ping`，客户端回 `pong`；超时断开
- 广播范围：仅聊天成员；`user_update`/`presence_update` 广播给所有在线连接
- 消息上限：`CHAT_WS_MAX_MSG_SIZE`（默认 64 KiB）

连接建立后服务端推送初始状态（等价 SSE 的 ready）：当前用户 + 聊天列表 + 在线用户 id。

## SSE（GET /api/events）

- 认证：`Authorization: Bearer <token>` 请求头
- 连接即推 `ready` 事件，**只有这一个事件带 `event:`/`id:` 字段**：

```
id: 0
event: ready
data: {"user": {...}, "chats": [...], "online_user_ids": [...]}

```

- 后续事件一律为**裸数据行**（无 `event:` 字段，前端按 `data:` 解析 JSON 信封）：

```
data: {"op":"message_create","payload":{...}}

```

- 保活：每 30s 推注释行 `:keepalive`（连接断开后前端需重连）

## Poll（前端降级）

- 无服务端推送；前端定时 `GET /api/chats/my` 轮询聊天列表，并按需拉取消息
- 事件合并在前端完成：轮询结果 diff 出 `chat_update`/`message_create` 后走与 WS 相同的 store handler
- 客户端需容忍过期响应：`coordinator` 用 `chatId` 校验丢弃非当前聊天的回包

## 前端接入

- 传输统一接口见 [frontend.md](frontend.md#实时协调器realtimecoordinatorjs)
- 事件分发：`coord.setHandlers({ onMessageCreate, onMessageUpdate, ... })`，store 内实现
- 发送操作（消息/反应/已读）走普通 HTTP；事件流只负责"别人改了"的同步，本端操作后通常依赖服务端回推保持一致
