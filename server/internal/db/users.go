package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")

func NewID() string { return uuid.NewString() }

var palette = []string{
	"#5865F2", "#3BA55D", "#FAA61A", "#ED4245",
	"#EB459E", "#9b59b6", "#1abc9c", "#e67e22",
}

func PickColor(seed string) string {
	if seed == "" {
		return palette[0]
	}
	id, err := uuid.Parse(seed)
	if err != nil {
		return palette[0]
	}
	h := int(id.ID())
	return palette[h%len(palette)]
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

// ── Users ────────────────────────────────────────────────────────────

func (d *DB) CreateUser(ctx context.Context, email, username, passwordHash string) (*models.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	username = strings.TrimSpace(username)
	if email == "" || username == "" {
		return nil, errors.New("email and username required")
	}
	id := NewID()
	color := PickColor(username)
	_, err := d.ExecContext(ctx,
		`INSERT INTO users (id, email, username, password_hash, avatar_color) VALUES (?,?,?,?,?)`,
		id, email, username, passwordHash, color,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			logutil.Warn("create user conflict: email=%s username=%s", email, username)
			return nil, ErrConflict
		}
		return nil, err
	}
	logutil.Info("created user %s (username=%s email=%s)", id, username, email)
	return d.GetUserByID(ctx, id)
}

func (d *DB) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	var (
		u            models.User
		lastSeen     string
		createdAt    string
		notifyBlocked sql.NullString
	)
	err := d.QueryRowContext(ctx,
		`SELECT id, email, username, avatar_color, avatar_url, status, last_seen, created_at, notify_blocked FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Email, &u.Username, &u.AvatarColor, &u.AvatarURL, &u.Status, &lastSeen, &createdAt, &notifyBlocked)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.LastSeen = parseTime(lastSeen)
	u.CreatedAt = parseTime(createdAt)
	if notifyBlocked.Valid && notifyBlocked.String != "" {
		if err := json.Unmarshal([]byte(notifyBlocked.String), &u.NotifyBlocked); err != nil {
			u.NotifyBlocked = []string{}
		}
	} else {
		u.NotifyBlocked = []string{}
	}
	return &u, nil
}

func (d *DB) GetUserByEmail(ctx context.Context, email string) (*models.User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var (
		u            models.User
		pwHash       string
		lastSeen     string
		createdAt    string
		notifyBlocked sql.NullString
	)
	err := d.QueryRowContext(ctx,
		`SELECT id, email, username, avatar_color, avatar_url, status, last_seen, created_at, password_hash, notify_blocked FROM users WHERE email = ?`,
		email,
	).Scan(&u.ID, &u.Email, &u.Username, &u.AvatarColor, &u.AvatarURL, &u.Status, &lastSeen, &createdAt, &pwHash, &notifyBlocked)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	u.LastSeen = parseTime(lastSeen)
	u.CreatedAt = parseTime(createdAt)
	if notifyBlocked.Valid && notifyBlocked.String != "" {
		if err := json.Unmarshal([]byte(notifyBlocked.String), &u.NotifyBlocked); err != nil {
			u.NotifyBlocked = []string{}
		}
	} else {
		u.NotifyBlocked = []string{}
	}
	return &u, pwHash, nil
}

func (d *DB) UpdateUserProfile(ctx context.Context, id, username, avatarColor, avatarURL string) (*models.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("username required")
	}
	if avatarColor == "" {
		avatarColor = PickColor(username)
	}
	var err error
	if avatarURL == "" {
		_, err = d.ExecContext(ctx,
			`UPDATE users SET username = ?, avatar_color = ? WHERE id = ?`,
			username, avatarColor, id,
		)
	} else {
		_, err = d.ExecContext(ctx,
			`UPDATE users SET username = ?, avatar_color = ?, avatar_url = ? WHERE id = ?`,
			username, avatarColor, avatarURL, id,
		)
	}
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrConflict
		}
		return nil, err
	}
	return d.GetUserByID(ctx, id)
}

func (d *DB) UpdateUserStatus(ctx context.Context, id, status string) error {
	_, err := d.ExecContext(ctx, `UPDATE users SET status = ? WHERE id = ?`, status, id)
	return err
}

func (d *DB) UpdateUserLastSeen(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := d.ExecContext(ctx, `UPDATE users SET last_seen = ? WHERE id = ?`, now, id)
	return err
}

func (d *DB) GetUserNotifyBlocked(ctx context.Context, userID string) ([]string, error) {
	var raw string
	err := d.QueryRowContext(ctx,
		`SELECT COALESCE(notify_blocked, '[]') FROM users WHERE id = ?`, userID,
	).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		ids = []string{}
	}
	return ids, nil
}

func (d *DB) SetUserNotifyBlocked(ctx context.Context, userID string, blocked []string) error {
	b, err := json.Marshal(blocked)
	if err != nil {
		return err
	}
	_, err = d.ExecContext(ctx,
		`UPDATE users SET notify_blocked = ? WHERE id = ?`, string(b), userID,
	)
	return err
}

func (d *DB) SearchUsers(ctx context.Context, query string, limit int) ([]models.User, error) {
	if limit <= 0 || limit > 50 {
		limit = 25
	}
	query = strings.TrimSpace(query)
	rows, err := d.QueryContext(ctx,
		`SELECT id, username, avatar_color, avatar_url, status, last_seen, created_at FROM users
		 WHERE username LIKE ? OR id = ?
		 ORDER BY username LIMIT ?`,
		"%"+query+"%", query, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.User{}
	for rows.Next() {
		var u models.User
		var lastSeen, createdAt string
		if err := rows.Scan(&u.ID, &u.Username, &u.AvatarColor, &u.AvatarURL, &u.Status, &lastSeen, &createdAt); err != nil {
			return nil, err
		}
		u.LastSeen = parseTime(lastSeen)
		u.CreatedAt = parseTime(createdAt)
		out = append(out, u)
	}
	return out, rows.Err()
}
