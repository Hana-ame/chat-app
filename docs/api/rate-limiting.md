# 限速

基于 `github.com/go-chi/httprate`，全部按窗口滑动计数。规则来自 `handlers/router.go`：

| 范围 | 限制 | 键 | 覆盖路由 |
|---|---|---|---|
| 全局 API | 120 次/分钟/IP | IP | `/api/*`（upload 与 SSE 除外） |
| 上传 | 60 次/分钟/IP | IP | `/api/upload`、`/api/local/*` |
| 登录 | 10 次/分钟/IP | IP | `POST /api/auth/login` |
| 注册 | 5 次/分钟/IP | IP | `POST /api/auth/register` |
| 用户搜索 | 30 次/分钟/用户 | user id（未登录回退 IP） | `GET /api/users` |
| 发送消息 | 30 次/分钟/用户 | user id（未登录回退 IP） | `POST /api/chats/{id}/messages` |

被限速时返回 HTTP 429 + `Retry-After` 头（httprate 默认行为）。

注意：SSE（`/api/events`）与 WebSocket（`/ws`）不在 120/min 限速内（长连接组单独超时配置，见 [backend.md](../architecture/backend.md#路由与中间件routergo)）。
