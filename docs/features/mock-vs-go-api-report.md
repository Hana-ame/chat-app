# Mock API vs Go API 对照报告

> 检查日期：2026-07-10
> 图例：✅ 对齐　⚠️ 轻微差异（不影响主流程）　❌ 重大差异（行为/安全语义不同）
> 源码：`server/internal/handlers/*.go` 与 `client/src/api/mock.js`

## Auth

| Go 端点 | Go Handler | Mock 函数 | 差异 |
|---------|-----------|-----------|------|
| `POST /api/auth/register` | `Register` | `mockRegister` | ⚠️ 部分差异。mock 校验 email 唯一性、生成 id+avatar_color、返回 `{user,access_token,expires_in}` 均对齐；但 **mock 无 username 格式校验(`ValidateUsername`)、无密码强度校验(`weak_password`)、不哈希密码**；`expires_in` 硬编码 3600，Go 用配置 `AccessTokenTTL` |
| `POST /api/auth/login` | `Login` | `mockLogin` | ❌ 重大差异。**mock 完全不校验密码**（任意密码均可登录），Go 用 `VerifyPassword` 校验哈希。其余（查 users/MOCK_USERS、返回 session）对齐 |
| `POST /api/auth/refresh` | `Refresh` | `mockRefresh` | ⚠️ 差异。mock 不校验 refresh token，直接从 localStorage 取 currentUser 返回新 token；Go 校验 refresh cookie 有效性/过期（缺→400、无效→401、过期→401） |
| `POST /api/auth/logout` | `Logout` | `mockLogout` | ✅ 对齐（均返回 `{ok:true}`）；mock 不真正失效 token（无 cookie 概念，前端清 localStorage 等效） |
| `GET /api/users/me` | `Me` | `mockMe` | ✅ 逻辑等价；mock 无 token 鉴权，fallback `dev-self` |
| `PATCH /api/users/me` | `UpdateMe` | `mockUpdateProfile` | ✅ 对齐（username 唯一性、avatar_color/url fallback、broadcast）；Go 用 `BroadcastUserUpdate` 全局广播，mock 逐 chat 调 `onChatUpdate`（与下方实时事件一致） |
| `GET /api/users?q=` | `SearchUsers` | `mockSearchUsers` | ⚠️ 差异。Go：`LIKE` 前缀匹配 username、排除自己、limit 20、DB 排序；mock：用 `includes` 子串匹配 + 支持 `id===q` 精确匹配、slice 20 **但不排序** |

## Chats

| Go 端点 | Go Handler | Mock 函数 | 差异 |
|---------|-----------|-----------|------|
| `GET /api/chats` | `ListChats` | `mockListChats` | ✅ 对齐。只返回当前用户是 member 的 chat，按 `last_message_at DESC` 排序 |
| `GET /api/chats/public` | `ListPublicChats` | `mockListPublicChats` | ⚠️ 差异。均筛 `visibility='public'`；但 **mock 不排序**（Go 按 `created_at DESC`，由 DB 实现） |
| `POST /api/chats` | `CreateChat` | `mockCreateChat` | ⚠️ 差异。对齐：name 必填、自动加入创建者、返回 chat；**Go 返回 201 Created，mock 返回 200**；Go 支持 `type` 字段且若 `type=dm` 拒绝（走 `/dms`），mock 强制 `type='group'` |
| `POST /api/dms` | `CreateOrGetDM` | `mockCreateDM` | ✅ 基本对齐。先找现有 DM、user_id 不能为空/自己（否则 400）、找不到用 `GetUserByID` 校验（404）；**Go 返回 200(已存在)/201(新建)，mock 统一返回 200** |
| `GET /api/chats/{id}` | `GetChat` | `mockGetChat` | ✅ 对齐。校验 membership，返回完整 chat |
| `PATCH /api/chats/{id}` | `RenameChat` | `mockRenameChat` | ✅ 基本对齐（owner only、DM 不可、返回更新 chat）；微小：Go 不显式校验空 name（依赖 DB），mock 校验 `name` 为空→400 |
| `DELETE /api/chats/{id}` | `DeleteChat` | `mockDeleteChat` | ✅ 对齐（owner only、DM 不可删除、返回 `{ok:true}`） |
| `POST /api/chats/{id}/join` | `JoinChat` | `mockJoinChat` | ✅ 基本对齐（public/unlisted 可加入、private 拒绝 400、返回 `{ok:true}`）；Go 由 DB 约束可见性，mock 显式检查 |
| `POST /api/chats/{id}/announcement` | `PinChat` | `mockSetAnnouncement` | ✅ 对齐（owner only、>=3 members、存 `{content, pinned_at}`、返回 `{ok:true}`） |
| `PATCH /api/chats/{id}/announcement` | `UpdatePinnedChat` | — | ✅ 同 `PinChat`，mock 共用 `mockSetAnnouncement` |
| `DELETE /api/chats/{id}/announcement` | `DeletePinnedChat` | `mockClearAnnouncement` | ✅ 对齐（owner/admin only、clear `pinned_message`、返回 `{ok:true}`） |
| `POST /api/chats/{id}/announcement/read` | `MarkPinnedRead` | `mockMarkAnnouncementRead` | ✅ 对齐。更新 `pinned_last_read_at` |
| `POST /api/chats/{id}/pin` | `PinChatList` | `mockPinChat` | ✅ 对齐。set `pinned=true`，触发 `onChatUpdate` |
| `POST /api/chats/{id}/unpin` | `UnpinChatList` | `mockUnpinChat` | ✅ 对齐。set `pinned=false`，触发 `onChatUpdate` |
| `POST /api/chats/{id}/visit` | `VisitChat` | — | ⚠️ Mock 无对应实现（非关键 UI 路径） |

