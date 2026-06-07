package testutil_test

import (
	"testing"

	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

func TestFixtureSetup(t *testing.T) {
	f := testutil.New(t)
	if f.DB == nil || f.Auth == nil || f.Hub == nil || f.Server == nil || f.HTTP == nil {
		t.Fatal("fixture incomplete")
	}
}

func TestUserRegisterLogin(t *testing.T) {
	f := testutil.New(t)
	s1 := f.Register(t, "alice@test.dev", "alice", "password123")
	if s1.AccessToken == "" || s1.RefreshToken == "" || s1.UserID == "" {
		t.Fatal("register response incomplete")
	}
	s2 := f.Login(t, "alice@test.dev", "password123")
	if s2.UserID != s1.UserID {
		t.Fatal("login returned different user")
	}
}

func TestDuplicateEmail(t *testing.T) {
	f := testutil.New(t)
	f.Register(t, "bob@test.dev", "bob", "password123")
	res := f.Do(t, "POST", "/api/auth/register", "", map[string]string{
		"email": "bob@test.dev", "username": "bob2", "password": "password123",
	})
	defer res.Body.Close()
	if res.StatusCode != 409 {
		t.Fatalf("duplicate email: want 409 got %d", res.StatusCode)
	}
}

func TestUnauthorizedAccess(t *testing.T) {
	f := testutil.New(t)
	res := f.Do(t, "GET", "/api/users/me", "", nil)
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("want 401 got %d", res.StatusCode)
	}
	res = f.Do(t, "GET", "/api/chats", "", nil)
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("want 401 got %d", res.StatusCode)
	}
}

func TestRefreshTokenFlow(t *testing.T) {
	f := testutil.New(t)
	s := f.Register(t, "refresh@test.dev", "refresher", "password123")
	res := f.Do(t, "POST", "/api/auth/refresh", "", map[string]string{
		"refresh_token": s.RefreshToken,
	})
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("refresh failed: %d", res.StatusCode)
	}
	res2 := f.Do(t, "POST", "/api/auth/refresh", "", map[string]string{
		"refresh_token": s.RefreshToken,
	})
	defer res2.Body.Close()
	if res2.StatusCode == 200 {
		t.Fatal("reused refresh token should fail")
	}
}