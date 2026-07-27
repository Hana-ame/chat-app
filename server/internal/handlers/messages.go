package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/ai"
	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
	"github.com/go-chi/chi/v5"
)

type sendMsgReq struct {
	Content     string              `json:"content"`
	Attachments []models.Attachment `json:"attachments"`
	Type        string              `json:"type"`
	Source      *ai.Source          `json:"source"`
	MsgID       string              `json:"msg_id"`
}

type editMsgReq struct {
	Content string `json:"content"`
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
// @Description  Create and broadcast a new message. For type "stream" messages, returns SSE stream.
// @Tags         messages
// @Security     BearerAuth
// @Param        chatID  path  string       true  "Chat ID"
// @Param        body    body  sendMsgReq   true  "Message content, type, and attachments"
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

	if req.Type == "stream" {
		s.handleStreamMessage(w, r, u, id, &req)
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

func (s *Server) handleStreamMessage(w http.ResponseWriter, r *http.Request, u *models.User, chatID string, req *sendMsgReq) {
	src := req.Source
	if src == nil || src.Endpoint == "" || src.AuthKey == "" || len(src.Body) == 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "source with endpoint, auth_key, and body is required")
		return
	}
	if err := s.Services.Authz.MustBeMember(r.Context(), chatID, u.ID); err != nil {
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}

	msgID := req.MsgID
	if msgID == "" {
		msgID = db.NewID()
	}

	aiCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	go func() {
		<-r.Context().Done()
		cancel()
	}()

	ch, err := s.Services.Stream.StartStream(aiCtx, chatID, u.ID, msgID, *src, u)
	if err != nil {
		logutil.Error("ai: stream failed for user %s: %v", logutil.SafeID(u.ID), err)
		writeError(w, http.StatusBadGateway, "upstream_error", "AI upstream request failed")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	var buf bytes.Buffer

	for chunk := range ch {
		if chunk.Done {
			break
		}
		buf.WriteString(chunk.Content)
		s.Services.Stream.AppendChunk(msgID, chunk.Content)
		data, _ := json.Marshal(map[string]string{"content": chunk.Content})
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(data)
		_, _ = w.Write([]byte("\n\n"))
		flusher.Flush()
	}

	content := buf.String()
	if content == "" {
		content = "（AI 响应为空，请检查 endpoint / auth_key / body 设置）"
	}
	s.Services.Stream.FinishStream(r.Context(), chatID, u.ID, msgID, content)

	_, _ = w.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}

func (s *Server) StreamMessageContent(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chatID := chi.URLParam(r, "chatID")
	msgID := chi.URLParam(r, "messageID")
	if msgID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "missing messageID")
		return
	}
	if err := s.Services.Authz.MustBeMember(r.Context(), chatID, u.ID); err != nil {
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	writeSSE := func(content string) {
		data, _ := json.Marshal(map[string]string{"content": content})
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(data)
		_, _ = w.Write([]byte("\n\n"))
		flusher.Flush()
	}

	sendDone := func() {
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}

	chunks, done, ok, notify := s.Services.Stream.SubscribeFrom(msgID, 0)

	// buffer 不存在 → 从 DB 读
	if !ok {
		msg, err := s.Services.Stream.GetMessage(r.Context(), msgID)
		if err == nil && msg.Content != "" {
			writeSSE(msg.Content)
		}
		sendDone()
		return
	}
	defer s.Services.Stream.Unsubscribe(msgID, notify)

	// 写已有 chunks
	for _, c := range chunks {
		writeSSE(c)
	}

	// 流已结束 → 直接 DONE
	if done {
		sendDone()
		return
	}

	idx := len(chunks)

	deadline := time.After(5 * time.Minute)
loop:
	for {
		select {
		case <-notify:
			chunks, done, ok := s.Services.Stream.StreamStatus(msgID, idx)
			if !ok {
				break loop
			}
			for _, c := range chunks {
				writeSSE(c)
			}
			idx += len(chunks)
			if done {
				break loop
			}
		case <-r.Context().Done():
			return
		case <-deadline:
			break loop
		}
	}
	sendDone()
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
