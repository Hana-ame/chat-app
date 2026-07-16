package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

// ── Chats ────────────────────────────────────────────────────────────

func (d *DB) CreateChat(ctx context.Context, typ, name, visibility, ownerID string, memberIDs []string) (*models.Chat, error) {
	if typ != "dm" && typ != "group" {
		return nil, errors.New("invalid chat type")
	}
	name = strings.TrimSpace(name)
	if typ == "group" && name == "" {
		return nil, errors.New("group chat requires name")
	}
	if len(memberIDs) == 0 {
		return nil, errors.New("at least one member required")
	}
	if typ == "dm" && len(memberIDs) != 2 {
		return nil, errors.New("dm requires exactly 2 members")
	}

	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	logutil.Debug("creating chat: type=%s name=%s owner=%s members=%v", typ, name, ownerID, memberIDs)

	id := NewID()
	color := PickColor(id)
	if typ == "group" {
		color = PickColor(name)
	}

	var ownerVal interface{}
	if ownerID == "" {
		ownerVal = nil
	} else {
		ownerVal = ownerID
	}
	var nameVal interface{}
	if name == "" {
		nameVal = nil
	} else {
		nameVal = name
	}

	if visibility != "public" && visibility != "unlisted" {
		visibility = "private"
	}
	if typ == "dm" {
		visibility = ""
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO chats (id, type, name, icon_color, owner_id, visibility, member_count) VALUES (?,?,?,?,?,?,?)`,
		id, typ, nameVal, color, ownerVal, visibility, len(memberIDs),
	)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(memberIDs))
	for _, m := range memberIDs {
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		role := ""
		if m == ownerID {
			role = "owner"
		}
		_, err = tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO chat_members (chat_id, user_id, role) VALUES (?,?,?)`,
			id, m, role,
		)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d.GetChat(ctx, id)
}

func (d *DB) GetChat(ctx context.Context, id string) (*models.Chat, error) {
	var (
		c              models.Chat
		name           sql.NullString
		owner          sql.NullString
		createdAt      string
		lastMsgAt      sql.NullString
		lastMsgID      sql.NullString
		pinnedMsg      sql.NullString
		pinnedUpdAt    sql.NullString
		memberCount    int
	)
	err := d.QueryRowContext(ctx,
		`SELECT id, type, name, icon_color, avatar_url, banner_url, banner_opacity, background_url, visibility, owner_id, created_at, last_message_at, last_message_id, pinned_message, pinned_updated_at, member_count
		 FROM chats WHERE id = ?`,
		id,
	).Scan(&c.ID, &c.Type, &name, &c.IconColor, &c.AvatarURL, &c.BannerURL, &c.BannerOpacity, &c.BackgroundURL, &c.Visibility, &owner, &createdAt, &lastMsgAt, &lastMsgID, &pinnedMsg, &pinnedUpdAt, &memberCount)
	if errors.Is(err, sql.ErrNoRows) {
		logutil.Debug("chat not found: %s", id)
		return nil, ErrNotFound
	}
	if err != nil {
		logutil.Error("get chat %s: %v", id, err)
		return nil, err
	}
	c.Name = name.String
	c.OwnerID = owner.String
	c.CreatedAt = parseTime(createdAt)
	if lastMsgAt.Valid && lastMsgAt.String != "" {
		c.LastMessageAt = parseTime(lastMsgAt.String)
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
	c.MemberCount = memberCount
	c.LastMessageID = lastMsgID.String
	if c.LastMessageID != "" {
		lastMsg, err := d.GetMessage(ctx, c.LastMessageID)
		if err == nil {
			c.LastMessage = lastMsg
		}
	}
	logutil.Debug("get chat %s: type=%s name=%s members=%d", id, c.Type, c.Name, c.MemberCount)
	return &c, nil
}



func (d *DB) ListUserChats(ctx context.Context, userID string) ([]models.Chat, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT c.id, c.type, c.name, c.icon_color, c.avatar_url, c.banner_url, c.banner_opacity, c.background_url, c.visibility, c.owner_id, c.created_at, c.last_message_at, c.last_message_id,
		        cm.last_read_message_id, c.pinned_message, c.pinned_updated_at, c.member_count,
		        cm.pinned_last_read_at, cm.pinned, cm.last_active_at
		 FROM chat_members cm JOIN chats c ON c.id = cm.chat_id
		 WHERE cm.user_id = ?
		 ORDER BY COALESCE(c.last_message_at, c.created_at) DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Chat{}
	type row struct {
		chat             models.Chat
		lastRead         sql.NullString
		pinnedLastReadAt sql.NullString
	}
	rows2 := []row{}
	for rows.Next() {
		var c models.Chat
		var name, owner, lastMsg, lastMsgID, lastRead, pinnedMsg, pinnedUpdAt, pinnedLastReadAt, lastActiveAt sql.NullString
		var visibility sql.NullString
		var created string
		var memberCount int
		var pinnedBool bool
		if err := rows.Scan(&c.ID, &c.Type, &name, &c.IconColor, &c.AvatarURL, &c.BannerURL, &c.BannerOpacity, &c.BackgroundURL, &visibility, &owner, &created, &lastMsg, &lastMsgID, &lastRead, &pinnedMsg, &pinnedUpdAt, &memberCount, &pinnedLastReadAt, &pinnedBool, &lastActiveAt); err != nil {
			return nil, err
		}
		c.Name = name.String
		c.Visibility = visibility.String
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
		if lastActiveAt.Valid && lastActiveAt.String != "" {
			t := parseTime(lastActiveAt.String)
			c.LastActiveAt = &t
		}
		c.Pinned = pinnedBool
		c.MemberCount = memberCount
		c.LastMessageID = lastMsgID.String
		rows2 = append(rows2, row{chat: c, lastRead: lastRead, pinnedLastReadAt: pinnedLastReadAt})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, r := range rows2 {
		c := r.chat
		if r.pinnedLastReadAt.Valid && r.pinnedLastReadAt.String != "" {
			t := parseTime(r.pinnedLastReadAt.String)
			c.PinnedLastReadAt = &t
		}
		c.UnreadCount = 0
		if c.LastActiveAt != nil {
			c.UnreadCount = d.UnreadCount(ctx, c.ID, *c.LastActiveAt)
		}
		out = append(out, c)
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].LastMessageAt.After(out[j].LastMessageAt)
	})
	logutil.Debug("list chats for user %s: %d chats", userID, len(out))
	return out, nil
}

