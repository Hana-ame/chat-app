package service

import (
	"context"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

// ── PinService（【本地改动 2026-09-02】chatto FDR-037 多消息置顶）────────

// PinService 管理消息置顶（区别于聊天公告 pinned_message）。
// 权限语义：owner/admin 可 pin/unpin；任何 member 可 list。
// DM 不支持置顶（service 层拒绝）。幂等：重复 pin/unpin 返回 200 不报错。
// 一条消息在一条聊天中最多一个 pin（唯一索引保证）。
type PinService struct {
	*Service
}

// PinMessage 置顶 chat 中的 message。返回是否原本已置顶。
// DM chat → ErrInvalidInput。消息不在 chat / 已删除 → ErrNotFound。
// 操作者非 owner/admin → ErrForbidden。
func (s *PinService) PinMessage(ctx context.Context, chatID, messageID, actorID string) (bool, error) {
	if err := s.Authz.RequireOwnerOrAdmin(ctx, chatID, actorID); err != nil {
		return false, err
	}
	chat, err := s.db.GetChat(ctx, chatID)
	if err != nil {
		return false, err
	}
	// DM 不支持置顶（chatto FDR-037）
	if chat.Type == "dm" {
		return false, ErrInvalidInput
	}
	// 目标消息必须存在且属于该 chat 且未删除
	msg, err := s.db.GetMessage(ctx, messageID)
	if err != nil {
		return false, err
	}
	if msg.ChatID != chatID {
		return false, ErrNotFound
	}
	if msg.DeletedAt != nil {
		return false, ErrNotFound
	}
	already, err := s.db.PinMessage(ctx, chatID, messageID, actorID)
	if err != nil {
		return false, err
	}
	logutil.Debug("PinMessage chat=%s msg=%s by=%s already=%v", chatID, messageID, actorID, already)
	return already, nil
}

// UnpinMessage 取消置顶。返回是否原本有置顶。
// 未置顶 → false、无错误（幂等）。
func (s *PinService) UnpinMessage(ctx context.Context, chatID, messageID, actorID string) (bool, error) {
	if err := s.Authz.RequireOwnerOrAdmin(ctx, chatID, actorID); err != nil {
		return false, err
	}
	wasPinned, err := s.db.UnpinMessage(ctx, chatID, messageID)
	if err != nil {
		return false, err
	}
	logutil.Debug("UnpinMessage chat=%s msg=%s by=%s was=%v", chatID, messageID, actorID, wasPinned)
	return wasPinned, nil
}

// ListPinnedMessages 返回 chat 的置顶列表（created_at DESC，cursor 分页）。
// 仅 member 可调用；DM 不支持置顶（返回空列表）。
// 返回 (entries, hasMore, err)。
// 【本地改动 2026-09-02】与 chatto 保持一致：已删除的消息仍保留 pin 行（简化实现），
// 前端按 Message.Deleted 过滤显示。踩坑：若消息删除后立即自动 unpin 会导致
// 消息恢复后 pin 丢失，故不自动清理。
func (s *PinService) ListPinnedMessages(ctx context.Context, chatID, actorID string, before string, limit int) ([]models.PinEntry, bool, error) {
	if err := s.Authz.MustBeMember(ctx, chatID, actorID); err != nil {
		return nil, false, err
	}
	chat, err := s.db.GetChat(ctx, chatID)
	if err != nil {
		return nil, false, err
	}
	if chat.Type == "dm" {
		return nil, false, nil // DM 不支持置顶：返回空
	}
	pins, err := s.db.ListPinnedMessages(ctx, chatID, before, limit)
	if err != nil {
		return nil, false, err
	}

	// 多取了一条则说明还有更多
	hasMore := len(pins) > limit
	pins = pins[:min(len(pins), limit)]

	var entries []models.PinEntry
	for _, p := range pins {
		entries = append(entries, models.PinEntry{
			ChatID:    p.ChatID,
			MessageID: p.MessageID,
			PinnedBy:  p.PinnedBy,
			PinnedAt:  p.PinnedAt,
			Message:   p.Message,
		})
	}

	return entries, hasMore, nil
}
