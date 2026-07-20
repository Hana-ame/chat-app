package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/ai"
	"github.com/Hana-ame/chat-app/server/internal/logutil"
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
	CSPConnectSrc    string
	AISources        []ai.SourceConfig
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

	var aiSources []ai.SourceConfig
	if s := os.Getenv("CHAT_AI_SOURCES"); s != "" {
		if err := json.Unmarshal([]byte(s), &aiSources); err != nil {
			logutil.Warn("CHAT_AI_SOURCES invalid JSON: %v", err)
		}
	}
	if len(aiSources) == 0 {
		if key := os.Getenv("CHAT_AI_KEY"); key != "" {
			aiSources = []ai.SourceConfig{{
				Name:    "default",
				Key:     key,
				BaseURL: getenv("CHAT_AI_BASE_URL", "https://api.siliconflow.cn/v1"),
				Model:   getenv("CHAT_AI_MODEL", "deepseek-ai/Deepseek-V4-Flash"),
			}}
		}
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
		StaticDir:       getenv("CHAT_STATIC_DIR", "../client/dist"),
		AllowOrigins:    []string{"*"},
		CSPConnectSrc:   getenv("CHAT_CSP_CONNECT_SRC", "'self' wss://wsl-8080.moonchan.xyz https://upload.moonchan.xyz"),
		AISources:       aiSources,
	}
	logutil.Debug("config loaded: addr=%s db=%s upload_dir=%s static_dir=%s base_url=%s",
		cfg.Addr, cfg.DBPath, cfg.UploadDir, cfg.StaticDir, cfg.BaseURL)
	return cfg
}
