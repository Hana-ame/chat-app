# Security Reference

## Content Security Policy (CSP)

Applied globally in `router.go`. Policy:

```
default-src 'self'
script-src 'self' 'unsafe-inline'
style-src 'self' 'unsafe-inline'
img-src 'self' https://upload.moonchan.xyz data:
connect-src 'self' wss://wsl-8080.moonchan.xyz https://upload.moonchan.xyz
font-src 'self' data:
```

## CORS

- **AllowOriginFunc**: returns `true` for all origins (open CORS)
- **Methods**: GET, POST, PATCH, PUT, DELETE, OPTIONS
- **Headers**: all allowed
- **Credentials**: true
- **MaxAge**: 300s

## Cookie Security

Access and refresh tokens are stored in httpOnly cookies:

| Cookie | Path | HttpOnly | Secure | SameSite |
|---|---|---|---|---|
| `access_token` | `/` | Yes | Auto-detected (TLS / X-Forwarded-Proto) | Lax |
| `refresh_token` | `/api/auth/refresh` | Yes | Auto-detected | Lax |

`Secure` flag is set when `r.TLS != nil` or `X-Forwarded-Proto: https`.

## JWT

- **Algorithm**: HS256
- **Access token**: short-lived (default 30m), contains `uid`, `exp`, `iat`, `sub`
- **Refresh token**: 32-byte random + SHA256 hash stored in DB, single-use

## IP Detection (Cloudflare)

When behind Cloudflare, the server fetches Cloudflare IP ranges from `https://www.cloudflare.com/ips-v4` and `ips-v6`, then strips them from `X-Forwarded-For` to get the real client IP. Falls back to hardcoded CIDRs if fetch fails.

## Rate Limiting

See [rate-limiting.md](./rate-limiting.md).
