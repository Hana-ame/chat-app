package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/go-chi/chi/v5"
)

type createChatReq struct {
	Type       string   `json:"type"`
	Name       string   `json:"name"`
	Visibility string   `json:"visibility"`
	MemberIDs  []string `json:"member_ids"`
}

// Deprecated.
type createDMReq struct {
	UserID string `json:"user_id"`
}

type renameChatReq struct {
	Name string `json:"name"`
}

type joinReq struct {
	ChatID string `json:"chat_id"`
}

// ListChats godoc
// @Summary      List user's chats
// @Description  Get all chats the authenticated user is a member of
// @Tags         chats
// @Security     BearerAuth
// @Success      200  {object}  map[string]any
// @Router       /api/chats [get]
func (s *Server) ListChats(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	chats, err := s.DB.ListUserChats(r.Context(), u.ID)
	if err != nil {
		logutil.Error("list chats for %s: %v", u.ID[:8], err)
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	logutil.Debug("listed %d chats for user %s", len(chats), u.ID[:8])
	writeJSON(w, http.StatusOK, map[string]any{"chats": chats})
}

// CreateChat godoc
// @Summary      Create a group chat
// @Description  Create a new group chat with specified members
// @Tags         chats
// @Security     BearerAuth
// @Param        body  body  createChatReq  true  "Chat details"
// @Success      201  {object}  models.Chat
// @Router       /api/chats [post]
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
		logutil.Error("create chat: %v (user=%s name=%s)", err, u.ID[:8], req.Name)
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	logutil.Info("chat created: %s (name=%s owner=%s)", chat.ID[:8], req.Name, u.ID[:8])
	if s.Hub != nil {
		s.Hub.BroadcastChatCreated(chat)
	}
	writeJSON(w, http.StatusCreated, chat)
}

// Deprecated.
// CreateOrGetDM godoc
// @Summary      Create or get DM chat
// @Description  Find existing DM or create a new one with another user
// @Tags         chats
// @Security     BearerAuth
// @Param        body  body  createDMReq  true  "Target user ID"
// @Success      200  {object}  models.Chat
// @Success      201  {object}  models.Chat
// @Router       /api/dms [post]
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

// GetChat godoc
// @Summary      Get chat details
// @Description  Fetch a single chat by ID (must be a member)
// @Tags         chats
// @Security     BearerAuth
// @Param        chatID  path  string  true  "Chat ID"
// @Success      200  {object}  models.Chat
// @Router       /api/chats/{chatID} [get]
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

// RenameChat godoc
// @Summary      Rename a group chat
// @Description  Change chat name (owner only, not allowed on DMs)
// @Tags         chats
// @Security     BearerAuth
// @Param        chatID  path  string         true  "Chat ID"
// @Param        body    body  renameChatReq  true  "New name"
// @Success      200  {object}  models.Chat
// @Router       /api/chats/{chatID} [patch]
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
	updated, err := s.DB.GetChat(r.Context(), id)
	if err != nil {
		w.Header().Set("X-Error", err.Error())
	}
	if s.Hub != nil && updated != nil {
		s.Hub.BroadcastChatUpdated(updated)
	}
	writeJSON(w, http.StatusOK, updated)
}

// DeleteChat godoc
// @Summary      Delete a group chat
// @Description  Remove chat and all messages (owner only, not allowed on DMs)
// @Tags         chats
// @Security     BearerAuth
// @Param        chatID  path  string  true  "Chat ID"
// @Success      200  {object}  map[string]any
// @Router       /api/chats/{chatID} [delete]
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
		logutil.Error("delete chat %s: %v", id[:8], err)
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	logutil.Warn("chat deleted: %s by %s", id[:8], u.ID[:8])
	if s.Hub != nil {
		s.Hub.BroadcastChatDeleted(c, id)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ListPublicChats godoc
// @Summary      List public chats
// @Description  Get discoverable public chats with pagination, ordered by recent activity
// @Tags         chats
// @Security     BearerAuth
// @Param        page   query  int  false  "Page number (default 1)"
// @Param        limit  query  int  false  "Items per page (default 20, max 50)"
// @Success      200  {object}  map[string]any
// @Router       /api/chats/public [get]
func (s *Server) ListPublicChats(w http.ResponseWriter, r *http.Request) {
	page := intQueryParam(r, "page", 1)
	limit := intQueryParam(r, "limit", 20)
	chats, err := s.DB.ListPublicChats(r.Context(), page, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"chats": chats})
}

