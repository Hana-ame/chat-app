package service

import (
	"context"

	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/logutil"
)

// ── SearchService（【本地改动 2026-09-03】FTS5 消息搜索）────────

// SearchService 提供消息全文搜索，基于 SQLite FTS5 虚拟表 messages_fts。
// 权限：仅 member 可搜索其所在聊天（通过 Authz.MustBeMember 强制）。
// 若 chat_id 参数为空，则搜索当前用户所有可访问聊天中的消息（由 db.SearchMessages 通过
// chat_members 子查询强制访问控制，避免越权）。
type SearchService struct {
	*Service
}

// SearchMessages 调用 db.SearchMessages，先做 member 校验（chat_id 非空时）。
// chat_id 为空时交由 DB 层通过 chat_members 子查询强制访问控制。
// 【本地改动 2026-09-03】注意：query 未经转义直接进入 FTS5 MATCH，
// 支持空格分词（默认 OR）、"" 短语、* 前缀通配、AND 逻辑运算符。
func (s *SearchService) SearchMessages(ctx context.Context, chatID, actorID string, in db.SearchMessagesInput) (*db.SearchResult, error) {
	if chatID != "" {
		if err := s.Authz.MustBeMember(ctx, chatID, actorID); err != nil {
			return nil, err
		}
	}
	in.ActorID = actorID
	result, err := s.db.SearchMessages(ctx, in)
	if err != nil {
		logutil.Error("SearchMessages failed: %v", err)
		return nil, err
	}
	logutil.Debug("SearchMessages query=%q actor=%s chat=%s -> %d (has_more=%v)", in.Query, actorID, chatID, result.Total, result.HasMore)
	return result, nil
}
