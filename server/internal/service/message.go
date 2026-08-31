package service

import (
	"context"
	"regexp"
	"strings"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

type MessageService struct {
	*Service
}

func (s *MessageService) List(ctx context.Context, chatID, userID string, before string, limit int) ([]models.Message, error) {
	if err := s.Authz.MustBeMember(ctx, chatID, userID); err != nil {
		return nil, err
	}
	return s.db.GetMessages(ctx, chatID, before, limit)
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

func (s *MessageService) Send(ctx context.Context, chatID, userID, content string, attachments []models.Attachment, replyTo ...string) (*models.Message, error) {
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
		if !strings.Contains(a.URL, "/api/local/") {
			return nil, ErrInvalidInput
		}
		if a.MimeType == "" {
			attachments[i].MimeType = "application/octet-stream"
		}
	}
	mentions := extractMentions(content)
	msg, err := s.db.CreateMessage(ctx, chatID, userID, content, mentions, attachments, replyTo...)
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
		if err := s.Notification.CreateForMessage(ctx, chatID, userID, mentions, replyTo, msg); err != nil {
			logutil.Warn("notification trigger: %v", err)
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
