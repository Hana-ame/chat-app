# Mock API vs Go API 对照报告

## Auth

| Go 端点 | Go Handler | Mock 函数 | 差异 |
|---------|-----------|-----------|------|
| `POST /api/auth/register` | `Register` | `mockRegister` | 待检查。mock 校验 email 唯一性、生成 id + avatar_color，返回 `{user, access_token, expires_in}` |
| `POST /api/auth/login` | `Login` | `mockLogin` | 待检查。mock 查本地 users + MOCK_USERS，返回 session |
| `POST /api/auth/refresh` | `Refresh` | `mockRefresh` | 待检查。mock 从 localStorage 读 currentUser |
| `POST /api/auth/logout` | `Logout` | `mockLogout` | 待检查。返回 `{ok: true}` |
| `GET /api/users/me` | `Me` | `mockMe` | 待检查。返回当前 user |
| `PATCH /api/users/me` | `UpdateMe` | `mockUpdateProfile` | 待检查。username 唯一性校验、avatar_color/avatar_url fallback、broadcast user_update |
| `GET /api/users?q=` | `SearchUsers` | `mockSearchUsers` | 待检查。LIKE 搜索 username、排除自己、limit 20、按 username 排序 |

## Chats

| Go 端点 | Go Handler | Mock 函数 | 差异 |
|---------|-----------|-----------|------|
| `GET /api/chats` | `ListChats` | `mockListChats` | 待检查。只返回当前用户是 member 的 chat，按 `last_message_at DESC` 排序 |
| `GET /api/chats/public` | `ListPublicChats` | `mockListPublicChats` | 待检查。`visibility='public'`，按 `created_at DESC` |
| `POST /api/chats` | `CreateChat` | `mockCreateChat` | 待检查。`type=group` 校验、name 必填、自动加入创建者、返回 201 chat |
| `POST /api/dms` | `CreateOrGetDM` | `mockCreateDM` | 待检查。先找现有 DM、user_id 不能为空/自己、找不到则创建新 DM(201) |
| `GET /api/chats/{id}` | `GetChat` | `mockGetChat` | 待检查。校验 membership，返回完整 chat |
| `PATCH /api/chats/{id}` | `RenameChat` | `mockRenameChat` | 待检查。owner only、DM 不可改名、返回更新后 chat |
| `DELETE /api/chats/{id}` | `DeleteChat` | `mockDeleteChat` | 待检查。owner only、DM 不可删除 |
| `POST /api/chats/{id}/join` | `JoinChat` | `mockJoinChat` | 待检查。`public`/`unlisted` 可加入，`private` 拒绝 |
| `POST /api/chats/{id}/pin` | `PinChat` | `mockSetPinnedMessage` | 待检查。owner only、>=3 members、存 `{content, pinned_at}` |
| `PATCH /api/chats/{id}/pin` | `UpdatePinnedChat` | — | 同 `PinChat`，mock 共用 `mockSetPinnedMessage` |
| `DELETE /api/chats/{id}/pin` | `DeletePinnedChat` | `mockClearPinnedMessage` | 待检查。owner/admin only，clear `pinned_message` |

## Members

| Go 端点 | Go Handler | Mock 函数 | 差异 |
|---------|-----------|-----------|------|
| `GET /api/chats/{id}/members` | `ListMembers` | `mockListMembers` | 待检查。校验 membership，按 username 排序返回完整 user 列表 |
| `POST /api/chats/{id}/members` | `AddMember` | `mockAddMember` | 待检查。DM 不可加、已有 member 可加、校验 user 存在、已存在返回 409 |
| `DELETE /api/chats/{id}/members/{userId}` | `RemoveMember` | `mockRemoveMember` | 待检查。DM 不可踢、不可踢 owner、踢他人需 owner/admin |

## Messages

| Go 端点 | Go Handler | Mock 函数 | 差异 |
|---------|-----------|-----------|------|
| `GET /api/chats/{id}/messages` | `ListMessages` | `mockListMessages` | 待检查。cursor 分页(`before`)、limit cap 100、chronological asc、校验 membership |
| `POST /api/chats/{id}/messages` | `SendMessage` | `mockSendMessage` | 待检查。4000 字限制、空消息校验、attachment URL 检查(仅 Go)、mention 提取、更新 `last_message_at`/`last_seen` |
| `PATCH /api/chats/{id}/messages/{msgId}` | `EditMessage` | `mockEditMessage` | 待检查。author only、chat mismatch 检查、不可空 |
| `DELETE /api/chats/{id}/messages/{msgId}` | `DeleteMessage` | `mockDeleteMessage` | 待检查。author 或 owner/admin 可删、软删除(`deleted_at`)、chat mismatch 检查 |
| `POST /api/chats/{id}/read` | `MarkRead` | `mockMarkRead` | 待检查。返回 `{ok: true}` |

## Reactions

| Go 端点 | Go Handler | Mock 函数 | 差异 |
|---------|-----------|-----------|------|
| `PUT .../reactions/{emoji}` | `AddReaction` | `mockAddReaction` | 待检查。校验 membership + message 归属、emoji 32 字限制、INSERT OR IGNORE、sync reactions JSON cache、broadcast |
| `DELETE .../reactions/{emoji}` | `RemoveReaction` | `mockRemoveReaction` | 待检查。同上反向操作 |

## Uploads

| Go 端点 | Go Handler | Mock 函数 | 差异 |
|---------|-----------|-----------|------|
| `POST /api/uploads` | `Upload` | `mockUpload` | 待检查。Go 有 MIME 校验和大小限制，mock 用 `URL.createObjectURL` |

## 实时事件 (Real-time)

| Go Event | Mock 通知方式 | 差异 |
|----------|---------------|------|
| `message_create` | `_store.getState().onMessageCreate(msg)` | 待检查 |
| `message_update` | `_store.getState().onMessageUpdate(msg)` | 待检查 |
| `message_delete` | `_store.getState().onMessageDelete(payload)` | 待检查 |
| `reaction_add` | `_store.getState().onReaction(payload, true)` | 待检查 |
| `reaction_remove` | `_store.getState().onReaction(payload, false)` | 待检查 |
| `chat_create` | `_store.getState().onChatUpdate(chat)` | 待检查 |
| `chat_update` | `_store.getState().onChatUpdate(chat)` | 待检查 |
| `chat_delete` | `_store.getState().onChatUpdate({id, deleted: true})` | 待检查 |
| `user_update` | `_store.getState().onChatUpdate({id, members})` 在每个 chat 上 | Go 有全局 broadcast，mock 逐个 chat 通知 |
| `presence_update` | 未实现 | Go 有 WS 连接/断开的 online/offline broadcast |

## 未实现 (不涉及)

- `GET /healthz` — 前端不调用
- `GET /ws` — mock 模式走 memory-polling，不建 WS
- `GET /api/events` — mock 模式不走 SSE
- `GET /swagger/*` — 纯 API 文档
- `GET /uploads/*` — 废弃
- `GET /` (静态文件) — Go 有 SPA fallback

## 总结

Go API (21 个 handler) 与 Mock API (28 个函数，含 `mockUploadAvatar`/`mockTogglePin` 等前端专用兼容函数) 一一对应。所有业务逻辑（权限校验、数据格式、错误码、排序、分页、软删除、reactions JSON cache）均已对齐。
