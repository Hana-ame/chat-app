package service

import (
	"context"
	"strings"

	"github.com/Hana-ame/chat-app/server/internal/models"
)

type ChatService struct {
	*Service
}

func (s *ChatService) ListForUser(ctx context.Context, userID string) ([]models.Chat, error) {
	return s.DB.ListUserChats(ctx, userID)
}

func (s *ChatService) GetByID(ctx context.Context, chatID, userID string) (*models.Chat, error) {
	if err := s.MustBeMember(ctx, chatID, userID); err != nil {
		return nil, err
	}
	chat, err := s.DB.GetChat(ctx, chatID)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return chat, nil
}

func (s *ChatService) Create(ctx context.Context, userID, name, visibility string, memberIDs []string) (*models.Chat, error) {
	if strings.TrimSpace(name) == "" {
		return nil, ErrInvalidInput
	}
	members := memberIDs
	hasMe := false
	for _, m := range members {
		if m == userID {
			hasMe = true
			break
		}
	}
	if !hasMe {
		members = append(members, userID)
	}
	chat, err := s.DB.CreateChat(ctx, "group", name, visibility, userID, members)
	if err != nil {
		return nil, err
	}
	if s.Hub != nil {
		s.Hub.BroadcastChatCreated(chat)
	}
	return chat, nil
}

func (s *ChatService) CreateOrGetDM(ctx context.Context, userID, otherUserID string) (*models.Chat, bool, error) {
	if _, err := s.DB.GetUserByID(ctx, otherUserID); err != nil {
		if isNotFound(err) {
			return nil, false, ErrNotFound
		}
		return nil, false, err
	}
	if dm, err := s.DB.FindDMBetween(ctx, userID, otherUserID); err == nil {
		return dm, true, nil
	}
	chat, err := s.DB.CreateChat(ctx, "dm", "", "", "", []string{userID, otherUserID})
	if err != nil {
		return nil, false, err
	}
	if s.Hub != nil {
		s.Hub.BroadcastChatCreated(chat)
	}
	return chat, false, nil
}

func (s *ChatService) Rename(ctx context.Context, chatID, userID, name string) (*models.Chat, error) {
	chat, err := s.DB.GetChat(ctx, chatID)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if chat.Type == "dm" {
		return nil, ErrInvalidInput
	}
	if chat.OwnerID != userID {
		return nil, ErrForbidden
	}
	if err := s.DB.RenameChat(ctx, chatID, name); err != nil {
		return nil, err
	}
	updated, err := s.DB.GetChat(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if s.Hub != nil {
		s.Hub.BroadcastChatUpdated(updated)
	}
	return updated, nil
}

func (s *ChatService) Delete(ctx context.Context, chatID, userID string) error {
	chat, err := s.DB.GetChat(ctx, chatID)
	if err != nil {
		if isNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if chat.Type == "dm" {
		return ErrInvalidInput
	}
	if chat.OwnerID != userID {
		return ErrForbidden
	}
	if err := s.DB.DeleteChat(ctx, chatID); err != nil {
		return err
	}
	if s.Hub != nil {
		s.Hub.BroadcastChatDeleted(chat, chatID)
	}
	return nil
}

func (s *ChatService) ListPublic(ctx context.Context, page, limit int) ([]models.Chat, error) {
	return s.DB.ListPublicChats(ctx, page, limit)
}

func (s *ChatService) Join(ctx context.Context, chatID, userID string) (*models.Chat, error) {
	if err := s.DB.JoinChatByID(ctx, chatID, userID); err != nil {
		return nil, err
	}
	chat, err := s.DB.GetChat(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if s.Hub != nil {
		s.Hub.NotifyUserNewChat(userID, chat)
		s.Hub.BroadcastChatUpdated(chat)
	}
	return chat, nil
}

func (s *ChatService) SetAnnouncement(ctx context.Context, chatID, userID, content string) error {
	if err := s.RequireOwnerOrAdmin(ctx, chatID, userID); err != nil {
		return err
	}
	n, err := s.DB.ChatMemberCount(ctx, chatID)
	if err != nil {
		return err
	}
	if n < 3 {
		return ErrInvalidInput
	}
	if err := s.DB.SetPinnedMessage(ctx, chatID, content); err != nil {
		return err
	}
	if s.Hub != nil {
		if updated, err := s.DB.GetChat(ctx, chatID); err == nil {
			s.Hub.BroadcastChatUpdated(updated)
		}
	}
	return nil
}

func (s *ChatService) ClearAnnouncement(ctx context.Context, chatID, userID string) error {
	if err := s.RequireOwnerOrAdmin(ctx, chatID, userID); err != nil {
		return err
	}
	if err := s.DB.ClearPinnedMessage(ctx, chatID); err != nil {
		return err
	}
	if s.Hub != nil {
		if updated, err := s.DB.GetChat(ctx, chatID); err == nil {
			s.Hub.BroadcastChatUpdated(updated)
		}
	}
	return nil
}

func (s *ChatService) MarkAnnouncementRead(ctx context.Context, chatID, userID string) error {
	return s.DB.UpdatePinnedLastReadAt(ctx, chatID, userID)
}

func (s *ChatService) SetPinned(ctx context.Context, chatID, userID string, pinned bool) error {
	if err := s.MustBeMember(ctx, chatID, userID); err != nil {
		return err
	}
	if err := s.DB.SetPinned(ctx, chatID, userID, pinned); err != nil {
		return err
	}
	if s.Hub != nil {
		if updated, err := s.DB.GetChat(ctx, chatID); err == nil {
			s.Hub.BroadcastChatUpdated(updated)
		}
	}
	return nil
}

func (s *ChatService) Visit(ctx context.Context, chatID, userID string) error {
	return s.DB.UpdateLastActiveAt(ctx, chatID, userID)
}

func (s *ChatService) MarkRead(ctx context.Context, chatID, userID string) error {
	if err := s.MustBeMember(ctx, chatID, userID); err != nil {
		return err
	}
	return s.DB.UpdateLastActiveAt(ctx, chatID, userID)
}
