package testutil_test

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

func TestRegisterLoginRefresh(t *testing.T) {
	f := testutil.New(t)

	s1 := f.Register(t, "flow@test.dev", "FlowUser", "securePass1!")
	if s1.AccessToken == "" {
		t.Fatal("register: missing access_token")
	}
	if s1.RefreshToken == "" {
		t.Fatal("register: missing refresh_token cookie")
	}
	if s1.UserID == "" {
		t.Fatal("register: missing user id")
	}

	s2 := f.Login(t, "flow@test.dev", "securePass1!")
	if s2.UserID != s1.UserID {
		t.Fatal("login: user mismatch with register")
	}
	if s2.RefreshToken == "" {
		t.Fatal("login: missing refresh_token cookie")
	}

	s3 := f.Refresh(t, s2.RefreshToken)
	if s3.UserID != s1.UserID {
		t.Fatal("refresh: user mismatch")
	}
	if s3.RefreshToken == "" {
		t.Fatal("refresh: missing new refresh_token cookie")
	}
}

func TestAccessDeniedWithoutToken(t *testing.T) {
	f := testutil.New(t)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/users/me"},
		{"PATCH", "/api/users/me"},
		{"GET", "/api/users"},
		{"GET", "/api/chats"},
		{"POST", "/api/chats"},
		{"POST", "/api/dms"},
		{"POST", "/api/auth/logout"},
	}
	for _, ep := range endpoints {
		res := f.Do(t, ep.method, ep.path, "", nil)
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatalf("%s %s: want 401 got %d body=%s", ep.method, ep.path, res.StatusCode, string(body))
		}
	}
}

func TestInvalidAccessToken(t *testing.T) {
	f := testutil.New(t)

	res := f.Do(t, "GET", "/api/users/me", "this.is.not.a.jwt", nil)
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("invalid jwt: want 401 got %d", res.StatusCode)
	}

	res = f.Do(t, "GET", "/api/users/me", "Bearer .", nil)
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("malformed bearer: want 401 got %d", res.StatusCode)
	}

	res = f.Do(t, "GET", "/api/users/me", "", nil)
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("empty token: want 401 got %d", res.StatusCode)
	}
}

func TestTamperedRefreshToken(t *testing.T) {
	f := testutil.New(t)

	t.Run("totally random string", func(t *testing.T) {
		res := f.DoWithCookie(t, "POST", "/api/auth/refresh", "", "refresh_token", "i-am-not-a-real-token", nil)
		defer res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatalf("want 401 got %d", res.StatusCode)
		}
	})

	t.Run("empty cookie value", func(t *testing.T) {
		res := f.DoWithCookie(t, "POST", "/api/auth/refresh", "", "refresh_token", "", nil)
		defer res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatalf("want 401 got %d", res.StatusCode)
		}
	})

	t.Run("valid format but unknown hash", func(t *testing.T) {
		// RefreshHandler hashes the cookie value and looks it up in DB.
		// A syntactically valid token format but unknown hash should 401.
		res := f.DoWithCookie(t, "POST", "/api/auth/refresh", "", "refresh_token", "aabbccdd00112233445566778899aabbccdd00112233445566778899aabbccdd", nil)
		defer res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatalf("want 401 got %d", res.StatusCode)
		}
	})
}

func TestRefreshTokenRotation(t *testing.T) {
	f := testutil.New(t)

	s := f.Register(t, "rotate@test.dev", "RotateUser", "testPass1!")
	oldRefresh := s.RefreshToken

	s2 := f.Refresh(t, oldRefresh)
	if s2.RefreshToken == oldRefresh {
		t.Fatal("refresh should rotate the token")
	}

	// Reuse the old (already consumed) refresh token
	res := f.DoWithCookie(t, "POST", "/api/auth/refresh", "", "refresh_token", oldRefresh, nil)
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("reused old refresh token: want 401 got %d", res.StatusCode)
	}

	// New token should still work
	_ = f.Refresh(t, s2.RefreshToken)
}

func TestRefreshWithoutCookie(t *testing.T) {
	f := testutil.New(t)

	res := f.Do(t, "POST", "/api/auth/refresh", "", nil)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("no cookie: want 400 got %d", res.StatusCode)
	}

	var errResp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(res.Body).Decode(&errResp); err != nil {
		t.Fatal("expected JSON error response")
	}
	if errResp.Error != "bad_request" {
		t.Fatalf("want error='bad_request' got '%s'", errResp.Error)
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	f := testutil.New(t)

	f.Register(t, "dup@test.dev", "FirstUser", "testPass1!")
	res := f.Do(t, "POST", "/api/auth/register", "", map[string]string{
		"email": "dup@test.dev", "username": "SecondUser", "password": "testPass1!",
	})
	defer res.Body.Close()
	if res.StatusCode != 409 {
		t.Fatalf("duplicate email: want 409 got %d", res.StatusCode)
	}
}

func TestRegisterDuplicateUsername(t *testing.T) {
	f := testutil.New(t)

	f.Register(t, "dupname1@test.dev", "DupName", "testPass1!")
	res := f.Do(t, "POST", "/api/auth/register", "", map[string]string{
		"email": "dupname2@test.dev", "username": "DupName", "password": "testPass1!",
	})
	defer res.Body.Close()
	if res.StatusCode != 409 {
		t.Fatalf("duplicate username: want 409 got %d", res.StatusCode)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	f := testutil.New(t)

	f.Register(t, "wrongpw@test.dev", "WrongPW", "correct_horse")
	res := f.Do(t, "POST", "/api/auth/login", "", map[string]string{
		"email": "wrongpw@test.dev", "password": "battery_staple",
	})
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("wrong password: want 401 got %d", res.StatusCode)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&errResp); err != nil {
		t.Fatal("expected JSON error")
	}
	if errResp.Error != "invalid_credentials" {
		t.Fatalf("want 'invalid_credentials' got '%s'", errResp.Error)
	}
}

func TestRegisterInvalidInput(t *testing.T) {
	f := testutil.New(t)

	tests := []struct {
		name     string
		email    string
		username string
		password string
		wantCode int
	}{
		{"bad email", "not-an-email", "ValidName", "password123", 400},
		{"short username", "u@t.com", "a", "password123", 400},
		{"short password", "p@t.com", "ValidName", "12", 400},
		{"empty email", "", "ValidName", "password123", 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := f.Do(t, "POST", "/api/auth/register", "", map[string]string{
				"email": tt.email, "username": tt.username, "password": tt.password,
			})
			defer res.Body.Close()
			if res.StatusCode != tt.wantCode {
				b, _ := io.ReadAll(res.Body)
				t.Fatalf("want %d got %d body=%s", tt.wantCode, res.StatusCode, string(b))
			}
		})
	}
}
