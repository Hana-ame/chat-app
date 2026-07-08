package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/models"
)

// ── Messages ─────────────────────────────────────────────────────────

func (d *DB) CreateMessage(ctx context.Context, chatID, userID, content string, mentions []string, attachments []models.Attachment) (*models.Message, error) {
	content = strings.TrimRight(content, " \n\t")
	if len(content) > 4000 {
		content = content[:4000]
	}
	if content == "" && len(attachments) == 0 {
		return nil, errors.New("empty message")
	}

	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	id := NewID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx,
		`INSERT INTO messages (id, chat_id, user_id, content, created_at, attachment_count, mention_count) VALUES (?,?,?,?,?,?,?)`,
		id, chatID, userID, content, now, len(attachments), len(dedupe(mentions)),
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE chats SET last_message_at = ? WHERE id = ?`,
		now, chatID,
	)
	if err != nil {
		return nil, err
	}
	for _, m := range dedupe(mentions) {
		_, err = tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO mentions (message_id, user_id) VALUES (?,?)`,
			id, m,
		)
		if err != nil {
			return nil, err
		}
	}
	for i, a := range attachments {
		if a.ID == "" {
			a.ID = NewID()
			attachments[i].ID = a.ID
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO attachments (id, message_id, filename, mime_type, size, url) VALUES (?,?,?,?,?,?)`,
			a.ID, id, a.Filename, a.MimeType, a.Size, a.URL,
		)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d.GetMessage(ctx, id)
}

func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func (d *DB) GetMessage(ctx context.Context, id string) (*models.Message, error) {
	m, err := d.fetchMessageRow(ctx,
		`SELECT m.id, m.chat_id, m.user_id, m.content, m.created_at, m.edited_at, m.deleted_at,
		        m.attachment_count, m.mention_count, m.reaction_count,
		        u.id, u.username, u.avatar_color, u.status
		 FROM messages m JOIN users u ON u.id = m.user_id
		 WHERE m.id = ?`,
		id,
	)
	if err != nil {
		return nil, err
	}
	if err := d.attachExtras(ctx, m, ""); err != nil {
		return nil, err
	}
	return m, nil
}

func (d *DB) fetchMessageRow(ctx context.Context, q, id string) (*models.Message, error) {
	var (
		m         models.Message
		author    models.User
		edited    sql.NullString
		deletedAt sql.NullString
		created   string
		attCnt    int
		mentCnt   int
		rxnCnt    int
	)
	err := d.QueryRowContext(ctx, q, id).Scan(
		&m.ID, &m.ChatID, &m.UserID, &m.Content, &created, &edited, &deletedAt,
		&attCnt, &mentCnt, &rxnCnt,
		&author.ID, &author.Username, &author.AvatarColor, &author.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	m.CreatedAt = parseTime(created)
	if edited.Valid && edited.String != "" {
		t := parseTime(edited.String)
		m.EditedAt = &t
	}
	if deletedAt.Valid && deletedAt.String != "" {
		t := parseTime(deletedAt.String)
		m.DeletedAt = &t
		m.Content = ""
	}
	m.AttachmentCount = attCnt
	m.MentionCount = mentCnt
	m.ReactionCount = rxnCnt
	m.Author = &author
	return &m, nil
}

func (d *DB) attachExtras(ctx context.Context, m *models.Message, viewerID string) error {
	if m.AttachmentCount > 0 {
		atts, err := d.attachmentsFor(ctx, m.ID)
		if err != nil {
			return err
		}
		m.Attachments = atts
	}
	if m.MentionCount > 0 {
		mentions, err := d.mentionsFor(ctx, m.ID)
		if err != nil {
			return err
		}
		m.Mentions = mentions
	}
	if m.ReactionCount > 0 {
		rxs, err := d.reactionsFor(ctx, m.ID, viewerID)
		if err != nil {
			return err
		}
		m.Reactions = rxs
	}
	return nil
}

func (d *DB) GetMessages(ctx context.Context, chatID, viewerID, before string, limit int, details bool) ([]models.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var (
		rows *sql.Rows
		err  error
	)
	if before == "" {
		rows, err = d.QueryContext(ctx,
			`SELECT m.id, m.chat_id, m.user_id, m.content, m.created_at, m.edited_at, m.deleted_at,
			        m.attachment_count, m.mention_count, m.reaction_count,
			        u.id, u.username, u.avatar_color, u.status
			 FROM messages m JOIN users u ON u.id = m.user_id
			 WHERE m.chat_id = ?
			 ORDER BY m.created_at DESC, m.id DESC LIMIT ?`,
			chatID, limit,
		)
	} else {
		rows, err = d.QueryContext(ctx,
			`SELECT m.id, m.chat_id, m.user_id, m.content, m.created_at, m.edited_at, m.deleted_at,
			        m.attachment_count, m.mention_count, m.reaction_count,
			        u.id, u.username, u.avatar_color, u.status
			 FROM messages m JOIN users u ON u.id = m.user_id
			 WHERE m.chat_id = ? AND (m.created_at, m.id) < (
			    SELECT created_at, id FROM messages WHERE id = ?
			 )
			 ORDER BY m.created_at DESC, m.id DESC LIMIT ?`,
			chatID, before, limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Message{}
	for rows.Next() {
		var (
			m         models.Message
			author    models.User
			edited    sql.NullString
			deletedAt sql.NullString
			created   string
			attCnt    int
			mentCnt   int
			rxnCnt    int
		)
		if err := rows.Scan(
			&m.ID, &m.ChatID, &m.UserID, &m.Content, &created, &edited, &deletedAt,
			&attCnt, &mentCnt, &rxnCnt,
			&author.ID, &author.Username, &author.AvatarColor, &author.Status,
		); err != nil {
			return nil, err
		}
		m.CreatedAt = parseTime(created)
		if edited.Valid && edited.String != "" {
			t := parseTime(edited.String)
			m.EditedAt = &t
		}
		if deletedAt.Valid && deletedAt.String != "" {
			t := parseTime(deletedAt.String)
			m.DeletedAt = &t
			m.Content = ""
		}
		m.AttachmentCount = attCnt
		m.MentionCount = mentCnt
		m.ReactionCount = rxnCnt
		m.Author = &author
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if details {
		for i := range out {
			if err := d.attachExtras(ctx, &out[i], viewerID); err != nil {
				return nil, err
			}
		}
	}
	// Reverse to chronological ascending order for client
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (d *DB) LastMessage(ctx context.Context, chatID string) (*models.Message, error) {
	m, err := d.fetchMessageRow(ctx,
		`SELECT m.id, m.chat_id, m.user_id, m.content, m.created_at, m.edited_at, m.deleted_at,
		        m.attachment_count, m.mention_count, m.reaction_count,
		        u.id, u.username, u.avatar_color, u.status
		 FROM messages m JOIN users u ON u.id = m.user_id
		 WHERE m.chat_id = ?
		 ORDER BY m.created_at DESC, m.id DESC LIMIT 1`,
		chatID,
	)
	if err != nil {
		return nil, err
	}
	// No attachExtras — chat list only needs content preview and counts.
	return m, nil
}

func (d *DB) UnreadCount(ctx context.Context, chatID, lastReadID string) (int, error) {
	var n int
	if lastReadID == "" {
		err := d.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM messages WHERE chat_id = ? AND deleted_at IS NULL`,
			chatID,
		).Scan(&n)
		return n, err
	}
	err := d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM messages
		 WHERE chat_id = ? AND deleted = 0
		   AND (created_at, id) > (SELECT created_at, id FROM messages WHERE id = ?)`,
		chatID, lastReadID,
	).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (d *DB) UpdateMessage(ctx context.Context, id, userID, content string) (*models.Message, error) {
	content = strings.TrimRight(content, " \n\t")
	if content == "" {
		return nil, errors.New("empty content")
	}
	if len(content) > 4000 {
		content = content[:4000]
	}
	res, err := d.ExecContext(ctx,
		`UPDATE messages SET content = ?, edited_at = ? WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		content, time.Now().UTC().Format(time.RFC3339Nano), id, userID,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrNotFound
	}
	return d.GetMessage(ctx, id)
}

