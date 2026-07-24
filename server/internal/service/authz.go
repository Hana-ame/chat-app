package service

import (
	"context"
	"errors"

	"github.com/Hana-ame/chat-app/server/internal/db"
)

type Authz struct {
	DB *db.DB
}

func (a *Authz) MustBeMember(ctx context.Context, chatID, userID string) error {
	if userID == "" {
		return ErrForbidden
	}
	ok, err := a.DB.IsChatMember(ctx, chatID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

func (a *Authz) RequireOwnerOrAdmin(ctx context.Context, chatID, userID string) error {
	chat, err := a.DB.GetChat(ctx, chatID)
	if err != nil {
		if isNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if chat.OwnerID == userID {
		return nil
	}
	role, err := a.DB.GetChatMemberRole(ctx, chatID, userID)
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
	return errors.Is(err, db.ErrContentTooLong)
}
