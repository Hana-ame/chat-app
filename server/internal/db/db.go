package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type DB struct {
	*sql.DB
	maxContentLength int
}

type goMigration struct {
	version int
	fn      func(ctx context.Context, d *DB) error
}

var goMigrations = []goMigration{
	{1, migrateV1EnsureColumns},
	{2, migrateV2DropChatTypeCheck},
}

func migrateV1EnsureColumns(ctx context.Context, d *DB) error {
	if err := d.ensureLastActiveColumn(ctx); err != nil {
		return err
	}
	columns := []struct {
		table, name, definition string
	}{
		{"chats", "avatar_url", "avatar_url TEXT NOT NULL DEFAULT ''"},
		{"chats", "banner_url", "banner_url TEXT NOT NULL DEFAULT ''"},
		{"chats", "background_url", "background_url TEXT NOT NULL DEFAULT ''"},
		{"chats", "banner_opacity", "banner_opacity REAL NOT NULL DEFAULT 0.9"},
		{"chat_members", "notify_enabled", "notify_enabled INTEGER NOT NULL DEFAULT 1"},
		{"messages", "type", "type TEXT NOT NULL DEFAULT ''"},
		{"users", "notify_blocked", "notify_blocked TEXT NOT NULL DEFAULT '[]'"},
	}
	for _, c := range columns {
		if err := d.ensureColumn(ctx, c.table, c.name, c.definition); err != nil {
			return err
		}
	}
	return nil
}

func Open(path string, maxContentLength int) (*DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_time_format=sqlite", path)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(10)
	if err := conn.PingContext(context.Background()); err != nil {
		conn.Close()
		return nil, err
	}
	if maxContentLength <= 0 {
		maxContentLength = 4000
	}
	d := &DB{DB: conn, maxContentLength: maxContentLength}
	if err := d.Migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	logutil.Info("database opened: %s (maxMsgLen=%d)", path, maxContentLength)
	return d, nil
}

func (d *DB) Migrate() error {
	ctx := context.Background()
	if _, err := d.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// Read current version (latest applied SQL migration)
	var current int
	if err := d.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), -1) FROM schema_migrations`,
	).Scan(&current); err != nil {
		return fmt.Errorf("read version: %w", err)
	}

	// SQL migrations: find NNN_*.sql, apply, record, repeat
	for {
		next := current + 1
		pattern := fmt.Sprintf("migrations/%03d_*.sql", next)
		matches, err := fs.Glob(migrationFS, pattern)
		if err != nil || len(matches) == 0 {
			break
		}
		sort.Strings(matches)
		file := matches[0]

		data, err := migrationFS.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read %s: %w", file, err)
		}
		if _, err := d.ExecContext(ctx, string(data)); err != nil {
			return fmt.Errorf("apply %s (v%d): %w", file, next, err)
		}
		if _, err := d.ExecContext(ctx,
			`INSERT INTO schema_migrations (version) VALUES (?)`, next); err != nil {
			return fmt.Errorf("record v%d: %w", next, err)
		}
		logutil.Info("applied migration: %s (v%d)", file, next)
		current = next
	}

	// Go migrations (version 1000+)
	for _, gm := range goMigrations {
		var exists int
		if err := d.QueryRowContext(ctx,
			`SELECT 1 FROM schema_migrations WHERE version = ?`, 1000+gm.version).Scan(&exists); err != nil {
			exists = 0
		}
		if exists == 1 {
			continue
		}
		if err := gm.fn(ctx, d); err != nil {
			return fmt.Errorf("go migration %d: %w", gm.version, err)
		}
		if _, err := d.ExecContext(ctx,
			`INSERT INTO schema_migrations (version) VALUES (?)`, 1000+gm.version); err != nil {
			return fmt.Errorf("record go migration %d: %w", gm.version, err)
		}
		logutil.Info("applied go migration: %d", gm.version)
	}

	return nil
}
