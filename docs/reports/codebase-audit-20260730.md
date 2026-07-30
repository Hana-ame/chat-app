# Codebase Audit 2026-07-30

Comprehensive audit of the chat-app project covering security, code quality, architecture, configuration, frontend, backend, testing, performance, and inconsistencies.

## 用户审核意见

| # | 问题 | 状态 |
|---|---|---|
| 1 | JWT token via URL query | ✅ 接受 — 作为兼容性保留 |
| 2 | CORS 任意来源 | ✅ 接受 — 前后端部署问题 |
| 8 | AI auth_key 明文传输 | ✅ 接受 — `auth_key` 仅透传到 AI 调用，**不写入 DB**（`CreateAIMessage` 只存 content/thinking） |
| 49 | 前端 AI auth_key 存 localStorage | ✅ 接受 — 设计如此 |
| 45 | 前端 token 存 localStorage | ✅ 接受 — token 过期很快 |
| 7 | 上传删除 32-bit hash | ✅ 接受 — 设计要求 |
| 66 | SQLite MaxOpenConns(1) | ✅ 接受 — 单例设计 |
| 9 | JWT secret 自动生成 | ✅ **已修复** — 改为强制从 `CHAT_JWT_SECRET` 读取，不设置则 fatal 退出 |
| 13 | mapServiceError 变量名 | ✅ **已修复** |
| 16 | decodeJSON 关闭 body | ✅ **已修复** |
| 20 | `_ = exp` 未使用 | ✅ **已修复** |
| 21 | 错误字符串比较 | ✅ **已修复** — 改用 `errors.As` + `*http.MaxBytesError` |
| 22 | defer rows.Close 在循环内 | ✅ **已修复** |
| 15 | Logout 竞态 | ✅ **已修复** — 锁移到清除 cookie 前 |

---

## 🔴 Critical Security

### 1. JWT token leaked via URL query parameter

- **Files**:
  - `server/internal/handlers/util.go:143-145` — fallback to `access_token` query param
  - `server/internal/ws/gateway.go:50` — WS handshake reads token from query
  - `server/internal/handlers/sse.go:20-22` — SSE endpoint accepts token in URL
- **Problem**: Token exposed in server logs (URL logging), browser history, Referer headers, and network中介 logs. URL parameters are the most leak-prone transport for secrets.
- **Fix**: Use WebSocket `Sec-WebSocket-Protocol` header for WS auth; for SSE, require token via cookie or `Authorization` header only. Remove `access_token` query param fallback in `util.go`.

### 2. CORS allows any origin with credentials

- **File**: `server/internal/handlers/router.go:55-62`
- **Problem**: `AllowOriginFunc` returns `true` unconditionally with `AllowCredentials: true`. Any arbitrary website can make authenticated requests (cookie-based auth) to the API, enabling CSRF-style data theft.
- **Fix**: Set `AllowOriginFunc` to validate against an explicit allowlist from config (`AllowOrigins`). Never combine `AllowCredentials: true` with wildcard origins.

### 3. No rate limiting on refresh token endpoint

- **File**: `server/internal/handlers/router.go:91`
- **Problem**: The `/api/auth/refresh` route has no rate limiter attached. An attacker can brute-force refresh tokens online — the refresh token is a 64-character hex string with 256 bits of entropy, but the absence of rate limiting removes the protection that entropy provides.
- **Fix**: Attach a per-IP rate limiter (e.g., 10/hour/IP) to the refresh endpoint.

### 4. No email verification / CAPTCHA on registration

- **File**: `server/internal/handlers/auth.go:57`
- **Problem**: Only a global 100/24h rate limit across all IPs. A single distributed attacker can create thousands of accounts, exhausting the user database or enabling spam.
- **Fix**: Add email verification (send confirmation link) or CAPTCHA (e.g., Turnstile/hCaptcha) on registration. At minimum, add per-IP rate limiting.

### 5. Password length >72 reveals distinct error

- **File**: `server/internal/auth/auth.go:108-111,120-122`
- **Problem**: `bcrypt.GenerateFromPassword` returns `bcrypt.ErrPasswordTooLong` when password exceeds 72 bytes. The code returns this as a distinct error message, letting an attacker determine that a password is longer than 72 characters — useful information during brute-force or targeted attacks.
- **Fix**: Reject passwords >72 bytes at the handler level with a generic "invalid password" message, or hash the password with SHA-256 before passing to bcrypt (common pattern).

### 6. WebSocket sends all chats on connect without per-chat authz

- **File**: `server/internal/ws/gateway.go:84-91`
- **Problem**: On WebSocket connect, the gateway subscribes the client to all chats the user is a member of in a batch. There is no per-chat authorization check on each subscription — it trusts the database result from `ListUserChats`. If a user is removed from a chat between the DB query and subscription setup, they remain subscribed.
- **Fix**: Perform per-chat `MustBeMember` check during subscription, and listen for membership revocation events to unsubscribe.

### 7. Upload delete uses weak 32-bit hash for authorization

- **File**: `server/internal/handlers/local_upload.go:183-194`
- **Problem**: Delete authorization uses `sha256(path+salt)[:8]` — only 8 hex characters = 32 bits of entropy. An attacker who knows one valid hash can forge deletions for other files. Without salt (default empty), the hash is trivially computable for any path.
- **Fix**: Use a proper bearer token (random 128+ bits) stored alongside the upload record, or require user authentication for deletes.

