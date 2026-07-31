package service

import (
	"context"
	"strings"
	"sync"

	"github.com/Hana-ame/chat-app/server/internal/models"
)

type ChatService struct {
	*Service
	dmMu sync.Mutex
}

func (s *ChatService) ListForUser(ctx context.Context, userID string) ([]models.Chat, error) {
	return s.db.ListUserChats(ctx, userID)
}

func (s *ChatService) GetByID(ctx context.Context, chatID, userID string) (*models.Chat, error) {
	if err := s.Authz.MustBeMember(ctx, chatID, userID); err != nil {
		return nil, err
	}
	chat, err := s.db.GetChat(ctx, chatID)
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
	chat, err := s.db.CreateChat(ctx, "group", name, visibility, userID, members)
	if err != nil {
		return nil, err
	}
	if s.hub != nil {
		s.hub.BroadcastChatCreated(chat)
	}
	return chat, nil
}

func (s *ChatService) CreateOrGetNotificationsChat(ctx context.Context, userID string) (*models.Chat, error) {
	if chat, err := s.db.FindNotificationsChat(ctx, userID); err == nil {
		return chat, nil
	}
	chat, err := s.db.CreateNotificationsChat(ctx, userID)
	if err != nil {
		return nil, err
	}
	if s.hub != nil {
		s.hub.BroadcastChatCreated(chat)
	}
	return chat, nil
}

func (s *ChatService) CreateOrGetDM(ctx context.Context, userID, otherUserID string) (*models.Chat, bool, error) {
	if _, err := s.db.GetUserByID(ctx, otherUserID); err != nil {
		if isNotFound(err) {
			return nil, false, ErrNotFound
		}
		return nil, false, err
	}
	// Serialize find-or-create: two concurrent requests would otherwise both
	// miss FindDMBetween and create duplicate DM chats.
	s.dmMu.Lock()
	defer s.dmMu.Unlock()
	if dm, err := s.db.FindDMBetween(ctx, userID, otherUserID); err == nil {
		return dm, true, nil
	}
	chat, err := s.db.CreateChat(ctx, "dm", "", "", "", []string{userID, otherUserID})
	if err != nil {
		return nil, false, err
	}
	if s.hub != nil {
		s.hub.BroadcastChatCreated(chat)
	}
	return chat, false, nil
}

