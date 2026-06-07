package handlers

import (
	"errors"
	"net/http"

	"github.com/Hana-ame/chat-app/server/internal/auth"
	"github.com/Hana-ame/chat-app/server/internal/db"
)

type registerReq struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

type sessionResp struct {
	User         any    `json:"user"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func (s *Server) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	email, err := auth.NormalizeEmail(req.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_email", err.Error())
		return
	}
	username, err := auth.ValidateUsername(req.Username)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_username", err.Error())
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, "weak_password", err.Error())
		return
	}
	u, err := s.DB.CreateUser(r.Context(), email, username, hash)
	if err != nil {
		if errors.Is(err, db.ErrConflict) {
			writeError(w, http.StatusConflict, "email_taken", "email already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.issueSession(w, r, u.ID)
}

func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	email, err := auth.NormalizeEmail(req.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_email", err.Error())
		return
	}
	u, hash, err := s.DB.GetUserByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if err := auth.VerifyPassword(hash, req.Password); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}
	s.issueSession(w, r, u.ID)
}

func (s *Server) issueSession(w http.ResponseWriter, r *http.Request, userID string) {
	access, exp, err := s.Auth.IssueAccessToken(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	raw, hash := auth.GenerateRefreshToken()
	if _, err := s.DB.CreateRefreshToken(r.Context(), userID, hash, s.Cfg.RefreshTokenTTL); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	u, err := s.DB.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	_ = exp
	writeJSON(w, http.StatusOK, sessionResp{
		User:         u,
		AccessToken:  access,
		RefreshToken: raw,
		ExpiresIn:    int64(s.Cfg.AccessTokenTTL.Seconds()),
	})
}

func (s *Server) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "refresh_token required")
		return
	}
	hash := auth.HashRefreshToken(req.RefreshToken)
	rt, err := s.DB.FindRefreshToken(r.Context(), hash)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "refresh_invalid", "invalid refresh token")
		return
	}
	if rt.ExpiresAt.Before(timeNow()) {
		_ = s.DB.DeleteRefreshToken(r.Context(), rt.ID)
		writeError(w, http.StatusUnauthorized, "refresh_expired", "refresh token expired")
		return
	}
	if err := s.DB.DeleteRefreshToken(r.Context(), rt.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.issueSession(w, r, rt.UserID)
}

func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing user")
		return
	}
	var req refreshReq
	_ = decodeJSON(r, &req)
	if req.RefreshToken != "" {
		hash := auth.HashRefreshToken(req.RefreshToken)
		if rt, err := s.DB.FindRefreshToken(r.Context(), hash); err == nil && rt.UserID == u.ID {
			_ = s.DB.DeleteRefreshToken(r.Context(), rt.ID)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) Me(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	writeJSON(w, http.StatusOK, u)
}
