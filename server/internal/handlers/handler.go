package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/auth"
	"github.com/Hana-ame/chat-app/server/internal/config"
	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
	"github.com/Hana-ame/chat-app/server/internal/service"
	"github.com/Hana-ame/chat-app/server/internal/ws"
	"github.com/go-chi/chi/v5"
)

type ctxKey string

const (
	ctxKeyUser  ctxKey = "user"
	ctxKeyToken ctxKey = "token"
)

// Server is the HTTP handler container holding shared dependencies.
type Server struct {
	Cfg          *config.Config
	DB           *db.DB
	Auth         *auth.Service
	Hub          *ws.Hub
	Version      string
	Services     *service.Service
	refreshMu       sync.Mutex
	loginLimiter    *loginRateLimiter
	registerLimiter *registerLimiter
	aiHandler    *AIHandler
}

// New creates a new Server.
func New(cfg *config.Config, database *db.DB, authSvc *auth.Service, hub *ws.Hub) *Server {
	s := &Server{
		Cfg:          cfg,
		DB:           database,
		Auth:         authSvc,
		Hub:          hub,
		Services:     service.New(database, hub),
		loginLimiter:    newLoginRateLimiter(5, 1*time.Hour),
		registerLimiter: newRegisterLimiter(100, 24*time.Hour),
	}
	if len(cfg.AISources) > 0 {
		s.aiHandler = NewAIHandler(cfg.AISources)
	}
	return s
}

func userFrom(ctx context.Context) *models.User {
	v, _ := ctx.Value(ctxKeyUser).(*models.User)
	return v
}

func tokenFrom(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyToken).(string)
	return v
}

func (s *Server) VersionHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": s.Version})
}

func mapServiceError(err error) (int, string) {
	if err == nil {
		return 0, ""
	}
	if errors.Is(err, service.ErrForbidden) {
		return http.StatusForbidden, "forbidden"
	}
	if errors.Is(err, service.ErrNotFound) {
		return http.StatusNotFound, "not_found"
	}
	if errors.Is(err, service.ErrInvalidInput) {
		return http.StatusBadRequest, "bad_request"
	}
	if errors.Is(err, service.ErrConflict) {
		return http.StatusConflict, "conflict"
	}
	if errors.Is(err, service.ErrContentTooLong) {
		return http.StatusRequestEntityTooLarge, "content_too_long"
	}
	return http.StatusInternalServerError, "internal"
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		logutil.Error("writeJSON encode error: %v", err)
	}
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
	// Deprecated: URL query token leaks via server logs, browser history, and Referer headers.
	// Frontend should use Authorization header or cookie. This path is kept for backward compatibility
	// with existing SSE clients and will be removed in a future version.
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
			if c, err := r.Cookie("access_token"); err == nil {
				tok = c.Value
			}
		}
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

// trackLastActive updates chat_members.last_active_at on every request to a chat endpoint.
func (s *Server) trackLastActive(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chatID := chi.URLParam(r, "chatID")
		u := userFrom(r.Context())
		if chatID != "" && u != nil {
			id := u.ID
			go func() {
				if err := s.DB.UpdateLastActiveAt(r.Context(), chatID, id); err != nil {
					logutil.Error("trackLastActive error: %v", err)
				}
			}()
		}
		next.ServeHTTP(w, r)
	})
}
