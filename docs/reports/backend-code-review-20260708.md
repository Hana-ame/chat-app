# 后端代码审查报告

审查日期：2026-07-08  
审查范围：`server/internal/`（handlers, db, auth, ws, config, models）

---

## 安全风险

### 1. CORS + Credentials 配置错误
- 位置：`server/internal/handlers/router.go:23-30`
- 问题：`AllowOriginFunc` 始终返回 true，同时 `AllowCredentials: true`
- 影响：任意网站可跨域发请求并携带 cookie，导致 CSRF
- 建议：白名单来源或关闭 AllowCredentials

### 2. Token 通过 query string 传递
- 位置：`server/internal/handlers/handler.go:72-80`（bearerToken 函数）
- 问题：`access_token` 可从 URL 查询参数获取
- 影响：token 泄露到服务器日志、浏览器历史、Referer 头

### 3. SSE 端点 token 在 URL 中
- 位置：`server/internal/handlers/sse.go:18-21` + 客户端 `client.js:135`
- 问题：`/api/events?access_token=...` 将 token 暴露在 URL
- 影响：与 #2 相同，连接长期存在泄露风险

### 4. JWT 密钥随机生成
- 位置：`server/internal/config/config.go:67-71`
- 问题：`CHAT_JWT_SECRET` 未设置时，每次重启生成随机密钥
- 影响：所有 token 在重启后失效，用户被迫重新登录

### 5. 错误信息泄露内部细节
- 位置：多处 handler（如 `handler.go:61`, `messages.go:60`, `chat.go:40` 等）
- 问题：将 `err.Error()` 直接返回客户端
- 影响：可能暴露 SQL 语句、文件路径、堆栈信息

---

## 并发与性能

### 6. Hub register 在锁内执行 DB 操作
- 位置：`server/internal/ws/hub.go:57-70`
- 问题：持有锁 (`h.mu.Lock()`) 期间调用 `db.UpdateUserStatus`
- 影响：高并发下注册/注销阻塞，成为瓶颈

### 7. 消息分页 N+1 查询
- 位置：`server/internal/db/messages.go:218-227`
- 问题：每条消息额外查 attachments、reactions、mentions 三张表
- 影响：50 条消息最多触发 150 次额外查询

### 8. 刷新 token 竞态
- 位置：`server/internal/handlers/auth.go:143-162`
- 问题：使用 `refreshMu` 但锁粒度粗，多 goroutine 竞争时可能重复插入
- 现状：已有一把锁，但未覆盖全部路径

---

## 代码问题

### 9. 时间精度不一致
- 位置：`server/internal/db/messages.go:31` vs `chats.go:122`
- 问题：消息时间用毫秒格式，chat last_message_at 用纳秒格式
- 影响：极短时间内消息排序可能错乱

### 10. 上传文件名清洗不充分
- 位置：`server/internal/handlers/uploads.go:29-35`
- 问题：仅替换 `..` 和取 `filepath.Base`，未过滤控制字符
- 影响：文件名可能含 XSS 向量或不可见字符

### 11. serveUpload 路径检查薄弱
- 位置：`server/internal/handlers/router.go:93-105`
- 问题：仅检查 `..` 子串，未 `filepath.Clean`
- 影响：通过编码 `..` 可能绕过

### 12. 退出时未清除 access_token cookie
- 位置：`server/internal/handlers/auth.go:173-182`
- 问题：Logout 仅清除 refresh_token，access_token cookie 仍有效
- 影响：用户以为退出，但 cookie 还有效直至过期

### 13. auth.Service 空锁占位
- 位置：`server/internal/auth/auth.go:59-61`
- 问题：`lock()` 和 `unlock()` 为空函数
- 影响：给维护者误导，以为有并发保护

### 14. 无速率限制
- 范围：全局
- 问题：所有端点无任何限流
- 影响：可被暴力破解登录、耗尽上传配额

---

## 优先级总结

| 优先级 | 数量 | 关键问题 |
|--------|------|----------|
| **高** | 5 | CORS 配置、query string token、SSE token 暴露、JWT 密钥随机、错误信息泄露 |
| **中** | 4 | Hub 锁内 DB、N+1 查询、Filename 清洗、refresh 竞态 |
| **低** | 5 | 时间精度、serveUpload 清洗、Logout 未清 cookie、空锁、缺限流 |

---

## 已修复项（同批次）

- Cookie SameSite 从 Strict 改为 Lax（加注释说明）
- WS 测试在 `WS_ENABLED` 未设置时跳过，避免 CI 环境无效失败
- 测试适配 SameSite 变化
