# Auth Flow Test Report

## Files Changed
| File | Change |
|------|--------|
| `server/internal/testutil/client.go` | Added `DoWithCookie()`, `Refresh()`, `cookieValue()` helpers. `Register`/`Login` now extract `refresh_token` from Set-Cookie header |
| `server/internal/testutil/auth_flow_test.go` | **New file** — 9 auth tests |
| `server/internal/testutil/integration_test.go` | Fixed `TestRefreshTokenFlow` to use cookie-based refresh |
| `server/internal/testutil/handler_test.go` | Fixed 2 subtests in `TestAuthEndpoints` to use cookie-based refresh |

---

## Test Details

### 1. TestRegisterLoginRefresh
Happy path: register → login → refresh

| # | Method | Path | Body | Expected |
|---|--------|------|------|----------|
| 1 | POST | /api/auth/register | `{email,username,password}` | 200 + `access_token`, `refresh_token` (Set-Cookie), `user.id` |
| 2 | POST | /api/auth/login | `{email,password}` | 200 + same `user.id`, different `refresh_token` |
| 3 | POST | /api/auth/refresh | Cookie: `refresh_token=...` | 200 + same `user.id`, new `refresh_token` |

### 2. TestAccessDeniedWithoutToken
6 auth-required endpoints without Bearer token

| # | Method | Path | Expected |
|---|--------|------|----------|
| 1 | GET | /api/users/me | 401 |
| 2 | PATCH | /api/users/me | 401 |
| 3 | GET | /api/users | 401 |
| 4 | GET | /api/chats | 401 |
| 5 | POST | /api/chats | 401 |
| 6 | POST | /api/dms | 401 |
| 7 | POST | /api/auth/logout | 401 |

### 3. TestInvalidAccessToken
| # | Method | Path | Token | Expected |
|---|--------|------|-------|----------|
| 1 | GET | /api/users/me | `this.is.not.a.jwt` | 401 |
| 2 | GET | /api/users/me | `Bearer .` | 401 |
| 3 | GET | /api/users/me | `` | 401 |

### 4. TestTamperedRefreshToken
| # | Subtest | Method | Path | Cookie | Expected |
|---|---------|--------|------|--------|----------|
| 1 | random string | POST | /api/auth/refresh | `refresh_token=i-am-not-a-real-token` | 401 |
| 2 | empty value | POST | /api/auth/refresh | `refresh_token=` | 401 |
| 3 | unknown hash | POST | /api/auth/refresh | `refresh_token=aabbccdd0011...` | 401 |

### 5. TestRefreshTokenRotation
| # | Method | Path | Cookie | Expected |
|---|--------|------|--------|----------|
| 1 | POST | /api/auth/register | — | 200, get `refresh_token_A` |
| 2 | POST | /api/auth/refresh | `refresh_token=refresh_token_A` | 200, get `refresh_token_B` (different) |
| 3 | POST | /api/auth/refresh | `refresh_token=refresh_token_A` | 401 (rotated, reused) |
| 4 | POST | /api/auth/refresh | `refresh_token=refresh_token_B` | 200 (chain still works) |

### 6. TestRefreshWithoutCookie
| # | Method | Path | Cookie | Expected |
|---|--------|------|--------|----------|
| 1 | POST | /api/auth/refresh | (none) | 400 `{"error":"bad_request","message":"refresh token missing"}` |

### 7. TestRegisterDuplicateEmail
| # | Method | Path | Body | Expected |
|---|--------|------|------|----------|
| 1 | POST | /api/auth/register | `{email:"dup@test.dev",username:"FirstUser",password:...}` | 200 |
| 2 | POST | /api/auth/register | `{email:"dup@test.dev",username:"SecondUser",password:...}` | 409 |

### 8. TestRegisterDuplicateUsername
| # | Method | Path | Body | Expected |
|---|--------|------|------|----------|
| 1 | POST | /api/auth/register | `{email:"dupname1@...",username:"DupName",password:...}` | 200 |
| 2 | POST | /api/auth/register | `{email:"dupname2@...",username:"DupName",password:...}` | 409 |

### 9. TestLoginWrongPassword
| # | Method | Path | Body | Expected |
|---|--------|------|------|----------|
| 1 | POST | /api/auth/register | `{email:"wrongpw@...",password:"correct_horse"}` | 200 |
| 2 | POST | /api/auth/login | `{email:"wrongpw@...",password:"battery_staple"}` | 401 + `{"error":"invalid_credentials"}` |

### 10. TestRegisterInvalidInput
| # | Subtest | Method | Path | Body | Expected |
|---|---------|--------|------|------|----------|
| 1 | bad email | POST | /api/auth/register | `{email:"not-an-email",...}` | 400 |
| 2 | short username | POST | /api/auth/register | `{username:"a",...}` | 400 |
| 3 | short password | POST | /api/auth/register | `{password:"12",...}` | 400 |
| 4 | empty email | POST | /api/auth/register | `{email:"",...}` | 400 |

---

## Results
**33/33 tests PASS** across all packages:
- `internal/auth` — 0.228s
- `internal/db` — 0.530s
- `internal/testutil` — 2.197s
- `internal/ws` — 0.664s
