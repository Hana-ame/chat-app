package service

import (
	"context"

	"github.com/Hana-ame/chat-app/server/internal/models"
)

type ReactionService struct {
	*Service
}

func (s *ReactionService) Add(ctx context.Context, chatID, messageID, userID, emoji string) (*models.Message, error) {
	if err := s.Authz.MustBeMember(ctx, chatID, userID); err != nil {
		return nil, err
	}
	msg, err := s.db.GetMessage(ctx, messageID)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if msg.ChatID != chatID {
		return nil, ErrNotFound
	}
	if err := s.db.AddReaction(ctx, messageID, userID, emoji); err != nil {
		return nil, err
	}
	if s.hub != nil {
		s.hub.BroadcastReaction(chatID, messageID, emoji, userID, true)
	}
	updated, err := s.db.GetMessage(ctx, messageID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *ReactionService) Remove(ctx context.Context, chatID, messageID, userID, emoji string) (*models.Message, error) {
	if err := s.Authz.MustBeMember(ctx, chatID, userID); err != nil {
		return nil, err
	}
	if err := s.db.RemoveReaction(ctx, messageID, userID, emoji); err != nil {
		return nil, err
	}
	if s.hub != nil {
		s.hub.BroadcastReaction(chatID, messageID, emoji, userID, false)
	}
	updated, err := s.db.GetMessage(ctx, messageID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *ReactionService) List(ctx context.Context, chatID, messageID, viewerID string) ([]models.Reaction, error) {
	if err := s.Authz.MustBeMember(ctx, chatID, viewerID); err != nil {
		return nil, err
	}
	return s.db.ListReactions(ctx, messageID, viewerID)
}