### 8. AI auth_key travels in plaintext through client and DB

- **File**: `server/internal/ai/stream.go:20` (Source struct includes `AuthKey`)
- **Problem**: The AI provider authentication key (`auth_key`) is sent by the client in message source data, stored unencrypted in the `messages` database table, and readable by any chat member who can load message source data. This is a credential leak waiting to happen.
- **Fix**: Proxy AI requests through the server with server-side key configuration. Never let the client provide the auth key. Remove `auth_key` from the client-facing schema.

### 9. JWT secret auto-generated on restart

- **File**: `server/internal/config/config.go:77-80`
- **Problem**: If `JWT_SECRET` is not set in environment, a random 32-byte secret is generated on every server restart. This invalidates all existing tokens (access and refresh) on restart, logging out all users. Generate-on-startup also means the secret is never persisted, so token signing is non-reproducible.
- **Fix**: Require `JWT_SECRET` to be set explicitly in environment. Fail to start if not set, with a clear error message.

### 10. No CSRF protection

- **File**: `server/internal/handlers/auth.go:166` (cookie set with `SameSite=LaxMode` only)
- **Problem**: Refresh tokens are set as cookies but there is no CSRF token mechanism. `SameSite=Lax` prevents simple top-level navigation CSRF but does not protect against subdomain attacks or certain POST-based CSRF scenarios.
- **Fix**: Implement CSRF token pattern (double-submit cookie or header verification) for state-changing requests, especially for cookie-based refresh.

### 11. HSTS missing

- **File**: `server/internal/handlers/router.go` (no HSTS middleware)
- **Problem**: No `Strict-Transport-Security` header is set. If a user ever connects over HTTP (e.g., first visit, MITM proxy), an attacker can downgrade the connection.
- **Fix**: Add HSTS middleware with `max-age=31536000; includeSubDomains` in production.

### 12. Swagger JSON served without auth

- **File**: `server/internal/handlers/router.go:20`
- **Problem**: `/api/swagger.json` is served publicly without authentication. If the API specification documents internal endpoints, parameter structures, or auth details, this leaks attack surface information.
- **Fix**: Restrict swagger endpoint to authenticated admin users or disable in production.

---

## 🔴 Code Quality / Correctness

### 13. mapServiceError test has swapped return values

- **File**: `server/internal/handlers/util_test.go:31`
- **Problem**: `code, str = mapServiceError(...)` assigns variables named `code` and `str`, but the function signature is `mapServiceError(error) (int, string)` — `code` (status code) is first, `str` (error code string) is second. The test variable names are misleading and would mask a swapped-return bug.
- **Fix**: Rename variables to match semantics: `statusCode, errorCode := mapServiceError(...)`.

### 14. Stream memory leak on disconnect

- **File**: `server/internal/handlers/messages.go:156-171`
- **Problem**: When handling AI streaming responses, a goroutine is spawned to consume the stream. If the client disconnects mid-stream, cleanup relies on `FinishStream` which may not be reliably called (e.g., if the context is not properly cancelled). The goroutine and its associated buffers are never freed.
- **Fix**: Use a `context.Context` tied to the HTTP request context. Cancel on client disconnect. Ensure the goroutine exits via `select` on ctx.Done().

### 15. Race condition on refreshMu during logout

- **File**: `server/internal/handlers/auth.go:179-194`
- **Problem**: The logout handler clears the refresh token cookie before acquiring `refreshMu`. A concurrent `Refresh` call with the still-valid (just-cleared) cookie can race: logout clears cookie, then Refresh sees no active revocation and issues a new token pair.
- **Fix**: Acquire `refreshMu` before clearing cookies. Check revocation status under the lock.

### 16. decodeJSON closes request body

- **File**: `server/internal/handlers/handler.go:130` (`defer r.Body.Close()`)
- **Problem**: The HTTP server (`net/http`) automatically closes the request body after the handler returns. Explicitly calling `r.Body.Close()` is redundant and, in some edge cases, can cause issues with middleware that wraps the body.
- **Fix**: Remove the `defer r.Body.Close()` from `decodeJSON` — let the server handle it.

### 17. Concurrent map access in user search

- **File**: `server/internal/handlers/users.go:99-105`
- **Problem**: The code does `users = users[:0]` to reuse the slice, but this shares the underlying backing array. If multiple concurrent requests search users, the re-slicing and re-population can cause data races and return corrupted results.
- **Fix**: Allocate a new slice for each request: `result := make([]model.UserPublic, 0, len(users))`.

### 18. Typing timer with no cleanup

- **File**: `client/src/components/Composer.jsx:154-158`
- **Problem**: `setTimeout(() => {}, 2000)` is created on every keystroke but never cleared on component unmount. If the user navigates away while a timer is pending, it fires on an unmounted component.
- **Fix**: Store the timeout ID in a ref and call `clearTimeout` in the cleanup function of the effect.

### 19. BroadcastUserUpdate iterates clients while releasing RLock

