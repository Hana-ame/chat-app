# Error Codes Reference

All API errors return JSON with `error` and `message` fields:

```json
{"error": "invalid_credentials", "message": "invalid email or password"}
```

## 400 Bad Request

| Error Code | Meaning |
|---|---|
| `bad_request` | Generic bad request (missing fields, invalid JSON, etc.) |
| `invalid_username` | Username format validation failed |
| `weak_password` | Password too weak / too short |
| `content_too_long` | Message content exceeds 4000 chars |
| `too_large` | Upload file exceeds max size |

## 401 Unauthorized

| Error Code | Meaning |
|---|---|
| `unauthorized` | Missing token |
| `token_expired` | Access token expired |
| `token_invalid` | Access token malformed or invalid signature |
| `user_not_found` | Authenticated user no longer exists |
| `invalid_credentials` | Email/password mismatch |

## 403 Forbidden

| Error Code | Meaning |
|---|---|
| `forbidden` | Not a member, not the owner, or insufficient permissions |

## 404 Not Found

| Error Code | Meaning |
|---|---|
| `not_found` | Chat, message, or user not found |
| `user_not_found` | Target user does not exist |

## 409 Conflict

| Error Code | Meaning |
|---|---|
| `already_taken` | Email or username already registered |
| `username_taken` | Username already taken (on profile update) |
| `already_member` | User is already a member of the chat |

## 429 Too Many Requests

| Error Code | Meaning |
|---|---|
| `too_many_requests` | Rate limit exceeded (global or per-endpoint) |
| `rate_limited` | Login attempt rate limit exceeded |

## 413 Payload Too Large

| Error Code | Meaning |
|---|---|
| `too_large` | Upload exceeds `CHAT_MAX_UPLOAD_BYTES` |

## 415 Unsupported Media Type

| Error Code | Meaning |
|---|---|
| `bad_mime` | Upload file type not in allowed MIME list |

## 500 Internal Server Error

| Error Code | Meaning |
|---|---|
| `internal` | Unexpected server error (DB failure, etc.) |