## Members

| Go 端点 | Go Handler | Mock 函数 | 差异 |
|---------|-----------|-----------|------|
| `GET /api/chats/{id}/members` | `ListMembers` | `mockListMembers` | ✅ 对齐。校验 membership，按 username 排序返回完整 user 列表 |
| `POST /api/chats/{id}/members` | `AddMember` | `mockAddMember` | ✅ 对齐。DM 不可加、已有 member→409、校验 user 存在→404、返回更新 chat |
| `DELETE /api/chats/{id}/members/{userId}` | `RemoveMember` | `mockRemoveMember` | ✅ 对齐。DM 不可踢、不可踢 owner、踢他人需 owner/admin、返回 `{ok:true}`（支持 self-leave） |

## Messages

| Go 端点 | Go Handler | Mock 函数 | 差异 |
|---------|-----------|-----------|------|
| `GET /api/chats/{id}/messages` | `ListMessages` | `mockListMessages` | ✅ 基本对齐。cursor 分页(`before`)、limit cap 100、chronological asc、校验 membership；mock 额外给每条附加 `author`/`reactions.me`/`deleted` 标记（Go message model 已含 author） |
| `POST /api/chats/{id}/messages` | `SendMessage` | `mockSendMessage` | ❌ 重大差异。(1) **mock 无 attachment URL 校验**（Go 强制 `https://upload.moonchan.xyz/` 前缀 + url/filename 必填，否则 400）；(2) **mock 伪造 AI 自动回复**（Go 无此逻辑）；(3) **Go 返回 201，mock 返回 200**。对齐部分：4000 字限制、空消息校验、mention 提取(`<@uuid>`)、更新 `last_message_at`/`last_seen` |
| `PATCH /api/chats/{id}/messages/{msgId}` | `EditMessage` | `mockEditMessage` | ✅ 基本对齐。author only、chat mismatch 检查；mock 额外显式校验空内容+4000 字（Go 由 DB 处理），Go 在 `UpdateMessage` 内校验 author |
| `DELETE /api/chats/{id}/messages/{msgId}` | `DeleteMessage` | `mockDeleteMessage` | ✅ 对齐。author 或 owner/admin 可删、软删除(`deleted_at`)、chat mismatch 检查、返回 `{ok:true}` |
| `POST /api/chats/{id}/read` | `MarkRead` | `mockMarkRead` | ❌ 重大差异。**mock 完全无校验**（无 membership 检查、无 `message_id` 必填校验），直接返回 `{ok:true}`；Go 有 403 membership + 400 `message_id required` |

## Reactions

