package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ListNotifications godoc
// @Summary      List notification messages
// @Description  Get paginated messages for the current user's notification inbox
// @Tags         notifications
// @Security     BearerAuth
// @Param        limit   query int     false "Max messages (default 50)"
// @Param        before  query string  false "Message ID to paginate before"
// @Success      200  {object}  map[string]any
// @Router       /api/notifications/messages [get]
func (s *Server) ListNotifications(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chat, err := s.Services.Chat.CreateOrGetNotificationsChat(r.Context(), u.ID)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	before := r.URL.Query().Get("before")
	msgs, err := s.Services.Message.List(r.Context(), chat.ID, u.ID, before, limit)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

// SendNotification godoc
// @Summary      Send a notification message
// @Description  Send a message to the current user's notification inbox
// @Tags         notifications
// @Security     BearerAuth
// @Param        body  body  sendMsgReq  true  "Message content"
// @Success      201  {object}  models.Message
// @Router       /api/notifications/messages [post]
func (s *Server) SendNotification(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chat, err := s.Services.Chat.CreateOrGetNotificationsChat(r.Context(), u.ID)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	var req sendMsgReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	msg, err := s.Services.Message.Send(r.Context(), chat.ID, u.ID, req.Content, req.Attachments)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, msg)
}

// DeleteNotification godoc
// @Summary      Delete a notification message
// @Description  Delete a message from the user's notification inbox
// @Tags         notifications
// @Security     BearerAuth
// @Param        messageID  path  string  true  "Message ID"
// @Success      200  {object}  map[string]any
// @Router       /api/notifications/messages/{messageID} [delete]
func (s *Server) DeleteNotification(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chat, err := s.Services.Chat.CreateOrGetNotificationsChat(r.Context(), u.ID)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	messageID := chi.URLParam(r, "messageID")
	if err := s.Services.Message.Delete(r.Context(), chat.ID, messageID, u.ID); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// MarkNotificationsRead godoc
// @Summary      Mark all notifications as read
// @Description  Mark the current user's notification inbox as read
// @Tags         notifications
// @Security     BearerAuth
// @Success      200  {object}  map[string]any
// @Router       /api/notifications/read [post]
func (s *Server) MarkNotificationsRead(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chat, err := s.Services.Chat.CreateOrGetNotificationsChat(r.Context(), u.ID)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	if err := s.Services.Message.MarkRead(r.Context(), chat.ID, u.ID); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
