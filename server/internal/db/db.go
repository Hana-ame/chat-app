package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"time"

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

// goMigrations 只保留"一次性结构变更"(不能幂等重跑的操作)。
// 列补齐不再走版本记录:requiredColumns 是无条件幂等操作(ensureColumn
// 检查列存在),每次启动都执行一遍,杜绝"旧库把迁移记录为已应用后新列
// 永远不补"的整类故障(历史教训:v3/v4 因 requiredColumns 扩充而反复
// 被迫新增迁移版本)。往 requiredColumns 加列不再需要新版本。
var goMigrations = []goMigration{
	{2, migrateV2DropChatTypeCheck},
	{3, migrateV3NotifyUniqueIndex},
}

// requiredColumns 是所有历史版本缺失过、需要幂等确保存在的列集合。
// 注意:往这里加新列不会自动对"已应用 v1"的库生效,必须同时新增一个
// goMigration(见 v3 注释)。
var requiredColumns = []struct {
	table, name, definition string
}{
	{"chats", "avatar_url", "avatar_url TEXT NOT NULL DEFAULT ''"},
	{"chats", "banner_url", "banner_url TEXT NOT NULL DEFAULT ''"},
	{"chats", "background_url", "background_url TEXT NOT NULL DEFAULT ''"},
	{"chats", "banner_opacity", "banner_opacity REAL NOT NULL DEFAULT 0.9"},
	{"chats", "last_message_user_id", "last_message_user_id TEXT NOT NULL DEFAULT ''"},
	{"chats", "last_message_content", "last_message_content TEXT NOT NULL DEFAULT ''"},
	{"chats", "last_message_created_at", "last_message_created_at TEXT NOT NULL DEFAULT ''"},
	{"chat_members", "notify_enabled", "notify_enabled INTEGER NOT NULL DEFAULT 1"},
	{"chat_members", "unread_count", "unread_count INTEGER NOT NULL DEFAULT 0"},
	{"messages", "type", "type TEXT NOT NULL DEFAULT ''"},
	{"users", "notify_blocked", "notify_blocked TEXT NOT NULL DEFAULT '[]'"},
}

func ensureRequiredColumns(ctx context.Context, d *DB) error {
	for _, c := range requiredColumns {
		if err := d.ensureColumn(ctx, c.table, c.name, c.definition); err != nil {
			return err
		}
	}
	return nil
}

// ensureLastActiveColumn + ensureRequiredColumns 是幂等的列补齐,每次
// 启动无条件执行(不依赖迁移版本记录),任何旧库都能自愈。
func (d *DB) ensureSchemaColumns(ctx context.Context) error {
	// 改名逻辑(last_visited_at → last_active_at)同样幂等:目标列已存在即跳过。
	if err := d.ensureLastActiveColumn(ctx); err != nil {
		return err
	}
	return ensureRequiredColumns(ctx, d)
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

	// Go migrations (version 1000+) — 一次性结构变更
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

	// 幂等列补齐:无条件执行,不依赖版本记录(见 goMigrations 注释)。
	if err := d.ensureSchemaColumns(ctx); err != nil {
		return fmt.Errorf("ensure schema columns: %w", err)
	}

	// 【本地改动 2026-09-03】后台回填 FTS5 索引：对老消息（创建时未同步）做一次性索引。
	// 后台 goroutine，避免阻塞启动；若库大则可能耗时，但不会阻止服务就绪。
	go func() {
		bCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		indexed, skipped, err := d.BackfillFTS(bCtx)
		if err != nil {
			logutil.Error("BackfillFTS failed: %v", err)
			return
		}
		if indexed > 0 || skipped > 0 {
			logutil.Info("BackfillFTS done: indexed=%d skipped=%d", indexed, skipped)
		}
	}()

	return nil
}
