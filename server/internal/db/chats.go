package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

// ── Refresh Tokens ───────────────────────────────────────────────────

func (d *DB) CreateRefreshToken(ctx context.Context, userID, tokenHash string, ttl time.Duration) (*models.RefreshToken, error) {
	id := NewID()
	expires := time.Now().UTC().Add(ttl)
	_, err := d.ExecContext(ctx,
		`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES (?,?,?,?)`,
		id, userID, tokenHash, expires.Format(time.RFC3339Nano),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrConflict
		}
		return nil, err
	}
	return &models.RefreshToken{
		ID: id, UserID: userID, TokenHash: tokenHash, ExpiresAt: expires, CreatedAt: time.Now().UTC(),
	}, nil
}

func (d *DB) FindRefreshToken(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	var rt models.RefreshToken
	var expires, created string
	err := d.QueryRowContext(ctx,
		`SELECT id, user_id, token_hash, expires_at, created_at FROM refresh_tokens WHERE token_hash = ?`,
		tokenHash,
	).Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &expires, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rt.ExpiresAt = parseTime(expires)
	rt.CreatedAt = parseTime(created)
	return &rt, nil
}

func (d *DB) DeleteRefreshToken(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE id = ?`, id)
	return err
}

func (d *DB) DeleteUserRefreshTokens(ctx context.Context, userID string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE user_id = ?`, userID)
	return err
}

func (d *DB) PurgeExpiredTokens(ctx context.Context) (int64, error) {
	res, err := d.ExecContext(ctx,
		`DELETE FROM refresh_tokens WHERE expires_at < ?`,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

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
		`SELECT id, type, name, icon_color, visibility, owner_id, created_at, last_message_at, last_message_id, pinned_message, pinned_updated_at, member_count
		 FROM chats WHERE id = ?`,
		id,
	).Scan(&c.ID, &c.Type, &name, &c.IconColor, &c.Visibility, &owner, &createdAt, &lastMsgAt, &lastMsgID, &pinnedMsg, &pinnedUpdAt, &memberCount)
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

func (d *DB) ListUserChats(ctx context.Context, userID string) ([]models.Chat, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT c.id, c.type, c.name, c.icon_color, c.visibility, c.owner_id, c.created_at, c.last_message_at, c.last_message_id,
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
		if err := rows.Scan(&c.ID, &c.Type, &name, &c.IconColor, &visibility, &owner, &created, &lastMsg, &lastMsgID, &lastRead, &pinnedMsg, &pinnedUpdAt, &memberCount, &pinnedLastReadAt, &pinnedBool, &lastActiveAt); err != nil {
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
		// Deprecated.
		if c.LastMessageID != "" {
			last, err := d.GetMessage(ctx, c.LastMessageID)
			if err == nil {
				c.LastMessage = last
			}
		}
		var lastReadID string
		if r.lastRead.Valid {
			lastReadID = r.lastRead.String
		}
		// Deprecated.
		unread, err := d.UnreadCount(ctx, c.ID, lastReadID)
		if err != nil {
			return nil, err
		}
		// Deprecated.
		c.UnreadCount = unread
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

func (d *DB) UpdateLastRead(ctx context.Context, chatID, userID, messageID string) error {
	_, err := d.ExecContext(ctx,
		`UPDATE chat_members SET last_read_message_id = ? WHERE chat_id = ? AND user_id = ?`,
		messageID, chatID, userID,
	)
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

func (d *DB) TogglePinned(ctx context.Context, chatID, userID string) error {
	_, err := d.ExecContext(ctx,
		`UPDATE chat_members SET pinned = CASE WHEN pinned = 0 THEN 1 ELSE 0 END WHERE chat_id = ? AND user_id = ?`,
		chatID, userID,
	)
	return err
}
