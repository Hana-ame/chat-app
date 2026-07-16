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
}

type migration struct {
	version int
	name    string
}

func Open(path string) (*DB, error) {
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
	d := &DB{DB: conn}
	if err := d.Migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	logutil.Info("database opened: %s", path)
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

	// Unversioned startup fix: rename last_visited_at → last_active_at if needed.
	// Runs every startup regardless of schema_migrations, because ensure_db()
	// on the VPS may have bootstrapped the DB with last_visited_at and pre-recorded
	// version 1, causing the versioned migration 001 to be skipped.
	if err := d.ensureLastActiveColumn(ctx); err != nil {
		return err
	}

	if err := d.ensureChatAvatarColumn(ctx); err != nil {
		return err
	}

	if err := d.ensureChatBannerBackgroundColumn(ctx); err != nil {
		return err
	}

	if err := d.ensureBannerOpacityColumn(ctx); err != nil {
		return err
	}

	return nil
}

func (d *DB) ensureLastActiveColumn(ctx context.Context) error {
	var oldCnt, newCnt int
	if err := d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info('chat_members') WHERE name='last_visited_at'`).Scan(&oldCnt); err != nil {
		return fmt.Errorf("check last_visited_at: %w", err)
	}
	if err := d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info('chat_members') WHERE name='last_active_at'`).Scan(&newCnt); err != nil {
		return fmt.Errorf("check last_active_at: %w", err)
	}
	if newCnt > 0 {
		return nil // already correct
	}
	if oldCnt == 0 {
		return fmt.Errorf("chat_members has neither last_visited_at nor last_active_at")
	}
	logutil.Info("migrating chat_members.last_visited_at → last_active_at")
	if _, err := d.ExecContext(ctx,
		`ALTER TABLE chat_members RENAME COLUMN last_visited_at TO last_active_at`); err != nil {
		return fmt.Errorf("rename last_visited_at: %w", err)
	}
	logutil.Info("chat_members.last_visited_at → last_active_at done")
	return nil
}

func (d *DB) ensureChatAvatarColumn(ctx context.Context) error {
	var cnt int
	if err := d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info('chats') WHERE name='avatar_url'`).Scan(&cnt); err != nil {
		return fmt.Errorf("check avatar_url: %w", err)
	}
	if cnt > 0 {
		return nil
	}
	logutil.Info("migrating chats: add avatar_url column")
	if _, err := d.ExecContext(ctx,
		`ALTER TABLE chats ADD COLUMN avatar_url TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("add avatar_url: %w", err)
	}
	logutil.Info("chats.avatar_url column added")
	return nil
}

func (d *DB) ensureChatBannerBackgroundColumn(ctx context.Context) error {
	var hasBanner, hasBg int
	if err := d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info('chats') WHERE name='banner_url'`).Scan(&hasBanner); err != nil {
		return fmt.Errorf("check banner_url: %w", err)
	}
	if err := d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info('chats') WHERE name='background_url'`).Scan(&hasBg); err != nil {
		return fmt.Errorf("check background_url: %w", err)
	}
	if hasBanner > 0 && hasBg > 0 {
		return nil
	}
	logutil.Info("migrating chats: add banner_url, background_url columns")
	if hasBanner == 0 {
		if _, err := d.ExecContext(ctx,
			`ALTER TABLE chats ADD COLUMN banner_url TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add banner_url: %w", err)
		}
	}
	if hasBg == 0 {
		if _, err := d.ExecContext(ctx,
			`ALTER TABLE chats ADD COLUMN background_url TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add background_url: %w", err)
		}
	}
	logutil.Info("chats.banner_url, background_url columns added")
	return nil
}

func (d *DB) ensureBannerOpacityColumn(ctx context.Context) error {
	var cnt int
	if err := d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info('chats') WHERE name='banner_opacity'`).Scan(&cnt); err != nil {
		return fmt.Errorf("check banner_opacity: %w", err)
	}
	if cnt > 0 {
		return nil
	}
	logutil.Info("migrating chats: add banner_opacity column")
	if _, err := d.ExecContext(ctx,
		`ALTER TABLE chats ADD COLUMN banner_opacity REAL NOT NULL DEFAULT 0.9`); err != nil {
		return fmt.Errorf("add banner_opacity: %w", err)
	}
	logutil.Info("chats.banner_opacity column added")
	return nil
}

