package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

// ── Chat Pins（【本地改动 2026-09-02】chatto FDR-037 多消息置顶）────────────

// PinnedMessage 表示一条置顶记录 + 其指向的消息内容。
// 注意：此结构是展示层投影；chat_pins 表只存关联元数据，message 字段由
// ListPinnedMessages 用 JOIN 组装。
type PinnedMessage struct {
	ChatID     string    `json:"chat_id"`
	MessageID  string    `json:"message_id"`
	PinnedBy   string    `json:"pinned_by"`
	PinnedAt   time.Time `json:"pinned_at"`
	Message    models.Message
}

// PinMessage 在 chat 中置顶 message。幂等：已置顶 → 返回 true、无错误。
// 返回 (alreadyExisted, error)。DM chat 由 service 层拒绝。
// 【本地改动 2026-09-02】踩坑：SQLite ON CONFLICT DO NOTHING 的 RowsAffected 无
// 法定判断是新插还是已存在——用 SELECT ... WHERE 预检，再 INSERT。
func (d *DB) PinMessage(ctx context.Context, chatID, messageID, actorID string) (alreadyExisted bool, err error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	id := NewID()
	rows, err := d.QueryContext(ctx,
		`SELECT id FROM chat_pins WHERE chat_id = ? AND message_id = ?`,
		chatID, messageID,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if rows.Next() {
		return true, nil // 幂等：已置顶
	}
	_, err = d.ExecContext(ctx,
		`INSERT INTO chat_pins (id, chat_id, message_id, pinned_by, created_at) VALUES (?,?,?,?,?)`,
		id, chatID, messageID, actorID, now,
	)
	if err != nil {
		return false, err
	}
	logutil.Debug("pinned message %s in chat %s by %s", messageID, chatID, actorID)
	return false, nil
}

// UnpinMessage 取消 chat 中 message 的置顶。幂等：未置顶 → 返回 false、无错误。
// 返回 (wasPinned, error)。
func (d *DB) UnpinMessage(ctx context.Context, chatID, messageID string) (wasPinned bool, err error) {
	res, err := d.ExecContext(ctx,
		`DELETE FROM chat_pins WHERE chat_id = ? AND message_id = ?`,
		chatID, messageID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	logutil.Debug("unpinned message %s in chat %s (rows=%d)", messageID, chatID, n)
	return n > 0, nil
}

// ListPinnedMessages 返回 chat 的置顶消息列表，created_at DESC（最新置顶优先），
// 支持 cursor-based 分页 (before message_id) 与 limit。
// 每条 pin 携带原始 message 对象（含 author 等）。
func (d *DB) ListPinnedMessages(ctx context.Context, chatID string, before string, limit int) ([]PinnedMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	q := `SELECT cp.id, cp.chat_id, cp.message_id, cp.pinned_by, cp.created_at,
		         m.id, m.chat_id, m.user_id, m.content, m.created_at, m.edited_at, m.deleted_at,
		         m.attachment_count, m.mention_count, m.reaction_count, m.attachments, m.mentions, m.reactions,
		         m.thread_root_message_id
		  FROM chat_pins cp
		  LEFT JOIN messages m ON cp.message_id = m.id
		  WHERE cp.chat_id = ?`
	args := []any{chatID}
	if before != "" {
		q += ` AND cp.created_at < ?`
		args = append(args, before)
	}
	q += ` ORDER BY cp.created_at DESC LIMIT ?`
	args = append(args, limit+1)

	rows, err := d.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PinnedMessage
	for rows.Next() {
		var cp ChatPinRow
		var msg models.Message
		if err := rows.Scan(
			&cp.ID, &cp.ChatID, &cp.MessageID, &cp.PinnedBy, &cp.CreatedAt,
			&msg.ID, &msg.ChatID, &msg.UserID, &msg.Content, &msg.CreatedAt, &msg.EditedAt, &msg.DeletedAt,
			&msg.AttachmentCount, &msg.MentionCount, &msg.ReactionCount, &msg.Attachments, &msg.Mentions, &msg.Reactions,
			&msg.ThreadRootMessageID,
		); err != nil {
			return nil, err
		}
		// 将 ChatPinRow.CreatedAt 转为 time.Time
		pinnedAt, err := time.Parse(time.RFC3339Nano, cp.CreatedAt)
		if err != nil {
			// 兜底：如果解析失败用 message created_at
			pinnedAt = msg.CreatedAt
		}
		out = append(out, PinnedMessage{
			ChatID:    cp.ChatID,
			MessageID: cp.MessageID,
			PinnedBy:  cp.PinnedBy,
			PinnedAt:  pinnedAt,
			Message:   msg,
		})
	}
	// 如果多取了一条（limit+1），说明还有更多，截断
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type ChatPinRow struct {
	ID        string
	ChatID    string
	MessageID string
	PinnedBy  string
	CreatedAt string
}

// HasPin 检查 chat 中某 message 是否已被置顶。
func (d *DB) HasPin(ctx context.Context, chatID, messageID string) (bool, error) {
	var exists int
	err := d.QueryRowContext(ctx,
		`SELECT 1 FROM chat_pins WHERE chat_id = ? AND message_id = ? LIMIT 1`,
		chatID, messageID,
	).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// RemovePinsForChat 删除某 chat 的所有置顶。用于 chat 归档/删除时清理。
// 通常由 FK CASCADE 自动处理，此处供 service 层手动触发。
func (d *DB) RemovePinsForChat(ctx context.Context, chatID string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM chat_pins WHERE chat_id = ?`, chatID)
	return err
}

// RemovePinsForMessage 删除某 message 的所有置顶（跨 chat）。
// 用于消息软删除/删除时清理关联 pin。
func (d *DB) RemovePinsForMessage(ctx context.Context, messageID string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM chat_pins WHERE message_id = ?`, messageID)
	return err
}

// ── 兼容层 ───────────────────────────────────────────────────────────
// 【本地改动 2026-09-02】旧 schema 中 chat.pinned_message 是"聊天公告"（自写文本），
// 与本文件的多消息置顶（chat_pins 表）是两个独立功能；此处无兼容代码需要，
// 保留注释仅作架构说明。
