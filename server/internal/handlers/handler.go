package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Hana-ame/chat-app/server/internal/auth"
	"github.com/Hana-ame/chat-app/server/internal/config"
	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/models"
	"github.com/Hana-ame/chat-app/server/internal/ws"
)

type ctxKey string

const (
	ctxKeyUser  ctxKey = "user"
	ctxKeyToken ctxKey = "token"
)

// Server is the HTTP handler container holding shared dependencies.
type Server struct {
	Cfg  *config.Config
	DB   *db.DB
	Auth *auth.Service
	Hub  *ws.Hub
}

// New creates a new Server.
func New(cfg *config.Config, database *db.DB, authSvc *auth.Service, hub *ws.Hub) *Server {
	return &Server{Cfg: cfg, DB: database, Auth: authSvc, Hub: hub}
}

func userFrom(ctx context.Context) *models.User {
	v, _ := ctx.Value(ctxKeyUser).(*models.User)
	return v
}

func tokenFrom(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyToken).(string)
	return v
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": code, "message": message})
}

func decodeJSON(r *http.Request, into interface{}) error {
	if r.Body == nil {
		return errors.New("empty body")
	}
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(into)
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	if t := r.URL.Query().Get("access_token"); t != "" {
		return t
	}
	return ""
}

// authMiddleware authenticates requests via Bearer JWT token.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := bearerToken(r)
		if tok == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing token")
			return
		}
		claims, err := s.Auth.ParseAccessToken(tok)
		if err != nil {
			if errors.Is(err, auth.ErrTokenExpired) {
				writeError(w, http.StatusUnauthorized, "token_expired", "access token expired")
				return
			}
			writeError(w, http.StatusUnauthorized, "token_invalid", "access token invalid")
			return
		}
		u, err := s.DB.GetUserByID(r.Context(), claims.UserID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(w, http.StatusUnauthorized, "user_not_found", "user does not exist")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeyUser, u)
		ctx = context.WithValue(ctx, ctxKeyToken, tok)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
