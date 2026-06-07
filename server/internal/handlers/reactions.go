package handlers

import (
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
)

func (s *Server) AddReaction(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chatID := chi.URLParam(r, "chatID")
	msgID := chi.URLParam(r, "messageID")
	emojiRaw := chi.URLParam(r, "emoji")
	emoji, err := url.PathUnescape(emojiRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "bad emoji encoding")
		return
	}
	ok, _ := s.DB.IsChatMember(r.Context(), chatID, u.ID)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	msg, err := s.DB.GetMessage(r.Context(), msgID)
	if err != nil || msg.ChatID != chatID {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err := s.DB.AddReaction(r.Context(), msgID, u.ID, emoji); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	updated, _ := s.DB.GetMessage(r.Context(), msgID)
	if s.Hub != nil {
		s.Hub.BroadcastReaction(chatID, msgID, emoji, u.ID, true)
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) RemoveReaction(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chatID := chi.URLParam(r, "chatID")
	msgID := chi.URLParam(r, "messageID")
	emojiRaw := chi.URLParam(r, "emoji")
	emoji, err := url.PathUnescape(emojiRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "bad emoji encoding")
		return
	}
	ok, _ := s.DB.IsChatMember(r.Context(), chatID, u.ID)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	if err := s.DB.RemoveReaction(r.Context(), msgID, u.ID, emoji); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.BroadcastReaction(chatID, msgID, emoji, u.ID, false)
	}
	updated, _ := s.DB.GetMessage(r.Context(), msgID)
	writeJSON(w, http.StatusOK, updated)
}
