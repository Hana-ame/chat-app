package handlers

import (
	"net/http"

	"github.com/Hana-ame/chat-app/server/internal/auth"
	"github.com/Hana-ame/chat-app/server/internal/logutil"
)

type updateProfileReq struct {
	Username      string   `json:"username"`
	AvatarColor   string   `json:"avatar_color"`
	AvatarURL     string   `json:"avatar_url"`
	NotifyBlocked []string `json:"notify_blocked,omitempty"`
}

// UpdateMe godoc
// @Summary      Update user profile
// @Description  Update username, avatar color, or avatar URL
// @Tags         users
// @Security     BearerAuth
// @Param        body  body  updateProfileReq  true  "Profile fields to update"
// @Success      200  {object}  models.User
// @Router       /api/users/me [patch]
func (s *Server) UpdateMe(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	var req updateProfileReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var name string
	var err error
	if req.Username != "" {
		name, err = auth.ValidateUsername(req.Username)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_username", err.Error())
			return
		}
	} else {
		name = u.Username
	}
	if req.AvatarColor == "" {
		req.AvatarColor = u.AvatarColor
	}
	updated, err := s.Services.User.UpdateProfile(r.Context(), u.ID, name, req.AvatarColor, req.AvatarURL)
	if err != nil {
		status, code := mapServiceError(err)
		if status == http.StatusNotFound {
			writeError(w, status, "not_found", "user disappeared")
			return
		}
		if status == http.StatusConflict {
			logutil.Warn("username taken: %s", name)
			writeError(w, status, "username_taken", "username already taken")
			return
		}
		writeError(w, status, code, err.Error())
		return
	}
	if req.NotifyBlocked != nil {
		updated, err = s.Services.User.UpdateNotifyBlocked(r.Context(), u.ID, req.NotifyBlocked)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
	}
	logutil.Info("profile updated: user=%s username=%s", logutil.SafeID(u.ID), name)
	s.Services.BroadcastUserUpdate(updated)
	writeJSON(w, http.StatusOK, updated)
}

// SearchUsers godoc
// @Summary      Search users
// @Description  Search users by username or email prefix (excludes self)
// @Tags         users
// @Security     BearerAuth
// @Param        q   query  string  true  "Search query (min 1 char)"
// @Success      200  {object}  map[string]any
// @Router       /api/users [get]
func (s *Server) SearchUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if len(q) < 1 {
		writeJSON(w, http.StatusOK, map[string]any{"users": []any{}})
		return
	}
	users, err := s.Services.User.Search(r.Context(), q, 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	me := userFrom(r.Context())
	filtered := users[:0]
	for _, u := range users {
		if me != nil && u.ID == me.ID {
			continue
		}
		filtered = append(filtered, u)
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": filtered})
}
