# Chat App — Session Handoff

> ⚠️ **每次推送前必须检查**: `client/package.json` 版本号是否 bump？详见 `docs/README.md`。

## 本次会话概览

Go 后端 Review + Auth 重构 + Swagger 文档集成。

## 已完成

### handlers 包重组 (包内分文件)
- 将 12 个文件按领域重组为 11 个
- 合并 `handlers.go` + `middleware.go` → `handler.go`
- 合并 `chats.go` + `chats_v2.go` → `chat.go`
- 拆分 members 相关 handler → `member.go`
- `MarkRead` 从 chats 移到 messages
- 删除 `chats_v2.go` (命名误导, 非 v2 API)

### Swagger 文档
- 安装 `swaggo/swag` CLI + `http-swagger/v2`
- `main.go` 添加 `@title`, `@BasePath`, `@securityDefinitions`
- 所有 handler 添加 swaggo 注释
- 生成 `docs/swagger/` (swagger.json, swagger.yaml, docs.go)
- 注册 `/swagger/*` 路由 → 可访问 UI
- URL: `https://wsl-8080.moonchan.xyz/swagger/index.html`

### Auth 重构

**背景**: Access token 默认 TTL 从 10 年改为 30 分钟后, 需要配套改动。

**Refresh token → httpOnly Secure cookie**:
- 后端 `issueSession()`: `Set-Cookie: refresh_token=<raw>; Path=/api/auth/refresh; HttpOnly; Secure; SameSite=Strict`
- `Refresh()` handler: 从 cookie 读取, 不再接受 body
- `Logout()` handler: 清除 cookie + `DeleteUserRefreshTokens` (登出所有设备)
- 前端 `api.refresh()`: 改用 `credentials: 'include'` 直接 fetch
- 前端 `useAuthStore`: 移除 `refreshToken` 字段和 localStorage
- 前端 `request()` 401 自动刷新: 直接 fetch refresh endpoint (cookie 自动附带)
- Secure flag 自动判断: `r.TLS != nil` 或 `X-Forwarded-Proto: https`

**JWT Secret 持久化**:
- 引入 `godotenv` 自动加载 `.env`
- 创建 `server/.env` 固定 secret
- 默认 AccessTokenTTL: 10年 → 30分钟
- 默认 RefreshTokenTTL: 30天 → 1年

### 文档更新
- 创建 `docs/features/api-endpoints.md` (JS client API 参考)
- 创建 `docs/features/go-api-routes.md` (Go 33 路由表)
- 创建 `docs/features/go-api-models.md` (Go 请求/响应结构体)
- 创建 `client/src/components/README.md` (14 组件清单)
- 更新 `docs/README.md` 使用相对链接
- 更新所有 README 反映 auth 改动 (移除 refreshToken)
- 上传多份报告到 Board 666

### Migration 修复
- `0003_user_avatar.sql`: 移除 `DROP INDEX`, 加 `IF NOT EXISTS`
- `db.go`: 错误处理从仅对 0002 生效改为对所有 migration 生效 (`isDupColumnErr`)

### 前端 Auth 适配
- 401 自动刷新: `request()` 函数中 `_refreshing` 锁防并发, 刷新成功自动重试原请求
- 刷新失败才触发 `auth:unauthorized` → logout

## 当前状态

| 组件 | 状态 |
|------|------|
| Go 后端 | 运行中 (:8080) |
| Swagger UI | https://wsl-8080.moonchan.xyz/swagger/index.html |
| 前端 | build 通过, 需推送 Pages 部署 |
| Migration | 0003 索引冲突已修复 |

## 改动文件清单

### Go 后端
| 文件 | 改动 |
|------|------|
| `server/.env` | **新建** — JWT secret, TTL 配置 |
| `server/internal/config/config.go` | 加 `godotenv.Load()`, 改 TTL 默认值 |
| `server/internal/db/db.go` | migration 错误处理通用化 |
| `server/internal/db/migrations/0003_user_avatar.sql` | 加 `IF NOT EXISTS`, 移除 DROP |
| `server/internal/handlers/handler.go` | **新建** — 合并 handlers.go + middleware.go |
| `server/internal/handlers/chat.go` | **新建** — 合并 chats.go + chats_v2.go |
| `server/internal/handlers/member.go` | **新建** — 从 chats.go 拆分 members |
| `server/internal/handlers/auth.go` | refresh → cookie, logout → clear all |
| `server/internal/handlers/messages.go` | 加 MarkRead + readReq |
| `server/internal/handlers/router.go` | 加 swagger 路由 |
| `server/internal/handlers/util.go` | 加 setRefreshCookie / clearRefreshCookie |
| `server/internal/handlers/*.go` | 全部加 swaggo 注释 |
| `server/docs/swagger/` | **新建** — swagger.json + yaml + docs.go |
| 旧文件删除 | `handlers.go`, `middleware.go`, `chats.go`, `chats_v2.go` |

### React 前端
| 文件 | 改动 |
|------|------|
| `client/src/api/client.js` | refresh/logout 签名改, 401 自动刷新改 |
| `client/src/store/auth.js` | 移除 refreshToken 字段和存储 |
| `client/src/README.md` | store 字段更新 |
| `client/src/api/README.md` | API 签名更新 |

### 文档
| 文件 | 改动 |
|------|------|
| `docs/features/api-endpoints.md` | refresh 端点 body 改为 "httpOnly cookie" |
| `docs/features/go-api-models.md` | 移除 refreshReq, sessionResp 移除 RefreshToken |
| `docs/README.md` | 已更新 |

## 安全注意事项

1. **`lock()/unlock()` 空方法** — `IssueAccessToken` 里存在但函数体为空
2. **Access token 不可撤销** — 30 分钟 TTL 是唯一缓解手段
3. **无速率限制** — 登录接口可被暴力尝试
4. **无失败尝试审计** — 无登录失败日志

## 相关 URL

- Swagger UI: https://wsl-8080.moonchan.xyz/swagger/index.html
- Server: https://wsl-8080.moonchan.xyz (or localhost:8080)
- Board 666: https://board.moonchan.xyz (上传的报告)
