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

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

func (d *DB) CreateAIMessage(ctx context.Context, chatID, userID, msgID, content, thinking string) (*models.Message, error) {
	if msgID == "" {
		msgID = NewID()
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx,
		`INSERT INTO messages (id, chat_id, user_id, type, content, thinking, created_at, attachment_count, mention_count) VALUES (?,?,?,?,?,?,?,0,0)`,
		msgID, chatID, userID, "stream", content, thinking, now,
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE chats SET last_message_at = ?, last_message_id = ?, last_message_user_id = ?, last_message_content = ?, last_message_created_at = ? WHERE id = ?`,
		now, msgID, userID, content, now, chatID,
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE chat_members SET unread_count = unread_count + 1 WHERE chat_id = ? AND user_id != ?`,
		chatID, userID,
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &models.Message{
		ID:        msgID,
		ChatID:    chatID,
		UserID:    userID,
		Type:      "stream",
		Content:   content,
		Thinking:  thinking,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// ── Messages ─────────────────────────────────────────────────────────

const messageColumns = `m.id, m.chat_id, m.user_id, m.type, m.content, m.created_at, m.edited_at, m.deleted_at,
		        m.thinking, m.attachment_count, m.mention_count, m.reaction_count, m.reactions, m.attachments, m.mentions,
		        COALESCE(m.reply_to_message_id,''),
		        u.id, u.username, u.avatar_color, u.avatar_url, u.status, u.last_seen, COALESCE(cm.role,'')`

const messageJoins = ` FROM messages m JOIN users u ON u.id = m.user_id
		 LEFT JOIN chat_members cm ON cm.chat_id = m.chat_id AND cm.user_id = m.user_id`

func (d *DB) CreateMessage(ctx context.Context, chatID, userID, content string, mentions []string, attachments []models.Attachment, replyTo ...string) (*models.Message, error) {
	content = strings.TrimRight(content, " \n\t")
	if len(content) > d.maxContentLength {
		return nil, ErrContentTooLong
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
	replyToID := ""
	if len(replyTo) > 0 {
		replyToID = replyTo[0]
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO messages (id, chat_id, user_id, content, created_at, attachment_count, mention_count, attachments, mentions, reply_to_message_id) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		id, chatID, userID, content, now, len(attachments), len(dedupe(mentions)), string(attJSON), string(mentJSON), replyToID,
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE chats SET last_message_at = ?, last_message_id = ?, last_message_user_id = ?, last_message_content = ?, last_message_created_at = ? WHERE id = ?`,
		now, id, userID, content, now, chatID,
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE chat_members SET last_active_at = ? WHERE chat_id = ? AND user_id = ?`,
		now, chatID, userID,
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE chat_members SET unread_count = unread_count + 1 WHERE chat_id = ? AND user_id != ?`,
		chatID, userID,
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
	logutil.Debug("created message in chat %s by user %s (len=%d, att=%d)", chatID, userID, len(content), len(attachments))
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
		`SELECT `+messageColumns+messageJoins+` WHERE m.id = ?`,
		id,
	)
	if err != nil {
		return nil, err
	}
	if m.ReplyTo != "" {
		replied, err := d.GetMessage(ctx, m.ReplyTo)
		if err == nil {
			replied.Content = truncate(replied.Content, 150)
			m.RepliedTo = replied
		}
	}
	return m, nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func (d *DB) scanMessage(s scanner) (*models.Message, error) {
	var (
		m         models.Message
		author    models.User
		edited    sql.NullString
		deletedAt sql.NullString
		rxnJSON   sql.NullString
		attJSON   sql.NullString
		mentJSON  sql.NullString
		created   string
		thinking  sql.NullString
		attCnt    int
		mentCnt   int
		rxnCnt    int
	)
	var role string
	var lastSeen sql.NullString
	err := s.Scan(
		&m.ID, &m.ChatID, &m.UserID, &m.Type, &m.Content, &created, &edited, &deletedAt,
		&thinking,
		&attCnt, &mentCnt, &rxnCnt, &rxnJSON, &attJSON, &mentJSON,
		&m.ReplyTo,
		&author.ID, &author.Username, &author.AvatarColor, &author.AvatarURL, &author.Status, &lastSeen, &role,
	)
	if err != nil {
		return nil, err
	}
	if lastSeen.Valid && lastSeen.String != "" {
		author.LastSeen = parseTime(lastSeen.String)
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
	if thinking.Valid && thinking.String != "" {
		m.Thinking = thinking.String
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
	author.Role = role
	m.Author = &author
	return &m, nil
}

func (d *DB) fetchMessageRow(ctx context.Context, q, id string) (*models.Message, error) {
	m, err := d.scanMessage(d.QueryRowContext(ctx, q, id))
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
		limit = 100
	}
	var (
		rows *sql.Rows
		err  error
	)
	if before == "" {
		rows, err = d.QueryContext(ctx,
			`SELECT `+messageColumns+messageJoins+` WHERE m.chat_id = ?
			 ORDER BY m.created_at DESC, m.id DESC LIMIT ?`,
			chatID, limit,
		)
	} else {
		rows, err = d.QueryContext(ctx,
			`SELECT `+messageColumns+messageJoins+` WHERE m.chat_id = ? AND (m.created_at, m.id) < (
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
		m, err := d.scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].ReplyTo != "" {
			replied, err := d.GetMessage(ctx, out[i].ReplyTo)
			if err == nil {
				replied.Content = truncate(replied.Content, 150)
				out[i].RepliedTo = replied
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
		`SELECT `+messageColumns+messageJoins+` WHERE m.chat_id = ?
		 ORDER BY m.created_at DESC, m.id DESC LIMIT 1`,
		chatID,
	)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (d *DB) UnreadCount(ctx context.Context, chatID string, lastActiveAt time.Time) int {
	var n int
	err := d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM messages WHERE chat_id = ? AND deleted_at IS NULL AND created_at > ?`,
		chatID, lastActiveAt.UTC().Format(time.RFC3339Nano),
	).Scan(&n)
	if err != nil {
		return 0
	}
	if n > 99 {
		return 99
	}
	return n
}

var ErrContentTooLong = errors.New("content too long")

func (d *DB) UpdateMessage(ctx context.Context, id, userID, content string) (*models.Message, error) {
	content = strings.TrimRight(content, " \n\t")
	if content == "" {
		return nil, errors.New("empty content")
	}
	if len(content) > d.maxContentLength {
		return nil, ErrContentTooLong
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
	logutil.Debug("updated message %s by user %s", id, userID)
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
	logutil.Debug("deleted message %s by user %s", id, userID)
	return nil
}

// ── Mentions ─────────────────────────────────────────────────────────

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}


