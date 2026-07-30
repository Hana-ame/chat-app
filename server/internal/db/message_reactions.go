package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

func (d *DB) syncReactionsColumn(ctx context.Context, tx *sql.Tx, messageID string) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT emoji, user_id FROM reactions WHERE message_id = ? ORDER BY created_at`,
		messageID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	grouped := map[string]*models.Reaction{}
	order := []string{}
	for rows.Next() {
		var emoji, uid string
		if err := rows.Scan(&emoji, &uid); err != nil {
			return err
		}
		if r, ok := grouped[emoji]; ok {
			r.Count++
		} else {
			r := &models.Reaction{Emoji: emoji, Count: 1}
			grouped[emoji] = r
			order = append(order, emoji)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	out := make([]models.Reaction, 0, len(order))
	for _, e := range order {
		out = append(out, *grouped[e])
	}
	data, err := json.Marshal(out)
	if err != nil {
		data = []byte("[]")
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE messages SET reactions = ? WHERE id = ?`,
		string(data), messageID,
	)
	return err
}

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
	if err := d.syncReactionsColumn(ctx, tx, messageID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	logutil.Debug("added reaction %s to message %s by %s", emoji, messageID, userID)
	return nil
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
	if err := d.syncReactionsColumn(ctx, tx, messageID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
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
