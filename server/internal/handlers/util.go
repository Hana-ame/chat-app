package handlers

import (
	"net/http"
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
		SameSite: http.SameSiteLaxMode, // Lax for a better UX on navigation
		MaxAge:   int(ttl.Seconds()),
	})
}

func setRefreshCookie(w http.ResponseWriter, r *http.Request, raw string, ttl time.Duration) {
	setAuthCookie(w, r, "refresh_token", raw, "/api/auth/refresh", ttl)
}

func clearRefreshCookie(w http.ResponseWriter, r *http.Request) {
	setAuthCookie(w, r, "refresh_token", "", "/api/auth/refresh", -1*time.Second)
}
