# API 接口规范 (API Handlers Spec)

> 原始来源：`server/internal/handlers/`
> 路由定义：`router.go`

---

## 一、依赖组件

```go
type Server struct {
    Cfg       *config.Config   // 系统配置
    DB        *db.DB           // 数据访问层
    Auth      *auth.Service    // 认证服务
    Hub       *ws.Hub          // 实时广播（可选，可为 nil）
}
```

---

## 二、路由总表

### 2.1 公开路由（无需认证）

| # | 方法 | 路径 | 处理器 | 说明 |
|---|------|------|--------|------|
| 1 | `GET` | `/healthz` | inline | 健康检查 → `{"status":"ok"}` |
| 2 | `POST` | `/api/auth/register` | `Register` | 注册 |
| 3 | `POST` | `/api/auth/login` | `Login` | 登录 |
| 4 | `POST` | `/api/auth/refresh` | `Refresh` | 刷新 Token |

### 2.2 认证路由（需 `Bearer` Token）

| # | 方法 | 路径 | 处理器 | 说明 |
|---|------|------|--------|------|
| 5 | `POST` | `/api/auth/logout` | `Logout` | 登出 |
| 6 | `GET` | `/api/users/me` | `Me` | 获取当前用户 |
| 7 | `PATCH` | `/api/users/me` | `UpdateMe` | 更新个人资料 |
| 8 | `GET` | `/api/users` | `SearchUsers` | 搜索用户 |
| 9 | `GET` | `/api/chats` | `ListChats` | 我的聊天列表 |
| 10 | `GET` | `/api/chats/public` | `ListPublicChats` | 公开聊天列表 |
| 11 | `POST` | `/api/chats` | `CreateChat` | 创建群聊 |
| 12 | `POST` | `/api/dms` | `CreateOrGetDM` | (Deprecated) 创建/查找 DM |
| 13 | `GET` | `/api/chats/{chatID}` | `GetChat` | 聊天详情 |
| 14 | `PATCH` | `/api/chats/{chatID}` | `RenameChat` | 重命名聊天 |
| 15 | `DELETE` | `/api/chats/{chatID}` | `DeleteChat` | 删除聊天 |
| 16 | `GET` | `/api/chats/{chatID}/members` | `ListMembers` | 成员列表 |
| 17 | `POST` | `/api/chats/{chatID}/members` | `AddMember` | 添加成员 |
| 18 | `DELETE` | `/api/chats/{chatID}/members/{userID}` | `RemoveMember` | 移除成员 |
| 19 | `POST` | `/api/chats/{chatID}/read` | `MarkRead` | (Deprecated) 标记已读 |
| 20 | `GET` | `/api/chats/{chatID}/messages` | `ListMessages` | 消息列表 |
| 21 | `POST` | `/api/chats/{chatID}/messages` | `SendMessage` | 发送消息 |
| 22 | `PATCH` | `/api/chats/{chatID}/messages/{messageID}` | `EditMessage` | 编辑消息 |
| 23 | `DELETE` | `/api/chats/{chatID}/messages/{messageID}` | `DeleteMessage` | 删除消息 |
| 24 | `PUT` | `/api/chats/{chatID}/messages/{messageID}/reactions/{emoji}` | `AddReaction` | 添加反应 |
| 25 | `DELETE` | `/api/chats/{chatID}/messages/{messageID}/reactions/{emoji}` | `RemoveReaction` | 移除反应 |
| 26 | `POST` | `/api/chats/{chatID}/join` | `JoinChat` | 加入公开聊天 |
| 27 | `POST` | `/api/chats/{chatID}/pin` | `PinChat` | 设置置顶消息 |
| 28 | `PATCH` | `/api/chats/{chatID}/pin` | `UpdatePinnedChat` | 更新置顶消息 |
| 29 | `DELETE` | `/api/chats/{chatID}/pin` | `DeletePinnedChat` | 清除置顶消息 |
| 30 | `POST` | `/api/uploads` | `Upload` | (Deprecated) 本地上传 |

### 2.3 额外路由（不经过认证中间件）