func (s *ChatService) Rename(ctx context.Context, chatID, userID, name string) (*models.Chat, error) {
	chat, err := s.db.GetChat(ctx, chatID)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if chat.Type == "dm" || chat.Type == "notify" {
		return nil, ErrInvalidInput
	}
	if chat.OwnerID != userID {
		return nil, ErrForbidden
	}
	if err := s.db.RenameChat(ctx, chatID, name); err != nil {
		return nil, err
	}
	updated, err := s.db.GetChat(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if s.hub != nil {
		s.hub.BroadcastChatUpdated(updated)
	}
	return updated, nil
}

func (s *ChatService) UpdateAvatar(ctx context.Context, chatID, userID, avatarURL string) (*models.Chat, error) {
	chat, err := s.db.GetChat(ctx, chatID)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if chat.Type == "dm" || chat.Type == "notify" {
		return nil, ErrInvalidInput
	}
	if chat.OwnerID != userID {
		return nil, ErrForbidden
	}
	if err := s.db.UpdateChatAvatar(ctx, chatID, avatarURL); err != nil {
		return nil, err
	}
	updated, err := s.db.GetChat(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if s.hub != nil {
		s.hub.BroadcastChatUpdated(updated)
	}
	return updated, nil
}

func (s *ChatService) UpdateBanner(ctx context.Context, chatID, userID, bannerURL string, opacity float64) (*models.Chat, error) {
	chat, err := s.db.GetChat(ctx, chatID)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if chat.Type == "dm" || chat.Type == "notify" {
		return nil, ErrInvalidInput
	}
	if chat.OwnerID != userID {
		return nil, ErrForbidden
	}
	if err := s.db.UpdateChatBanner(ctx, chatID, bannerURL); err != nil {
		return nil, err
	}
	if opacity > 0 {
		if err := s.db.UpdateChatBannerOpacity(ctx, chatID, opacity); err != nil {
			return nil, err
		}
	}
	updated, err := s.db.GetChat(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if s.hub != nil {
		s.hub.BroadcastChatUpdated(updated)
	}
	return updated, nil
}

func (s *ChatService) UpdateBackground(ctx context.Context, chatID, userID, backgroundURL string) (*models.Chat, error) {
	chat, err := s.db.GetChat(ctx, chatID)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if chat.Type == "dm" || chat.Type == "notify" {
		return nil, ErrInvalidInput
	}
	if chat.OwnerID != userID {
		return nil, ErrForbidden
	}
	if err := s.db.UpdateChatBackground(ctx, chatID, backgroundURL); err != nil {
		return nil, err
	}
	updated, err := s.db.GetChat(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if s.hub != nil {
		s.hub.BroadcastChatUpdated(updated)
	}
	return updated, nil
}

func (s *ChatService) Delete(ctx context.Context, chatID, userID string) error {
	chat, err := s.db.GetChat(ctx, chatID)
	if err != nil {
		if isNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if chat.Type == "dm" || chat.Type == "notify" {
		return ErrInvalidInput
	}
	if chat.OwnerID != userID {
		return ErrForbidden
	}
	if err := s.db.DeleteChat(ctx, chatID); err != nil {
		return err
	}
	if s.hub != nil {
		s.hub.BroadcastChatDeleted(chat, chatID)
	}
	return nil
}

func (s *ChatService) ListPublic(ctx context.Context, page, limit int) ([]models.Chat, error) {
	return s.db.ListPublicChats(ctx, page, limit)
}

func (s *ChatService) Join(ctx context.Context, chatID, userID string) (*models.Chat, error) {
	if err := s.db.JoinChatByID(ctx, chatID, userID); err != nil {
		return nil, err
	}
	chat, err := s.db.GetChat(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if s.hub != nil {
		s.hub.NotifyUserNewChat(userID, chat)
		s.hub.BroadcastChatUpdated(chat)
	}
	return chat, nil
}

func (s *ChatService) SetAnnouncement(ctx context.Context, chatID, userID, content string) error {
	if err := s.Authz.RequireOwnerOrAdmin(ctx, chatID, userID); err != nil {
		return err
	}
	if err := s.db.SetPinnedMessage(ctx, chatID, content); err != nil {
		return err
	}
	if s.hub != nil {
		if updated, err := s.db.GetChat(ctx, chatID); err == nil {
			s.hub.BroadcastChatUpdated(updated)
		}
	}
	return nil
}

func (s *ChatService) ClearAnnouncement(ctx context.Context, chatID, userID string) error {
	if err := s.Authz.RequireOwnerOrAdmin(ctx, chatID, userID); err != nil {
		return err
	}
	if err := s.db.ClearPinnedMessage(ctx, chatID); err != nil {
		return err
	}
	if s.hub != nil {
		if updated, err := s.db.GetChat(ctx, chatID); err == nil {
			s.hub.BroadcastChatUpdated(updated)
		}
	}
	return nil
}

func (s *ChatService) MarkAnnouncementRead(ctx context.Context, chatID, userID string) error {
	return s.db.UpdatePinnedLastReadAt(ctx, chatID, userID)
}

func (s *ChatService) SetPinned(ctx context.Context, chatID, userID string, pinned bool) error {
	if err := s.Authz.MustBeMember(ctx, chatID, userID); err != nil {
		return err
	}
	if err := s.db.SetPinned(ctx, chatID, userID, pinned); err != nil {
		return err
	}
	if s.hub != nil {
		if updated, err := s.db.GetChat(ctx, chatID); err == nil {
			s.hub.BroadcastChatUpdated(updated)
		}
	}
	return nil
}

func (s *ChatService) SetChatNotifyEnabled(ctx context.Context, chatID, userID string, enabled bool) error {
	if err := s.Authz.MustBeMember(ctx, chatID, userID); err != nil {
		return err
	}
	return s.db.SetChatNotifyEnabled(ctx, chatID, userID, enabled)
}

func (s *ChatService) Visit(ctx context.Context, chatID, userID string) error {
	return s.db.UpdateLastActiveAt(ctx, chatID, userID)
}

func (s *ChatService) MarkRead(ctx context.Context, chatID, userID string) error {
	if err := s.Authz.MustBeMember(ctx, chatID, userID); err != nil {
		return err
	}
	return s.db.UpdateLastActiveAt(ctx, chatID, userID)
}
