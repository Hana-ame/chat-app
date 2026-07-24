package handlers

import (
	"net/http"
	"strconv"
	"time"
)

func timeNow() time.Time { return time.Now().UTC() }

func isSecure(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func setAuthCookie(w http.ResponseWriter, r *http.Request, name, value string, path string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		HttpOnly: true,
		Secure:   isSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
}

func setRefreshCookie(w http.ResponseWriter, r *http.Request, raw string, ttl time.Duration) {
    http.SetCookie(w, &http.Cookie{
        Name:     "refresh_token",
        Value:    raw,
        Path:     "/api/auth/refresh",
        HttpOnly: true,
        Secure:   isSecure(r),
        SameSite: http.SameSiteLaxMode,
        MaxAge:   int(ttl.Seconds()),
    })
}

func clearRefreshCookie(w http.ResponseWriter, r *http.Request) {
    http.SetCookie(w, &http.Cookie{
        Name:     "refresh_token",
        Value:    "",
        Path:     "/api/auth/refresh",
        HttpOnly: true,
        Secure:   isSecure(r),
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
    http.SetCookie(w, &http.Cookie{
        Name:     "access_token",
        Value:    "",
        Path:     "/",
        HttpOnly: true,
        Secure:   isSecure(r),
        SameSite: http.SameSiteLaxMode,
        MaxAge:   -1,
    })
}
