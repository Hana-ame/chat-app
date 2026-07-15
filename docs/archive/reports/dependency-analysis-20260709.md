# 代码依赖分析

## Tree

```
server/
├── cmd/chatd/main.go
├── docs/swagger/docs.go
└── internal/
    ├── auth/
    │   ├── auth.go
    │   └── auth_test.go
    ├── config/
    │   └── config.go
    ├── db/
    │   ├── db.go
    │   ├── chats.go / chats_ext.go
    │   ├── messages.go / messages_test.go
    │   └── users.go
    │   └── db_test.go
    ├── handlers/
    │   ├── handler.go / router.go / util.go
    │   ├── auth.go / users.go / chat.go
    │   ├── member.go / messages.go / reactions.go
    │   ├── sse.go / uploads.go
    ├── models/
    │   └── models.go
    ├── orderedmap/
    │   └── orderedmap.go
    ├── testutil/
    │   ├── testutil.go / client.go / multipart.go
    │   ├── handler_test.go / auth_flow_test.go / integration_test.go
    ├── ws/
    │   ├── client.go / gateway.go / hub.go
    │   └── ws_test.go
```

---

## models 影响文件（8 个）

| 文件 | 使用的模型类型 |
|------|---------------|
| `db/users.go` | `User`, `ChatMember` |
| `db/chats.go` | `Chat`, `ChatMember`, `RefreshToken`, `User`, `Message` |
| `db/chats_ext.go` | `Chat` |
| `db/messages.go` | `Message` |
| `db/messages_test.go` | `Message` |
| `ws/hub.go` | 全部（通过 `models.*` 引用） |
| `handlers/messages.go` | `Attachment`（在 `sendMsgReq` 中） |
| `handlers/handler.go` | import 但间接使用（通过 `userFrom` 返回 `*models.User`） |

---

## handlers 影响文件（2 个）

| 文件 | 用途 |
|------|------|
| `cmd/chatd/main.go` | 入口，创建 `Server` 并调用 `Router()` |
| `testutil/testutil.go` | 测试 Fixture，创建 `*handlers.Server` |

---

## 需要处理的文件（22 个 — 不直接依赖模型也不依赖 handler）

| 包 | 文件 | 说明 |
|----|------|------|
| **config** | `config.go` | 配置加载，零依赖 |
| **auth** | `auth.go`, `auth_test.go` | JWT/bcrypt 服务，仅依赖外部库 |
| **db** | `db.go`, `db_test.go` | DB 连接和迁移，不直接引用模型 |
| **ws** | `client.go`, `gateway.go`, `ws_test.go` | WS 连接管理（`hub.go` 已归入 models 依赖） |
| **orderedmap** | `orderedmap.go` | 独立 JSON 有序 map 实现 |
| **testutil** | `client.go`, `multipart.go` | HTTP 请求辅助（`testutil.go` 已归入 handlers 依赖） |
| **testutil** | `handler_test.go`, `auth_flow_test.go`, `integration_test.go` | 测试文件 |
| **docs** | `swagger/docs.go` | 自动生成 Swagger 文档 |