# 错误码

错误响应格式：`{"error": "<code>", "message": "<说明>"}`（HTTP 状态码 + 业务错误码）。

## 认证（401/403）

| HTTP | code | 触发 |
|---|---|---|
| 401 | `unauthorized` | 缺少/无效 token，或未登录访问需认证接口 |
| 401 | `token_expired` | access token 过期 |
| 401 | `token_invalid` | access token 无效 |
| 401 | `invalid_credentials` | 登录邮箱或密码错误 |
| 401 | `refresh_invalid` | refresh token 无效 |
| 401 | `refresh_expired` | refresh token 过期 |
| 403 | `forbidden` | 无权限（非成员、非 owner、编辑他人消息等） |

## 参数与状态（400/404/413/415）

| HTTP | code | 触发 |
|---|---|---|
| 400 | `bad_request` | 请求体/参数非法（未知字段被拒绝、缺字段、emoji 编码错误等） |
| 400 | `invalid_username` | 用户名不符合规则 |
| 400 | `weak_password` | 密码过弱 |
| 404 | `not_found` | 资源不存在（聊天/消息/成员/文件） |
| 413 | `too_large` | 上传超过 `CHAT_MAX_UPLOAD` |
| 415 | `unsupported_media_type` | 上传的 Content-Type 不支持 |

## 服务端（5xx）

| HTTP | code | 触发 |
|---|---|---|
| 500 | `internal` | 服务器内部错误（DB、存储等） |
| 502 | `upstream_error` | AI 上游（stream 源）请求失败 |

## 说明

- 权限错误细分为 `forbidden`（已认证但无权）与 `unauthorized`（未认证），前端按此决定跳登录还是提示
- `bad_request` 带具体 `message`（如 "source with endpoint, auth_key, and body is required"）；**不要**按 message 文本匹配，按 `error` 码
- AI 流式端点 SSRF 校验失败同样返回 `bad_request`（message 含 "endpoint" 字样的校验错误）