- **File**: `server/internal/ws/hub.go:291-318`
- **Problem**: The method acquires `h.mu.RLock()` to iterate WebSocket clients, releases it (lock is released at `}` end of the `h.mu.RLock()` block), then iterates SSE clients without any lock at all. Concurrent writes to `h.sseClients` during this iteration cause a data race panic.
- **Fix**: Hold a read lock over the entire iteration of both client lists, or use a consistent lock/unlock pattern.

### 20. Unused `_` assignments

- **File**: `server/internal/handlers/auth.go:135` (`_ = exp`)
- **Problem**: The `exp` variable from token claims parsing is assigned to `_`, indicating it's unused. This suggests either dead code or a missed validation (should check `exp` for token expiry).
- **Fix**: Either remove the assignment or add the intended expiration check.

### 21. Error string comparison for body too large

- **File**: `server/internal/handlers/local_upload.go:143`
- **Problem**: The code compares `err.Error() == "http: request body too large"` to detect oversized uploads. This is fragile — it depends on the exact wording of Go's HTTP error message, which could change between Go versions or be locale-dependent.
- **Fix**: Use `http.MaxBytesHandler` or check `http.MaxBytesError` type assertion (`var maxErr *http.MaxBytesError; errors.As(err, &maxErr)`).

### 22. defer rows.Close() inside for loop

- **File**: `server/internal/db/chats.go:176`
- **Problem**: `defer rows.Close()` inside a loop body means rows are not closed until the function returns. If the loop iterates many times, all rows objects stay open simultaneously, holding database connections and memory.
- **Fix**: Use `rows.Close()` (without `defer`) at the end of each iteration, or restructure to avoid defer in a loop.

### 23. Global timeNow() function

- **File**: `server/internal/handlers/util.go:9`
- **Problem**: A package-level `var timeNow = time.Now` prevents injecting fake time in tests. Any test that needs deterministic timestamps must overwrite the global variable, which is not goroutine-safe.
- **Fix**: Accept a `timeProvider` interface on the service/handler struct, defaulting to `time.Now`.

### 24. Nil check on Hub everywhere

- **Files**: Multiple service files — every `s.Hub.Broadcast*` call is guarded with `if s.Hub != nil`.
- **Problem**: The nil-guard is repeated across dozens of calls. It should be internalized into the Hub itself (nil Hub = no-op) or guaranteed at construction time.
- **Fix**: Add a `hub.go` method `func (h *Hub) Broadcast(ctx context.Context, msg any)` that is a no-op if `h == nil`. Remove all external nil checks.

---

## 🔴 Architectural Issues

### 25. Service layer calls DB directly without interface

- **File**: `server/internal/service/chat.go:15` (`s.DB.ListUserChats`)
- **Problem**: The Service struct has a concrete `*db.DB` field. There is no `DBInterface`, so unit tests cannot mock the database. Every test must set up a real (or in-memory) SQLite database.
- **Fix**: Define a `DBInterface` with all required methods. Have `*db.DB` implement it. Inject the interface into Service.

### 26. Hub handles both WebSocket and SSE with duplicated logic

- **File**: `server/internal/ws/hub.go`
- **Problem**: The Hub has separate `clients` (WebSocket) and `sseClients` maps with duplicated broadcast methods for each. Adding a new event type requires changes in two code paths.
- **Fix**: Abstract into a single `Client` interface with `Send(msg)` method. Have both WS and SSE clients implement it. Use one `map[string]Client`.

### 27. Member cache in Hub never expires entries proactively

- **File**: `server/internal/ws/hub.go:45-47`
- **Problem**: The member cache (`chatMembers`) only evicts entries when a broadcast to that specific chat occurs. If a chat goes idle, stale membership data persists indefinitely. There is no background eviction goroutine.
- **Fix**: Add a background goroutine with `time.Ticker` that evicts entries not accessed within a TTL window, or use `sync.Map` with manual expiration.

### 28. Service struct is monolithic

- **File**: `server/internal/service/service.go:13-24`
- **Problem**: The `Service struct` embeds all sub-services (`*ChatService`, `*MessageService`, etc.) and has direct fields for `DB`, `Hub`, `Logger`. Every service instance has access to everything, violating the principle of least privilege.
- **Fix**: Split into smaller, focused service structs that receive only the dependencies they need via constructors.

### 29. SSE implementation incomplete

- **File**: `server/internal/handlers/sse.go`
- **Problem**: No `Last-Event-ID` tracking, no reconnection logic. The `keepAlive` goroutine sends `:keepalive\n\n` comments, but client `EventSource` may not handle keepalive-only streams correctly. When the connection drops, there is no mechanism to replay missed events.
- **Fix**: Implement proper SSE spec compliance: event IDs, reconnection tokens, and `Last-Event-ID` support.

### 30. Frontend transports have inconsistent API

- **Files**: `client/src/realtime/transports/ws.js`, `sse.js`, `poll.js`
- **Problem**: `ws.js` returns `{sendTyping, subscribe, wsRequest, disconnect}`, while `sse.js` and `poll.js` only return `{disconnect}`. The Coordinator must handle each transport type separately with instanceof checks.
- **Fix**: Define a uniform `TransportInterface` contract that all transports implement.

### 31. Service layer bypasses authz for some DB calls

