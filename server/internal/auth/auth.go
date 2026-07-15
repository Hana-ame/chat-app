package auth

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/golang-jwt/jwt/v5"
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
	// No lock needed: IssueAccessToken is stateless (all inputs/receiver are read-only).
	// The empty lock/unlock stubs were removed; kept as comment for context.
	signed, err := tok.SignedString(s.jwtSecret)
	if err != nil {
		logutil.Error("issue access token: %v", err)
		return "", time.Time{}, err
	}
	logutil.Debug("issued access token for user %s (expires %s)", userID, exp.Format(time.RFC3339))
	return signed, exp, nil
}

// lock/unlock were empty stubs that gave a false sense of concurrency safety.
// IssueAccessToken is stateless — no lock needed. Removed; kept as comment for context.
// func (s *Service) lock()   {}
// func (s *Service) unlock() {}

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
			logutil.Debug("token expired")
			return nil, ErrTokenExpired
		}
		logutil.Warn("token parse failed: %v", err)
		return nil, ErrTokenInvalid
	}
	if !tok.Valid {
		logutil.Warn("token invalid (valid=false)")
		return nil, ErrTokenInvalid
	}
	if claims.UserID == "" {
		logutil.Warn("token missing user_id claim")
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
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	saltHex := hex.EncodeToString(salt)
	sum := md5.Sum([]byte(saltHex + password))
	return saltHex + "$" + hex.EncodeToString(sum[:]), nil
}

func VerifyPassword(stored, password string) error {
	parts := strings.SplitN(stored, "$", 2)
	if len(parts) != 2 {
		return ErrInvalidCredentials
	}
	saltHex, hashHex := parts[0], parts[1]
	sum := md5.Sum([]byte(saltHex + password))
	if hex.EncodeToString(sum[:]) != hashHex {
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
		return "", fmt.Errorf("username cannot be empty")
	}
	return username, nil
}
