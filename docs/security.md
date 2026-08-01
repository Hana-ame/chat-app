# 安全

## CSP

`router.go` 为所有响应设置 `Content-Security-Policy`：

```
default-src 'self'
script-src 'self' 'unsafe-inline' 'wasm-unsafe-eval' https://esm.sh https://static.cloudflareinsights.com
style-src 'self' 'unsafe-inline'
img-src 'self' data: blob:
connect-src <CHAT_CSP_CONNECT_SRC> https://esm.sh
font-src 'self' data:
```

- `connect-src` 来自 `CHAT_CSP_CONNECT_SRC` 配置（默认 `'self' wss://wsl-8080.moonchan.xyz`）——部署拓扑变化时改配置即可
- 允许 esm.sh（AI 前端依赖）与 Cloudflare Insights 脚本
- 上传图片通过本域 `/api/local/*` 或 `data:`/`blob:` 展示，无需外域白名单

## CORS

- 来源白名单来自 `CHAT_CORS_ORIGINS`(逗号分隔),默认 `*`(任意 Origin,
  `AllowOriginFunc` 回显请求 Origin);生产可配置为具体域名收紧
- `AllowCredentials: true`——实现上**不能**改成通配符 `*`(凭证 + 通配符
  违反 CORS 规范,见 router.go 注释);含 `*` 时必须用 AllowOriginFunc 回显
- SSE 响应的 `Access-Control-Allow-Origin` 与同一白名单保持一致
- WebSocket 握手 `CheckOrigin` 使用同一白名单(`config.OriginAllowed`,
  判断逻辑与 HTTP CORS 共享):跨站页面发起的 WS 连接被拒(CSWSH);
  gorilla 对未携带 Origin 的原始请求不校验,多数非浏览器库会自动补
  `http://<ws-host>` 形式的 Origin,同样按白名单校验
- 暴露全部请求头/响应头;预检缓存 300s

## 认证与 Cookie

- Access token：JWT HS256，`CHAT_ACCESS_TTL`（30m）；Refresh token：随机串哈希入库，`CHAT_REFRESH_TTL`（1y）
- 认证来源：`Authorization: Bearer` 或 `access_token` Cookie（前端登录后同时写入）
- 密码 bcrypt 存储；注册/登录有独立限速（见 [rate-limiting.md](api/rate-limiting.md)）
- `CHAT_JWT_SECRET` 未设置时**随机生成**——生产不设置会导致重启后全员掉线

## 上传安全

- 上传**无 token 认证**（`upload.html` 独立页面无登录流）；删除由 `?delete=<hash>` 密钥保护（`sha256(path + CHAT_UPLOAD_SALT)` 前 8 字节）
- 扩展名黑名单（.html/.svg/.js 等危险类型拒绝）；Content-Type 白名单校验
- 单文件 ≤ `CHAT_MAX_UPLOAD`（20 MiB）
- 文件存本地磁盘（`CHAT_UPLOAD_DIR`），经 `/api/local/*` 由服务端以正确 Content-Type 输出

## SSRF 防护（AI 流式）

- AI 端点由客户端在消息 `src` 中提供，服务端必须校验：
  - `ai.ValidateEndpoint(endpoint, allowPrivateIPs)`：仅 http/https、host 必填
  - 默认**拒绝**私有网段 / 回环 / 链路本地地址（DNS 解析后检查）
  - `CHAT_AI_ALLOW_PRIVATE=1` 可放行（仅建议本地 ollama 等场景，生产保持关闭）
- 校验在 `service/stream.go` 发起流式前执行（SSRF 相关测试见 `ai/stream_test.go`）

## 真实 IP

- `chimid.RealIP`：信任 `X-Forwarded-For`/`X-Real-IP`（反代必须透传，否则限速按代理 IP 聚合）
- `user_update` 广播脱敏：`Email` 置空后再推送给其他用户

## 其他

- `X-Content-Type-Options: nosniff` 全局设置
- 上传文件 Content-Type 由服务端规范化后输出，禁止嗅探
- 静态托管：`/api/*`、`/ws` 前缀不会回退到 SPA index.html（防止路由吞掉 API 404）