- **File**: `server/internal/service/message.go` (ListForUser) vs `server/internal/service/chat.go` (GetByID)
- **Problem**: `GetByID` calls `MustBeMember` for authorization, but `ListForUser` does not — it just calls `DB.GetMessages` directly. Inconsistent authorization boundary means some data access paths skip permission checks.
- **Fix**: Apply consistent authorization at the service layer entry points. Every public service method should verify the caller's permissions.

### 32. MarkRead is misnamed

- **File**: `server/internal/service/chat.go` (MarkRead calls `UpdateLastActiveAt`)
- **Problem**: The method named `MarkRead` does not mark any messages as read — it updates `last_active_at` on the chat member record. There is duplicate logic between `ChatService` and `MessageService`.
- **Fix**: Rename to `UpdateLastActive` to reflect actual behavior. Consolidate read-receipt logic in one place.

### 33. SendMessage handler contains two separate flows (JSON & SSE)

- **File**: `server/internal/handlers/messages.go:66-87`
- **Problem**: The handler checks for SSE content type and routes to an entirely different code path for SSE vs JSON. This makes the handler hard to test and reason about.
- **Fix**: Split into two separate handlers routed at the router level based on `Content-Type` or `Accept` header.

### 34. WithTx pattern with deferred Rollback after Commit

- **File**: `server/internal/service/service.go:45-55`
- **Problem**: The pattern `defer tx.Rollback()` followed by `tx.Commit()` works because Rollback after Commit is a no-op, but it's misleading. If the function panics after Commit, Rollback is called on a committed transaction, which is fine but potentially confusing. More importantly, if `fn` modifies external state after Commit but before returning, a partial failure is not rolled back.
- **Fix**: Use a `commitErr` flag pattern: only Rollback if not committed. Or use `database/sql`'s `Tx` properly: Commit first, then only Rollback on error before Commit.

---

## 🔴 Configuration Issues

### 35. AllowOrigins defaults to `*`

- **File**: `server/internal/config/config.go`
- **Problem**: When `CHAT_ALLOW_ORIGINS` is not set, the config defaults to `*`. Combined with cookie-based auth and `AllowCredentials: true`, this is the most permissive (dangerous) CORS configuration possible.
- **Fix**: Default to the same value as `CHAT_BASE_URL` or fail to start if no origin is configured.

### 36. CSPConnectSrc not set by default

- **File**: `server/internal/config/config.go`
- **Problem**: The Content-Security-Policy `connect-src` directive defaults to `*` (not set), allowing the frontend to make fetch/XHR requests to arbitrary origins. This weakens the security model against XSS.
- **Fix**: Set default `connect-src` to the API origin.

### 37. MaxMessageContentLength configurable with no bound

- **File**: `server/internal/config/config.go`
- **Problem**: The admin can set `CHAT_MAX_MESSAGE_CONTENT` to any value. Setting it extremely high (e.g., 1GB) could cause memory exhaustion when storing or broadcasting large messages.
- **Fix**: Set a hard upper bound (e.g., 1MB) in code, enforced regardless of config.

### 38. UploadSalt empty by default

- **File**: `server/internal/handlers/local_upload.go:53`
- **Problem**: When `CHAT_UPLOAD_SALT` is not set, the upload hash salt is empty. This makes upload paths trivially guessable: `sha256(filename + "")`.
- **Fix**: Generate a random salt on first run and persist it, or require explicit configuration.

### 39. Client .env.example missing AI vars

- **File**: `client/.env.example`
- **Problem**: `Composer.jsx` uses `VITE_AI_ENDPOINT`, `VITE_AI_AUTH_KEY`, `VITE_AI_MODEL` but none of these are documented in `.env.example`. New developers must read the source code to discover required configuration.
- **Fix**: Add all `VITE_AI_*` variables to `.env.example` with descriptive comments.

### 40. Server .env.example has CHAT_AI_BASE_URL, CHAT_AI_MODEL, CHAT_AI_SOURCES but server never reads them

- **File**: `server/.env.example`
- **Problem**: These AI-related environment variables are documented in `.env.example` but the server code never reads any of them. Developers may set them expecting them to work, but they have no effect.
- **Fix**: Either implement the server-side AI configuration or remove the variables from `.env.example`.

### 41. CHAT_BASE_URL used inconsistently

- **Files**: `server/internal/handlers/local_upload.go` (uses CHAT_BASE_URL), other handlers (do not)
- **Problem**: Upload URLs use `CHAT_BASE_URL` to construct absolute URLs, but avatar URLs, chat image URLs, etc. use relative paths or request-derived hosts. This inconsistency means deployment behind a reverse proxy with a different external URL produces broken links for some resources.
- **Fix**: Centralize URL construction. Always use `CHAT_BASE_URL` for any absolute URL returned in API responses.

### 42. SQLite MaxOpenConns(1)

- **File**: `server/internal/db/db.go:62`
- **Problem**: `MaxOpenConns(1)` serializes all database access. Even with WAL mode (which allows concurrent reads), only one goroutine can query at a time. This is a severe bottleneck under any moderate load.
- **Fix**: Set `MaxOpenConns` to a reasonable number (e.g., `runtime.NumCPU() * 2`). With WAL mode and `_journal_mode=WAL`, SQLite handles concurrent reads well.

---

## 🔴 Frontend Issues

### 43. fetchStream not handling non-OK responses