| Go 端点 | Go Handler | Mock 函数 | 差异 |
|---------|-----------|-----------|------|
| `PUT .../reactions/{emoji}` | `AddReaction` | `mockAddReaction` | ⚠️ 差异。对齐：membership 校验、message 归属(404)、INSERT OR IGNORE、sync reactions JSON、broadcast；**mock 额外校验 emoji 非空 + ≤32 字（Go 无此校验）**；**Go 对 emoji 做 `url.PathUnescape`（mock 不做）** |
| `DELETE .../reactions/{emoji}` | `RemoveReaction` | `mockRemoveReaction` | ✅ 基本对齐（membership 校验、删除、sync、broadcast）；mock 先校验 msg 存在(404)再删，Go 直接 `RemoveReaction` 后 `GetMessage` 返回更新 |
| `GET .../reactions` | `ListReactions` | `mockGetReactions` | ✅ 对齐。聚合 reactions 返回 `{reactions: [{emoji,count,user_ids,me}]}` |

## Uploads

| Go 端点 | Go Handler | Mock 函数 | 差异 |
|---------|-----------|-----------|------|
| `POST /api/uploads` | `Upload` | `mockUpload` | ❌ 重大差异。Go 有 MIME 白名单校验(`allowedMime`)+ 大小限制(`MaxUploadBytes`)+ 落盘 + 返回 `/uploads/{key}`；mock 用 `URL.createObjectURL` 无校验无限流。另：该 Go handler 已标 **Deprecated**（前端直传 `upload.moonchan.xyz`），实际不会被调用 |
| — | — | `mockUploadAvatar` | ⚠️ Mock only。前端通过 `upload` 上传后提取 URL，无对应独立 Go handler |

## 实时事件 (Real-time)

| Go Event | Mock 通知方式 | 差异 |
|----------|---------------|------|
| `message_create` | `_store.getState().onMessageCreate(msg)` | ✅ 对齐（store 方法存在，见 `client/src/store/chat.js:225`） |
| `message_update` | `_store.getState().onMessageUpdate(msg)` | ✅ 对齐（`chat.js:256`） |
| `message_delete` | `_store.getState().onMessageDelete(payload)` | ✅ 对齐（`chat.js:260`） |
| `reaction_add` | `_store.getState().onReaction(payload, true)` | ✅ 对齐（`chat.js:264`） |
| `reaction_remove` | `_store.getState().onReaction(payload, false)` | ✅ 对齐（`chat.js:264`） |
| `chat_create` | `_store.getState().onChatUpdate(chat)` | ✅ 对齐（`chat.js:183`） |
| `chat_update` | `_store.getState().onChatUpdate(chat)` | ✅ 对齐（`chat.js:183`） |
| `chat_delete` | `_store.getState().onChatUpdate({id, deleted: true})` | ✅ 对齐 |
| `user_update` | `_store.getState().onChatUpdate({id, members})` 在每个 chat 上 | ⚠️ 差异。Go 用 `BroadcastUserUpdate` 全局广播；mock 逐 chat 遍历通知（语义等价，但 mock 仅通知当前用户已加入的 chat） |
| `presence_update` | 未实现 | ❌ 差异。Go 有 WS 连接/断开的 online/offline broadcast；mock 模式无 presence |
| `pin` | `_store.getState().onChatUpdate({id, pinned: true})` | ✅ 对齐（PinChatList 广播 chat_update，mock 直接调 onChatUpdate） |
| `unpin` | `_store.getState().onChatUpdate({id, pinned: false})` | ✅ 对齐 |
| `announcement_read` | `_store.getState().onChatUpdate({id, pinned_last_read_at})` | ✅ 对齐 |

## 未实现 (不涉及)

- `GET /healthz` — 前端不调用
- `GET /ws` — mock 模式走 memory-polling，不建 WS
- `GET /api/events` — mock 模式不走 SSE
- `GET /swagger/*` — 纯 API 文档
- `GET /api/version` — 前端不直接调用
- `GET /uploads/*` — 废弃
- `GET /` (静态文件) — Go 有 SPA fallback
- `POST /api/chats/{id}/visit` — mock 无对应实现

## 实现细节与逻辑区别分析

> 本节从内部实现角度，剖析 mock API（内存态 JS）与 Go 真实 API（SQLite + WS hub）的深层逻辑差异。源码：`server/internal/handlers/*.go`、`server/internal/models/models.go`、`client/src/api/mock.js`、`client/src/store/chat.js`。

### 1. 身份与鉴权模型（根本差异）

