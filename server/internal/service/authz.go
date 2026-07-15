package service

import (
	"context"
	"errors"
	"strings"

	"github.com/Hana-ame/chat-app/server/internal/db"
)

func (s *ChatService) MustBeMember(ctx context.Context, chatID, userID string) error {
	if userID == "" {
		return ErrForbidden
	}
	ok, err := s.DB.IsChatMember(ctx, chatID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

func (s *ChatService) RequireOwnerOrAdmin(ctx context.Context, chatID, userID string) error {
	chat, err := s.DB.GetChat(ctx, chatID)
	if err != nil {
		if isNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if chat.OwnerID == userID {
		return nil
	}
	role, err := s.DB.GetChatMemberRole(ctx, chatID, userID)
	if err != nil {
		if isNotFound(err) {
			return ErrForbidden
		}
		return err
	}
	if role == "admin" {
		return nil
	}
	return ErrForbidden
}

func isNotFound(err error) bool {
	return errors.Is(err, db.ErrNotFound)
}

func isConflict(err error) bool {
	return errors.Is(err, db.ErrConflict)
}

func isContentTooLong(err error) bool {
	return err != nil && strings.Contains(err.Error(), "content too long")
}
