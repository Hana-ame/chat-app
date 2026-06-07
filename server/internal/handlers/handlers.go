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

type Server struct {
	Cfg  *config.Config
	DB   *db.DB
	Auth *auth.Service
	Hub  *ws.Hub
}

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
