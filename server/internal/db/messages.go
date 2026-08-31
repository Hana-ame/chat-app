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
		        COALESCE(m.reply_to_message_id,''), COALESCE(m.thread_root_message_id,''),
		        u.id, u.username, u.avatar_color, u.avatar_url, u.status, u.last_seen, COALESCE(cm.role,'')`

const messageJoins = ` FROM messages m JOIN users u ON u.id = m.user_id
		 LEFT JOIN chat_members cm ON cm.chat_id = m.chat_id AND cm.user_id = m.user_id`

// 【本地改动 2026-09-03】FTS5 索引维护：与 messages 表同步。
// upsertFTS 用 INSERT OR REPLACE（FTS5 对 msg_id UNINDEXED 列隐式 UNIQUE，重复写入自动替换）。
// deleteFTS 显式删行。注意：不用 FTS5 rebuild 控制命令，modernc.org/sqlite 对
// FTS5 external content 支持不完整（rowid 强制 INTEGER，与 UUID msg ID 冲突）。
func upsertFTS(tx dbTx, ctx context.Context, msgID, content string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO messages_fts(content, msg_id) VALUES(?, ?)`,
		content, msgID,
	)
	return err
}
func deleteFTS(tx dbTx, ctx context.Context, msgID string) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM messages_fts WHERE msg_id = ?`, msgID)
	return err
}

type dbTx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}


func (d *DB) CreateMessage(ctx context.Context, chatID, userID, content string, mentions []string, attachments []models.Attachment, opts ...CreateMessageOpt) (*models.Message, error) {
	o := &struct {
		replyTo    string
		threadRoot string
	}{}
	for _, f := range opts {
		f(o)
	}

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
	replyToID := o.replyTo
	threadRootID := o.threadRoot
	_, err = tx.ExecContext(ctx,
		`INSERT INTO messages (id, chat_id, user_id, content, created_at, attachment_count, mention_count, attachments, mentions, reply_to_message_id, thread_root_message_id) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		id, chatID, userID, content, now, len(attachments), len(dedupe(mentions)), string(attJSON), string(mentJSON), replyToID, threadRootID,
	)
	if err != nil {
		return nil, err
	}
	// 【本地改动 2026-09-03】同步 FTS5 索引。
	if err := upsertFTS(tx, ctx, id, content); err != nil {
		return nil, err
	}
	// 【本地改动 2026-08-31】线程根自引用：StartThread 时 threadRootID=="__SELF__"，
	// 插入后回写为本消息 id（自引用），从而把该消息标记为线程根。
	if threadRootID == "__SELF__" {
		_, err = tx.ExecContext(ctx,
			`UPDATE messages SET thread_root_message_id = ? WHERE id = ?`,
			id, id,
		)
		if err != nil {
			return nil, err
		}
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

// 【本地改动 2026-08-31】线程：err_invalid_input 是 CreateMessage 的输入错误。
var errInvalidInput = errors.New("invalid input")

// CreateMessageOpt 是 CreateMessage 的可选参数（【本地改动 2026-08-31】线程：
// 分别标识直接引用和线程归属的根 ID）。
type CreateMessageOpt func(*struct { replyTo, threadRoot string })

func WithReplyTo(replyTo string) CreateMessageOpt {
	return func(o *struct { replyTo, threadRoot string }) { o.replyTo = replyTo }
}
func WithThreadRoot(threadRoot string) CreateMessageOpt {
	return func(o *struct { replyTo, threadRoot string }) { o.threadRoot = threadRoot }
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
		&m.ReplyTo, &m.ThreadRootMessageID,
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
func (d *DB) GetMessages(ctx context.Context, chatID, before string, limit int, inThread ...string) ([]models.Message, error) {
	threadFilter := ""
	if len(inThread) > 0 && inThread[0] != "" {
		threadFilter = " AND m.thread_root_message_id = ?"
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	var (
		rows *sql.Rows
		err  error
	)
	if before == "" {
		if threadFilter != "" {
			rows, err = d.QueryContext(ctx,
				`SELECT `+messageColumns+messageJoins+` WHERE m.chat_id = ?`+threadFilter+`
				 ORDER BY m.created_at DESC, m.id DESC LIMIT ?`,
				chatID, inThread[0], limit,
			)
		} else {
			rows, err = d.QueryContext(ctx,
				`SELECT `+messageColumns+messageJoins+` WHERE m.chat_id = ?
				 ORDER BY m.created_at DESC, m.id DESC LIMIT ?`,
				chatID, limit,
			)
		}
	} else {
		if threadFilter != "" {
			rows, err = d.QueryContext(ctx,
				`SELECT `+messageColumns+messageJoins+` WHERE m.chat_id = ?`+threadFilter+` AND (m.created_at, m.id) < (
				    SELECT created_at, id FROM messages WHERE id = ?
				 )
				 ORDER BY m.created_at DESC, m.id DESC LIMIT ?`,
				chatID, inThread[0], before, limit,
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
	// 【本地改动 2026-09-03】同步 FTS5 索引（重新插入以覆盖旧内容）。
	if err := upsertFTS(d, ctx, id, content); err != nil {
		return nil, err
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
	// 【本地改动 2026-09-03】消息软删除时同步清理 FTS 索引（否则搜索仍会命中已删除消息）。
	_ = deleteFTS(d, ctx, id)
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



// ── 【本地改动 2026-09-03】FTS5 消息搜索 ──────────────────────────────

// SearchMessagesInput 是 SearchMessages 的查询参数。
type SearchMessagesInput struct {
	Query   string // FTS5 MATCH 表达式（空格分词，多词 OR；"" 强制精确短语；foo AND bar 要求同时出现）
	ChatID  string // 可选：限定在某聊天内
	UserID  string // 可选：限定某用户
	ActorID string // 可选：当 ChatID 为空时，用 chat_members 子查询强制访问控制
	Before  string // 可选：created_at 游标（严格小于）
	Limit   int    // 默认 50，最大 100
}

// SearchResult 是 SearchMessages 的返回（含 has_more 分页提示）。
type SearchResult struct {
	Messages []models.Message
	HasMore  bool
	Total    int // 本次返回条数
}

// SearchMessages 用 FTS5 MATCH 搜索消息。
// 已删除消息（deleted_at != null）不返回；content 列空的消息也不返回。
// 返回按 created_at DESC 排序（+1 多取一条判 has_more）。
func (d *DB) SearchMessages(ctx context.Context, in SearchMessagesInput) (*SearchResult, error) {
	if in.Query == "" {
		return &SearchResult{}, nil
	}
	if in.Limit <= 0 || in.Limit > 100 {
		in.Limit = 50
	}

	// 构造 WHERE + args。FTS5 MATCH 放在最前，配合索引。
	q := `SELECT ` + messageColumns + messageJoins + `
		INNER JOIN messages_fts f ON f.msg_id = m.id
		WHERE f.content MATCH ?
		  AND m.deleted_at IS NULL`
	args := []any{in.Query}
	if in.ChatID != "" {
		q += ` AND m.chat_id = ?`
		args = append(args, in.ChatID)
	} else if in.ActorID != "" {
		// 【本地改动 2026-09-03】ChatID 为空时（全局搜索）：用 chat_members 子查询强制访问控制，
		// 防止用户搜到非成员聊天的消息。踩坑：若此处不强制，用户可越权搜索任意消息。
		q += ` AND EXISTS (SELECT 1 FROM chat_members WHERE chat_id = m.chat_id AND user_id = ?)`
		args = append(args, in.ActorID)
	}
	if in.UserID != "" {
		q += ` AND m.user_id = ?`
		args = append(args, in.UserID)
	}
	if in.Before != "" {
		q += ` AND m.created_at < ?`
		args = append(args, in.Before)
	}
	q += ` ORDER BY m.created_at DESC, m.id DESC LIMIT ?`
	args = append(args, in.Limit+1)

	rows, err := d.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Message
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

	hasMore := len(out) > in.Limit
	if hasMore {
		out = out[:in.Limit]
	}
	return &SearchResult{Messages: out, HasMore: hasMore, Total: len(out)}, nil
}

// 【本地改动 2026-09-03】BackfillFTS 对未索引的消息做 FTS 回填。
// 启动时调用一次：扫描 messages 中无对应 FTS 行且有 content 的消息，逐条插入。
// 避免对大库做批量 INSERT OR REPLACE（可能导致 FTS 表瞬间膨胀）。
// 返回 (indexedCount, skippedCount)。
func (d *DB) BackfillFTS(ctx context.Context) (indexed int, skipped int, err error) {
	// 找所有 content 非空但 FTS 表未索引的消息
	rows, err := d.QueryContext(ctx,
		`SELECT id, content FROM messages WHERE content != '' AND content IS NOT NULL
		   AND id NOT IN (SELECT msg_id FROM messages_fts)`,
	)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, content string
		if err := rows.Scan(&id, &content); err != nil {
			return indexed, skipped, err
		}
		if err := upsertFTS(d, ctx, id, content); err != nil {
			skipped++
			continue
		}
		indexed++
	}
	return indexed, skipped, rows.Err()
}
