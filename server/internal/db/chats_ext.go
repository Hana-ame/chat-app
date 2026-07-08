package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

func (d *DB) ListPublicChats(ctx context.Context) ([]models.Chat, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id, type, name, icon_color, COALESCE(visibility,'private'), owner_id, created_at, last_message_at, pinned_message,
		        (SELECT COUNT(*) FROM chat_members WHERE chat_id = id) AS member_count
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
		var name, owner, lastMsg, pinnedMsg sql.NullString
		var created string
		var memberCount int
		if err := rows.Scan(&c.ID, &c.Type, &name, &c.IconColor, &c.Visibility, &owner, &created, &lastMsg, &pinnedMsg, &memberCount); err != nil {
			return nil, err
		}
		c.Name = name.String
		c.OwnerID = owner.String
		c.CreatedAt = parseTime(created)
		if lastMsg.Valid && lastMsg.String != "" {
			c.LastMessageAt = parseTime(lastMsg.String)
		} else {
			c.LastMessageAt = c.CreatedAt
		}
		if pinnedMsg.Valid && pinnedMsg.String != "" {
			var pc models.PinnedContent
			if err := json.Unmarshal([]byte(pinnedMsg.String), &pc); err == nil {
				c.PinnedMessage = &pc
			}
		}
		c.MemberCount = memberCount
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DB) JoinChatByID(ctx context.Context, chatID, userID string) error {
	var visibility string
	err := d.QueryRowContext(ctx,
		`SELECT COALESCE(visibility,'private') FROM chats WHERE id = ?`, chatID,
	).Scan(&visibility)
	if err != nil {
		return err
	}
	if visibility == "private" {
		return errors.New("chat is private, invitation required")
	}
	_, err = d.ExecContext(ctx,
		`INSERT OR IGNORE INTO chat_members (chat_id, user_id, role) VALUES (?,?,'')`,
		chatID, userID,
	)
	return err
}

func (d *DB) SetPinnedMessage(ctx context.Context, chatID, content string) error {
	now := time.Now().UTC()
	pc := models.PinnedContent{Content: content, PinnedAt: now}
	data, _ := json.Marshal(pc)
	_, err := d.ExecContext(ctx,
		`UPDATE chats SET pinned_message = ?, pinned_updated_at = ? WHERE id = ?`,
		string(data), now.Format(time.RFC3339Nano), chatID,
	)
	return err
}

func (d *DB) ClearPinnedMessage(ctx context.Context, chatID string) error {
	_, err := d.ExecContext(ctx,
		`UPDATE chats SET pinned_message = '', pinned_updated_at = NULL WHERE id = ?`,
		chatID,
	)
	return err
}
