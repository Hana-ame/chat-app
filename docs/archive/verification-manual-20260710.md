# 系统验证手册 (System Verification Manual)
日期: 20260710
版本: 1.0

## 1. 概述
本手册旨在提供一套标准化的验证流程，用于核对 `chat-app` 后端实现与技术规范之间的一致性。手册涵盖了 REST API、WebSocket (WS) 以及 SSE 的验证方法。

---

## 0. 环境预检 (Pre-flight Check)
**验证迁移脚本与运行态数据库的兼容性。**

### 0.1 清理旧数据
如果数据库文件是之前版本的遗留产物，其 schema 可能与当前代码定义的迁移脚本不一致。务必先删除旧库：

```bash
rm -f chat.db chat.db-shm chat.db-wal
```

### 0.2 启动服务器并验证迁移
```bash
# 全新启动，自动执行 migrations/init.sql
go run cmd/chatd/main.go
```

### 0.3 迁移冒烟测试
启动后立即调用注册接口，确认迁移成功：
```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"smoke@test.com","username":"smoke","password":"test123"}'
```
**预期**: 返回 `200 OK` 并包含 `access_token`。  
**失败**: 若返回 `"SQL logic error: no such column: ..."`，说明数据库存在旧 schema，请回到 **0.1** 清理后重试。

> **⚠ 为什么需要这步？**  
> 审计只验证代码与规范文档的一致性。**运行态**的遗留数据库资产需要单独验证，这是之前审计遗漏的环节。

## 2. REST API 验证指南

### 2.1 验证方法
**推荐工具:** Postman, Insomnia, 或 `curl`。

**验证步骤:**
1. **环境预检**: 先执行第 0 节的迁移冒烟测试。
2. **认证准备**: 调用 `/api/auth/register` 或 `/api/auth/login` 获取 `access_token`。
3. **请求构造**: 根据 $\text{Payload 对照表}$ 构造 JSON Body。
4. **Header 设置**: 添加 `Authorization: Bearer <token>`。
5. **结果核对**:
   - **状态码**: 核对是否符合 $\text{API 逻辑一致性报告}$ 中的定义 (如 400, 401, 403, 409)。
   - **响应体**: 检查返回的 JSON 字段是否与 `models` 定义一致。
   - **副作用**: 检查数据库 (SQLite) 中相关表的数据是否已正确更新。

### 2.2 关键用例 (Test Cases)
- **权限拦截**: 使用未认证 Token 请求 `/api/users/me` $\rightarrow$ 预期 `401 Unauthorized`。
- **输入校验**: 发送 `username` 为空的注册请求 $\rightarrow$ 预期 `400 Bad Request`。
- **业务约束**: 在成员数 $< 3$ 的群组尝试 Pin 消息 $\rightarrow$ 预期 `400 "need at least 3 members to pin"`。

---

## 3. WebSocket (WS) 验证指南

### 3.1 验证方法
**推荐工具:** Postman WebSocket Client 或浏览器 Console JS。

**连接流程:**
1. **握手**: 发起连接 `ws://<host>/ws?access_token=<token>`。
2. **初始化**: 验证收到第一个消息是否为 `OpReady` $\rightarrow$ 核对其中的 `user`, `chats`, `online_user_ids`。

### 3.2 协议验证点
| 验证项 | 发送/接收 Payload | 预期结果 |
|---|---|---|
| **心跳** | 发送 `{"op":"ping"}` | 立即收到 `{"op":"pong"}` |
| **订阅** | 发送 `{"op":"subscribe", "payload":{"chat_id":"..."}}` | 此时该连接应能收到该聊天的消息广播 |
| **输入状态** | 发送 `{"op":"typing", "payload":{"chat_id":"..."}}` | 聊天内其他在线用户收到 `OpTyping` 广播 |
| **消息推送** | 通过 REST API 发送消息 | 所有订阅了该聊天的 WS 客户端收到 `OpMessageCreate` |

---

## 4. SSE 验证指南

### 4.1 验证方法
**推荐工具:** `curl -N` 或浏览器直接访问。

**验证步骤:**
1. **建立连接**: `curl -N "http://<host>/api/events?access_token=<token>"`。
2. **就绪检查**: 验证第一条输出是否为 `event: ready` $\rightarrow$ 核对 `data` 内容。
3. **推送检查**: 保持连接，在另一个窗口执行 API 操作（如更新资料） $\rightarrow$ 验证 SSE 收到 `data: {"op":"user_update", ...}`。

---

## 5. 跨协议同步验证 (Cross-Protocol Sync)

**验证目标**: 确保同一个业务事件同时触发 WS 和 SSE 推送。

**验证流程:**
1. **双连**: 同时开启一个 WS 连接和一个 SSE 连接（同一用户）。
2. **触发**: 调用 `/api/chats/{id}/messages` 发送一条消息。
3. **核对**:
   - WS 客户端收到 `OpMessageCreate`。
   - SSE 客户端收到 `data: {"op":"message_create", ...}`。
   - 两者中的 `payload` 数据必须完全一致。

---

## 6. 自动化回归验证
项目内置了 Playwright 测试集，可快速验证上述逻辑：
- **Mock 测试**: `npm run test:ci` $\rightarrow$ 验证 API 契约与基本流。
- **E2E 测试**: `npm run test:e2e` $\rightarrow$ 验证全链路（含 WS 实时性）。

---
*由 opencode 制定*
