# WebChat App

Discord风格实时聊天应用。React 19 + Go + SQLite3。

```
chat-app/
├── server/                    # Go backend (chi + gorilla/ws + jwt + sqlite)
│   ├── cmd/chatd/main.go      # 入口
│   ├── internal/
│   │   ├── auth/              # bcrypt + JWT
│   │   ├── db/                # SQLite DAO (users/chats/messages/reactions)
│   │   ├── db/migrations/     # 嵌入式SQL DDL
│   │   ├── handlers/          # HTTP路由 + 中间件
│   │   ├── ws/                # WebSocket/SSE gateway (Hub + Client)
│   │   ├── models/            # 数据模型
│   │   ├── config/            # 环境变量配置
│   │   └── testutil/          # 测试夹具
├── client/                    # React 19 + Vite + Zustand
│   ├── src/
│   │   ├── routes/            # LoginPage/RegisterPage/ChatPage
│   │   ├── components/        # ChatList/ChatView/MessageItem/Composer/MemberPanel
│   │   ├── store/             # Zustand stores (auth + chat)
│   │   ├── api/               # HTTP client
│   │   └── styles/            # Discord dark theme CSS
│   ├── tests/                 # Playwright E2E
│   └── dist/                  # Build output
├── .github/workflows/ci.yml
└── Makefile
```

## Quick Start

```bash
# Install
cd client && npm install
cd server && go mod tidy

# Dev (two terminals or use Makefile)
cd server && CHAT_JWT_SECRET=your-secret go run ./cmd/chatd
cd client && npm run dev

# Open http://localhost:5173 (Vite proxies /api to :8080)
```

## Tests

```bash
# Go 全套测试 (auth/db/handlers/ws)
cd server && go test ./... -cover -count=1 -timeout 120s
```

## API

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/auth/register` | - | Register (email+user+pw) |
| POST | `/api/auth/login` | - | Login → access+refresh JWT |
| POST | `/api/auth/refresh` | - | Refresh access token |
| POST | `/api/auth/logout` | Bearer | Invalidate refresh token |
| GET | `/api/users/me` | Bearer | Current user |
| PATCH | `/api/users/me` | Bearer | Update profile |
| GET | `/api/users?q=` | Bearer | Search users |
| GET | `/api/chats` | Bearer | List my chats (sorted: pinned first) |
| GET | `/api/chats/public` | Bearer | List all public groups (any user can see) |
| POST | `/api/chats` | Bearer | Create group (visibility: public/private) |
| POST | `/api/dms` | Bearer | Get-or-create DM |
| GET | `/api/chats/:id` | Bearer | Chat detail |
| PATCH | `/api/chats/:id` | Bearer | Rename (owner) |
| DELETE | `/api/chats/:id` | Bearer | Delete (owner) |
| GET | `/api/chats/:id/members` | Bearer | Members |
| POST | `/api/chats/:id/members` | Bearer | Add member |
| DELETE | `/api/chats/:id/members/:uid` | Bearer | Kick/Leave |
| POST | `/api/chats/:id/join` | Bearer | Join a public chat |
| POST | `/api/chats/:id/pin` | Bearer | Pin to top |
| POST | `/api/chats/:id/unpin` | Bearer | Unpin |
| GET | `/api/chats/:id/messages` | Bearer | History (before,limit) |
| POST | `/api/chats/:id/messages` | Bearer | Send message |
| PATCH | `/api/chats/:id/messages/:mid` | Bearer | Edit |
| DELETE | `/api/chats/:id/messages/:mid` | Bearer | Delete |
| PUT | `/api/chats/:id/messages/:mid/reactions/:emoji` | Bearer | Add reaction |
| DELETE | `/api/chats/:id/messages/:mid/reactions/:emoji` | Bearer | Remove |
| POST | `/api/uploads` | Bearer | File upload (multipart) |
| GET | `/ws?access_token=` | - | WebSocket gateway |
| GET | `/api/events?access_token=` | - | SSE event stream |

## WS / SSE / Polling

前端支持三种实时模式，可在侧边栏切换：

- **WS**: WebSocket, `GET /ws?access_token=token`
- **SSE**: Server-Sent Events, `GET /api/events?access_token=token`
- **Polling**: HTTP轮询 `/api/chats` + `/api/messages`, 2s 间隔

## WS Protocol

```
Client → Server:  {op:"ping"} / {op:"subscribe"/"unsubscribe","chat_id"} / {op:"typing","chat_id"}
Server → Client:  {op:"pong"} / {op:"ready",payload:{user,chats,online_user_ids}}
                  {op:"message_create"/"message_update"/"message_delete"}
                  {op:"reaction_add"/"reaction_remove"}
                  {op:"chat_create"/"chat_update"/"chat_delete"/"chat_remove"}
                  {op:"presence_update","user_id","status"}
                  {op:"typing","chat_id","user_id"}
