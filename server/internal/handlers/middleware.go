package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/Hana-ame/chat-app/server/internal/auth"
	"github.com/Hana-ame/chat-app/server/internal/db"
)

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
