package handlers

import (
	"errors"
	"net/http"

	"github.com/Hana-ame/chat-app/server/internal/db"
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
	ok, err := s.DB.IsChatMember(r.Context(), id, u.ID)
	if err != nil || !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	members, err := s.DB.GetChatMembers(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
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
	c, err := s.DB.GetChat(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if c.Type == "dm" {
		writeError(w, http.StatusBadRequest, "bad_request", "cannot add to dm")
		return
	}
	ok, err := s.DB.IsChatMember(r.Context(), id, u.ID)
	if err != nil {
		w.Header().Set("X-Error", err.Error())
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	var req addMemberReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if _, err := s.DB.GetUserByID(r.Context(), req.UserID); err != nil {
		writeError(w, http.StatusNotFound, "user_not_found", "")
		return
	}
	if err := s.DB.AddChatMember(r.Context(), id, req.UserID); err != nil {
		if errors.Is(err, db.ErrConflict) {
			writeError(w, http.StatusConflict, "already_member", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	updated, err := s.DB.GetChat(r.Context(), id)
	if err != nil {
		w.Header().Set("X-Error", err.Error())
	}
	if s.Hub != nil && updated != nil {
		s.Hub.BroadcastChatUpdated(updated)
		s.Hub.NotifyUserNewChat(req.UserID, updated)
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
	c, err := s.DB.GetChat(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if c.Type == "dm" {
		writeError(w, http.StatusBadRequest, "bad_request", "cannot remove from dm")
		return
	}
	if target == c.OwnerID && target != u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "cannot kick owner")
		return
	}
	if target != u.ID {
		if err := s.requireOwnerOrAdmin(r.Context(), id, u.ID); err != nil {
			writeError(w, http.StatusForbidden, "forbidden", "only owner or admin can kick others")
			return
		}
	}
	if err := s.DB.RemoveChatMember(r.Context(), id, target); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.NotifyUserLeftChat(target, id)
		updated, err := s.DB.GetChat(r.Context(), id)
		if err != nil {
			w.Header().Set("X-Error", err.Error())
		}
		if updated != nil {
			s.Hub.BroadcastChatUpdated(updated)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
