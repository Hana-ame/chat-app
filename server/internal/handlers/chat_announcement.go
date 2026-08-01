package handlers

import (
	"net/http"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/go-chi/chi/v5"
)

type pinContentReq struct {
	Content string `json:"content"`
}

func (s *Server) PinChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	var req pinContentReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := s.Services.Chat.SetAnnouncement(r.Context(), id, u.ID, req.Content); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	chat, err := s.Services.Chat.GetByID(r.Context(), id, u.ID)
	if err != nil {
		logutil.Error("pin chat: GetByID failed after SetAnnouncement: %v", err)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pinned_message": chat.PinnedMessage, "pinned_updated_at": chat.PinnedUpdatedAt})
}

func (s *Server) DeletePinnedChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	if err := s.Services.Chat.ClearAnnouncement(r.Context(), id, u.ID); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) MarkPinnedRead(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	if err := s.Services.Chat.MarkAnnouncementRead(r.Context(), id, u.ID); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
