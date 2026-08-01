package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) PinChatList(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	if err := s.Services.Chat.SetPinned(r.Context(), id, u.ID, true); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "pinned": true})
}

func (s *Server) UnpinChatList(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	if err := s.Services.Chat.SetPinned(r.Context(), id, u.ID, false); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "pinned": false})
}

type updateNotifyReq struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) UpdateChatNotify(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	var req updateNotifyReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := s.Services.Chat.SetChatNotifyEnabled(r.Context(), id, u.ID, req.Enabled); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "enabled": req.Enabled})
}