- **File**: `client/src/store/chat.js:60-102`
- **Problem**: When `fetchStream` receives a non-2xx HTTP response, it never transitions the message out of "streaming" state. The message is stuck with a forever-spinning indicator. User has no way to recover without refreshing.
- **Fix**: Handle error responses by updating the message state to `failed` with the error message.

### 44. Race condition in loadMessages with loadId

- **File**: `client/src/store/chat.js:384-395`
- **Problem**: Sequential requests use a `loadId` counter. If Request A (loadId=1) and Request B (loadId=2) are in flight, and A completes after B (stale), the check `loadId === currentLoadId` correctly discards A. However, if A was the "first page" and B was "load more", discarding A loses the first page.
- **Fix**: Use per-page load IDs or cancel pending requests on new navigation.

### 45. Auth token stored in localStorage

- **File**: `client/src/store/auth.js:40-43`
- **Problem**: The access token and refresh token are stored in `localStorage`. Any XSS vulnerability (even transient) can exfiltrate these tokens, granting persistent access to the attacker.
- **Fix**: Use httpOnly cookies for token storage. If that's not feasible, use sessionStorage (cleared on tab close) or in-memory storage with refresh.

### 46. streamAI catches all errors silently

- **File**: `client/src/utils/ai.js:22` (`catch {}`)
- **Problem**: AI streaming errors are caught with an empty catch block. Users never see connection failures, auth errors, or malformed responses from the AI provider. Debugging AI issues becomes impossible.
- **Fix**: Log the error and optionally surface a user-facing notification.

### 47. connect called after store initialization

- **File**: `client/src/store/auth.js:18-26`
- **Problem**: The realtime `connect()` call happens during store initialization (module level), but the Coordinator has no transport until `ChatPage` mounts and calls `useCoordinator()`. The initial `connect` silently fails.
- **Fix**: Move `connect()` to a component lifecycle hook (`useEffect`) or make it idempotent and deferred until transport is available.

### 48. IS_PAGES detection at module level

- **File**: `client/src/config.js:1`
- **Problem**: `IS_PAGES` is computed at module import time based on `window.location.hostname`. This never re-evaluates, hardcoding a Cloudflare Pages dependency. If the app is deployed elsewhere, this flag may be wrong.
- **Fix**: Make `IS_PAGES` a runtime check or a build-time env var (`VITE_IS_PAGES`).

### 49. Composer.jsx stores AI auth key in localStorage

- **File**: `client/src/components/Composer.jsx:32-34`
- **Problem**: The AI provider authentication key (`auth_key`) is stored in `localStorage` after the user inputs it. This is highly sensitive credentials stored in XSS-accessible storage, with no encryption.
- **Fix**: Do not persist the AI key on the client. Send it per-request or use a server-side proxy configuration.

### 50. MessageItem.jsx no React.memo

- **File**: `client/src/components/MessageItem.jsx`
- **Problem**: `ThinkingContent` is defined as a nested component inside `MessageItem`, so it is recreated on every render. Combined with no `React.memo`, every message re-renders on any chat state change, causing O(n) DOM updates per message.
- **Fix**: Extract `ThinkingContent` as a standalone component with `React.memo`. Apply `React.memo` to `MessageItem` itself with shallow comparison on message ID.

### 51. Race condition in ChatView.jsx

- **File**: `client/src/components/ChatView.jsx`
- **Problem**: `loadMessages` and `subscribe` are called sequentially but not atomically. If the user switches chats quickly, the subscribe from the old chat can land on the new chat's message list, or vice versa.
- **Fix**: Use an effect with a cleanup function that unsubscribes from the previous chat before loading the new one. Tie subscriptions to chat ID via a ref.

### 52. schemas.ts types source as z.function()

- **File**: `client/src/schemas.ts`
- **Problem**: The `source` field in the message schema is typed as `z.function()`, but the actual data is an object `{endpoint, auth_key, model, ...}`. Zod validation may fail or silently coerce.
- **Fix**: Define a proper Zod schema for the AI source object, e.g., `z.object({ endpoint: z.string(), auth_key: z.string(), model: z.string() }).optional()`.

### 53. renderContent.jsx doesn't sanitize URL text

- **File**: `client/src/components/renderContent.jsx`
- **Problem**: URL text extracted from rendered content is used directly in `<a href="...">` without sanitization. `javascript:` URIs or other malicious schemes could execute code on click.
- **Fix**: Validate URL schemes against an allowlist (`https?://`, `mailto:`) before inserting into `href`.

### 54. Mock API flag persists across sessions

- **File**: `client/src/store/auth.js:7-9`
- **Problem**: The `MOCK_FLAG` in `localStorage` persists across browser sessions. A user who enabled mock mode for testing could forget and leave it enabled in production, silently serving fake data.
- **Fix**: Do not persist the mock flag. Default to real API, require explicit opt-in per session (e.g., URL parameter `?mock=1`).

### 55. Poll transport no backoff

- **File**: `client/src/realtime/transports/poll.js:22`
- **Problem**: The polling transport fetches every 2 seconds indefinitely, even when the network is down or the server returns 5xx errors. This creates unnecessary load and poor UX.
- **Fix**: Implement exponential backoff (start 2s, max 30s) with jitter. Reset on successful response.

