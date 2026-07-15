package service

import (
	"context"

	"github.com/Hana-ame/chat-app/server/internal/models"
)

type ReactionService struct {
	*Service
}

func (s *ReactionService) Add(ctx context.Context, chatID, messageID, userID, emoji string) (*models.Message, error) {
	if err := s.Chat.MustBeMember(ctx, chatID, userID); err != nil {
		return nil, err
	}
	msg, err := s.DB.GetMessage(ctx, messageID)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if msg.ChatID != chatID {
		return nil, ErrNotFound
	}
	if err := s.DB.AddReaction(ctx, messageID, userID, emoji); err != nil {
		return nil, err
	}
	if s.Hub != nil {
		s.Hub.BroadcastReaction(chatID, messageID, emoji, userID, true)
	}
	updated, err := s.DB.GetMessage(ctx, messageID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *ReactionService) Remove(ctx context.Context, chatID, messageID, userID, emoji string) (*models.Message, error) {
	if err := s.Chat.MustBeMember(ctx, chatID, userID); err != nil {
		return nil, err
	}
	if err := s.DB.RemoveReaction(ctx, messageID, userID, emoji); err != nil {
		return nil, err
	}
	if s.Hub != nil {
		s.Hub.BroadcastReaction(chatID, messageID, emoji, userID, false)
	}
	updated, err := s.DB.GetMessage(ctx, messageID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *ReactionService) List(ctx context.Context, chatID, messageID, viewerID string) ([]models.Reaction, error) {
	if err := s.Chat.MustBeMember(ctx, chatID, viewerID); err != nil {
		return nil, err
	}
	return s.DB.ListReactions(ctx, messageID, viewerID)
}
