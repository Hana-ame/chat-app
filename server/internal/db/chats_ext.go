package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

func (d *DB) ListPublicChats(ctx context.Context, page, limit int) ([]models.Chat, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit
	logutil.Debug("list public chats: page=%d limit=%d", page, limit)
	rows, err := d.QueryContext(ctx,
		`SELECT c.id, c.type, c.name, c.icon_color, COALESCE(c.visibility,'private'), c.owner_id, c.created_at, c.last_message_at, c.pinned_message, c.pinned_updated_at,
		        (SELECT COUNT(*) FROM chat_members WHERE chat_id = c.id) AS member_count,
		        (SELECT content FROM messages WHERE chat_id = c.id AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 1) AS last_message_content
		 FROM chats c WHERE c.type = 'group' AND c.visibility = 'public'
		 ORDER BY c.last_message_at DESC NULLS LAST, c.created_at DESC
		 LIMIT ? OFFSET ?`, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Chat{}
	for rows.Next() {
		var c models.Chat
		var name, owner, lastMsg, pinnedMsg, pinnedUpdAt, lastMsgContent sql.NullString
		var created string
		var memberCount int
		if err := rows.Scan(&c.ID, &c.Type, &name, &c.IconColor, &c.Visibility, &owner, &created, &lastMsg, &pinnedMsg, &pinnedUpdAt, &memberCount, &lastMsgContent); err != nil {
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
		if pinnedUpdAt.Valid && pinnedUpdAt.String != "" {
			t := parseTime(pinnedUpdAt.String)
			c.PinnedUpdatedAt = &t
		}
		if lastMsgContent.Valid && lastMsgContent.String != "" {
			content := lastMsgContent.String
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			c.LastMessage = &models.Message{Content: content}
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
		logutil.Warn("join private chat %s rejected for %s", chatID, userID)
		return errors.New("chat is private, invitation required")
	}
	res, err := d.ExecContext(ctx,
		`INSERT OR IGNORE INTO chat_members (chat_id, user_id, role) VALUES (?,?,'')`,
		chatID, userID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		_, err = d.ExecContext(ctx, `UPDATE chats SET member_count = member_count + 1 WHERE id = ?`, chatID)
		logutil.Info("user %s joined chat %s", userID, chatID)
	}
	return err
}

func (d *DB) SetPinnedMessage(ctx context.Context, chatID, content string) error {
	now := time.Now().UTC()
	pc := models.PinnedContent{Content: content, PinnedAt: now}
	data, err := json.Marshal(pc)
	if err != nil {
		data = []byte("{}")
	}
	_, err = d.ExecContext(ctx,
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