func (d *DB) DeleteMessage(ctx context.Context, id, userID string, allowAny bool) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var q string
	var args []interface{}
	if allowAny {
		q = `UPDATE messages SET deleted_at = ?, content = '' WHERE id = ?`
		args = []interface{}{now, id}
	} else {
		q = `UPDATE messages SET deleted_at = ?, content = '' WHERE id = ? AND user_id = ?`
		args = []interface{}{now, id, userID}
	}
	res, err := d.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ── Attachments ──────────────────────────────────────────────────────

func (d *DB) attachmentsFor(ctx context.Context, messageID string) ([]models.Attachment, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id, message_id, filename, mime_type, size, url FROM attachments WHERE message_id = ?`,
		messageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Attachment{}
	for rows.Next() {
		var a models.Attachment
		if err := rows.Scan(&a.ID, &a.MessageID, &a.Filename, &a.MimeType, &a.Size, &a.URL); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ── Reactions ────────────────────────────────────────────────────────

func (d *DB) AddReaction(ctx context.Context, messageID, userID, emoji string) error {
	emoji = strings.TrimSpace(emoji)
	if emoji == "" {
		return errors.New("emoji required")
	}
	if len(emoji) > 32 {
		return errors.New("emoji too long")
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO reactions (message_id, user_id, emoji) VALUES (?,?,?)`,
		messageID, userID, emoji,
	)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE messages SET reaction_count = (
			SELECT COUNT(*) FROM reactions WHERE message_id = ?
		) WHERE id = ?`,
		messageID, messageID,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (d *DB) RemoveReaction(ctx context.Context, messageID, userID, emoji string) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx,
		`DELETE FROM reactions WHERE message_id = ? AND user_id = ? AND emoji = ?`,
		messageID, userID, emoji,
	)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE messages SET reaction_count = (
			SELECT COUNT(*) FROM reactions WHERE message_id = ?
		) WHERE id = ?`,
		messageID, messageID,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (d *DB) reactionsFor(ctx context.Context, messageID, viewerID string) ([]models.Reaction, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT emoji, user_id FROM reactions WHERE message_id = ? ORDER BY created_at`,
		messageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	grouped := map[string]*models.Reaction{}
	order := []string{}
	for rows.Next() {
		var emoji, uid string
		if err := rows.Scan(&emoji, &uid); err != nil {
			return nil, err
		}
		if r, ok := grouped[emoji]; ok {
			r.Count++
			r.UserIDs = append(r.UserIDs, uid)
			if uid == viewerID {
				r.Me = true
			}
		} else {
			r := &models.Reaction{
				Emoji:   emoji,
				Count:   1,
				UserIDs: []string{uid},
				Me:      viewerID != "" && uid == viewerID,
			}
			grouped[emoji] = r
			order = append(order, emoji)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]models.Reaction, 0, len(order))
	for _, e := range order {
		out = append(out, *grouped[e])
	}
	return out, nil
}

// ── Mentions ─────────────────────────────────────────────────────────

func (d *DB) mentionsFor(ctx context.Context, messageID string) ([]string, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT user_id FROM mentions WHERE message_id = ?`,
		messageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		out = append(out, uid)
	}
	return out, rows.Err()
}
