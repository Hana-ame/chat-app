package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ── 【本地改动 2026-09-02】chatto FDR-037 消息置顶（多消息）────────────────
// 与 chat_announcement.go（聊天公告）/ chat_prefs.go（用户侧 chat 置顶）完全独立。
//
// 路由：
//   - POST /api/chats/{chatID}/pins/{messageID}  — 置顶某消息
//   - DELETE /api/chats/{chatID}/pins/{messageID} — 取消置顶
//   - GET /api/chats/{chatID}/pins[?before=<cursor>&limit=N] — 列出置顶
//
// 幂等：重复 pin/unpin 返回 200，不报错（对齐 chatto 行为）。
// 权限：owner/admin 才能 pin/unpin；任何 member 可 list；DM 不接受 pin。

type pinMessageReq struct {
	// 可选：幂等 key，用于防抖（暂存字段，未使用）。
}

func (s *Server) PinMessage(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	chatID := chi.URLParam(r, "chatID")
	messageID := chi.URLParam(r, "messageID")
	if chatID == "" || messageID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "chat_id and message_id required")
		return
	}

	alreadyExisted, err := s.Services.Pin.PinMessage(r.Context(), chatID, messageID, u.ID)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"already":      alreadyExisted,
		"chat_id":      chatID,
		"message_id":   messageID,
	})
}

func (s *Server) UnpinMessage(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	chatID := chi.URLParam(r, "chatID")
	messageID := chi.URLParam(r, "messageID")
	if chatID == "" || messageID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "chat_id and message_id required")
		return
	}

	wasPinned, err := s.Services.Pin.UnpinMessage(r.Context(), chatID, messageID, u.ID)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"unpinned": wasPinned,
		"chat_id":  chatID,
	})
}

func (s *Server) ListPinnedMessages(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	chatID := chi.URLParam(r, "chatID")
	if chatID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "chat_id required")
		return
	}

	before := r.URL.Query().Get("before")
	// 【本地改动 2026-09-02】limit 用 query 参数（默认 20，最大 100）；
	// 与 chatto FDR-037 的 paginate-first 语义对齐。
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		n, err := strconv.Atoi(limitStr)
		if err == nil && n > 0 {
			limit = n
		}
	}

	entries, hasMore, err := s.Services.Pin.ListPinnedMessages(r.Context(), chatID, u.ID, before, limit)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}

	lastCursor := ""
	if len(entries) > 0 {
		lastCursor = entries[len(entries)-1].PinnedAt.Format("2006-01-02T15:04:05.999999999Z07:00")
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"chat_id":  chatID,
		"pins":     entries,
		"total":    len(entries),
		"has_more": hasMore,
		"next":     lastCursor,
	})
}
