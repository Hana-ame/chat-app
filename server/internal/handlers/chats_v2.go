package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) ListPublicChats(w http.ResponseWriter, r *http.Request) {
	chats, err := s.DB.ListPublicChats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"chats": chats})
}

type joinReq struct {
	ChatID string `json:"chat_id"`
}

func (s *Server) JoinChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	if err := s.DB.JoinPublicChat(r.Context(), id, u.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	chat, _ := s.DB.GetChat(r.Context(), id)
	if s.Hub != nil && chat != nil {
		s.Hub.NotifyUserNewChat(u.ID, chat)
		s.Hub.BroadcastChatUpdated(chat)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) PinChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	if err := s.DB.PinChat(r.Context(), id, u.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) UnpinChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	if err := s.DB.UnpinChat(r.Context(), id, u.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
