package service

import (
	"context"
	"regexp"
	"strings"

	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

type MessageService struct {
	*Service
}

func (s *MessageService) List(ctx context.Context, chatID, userID string, before string, limit int, inThread ...string) ([]models.Message, error) {
	if err := s.Authz.MustBeMember(ctx, chatID, userID); err != nil {
		return nil, err
	}
	tf := ""
	if len(inThread) > 0 {
		tf = inThread[0]
	}
	return s.db.GetMessages(ctx, chatID, before, limit, tf)
}

func (s *MessageService) SendAI(ctx context.Context, chatID, userID, content, thinking, msgID string, author *models.User) (*models.Message, error) {
	if strings.TrimSpace(content) == "" && strings.TrimSpace(thinking) == "" {
		return nil, ErrInvalidInput
	}
	msg, err := s.db.CreateAIMessage(ctx, chatID, userID, msgID, content, thinking)
	if err != nil {
		return nil, err
	}
	msg.Author = author
	if s.hub != nil {
		s.hub.BroadcastMessageCreate(msg)
	}
	return msg, nil
}

func (s *MessageService) Send(ctx context.Context, chatID, userID, content string, attachments []models.Attachment, replyTo, explicitThreadRoot string, startThread bool) (*models.Message, error) {
	if err := s.Authz.MustBeMember(ctx, chatID, userID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(content) == "" && len(attachments) == 0 {
		return nil, ErrInvalidInput
	}
	for i, a := range attachments {
		if a.URL == "" || a.Filename == "" {
			return nil, ErrInvalidInput
		}
		// 【本地改动 2026-09-02】URL 校验：接受旧 /api/local/ 与新 /assets/files/
		// 两种模式（向后兼容 + 新公开 URL）。拒绝其他域/路径，防止外部 URL 注入。
		if a.URL != "" && !strings.Contains(a.URL, "/api/local/") && !strings.Contains(a.URL, "/assets/files/") {
			return nil, ErrInvalidInput
		}
		if a.MimeType == "" {
			attachments[i].MimeType = "application/octet-stream"
		}
	}
	mentions := extractMentions(content)

	// 【本地改动 2026-08-31】线程根计算语义：
	// startThread → 该消息自身为根；显式 thread_root → 使用给定值；reply_to
	// 给定 → 继承父消息的 thread_root；父无根 → 该消息成新根。根 = 消息 ID
	// 自身（自引用）；非根线程回复指向根消息。
	threadRoot := explicitThreadRoot
	if startThread {
		threadRoot = "__SELF__"
	} else if replyTo != "" {
		if threadRoot == "" {
			parent, err := s.db.GetMessage(ctx, replyTo)
			if err == nil && parent.ThreadRootMessageID != "" {
				threadRoot = parent.ThreadRootMessageID
			} else {
				// 父消息不在任何线程内 → 本消息成为新线程根
				threadRoot = "__SELF__"
			}
		}
	}
	msg, err := s.db.CreateMessage(ctx, chatID, userID, content, mentions, attachments,
		db.WithReplyTo(replyTo),
		db.WithThreadRoot(threadRoot),
	)
	if err != nil {
		if isContentTooLong(err) {
			return nil, ErrContentTooLong
		}
		return nil, err
	}
	if s.hub != nil {
		s.hub.BroadcastMessageCreate(msg)
	}
	// 【本地改动 2026-08-31】持久化通知触发：提及 + 回复。尽力而为，
	// 失败只记日志，绝不拖垮消息发送。
	if s.Notification != nil {
		replyToSlice := []string{}
		if replyTo != "" {
			replyToSlice = []string{replyTo}
		}
		if err := s.Notification.CreateForMessage(ctx, chatID, userID, mentions, replyToSlice, msg); err != nil {
			logutil.Warn("notification trigger: %v", err)
		}
		// 【本地改动 2026-08-31】线程回复通知：向该线程的所有关注者（除作者本人）
		// 发 reply_in_thread 通知。
		// thread notifications 语义对齐。尽力而为。
		if msg.ThreadRootMessageID != "" {
			if err := s.notifyThreadWatchers(ctx, chatID, userID, msg); err != nil {
				logutil.Warn("thread follow notification: %v", err)
			}
		}
	}
	return msg, nil
}

func (s *MessageService) Edit(ctx context.Context, chatID, messageID, userID, content string) (*models.Message, error) {
	if err := s.Authz.MustBeMember(ctx, chatID, userID); err != nil {
		return nil, err
	}
	// Validate that the message belongs to the requested chat BEFORE any
	// write: UpdateMessage only filters by message ID and author, so a
	// mismatched chatID would otherwise silently edit a message in another
	// chat the caller is also a member of.
	existing, err := s.db.GetMessage(ctx, messageID)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if existing.ChatID != chatID {
		return nil, ErrInvalidInput
	}
	msg, err := s.db.UpdateMessage(ctx, messageID, userID, content)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if s.hub != nil {
		s.hub.BroadcastMessageUpdate(msg)
	}
	return msg, nil
}

func (s *MessageService) Delete(ctx context.Context, chatID, messageID, userID string) error {
	if err := s.Authz.MustBeMember(ctx, chatID, userID); err != nil {
		return err
	}
	existing, err := s.db.GetMessage(ctx, messageID)
	if err != nil {
		return err
	}
	if existing.ChatID != chatID {
		return ErrInvalidInput
	}
	canDeleteAny := false
	if chat, err := s.db.GetChat(ctx, chatID); err == nil {
		canDeleteAny = chat.OwnerID == userID || s.Authz.RequireOwnerOrAdmin(ctx, chatID, userID) == nil
	}
	if existing.UserID != userID && !canDeleteAny {
		return ErrForbidden
	}
	if err := s.db.DeleteMessage(ctx, messageID, userID, canDeleteAny); err != nil {
		return err
	}
	if s.hub != nil {
		s.hub.BroadcastMessageDelete(chatID, messageID)
	}
	return nil
}

func (s *MessageService) MarkRead(ctx context.Context, chatID, userID string) error {
	if err := s.Authz.MustBeMember(ctx, chatID, userID); err != nil {
		return err
	}
	return s.db.UpdateLastActiveAt(ctx, chatID, userID)
}

var mentionRegex = regexp.MustCompile(`<@([a-f0-9-]{36})>`)

func extractMentions(content string) []string {
	matches := mentionRegex.FindAllStringSubmatch(content, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

// 【本地改动 2026-08-31】线程回复通知触发：遍历线程关注者，除作者外每人触发一条
// reply_in_thread 通知（含 Web Push 落库/离线投递，见 notification.trigger）。
func (s *MessageService) notifyThreadWatchers(ctx context.Context, chatID, authorID string, msg *models.Message) error {
	if s.db == nil || s.Notification == nil {
		return nil
	}
	followers, err := s.db.ThreadWatchers(ctx, msg.ThreadRootMessageID)
	if err != nil {
		return err
	}
	if len(followers) == 0 {
		return nil
	}
	root, err := s.db.GetMessage(ctx, msg.ThreadRootMessageID)
	if err != nil {
		return err
	}
	for _, followerID := range followers {
		if followerID == authorID {
			continue
		}
		// 标题：「{作者} 在 {根内容前 30 字}」；正文：本回复内容截断。
		rootSnippet := truncateString(root.Content, 30)
		title := truncateString(msg.Content, 80)
		body := "在 " + rootSnippet
		s.Notification.trigger(ctx, followerID, "reply_in_thread", chatID, msg.ID, authorID, title, body)
	}
	return nil
}

func truncateString(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
