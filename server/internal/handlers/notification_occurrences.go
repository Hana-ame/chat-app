package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ListNotificationOccurrences godoc
// @Summary      List persistent notification occurrences
// @Description  Get the current user's persistent notifications (newest first)
// @Tags         notifications
// @Security     BearerAuth
// @Param        limit   query int    false "Max occurrences (default 50)"
// @Param        before  query string false "created_at cursor for pagination"
// @Success      200  {object}  map[string]any
// @Router       /api/notifications [get]
func (s *Server) ListNotificationOccurrences(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	before := r.URL.Query().Get("before")
	occ, err := s.Services.Notification.List(r.Context(), u.ID, before, limit)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"occurrences": occ})
}

// NotificationUnreadCount godoc
// @Summary      Count unread notification occurrences
// @Description  Return the current user's unread persistent notification count
// @Tags         notifications
// @Security     BearerAuth
// @Success      200  {object}  map[string]any
// @Router       /api/notifications/unread-count [get]
func (s *Server) NotificationUnreadCount(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	n, err := s.Services.Notification.UnreadCount(r.Context(), u.ID)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": n})
}

// MarkNotificationRead godoc
// @Summary      Mark one notification occurrence as read
// @Tags         notifications
// @Security     BearerAuth
// @Param        id  path  string  true  "Occurrence ID"
// @Success      200  {object}  map[string]any
// @Router       /api/notifications/{id}/read [post]
func (s *Server) MarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "id")
	if err := s.Services.Notification.MarkRead(r.Context(), id, u.ID); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// MarkAllNotificationsRead godoc
// @Summary      Mark all notification occurrences as read
// @Tags         notifications
// @Security     BearerAuth
// @Success      200  {object}  map[string]any
// @Router       /api/notifications/read-all [post]
func (s *Server) MarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if err := s.Services.Notification.MarkAllRead(r.Context(), u.ID); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// DeleteNotificationOccurrence godoc
// @Summary      Delete one notification occurrence
// @Tags         notifications
// @Security     BearerAuth
// @Param        id  path  string  true  "Occurrence ID"
// @Success      200  {object}  map[string]any
// @Router       /api/notifications/{id} [delete]
func (s *Server) DeleteNotificationOccurrence(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "id")
	if err := s.Services.Notification.Delete(r.Context(), id, u.ID); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