| 维度 | Go 真实 API | Mock API |
|------|------------|----------|
| "当前用户"来源 | `userFrom(r.Context())` —— 从 JWT access token 解析 | `currentUser()` —— 读 `localStorage.auth.user`，**失败则 fallback 到 `userById('dev-self')`（Alice）** |
| 密码 | `HashPassword` + `VerifyPassword`（bcrypt 类哈希） | **完全忽略密码**，注册/登录只比对 email |
| Token | 真实 JWT（`IssueAccessToken`）+ httpOnly refresh cookie，DB 存 refresh hash | 返回字符串 `'mock-token-' + id`，无 cookie、无过期校验 |
| Refresh | 校验 refresh cookie 哈希、过期时间、删旧发新 | 直接拿 localStorage 的 user 发新 token，**任何调用都成功** |
| 未登录 | 中间件 401 | 静默伪装成 dev-self（等同于"无需登录"） |

**逻辑后果**：mock 模式下"你是谁"由 localStorage 决定，缺失时恒为 Alice；Go 由 token 决定。任何依赖"多用户隔离"的测试在 mock 下失真（如 A 用户看不到 B 的私有 chat —— mock 靠 `c.members` 数组模拟，但 AI 回复、广播都只针对单客户端）。

### 2. 数据持久化与状态表示

- **Go**：单一事实源 = SQLite（`users` / `chats` / `chat_members` / `messages` / `reactions` / `refresh_tokens` 表，`modernc.org/sqlite` 纯 Go 驱动）。每次写都走 DB，多客户端可见。
- **Mock**：内存 JS 对象 `data = { users, chats, messages, reactions, chatMembers }`，进程重启即丢（但 `ensureData()` 每次重新 `generateDummyData` 生成 10 chat × 150 消息的假数据）。

**双数据源问题**：Mock 的 chat 成员同时存在于两处——
1. `c.members`（chat 对象上的数组，`buildChatResponse` 用它算 `member_count`、`isMember`）
2. `d.chatMembers`（扁平 `{chat_id,user_id,role,joined_at}`，`mockListChats` / `mockAddMember` / `mockRemoveMember` 用它）

二者靠手写同步，易不一致。例如 `mockCreateChat` 只往 `chatMembers` push owner，但 `buildChatResponse` 的 `member_count` 优先取 `c.members.length`（新建时 `c.members` 是 `{id, role}` 数组，长度=成员数，暂时一致；但后续 `mockAddMember` 同时改两处，若某路径漏改就会偏离）。**Go 只有 `chat_members` 一张表，无此风险。**

### 3. 错误模型（对齐较好）

双方错误码字符串基本一致：`already_taken` / `invalid_credentials` / `forbidden` / `not_found` / `bad_request` / `content_too_long` / `user_not_found` / `already_member`。Mock `throw {status,error,message}`，Go `writeError(w,status,code,msg)`，前端应统一映射。✅ 这点做得好。

### 4. 响应字段形状

对照 `models.Chat` / `models.Message`：

- **Chat**：mock `buildChatResponse` 产出的字段（`id,type,name,icon_color,visibility,owner_id,created_at,last_message_at,member_count,unread_count,pinned_message,last_message_id,last_message`）与 Go `Chat` 结构体高度一致，包括已废弃的 `unread_count` / `last_message`。⚠️ 但 mock 的 `unread_count` **恒为 0**（data 从不设置），Go 虽 deprecated 但可能有值；且 mock 的 `member_count` 来自易失的 `c.members` 数组。
- **Message**：Go `Message` 把 `author` / `attachments` / `reactions` / `mentions` 存为 JSON 列（`author` 已废弃）。Mock 在 `mockListMessages` 里**实时**给每条补 `author`（调用 `userById`）、`reactions.me`（按 `user_ids.includes(cu.id)`）、`deleted` 布尔。形状兼容，但 mock 额外注入 `me` / `deleted` 等前端友好字段。

### 5. 分页 / 游标语义

- Go：`GetMessages(id, before, limit)` —— DB 做基于 `before` 的游标分页，`limit` 默认 50。
- Mock：`messagesFor` 按 `created_at` 升序后：
  - 有 `before`：找该消息 index，`slice(start, idx)`；**若 idx<=0（消息不存在或是第一条）直接返回空数组**。
  - 无 `before`：取最后 `min(limit,100)` 条（cap 100 与 Go 对齐）。
- ⚠️ 差异：mock 的游标"上一页"边界处理（找不到 before 即空）与 Go 的 DB `< created_at` 语义不同；且 mock 排序依赖 JS `new Date()` 字符串比较，时区/精度不如 DB。

