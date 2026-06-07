package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

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
	h := 0
	for _, r := range seed {
		h = (h*31 + int(r)) & 0x7fffffff
	}
	return palette[h%len(palette)]
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func parseTimePtr(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	t := parseTime(s.String)
	if t.IsZero() {
		return nil
	}
	return &t
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
			return nil, ErrConflict
		}
		return nil, err
	}
	return d.GetUserByID(ctx, id)
}

func (d *DB) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	var (
		u         models.User
		createdAt string
	)
	err := d.QueryRowContext(ctx,
		`SELECT id, email, username, avatar_color, status, created_at FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Email, &u.Username, &u.AvatarColor, &u.Status, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.CreatedAt = parseTime(createdAt)
	return &u, nil
}

func (d *DB) GetUserByEmail(ctx context.Context, email string) (*models.User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var (
		u         models.User
		pwHash    string
		createdAt string
	)
	err := d.QueryRowContext(ctx,
		`SELECT id, email, username, avatar_color, status, created_at, password_hash FROM users WHERE email = ?`,
		email,
	).Scan(&u.ID, &u.Email, &u.Username, &u.AvatarColor, &u.Status, &createdAt, &pwHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	u.CreatedAt = parseTime(createdAt)
	return &u, pwHash, nil
}

func (d *DB) UpdateUserProfile(ctx context.Context, id, username, avatarColor string) (*models.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("username required")
	}
	if avatarColor == "" {
		avatarColor = PickColor(username)
	}
	_, err := d.ExecContext(ctx,
		`UPDATE users SET username = ?, avatar_color = ? WHERE id = ?`,
		username, avatarColor, id,
	)
	if err != nil {
		return nil, err
	}
	return d.GetUserByID(ctx, id)
}

func (d *DB) UpdateUserStatus(ctx context.Context, id, status string) error {
	_, err := d.ExecContext(ctx, `UPDATE users SET status = ? WHERE id = ?`, status, id)
	return err
}

func (d *DB) SearchUsers(ctx context.Context, query string, limit int) ([]models.User, error) {
	if limit <= 0 || limit > 50 {
		limit = 25
	}
	query = strings.TrimSpace(query)
	rows, err := d.QueryContext(ctx,
		`SELECT id, username, avatar_color, status, created_at FROM users
		 WHERE username LIKE ? OR email LIKE ?
		 ORDER BY username LIMIT ?`,
		"%"+query+"%", "%"+strings.ToLower(query)+"%", limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.User{}
	for rows.Next() {
		var u models.User
		var createdAt string
		if err := rows.Scan(&u.ID, &u.Username, &u.AvatarColor, &u.Status, &createdAt); err != nil {
			return nil, err
		}
		u.CreatedAt = parseTime(createdAt)
		out = append(out, u)
	}
	return out, rows.Err()
}
