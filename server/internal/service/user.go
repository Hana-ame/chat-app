package service

import (
	"context"
	"errors"

	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

type UserService struct {
	*Service
}

func (s *UserService) GetByID(ctx context.Context, id string) (*models.User, error) {
	u, err := s.db.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

func (s *UserService) GetByEmail(ctx context.Context, email string) (*models.User, string, error) {
	u, hash, err := s.db.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, "", ErrNotFound
		}
		return nil, "", err
	}
	return u, hash, nil
}

func (s *UserService) Create(ctx context.Context, email, username, hash string) (*models.User, error) {
	u, err := s.db.CreateUser(ctx, email, username, hash)
	if err != nil {
		if errors.Is(err, db.ErrConflict) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return u, nil
}

func (s *UserService) UpdateProfile(ctx context.Context, id, username, avatarColor, avatarURL string) (*models.User, error) {
	u, err := s.db.UpdateUserProfile(ctx, id, username, avatarColor, avatarURL)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, ErrNotFound
		}
		if errors.Is(err, db.ErrConflict) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return u, nil
}

func (s *UserService) UpdateNotifyBlocked(ctx context.Context, userID string, blocked []string) (*models.User, error) {
	if blocked == nil {
		blocked = []string{}
	}
	if err := s.db.SetUserNotifyBlocked(ctx, userID, blocked); err != nil {
		return nil, err
	}
	return s.db.GetUserByID(ctx, userID)
}

func (s *UserService) Search(ctx context.Context, query string, limit int) ([]models.User, error) {
	return s.db.SearchUsers(ctx, query, limit)
}
