package db

import (
	"context"
	"database/sql"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

func (d *DB) ListPublicChats(ctx context.Context) ([]models.Chat, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id, type, name, icon_color, COALESCE(visibility,'private'), owner_id, created_at, last_message_at
		 FROM chats WHERE type = 'group' AND visibility = 'public'
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Chat{}
	for rows.Next() {
		var c models.Chat
		var name, owner, lastMsg sql.NullString
		var created string
		if err := rows.Scan(&c.ID, &c.Type, &name, &c.IconColor, &c.Visibility, &owner, &created, &lastMsg); err != nil {
			return nil, err
		}
		c.Name = name.String
		c.OwnerID = owner.String
		c.CreatedAt = parseTime(created)
		c.LastMessageAt = parseTimePtr(lastMsg)
		members, _ := d.GetChatMembers(ctx, c.ID)
		c.Members = members
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DB) JoinPublicChat(ctx context.Context, chatID, userID string) error {
	_, err := d.ExecContext(ctx,
		`INSERT OR IGNORE INTO chat_members (chat_id, user_id) VALUES (?,?)`,
		chatID, userID,
	)
	return err
}

func (d *DB) PinChat(ctx context.Context, chatID, userID string) error {
	_, err := d.ExecContext(ctx,
		`UPDATE chat_members SET pinned = 1 WHERE chat_id = ? AND user_id = ?`,
		chatID, userID,
	)
	return err
}

func (d *DB) UnpinChat(ctx context.Context, chatID, userID string) error {
	_, err := d.ExecContext(ctx,
		`UPDATE chat_members SET pinned = 0 WHERE chat_id = ? AND user_id = ?`,
		chatID, userID,
	)
	return err
}
