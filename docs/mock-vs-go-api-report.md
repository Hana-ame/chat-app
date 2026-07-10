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
| `POST /api/chats/{id}/pin` | `PinChat` | `mockSetPinnedMessage` | ✅ 对齐（owner only、>=3 members、存 `{content, pinned_at}`、返回 `{ok:true}`） |
| `PATCH /api/chats/{id}/pin` | `UpdatePinnedChat` | — | ✅ 同 `PinChat`，mock 共用 `mockSetPinnedMessage` |
| `DELETE /api/chats/{id}/pin` | `DeletePinnedChat` | `mockClearPinnedMessage` | ✅ 对齐（owner/admin only、clear `pinned_message`、返回 `{ok:true}`） |

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

## Uploads

| Go 端点 | Go Handler | Mock 函数 | 差异 |
|---------|-----------|-----------|------|
| `POST /api/uploads` | `Upload` | `mockUpload` | ❌ 重大差异。Go 有 MIME 白名单校验(`allowedMime`)+ 大小限制(`MaxUploadBytes`)+ 落盘 + 返回 `/uploads/{key}`；mock 用 `URL.createObjectURL` 无校验无限流。另：该 Go handler 已标 **Deprecated**（前端直传 `upload.moonchan.xyz`），实际不会被调用 |

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

## 未实现 (不涉及)

- `GET /healthz` — 前端不调用
- `GET /ws` — mock 模式走 memory-polling，不建 WS
- `GET /api/events` — mock 模式不走 SSE
- `GET /swagger/*` — 纯 API 文档
- `GET /uploads/*` — 废弃
- `GET /` (静态文件) — Go 有 SPA fallback

## 总结

Go API 与 Mock API 已逐一对照（共 21 个 handler + 9 类实时事件）。**绝大多数权限校验、数据格式、错误码、排序、分页、软删除、reactions JSON cache 逻辑均已对齐**。但检查暴露出以下需关注项：

**❌ 重大差异（建议在 mock 中补齐或前端调用时知悉）：**
1. `Login`：mock 不校验密码（安全风险仅限本地 dev，但会导致"任意密码登录"行为与 Go 不一致）。
2. `SendMessage`：mock 无 attachment URL 校验、伪造 AI 回复、返回 200（Go 201）。
3. `MarkRead`：mock 完全无校验（缺 membership / message_id 校验）。
4. `Upload`：handler 已 Deprecated，mock 与 Go 实现本就不同路径，不具可比性。

**⚠️ 轻微差异：** Register 缺 username/password 校验与 expires_in 配置化；Refresh 无 token 校验；SearchUsers 排序缺失；ListPublicChats 不排序；CreateChat 状态码 201 vs 200 且强制 group；AddReaction 缺 emoji 空/长度校验与 URL unescape；user_update 广播范围。

**未实现：** `presence_update`（presence/online-offline）在 mock 中缺失。
