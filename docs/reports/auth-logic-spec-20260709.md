# Auth 认证逻辑规范

> 原始来源：`server/internal/auth/auth.go`

---

## 一、原始代码

```go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenExpired       = errors.New("token expired")
	ErrTokenInvalid       = errors.New("token invalid")
)

type Service struct {
	jwtSecret []byte
	accessTTL time.Duration
}

func New(secret []byte, accessTTL time.Duration) *Service {
	return &Service{jwtSecret: secret, accessTTL: accessTTL}
}

type AccessClaims struct {
	UserID string `json:"uid"`
	jwt.RegisteredClaims
}

func (s *Service) IssueAccessToken(userID string) (string, time.Time, error) {
	exp := time.Now().UTC().Add(s.accessTTL)
	claims := AccessClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			Subject:   userID,
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(s.jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

func (s *Service) ParseAccessToken(tokenStr string) (*AccessClaims, error) {
	if tokenStr == "" {
		return nil, ErrTokenInvalid
	}
	claims := &AccessClaims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrTokenInvalid
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}
	if !tok.Valid {
		return nil, ErrTokenInvalid
	}
	if claims.UserID == "" {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}

func GenerateRefreshToken() (raw, hash string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	raw = base64.RawURLEncoding.EncodeToString(b)
	hash = HashRefreshToken(raw)
	return
}

func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func HashPassword(password string) (string, error) {
	if len(password) > 72 {
		password = password[:72]
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func VerifyPassword(hash, password string) error {
	if len(password) > 72 {
		password = password[:72]
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func ValidateUsername(username string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return "", errors.New("username required")
	}
	return username, nil
}
```

---

## 二、字段总表

### Service

| 字段 | 类型 | 说明 |
|------|------|------|
| jwtSecret | `[]byte` | HMAC-SHA256 签名密钥 |
| accessTTL | `time.Duration` | Access Token 有效期 |

### AccessClaims

| 字段 | 类型 | JSON | 说明 |
|------|------|------|------|
| UserID | `string` | `"uid"` | 用户 UUID |
| RegisteredClaims | `jwt.RegisteredClaims` | — | 标准 JWT claims（ExpiresAt、IssuedAt、Subject） |

---

## 三、函数总表

| 函数 | 签名 | 说明 |
|------|------|------|
| `New` | `(secret, accessTTL) *Service` | 创建认证服务实例 |
| `IssueAccessToken` | `(userID) (string, time.Time, error)` | 签发 JWT，返回 token + 过期时间 |
| `ParseAccessToken` | `(tokenStr) (*AccessClaims, error)` | 解析 JWT，校验签名和过期 |
| `GenerateRefreshToken` | `() (raw, hash string)` | 生成随机 refresh token（32 字节 base64）|
| `HashRefreshToken` | `(raw) string` | SHA256 hash refresh token |
| `HashPassword` | `(password) (string, error)` | bcrypt 哈希密码（截断至 72 字节）|
| `VerifyPassword` | `(hash, password) error` | bcrypt 验证密码 |
| `NormalizeEmail` | `(email) string` | 转小写 + trim |
| `ValidateUsername` | `(username) (string, error)` | trim + 非空检查 |

---

## 四、依赖链

### JWT 签发流程

```
IssueAccessToken(userID)
  ├─ time.Now().UTC().Add(accessTTL) → exp
  ├─ jwt.NewWithClaims(HS256, claims{UserID, ExpiresAt, IssuedAt, Subject})
  ├─ tok.SignedString(jwtSecret)
  └─ return signed, exp, nil
```

### JWT 解析流程

```
ParseAccessToken(tokenStr)
  ├─ tokenStr == "" → ErrTokenInvalid
  ├─ jwt.ParseWithClaims(tokenStr, claims, keyFunc)
  │   ├─ 签名算法不是 HMAC → ErrTokenInvalid
  │   └─ 返回 jwtSecret
  ├─ errors.Is(err, jwt.ErrTokenExpired) → ErrTokenExpired
  ├─ err != nil → ErrTokenInvalid
  ├─ !tok.Valid → ErrTokenInvalid
  ├─ claims.UserID == "" → ErrTokenInvalid
  └─ return claims, nil
```

### Refresh Token 流程

```
GenerateRefreshToken()
  ├─ rand.Read(32 bytes)
  ├─ base64.RawURLEncoding → raw
  ├─ HashRefreshToken(raw) → sha256 → hex → hash
  └─ return raw, hash
```

### 密码流程

```
HashPassword(password)
  └─ len > 72 → truncate to 72
  └─ bcrypt.GenerateFromPassword(DefaultCost) → hash

VerifyPassword(hash, password)
  └─ len > 72 → truncate to 72
  ├─ bcrypt.CompareHashAndPassword 失败 → ErrInvalidCredentials
  └─ return nil
```

---

## 五、条件分支

| 条件 | 返回 |
|------|------|
| `ParseAccessToken("")` | `ErrTokenInvalid` |
| 签名算法不是 HMAC | `ErrTokenInvalid` |
| `jwt.ErrTokenExpired` | `ErrTokenExpired` |
| 其他 JWT 错误 | `ErrTokenInvalid` |
| `!tok.Valid` | `ErrTokenInvalid` |
| `claims.UserID == ""` | `ErrTokenInvalid` |
| `HashPassword(password)` — `len(password) > 72` | 截断至 72 字节后再 bcrypt |
| `VerifyPassword` — `bcrypt.CompareHashAndPassword` 失败 | `ErrInvalidCredentials` |
| `ValidateUsername("")` | `errors.New("username required")` |

---

## 六、约束汇总

| 约束 | 说明 |
|------|------|
| JWT 签名算法 | 仅支持 `HS256`（HMAC-SHA256） |
| Password 最大长度 | bcrypt 限制 72 字节，超长时截断 |
| Refresh Token 存储 | 仅存 SHA256 hash（raw 只返回客户端） |
| Refresh Token 长度 | 32 字节随机 → base64 编码 ≈ 43 字符 |
| Email 标准化 | `strings.ToLower` + `strings.TrimSpace` |
| Username 校验 | 仅 `strings.TrimSpace` + 非空检查 |