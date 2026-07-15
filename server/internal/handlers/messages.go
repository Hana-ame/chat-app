package handlers

import (
	"net/http"
	"strconv"

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

// Deprecated: no longer needed; MarkRead now uses last_active_at.
type readReq struct {
	MessageID string `json:"message_id"`
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
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	before := r.URL.Query().Get("before")
	msgs, err := s.Services.Message.List(r.Context(), id, u.ID, before, limit)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
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
	var req sendMsgReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	msg, err := s.Services.Message.Send(r.Context(), id, u.ID, req.Content, req.Attachments)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
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
	messageID := chi.URLParam(r, "messageID")
	var req editMsgReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	msg, err := s.Services.Message.Edit(r.Context(), chatID, messageID, u.ID, req.Content)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
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
	messageID := chi.URLParam(r, "messageID")
	if err := s.Services.Message.Delete(r.Context(), chatID, messageID, u.ID); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) MarkRead(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	if err := s.Services.Message.MarkRead(r.Context(), id, u.ID); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
