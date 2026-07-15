# v1.0 Deployment & Same-Site Hardening Report

## 🎯 Objective
Transition the application to a **Same-Origin (Same-Site)** deployment architecture to maximize security by leveraging `HttpOnly` cookies and eliminating sensitive token exposure in URLs and JavaScript state.

## 🛠️ Implementation Details

### 1. WebSocket (WS) Security Isolation
- **Action**: Added a "Soft-Disable" switch in `internal/ws/gateway.go`.
- **Logic**: Checks `os.Getenv("WS_ENABLED")`. If not `"true"`, returns `403 Forbidden`.
- **Security Gain**: Completely isolates the vulnerable WebSocket broadcast and auth logic until v2.0 fixes are deployed, preventing CSWSH and token leakage.

### 2. Cookie-Based Authentication Migration
- **Backend (`auth.go`, `util.go`)**: 
    - Implemented `setAuthCookie` helper.
    - `issueSession` now sets both `access_token` and `refresh_token` as **HttpOnly, Secure, SameSite=Lax** cookies.
- **Middleware (`handler.go`)**: 
    - `authMiddleware` now prioritizes reading the `access_token` from cookies if the `Authorization` header is absent.
- **Security Gain**: Tokens are now opaque to the frontend JS, preventing theft via XSS.

### 3. Frontend Adaptation
- **API Client (`client.js`)**: 
    - Global `fetch` options updated to `credentials: 'include'`.
    - Removed manual `Authorization: Bearer <token>` header injection.
- **Auth Store (`auth.js`)**: 
    - Removed `accessToken` from Zustand state and `localStorage`.
- **Result**: Frontend is now a "thin client" regarding auth, relying on the browser's secure cookie management.

### 4. Same-Site Static Hosting & SPA Routing
- **Configuration (`.env`)**: Set `CHAT_STATIC_DIR=/mnt/d/WorkPlace/chat-app/client/dist`.
- **Hosting**: The Go server now serves the `dist` folder, including the `assets/` directory.
- **SPA Fallback**: `serveStatic` handles unmatched routes by serving `index.html`, enabling client-side routing (e.g., `/chat/123`) to work on refresh.

### 5. Path Traversal Vulnerability Fix
- **Vulnerability**: `serveStatic` previously joined `StaticDir` with raw `r.URL.Path`, allowing `../` attacks.
- **Fix**: 
    - Applied `filepath.Clean` to the requested path.
    - Added a strict prefix check: `if !strings.HasPrefix(p, s.Cfg.StaticDir) { return 404 }`.
- **Security Gain**: Prevents unauthorized access to files outside the static directory.

---

## 🔒 Final v1.0 Security Posture

| Component | Status | Security Mechanism |
|---|---|---|
| **Access Token** | ✅ Hardened | HttpOnly Cookie (SameSite=Lax) |
| **Refresh Token** | ✅ Hardened | HttpOnly Cookie (SameSite=Strict) |
| **WebSocket** | ✅ Isolated | Disabled via `WS_ENABLED=false` |
| **Static Files** | ✅ Hardened | Path Traversal Guard + SPA Fallback |
| **Transport** | ✅ Secured | `credentials: 'include'` $\rightarrow$ Secure Cookies |

## 🚀 Deployment Checklist
- [ ] Set `WS_ENABLED=false` in environment.
- [ ] Ensure `CHAT_STATIC_DIR` points to the build output.
- [ ] Deploy via Cloudflare Tunnel with HTTPS enabled.
