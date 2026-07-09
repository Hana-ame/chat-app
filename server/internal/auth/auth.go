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
	s.lock()
	defer s.unlock()
	signed, err := tok.SignedString(s.jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

func (s *Service) lock()   {}
func (s *Service) unlock() {}

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
	return username, nil
}