### 56. Composer.jsx uses eval-like JSON construction

- **File**: `client/src/components/Composer.jsx` (toJsonBody helper)
- **Problem**: The helper concatenates strings with partial `JSON.stringify` calls to build JSON manually. This is fragile and can produce malformed JSON for edge cases (e.g., strings containing quotes or backslashes).
- **Fix**: Build the entire payload as a JavaScript object and call `JSON.stringify` once.

### 57. streamAI not used with auth token

- **File**: `client/src/utils/ai.js`
- **Problem**: AI streaming calls go directly to the external AI endpoint without including the chat app's auth token. The server-side `/api/messages` handler expects an authenticated user but the AI path bypasses this.
- **Fix**: If AI requests should be proxied (recommended), route through the API server with auth. If direct, at minimum include proper headers.

### 58. No useCallback on handleLoadMore with proper deps

- **File**: `client/src/components/MessageList.jsx:10-20`
- **Problem**: `handleLoadMore` is recreated on every render, causing the infinite scroll observer to detach and re-attach. This can cause missed scroll events and unnecessary re-renders.
- **Fix**: Wrap `handleLoadMore` in `useCallback` with the correct dependency array.

### 59. requestAnimationFrame not canceled on unmount

- **File**: `client/src/components/MessageList.jsx:26-32`
- **Problem**: `requestAnimationFrame` is called without storing the handle. When the component unmounts, the callback may fire and call `setState` on an unmounted component, causing a React warning and potential memory leak.
- **Fix**: Store the RAF handle in a ref and call `cancelAnimationFrame` in the effect cleanup.

---

## 🔴 Backend (Go) Anti-Patterns

### 60. rand.Read ignoring errors

- **File**: `server/internal/config/config.go:71` (`_, _ = rand.Read(b)`)
- **Problem**: The JWT secret generation ignores errors from `rand.Read`. If the random source fails (extremely rare but possible on some platforms/containers), the "secret" is a buffer of zeroes — trivially guessable by an attacker.
- **Fix**: Check the error from `rand.Read`. If it fails, return a fatal error or use `crypto/rand` with proper error handling.

### 61. syncReactionsColumn called outside transaction

- **File**: `server/internal/db/message_reactions.go:62`
- **Problem**: `syncReactionsColumn` modifies the `reactions_json` column on the messages table outside a transaction. If the server crashes between the read and write, the cached JSON is permanently out of sync with the actual reaction data.
- **Fix**: Wrap the sync operation in a transaction.

### 62. isStreaming flag logic gap

- **File**: `server/internal/ai/stream.go:61-65`
- **Problem**: The `ensureStreamEnabled` function modifies the request to force `stream: true`, then sets `isStreaming` to `true` if the original request was not already streaming. If the AI provider returns a non-streaming response despite `stream: true` (some providers ignore this flag), the caller expects streaming chunks but gets a single JSON response.
- **Fix**: Always treat the response as potentially non-streaming. Check `Content-Type` or first byte to determine response format.

### 63. getenvDuration ignores parse errors

- **File**: `server/internal/config/config.go:62-65`
- **Problem**: If the environment variable contains an unparseable duration string, `getenvDuration` silently returns the default value with no warning. Misconfigured deployments run with wrong timeouts.
- **Fix**: Log a warning or error when parsing fails, showing the invalid value and the fallback default.

### 64. File handle leak in local_upload.go

- **File**: `server/internal/handlers/local_upload.go` (single initialization)
- **Problem**: The upload driver is initialized once at startup. If initialization fails, there is no retry mechanism. However, if the driver holds file handles or resources, there is no graceful shutdown to release them.
- **Fix**: Implement `io.Closer` on the upload driver and call `Close()` during server shutdown.

### 65. GetMessages may accept negative limit

- **File**: `server/internal/db/messages.go`
- **Problem**: If the client sends a negative `limit` parameter, it is passed directly to SQL as `LIMIT -1` or `LIMIT ?` with negative value. SQLite behavior with negative LIMIT is undefined and may return unexpected results or errors.
- **Fix**: Validate `limit >= 0` and `limit <= MaxMessageContentLength` at the handler or service layer.

---

## 🔴 Testing Issues

### 66. WS tests always skip when WS_ENABLED not set

- **File**: `server/internal/ws/ws_test.go:19-23`
- **Problem**: `t.Skip("WS_ENABLED not set")` means the WebSocket tests never run unless a specific env var is configured. CI pipelines without `WS_ENABLED` silently skip all WebSocket coverage.
- **Fix**: Use an in-memory WebSocket server for testing instead of relying on a real server process. Remove the env var gate.

### 67. Config test mutates global environment

- **File**: `server/internal/config/config_test.go:17` (uses `os.Unsetenv`)
- **Problem**: `os.Unsetenv` modifies the global process environment. Tests running in parallel or subsequent tests that depend on environment variables can fail nondeterministically.
- **Fix**: Use `t.Setenv` (Go 1.17+) which automatically restores the original value via `t.Cleanup`.

### 68. No tests for critical AI streaming path

