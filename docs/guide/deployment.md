# 生产部署与发布

## 部署架构

两种方式：

**一体化**：后端二进制同时托管前端静态文件（`CHAT_STATIC_DIR` 指向 `client/dist`）。单域名、单进程，适合小规模。

**前后端分离**（当前线上方式）：
- 后端 API：`https://chat.moonchan.xyz`（反向代理到后端进程）
- 前端静态：Cloudflare Pages（同一域名，路径规则 `/api/*`、`/ws`、`/swagger/*` 代理到后端）

## 环境变量

完整配置（来自 `server/internal/config/config.go`）：

| 变量 | 默认 | 说明 |
|---|---|---|
| `CHAT_JWT_SECRET` | 随机生成 | JWT 签名密钥；**生产必须设置**，变更会使所有会话失效 |
| `CHAT_ADDR` | `:8080` | 监听地址 |
| `CHAT_DB_PATH` | `chat.db` | SQLite 路径 |
| `CHAT_STATIC_DIR` | `../client/dist` | 前端静态目录（一体化模式） |
| `CHAT_BASE_URL` | 空 | 上传等响应的绝对 URL 前缀；生产必填，否则从请求头推导 |
| `CHAT_UPLOAD_DIR` | `uploads` | 上传文件目录 |
| `CHAT_UPLOAD_SALT` | 随机生成 | 删除链接（`delete_url`）签名盐 |
| `CHAT_MAX_UPLOAD` | 20971520 | 单文件上限（字节），20 MiB |
| `CHAT_ACCESS_TTL` | `30m` | Access Token 有效期 |
| `CHAT_REFRESH_TTL` | `8760h` | Refresh Token 有效期 |
| `CHAT_MAX_MESSAGE_LENGTH` | 4000 | 消息内容上限（字符） |
| `CHAT_WS_MAX_MSG_SIZE` | 65536 | WS 单帧上限（字节） |
| `CHAT_API_TIMEOUT` | `10s` | 常规 API 超时 |
| `CHAT_UPLOAD_TIMEOUT` | `5m` | 上传接口超时 |
| `CHAT_READ_TIMEOUT` | `10m` | SSE / AI 流式超时 |
| `CHAT_READ_HEADER_TIMEOUT` | `10s` | 读请求头超时 |
| `CHAT_CSP_CONNECT_SRC` | `'self' wss://...` | CSP `connect-src` 追加源 |
| `CHAT_AI_ALLOW_PRIVATE` | `0` | 允许 AI 端点解析到内网（SSRF 防护，默认拦截） |

## nginx 反向代理模板

```nginx
server {
    listen 443 ssl;
    server_name chat.moonchan.xyz;

    # 前端静态（Cloudflare Pages 场景则跳转到 Pages，无需此块）
    location / { proxy_pass http://127.0.0.1:5173; }

    # API + 实时
    location /api/  { proxy_pass http://127.0.0.1:8080; proxy_set_header X-Forwarded-Proto $scheme; }
    location /ws    { proxy_pass http://127.0.0.1:8080; proxy_http_version 1.1;
                      proxy_set_header Upgrade $http_upgrade; proxy_set_header Connection "upgrade"; }
    location /swagger/ { proxy_pass http://127.0.0.1:8080; }
}
```

注意：CSP/限速基于 `X-Forwarded-*` 与 `RealIP` 中间件，反代必须透传 `X-Forwarded-For`/`X-Forwarded-Proto`（详见 [security.md](../security.md)）。

## 发布流程（CI/CD）

1. 修改代码 → `git add` + `git commit`
2. bump 版本：同步两处 `client/package.json` 与 `server/internal/handlers/swagger.json` 的 `"version"`
3. `git tag v<version>`
4. `git push && git push --tags`
5. 观察 CI：`gh run list` → `gh run watch <run-id> --exit-status`

CI 流水线（push 到 `main`/`dev` 或 tag 时触发）：

| 工作流 | 任务 | 内容 |
|---|---|---|
| `CI` | `go-test` | 后端 `go test ./...` + 覆盖率产物 |
| `CI` | `frontend-build` | `tsc --noEmit` + `vite build` |
| `CI` | `go-build` / `release` | 仅 tag 触发，构建跨平台产物并发布 Release |
| `Frontend CI` | `mock-test` | Playwright mock 模式（ci.spec + real-time.spec） |
| `Frontend CI` | `full-e2e` | 起后端 + Vite 跑 `e2e.spec.mjs` |

## 上线检查清单

- [ ] `CHAT_JWT_SECRET` 已设置且唯一
- [ ] `CHAT_BASE_URL` 为线上 https 地址
- [ ] 上传目录持久化（数据盘），数据库定期备份
- [ ] 反向代理透传 `X-Forwarded-*`，WebSocket Upgrade 头
- [ ] CI 全绿；版本 tag 已打
- [ ] `GET /api/version` 返回预期版本号
