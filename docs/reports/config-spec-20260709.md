# 配置规范 (Config Spec)

> 原始来源：`server/internal/config/config.go`

---

## 一、配置项汇总

| 配置项 | 环境变量 | 类型 | 默认值 | 说明 |
|---|---|---|---|---|
| `Addr` | `CHAT_ADDR` | `string` | `":8080"` | 服务监听地址 |
| `DBPath` | `CHAT_DB_PATH` | `string` | `"chat.db"` | SQLite 数据库文件路径 |
| `UploadDir` | `CHAT_UPLOAD_DIR` | `string` | `"uploads"` | (Deprecated) 本地上传目录 |
| `BaseURL` | `CHAT_BASE_URL` | `string` | `""` | 服务对外公开的根 URL |
| `JWTSecret` | `CHAT_JWT_SECRET` | `[]byte` | 随机 32 字节 | JWT 签名密钥 |
| `AccessTokenTTL` | `CHAT_ACCESS_TTL` | `Duration` | `30m` | Access Token 有效期 |
| `RefreshTokenTTL` | `CHAT_REFRESH_TTL` | `Duration` | `365d` | Refresh Token 有效期 |
| `MaxUploadBytes` | `CHAT_MAX_UPLOAD` | `int64` | `20MB` | (Deprecated) 最大上传文件大小 |
| `StaticDir` | `CHAT_STATIC_DIR` | `string` | `"../client/dist"` | 静态资源根目录 |

---

## 二、加载逻辑

1. **环境变量优先**：使用 `godotenv` 加载 `.env` 文件，然后通过 `os.Getenv` 读取。
2. **默认值 fallback**：若环境变量为空，则使用代码中定义的默认值。
3. **动态生成**：若 `CHAT_JWT_SECRET` 未提供，则在启动时随机生成 32 字节密钥（重启后失效）。
4. **路径绝对化**：`UploadDir` 在加载时会被转换为绝对路径。
