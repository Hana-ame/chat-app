package handlers

import (
	"errors"
	"net/http"

	"github.com/Hana-ame/chat-app/server/internal/auth"
	"github.com/Hana-ame/chat-app/server/internal/db"
)

type updateProfileReq struct {
	Username    string `json:"username"`
	AvatarColor string `json:"avatar_color"`
}

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
	if req.Username == "" {
		req.Username = u.Username
	}
	name, err := auth.ValidateUsername(req.Username)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_username", err.Error())
		return
	}
	if req.AvatarColor == "" {
		req.AvatarColor = u.AvatarColor
	}
	updated, err := s.DB.UpdateUserProfile(r.Context(), u.ID, name, req.AvatarColor)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "user disappeared")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.BroadcastUserUpdate(updated)
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) SearchUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if len(q) < 1 {
		writeJSON(w, http.StatusOK, map[string]any{"users": []any{}})
		return
	}
	users, err := s.DB.SearchUsers(r.Context(), q, 20)
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
