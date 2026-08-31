package handlers

import (
	"net/http"
	"strconv"
)

// ── 【本地改动 2026-09-03】FTS5 消息搜索 ──────────────────────────────────
//
// GET /api/search/messages?query=xxx&chat_id=optional&user_id=optional&before=cursor&limit=50
//
// 访问控制：
//   - chat_id 非空：Authz.MustBeMember 校验（403 若非成员）
//   - chat_id 为空：DB 层通过 chat_members 子查询强制访问控制（ActorID=当前用户）
//
// query 直接透传到 FTS5 MATCH：空格分词（多词 OR），"" 短语，* 前缀，AND 逻辑运算。
// 用户输入无需后端转义（FTS5 MATCH 本身安全）。
//
// 返回：
//
//	{ "messages": [...], "has_more": bool, "next": "<created_at cursor>", "total": N }
func (s *Server) SearchMessages(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	query := r.URL.Query().Get("query")
	if query == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "query is required")
		return
	}
	chatID := r.URL.Query().Get("chat_id")
	userID := r.URL.Query().Get("user_id")
	before := r.URL.Query().Get("before")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		n, err := strconv.Atoi(limitStr)
		if err == nil && n > 0 {
			limit = n
		}
	}

	result, err := s.Services.Search.SearchMessages(r.Context(), chatID, u.ID, struct {
		Query   string
		ChatID  string
		UserID  string
		ActorID string
		Before  string
		Limit   int
	}{
		Query:  query,
		ChatID: chatID,
		UserID: userID,
		ActorID: u.ID,
		Before: before,
		Limit:  limit,
	})
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}

	// next cursor = 最后一条 message 的 created_at（仅当 has_more 时返回）
	next := ""
	if result.HasMore && len(result.Messages) > 0 {
		last := result.Messages[len(result.Messages)-1]
		next = last.CreatedAt.Format("2006-01-02T15:04:05.999999999Z07:00")
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"messages": result.Messages,
		"has_more": result.HasMore,
		"next":     next,
		"total":    result.Total,
	})
}
