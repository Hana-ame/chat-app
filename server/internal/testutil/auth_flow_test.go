// Package testutil_test 覆盖 HTTP 层认证流程:注册/登录/刷新、无 token 拒绝、
// 篡改 refresh token、并发轮换、登出失效、Cookie 安全属性、多设备隔离。
//
// 运行方式: cd server && go test ./internal/testutil/
package testutil_test

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

func TestRegisterLoginRefresh(t *testing.T) {
	f := testutil.New(t)

	s1 := f.Register(t, "flow@test.dev", "FlowUser", "securePass1!")
	testutil.RequireTrue(t, s1.AccessToken != "", "register: missing access_token")
	testutil.RequireTrue(t, s1.RefreshToken != "", "register: missing refresh_token cookie")
	testutil.RequireTrue(t, s1.UserID != "", "register: missing user id")

	s2 := f.Login(t, "flow@test.dev", "securePass1!")
	testutil.RequireEqual(t, s2.UserID, s1.UserID)
	testutil.RequireTrue(t, s2.RefreshToken != "", "login: missing refresh_token cookie")

	s3 := f.Refresh(t, s2.RefreshToken)
	testutil.RequireEqual(t, s3.UserID, s1.UserID)
	testutil.RequireTrue(t, s3.RefreshToken != "", "refresh: missing new refresh_token cookie")
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
		{"GET", "/api/chats/my"},
		{"POST", "/api/chats"},
		{"POST", "/api/dms"},
		{"POST", "/api/auth/logout"},
	}
	for _, ep := range endpoints {
		res := f.Do(t, ep.method, ep.path, "", nil)
		res.Body.Close()
		testutil.RequireStatus(t, res, 401)
	}
}

func TestInvalidAccessToken(t *testing.T) {
	f := testutil.New(t)

	res := f.Do(t, "GET", "/api/users/me", "this.is.not.a.jwt", nil)
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 401)

	res = f.Do(t, "GET", "/api/users/me", "Bearer .", nil)
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 401)

	res = f.Do(t, "GET", "/api/users/me", "", nil)
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 401)
}

func TestTamperedRefreshToken(t *testing.T) {
	f := testutil.New(t)

	t.Run("totally random string", func(t *testing.T) {
		res := f.DoWithCookie(t, "POST", "/api/auth/refresh", "", "refresh_token", "i-am-not-a-real-token", nil)
		defer res.Body.Close()
		testutil.RequireStatus(t, res, 401)
	})

	t.Run("empty cookie value", func(t *testing.T) {
		res := f.DoWithCookie(t, "POST", "/api/auth/refresh", "", "refresh_token", "", nil)
		defer res.Body.Close()
		testutil.RequireStatus(t, res, 401)
	})

	t.Run("valid format but unknown hash", func(t *testing.T) {
		// RefreshHandler hashes the cookie value and looks it up in DB.
		// A syntactically valid token format but unknown hash should 401.
		res := f.DoWithCookie(t, "POST", "/api/auth/refresh", "", "refresh_token", "aabbccdd00112233445566778899aabbccdd00112233445566778899aabbccdd", nil)
		defer res.Body.Close()
		testutil.RequireStatus(t, res, 401)
	})
}

func TestRefreshTokenRotation(t *testing.T) {
	f := testutil.New(t)

	s := f.Register(t, "rotate@test.dev", "RotateUser", "testPass1!")
	oldRefresh := s.RefreshToken

	s2 := f.Refresh(t, oldRefresh)
	testutil.RequireNotEqual(t, s2.RefreshToken, oldRefresh)

	// Reuse the old (already consumed) refresh token
	res := f.DoWithCookie(t, "POST", "/api/auth/refresh", "", "refresh_token", oldRefresh, nil)
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 401)

	// New token should still work
	_ = f.Refresh(t, s2.RefreshToken)
}

func TestRefreshWithoutCookie(t *testing.T) {
	f := testutil.New(t)

	res := f.Do(t, "POST", "/api/auth/refresh", "", nil)
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 400)
	var errResp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	testutil.RequireNoError(t, json.NewDecoder(res.Body).Decode(&errResp))
	testutil.RequireEqual(t, errResp.Error, "bad_request")
}

func TestRegisterDuplicateEmail(t *testing.T) {
	f := testutil.New(t)

	f.Register(t, "dup@test.dev", "FirstUser", "testPass1!")
	res := f.Do(t, "POST", "/api/auth/register", "", map[string]string{
		"email": "dup@test.dev", "username": "SecondUser", "password": "testPass1!",
	})
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 409)
}

func TestRegisterDuplicateUsername(t *testing.T) {
	f := testutil.New(t)

	f.Register(t, "dupname1@test.dev", "DupName", "testPass1!")
	res := f.Do(t, "POST", "/api/auth/register", "", map[string]string{
		"email": "dupname2@test.dev", "username": "DupName", "password": "testPass1!",
	})
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 409)
}

