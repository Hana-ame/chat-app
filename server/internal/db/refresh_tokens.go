package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

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

// FindAndDeleteRefreshToken atomically finds and deletes a refresh token
// in a single transaction, preventing concurrent consumption.
func (d *DB) FindAndDeleteRefreshToken(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var rt models.RefreshToken
	var expires, created string
	err = tx.QueryRowContext(ctx,
		`SELECT id, user_id, token_hash, expires_at, created_at FROM refresh_tokens WHERE token_hash = ?`,
		tokenHash,
	).Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &expires, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE id = ?`, rt.ID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	rt.ExpiresAt = parseTime(expires)
	rt.CreatedAt = parseTime(created)
	return &rt, nil
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
