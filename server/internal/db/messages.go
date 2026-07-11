package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/models"
)

func (d *DB) syncReactionsColumn(ctx context.Context, messageID string) error {
	rxs, err := d.reactionsFor(ctx, messageID, "")
	if err != nil {
		return err
	}
	data, err := json.Marshal(rxs)
	if err != nil {
		data = []byte("[]")
	}
	_, err = d.ExecContext(ctx,
		`UPDATE messages SET reactions = ? WHERE id = ?`,
		string(data), messageID,
	)
	return err
}

// ── Messages ─────────────────────────────────────────────────────────

func (d *DB) CreateMessage(ctx context.Context, chatID, userID, content string, mentions []string, attachments []models.Attachment) (*models.Message, error) {
	content = strings.TrimRight(content, " \n\t")
	if len(content) > 4000 {
		return nil, errors.New("content too long, use file upload instead")
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
	for i := range attachments {
		if attachments[i].ID == "" {
			attachments[i].ID = NewID()
		}
		if len(attachments[i].Filename) > 200 {
			ext := filepath.Ext(attachments[i].Filename)
			attachments[i].Filename = "file-" + strconv.FormatInt(time.Now().Unix(), 10) + ext
		}
	}
	attJSON, err := json.Marshal(attachments)
	if err != nil {
		attJSON = []byte("[]")
	}
	mentJSON, err := json.Marshal(dedupe(mentions))
	if err != nil {
		mentJSON = []byte("[]")
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO messages (id, chat_id, user_id, content, created_at, attachment_count, mention_count, attachments, mentions) VALUES (?,?,?,?,?,?,?,?,?)`,
		id, chatID, userID, content, now, len(attachments), len(dedupe(mentions)), string(attJSON), string(mentJSON),
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE chats SET last_message_at = ?, last_message_id = ? WHERE id = ?`,
		now, id, chatID,
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE chat_members SET last_seen = ? WHERE chat_id = ? AND user_id = ?`,
		now, chatID, userID,
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE users SET last_seen = ? WHERE id = ?`,
		now, userID,
	)
	if err != nil {
		return nil, err
	}
	// Deprecated: mentions/attachments are stored as JSON in messages.mentions column.
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
		        m.attachment_count, m.mention_count, m.reaction_count, m.reactions, m.attachments, m.mentions,
		        u.id, u.username, u.avatar_color, u.status
		 FROM messages m JOIN users u ON u.id = m.user_id
		 WHERE m.id = ?`,
		id,
	)
	if err != nil {
		return nil, err
	}
	return m, nil
}

type scanner interface{ Scan(dest ...interface{}) error }

func scanMessage(s scanner) (*models.Message, error) {
	var (
		m         models.Message
		author    models.User
		edited    sql.NullString
		deletedAt sql.NullString
		rxnJSON   sql.NullString
		attJSON   sql.NullString
		mentJSON  sql.NullString
		created   string
		attCnt    int
		mentCnt   int
		rxnCnt    int
	)
	err := s.Scan(
		&m.ID, &m.ChatID, &m.UserID, &m.Content, &created, &edited, &deletedAt,
		&attCnt, &mentCnt, &rxnCnt, &rxnJSON, &attJSON, &mentJSON,
		&author.ID, &author.Username, &author.AvatarColor, &author.Status,
	)
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
	if rxnJSON.Valid && rxnJSON.String != "" {
		m.Reactions = json.RawMessage(rxnJSON.String)
	}
	if attJSON.Valid && attJSON.String != "" {
		m.Attachments = json.RawMessage(attJSON.String)
	}
	if mentJSON.Valid && mentJSON.String != "" {
		m.Mentions = json.RawMessage(mentJSON.String)
	}
	m.Author = &author
	return &m, nil
}

func (d *DB) fetchMessageRow(ctx context.Context, q, id string) (*models.Message, error) {
	m, err := scanMessage(d.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Attachments, mentions, and reactions are loaded from JSON columns
// (m.attachments, m.mentions, m.reactions) in the main SELECT query.
// The legacy attachExtras hook that fetched them via N+1 subqueries
// has been removed — all data is now in the row.
func (d *DB) GetMessages(ctx context.Context, chatID, before string, limit int) ([]models.Message, error) {
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
			        m.attachment_count, m.mention_count, m.reaction_count, m.reactions, m.attachments, m.mentions,
			        u.id, u.username, u.avatar_color, u.status
			 FROM messages m JOIN users u ON u.id = m.user_id
			 WHERE m.chat_id = ?
			 ORDER BY m.created_at DESC, m.id DESC LIMIT ?`,
			chatID, limit,
		)
	} else {
		rows, err = d.QueryContext(ctx,
			`SELECT m.id, m.chat_id, m.user_id, m.content, m.created_at, m.edited_at, m.deleted_at,
			        m.attachment_count, m.mention_count, m.reaction_count, m.reactions, m.attachments, m.mentions,
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
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
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
		        m.attachment_count, m.mention_count, m.reaction_count, m.reactions, m.attachments, m.mentions,
		        u.id, u.username, u.avatar_color, u.status
		 FROM messages m JOIN users u ON u.id = m.user_id
		 WHERE m.chat_id = ?
		 ORDER BY m.created_at DESC, m.id DESC LIMIT 1`,
		chatID,
	)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Deprecated.
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
		 WHERE chat_id = ? AND deleted_at IS NULL
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
		return nil, errors.New("content too long, use file upload instead")
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

// Deprecated.
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
	if err := tx.Commit(); err != nil {
		return err
	}
	return d.syncReactionsColumn(ctx, messageID)
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
	if err := tx.Commit(); err != nil {
		return err
	}
	return d.syncReactionsColumn(ctx, messageID)
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
		} else {
			r := &models.Reaction{
				Emoji: emoji,
				Count: 1,
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

func (d *DB) ListReactions(ctx context.Context, messageID, viewerID string) ([]models.Reaction, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT emoji, user_id FROM reactions WHERE message_id = ? ORDER BY created_at`,
		messageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct{ emoji, uid string }
	var all []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.emoji, &r.uid); err != nil {
			return nil, err
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	grouped := map[string]*models.Reaction{}
	order := []string{}
	for _, r := range all {
		grp, ok := grouped[r.emoji]
		if !ok {
			grp = &models.Reaction{Emoji: r.emoji, UserIDs: []string{}}
			grouped[r.emoji] = grp
			order = append(order, r.emoji)
		}
		grp.Count++
		grp.UserIDs = append(grp.UserIDs, r.uid)
	}
	for _, grp := range grouped {
		for _, uid := range grp.UserIDs {
			if uid == viewerID {
				grp.Me = true
				break
			}
		}
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
