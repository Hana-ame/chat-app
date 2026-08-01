// Package auth_test 覆盖认证服务:密码哈希/校验、JWT 签发/解析/过期/篡改、
// 邮箱与用户名规范化、refresh token 轮换。
//
// 运行方式: cd server && go test ./internal/auth/
// 说明:纯单元测试,不依赖 DB;token 用内存 secret 构造。
package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/auth"
	"github.com/golang-jwt/jwt/v5"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := auth.HashPassword("my-secret-123")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "my-secret-123" {
		t.Fatal("password not hashed")
	}
	if err := auth.VerifyPassword(hash, "my-secret-123"); err != nil {
		t.Fatalf("verify correct pw: %v", err)
	}
	if err := auth.VerifyPassword(hash, "wrong-password"); err == nil {
		t.Fatal("wrong password should fail")
	}
	// No restriction on password length; any non-empty works.
}

func TestJWTIssueAndParse(t *testing.T) {
	secret := []byte("test-secret-for-jwt-testing--32b")
	svc := auth.New(secret, 15*time.Minute)

	access, exp, err := svc.IssueAccessToken("user-abc-123")
	if err != nil {
		t.Fatal(err)
	}
	if access == "" {
		t.Fatal("empty access token")
	}
	if time.Until(exp) < 14*time.Minute {
		t.Fatal("exp too soon")
	}

	claims, err := svc.ParseAccessToken(access)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != "user-abc-123" {
		t.Fatalf("wrong userID: %s", claims.UserID)
	}
	if claims.Subject != "user-abc-123" {
		t.Fatalf("wrong sub: %s", claims.Subject)
	}
}

func TestJWTExpired(t *testing.T) {
	secret := []byte("test-secret-expired")
	svc := auth.New(secret, -1*time.Second)
	tok, _, err := svc.IssueAccessToken("u")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.ParseAccessToken(tok)
	if err == nil {
		t.Fatal("expired token should fail")
	}
}

func TestJWTInvalid(t *testing.T) {
	secret := []byte("test-secret-real")
	svc := auth.New(secret, time.Hour)
	_, err := svc.ParseAccessToken("")
	if err == nil {
		t.Fatal("empty token should fail")
	}
	_, err = svc.ParseAccessToken("not.a.jwt")
	if err == nil {
		t.Fatal("garbage token should fail")
	}
	other := auth.New([]byte("different-secret"), time.Hour)
	tok, _, _ := other.IssueAccessToken("u")
	_, err = svc.ParseAccessToken(tok)
	if err == nil {
		t.Fatal("wrong secret should fail")
	}
}

func TestNormalizeEmail(t *testing.T) {
	cases := []struct{ in, want string }{
		{"alice@test.dev", "alice@test.dev"},
		{"Bob.Example@FOO.COM ", "bob.example@foo.com"},
		{"a+b@x.co", "a+b@x.co"},
		{"", ""},
		{"not-an-email", "not-an-email"},
	}
	for _, c := range cases {
		got := auth.NormalizeEmail(c.in)
		if got != c.want {
			t.Errorf("normalize %q: want %q got %q", c.in, c.want, got)
		}
	}
}

func TestValidateUsername(t *testing.T) {
	valid := []string{"alice", "Bob_Marley", "  cool-name  ", "ab", "a", "\x00name"}
	for _, u := range valid {
		n, err := auth.ValidateUsername(u)
		if err != nil {
			t.Errorf("validate %q: %v", u, err)
		}
		if n == "" {
			t.Errorf("empty result for %q", u)
		}
	}
	if _, err := auth.ValidateUsername(""); err == nil {
		t.Error("empty username should fail")
	}
}

func TestPasswordTruncation(t *testing.T) {
	_, err := auth.HashPassword(strings.Repeat("a", 73))
	if err == nil {
		t.Fatal("expected error for password >72 bytes")
	}
	// 72 bytes should still work.
	hash, err := auth.HashPassword(strings.Repeat("a", 72))
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.VerifyPassword(hash, strings.Repeat("a", 72)); err != nil {
		t.Fatal("verify with 72-byte password failed")
	}
}

func TestUsernameBoundaries(t *testing.T) {
	valid := []string{
		"a",
		strings.Repeat("a", 100),
		"正常用户名",
		"user-name_123",
		"  spaced  ",
	}
	for _, u := range valid {
		n, err := auth.ValidateUsername(u)
		if err != nil {
			t.Errorf("valid username %q should pass: %v", u, err)
		}
		if n == "" {
			t.Errorf("valid username %q returned empty", u)
		}
	}
}

func TestParseAccessToken_WrongSigningMethod(t *testing.T) {
	svc := auth.New([]byte("secret"), time.Hour)
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"uid": "test"})
	tokenStr, _ := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	_, err := svc.ParseAccessToken(tokenStr)
	if err != auth.ErrTokenInvalid {
		t.Fatalf("want ErrTokenInvalid, got %v", err)
	}
}

func TestParseAccessToken_EmptyUserID(t *testing.T) {
	svc := auth.New([]byte("secret"), time.Hour)
	tok, _, err := svc.IssueAccessToken("")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.ParseAccessToken(tok)
	if err != auth.ErrTokenInvalid {
		t.Fatalf("want ErrTokenInvalid, got %v", err)
	}
}

func TestRefreshTokenCycle(t *testing.T) {
	raw1, hash1 := auth.GenerateRefreshToken()
	raw2, hash2 := auth.GenerateRefreshToken()
	if raw1 == raw2 {
		t.Fatal("two refresh tokens should differ")
	}
	if hash1 == hash2 {
		t.Fatal("two hashes should differ even if raw differs (very unlikely to fail)")
	}
	if auth.HashRefreshToken(raw1) != hash1 {
		t.Fatal("hash not deterministic")
	}
}
