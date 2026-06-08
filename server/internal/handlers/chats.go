package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/go-chi/chi/v5"
)

type createChatReq struct {
	Type       string   `json:"type"`
	Name       string   `json:"name"`
	Visibility string   `json:"visibility"`
	MemberIDs  []string `json:"member_ids"`
}

type createDMReq struct {
	UserID string `json:"user_id"`
}

type addMemberReq struct {
	UserID string `json:"user_id"`
}

type renameChatReq struct {
	Name string `json:"name"`
}

type readReq struct {
	MessageID string `json:"message_id"`
}

func (s *Server) ListChats(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chats, err := s.DB.ListUserChats(r.Context(), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"chats": chats})
}

func (s *Server) CreateChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	var req createChatReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if req.Type != "group" && req.Type != "dm" {
		writeError(w, http.StatusBadRequest, "bad_request", "type must be group or dm")
		return
	}
	if req.Type == "dm" {
		writeError(w, http.StatusBadRequest, "bad_request", "use POST /api/dms for direct messages")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "name required")
		return
	}
	members := req.MemberIDs
	hasMe := false
	for _, m := range members {
		if m == u.ID {
			hasMe = true
			break
		}
	}
	if !hasMe {
		members = append(members, u.ID)
	}
	chat, err := s.DB.CreateChat(r.Context(), "group", req.Name, req.Visibility, u.ID, members)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.BroadcastChatCreated(chat)
	}
	writeJSON(w, http.StatusCreated, chat)
}

func (s *Server) CreateOrGetDM(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	var req createDMReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if req.UserID == "" || req.UserID == u.ID {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid user_id")
		return
	}
	other, err := s.DB.GetUserByID(r.Context(), req.UserID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user_not_found", "")
		return
	}
	if dm, err := s.DB.FindDMBetween(r.Context(), u.ID, other.ID); err == nil {
		writeJSON(w, http.StatusOK, dm)
		return
	}
	chat, err := s.DB.CreateChat(r.Context(), "dm", "", "", "", []string{u.ID, other.ID})
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.BroadcastChatCreated(chat)
	}
	writeJSON(w, http.StatusCreated, chat)
}

func (s *Server) GetChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	ok, err := s.DB.IsChatMember(r.Context(), id, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "not a member")
		return
	}
	c, err := s.DB.GetChat(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) RenameChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	c, err := s.DB.GetChat(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if c.Type == "dm" {
		writeError(w, http.StatusBadRequest, "bad_request", "cannot rename dm")
		return
	}
	if c.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "only owner can rename")
		return
	}
	var req renameChatReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := s.DB.RenameChat(r.Context(), id, req.Name); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	updated, _ := s.DB.GetChat(r.Context(), id)
	if s.Hub != nil && updated != nil {
		s.Hub.BroadcastChatUpdated(updated)
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) DeleteChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	id := chi.URLParam(r, "chatID")
	c, err := s.DB.GetChat(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if c.Type == "dm" {
		writeError(w, http.StatusBadRequest, "bad_request", "cannot delete dm; leave instead")
		return
	}
	if c.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "only owner can delete")
		return
	}
	if err := s.DB.DeleteChat(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.BroadcastChatDeleted(c, id)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

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
	ok, _ := s.DB.IsChatMember(r.Context(), id, u.ID)
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
	updated, _ := s.DB.GetChat(r.Context(), id)
	if s.Hub != nil && updated != nil {
		s.Hub.BroadcastChatUpdated(updated)
		s.Hub.NotifyUserNewChat(req.UserID, updated)
	}
	writeJSON(w, http.StatusOK, updated)
}

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
	if target != u.ID && c.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "only owner can kick others")
		return
	}
	if target == c.OwnerID && target != u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "cannot kick owner")
		return
	}
	if err := s.DB.RemoveChatMember(r.Context(), id, target); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if s.Hub != nil {
		s.Hub.NotifyUserLeftChat(target, id)
		if updated, _ := s.DB.GetChat(r.Context(), id); updated != nil {
			s.Hub.BroadcastChatUpdated(updated)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

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
