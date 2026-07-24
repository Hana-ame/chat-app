package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type DB struct {
	*sql.DB
	maxContentLength int
}

type migration struct {
	version int
	name    string
}

type goMigration struct {
	version int
	fn      func(ctx context.Context, d *DB) error
}

// goMigrations are versioned schema fixups that need Go logic (e.g. ALTER
// TABLE ADD COLUMN with existence checks that SQLite can't express portably).
// New entries must append with an incremented version number.
var goMigrations = []goMigration{
	{1, migrateV1EnsureColumns},
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
	conn.SetMaxOpenConns(1)
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

	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return err
	}

	var migrations []migration
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		v, err := strconv.Atoi(e.Name()[:3])
		if err != nil {
			continue
		}
		migrations = append(migrations, migration{v, e.Name()})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	for _, m := range migrations {
		var exists int
		d.QueryRowContext(ctx,
			`SELECT 1 FROM schema_migrations WHERE version = ?`, m.version).Scan(&exists)
		if exists == 1 {
			continue
		}

		b, err := migrationFS.ReadFile("migrations/" + m.name)
		if err != nil {
			return fmt.Errorf("read %s: %w", m.name, err)
		}
		if _, err := d.ExecContext(ctx, string(b)); err != nil {
			return fmt.Errorf("apply %s: %w", m.name, err)
		}
		if _, err := d.ExecContext(ctx,
			`INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
			return fmt.Errorf("record %s: %w", m.name, err)
		}
		logutil.Info("applied migration: %s", m.name)
	}

	// ── Go-based migrations (column ensure, schema fixups) ──
	for _, gm := range goMigrations {
		var exists int
		d.QueryRowContext(ctx,
			`SELECT 1 FROM schema_migrations WHERE version = ?`, gm.version).Scan(&exists)
		if exists == 1 {
			continue
		}
		if err := gm.fn(ctx, d); err != nil {
			return fmt.Errorf("apply migration %d: %w", gm.version, err)
		}
		if _, err := d.ExecContext(ctx,
			`INSERT INTO schema_migrations (version) VALUES (?)`, gm.version); err != nil {
			return fmt.Errorf("record migration %d: %w", gm.version, err)
		}
		logutil.Info("applied go migration: %d", gm.version)
	}

	return nil
}

