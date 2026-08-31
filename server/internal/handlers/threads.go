package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// 【本地改动 2026-08-31】实现消息线程聚合（root 消息 + reply_to 树）： API。
type followThreadReq struct {
	ThreadRootMessageID string `json:"thread_root_message_id"`
}
type threadReadReq struct {
	ThreadRootMessageID string `json:"thread_root_message_id"`
}

// ListThreadSummarys godoc
// @Summary      List threads the user is following
// @Description  Paginated list of followed threads with metadata
// @Tags         threads
// @Security     BearerAuth
// @Param        limit  query   int     false "Max threads (default 50)"
// @Param        before query   string  false "Thread root message ID to paginate before"
// @Success      200    {object} map[string]any
// @Failure      401    {object} map[string]any
// @Router       /api/threads [get]
func (s *Server) ListThreadSummarys(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	threads, err := s.DB.ListThreadSummarys(r.Context(), u.ID, r.URL.Query().Get("before"), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"threads": threads})
}

// FollowThread godoc
// @Summary      Follow a thread
// @Description  Opt into reply_in_thread notifications for a thread root
// @Tags         threads
// @Security     BearerAuth
// @Param        body body followThreadReq true "Thread root message ID"
// @Success      200  {object} map[string]any
// @Failure      400  {object} map[string]any
// @Router       /api/threads/follow [post]
func (s *Server) FollowThread(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	var req followThreadReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if req.ThreadRootMessageID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "thread_root_message_id required")
		return
	}
	if err := s.DB.FollowThread(r.Context(), u.ID, req.ThreadRootMessageID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "following": true})
}

// UnfollowThread godoc
// @Summary      Unfollow a thread
// @Description  Stop receiving reply_in_thread notifications for a thread root
// @Tags         threads
// @Security     BearerAuth
// @Param        body body followThreadReq true "Thread root message ID"
// @Success      200  {object} map[string]any
// @Failure      400  {object} map[string]any
// @Router       /api/threads/follow [delete]
func (s *Server) UnfollowThread(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	var req followThreadReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if req.ThreadRootMessageID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "thread_root_message_id required")
		return
	}
	if err := s.DB.UnfollowThread(r.Context(), u.ID, req.ThreadRootMessageID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "following": false})
}

// GetThreadSummary godoc
// @Summary      Get a single thread summary
// @Description  Root message + metadata (reply_count, is_following, has_unread, ...)
// @Tags         threads
// @Security     BearerAuth
// @Param        chatID       path string true "Chat ID"
// @Param        threadRootID path string true "Thread root message ID"
// @Success      200          {object} map[string]any
// @Failure      404          {object} map[string]any
// @Router       /api/chats/{chatID}/threads/{threadRootID} [get]
func (s *Server) GetThreadSummary(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chatID := chi.URLParam(r, "chatID")
	threadRootID := chi.URLParam(r, "threadRootID")
	if err := s.Services.Authz.MustBeMember(r.Context(), chatID, u.ID); err != nil {
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	thread, err := s.DB.GetThreadSummary(r.Context(), chatID, u.ID, threadRootID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, thread)
}

// MarkThreadRead godoc
// @Summary      Mark a thread as read
// @Description  Advance the user's read cursor to the latest reply in the thread
// @Tags         threads
// @Security     BearerAuth
// @Param        body body threadReadReq true "Thread root message ID"
// @Success      200  {object} map[string]any
// @Failure      400  {object} map[string]any
// @Router       /api/threads/read [post]
func (s *Server) MarkThreadRead(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	var req threadReadReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if req.ThreadRootMessageID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "thread_root_message_id required")
		return
	}
	latestID, err := s.DB.LatestReplyIDForThread(r.Context(), req.ThreadRootMessageID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if latestID == "" {
		latestID = req.ThreadRootMessageID
	}
	if err := s.DB.SetThreadRead(r.Context(), u.ID, req.ThreadRootMessageID, latestID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "thread_root_message_id": req.ThreadRootMessageID})
}
