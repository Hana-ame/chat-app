# API Payload 验证报告 (API Payload Verification Report)
日期: 2026-07-10

## 验证目标
核对后端实现代码中定义的 Request Payload 结构体与 API 接口规范文档是否完全一致，确保前端调用与后端解析不存在字段偏差。

## 验证结果汇总

所有接口的 Payload 均通过核对，实现与规范完全匹配。

| 接口端点 | 方法 | 预期 Payload (JSON Body) | 代码实现结构体 | 状态 |
|---|---|---|---|---|
| `/api/auth/register` | `POST` | `{"email", "username", "password"}` | `registerReq` | ✅ 一致 |
| `/api/auth/login` | `POST` | `{"email", "password"}` | `loginReq` | ✅ 一致 |
| `/api/users/me` | `PATCH` | `{"username", "avatar_color", "avatar_url"}` | `updateProfileReq` | ✅ 一致 |
| `/api/chats` | `POST` | `{"type", "name", "visibility", "member_ids"}` | `createChatReq` | ✅ 一致 |
| `/api/dms` | `POST` | `{"user_id"}` | `createDMReq` | ✅ 一致 |
| `/api/chats/{chatID}` | `PATCH` | `{"name"}` | `renameChatReq` | ✅ 一致 |
| `/api/chats/{chatID}/members` | `POST` | `{"user_id"}` | `addMemberReq` | ✅ 一致 |
| `/api/chats/{chatID}/messages` | `POST` | `{"content", "attachments"}` | `sendMsgReq` | ✅ 一致 |
| `/api/chats/{chatID}/messages/{msgID}` | `PATCH` | `{"content"}` | `editMsgReq` | ✅ 一致 |
| `/api/chats/{chatID}/read` | `POST` | `{"message_id"}` | `readReq` | ✅ 一致 |
| `/api/chats/{chatID}/pin` | `POST` | `{"content"}` | `pinContentReq` | ✅ 一致 |

## 关键技术细节
- **严格解析**: 后端统一使用 `json.NewDecoder(r.Body).DisallowUnknownFields()`，这意味着任何不在结构体定义中的额外字段都会导致 `400 Bad Request`，确保了接口的强类型契约。
- **字段映射**: 所有结构体均正确使用了 `json:"..."` 标签，与规范中的驼峰/下划线命名约定一致。

---
*报告由 opencode 自动生成*