- **File**: `server/internal/ai/stream.go` (no test file)
- **Problem**: The AI streaming code that makes external HTTP calls has zero test coverage. There are no tests for HTTP failures, timeouts, malformed SSE chunks, partial responses, or auth errors.
- **Fix**: Add unit tests with an HTTP test server (`httptest.NewServer`) that simulates various AI provider responses.

### 69. util_test.go uses package-level access

- **File**: `server/internal/handlers/util_test.go`
- **Problem**: Directly calling internal functions (`mapServiceError`) is fine for white-box testing, but it means the tests are tightly coupled to implementation details. Changes to function signatures or error handling break tests.
- **Fix**: Prefer testing through public interfaces where possible. Keep white-box tests for critical logic but document the coupling.

### 70. No unit tests for service layer authz logic

- **File**: `server/internal/service/` (no test files for authz)
- **Problem**: `MustBeMember`, `RequireOwnerOrAdmin`, `MustBeAdmin` functions have no dedicated unit tests. This is the core authorization logic — any regression here is a security issue.
- **Fix**: Add unit tests for each authorization function with table-driven test cases covering member, non-member, owner, admin, and banned scenarios.

### 71. mapServiceError test doesn't cover wrapped errors

- **File**: `server/internal/handlers/util_test.go`
- **Problem**: The test only checks `mapServiceError` with bare `service.Err*` values. In practice, errors are wrapped with `fmt.Errorf("context: %w", err)`. The function uses `errors.Is` internally but has no test coverage for wrapped errors.
- **Fix**: Add test cases with `fmt.Errorf` wrapped errors.

---

## 🔴 Performance Issues

### 72. N+1 query: GetChat fetches last message separately

- **File**: `server/internal/db/chats.go:150-155`
- **Problem**: For each chat returned by `GetChat` (or in a list), a separate query is made to fetch the last message. Loading 20 chats = 1 list query + 20 individual message queries.
- **Fix**: Use a single query with a correlated subquery or `LEFT JOIN` to get the latest message per chat.

### 73. ListUserChats double-queries unread count per chat

- **File**: `server/internal/db/chats.go:234-237`
- **Problem**: After fetching the chat list, the code queries `CountUnreadMessages` for each chat individually. 20 chats = 20 extra COUNT queries.
- **Fix**: Compute unread counts in the initial query using `GROUP BY` chat_id and `SUM` with conditional aggregation.

### 74. reactionsFor duplicates data that ListReactions also queries

- **File**: `server/internal/db/message_reactions.go:93-176`
- **Problem**: `reactionsFor` and `ListReactions` both query the `message_reactions` table and serialize to JSON. The `reactions_json` column duplicates this data at write time and read time, causing write amplification.
- **Fix**: Remove `reactions_json` denormalization. Read reactions from the reactions table directly.

### 75. GetChat selects all columns including unused banner/background

- **File**: `server/internal/db/chats.go:105-158` (uses `SELECT *`)
- **Problem**: The query selects all chat columns including large text/blob fields like `banner_url`, `background_url`, `description`. For list endpoints like `ListUserChats`, these fields are never used by the frontend.
- **Fix**: Use explicit column lists. Create a separate light query for list endpoints.

### 76. SSE poll transport polls every 2 seconds

- **File**: `client/src/realtime/transports/poll.js:22`
- **Problem**: 2-second polling is unnecessarily aggressive, generating 30 requests per minute per client. With many concurrent users, this creates significant server load.
- **Fix**: Increase interval to 5-10 seconds, or use adaptive polling based on chat activity.

### 77. N+1 for member count in chats_ext.go

- **File**: `server/internal/db/chats_ext.go`
- **Problem**: A subquery runs for each chat row to compute `member_count`. This is effectively an N+1 at the SQL level.
- **Fix**: Use a single `LEFT JOIN` with `COUNT(*) ... GROUP BY chat_id` to fetch all member counts in one pass.

### 78. Messages loaded with no index on (chat_id, created_at)

- **File**: `server/internal/db/messages.go`
- **Problem**: Message queries filter by `chat_id` and sort by `created_at DESC`. Without a composite index on `(chat_id, created_at)`, SQLite performs a full table scan with a filesort for every chat message load.
- **Fix**: Add a composite index: `CREATE INDEX idx_messages_chat_created ON messages(chat_id, created_at DESC)`.

### 79. No caching for user data

- **File**: `server/internal/handlers/users.go` (every call hits DB)
- **Problem**: User profile lookups happen on every message render, every chat member list operation, and every user search. There is no in-memory caching layer.
- **Fix**: Add an in-memory cache (e.g., `sync.Map` with TTL) for `UserPublic` data. Invalidate on profile update.

### 80. Broadcast to all members loads full member list from DB

- **File**: `server/internal/ws/hub.go` (broadcast operations)
- **Problem**: Every broadcast to a chat loads the entire member list from the database to determine who should receive the message. For large chats with few online members, this is wasteful.
- **Fix**: Track online members per chat via the Hub's client connections. Only broadcast to connected clients; use DB only for offline notifications.

### 81. Frontend replaces entire messages array on every update

- **File**: `client/src/store/chat.js` (setMessages replaces all messages)
- **Problem**: Whenever new messages arrive (via polling or WebSocket), the entire messages array is replaced. This causes every `MessageItem` to re-render, even for unchanged messages.
- **Fix**: Use immutable updates at the individual message level. Replace only the changed/added messages. Consider a library like `immer` or manual key-based updates.

