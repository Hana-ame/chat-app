package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type addMemberReq struct {
	UserID string `json:"user_id"`
}

// ListMembers godoc
// @Summary      List chat members
// @Description  Get all members of a chat (must be a member)
// @Tags         members
// @Security     BearerAuth
// @Param        chatID  path  string  true  "Chat ID"
// @Success      200  {object}  map[string]any
// @Router       /api/chats/{chatID}/members [get]
func (s *Server) ListMembers(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	members, err := s.Services.Member.List(r.Context(), id, u.ID)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

// AddMember godoc
// @Summary      Add a member to a chat
// @Description  Add a user to a group chat (existing member required, DM not allowed)
// @Tags         members
// @Security     BearerAuth
// @Param        chatID  path  string        true  "Chat ID"
// @Param        body    body  addMemberReq  true  "User ID to add"
// @Success      200  {object}  models.Chat
// @Router       /api/chats/{chatID}/members [post]
func (s *Server) AddMember(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	var req addMemberReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	updated, err := s.Services.Member.Add(r.Context(), id, u.ID, req.UserID)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// RemoveMember godoc
// @Summary      Remove a member from a chat
// @Description  Kick a user or leave chat (DM not allowed, owner cannot be kicked)
// @Tags         members
// @Security     BearerAuth
// @Param        chatID  path  string  true  "Chat ID"
// @Param        userID  path  string  true  "User ID to remove"
// @Success      200  {object}  map[string]any
// @Router       /api/chats/{chatID}/members/{userID} [delete]
func (s *Server) RemoveMember(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	target := chi.URLParam(r, "userID")
	if err := s.Services.Member.Remove(r.Context(), id, u.ID, target); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
