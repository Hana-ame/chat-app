package handlers

import (
	"net/http"
	"net/url"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/go-chi/chi/v5"
)

// AddReaction godoc
// @Summary      Add a reaction to a message
// @Description  React with an emoji to a message
// @Tags         reactions
// @Security     BearerAuth
// @Param        chatID     path  string  true  "Chat ID"
// @Param        messageID  path  string  true  "Message ID"
// @Param        emoji      path  string  true  "Emoji (URL-encoded)"
// @Success      200  {object}  models.Message
// @Router       /api/chats/{chatID}/messages/{messageID}/reactions/{emoji} [put]
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
	if err := s.Services.Chat.MustBeMember(r.Context(), chatID, u.ID); err != nil {
		status, code := mapServiceError(err)
		if status >= 500 {
			w.Header().Set("X-Error", err.Error())
		}
		writeError(w, status, code, "")
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
	logutil.Debug("reaction added: %s on %s by %s", emoji, msgID[:8], u.ID[:8])
	updated, err := s.DB.GetMessage(r.Context(), msgID)
	if err != nil {
		w.Header().Set("X-Error", err.Error())
	}
	if s.Hub != nil {
		s.Hub.BroadcastReaction(chatID, msgID, emoji, u.ID, true)
	}
	writeJSON(w, http.StatusOK, updated)
}

// RemoveReaction godoc
// @Summary      Remove a reaction from a message
// @Description  Remove your emoji reaction from a message
// @Tags         reactions
// @Security     BearerAuth
// @Param        chatID     path  string  true  "Chat ID"
// @Param        messageID  path  string  true  "Message ID"
// @Param        emoji      path  string  true  "Emoji (URL-encoded)"
// @Success      200  {object}  models.Message
// @Router       /api/chats/{chatID}/messages/{messageID}/reactions/{emoji} [delete]
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
	if err := s.Services.Chat.MustBeMember(r.Context(), chatID, u.ID); err != nil {
		status, code := mapServiceError(err)
		if status >= 500 {
			w.Header().Set("X-Error", err.Error())
		}
		writeError(w, status, code, "")
		return
	}
	if err := s.DB.RemoveReaction(r.Context(), msgID, u.ID, emoji); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.BroadcastReaction(chatID, msgID, emoji, u.ID, false)
	}
	updated, err := s.DB.GetMessage(r.Context(), msgID)
	if err != nil {
		w.Header().Set("X-Error", err.Error())
	}
	writeJSON(w, http.StatusOK, updated)
}

// ListReactions godoc
// @Summary      List reactions for a message
// @Description  Get aggregated reactions with user IDs and me flag for the current user
// @Tags         reactions
// @Security     BearerAuth
// @Param        chatID     path  string  true  "Chat ID"
// @Param        messageID  path  string  true  "Message ID"
// @Success      200  {object}  map[string]any
// @Router       /api/chats/{chatID}/messages/{messageID}/reactions [get]
func (s *Server) ListReactions(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chatID := chi.URLParam(r, "chatID")
	msgID := chi.URLParam(r, "messageID")
	if err := s.Services.Chat.MustBeMember(r.Context(), chatID, u.ID); err != nil {
		status, code := mapServiceError(err)
		if status >= 500 {
			w.Header().Set("X-Error", err.Error())
		}
		writeError(w, status, code, "")
		return
	}
	rxs, err := s.DB.ListReactions(r.Context(), msgID, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reactions": rxs})
}
