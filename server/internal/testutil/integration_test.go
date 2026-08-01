// Package testutil_test 覆盖基础冒烟:fixture 装配、注册登录、重复邮箱、
// 未授权、refresh 流程。
//
// 运行方式: cd server && go test ./internal/testutil/
package testutil_test

import (
	"testing"

	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

func TestFixtureSetup(t *testing.T) {
	f := testutil.New(t)
	testutil.RequireNotNil(t, f.DB)
	testutil.RequireNotNil(t, f.Auth)
	testutil.RequireNotNil(t, f.Hub)
	testutil.RequireNotNil(t, f.Server)
	testutil.RequireNotNil(t, f.HTTP)
}

func TestUserRegisterLogin(t *testing.T) {
	f := testutil.New(t)
	s1 := f.Register(t, "alice@test.dev", "alice", "password123")
	testutil.RequireTrue(t, s1.AccessToken != "" && s1.RefreshToken != "" && s1.UserID != "", "register response incomplete")
	s2 := f.Login(t, "alice@test.dev", "password123")
	testutil.RequireEqual(t, s2.UserID, s1.UserID)
}

func TestDuplicateEmail(t *testing.T) {
	f := testutil.New(t)
	f.Register(t, "bob@test.dev", "bob", "password123")
	res := f.Do(t, "POST", "/api/auth/register", "", map[string]string{
		"email": "bob@test.dev", "username": "bob2", "password": "password123",
	})
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 409)
}

func TestUnauthorizedAccess(t *testing.T) {
	f := testutil.New(t)
	res := f.Do(t, "GET", "/api/users/me", "", nil)
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 401)
	res = f.Do(t, "GET", "/api/chats/my", "", nil)
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 401)
}

func TestRefreshTokenFlow(t *testing.T) {
	f := testutil.New(t)
	s := f.Register(t, "refresh@test.dev", "refresher", "password123")
	s2 := f.Refresh(t, s.RefreshToken)
	testutil.RequireEqual(t, s2.UserID, s.UserID)
	res2 := f.DoWithCookie(t, "POST", "/api/auth/refresh", "", "refresh_token", s.RefreshToken, nil)
	defer res2.Body.Close()
	testutil.RequireStatus(t, res2, 401)
}
