package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"

	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/models"
	"github.com/go-chi/chi/v5"
)

type sendMsgReq struct {
	Content     string              `json:"content"`
	Attachments []models.Attachment `json:"attachments"`
}

type editMsgReq struct {
	Content string `json:"content"`
}

var mentionRegex = regexp.MustCompile(`<@([a-f0-9-]{36})>`)

func extractMentions(content string) []string {
	matches := mentionRegex.FindAllStringSubmatch(content, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

func (s *Server) ListMessages(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	ok, _ := s.DB.IsChatMember(r.Context(), id, u.ID)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	before := r.URL.Query().Get("before")
	msgs, err := s.DB.GetMessages(r.Context(), id, u.ID, before, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (s *Server) SendMessage(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	ok, _ := s.DB.IsChatMember(r.Context(), id, u.ID)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	var req sendMsgReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	for i, a := range req.Attachments {
		if a.URL == "" || a.Filename == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "attachment missing url/filename")
			return
		}
		if a.MimeType == "" {
			req.Attachments[i].MimeType = "application/octet-stream"
		}
	}
	mentions := extractMentions(req.Content)
	msg, err := s.DB.CreateMessage(r.Context(), id, u.ID, req.Content, mentions, req.Attachments)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.BroadcastMessageCreate(msg)
	}
	writeJSON(w, http.StatusCreated, msg)
}

func (s *Server) EditMessage(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chatID := chi.URLParam(r, "chatID")
	id := chi.URLParam(r, "messageID")
	ok, _ := s.DB.IsChatMember(r.Context(), chatID, u.ID)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	var req editMsgReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	msg, err := s.DB.UpdateMessage(r.Context(), id, u.ID, req.Content)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "")
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if msg.ChatID != chatID {
		writeError(w, http.StatusBadRequest, "bad_request", "chat mismatch")
		return
	}
	if s.Hub != nil {
		s.Hub.BroadcastMessageUpdate(msg)
	}
	writeJSON(w, http.StatusOK, msg)
}

func (s *Server) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chatID := chi.URLParam(r, "chatID")
	id := chi.URLParam(r, "messageID")
	ok, _ := s.DB.IsChatMember(r.Context(), chatID, u.ID)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	existing, err := s.DB.GetMessage(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if existing.ChatID != chatID {
		writeError(w, http.StatusBadRequest, "bad_request", "chat mismatch")
		return
	}
	chat, _ := s.DB.GetChat(r.Context(), chatID)
	canDeleteAny := chat != nil && chat.OwnerID == u.ID
	if existing.UserID != u.ID && !canDeleteAny {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	if err := s.DB.DeleteMessage(r.Context(), id, u.ID, canDeleteAny); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.BroadcastMessageDelete(chatID, id)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
