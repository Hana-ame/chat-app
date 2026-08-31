package service

import (
	"context"
	"strings"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

const (
	// notificationBodySnippetLen 是通知正文的截断长度（避免把整条长消息塞进通知）。
	notificationBodySnippetLen = 120
)

// NotificationService 提供持久化通知（occurrence）的写入触发与读取命令，
// 移植 chatto FDR-012 的通知机制到 SQLite 栈：
//   - 写入：消息发送时按「提及 + 回复」触发，每个 (收件人, kind, chat, 源消息)
//     由数据库唯一索引保证幂等（重复触发不重复插行、不重置已读）。
//   - 投递：创建成功且收件人在线时立即经 ws.Hub 广播 `notification` 实时事件；
//     离线用户不补偿（持久化行保留，客户端下次拉取；离线 Web Push 属下一阶段）。
//   - 读取/生命周期：未读计数、列表、单条/全部已读、删除；过期行由清理 worker
//     定期删除（cmd/chatd/main.go 的清理循环）。
type NotificationService struct {
	*Service
}

// kind 常量：occurrence 类型（与迁移 005 的 kind 列对应）。
const (
	NotificationKindMention = "mention"
	NotificationKindReply   = "reply"
	NotificationKindSystem  = "system"
)

// CreateForMessage 在一条消息写入后触发收件通知：
//   - 直接把收件人 @ 到的用户（排除发送者）→ kind=mention；
//   - 回复的那条消息的作者（排除发送者、排除回复自身）→ kind=reply。
//
// 任一单条失败只记日志不中断（通知是尽力而为，不能拖垮消息发送）。
func (s *NotificationService) CreateForMessage(ctx context.Context, chatID, senderID string, mentions []string, replyTo []string, msg *models.Message) error {
	chat, err := s.db.GetChat(ctx, chatID)
	if err != nil {
		// 聊天不存在时通知无从标 title；消息本身都已写入，直接放弃。
		logutil.Warn("notification: chat %s lookup: %v", logutil.SafeID(chatID), err)
		return nil
	}
	title := chat.Name
	if title == "" {
		title = "新消息"
	}
	body := snippet(msg.Content, notificationBodySnippetLen)

	for _, mentionID := range mentions {
		if mentionID == "" || mentionID == senderID {
			continue
		}
		if member, err := s.db.IsChatMember(ctx, chatID, mentionID); err != nil || !member {
			continue // 非成员（如已被移出）不通知
		}
		s.trigger(ctx, mentionID, NotificationKindMention, chatID, msg.ID, senderID, title, body)
	}
	for _, replyToID := range replyTo {
		if replyToID == "" {
			continue
		}
		replied, err := s.db.GetMessage(ctx, replyToID)
		if err != nil || replied.ChatID != chatID {
			continue
		}
		if replied.UserID == "" || replied.UserID == senderID || replied.ID == msg.ID {
			continue
		}
		s.trigger(ctx, replied.UserID, NotificationKindReply, chatID, msg.ID, senderID, title, body)
	}
	return nil
}

// trigger 创建单条 occurrence：撞唯一索引（同源事件已存在）时静默跳过；
// 新创建且收件人在线时立即广播实时事件。
func (s *NotificationService) trigger(ctx context.Context, recipientID, kind, chatID, messageID, actorID, title, body string) {
	created, err := s.db.CreateNotificationOccurrence(ctx, recipientID, kind, chatID, messageID, actorID, title, body, time.Now())
	if err != nil {
		logutil.Warn("notification: create occurrence for %s: %v", logutil.SafeID(recipientID), err)
		return
	}
	if !created {
		return // 同源事件已存在，不重复插行、不重置已读
	}
	// 投递分流：在线 → 实时广播；离线 → Web Push（有订阅才发）。
	// 移植 chatto 的「在线实时 / 离线推送」双通道语义；Push 未配置时
	// PushForOfflineUser 内部直接跳过，不影响在线广播。
	if s.hub != nil && s.hub.IsOnline(recipientID) {
		occ, err := s.db.GetNotificationOccurrenceByKey(ctx, recipientID, kind, chatID, messageID)
		if err == nil && occ != nil {
			s.hub.BroadcastNotification(recipientID, occ)
		}
		return
	}
	if s.Push != nil {
		s.Push.PushForOfflineUser(ctx, recipientID, title, body)
	}
}

// List 返回用户最近的持久化通知（新→旧，分页游标 before=created_at）。
func (s *NotificationService) List(ctx context.Context, userID, before string, limit int) ([]models.NotificationOccurrence, error) {
	return s.db.ListNotificationOccurrences(ctx, userID, before, limit)
}

// UnreadCount 返回未读通知数。
func (s *NotificationService) UnreadCount(ctx context.Context, userID string) (int, error) {
	return s.db.CountUnreadNotificationOccurrences(ctx, userID)
}

// MarkRead 标记单条已读（仅 owner）。
func (s *NotificationService) MarkRead(ctx context.Context, id, userID string) error {
	return s.db.MarkNotificationOccurrenceRead(ctx, id, userID)
}

// MarkAllRead 标记该用户全部已读。
func (s *NotificationService) MarkAllRead(ctx context.Context, userID string) error {
	return s.db.MarkAllNotificationOccurrencesRead(ctx, userID)
}

// Delete 删除单条（仅 owner）。
func (s *NotificationService) Delete(ctx context.Context, id, userID string) error {
	return s.db.DeleteNotificationOccurrence(ctx, id, userID)
}

// PruneExpired 删除全部已过期通知（清理 worker 调用）。
func (s *NotificationService) PruneExpired(ctx context.Context) (int64, error) {
	return s.db.PruneExpiredNotificationOccurrences(ctx, time.Now())
}

func snippet(text string, max int) string {
	text = strings.TrimSpace(text)
	if len(text) <= max {
		return text
	}
	return text[:max] + "…"
}
