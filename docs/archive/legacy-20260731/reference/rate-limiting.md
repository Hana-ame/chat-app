# Rate Limiting

All rate limits enforced on the server side.

| Scope | Limit | Window | Method | Implementation |
|---|---|---|---|---|
| Global API | 120 requests | 1 minute | `httprate.LimitByIP` (chi middleware) |
| Login attempts | 10 requests | 1 minute | `httprate.LimitByIP` | 
| Registration | 5 requests | 1 minute | `httprate.LimitByIP` |
| Login failures | 5 failures | 1 hour | Custom `loginRateLimiter` (per-IP attempt tracking) |
| User search | 30 requests | 1 minute | `rateLimitByUser` (keyed on user ID, fallback IP) |
| Send message | 30 requests | 1 minute | `rateLimitByUser` |

Response on limit reached: `HTTP 429` with body `{"error":"too_many_requests","message":"..."}`.
