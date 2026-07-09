# Config 配置规范

> 原始来源：`server/internal/config/config.go`

---

## 一、原始代码

```go
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
	UploadDir        string        // Deprecated
	BaseURL          string
	JWTSecret        []byte
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	MaxUploadBytes   int64         // Deprecated
	StaticDir        string
	AllowOrigins     []string
}

func getenv(key, def string) string {
	v := os.Getenv(key)
	if v == "" { return def }
	return v
}

func getenvInt64(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" { return def }
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil { return def }
	return n
}

func getenvDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" { return def }
	d, err := time.ParseDuration(v)
	if err != nil { return def }
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
	if err == nil { uploadDir = abs }

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
```

---

## 二、配置字段总表

| 字段 | 类型 | 环境变量 | 默认值 | 生成规则 |
|------|------|----------|--------|----------|
| Addr | `string` | `CHAT_ADDR` | `":8080"` | 直接读取 |
| DBPath | `string` | `CHAT_DB_PATH` | `"chat.db"` | 直接读取 |
| UploadDir | `string` | `CHAT_UPLOAD_DIR` | `"uploads"` | filepath.Abs 转换绝对路径 |
| BaseURL | `string` | `CHAT_BASE_URL` | `""` | 直接读取 |
| JWTSecret | `[]byte` | `CHAT_JWT_SECRET` | 随机生成 | `randomHex(32)`（64 hex chars） |
| AccessTokenTTL | `time.Duration` | `CHAT_ACCESS_TTL` | `30m` | `time.ParseDuration` |
| RefreshTokenTTL | `time.Duration` | `CHAT_REFRESH_TTL` | `8760h`（365天） | `time.ParseDuration` |
| MaxUploadBytes | `int64` | `CHAT_MAX_UPLOAD` | `20971520`（20MB） | `strconv.ParseInt` |
| StaticDir | `string` | `CHAT_STATIC_DIR` | `"../client/dist"` | 直接读取 |
| AllowOrigins | `[]string` | — | `["*"]` | 硬编码 |

---

## 三、生成规则

### `Load()`

1. `godotenv.Load()` — 自动加载 `.env` 文件（不会覆盖已有环境变量）
2. `CHAT_JWT_SECRET` 为空时自动生成 32 字节随机 hex 字符串
3. 各字段通过 `getenv`/`getenvInt64`/`getenvDuration` 读取

### 辅助函数

| 函数 | 说明 |
|------|------|
| `getenv(key, def)` | 读取字符串环境变量，不存在返回默认值 |
| `getenvInt64(key, def)` | 读取整数环境变量，解析失败返回默认值 |
| `getenvDuration(key, def)` | 读取时长环境变量（如 `30m`、`24h`），解析失败返回默认值 |
| `randomHex(n)` | 生成 n 字节随机 hex 字符串（用于无 JWT_SECRET 时回退） |

---

## 四、约束汇总

| 约束 | 说明 |
|------|------|
| .env 自动加载 | `godotenv.Load()` 在 `Load()` 开头调用 |
| JWT Secret 回退 | 未设置时自动生成 64 字符 hex（每次启动不同！生产需固定） |
| UploadDir 绝对路径 | 自动 `filepath.Abs` 转换 |
| Duration 格式 | 支持 Go duration 格式（`30m`、`24h`、`8760h`） |
| Deprecated 字段 | `UploadDir`、`MaxUploadBytes` 为前端直传方案保留，未来移除 |