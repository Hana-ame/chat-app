package handlers

import (
	"net/http"

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
	chats, err := s.Services.Chat.ListForUser(r.Context(), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
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
	chat, err := s.Services.Chat.Create(r.Context(), u.ID, req.Name, req.Visibility, req.MemberIDs)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
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
	c, err := s.Services.Chat.GetByID(r.Context(), id, u.ID)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
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
	var req renameChatReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	updated, err := s.Services.Chat.Rename(r.Context(), id, u.ID, req.Name)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
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
	if err := s.Services.Chat.Delete(r.Context(), id, u.ID); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
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
	chats, err := s.Services.Chat.ListPublic(r.Context(), page, limit)
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
	chat, err := s.Services.Chat.Join(r.Context(), id, u.ID)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"chat": chat})
}

type pinContentReq struct {
	Content string `json:"content"`
}

func (s *Server) PinChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	var req pinContentReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := s.Services.Chat.SetAnnouncement(r.Context(), id, u.ID, req.Content); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) UpdatePinnedChat(w http.ResponseWriter, r *http.Request) {
	s.PinChat(w, r)
}

func (s *Server) DeletePinnedChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	if err := s.Services.Chat.ClearAnnouncement(r.Context(), id, u.ID); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) PinChatList(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	if err := s.Services.Chat.SetPinned(r.Context(), id, u.ID, true); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "pinned": true})
}

func (s *Server) UnpinChatList(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	if err := s.Services.Chat.SetPinned(r.Context(), id, u.ID, false); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "pinned": false})
}

func (s *Server) MarkPinnedRead(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	if err := s.Services.Chat.MarkAnnouncementRead(r.Context(), id, u.ID); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) VisitChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id := chi.URLParam(r, "chatID")
	if err := s.Services.Chat.Visit(r.Context(), id, u.ID); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
