package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/auth"
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
	if _, err := auth.HashPassword("short"); err == nil {
		t.Fatal("short password should fail")
	}
	long := ""
	for i := 0; i < 80; i++ {
		long += "a"
	}
	if len(long) != 80 {
		t.Fatal("whoops")
	}
	_, err = auth.HashPassword(long)
	if err != nil {
		t.Fatalf("long password: %v", err)
	}
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
	valid := []string{"alice@test.dev", "Bob.Example@FOO.COM ", "a+b@x.co"}
	for _, e := range valid {
		n, err := auth.NormalizeEmail(e)
		if err != nil {
			t.Errorf("normalize %q: %v", e, err)
		}
		if n != n {
			t.Errorf("not case-normalized: %q -> %q", e, n)
		}
	}
	invalid := []string{"", "not-an-email", "@missing", "spaces in@addr.com"}
	for _, e := range invalid {
		_, err := auth.NormalizeEmail(e)
		if err == nil {
			t.Errorf("should be invalid: %q", e)
		}
	}
}

func TestValidateUsername(t *testing.T) {
	valid := []string{"alice", "Bob_Marley", "  cool-name  ", "ab"}
	for _, u := range valid {
		n, err := auth.ValidateUsername(u)
		if err != nil {
			t.Errorf("validate %q: %v", u, err)
		}
		if n == "" {
			t.Errorf("empty result for %q", u)
		}
	}
	invalid := []string{"", "a", "\x00name", string(rune(0x7f)), ""}
	for _, u := range invalid {
		_, err := auth.ValidateUsername(u)
		if err == nil && u != "" {
			continue
		}
		if err == nil {
			t.Errorf("should be invalid: %q", u)
		}
	}
}

func TestPasswordTruncation(t *testing.T) {
	// bcrypt only considers the first 72 bytes.
	// HashPassword truncates to 72 before hashing.
	long := strings.Repeat("a", 100)
	hash, err := auth.HashPassword(long)
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.VerifyPassword(hash, long); err != nil {
		t.Fatal("verify with original long password failed")
	}
	if err := auth.VerifyPassword(hash, long[:72]); err != nil {
		t.Fatal("verify with truncated password failed")
	}
}

func TestUsernameBoundaries(t *testing.T) {
	valid := []string{
		strings.Repeat("a", 32),
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

	invalid := []string{
		"",
		" ",
		"a",
		strings.Repeat("a", 33),
		"\x00null",
		string(rune(0x7f)),
		"new\nline",
	}
	for _, u := range invalid {
		_, err := auth.ValidateUsername(u)
		if err == nil {
			t.Errorf("invalid username %q should fail", u)
		}
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