### 6. 副作用：AI 自动回复（最大行为差异）

`mockSendMessage` 在用户发消息后，用 `setTimeout` 注入一条 `user_id:'ai'` 的流式回复（`streaming:true` + `source` 异步 emit）。**Go 完全没有这层逻辑**（真实后端不会自言自语）。这导致：
- mock 下消息列表 / 未读 / `last_message_at` 会被 AI 消息改动；
- 流式消息的 `source` 闭包只在内存生效，Go 无对应概念。

### 7. 实时广播语义（单客户端 vs 多客户端）

- **Go**：`s.Hub` WS hub，`BroadcastMessageCreate` 等推给**所有**连上该 chat 的客户端；`BroadcastUserUpdate` **全局**广播（含其他用户的会话）。
- **Mock**：`mockSendMessage` 等直接调 `_store.getState().onMessageCreate(...)` —— **只更新当前这个浏览器客户端**。
- `user_update` 尤甚：Go 全局广播，mock 仅遍历"当前用户已加入的 chat"逐个 `onChatUpdate({id,members})`，且只改当前用户自己的 member 对象。其他"用户"在 mock 里根本不存在第二客户端。
- ❌ `presence_update`：Go 由 WS 连接/断开产生 online/offline；mock 完全没有。

### 8. 缺失 / 不对称的业务校验

| 校验 | Go | Mock |
|------|----|------|
| 密码强度 (`weak_password`) | ✅ | ❌ 忽略 |
| 登录密码比对 | ✅ `VerifyPassword` | ❌ 任意密码 |
| 附件 URL 必须在 `upload.moonchan.xyz` + url/filename 必填 | ✅ 400 | ❌ 直接透传 blob URL |
| `MarkRead` 的 membership + `message_id` 必填 | ✅ 403/400 | ❌ 直接 `{ok:true}` |
| `AddReaction` emoji 空 / ≤32 字 + URL unescape | ❌ Go 不校验（mock 反而更严） | ✅ 校验 |
| `Register` username 格式 (`ValidateUsername`) | ✅ | ❌ 忽略 |

值得注意：**有一处 mock 比 Go 更严格** —— `AddReaction` 的 emoji 长度 / 空校验，Go 端缺失（潜在不一致，若前端依赖 Go 拒绝超长 emoji 则会失败）。

### 9. 并发 / 竞态

- Go 在 `Refresh` / `Logout` 有文档化的 `refreshMu` 竞态（注释已说明为 low-risk 接受）。
- Mock 是单线程 JS 事件循环，`ensureData` 内数组操作无锁也安全，但代价是**没有真正的并发语义**，无法验证 Go 的竞态处理。

### 一句话总结

Mock 是"单用户、内存态、无鉴权、带 AI 彩蛋"的**行为近似器**：权限分支（owner/admin/DM/成员）和错误码基本对齐，但**身份来源、密码校验、附件校验、MarkRead 校验、多客户端实时广播、presence** 这几处真实后端的核心逻辑在 mock 中要么被简化、要么缺失。它适合跑 UI / 交互，不适合验证安全边界与多用户一致性。

## 总结

Go API 与 Mock API 已逐一对照（共 21 个 handler + 9 类实时事件）。**绝大多数权限校验、数据格式、错误码、排序、分页、软删除、reactions JSON cache 逻辑均已对齐**。但检查暴露出以下需关注项：

**❌ 重大差异（建议在 mock 中补齐或前端调用时知悉）：**
1. `Login`：mock 不校验密码（安全风险仅限本地 dev，但会导致"任意密码登录"行为与 Go 不一致）。
2. `SendMessage`：mock 无 attachment URL 校验、伪造 AI 回复、返回 200（Go 201）。
3. `MarkRead`：mock 完全无校验（缺 membership / message_id 校验）。
4. `Upload`：handler 已 Deprecated，mock 与 Go 实现本就不同路径，不具可比性。

**⚠️ 轻微差异：** Register 缺 username/password 校验与 expires_in 配置化；Refresh 无 token 校验；SearchUsers 排序缺失；ListPublicChats 不排序；CreateChat 状态码 201 vs 200 且强制 group；AddReaction 缺 emoji 空/长度校验与 URL unescape；user_update 广播范围。

**未实现：** `presence_update`（presence/online-offline）在 mock 中缺失。
