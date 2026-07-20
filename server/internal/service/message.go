package service

import (
	"context"
	"regexp"
	"strings"

	"github.com/Hana-ame/chat-app/server/internal/models"
)

type MessageService struct {
	*Service
}

func (s *MessageService) List(ctx context.Context, chatID, userID string, before string, limit int) ([]models.Message, error) {
	if err := s.Authz.MustBeMember(ctx, chatID, userID); err != nil {
		return nil, err
	}
	return s.DB.GetMessages(ctx, chatID, before, limit)
}

func (s *MessageService) Send(ctx context.Context, chatID, userID, content string, attachments []models.Attachment) (*models.Message, error) {
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
		if !strings.HasPrefix(a.URL, "https://upload.moonchan.xyz/") {
			return nil, ErrInvalidInput
		}
		if a.MimeType == "" {
			attachments[i].MimeType = "application/octet-stream"
		}
	}
	mentions := extractMentions(content)
	msg, err := s.DB.CreateMessage(ctx, chatID, userID, content, mentions, attachments)
	if err != nil {
		if isContentTooLong(err) {
			return nil, ErrContentTooLong
		}
		return nil, err
	}
	if s.Hub != nil {
		s.Hub.BroadcastMessageCreate(msg)
	}
	return msg, nil
}

func (s *MessageService) Edit(ctx context.Context, chatID, messageID, userID, content string) (*models.Message, error) {
	if err := s.Authz.MustBeMember(ctx, chatID, userID); err != nil {
		return nil, err
	}
	msg, err := s.DB.UpdateMessage(ctx, messageID, userID, content)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if msg.ChatID != chatID {
		return nil, ErrInvalidInput
	}
	if s.Hub != nil {
		s.Hub.BroadcastMessageUpdate(msg)
	}
	return msg, nil
}

func (s *MessageService) Delete(ctx context.Context, chatID, messageID, userID string) error {
	if err := s.Authz.MustBeMember(ctx, chatID, userID); err != nil {
		return err
	}
	existing, err := s.DB.GetMessage(ctx, messageID)
	if err != nil {
		return err
	}
	if existing.ChatID != chatID {
		return ErrInvalidInput
	}
	canDeleteAny := false
	if chat, err := s.DB.GetChat(ctx, chatID); err == nil {
		canDeleteAny = chat.OwnerID == userID || s.Authz.RequireOwnerOrAdmin(ctx, chatID, userID) == nil
	}
	if existing.UserID != userID && !canDeleteAny {
		return ErrForbidden
	}
	if err := s.DB.DeleteMessage(ctx, messageID, userID, canDeleteAny); err != nil {
		return err
	}
	if s.Hub != nil {
		s.Hub.BroadcastMessageDelete(chatID, messageID)
	}
	return nil
}

func (s *MessageService) MarkRead(ctx context.Context, chatID, userID string) error {
	if err := s.Authz.MustBeMember(ctx, chatID, userID); err != nil {
		return err
	}
	return s.DB.UpdateLastActiveAt(ctx, chatID, userID)
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
