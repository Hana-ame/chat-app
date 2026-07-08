package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Addr             string
	DBPath           string
	UploadDir        string        // Deprecated: frontend uploads directly to upload.moonchan.xyz. Remove in future version.
	BaseURL          string
	JWTSecret        []byte
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	MaxUploadBytes   int64         // Deprecated: frontend uploads directly to upload.moonchan.xyz. Remove in future version.
	StaticDir        string
	AllowOrigins     []string
}

func getenv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func getenvInt64(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

func getenvDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func Load() *Config {
	_ = godotenv.Load()
	secret := os.Getenv("CHAT_JWT_SECRET")
	if secret == "" {
		secret = randomHex(32)
	}

	dbPath := getenv("CHAT_DB_PATH", "chat.db")
	uploadDir := getenv("CHAT_UPLOAD_DIR", "uploads")
	abs, err := filepath.Abs(uploadDir)
	if err == nil {
		uploadDir = abs
	}

	return &Config{
		Addr:            getenv("CHAT_ADDR", ":8080"),
		DBPath:          dbPath,
		UploadDir:       uploadDir,
		BaseURL:         getenv("CHAT_BASE_URL", ""),
		JWTSecret:       []byte(secret),
		AccessTokenTTL:  getenvDuration("CHAT_ACCESS_TTL", 30*time.Minute),
		RefreshTokenTTL: getenvDuration("CHAT_REFRESH_TTL", 365*24*time.Hour),
		MaxUploadBytes:  getenvInt64("CHAT_MAX_UPLOAD", 20<<20),
		StaticDir:       getenv("CHAT_STATIC_DIR", "../client/dist"),
		AllowOrigins:    []string{"*"},
	}
}
