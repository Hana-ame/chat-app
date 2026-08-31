package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

const (
	// notificationOccurrenceTTL 是通知的存活期（移植 chatto 的 90 天 TTL 语义）。
	// 过期行由 chatd 启动的定期 worker 删除（见 cmd/chatd/main.go 的清理循环）。
	notificationOccurrenceTTL = 90 * 24 * time.Hour
)

// newNotificationOccurrence 构造一条待持久化的通知。id 由调用方通过 NewID 分配。
func newNotificationOccurrence(userID, kind, chatID, messageID, actorID, title, body string, now time.Time) *models.NotificationOccurrence {
	return &models.NotificationOccurrence{
		ID:        NewID(),
		UserID:    userID,
		Kind:      kind,
		ChatID:    chatID,
		MessageID: messageID,
		ActorID:   actorID,
		Title:     title,
		Body:      body,
		Read:      false,
		CreatedAt: now.UTC(),
		ExpiresAt: now.UTC().Add(notificationOccurrenceTTL),
	}
}

// CreateNotificationOccurrence 幂等插入一条通知：(user_id, kind, chat_id,
// message_id) 由迁移 005 的唯一索引保证每用户每事件唯一，重复触发撞唯一
// 约束时（created=false）直接返回，不重复插行、也不把已读改成未读。
// 并发安全：依赖 SQLite 唯一索引兜底（与 notify chat 的「锁 + 唯一索引 +
// 冲突回退」同思路；这里是单一 INSERT，无需应用层锁）。
func (d *DB) CreateNotificationOccurrence(ctx context.Context, userID, kind, chatID, messageID, actorID, title, body string, now time.Time) (created bool, err error) {
	occ := newNotificationOccurrence(userID, kind, chatID, messageID, actorID, title, body, now)
	res, err := d.ExecContext(ctx,
		`INSERT INTO notification_occurrences
		   (id, user_id, kind, chat_id, message_id, actor_id, title, body, read, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		occ.ID, occ.UserID, occ.Kind, occ.ChatID, occ.MessageID, occ.ActorID, occ.Title, occ.Body,
		occ.CreatedAt.Format(time.RFC3339Nano), occ.ExpiresAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return false, nil
		}
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// GetNotificationOccurrenceByKey 按唯一键 (user_id, kind, chat_id, message_id)
// 取回刚创建的行（广播实时事件需要行的 id 等完整字段）。无行时返回
// ErrNotFound。
func (d *DB) GetNotificationOccurrenceByKey(ctx context.Context, userID, kind, chatID, messageID string) (*models.NotificationOccurrence, error) {
	const q = `SELECT id, user_id, kind, chat_id, message_id, actor_id, title, body, read, created_at, expires_at
	           FROM notification_occurrences
	           WHERE user_id = ? AND kind = ? AND chat_id = ? AND message_id = ?`
	var occ models.NotificationOccurrence
	var read int
	var createdAt, expiresAt string
	err := d.QueryRowContext(ctx, q, userID, kind, chatID, messageID).Scan(
		&occ.ID, &occ.UserID, &occ.Kind, &occ.ChatID, &occ.MessageID, &occ.ActorID,
		&occ.Title, &occ.Body, &read, &createdAt, &expiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	occ.Read = read != 0
	occ.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	occ.ExpiresAt, _ = time.Parse(time.RFC3339Nano, expiresAt)
	return &occ, nil
}

// ListNotificationOccurrences 返回某用户最近的持久化通知（新→旧）。before
// 为分页游标（created_at < before 的时间戳）；limit<=0 时用默认 50。
func (d *DB) ListNotificationOccurrences(ctx context.Context, userID string, before string, limit int) ([]models.NotificationOccurrence, error) {
	if limit <= 0 {
		limit = 50
	}
	const q = `SELECT id, user_id, kind, chat_id, message_id, actor_id, title, body, read, created_at, expires_at
	           FROM notification_occurrences
	           WHERE user_id = ? AND (? = '' OR created_at < ?)
	           ORDER BY created_at DESC, id DESC
	           LIMIT ?`
	rows, err := d.QueryContext(ctx, q, userID, before, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.NotificationOccurrence, 0)
	for rows.Next() {
		var occ models.NotificationOccurrence
		var read int
		var createdAt, expiresAt string
		if err := rows.Scan(&occ.ID, &occ.UserID, &occ.Kind, &occ.ChatID, &occ.MessageID,
			&occ.ActorID, &occ.Title, &occ.Body, &read, &createdAt, &expiresAt); err != nil {
			return nil, err
		}
		occ.Read = read != 0
		occ.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		occ.ExpiresAt, _ = time.Parse(time.RFC3339Nano, expiresAt)
		out = append(out, occ)
	}
	return out, rows.Err()
}

// CountUnreadNotificationOccurrences 返回未读通知数（含已过期但未清理的，
// 清理 worker 会随后删除）。
func (d *DB) CountUnreadNotificationOccurrences(ctx context.Context, userID string) (int, error) {
	var n int
	err := d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notification_occurrences WHERE user_id = ? AND read = 0`,
		userID,
	).Scan(&n)
	return n, err
}

// MarkNotificationOccurrenceRead 把某条通知标记已读；只允许该通知的 owner
// 操作（WHERE user_id = ?），返回 ErrNotFound 当不存在或不属于该用户。
func (d *DB) MarkNotificationOccurrenceRead(ctx context.Context, id, userID string) error {
	res, err := d.ExecContext(ctx,
		`UPDATE notification_occurrences SET read = 1 WHERE id = ? AND user_id = ?`,
		id, userID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkAllNotificationOccurrencesRead 将该用户全部通知标记已读。
func (d *DB) MarkAllNotificationOccurrencesRead(ctx context.Context, userID string) error {
	_, err := d.ExecContext(ctx,
		`UPDATE notification_occurrences SET read = 1 WHERE user_id = ?`,
		userID,
	)
	return err
}

// DeleteNotificationOccurrence 删除某条通知（仅 owner，WHERE user_id = ?）。
func (d *DB) DeleteNotificationOccurrence(ctx context.Context, id, userID string) error {
	res, err := d.ExecContext(ctx,
		`DELETE FROM notification_occurrences WHERE id = ? AND user_id = ?`,
		id, userID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// PruneExpiredNotificationOccurrences 删除全部已过期通知（worker 用）。
func (d *DB) PruneExpiredNotificationOccurrences(ctx context.Context, now time.Time) (int64, error) {
	res, err := d.ExecContext(ctx,
		`DELETE FROM notification_occurrences WHERE expires_at <= ?`,
		now.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n > 0 {
		logutil.Info("pruned expired notification occurrences: %d", n)
	}
	return n, nil
}

// isUniqueViolation 判断 SQLite 唯一约束冲突（错误文本含 UNIQUE）。
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE")
}