// JoinChat godoc
// @Summary      Join a public chat
// @Description  Authenticated user joins a public chat by ID
// @Tags         chats
// @Security     BearerAuth
// @Param        chatID  path  string  true  "Chat ID"
// @Success      200  {object}  map[string]any
// @Router       /api/chats/{chatID}/join [post]
func (s *Server) JoinChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	if err := s.DB.JoinChatByID(r.Context(), id, u.ID); err != nil {
		logutil.Error("join chat %s: %v (user=%s)", id[:8], err, u.ID[:8])
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	logutil.Info("user %s joined chat %s", u.ID[:8], id[:8])
	chat, err := s.DB.GetChat(r.Context(), id)
	if err != nil {
		w.Header().Set("X-Error", err.Error())
	}
	if s.Hub != nil && chat != nil {
		s.Hub.NotifyUserNewChat(u.ID, chat)
		s.Hub.BroadcastChatUpdated(chat)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type pinContentReq struct {
	Content string `json:"content"`
}

func (s *Server) requireOwnerOrAdmin(ctx context.Context, chatID, userID string) error {
	c, err := s.DB.GetChat(ctx, chatID)
	if err != nil {
		return err
	}
	if c.OwnerID == userID {
		return nil
	}
	role, err := s.DB.GetChatMemberRole(ctx, chatID, userID)
	if err != nil {
		return err
	}
	if role == "admin" {
		return nil
	}
	return errors.New("forbidden")
}

// PinChat godoc
// @Summary      Set pinned message
// @Description  Set pinned message text (owner/admin only, chat must have ≥3 members)
// @Tags         chats
// @Security     BearerAuth
// @Param        chatID  path  string        true  "Chat ID"
// @Param        body    body  pinContentReq true  "Pinned message content"
// @Success      200  {object}  map[string]any
// @Router       /api/chats/{chatID}/pin [post]
func (s *Server) PinChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	c, err := s.DB.GetChat(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if c.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	n, err := s.DB.ChatMemberCount(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if n < 3 {
		writeError(w, http.StatusBadRequest, "bad_request", "need at least 3 members to pin")
		return
	}
	var req pinContentReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := s.DB.SetPinnedMessage(r.Context(), id, req.Content); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if s.Hub != nil {
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

// UpdatePinnedChat godoc
// @Summary      Update pinned message
// @Description  Update the pinned message text
// @Tags         chats
// @Security     BearerAuth
// @Param        chatID  path  string        true  "Chat ID"
// @Param        body    body  pinContentReq true  "New pinned message content"
// @Success      200  {object}  map[string]any
// @Router       /api/chats/{chatID}/pin [patch]
func (s *Server) UpdatePinnedChat(w http.ResponseWriter, r *http.Request) {
	s.PinChat(w, r)
}

// DeletePinnedChat godoc
// @Summary      Remove pinned message
// @Description  Clear the pinned message from a chat (owner/admin only)
// @Tags         chats
// @Security     BearerAuth
// @Param        chatID  path  string  true  "Chat ID"
// @Success      200  {object}  map[string]any
// @Router       /api/chats/{chatID}/pin [delete]
func (s *Server) DeletePinnedChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	if err := s.requireOwnerOrAdmin(r.Context(), id, u.ID); err != nil {
		writeError(w, http.StatusForbidden, "forbidden", "")
		return
	}
	if err := s.DB.ClearPinnedMessage(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if s.Hub != nil {
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

// TogglePin godoc
// @Summary      Toggle sidebar pinning
// @Description  Toggle whether the chat appears at the top of the user's sidebar list
// @Tags         chats
// @Security     BearerAuth
// @Param        chatID  path  string  true  "Chat ID"
// @Success      200  {object}  map[string]any
// @Router       /api/chats/{chatID}/pin-toggle [post]
func (s *Server) TogglePin(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
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
	if err := s.DB.TogglePinned(r.Context(), id, u.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if s.Hub != nil {
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

// MarkPinnedRead godoc
// @Summary      Mark pinned message as read
// @Description  Update pinned_last_read_at for the current user in the given chat
// @Tags         chats
// @Security     BearerAuth
// @Param        chatID  path  string  true  "Chat ID"
// @Success      200  {object}  map[string]any
// @Router       /api/chats/{chatID}/pin-read [post]
func (s *Server) MarkPinnedRead(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	if err := s.DB.UpdatePinnedLastReadAt(r.Context(), id, u.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// VisitChat godoc
// @Summary      Record a chat visit
// @Description  Update last_visited_at for the current user in the given chat
// @Tags         chats
// @Security     BearerAuth
// @Param        chatID  path  string  true  "Chat ID"
// @Success      200  {object}  map[string]any
// @Router       /api/chats/{chatID}/visit [post]
func (s *Server) VisitChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	if err := s.DB.UpdateLastVisitedAt(r.Context(), id, u.ID); err != nil {
		logutil.Error("visit chat %s: %v", id[:8], err)
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	logutil.Debug("user %s visited chat %s", u.ID[:8], id[:8])
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
