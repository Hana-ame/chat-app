package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"strings"

	"github.com/Hana-ame/chat-app/server/internal/models"
)


// ── Threads（【本地改动 2026-08-31】移植 chatto 线程）─────────────────
// 线程模型：messages.thread_root_message_id 自引用。空串 = 顶层消息；
// == id = 该消息即线程根（start_thread=true 时写入）；非空其他值 = 回复在该根
// 下。reply_to_message_id 语义不变（父消息，用于线程内的嵌套回复）。
// thread_follows 是用户 opt-in 关注（UNIQUE(user_id, thread_root_message_id)），
// thread_read_state 是每个用户每线程的已读游标（last_seen_message_id 为空 → 未读）。

// FollowThread 关注一条线程，已关注则幂等跳过。
func (d *DB) FollowThread(ctx context.Context, userID, threadRootMessageID string) error {
	if userID == "" || threadRootMessageID == "" {
		return errInvalidInput
	}
	_, err := d.ExecContext(ctx,
		`INSERT INTO thread_follows (id, user_id, thread_root_message_id, created_at)
		 VALUES (?,?,?,?)
		 ON CONFLICT(user_id, thread_root_message_id) DO NOTHING`,
		NewID(), userID, threadRootMessageID, time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

// UnfollowThread 取关一条线程（幂等）。
func (d *DB) UnfollowThread(ctx context.Context, userID, threadRootMessageID string) error {
	if userID == "" || threadRootMessageID == "" {
		return errInvalidInput
	}
	_, err := d.ExecContext(ctx,
		`DELETE FROM thread_follows WHERE user_id = ? AND thread_root_message_id = ?`,
		userID, threadRootMessageID,
	)
	return err
}

// IsFollowingThread 是否关注。
func (d *DB) IsFollowingThread(ctx context.Context, userID, threadRootMessageID string) (bool, error) {
	var n int
	err := d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM thread_follows WHERE user_id = ? AND thread_root_message_id = ?`,
		userID, threadRootMessageID,
	).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ThreadFollowers 返回线程的所有关注者 ID。
func (d *DB) ThreadFollowers(ctx context.Context, threadRootMessageID string) ([]string, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT user_id FROM thread_follows WHERE thread_root_message_id = ?`,
		threadRootMessageID,
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
	return out, nil
}

// ThreadReplyCount 返回一条线程的回复数（thread_root_message_id == root 的所有消息，
// 包括根本身）。
func (d *DB) ThreadReplyCount(ctx context.Context, threadRootMessageID string) (int, error) {
	var n int
	err := d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM messages WHERE thread_root_message_id = ? AND deleted_at IS NULL`,
		threadRootMessageID,
	).Scan(&n)
	return n, err
}

// SetThreadRead 设置用户的线程已读游标到 lastSeenMessageID。
func (d *DB) SetThreadRead(ctx context.Context, userID, threadRootMessageID, lastSeenMessageID string) error {
	if userID == "" || threadRootMessageID == "" || lastSeenMessageID == "" {
		return errInvalidInput
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := d.ExecContext(ctx,
		`INSERT INTO thread_read_state (id, user_id, thread_root_message_id, last_seen_message_id, updated_at)
		 VALUES (?,?,?,?,?)
		 ON CONFLICT(user_id, thread_root_message_id)
		 DO UPDATE SET last_seen_message_id = ?, updated_at = ?`,
		NewID(), userID, threadRootMessageID, lastSeenMessageID, now,
		lastSeenMessageID, now,
	)
	return err
}

// GetThreadReadCursor 返回用户某线程的已读游标消息 ID；未读过返回空串。
func (d *DB) GetThreadReadCursor(ctx context.Context, userID, threadRootMessageID string) (string, error) {
	var lastSeen sql.NullString
	err := d.QueryRowContext(ctx,
		`SELECT last_seen_message_id FROM thread_read_state WHERE user_id = ? AND thread_root_message_id = ?`,
		userID, threadRootMessageID,
	).Scan(&lastSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if lastSeen.Valid {
		return lastSeen.String, nil
	}
	return "", nil
}

// ListFollowedThreads 列出用户关注的线程摘要（含根消息、回复数、最新回复、has_unread）。
// 返回的 Thread 结构包含根消息 + 聚合元数据；HasUnread 依据线程最新回复时间是否
// 严格晚于用户已读游标对应的时间戳。limit<=0 或 >100 时默认 100。
func (d *DB) ListFollowedThreads(ctx context.Context, userID string, before string, limit int) ([]models.ThreadSummary, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	// 1) 用户关注的所有 thread_root_message_id。
	rows, err := d.QueryContext(ctx,
		`SELECT tf.thread_root_message_id FROM thread_follows tf ORDER BY tf.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	var roots []string
	for rows.Next() {
		var rid string
		if err := rows.Scan(&rid); err != nil {
			rows.Close()
			return nil, err
		}
		roots = append(roots, rid)
	}
	rows.Close()
	if len(roots) == 0 {
		return []models.ThreadSummary{}, nil
	}

	placeholders := make([]string, len(roots))
	for i := range roots {
		placeholders[i] = "?"
	}
	// 2) 每个线程的最新回复时间 + 回复数 + chat_id。
	latestByRoot := map[string]struct {
		chatID       string
		lastReplyAt  sql.NullString
		replyCount   int
		replyUserID  string
	}{}
	latestRows, err := d.QueryContext(ctx,
		`SELECT m.chat_id, COUNT(m.id) AS reply_count, MAX(m.created_at) AS last_reply_at,
		        (SELECT m2.user_id FROM messages m2 WHERE m2.thread_root_message_id = m.thread_root_message_id
		        	 AND m2.deleted_at IS NULL ORDER BY m2.created_at DESC, m2.id DESC LIMIT 1) AS reply_user_id
		 FROM messages m
		 WHERE m.thread_root_message_id IN (`+strings.Join(placeholders, ",")+`)
		  AND m.deleted_at IS NULL
		 GROUP BY m.thread_root_message_id`,
		toAnySlice(roots)...
	)
	if err != nil {
		return nil, err
	}
	var rootID string
	for latestRows.Next() {
		var entry struct {
			chatID      string
			replyCount  int
			lastReplyAt sql.NullString
			replyUserID sql.NullString
		}
		if err := latestRows.Scan(&rootID, &entry.replyCount, &entry.lastReplyAt, &entry.replyUserID); err != nil {
			latestRows.Close()
			return nil, err
		}
		latestByRoot[rootID] = struct {
			chatID      string
			lastReplyAt sql.NullString
			replyCount  int
			replyUserID string
		}{entry.chatID, entry.lastReplyAt, entry.replyCount, entry.replyUserID.String}
	}
	latestRows.Close()

	// 3) 为每个根收集已读游标。
	cursors := map[string]string{}
	if len(roots) > 0 {
		cursorsRows, err := d.QueryContext(ctx,
			`SELECT thread_root_message_id, last_seen_message_id FROM thread_read_state WHERE user_id = ?`,
			userID,
		)
		if err == nil {
			for cursorsRows.Next() {
				var rid, lsm string
				if err := cursorsRows.Scan(&rid, &lsm); err == nil {
					cursors[rid] = lsm
				}
			}
			cursorsRows.Close()
		}
	}

	// 4) 组装：按 last_reply_at DESC（顶层/无回复线程回退到根消息 created_at DESC）。
	type rootInfo struct {
		root    string
		msg     *models.Message
		meta    models.ThreadMeta
		cursor  string
	}
	type rootMeta struct {
		lastReplyAt sql.NullString
		replyCount  int
		replyUserID string
		chatID      string
	}
	metaByRoot := map[string]rootMeta{}
	for r, v := range latestByRoot {
		metaByRoot[r] = rootMeta{v.lastReplyAt, v.replyCount, v.replyUserID, v.chatID}
	}

	infoList := make([]rootInfo, 0, len(roots))
	for _, r := range roots {
		msg, err := d.GetMessage(ctx, r)
		if err != nil {
			continue
		}
		latestMsgID := ""
		latestMsgAt := msg.CreatedAt
		mt := metaByRoot[r]
		if mt.lastReplyAt.Valid {
			latestMsgAt = parseTime(mt.lastReplyAt.String)
			// 找最新回复 ID
			var lmID string
			var err error
			if r != "" {
				lmID, err = d.LatestReplyIDForThread(ctx, r)
				if err != nil {
					lmID = ""
				}
			}
			latestMsgID = lmID
		}
		cursor := cursors[r]
		cursorMsg, err := d.GetMessage(ctx, cursor)
		hasUnread := false
		if cursorMsg != nil && (cursorMsg.CreatedAt.IsZero() || latestMsgAt.After(cursorMsg.CreatedAt)) {
			hasUnread = true
		}
		if cursorMsg == nil && (mt.lastReplyAt.Valid || r != "") {
			hasUnread = true
		}

		infoList = append(infoList, rootInfo{
			root: r,
			msg:  msg,
			meta: models.ThreadMeta{
				ThreadRootMessageID: r,
				ChatID:              mt.chatID,
				ReplyCount:          mt.replyCount - 1, // 减去根自身
				LastReplyAt:         latestMsgAt,
				LatestReplyID:       latestMsgID,
				IsFollowing:         true,
				HasUnread:           hasUnread,
			},
			cursor: cursor,
		})
	}

	// 排序：last_reply_at DESC，回退到根 created_at DESC，再 id 稳定。
	for i := 0; i < len(infoList); i++ {
		for j := i + 1; j < len(infoList); j++ {
			a := infoList[i]
			b := infoList[j]
			alat, blat := a.meta.LastReplyAt, b.meta.LastReplyAt
			if alat.Equal(blat) {
				if a.msg != nil && b.msg != nil {
					if a.msg.CreatedAt.Equal(b.msg.CreatedAt) {
						if a.meta.ThreadRootMessageID > b.meta.ThreadRootMessageID {
							infoList[i], infoList[j] = infoList[j], infoList[i]
						}
					} else if b.msg.CreatedAt.After(a.msg.CreatedAt) {
						infoList[i], infoList[j] = infoList[j], infoList[i]
					}
				}
			} else if blat.After(alat) {
				infoList[i], infoList[j] = infoList[j], infoList[i]
			}
		}
	}

	// 分页：before 是 cursor（上一个返回的 ThreadRootMessageID）。
	if before != "" {
		for i, it := range infoList {
			if it.root == before {
				infoList = infoList[i+1:]
				break
			}
		}
	}
	if limit > 0 && len(infoList) > limit {
		infoList = infoList[:limit]
	}

	out := make([]models.ThreadSummary, len(infoList))
	for i, it := range infoList {
		out[i] = models.ThreadSummary{
			RootMessage: it.msg,
			Meta:        it.meta,
		}
	}
	return out, nil
}

// ThreadSummary 返回单条线程详情：根消息、根所在 chat 名/图标色/成员数、回复数、最新回复时间、has_unread、is_following。
func (d *DB) GetThreadSummary(ctx context.Context, chatID, userID, threadRootMessageID string) (*models.ThreadSummary, error) {
	if threadRootMessageID == "" {
		return nil, errInvalidInput
	}
	rootMsg, err := d.GetMessage(ctx, threadRootMessageID)
	if err != nil {
		return nil, err
	}
	if rootMsg.ChatID != chatID {
		return nil, ErrNotFound
	}
	replyCount, err := d.ThreadReplyCount(ctx, threadRootMessageID)
	if err != nil {
		return nil, err
	}
	following, err := d.IsFollowingThread(ctx, userID, threadRootMessageID)
	if err != nil {
		return nil, err
	}
	cursor, err := d.GetThreadReadCursor(ctx, userID, threadRootMessageID)
	if err != nil {
		return nil, err
	}
	latestMsgAt := rootMsg.CreatedAt
	latestMsgID := ""
	lid, err := d.LatestReplyIDForThread(ctx, threadRootMessageID)
	if err == nil && lid != "" {
		latestMsgID = lid
		latest, err := d.GetMessage(ctx, lid)
		if err == nil && !latest.CreatedAt.IsZero() {
			latestMsgAt = latest.CreatedAt
		}
	}
	cursorMsg, err := d.GetMessage(ctx, cursor)
	hasUnread := false
	if cursorMsg != nil && (cursorMsg.CreatedAt.IsZero() || latestMsgAt.After(cursorMsg.CreatedAt)) {
		hasUnread = true
	}
	if cursorMsg == nil && (latestMsgID != "" || !latestMsgAt.Equal(rootMsg.CreatedAt)) {
		hasUnread = true
	}
	meta := models.ThreadMeta{
		ThreadRootMessageID: threadRootMessageID,
		ChatID:              chatID,
		ReplyCount:          replyCount - 1,
		LastReplyAt:         latestMsgAt,
		LatestReplyID:       latestMsgID,
		IsFollowing:         following,
		HasUnread:           hasUnread,
	}
	return &models.ThreadSummary{RootMessage: rootMsg, Meta: meta}, nil
}

func (d *DB) LatestReplyIDForThread(ctx context.Context, threadRootMessageID string) (string, error) {
	var id sql.NullString
	err := d.QueryRowContext(ctx,
		`SELECT m.id FROM messages m
		 WHERE m.thread_root_message_id = ? AND m.deleted_at IS NULL
		 ORDER BY m.created_at DESC, m.id DESC LIMIT 1`,
		threadRootMessageID,
	).Scan(&id)
	if err != nil {
		return "", err
	}
	if !id.Valid {
		return "", nil
	}
	return id.String, nil
}

// ThreadReplies 返回线程内回复（按 created_at ASC），不含根。
func (d *DB) ThreadReplies(ctx context.Context, threadRootMessageID string, before string, limit int) ([]models.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if before == "" {
		return d.fetchThreadReplies(ctx,
			`SELECT `+messageColumns+messageJoins+`
			 WHERE m.thread_root_message_id = ? AND m.id != ?
			 ORDER BY m.created_at ASC, m.id ASC LIMIT ?`,
			threadRootMessageID, threadRootMessageID, limit,
		)
	}
	return d.fetchThreadReplies(ctx,
		`SELECT `+messageColumns+messageJoins+`
		 WHERE m.thread_root_message_id = ? AND m.id != ? AND m.created_at < (SELECT created_at FROM messages WHERE id = ?)
		 ORDER BY m.created_at DESC, m.id DESC LIMIT ?`,
		threadRootMessageID, threadRootMessageID, before, limit,
	)
}

func (d *DB) fetchThreadReplies(ctx context.Context, q string, args ...any) ([]models.Message, error) {
	rows, err := d.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	msgs := []models.Message{}
	for rows.Next() {
		m, err := d.scanMessage(rows)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, *m)
	}
	// before 分页是倒序，需要反转回来按时间升序。
	if len(args) >= 4 {
		reverseSlice(msgs)
	}
	return msgs, nil
}

func reverseSlice(ms []models.Message) {
	for i, j := 0, len(ms)-1; i < j; i, j = i+1, j-1 {
		ms[i], ms[j] = ms[j], ms[i]
	}
}

func toAnySlice(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}