// Deprecated.
func (d *DB) FindDMBetween(ctx context.Context, a, b string) (*models.Chat, error) {
	if a == b {
		return nil, errors.New("cannot dm yourself")
	}
	var id string
	err := d.QueryRowContext(ctx,
		`SELECT c.id FROM chats c
		 JOIN chat_members cm1 ON cm1.chat_id = c.id AND cm1.user_id = ?
		 JOIN chat_members cm2 ON cm2.chat_id = c.id AND cm2.user_id = ?
		 WHERE c.type = 'dm'
		 LIMIT 1`,
		a, b,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return d.GetChat(ctx, id)
}



func (d *DB) DeleteChat(ctx context.Context, chatID string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM chats WHERE id = ?`, chatID)
	if err != nil {
		logutil.Error("delete chat %s: %v", chatID, err)
	} else {
		logutil.Warn("deleted chat %s", chatID)
	}
	return err
}

func (d *DB) RenameChat(ctx context.Context, chatID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name required")
	}
	_, err := d.ExecContext(ctx, `UPDATE chats SET name = ? WHERE id = ?`, name, chatID)
	if err == nil {
		logutil.Info("renamed chat %s to %q", chatID, name)
	}
	return err
}

func (d *DB) UpdateChatAvatar(ctx context.Context, chatID, avatarURL string) error {
	_, err := d.ExecContext(ctx, `UPDATE chats SET avatar_url = ? WHERE id = ?`, avatarURL, chatID)
	if err == nil {
		logutil.Info("updated chat %s avatar", chatID)
	}
	return err
}

func (d *DB) UpdateChatBanner(ctx context.Context, chatID, bannerURL string) error {
	_, err := d.ExecContext(ctx, `UPDATE chats SET banner_url = ? WHERE id = ?`, bannerURL, chatID)
	if err == nil {
		logutil.Info("updated chat %s banner", chatID)
	}
	return err
}

func (d *DB) UpdateChatBannerOpacity(ctx context.Context, chatID string, opacity float64) error {
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 1 {
		opacity = 1
	}
	_, err := d.ExecContext(ctx, `UPDATE chats SET banner_opacity = ? WHERE id = ?`, opacity, chatID)
	if err == nil {
		logutil.Info("updated chat %s banner opacity to %.2f", chatID, opacity)
	}
	return err
}

func (d *DB) UpdateChatBackground(ctx context.Context, chatID, backgroundURL string) error {
	_, err := d.ExecContext(ctx, `UPDATE chats SET background_url = ? WHERE id = ?`, backgroundURL, chatID)
	if err == nil {
		logutil.Info("updated chat %s background", chatID)
	}
	return err
}

// Deprecated: use UpdateLastActiveAt instead.
func (d *DB) UpdateLastRead(ctx context.Context, chatID, userID, messageID string) error {
	_, err := d.ExecContext(ctx,
		`UPDATE chat_members SET last_read_message_id = ? WHERE chat_id = ? AND user_id = ?`,
		messageID, chatID, userID,
	)
	return err
}


