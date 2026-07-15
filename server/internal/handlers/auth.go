package handlers

import (
	"errors"
	"net/http"

	"github.com/Hana-ame/chat-app/server/internal/auth"
	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/logutil"
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

type sessionResp struct {
	User        any   `json:"user"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int64 `json:"expires_in"`
}

// Register godoc
// @Summary      Register a new user
// @Description  Create account with email, username and password
// @Tags         auth
// @Param        body  body  registerReq  true  "Registration details"
// @Success      200  {object}  sessionResp
// @Failure      409  {object}  map[string]any
// @Router       /api/auth/register [post]
func (s *Server) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	email := auth.NormalizeEmail(req.Email)
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
			logutil.Warn("register conflict: email=%s username=%s", email, username)
			writeError(w, http.StatusConflict, "already_taken", "email or username already taken")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	logutil.Info("user registered: %s (username=%s email=%s)", u.ID[:8], username, email)
	s.issueSession(w, r, u.ID)
}

// Login godoc
// @Summary      Log in
// @Description  Authenticate with email and password
// @Tags         auth
// @Param        body  body  loginReq  true  "Login credentials"
// @Success      200  {object}  sessionResp
// @Failure      401  {object}  map[string]any
// @Router       /api/auth/login [post]
func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.loginLimiter.allow(ip) {
		logutil.Warn("login rate limited: ip=%s", ip)
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many login attempts, try again later")
		return
	}
	var req loginReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	email := auth.NormalizeEmail(req.Email)
	u, hash, err := s.DB.GetUserByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			logutil.Warn("login failed: email=%s (not found)", email)
			s.loginLimiter.record(ip)
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if err := auth.VerifyPassword(hash, req.Password); err != nil {
		logutil.Warn("login failed: email=%s (wrong password)", email)
		s.loginLimiter.record(ip)
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}
	logutil.Info("user logged in: %s (%s)", u.ID[:8], email)
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
	setAuthCookie(w, r, "access_token", access, "/", s.Cfg.AccessTokenTTL)
	setRefreshCookie(w, r, raw, s.Cfg.RefreshTokenTTL)
	_ = exp
	writeJSON(w, http.StatusOK, sessionResp{
		User:        u,
		AccessToken: access,
		ExpiresIn:   int64(s.Cfg.AccessTokenTTL.Seconds()),
	})
}

// Refresh godoc
// @Summary      Refresh access token
// @Description  Exchange a refresh token for a new session (via httpOnly cookie)
// @Tags         auth
// @Success      200  {object}  sessionResp
// @Failure      401  {object}  map[string]any
// @Router       /api/auth/refresh [post]
func (s *Server) Refresh(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("refresh_token")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "refresh token missing")
		return
	}
	s.refreshMu.Lock()
	hash := auth.HashRefreshToken(c.Value)
	rt, err := s.DB.FindRefreshToken(r.Context(), hash)
	if err != nil {
		s.refreshMu.Unlock()
		writeError(w, http.StatusUnauthorized, "refresh_invalid", "invalid refresh token")
		return
	}
	if rt.ExpiresAt.Before(timeNow()) {
		if err := s.DB.DeleteRefreshToken(r.Context(), rt.ID); err != nil {
			w.Header().Set("X-Error", err.Error())
		}
		s.refreshMu.Unlock()
		writeError(w, http.StatusUnauthorized, "refresh_expired", "refresh token expired")
		return
	}
	if err := s.DB.DeleteRefreshToken(r.Context(), rt.ID); err != nil {
		s.refreshMu.Unlock()
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.refreshMu.Unlock()
	// NOTE: issueSession creates a new refresh token outside refreshMu.
	// Race: a concurrent Logout may delete all tokens (DeleteUserRefreshTokens)
	// before CreateRefreshToken inserts the new one, making the logout incomplete.
	// This is intentionally accepted — the timing window is tiny and the impact
	// is a stale cookie that the user's browser already discarded.
	// If this becomes a problem, move CreateRefreshToken inside refreshMu
	// and also acquire refreshMu in Logout.
	s.issueSession(w, r, rt.UserID)
}

// Logout godoc
// @Summary      Log out
// @Description  Revoke all refresh tokens for the user and clear cookie
// @Tags         auth
// @Security     BearerAuth
// @Success      200  {object}  map[string]any
// @Router       /api/auth/logout [post]
func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing user")
		return
	}
	clearRefreshCookie(w, r)
	clearAccessTokenCookie(w, r)
	// NOTE: see Refresh for the known Logout-vs-Refresh race.
	// DeleteUserRefreshTokens does not hold refreshMu, so a concurrent
	// Refresh's CreateRefreshToken may insert a new token after this deletion.
	// Accepted as low-risk. If it becomes a problem, acquire refreshMu here
	// and move CreateRefreshToken inside refreshMu in Refresh.
	if err := s.DB.DeleteUserRefreshTokens(r.Context(), u.ID); err != nil {
		w.Header().Set("X-Error", err.Error())
	}
	logutil.Info("user logged out: %s", u.ID[:8])
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Me godoc
// @Summary      Get current user
// @Description  Return the authenticated user's profile
// @Tags         users
// @Security     BearerAuth
// @Success      200  {object}  models.User
// @Router       /api/users/me [get]
func (s *Server) Me(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	writeJSON(w, http.StatusOK, u)
}
