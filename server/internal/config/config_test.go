// Package config_test 覆盖配置加载:默认值、环境变量覆盖、非法值报错、
// JWT secret 自动生成。
//
// 运行方式: cd server && go test ./internal/config/
// 说明:通过 t.Setenv 控制环境变量,不触碰真实 .env。
package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	for _, k := range []string{
		"CHAT_ADDR", "CHAT_DB_PATH", "CHAT_UPLOAD_DIR", "CHAT_BASE_URL",
		"CHAT_JWT_SECRET", "CHAT_ACCESS_TTL", "CHAT_REFRESH_TTL",
		"CHAT_MAX_UPLOAD", "CHAT_STATIC_DIR", "CHAT_CSP_CONNECT_SRC",
	} {
		os.Unsetenv(k)
	}

	cfg := config.Load()
	if cfg.Addr != ":8080" {
		t.Fatalf("Addr: want :8080, got %s", cfg.Addr)
	}
	if cfg.DBPath != "chat.db" {
		t.Fatalf("DBPath: want chat.db, got %s", cfg.DBPath)
	}
	if len(cfg.JWTSecret) == 0 {
		t.Fatal("JWTSecret should not be empty")
	}
	if cfg.AccessTokenTTL != 30*time.Minute {
		t.Fatalf("AccessTokenTTL: want 30m, got %v", cfg.AccessTokenTTL)
	}
	if cfg.RefreshTokenTTL != 365*24*time.Hour {
		t.Fatalf("RefreshTokenTTL: want 8760h, got %v", cfg.RefreshTokenTTL)
	}
	if cfg.MaxUploadBytes != 20<<20 {
		t.Fatalf("MaxUploadBytes: want %d, got %d", 20<<20, cfg.MaxUploadBytes)
	}
	if len(cfg.AllowOrigins) != 1 || cfg.AllowOrigins[0] != "*" {
		t.Fatal("AllowOrigins should be [*]")
	}
	if cfg.StaticDir != "../client/dist" {
		t.Fatalf("StaticDir: want ../client/dist, got %s", cfg.StaticDir)
	}
}

func TestLoadCustomEnv(t *testing.T) {
	t.Setenv("CHAT_ADDR", ":3000")
	t.Setenv("CHAT_DB_PATH", "/tmp/test.db")
	t.Setenv("CHAT_BASE_URL", "https://example.com")
	t.Setenv("CHAT_JWT_SECRET", "my-secret-key")
	t.Setenv("CHAT_ACCESS_TTL", "15m")
	t.Setenv("CHAT_REFRESH_TTL", "72h")
	t.Setenv("CHAT_MAX_UPLOAD", "1048576")
	t.Setenv("CHAT_STATIC_DIR", "/tmp/static")
	t.Setenv("CHAT_CSP_CONNECT_SRC", "'self' https://example.com")

	cfg := config.Load()
	if cfg.Addr != ":3000" {
		t.Fatalf("Addr: want :3000, got %s", cfg.Addr)
	}
	if cfg.DBPath != "/tmp/test.db" {
		t.Fatalf("DBPath: want /tmp/test.db, got %s", cfg.DBPath)
	}
	if string(cfg.JWTSecret) != "my-secret-key" {
		t.Fatalf("JWTSecret: want my-secret-key, got %s", string(cfg.JWTSecret))
	}
	if cfg.AccessTokenTTL != 15*time.Minute {
		t.Fatalf("AccessTokenTTL: want 15m, got %v", cfg.AccessTokenTTL)
	}
	if cfg.RefreshTokenTTL != 72*time.Hour {
		t.Fatalf("RefreshTokenTTL: want 72h, got %v", cfg.RefreshTokenTTL)
	}
	if cfg.MaxUploadBytes != 1048576 {
		t.Fatalf("MaxUploadBytes: want 1048576, got %d", cfg.MaxUploadBytes)
	}
	if cfg.StaticDir != "/tmp/static" {
		t.Fatalf("StaticDir: want /tmp/static, got %s", cfg.StaticDir)
	}
	if cfg.CSPConnectSrc != "'self' https://example.com" {
		t.Fatalf("CSPConnectSrc: got %s", cfg.CSPConnectSrc)
	}
}

func TestLoadInvalidAccessTTL(t *testing.T) {
	t.Setenv("CHAT_ACCESS_TTL", "invalid")
	cfg := config.Load()
	if cfg.AccessTokenTTL != 30*time.Minute {
		t.Fatalf("invalid TTL should fall back to default, got %v", cfg.AccessTokenTTL)
	}
}

func TestLoadInvalidMaxUpload(t *testing.T) {
	t.Setenv("CHAT_MAX_UPLOAD", "not-a-number")
	cfg := config.Load()
	if cfg.MaxUploadBytes != 20<<20 {
		t.Fatalf("invalid MaxUpload should fall back to default, got %d", cfg.MaxUploadBytes)
	}
}

func TestLoadNegativeMaxUpload(t *testing.T) {
	t.Setenv("CHAT_MAX_UPLOAD", "-1")
	cfg := config.Load()
	if cfg.MaxUploadBytes != -1 {
		t.Fatalf("want -1, got %d", cfg.MaxUploadBytes)
	}
}

func TestJWTSecretRandomGeneration(t *testing.T) {
	os.Unsetenv("CHAT_JWT_SECRET")
	cfg1 := config.Load()
	os.Unsetenv("CHAT_JWT_SECRET")
	cfg2 := config.Load()
	if string(cfg1.JWTSecret) == string(cfg2.JWTSecret) {
		t.Fatal("random secrets should differ")
	}
	if len(cfg1.JWTSecret) != 64 {
		t.Fatalf("hex-encoded 32 bytes should be 64 chars, got %d", len(cfg1.JWTSecret))
	}
}
