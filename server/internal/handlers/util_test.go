package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/service"
)

func TestMapServiceError(t *testing.T) {
	tests := []struct {
		err      error
		wantCode int
		wantStr  string
	}{
		{nil, 0, ""},
		{service.ErrForbidden, http.StatusForbidden, "forbidden"},
		{service.ErrNotFound, http.StatusNotFound, "not_found"},
		{service.ErrInvalidInput, http.StatusBadRequest, "bad_request"},
		{service.ErrConflict, http.StatusConflict, "conflict"},
		{service.ErrContentTooLong, http.StatusRequestEntityTooLarge, "content_too_long"},
		{errors.New("unknown"), http.StatusInternalServerError, "internal"},
	}
	for _, tc := range tests {
		statusCode, errorCode := mapServiceError(tc.err)
		if statusCode != tc.wantCode || errorCode != tc.wantStr {
			t.Errorf("mapServiceError(%v): got (%d, %s), want (%d, %s)",
				tc.err, statusCode, errorCode, tc.wantCode, tc.wantStr)
		}
	}
}

func TestIntQueryParam(t *testing.T) {
	r := httptest.NewRequest("GET", "/test?n=42&e=&invalid=abc", nil)
	if v := intQueryParam(r, "n", 0); v != 42 {
		t.Fatalf("want 42, got %d", v)
	}
	if v := intQueryParam(r, "e", 10); v != 10 {
		t.Fatalf("want 10 (default), got %d", v)
	}
	if v := intQueryParam(r, "invalid", 99); v != 99 {
		t.Fatalf("want 99 (default for invalid), got %d", v)
	}
	if v := intQueryParam(r, "missing", 7); v != 7 {
		t.Fatalf("want 7 (default), got %d", v)
	}
}

func TestBearerToken_AuthorizationHeader(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer my-token-123")
	tok := bearerToken(r)
	if tok != "my-token-123" {
		t.Fatalf("want my-token-123, got %s", tok)
	}
}

func TestBearerToken_NoHeader(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	tok := bearerToken(r)
	if tok != "" {
		t.Fatalf("want empty, got %s", tok)
	}
}

func TestBearerToken_QueryParam(t *testing.T) {
	// Query-param tokens are rejected: they leak via access logs and Referer.
	r := httptest.NewRequest("GET", "/?access_token=query-token", nil)
	tok := bearerToken(r)
	if tok != "" {
		t.Fatalf("want empty (query param rejected), got %s", tok)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "value"})

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("want application/json, got %s", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["key"] != "value" {
		t.Fatalf("want value, got %s", body["key"])
	}
}

func TestWriteJSON_NilBody(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusNoContent, nil)
	resp := w.Result()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if len(body) != 0 {
		t.Fatalf("want empty body, got %s", string(body))
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "bad_request", "invalid input")
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "bad_request" || body["message"] != "invalid input" {
		t.Fatalf("unexpected body: %v", body)
	}
}

func TestDecodeJSON(t *testing.T) {
	body := `{"key":"value","num":42}`
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	var out struct {
		Key string `json:"key"`
		Num int    `json:"num"`
	}
	if err := decodeJSON(r, &out); err != nil {
		t.Fatal(err)
	}
	if out.Key != "value" || out.Num != 42 {
		t.Fatalf("got %+v", out)
	}
}

func TestDecodeJSON_EmptyBody(t *testing.T) {
	r := httptest.NewRequest("POST", "/", http.NoBody)
	err := decodeJSON(r, &struct{}{})
	if err == nil {
		t.Fatal("expected error for empty body")
	}
}

func TestDecodeJSON_DisallowUnknownFields(t *testing.T) {
	body := `{"known":"ok","unknown":"field"}`
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	var out struct {
		Known string `json:"known"`
	}
	err := decodeJSON(r, &out)
	if err == nil {
		t.Fatal("should reject unknown fields")
	}
}

func TestSetAuthCookie(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	setAuthCookie(w, r, "access_token", "tok123", "/", time.Hour)

	cookies := w.Result().Cookies()
	var found *http.Cookie
	for _, c := range cookies {
		if c.Name == "access_token" {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("cookie not set")
	}
	if found.Value != "tok123" {
		t.Fatalf("want tok123, got %s", found.Value)
	}
	if !found.HttpOnly {
		t.Fatal("cookie should be HttpOnly")
	}
	if found.MaxAge != 3600 {
		t.Fatalf("want MaxAge 3600, got %d", found.MaxAge)
	}
	if found.SameSite != http.SameSiteLaxMode {
		t.Fatal("cookie should be SameSite=Lax")
	}
}

func TestSetAuthCookie_Https(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	setAuthCookie(w, r, "access_token", "tok123", "/", time.Hour)

	cookies := w.Result().Cookies()
	var found *http.Cookie
	for _, c := range cookies {
		if c.Name == "access_token" {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("cookie not set")
	}
	if !found.Secure {
		t.Fatal("cookie should be Secure when HTTPS")
	}
}

func TestSetRefreshCookie(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	setRefreshCookie(w, r, "raw-refresh-token", 72*time.Hour)

	cookies := w.Result().Cookies()
	var found *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("refresh cookie not set")
	}
	if found.Path != "/api/auth/refresh" {
		t.Fatalf("want /api/auth/refresh, got %s", found.Path)
	}
	if !found.HttpOnly {
		t.Fatal("should be HttpOnly")
	}
}

func TestClearRefreshCookie(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	clearRefreshCookie(w, r)

	cookies := w.Result().Cookies()
	var found *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("clear cookie not set")
	}
	if found.Value != "" {
		t.Fatalf("want empty value, got %s", found.Value)
	}
	if found.MaxAge != -1 {
		t.Fatalf("want MaxAge -1, got %d", found.MaxAge)
	}
}

func TestClearAccessTokenCookie(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	clearAccessTokenCookie(w, r)

	cookies := w.Result().Cookies()
	var found *http.Cookie
	for _, c := range cookies {
		if c.Name == "access_token" {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("clear token cookie not set")
	}
	if found.Value != "" {
		t.Fatalf("want empty, got %s", found.Value)
	}
	if found.Path != "/" {
		t.Fatalf("want /, got %s", found.Path)
	}
}

func TestBearerToken_AuthorizationPriority(t *testing.T) {
	r := httptest.NewRequest("GET", "/?access_token=query-token", nil)
	r.Header.Set("Authorization", "Bearer header-token")
	tok := bearerToken(r)
	if tok != "header-token" {
		t.Fatalf("want header-token, got %s", tok)
	}
}