| # | 方法 | 路径 | 处理器 | 说明 |
|---|------|------|--------|------|
| 31 | `GET` | `/ws` | `Gateway.ServeHTTP` | WebSocket 升级入口 |
| 32 | `GET` | `/api/events` | `SSE` | SSE 事件流 |
| 33 | `GET` | `/swagger/*` | httpSwagger | Swagger 文档 |
| 34 | `GET` | `/uploads/*` | `serveUpload` | (Deprecated) 静态文件服务 |
| 35 | `*` | `/*` | `serveStatic` | SPA 静态文件兜底 |

---

## 三、认证流程

### 3.1 认证中间件 (`authMiddleware`)

1. 从请求头 `Authorization: Bearer <token>` 或 URL 参数 `access_token` 或 Cookie `access_token` 获取 Token。
2. 调用 `Auth.ParseAccessToken` 校验 JWT。
3. 通过 `DB.GetUserByID` 加载用户信息到 Context。

### 3.2 Token 刷新 (`Refresh`)

1. 从 Cookie `refresh_token` 读取原始 Refresh Token。
2. 计算 `sha256(raw)` → 查找数据库记录。
3. 检查有效期，过期则删除并返回 401。
4. 删除旧 Token，**单次使用**策略。
5. 签发新 Session（新 Access + Refresh Token）。

### 3.3 Cookie 设置 (`issueSession`)

- `access_token`：HttpOnly=false，Path="/"，同 Config.AccessTokenTTL。
- `refresh_token`：HttpOnly=true，Path="/"，同 Config.RefreshTokenTTL。

---

## 四、各端点详细逻辑

### 4.1 用户/认证

#### `POST /api/auth/register`
- 验证 Email 格式（NormalizeEmail）+ Username 非空。
- `auth.HashPassword` → `DB.CreateUser`。
- 调用 `issueSession` 签发 Token。

#### `GET /api/users` (SearchUsers)
- 查询参数 `q`（min 1 char）。
- `DB.SearchUsers(q, 20)` → 过滤掉当前用户。

---

### 4.2 聊天

#### `POST /api/chats` (CreateChat)
- 校验 `type` 为 `group`、`name` 非空。
- 若 `member_ids` 不包含当前用户，自动追加。
- `DB.CreateChat` → `Hub.BroadcastChatCreated`。

#### `GET /api/chats/{chatID}` (GetChat)
- `IsChatMember` 检查权限 → `DB.GetChat`。

#### `POST /api/chats/{chatID}/join` (JoinChat)
- `DB.JoinChatByID` → 仅允许 `public` 或 `unlisted` 聊天。
- `Hub.NotifyUserNewChat` + `Hub.BroadcastChatUpdated`。

---

### 4.3 成员管理

#### `POST /api/chats/{chatID}/members` (AddMember)
- 仅限群聊、现有成员可操作。
- 验证目标用户存在 → `DB.AddChatMember`。
- `Hub.BroadcastChatUpdated` + `Hub.NotifyUserNewChat`。

#### `DELETE /api/chats/{chatID}/members/{userID}` (RemoveMember)
- 移除自己或 Owner 移除他人（Owner 不可被踢）。
- `DB.RemoveChatMember` → `Hub.NotifyUserLeftChat` + `Hub.BroadcastChatUpdated`。

---

### 4.4 置顶消息

#### `POST /api/chats/{chatID}/pin`
- `requireOwnerOrAdmin` → 检查成员数 ≥ 3。
- `DB.SetPinnedMessage`。

#### `DELETE /api/chats/{chatID}/pin`
- `requireOwnerOrAdmin` → `DB.ClearPinnedMessage`。

---

### 4.5 上传 (Deprecated)

#### `POST /api/uploads`
- 限制文件大小 ≤ `Config.MaxUploadBytes`。
- 校验 MIME 类型（白名单）。
- 生成随机文件名 → 保存到 `UploadDir`。
- 返回 `{id, url, filename, mime_type, size}`。

---

### 4.6 SSE

#### `GET /api/events`
- 验证 Token → 发送初始 `ready` 事件。
- 注册到 `Hub.SSERegister` → 监听广播信道。
- 客户端断开时 `Hub.SSEUnregister`。
