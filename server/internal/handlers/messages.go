package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

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

type readReq struct {
	MessageID string `json:"message_id"`
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

// ListMessages godoc
// @Summary      List messages in a chat
// @Description  Get paginated messages for a chat the user is a member of
// @Tags         messages
// @Security     BearerAuth
// @Param        chatID  path  string  true  "Chat ID"
// @Param        limit   query int     false "Max messages (default 50)"
// @Param        before  query string  false "Message ID to paginate before"
// @Success      200  {object}  map[string]any
// @Failure      403  {object}  map[string]any
// @Router       /api/chats/{chatID}/messages [get]
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
	details := r.URL.Query().Get("details") == "true"
	msgs, err := s.DB.GetMessages(r.Context(), id, u.ID, before, limit, details)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

// SendMessage godoc
// @Summary      Send a message to a chat
// @Description  Create and broadcast a new message
// @Tags         messages
// @Security     BearerAuth
// @Param        chatID  path  string       true  "Chat ID"
// @Param        body    body  sendMsgReq   true  "Message content and attachments"
// @Success      201  {object}  models.Message
// @Failure      403  {object}  map[string]any
// @Router       /api/chats/{chatID}/messages [post]
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
		if !strings.HasPrefix(a.URL, "https://upload.moonchan.xyz/") {
			writeError(w, http.StatusBadRequest, "bad_request", "attachment url must be on upload.moonchan.xyz")
			return
		}
		if a.MimeType == "" {
			req.Attachments[i].MimeType = "application/octet-stream"
		}
	}
	mentions := extractMentions(req.Content)
	msg, err := s.DB.CreateMessage(r.Context(), id, u.ID, req.Content, mentions, req.Attachments)
	if err != nil {
		if strings.Contains(err.Error(), "content too long") {
			writeError(w, http.StatusForbidden, "content_too_long", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.BroadcastMessageCreate(msg)
	}
	writeJSON(w, http.StatusCreated, msg)
}

// EditMessage godoc
// @Summary      Edit a message
// @Description  Update message content (author only)
// @Tags         messages
// @Security     BearerAuth
// @Param        chatID     path  string      true  "Chat ID"
// @Param        messageID  path  string      true  "Message ID"
// @Param        body       body  editMsgReq  true  "New content"
// @Success      200  {object}  models.Message
// @Failure      403  {object}  map[string]any
// @Router       /api/chats/{chatID}/messages/{messageID} [patch]
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

// DeleteMessage godoc
// @Summary      Delete a message
// @Description  Delete own message or any message as chat owner
// @Tags         messages
// @Security     BearerAuth
// @Param        chatID     path  string  true  "Chat ID"
// @Param        messageID  path  string  true  "Message ID"
// @Success      200  {object}  map[string]any
// @Failure      403  {object}  map[string]any
// @Router       /api/chats/{chatID}/messages/{messageID} [delete]
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
	canDeleteAny := chat != nil && (chat.OwnerID == u.ID || s.requireOwnerOrAdmin(r.Context(), chatID, u.ID) == nil)
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

// MarkRead godoc
// @Summary      Mark messages as read
// @Description  Update the last-read message pointer for a chat
// @Tags         messages
// @Security     BearerAuth
// @Param        chatID  path  string   true  "Chat ID"
// @Param        body    body  readReq  true  "Last read message ID"
// @Success      200  {object}  map[string]any
// @Failure      403  {object}  map[string]any
// @Router       /api/chats/{chatID}/read [post]
func (s *Server) MarkRead(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	ok, _ := s.DB.IsChatMember(r.Context(), id, u.ID)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	var req readReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if req.MessageID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "message_id required")
		return
	}
	if err := s.DB.UpdateLastRead(r.Context(), id, u.ID, req.MessageID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