func TestLoginWrongPassword(t *testing.T) {
	f := testutil.New(t)

	f.Register(t, "wrongpw@test.dev", "WrongPW", "correct_horse")
	res := f.Do(t, "POST", "/api/auth/login", "", map[string]string{
		"email": "wrongpw@test.dev", "password": "battery_staple",
	})
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 401)
	var errResp struct {
		Error string `json:"error"`
	}
	testutil.RequireNoError(t, json.NewDecoder(res.Body).Decode(&errResp))
	testutil.RequireEqual(t, errResp.Error, "invalid_credentials")
}

func TestConcurrentRefreshRotation(t *testing.T) {
	f := testutil.New(t)
	s := f.Register(t, "concur@test.dev", "ConcurUser", "testPass1!")

	const N = 10
	var mu sync.Mutex
	okCount := 0
	errCount := 0

	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := f.DoWithCookie(t, "POST", "/api/auth/refresh", "", "refresh_token", s.RefreshToken, nil)
			res.Body.Close()
			mu.Lock()
			if res.StatusCode == 200 {
				okCount++
			} else {
				errCount++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	testutil.RequireEqual(t, okCount, 1)
	testutil.RequireEqual(t, errCount, N-1)
}

func TestLogoutInvalidatesTokens(t *testing.T) {
	f := testutil.New(t)
	s := f.Register(t, "logout2@test.dev", "LogoutTester", "testPass1!")

	res := f.Do(t, "POST", "/api/auth/logout", s.AccessToken, nil)
	res.Body.Close()
	testutil.RequireStatus(t, res, 200)

	// JWT is stateless — access token remains valid until expiry.
	// Logout only kills the refresh token chain.
	res2 := f.Do(t, "GET", "/api/users/me", s.AccessToken, nil)
	res2.Body.Close()
	testutil.RequireStatus(t, res2, 200)

	res3 := f.DoWithCookie(t, "POST", "/api/auth/refresh", "", "refresh_token", s.RefreshToken, nil)
	res3.Body.Close()
	testutil.RequireStatus(t, res3, 401)
}

func TestCookieSecurityAttributes(t *testing.T) {
	f := testutil.New(t)
	res := f.Do(t, "POST", "/api/auth/register", "", map[string]string{
		"email": "cookie@test.dev", "username": "CookieUser", "password": "testPass1!",
	})
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 200)
	c := testutil.ResponseCookie(res, "refresh_token")
	testutil.RequireNotNil(t, c)
	if !c.HttpOnly {
		t.Error("cookie missing HttpOnly flag")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite: want LaxMode(%d) got %d", http.SameSiteLaxMode, c.SameSite)
	}
}

func TestMultiDeviceRefreshIsolation(t *testing.T) {
	f := testutil.New(t)

	_ = f.Register(t, "multi@test.dev", "MultiUser", "testPass1!")
	devA := f.Login(t, "multi@test.dev", "testPass1!")
	devB := f.Login(t, "multi@test.dev", "testPass1!")
	testutil.RequireNotEqual(t, devA.RefreshToken, devB.RefreshToken)

	devA2 := f.Refresh(t, devA.RefreshToken)
	devB2 := f.Refresh(t, devB.RefreshToken)

	resA := f.DoWithCookie(t, "POST", "/api/auth/refresh", "", "refresh_token", devA.RefreshToken, nil)
	defer resA.Body.Close()
	testutil.RequireStatus(t, resA, 401)

	resB := f.DoWithCookie(t, "POST", "/api/auth/refresh", "", "refresh_token", devB.RefreshToken, nil)
	defer resB.Body.Close()
	testutil.RequireStatus(t, resB, 401)

	devA3 := f.Refresh(t, devA2.RefreshToken)
	testutil.RequireEqual(t, devA3.UserID, devA2.UserID)

	devB3 := f.Refresh(t, devB2.RefreshToken)
	testutil.RequireEqual(t, devB3.UserID, devB2.UserID)
}

func TestRegisterNoValidation(t *testing.T) {
	// All validations removed per spec: only uniqueness check remains.
	f := testutil.New(t)
	tests := []struct {
		name     string
		email    string
		username string
		password string
	}{
		{"any email", "not-an-email", "UserA", "password123"},
		{"short username", "u@t.com", "a", "password123"},
		{"valid input", "p@t.com", "UserB", "password123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := f.Do(t, "POST", "/api/auth/register", "", map[string]string{
				"email": tt.email, "username": tt.username, "password": tt.password,
			})
			res.Body.Close()
			testutil.RequireStatus(t, res, 200)
		})
	}
	// empty email should error at DB layer
	res := f.Do(t, "POST", "/api/auth/register", "", map[string]string{
		"email": "", "username": "UserC", "password": "password123",
	})
	res.Body.Close()
	testutil.RequireStatus(t, res, 500)
}
