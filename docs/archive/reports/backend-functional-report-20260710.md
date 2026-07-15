# 后端功能规格报告

报告日期：2026-07-10
版本：v1.1
审查范围：`server/internal/` (handlers, db, auth, ws, models)

---

## 1. 核心功能模块

### 1.1 用户与认证系统 (`/api/auth`, `/api/users`)
后端实现了完整的基于 JWT 的会话管理机制。

| 功能 | 端点/逻辑 | 参考代码 | 描述 |
|------|----------|----------|------|
| **用户注册** | `POST /api/auth/register` | `server/internal/handlers/auth.go:36` | 创建新用户，存储哈希化密码。 |
| **用户登录** | `POST /api/auth/login` | `server/internal/handlers/auth.go:73` | 验证凭据，签发 `access_token` (短效) 和 `refresh_token` (长效)。 |
| **Token 刷新** | `POST /api/auth/refresh` | `server/internal/handlers/auth.go:129` | 使用 refresh token 换取新的 access token。 |
| **用户登出** | `POST /api/auth/logout` | `server/internal/handlers/auth.go:172` | 使当前 refresh token 失效并清除客户端 Cookie。 |
| **个人资料** | `GET/PATCH /api/users/me` | `server/internal/handlers/auth.go:196`, `server/internal/handlers/users.go:25` | 获取及更新当前登录用户的基本信息（用户名、头像颜色等）。 |
| **用户搜索** | `GET /api/users` | `server/internal/handlers/users.go:77` | 根据关键字模糊搜索注册用户。 |

### 1.2 聊天室管理 (`/api/chats`)
支持私聊 (DM) 和群聊 (Group) 两种模式。

| 功能 | 端点 | 参考代码 | 描述 |
|------|------|----------|------|
| **创建聊天** | `POST /api/chats` | `server/internal/handlers/chat.go:57` | 创建群聊或初始化私聊会话。 |
| **获取列表** | `GET /api/chats/my` | `server/internal/handlers/chat.go:39` | 获取当前用户加入的所有聊天室及其最后一条消息预览。 |
| **公开频道** | `GET /api/chats/public` | `server/internal/handlers/chat.go:249` | 列出所有可见性为 `public` 的公开群聊。 |
| **聊天详情** | `GET /api/chats/{id}` | `server/internal/handlers/chat.go:147` | 获取指定聊天室的元数据及成员概览。 |
| **管理会话** | `PATCH/DELETE /api/chats/{id}` | `server/internal/handlers/chat.go:176`, `server/internal/handlers/chat.go:216` | 重命名聊天室或删除会话。 |
| **置顶通知** | `POST/PATCH/DELETE /api/chats/{id}/pin` | `server/internal/handlers/chat.go:316`, `server/internal/handlers/chat.go:362`, `server/internal/handlers/chat.go:374` | 设置、更新或清除聊天室顶部的公告 (Pinned Notice)。 |

### 1.3 成员管理 (`/api/chats/{id}/members`)
控制用户对聊天室的访问权限。

| 功能 | 端点 | 参考代码 | 描述 |
|------|------|----------|------|
| **成员列表** | `GET /api/chats/{id}/members` | `server/internal/handlers/member.go:23` | 列出所有成员及其角色 (Owner/Admin/Member)。 |
| **添加成员** | `POST /api/chats/{id}/members` | `server/internal/handlers/member.go:48` | 将指定用户邀请进聊天室。 |
| **移除成员** | `DELETE /api/chats/{id}/members/{uid}` | `server/internal/handlers/member.go:99` | 将成员从聊天室中剔除。 |
| **加入频道** | `POST /api/chats/{id}/join` | `server/internal/handlers/chat.go:266` | 用户主动申请/加入公开频道。 |

### 1.4 消息与交互 (`/api/chats/{id}/messages`)
实现高效的消息流转与轻量级交互。

| 功能 | 端点 | 参考代码 | 描述 |
|------|------|----------|------|
| **发送消息** | `POST /api/chats/{id}/messages` | `server/internal/handlers/messages.go:78` | 发送文本消息，支持附件 URL。包含 4000 字符长度限制。 |
| **消息历史** | `GET /api/chats/{id}/messages` | `server/internal/handlers/messages.go:50` | 分页获取消息历史，包含聚合后的 reactions 和 attachments。 |
| **编辑消息** | `PATCH /api/chats/{id}/messages/{mid}` | `server/internal/handlers/messages.go:131` | 修改已发送的消息内容。 |
| **删除消息** | `DELETE /api/chats/{id}/messages/{mid}` | `server/internal/handlers/messages.go:174` | 逻辑删除消息 (`deleted_at` 标记)。 |
| **表情响应** | `PUT/DELETE .../reactions/{emoji}` | `server/internal/handlers/reactions.go:20`, `server/internal/handlers/reactions.go:61` | 为消息添加或移除特定的 Emoji 响应。 |
| **已读标记** | `POST /api/chats/{id}/read` | `server/internal/handlers/messages.go:218` | 更新用户的 `last_read_message_id`。 |

### 1.5 实时通信 (Real-time)
提供三种层级的实时数据分发方案。

- **WebSocket (`/ws`)**: `server/internal/ws/hub.go` (核心分发逻辑) / `server/internal/handlers/router.go:105`
- **SSE (`/api/events`)**: `server/internal/handlers/sse.go:17`
- **Polling (API)**: `server/internal/handlers/chat.go:39` (通过定时调用 `ListChats` 模拟)

---

## 2. 技术实现细节

### 2.1 数据存储架构
使用 **SQLite (WAL 模式)** 存储，采用高度扁平化的 JSON 缓存策略以消除 N+1 查询。

- **JSON 聚合列**: `server/internal/db/migrations/init.sql:116-118` (reactions, attachments, mentions 缓存列)
- **计数冗余**: `server/internal/db/migrations/init.sql:106-108` (reaction_count 等冗余字段)
- **索引优化**: `server/internal/db/migrations/init.sql:34-35`, `server/internal/db/migrations/init.sql:63`, `server/internal/db/migrations/init.sql:102-103`

### 2.2 安全与防御机制
- **速率限制 (Rate Limiting)**: `server/internal/handlers/router.go:63`, `server/internal/handlers/router.go:65-66`, `server/internal/handlers/router.go:139`
- **安全头**: `server/internal/handlers/router.go:28-35` (CSP & nosniff)
- **跨域控制**: `server/internal/handlers/router.go:39-46` (CORS)
- **输入清洗**: `server/internal/handlers/messages.go:105-113` (长度与 URL 限制)
