package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

func (d *DB) GetChatMembers(ctx context.Context, chatID string) ([]models.User, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT u.id, u.username, u.avatar_color, u.avatar_url, u.status, u.last_seen, u.created_at, cm.role
		 FROM chat_members cm JOIN users u ON u.id = cm.user_id
		 WHERE cm.chat_id = ?
		 ORDER BY u.username`,
		chatID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.User{}
	for rows.Next() {
		var u models.User
		var lastSeen, created string
		if err := rows.Scan(&u.ID, &u.Username, &u.AvatarColor, &u.AvatarURL, &u.Status, &lastSeen, &created, &u.Role); err != nil {
			return nil, err
		}
		u.LastSeen = parseTime(lastSeen)
		u.CreatedAt = parseTime(created)
		out = append(out, u)
	}
	return out, rows.Err()
}

func (d *DB) GetChatMemberRole(ctx context.Context, chatID, userID string) (string, error) {
	var role string
	err := d.QueryRowContext(ctx,
		`SELECT COALESCE(role,'') FROM chat_members WHERE chat_id = ? AND user_id = ?`,
		chatID, userID,
	).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return role, nil
}

func (d *DB) ChatMemberCount(ctx context.Context, chatID string) (int, error) {
	var n int
	err := d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM chat_members WHERE chat_id = ?`,
		chatID,
	).Scan(&n)
	return n, err
}

func (d *DB) IsChatMember(ctx context.Context, chatID, userID string) (bool, error) {
	var x int
	err := d.QueryRowContext(ctx,
		`SELECT 1 FROM chat_members WHERE chat_id = ? AND user_id = ?`,
		chatID, userID,
	).Scan(&x)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (d *DB) AddChatMember(ctx context.Context, chatID, userID string) error {
	res, err := d.ExecContext(ctx,
		`INSERT OR IGNORE INTO chat_members (chat_id, user_id) VALUES (?,?)`,
		chatID, userID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		logutil.Debug("add member %s to %s: already member", userID, chatID)
		return ErrConflict
	}
	_, err = d.ExecContext(ctx, `UPDATE chats SET member_count = member_count + 1 WHERE id = ?`, chatID)
	logutil.Info("added member %s to chat %s", userID, chatID)
	return err
}

func (d *DB) RemoveChatMember(ctx context.Context, chatID, userID string) error {
	res, err := d.ExecContext(ctx,
		`DELETE FROM chat_members WHERE chat_id = ? AND user_id = ?`,
		chatID, userID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		_, err = d.ExecContext(ctx, `UPDATE chats SET member_count = member_count - 1 WHERE id = ?`, chatID)
		logutil.Info("removed member %s from chat %s", userID, chatID)
	} else {
		logutil.Debug("remove member %s from %s: not found", userID, chatID)
	}
	return err
}

func (d *DB) UpdatePinnedLastReadAt(ctx context.Context, chatID, userID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := d.ExecContext(ctx,
		`UPDATE chat_members SET pinned_last_read_at = ? WHERE chat_id = ? AND user_id = ?`,
		now, chatID, userID,
	)
	return err
}

func (d *DB) UpdateLastActiveAt(ctx context.Context, chatID, userID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := d.ExecContext(ctx,
		`UPDATE chat_members SET last_active_at = ? WHERE chat_id = ? AND user_id = ?`,
		now, chatID, userID,
	)
	return err
}

func (d *DB) SetPinned(ctx context.Context, chatID, userID string, pinned bool) error {
	v := 0
	if pinned {
		v = 1
	}
	_, err := d.ExecContext(ctx,
		`UPDATE chat_members SET pinned = ? WHERE chat_id = ? AND user_id = ?`,
		v, chatID, userID,
	)
	return err
}

func (d *DB) SetChatNotifyEnabled(ctx context.Context, chatID, userID string, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	_, err := d.ExecContext(ctx,
		`UPDATE chat_members SET notify_enabled = ? WHERE chat_id = ? AND user_id = ?`,
		v, chatID, userID,
	)
	return err
}

func (d *DB) TogglePinned(ctx context.Context, chatID, userID string) error {
	_, err := d.ExecContext(ctx,
		`UPDATE chat_members SET pinned = CASE WHEN pinned = 0 THEN 1 ELSE 0 END WHERE chat_id = ? AND user_id = ?`,
		chatID, userID,
	)
	return err
}
