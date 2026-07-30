package service

import (
	"context"

	"github.com/Hana-ame/chat-app/server/internal/models"
)

type MemberService struct {
	*Service
}

func (s *MemberService) List(ctx context.Context, chatID, userID string) ([]models.User, error) {
	if err := s.Authz.MustBeMember(ctx, chatID, userID); err != nil {
		return nil, err
	}
	return s.db.GetChatMembers(ctx, chatID)
}

func (s *MemberService) Add(ctx context.Context, chatID, userID, targetID string) (*models.Chat, error) {
	chat, err := s.db.GetChat(ctx, chatID)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if chat.Type == "dm" {
		return nil, ErrInvalidInput
	}
	if err := s.Authz.MustBeMember(ctx, chatID, userID); err != nil {
		return nil, err
	}
	if _, err := s.db.GetUserByID(ctx, targetID); err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := s.db.AddChatMember(ctx, chatID, targetID); err != nil {
		if isConflict(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	updated, err := s.db.GetChat(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if s.hub != nil {
		s.hub.BroadcastChatUpdated(updated)
		s.hub.NotifyUserNewChat(targetID, updated)
	}
	return updated, nil
}

func (s *MemberService) Remove(ctx context.Context, chatID, userID, targetID string) error {
	chat, err := s.db.GetChat(ctx, chatID)
	if err != nil {
		if isNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if chat.Type == "dm" {
		return ErrInvalidInput
	}
	if targetID == chat.OwnerID && targetID != userID {
		return ErrForbidden
	}
	if targetID != userID {
		if err := s.Authz.RequireOwnerOrAdmin(ctx, chatID, userID); err != nil {
			return err
		}
	}
	if err := s.db.RemoveChatMember(ctx, chatID, targetID); err != nil {
		return err
	}
	if s.hub != nil {
		s.hub.NotifyUserLeftChat(targetID, chatID)
		if updated, err := s.db.GetChat(ctx, chatID); err == nil {
			s.hub.BroadcastChatUpdated(updated)
		}
	}
	return nil
}