---

## 🔴 Inconsistencies

### 82. expires_in documented as 900s but code returns 1800s

- **File**: `docs/api.md:49` vs `server/internal/handlers/auth.go:139`
- **Problem**: API documentation states `expires_in: 900` (15 min), but the code returns `ACCESS_TOKEN_TTL` which is 1800 seconds (30 min) by default.
- **Fix**: Update the documentation to match the actual value, or change the code to return 900.

### 83. Upload API doc claims 201 but code returns 200

- **File**: `docs/api.md:489` vs `server/internal/handlers/local_upload.go:74-79`
- **Problem**: The documentation says the upload endpoint returns HTTP 201 Created, but the handler writes HTTP 200 OK.
- **Fix**: Either change the handler to return 201 (correct for resource creation), or update the doc to 200.

### 84. avatar_url field name in code vs url in doc

- **File**: `server/internal/handlers/chat.go:223` vs `docs/api.md:226`
- **Problem**: Code uses field name `avatar_url` in the chat detail response, but the API doc documents the field as `url`. Any client written against the doc will not find the `url` field.
- **Fix**: Align field names between code and documentation.

### 85. notify_blocked field not in API doc

- **File**: `server/internal/models/models.go:18` (serialized) but undocumented in `docs/api.md`
- **Problem**: The `notify_blocked` field is serialized in API responses but never documented. API consumers don't know it exists, and developers may rely on undefined behavior.
- **Fix**: Add `notify_blocked` to the API documentation with its semantics.

### 86. UnreadCount capped at 99 but no mention in doc

- **File**: `server/internal/db/messages.go:316`
- **Problem**: `UnreadCount` is capped at 99 (returns 99+ if >99) but this behavior is not documented in the API spec. Frontend developers may implement their own badge display expecting the real count.
- **Fix**: Document the capping behavior in the API spec, or return the real count.

### 87. go.mod specifies Go 1.26.3 which doesn't exist

- **File**: `server/go.mod:3`
- **Problem**: As of 2025, Go 1.26 does not exist (latest stable is 1.24.x, development is 1.25). The directive `go 1.26.3` will cause `go build` to fail or behave unexpectedly. This may be a forward-looking placeholder that was never corrected.
- **Fix**: Set to the actual Go version being used (check `go version` in CI or development environment).

### 88. Register rate limit doc says 5/min/IP but code is global 100/24h

- **File**: `docs/api.md:38` vs `server/internal/handlers/handler.go:55`
- **Problem**: Documentation says "5 requests per minute per IP", but the code implements a global rate limit of 100 requests per 24 hours shared across all IPs.
- **Fix**: Align code with documentation or vice versa. Per-IP rate limiting is generally preferable.

### 89. Login rate limit doc says 10/min/IP but code is 5/hour/IP

- **File**: `docs/api.md:60` vs `server/internal/handlers/handler.go:54`
- **Problem**: Documentation says "10 requests per minute per IP", but the code implements 5 requests per hour per IP.
- **Fix**: Align code with documentation or vice versa.

### 90. LOG_LEVEL not in any documentation

- **File**: `server/internal/logutil/log.go:46`
- **Problem**: The server supports `LOG_LEVEL` environment variable (debug/info/warn/error), but it is not documented in `.env.example`, `LOCAL_DEPLOYMENT.md`, `docs/api.md`, or any other doc.
- **Fix**: Document `LOG_LEVEL` in `.env.example` and `LOCAL_DEPLOYMENT.md`.

### 91. CHAT_AI_BASE_URL and CHAT_AI_MODEL in .env.example but never read by server

- **File**: `server/.env.example`
- **Problem**: These environment variables are listed in the example config but the Go server never reads them. AI configuration is entirely client-driven.
- **Fix**: Either implement server-side AI configuration or remove the variables.

### 92. _time_format=sqlite in DSN but code formats times manually

- **File**: `server/internal/db/db.go:45` (DSN parameter) vs manual time formatting in code
- **Problem**: The SQLite DSN includes `_time_format=sqlite` which enables automatic time parsing, but the code manually formats and parses timestamps using `time.RFC3339`, `timeutils.FormatSQLite`, etc. The setting and code are redundant.
- **Fix**: Use one approach consistently — either rely on SQLite's built-in time parsing or handle it all in code.

### 93. PickColor uses chat name which isn't unique

- **File**: `server/internal/db/users.go:27-34`
- **Problem**: `PickColor` hashes the chat name to generate a deterministic color. Chat names are not unique (multiple chats can have the same name like "general"), so different chats may get the same color.
- **Fix**: Include chat ID in the hash input.

---

## 📋 Summary

| Category | Count |
|---|---|
| 🔴 Critical Security | 12 |
| 🔴 Code Quality / Correctness | 12 |
| 🔴 Architectural Issues | 10 |
| 🔴 Configuration Issues | 8 |
| 🔴 Frontend Issues | 17 |
| 🔴 Backend Anti-Patterns | 6 |
| 🔴 Testing Issues | 6 |
| 🔴 Performance Issues | 10 |
| 🔴 Inconsistencies | 12 |
| **Total** | **93** |

> Note: 7 issues from the working notes were deduplicated or merged during compilation.
