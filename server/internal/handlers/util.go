package handlers

import (
	"net/http"
	"strconv"
	"time"
)

func timeNow() time.Time { return time.Now().UTC() }

func setAuthCookie(w http.ResponseWriter, r *http.Request, name, value string, path string, ttl time.Duration) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode, // Lax is intentionally allowed for better UX on cross‑site navigation (e.g., redirects after OAuth login). This is a conscious decision; see security considerations in the documentation.
		MaxAge:   int(ttl.Seconds()),
	})
}

func setRefreshCookie(w http.ResponseWriter, r *http.Request, raw string, ttl time.Duration) {
    secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
    http.SetCookie(w, &http.Cookie{
        Name:     "refresh_token",
        Value:    raw,
        Path:     "/api/auth/refresh",
        HttpOnly: true,
        Secure:   secure,
        // Lax is intentionally allowed for better UX on cross‑site navigation (e.g., redirects after OAuth login).
        SameSite: http.SameSiteLaxMode,
        MaxAge:   int(ttl.Seconds()),
    })
}

func clearRefreshCookie(w http.ResponseWriter, r *http.Request) {
    secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
    http.SetCookie(w, &http.Cookie{
        Name:     "refresh_token",
        Value:    "",
        Path:     "/api/auth/refresh",
        HttpOnly: true,
        Secure:   secure,
        // Lax is intentionally allowed for consistency with the refresh cookie creation.
        SameSite: http.SameSiteLaxMode,
        MaxAge:   -1,
    })
}

func intQueryParam(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func clearAccessTokenCookie(w http.ResponseWriter, r *http.Request) {
    secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
    http.SetCookie(w, &http.Cookie{
        Name:     "access_token",
        Value:    "",
        Path:     "/",
        HttpOnly: true,
        Secure:   secure,
        SameSite: http.SameSiteLaxMode,
        MaxAge:   -1,
    })
}
