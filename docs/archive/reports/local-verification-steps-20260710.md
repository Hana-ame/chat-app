# 本地验证步骤指南 (Local Verification Steps)
日期: 2026-07-10

## 1. 环境启动

**启动后端 (Go):**
```bash
cd server
export WS_ENABLED=true
go run cmd/chatd/main.go
```
默认端口: `8080`

**启动前端 (Vite):**
```bash
cd client
npm install
npm run dev
```
默认端口: `5173`

---

## 2. REST API 验证

### A. 注册测试用户
```bash
curl -X POST http://localhost:8080/api/auth/register \
     -H "Content-Type: application/json" \
     -d '{"email":"test@example.com", "username":"tester", "password":"password123"}'
```
记录响应中的 `access_token`。

### B. 登录
```bash
curl -X POST http://localhost:8080/api/auth/login \
     -H "Content-Type: application/json" \
     -d '{"email":"test@example.com", "password":"password123"}'
```

### C. 获取个人资料
```bash
curl -H "Authorization: Bearer YOUR_TOKEN" http://localhost:8080/api/users/me
```

---

## 3. 实时通信验证

### SSE 验证
```bash
curl -N "http://localhost:8080/api/events?access_token=YOUR_TOKEN"
```
应立即收到 `event: ready`。

### WebSocket 验证 (浏览器 Console)
```javascript
const token = "YOUR_TOKEN";
const ws = new WebSocket(`ws://localhost:8080/ws?access_token=${token}`);
ws.onmessage = (e) => console.log(JSON.parse(e.data));

// 心跳
ws.send(JSON.stringify({ op: "ping" }));
// 订阅
ws.send(JSON.stringify({ op: "subscribe", payload: { chat_id: "CHAT_ID" } }));
// 输入状态
ws.send(JSON.stringify({ op: "typing", payload: { chat_id: "CHAT_ID" } }));
```

---

## 4. 自动化测试
```bash
cd client
npm run test:ci   # Mock 测试
npm run test:e2e  # 全链路 E2E 测试
```

---

## 验证对照表

| 目标 | 手段 | 预期结果 |
| :--- | :--- | :--- |
| API 契约 | `curl` / `test:ci` | 状态码 + Payload + 数据库 |
| WS 实时性 | Console JS | `ready` $\rightarrow$ `pong` $\rightarrow$ 广播 |
| SSE 同步 | `curl -N` | `ready` + 同 WS 的 `op` 推送 |
| 全链路 | `test:e2e` | ALL PASS |