```

## Env

| Variable | Default | Description |
|----------|---------|-------------|
| CHAT_ADDR | :8080 | Listen address |
| CHAT_DB_PATH | chat.db | SQLite file |
| CHAT_UPLOAD_DIR | uploads | File upload directory |
| CHAT_JWT_SECRET | random | **生产必须设! 不设每次重启token全失效** |
| CHAT_ACCESS_TTL | 87600h (10yr) | Access token TTL |
| CHAT_REFRESH_TTL | 720h | Refresh token TTL |
| CHAT_MAX_UPLOAD | 20971520 | Max upload size (bytes) |
| CHAT_STATIC_DIR | ../client/dist | Frontend static files |

## Deploy

```
# Backend: wsl-8080.moonchan.xyz
# Frontend: chat-app-fastapi.pages.dev (Cloudflare Pages, auto-deploy on push)

# 后端: 启动时必设 CHAT_JWT_SECRET
cd server && CHAT_JWT_SECRET=your-fixed-secret nohup go run ./cmd/chatd &

# 首次编译 modernc.org/sqlite 需 3-5min
# 用 go run 而非 go build: go build 首次也会卡在 link
```

## ⚠️ 重要：每次修改后必须 Push

每次代码或文档修改完成后，**必须 `git push`**，Cloudflare Pages 会自动重新部署前端。

```
git add -A
git commit -m "描述改动"
git push
```

如果只改了后端，也需要 push（CI 触发重新编译），但后端可手动 `go run` 重启。

---

## Dev Log & Pitfalls

### Architecture decisions
- **chats 表统一 `dm | group`**: 不做 Discord 的 guild → channel 两层,用户说"群聊+私聊放一个list就行"
- **modernc.org/sqlite**: pure Go, no CGO → CI/CD 零依赖交叉编译,但首次编译慢(生成大量Go代码)
- **chi router**: 比 gin 更轻量更 Go 原生
- **JWT 10yr**: access token 10年有效期,无 refresh 刷新机制
- **外链文件上传**: 客户端走 `PUT upload.moonchan.xyz/api/upload` 上传,下载 URL `https://upload.moonchan.xyz/api/{id}/{filename}`

### Pitfalls (踩坑记录)

1. **JWT secret 随机化**: 若不设 `CHAT_JWT_SECRET` 环境变量,每次重启随机生成 secret,所有 token 失效 → 生产必须设置固定值

2. **Timeout 中间件杀 SSE**: chi `Timeout(30s)` 全局使用会掐断 SSE 长连接 → 改为只在 `/api/*` 路由组内生效,WS 和 SSE 不应用

3. **401 多触发**: 旧 token 失效时,多个 API 同时 401 → 多次 `navigate('/login')` 冲突 → 用 useRef 防重入

4. **LSP 先写文件顺序**: handler 引用 ws.Hub,必须先给 ws 包建 stub,否则 LSP 报错但不影响 build

5. **Privoxy 代理**: dev 环境中 curl localhost 被 privoxy 拦截 → 必须加 `--noproxy '*'` 绕过

6. **WS close race**: `close(c.send)` 时若其他 goroutine 还在 `c.send <- env` → panic → 用 `sync.Once` + 不关闭channel修复

7. **WS ready vs presence 顺序**: register 后 `go broadcastPresence` 会异步写 presence → ready 必须先于 presence → 调换 register 顺序(先 queue ready 再 register)

8. **sed 损毁文件**: `sed -i` 删掉了不该删的代码 → `.go` 文件只能用 Edit 工具改,别用 sed

9. **Go binary build hang**: `modernc.org/sqlite` 首次 full link 极慢(~5min+) → CI 用缓存,开发用 `go run`

10. **DB 迁移幂等**: `ALTER TABLE ADD COLUMN IF NOT EXISTS` SQLite 3.37+ 支持,但 safer: Go 层面 catch `duplicate column name` error 并忽略

11. **DisallowUnknownFields**: `json.Decoder` 设置了 `DisallowUnknownFields()` → 新字段(`visibility`)发送到旧 server 报错 → 更新 server 重启即可

12. **测试 CreateChat 签名变更**: 加 `visibility` 参数后 → 所有测试 `CreateChat()` 调用需加 `""` → 用 perl regex 批量替换