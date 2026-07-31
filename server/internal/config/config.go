package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/joho/godotenv"
)

type Config struct {
	Addr            string
	DBPath          string
	BaseURL         string
	JWTSecret       []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	StaticDir       string
	AllowOrigins    []string
	CSPConnectSrc   string
	UploadDir       string
	MaxUploadBytes  int64
	UploadSalt      string

	MaxMessageContentLength int
	WSMaxMessageSize        int64
	APITimeout              time.Duration
	UploadTimeout           time.Duration
	ReadTimeout             time.Duration
	ReadHeaderTimeout       time.Duration

	// AIAllowPrivateIPs permits AI stream endpoints that resolve to private,
	// loopback, or link-local addresses (e.g. a local ollama). Off by default
	// to prevent SSRF from a public deployment.
	AIAllowPrivateIPs bool
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

func getenvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
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
		logutil.Warn("CHAT_JWT_SECRET not set — generated random key (all existing sessions invalidated). Set CHAT_JWT_SECRET in production.")
	}

	dbPath := getenv("CHAT_DB_PATH", "chat.db")
	uploadDir := getenv("CHAT_UPLOAD_DIR", "uploads")
	abs, err := filepath.Abs(uploadDir)
	if err == nil {
		uploadDir = abs
	}

	uploadSalt := getenv("CHAT_UPLOAD_SALT", "")
	if uploadSalt == "" {
		uploadSalt = randomHex(16)
	}

	cfg := &Config{
		Addr:            getenv("CHAT_ADDR", ":8080"),
		DBPath:          dbPath,
		UploadDir:       uploadDir,
		BaseURL:         getenv("CHAT_BASE_URL", ""),
		JWTSecret:       []byte(secret),
		AccessTokenTTL:  getenvDuration("CHAT_ACCESS_TTL", 30*time.Minute),
		RefreshTokenTTL: getenvDuration("CHAT_REFRESH_TTL", 365*24*time.Hour),
		MaxUploadBytes:  getenvInt64("CHAT_MAX_UPLOAD", 20<<20),
		UploadSalt:      uploadSalt,

		StaticDir:     getenv("CHAT_STATIC_DIR", "../client/dist"),
		AllowOrigins:  []string{"*"},
		CSPConnectSrc: getenv("CHAT_CSP_CONNECT_SRC", "'self' wss://wsl-8080.moonchan.xyz"),

		MaxMessageContentLength: int(getenvInt64("CHAT_MAX_MESSAGE_LENGTH", 4000)),
		WSMaxMessageSize:        getenvInt64("CHAT_WS_MAX_MSG_SIZE", 1<<16),
		APITimeout:              getenvDuration("CHAT_API_TIMEOUT", 10*time.Second),
		UploadTimeout:           getenvDuration("CHAT_UPLOAD_TIMEOUT", 5*time.Minute),
		ReadTimeout:             getenvDuration("CHAT_READ_TIMEOUT", 10*time.Minute),
		ReadHeaderTimeout:       getenvDuration("CHAT_READ_HEADER_TIMEOUT", 10*time.Second),

		AIAllowPrivateIPs: getenvBool("CHAT_AI_ALLOW_PRIVATE", false),
	}
	logutil.Info("config loaded: addr=%s db=%s upload_dir=%s static_dir=%s base_url=%s",
		cfg.Addr, cfg.DBPath, cfg.UploadDir, cfg.StaticDir, cfg.BaseURL)
	return cfg
}
