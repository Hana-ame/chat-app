package db

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
)

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
		return nil
	}
	if oldCnt == 0 {
		return fmt.Errorf("chat_members has neither last_visited_at nor last_active_at")
	}
	logutil.Info("migrating chat_members.last_visited_at \u2192 last_active_at")
	if _, err := d.ExecContext(ctx,
		`ALTER TABLE chat_members RENAME COLUMN last_visited_at TO last_active_at`); err != nil {
		return fmt.Errorf("rename last_visited_at: %w", err)
	}
	logutil.Info("chat_members.last_visited_at \u2192 last_active_at done")
	return nil
}

func (d *DB) ensureColumn(ctx context.Context, table, name, definition string) error {
	// WARNING: table and definition must be hardcoded constants (not user input)
	// to prevent SQL injection — ALTER TABLE cannot use parameterized placeholders.
	for _, r := range table {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return fmt.Errorf("invalid table name %q: must be a simple identifier", table)
		}
	}
	if strings.ContainsAny(definition, ";") || strings.Contains(definition, "--") || strings.Contains(definition, "/*") {
		return fmt.Errorf("invalid column definition %q: potential injection", definition)
	}
	var cnt int
	if err := d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, table, name,
	).Scan(&cnt); err != nil {
		return fmt.Errorf("check %s.%s: %w", table, name, err)
	}
	if cnt > 0 {
		return nil
	}
	logutil.Info("migrating %s: add %s column", table, name)
	q := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", table, definition)
	if _, err := d.ExecContext(ctx, q); err != nil {
		return fmt.Errorf("add %s.%s: %w", table, name, err)
	}
	logutil.Info("%s.%s column added", table, name)
	return nil
}

func migrateV2DropChatTypeCheck(ctx context.Context, d *DB) error {
	rows, err := d.QueryContext(ctx, `SELECT name FROM pragma_table_info('chats') ORDER BY cid`)
	if err != nil {
		return fmt.Errorf("read chats columns: %w", err)
	}
	defer rows.Close()

	var oldCols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		oldCols = append(oldCols, name)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	colList := strings.Join(oldCols, ", ")

	defs := []string{
		"id TEXT PRIMARY KEY",
		"type TEXT NOT NULL",
		"name TEXT",
		"icon_color TEXT NOT NULL DEFAULT '#5865F2'",
		"owner_id TEXT REFERENCES users(id) ON DELETE SET NULL",
		"created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))",
		"last_message_at TEXT",
		"last_message_id TEXT",
		"member_count INTEGER NOT NULL DEFAULT 0",
		"visibility TEXT NOT NULL DEFAULT 'private'",
		"pinned_message TEXT NOT NULL DEFAULT ''",
		"pinned_updated_at TEXT",
		"avatar_url TEXT NOT NULL DEFAULT ''",
		"banner_url TEXT NOT NULL DEFAULT ''",
		"background_url TEXT NOT NULL DEFAULT ''",
		"banner_opacity REAL NOT NULL DEFAULT 0.9",
	}

	schema := strings.Join(defs, ", ")

	createSQL := fmt.Sprintf(`CREATE TABLE chats_new (%s)`, schema)
	if _, err := d.ExecContext(ctx, `PRAGMA foreign_keys=OFF`); err != nil {
		return err
	}
	if _, err := d.ExecContext(ctx, createSQL); err != nil {
		return fmt.Errorf("create chats_new: %w", err)
	}
	if _, err := d.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO chats_new (%s) SELECT %s FROM chats`, colList, colList),
	); err != nil {
		return fmt.Errorf("copy chats data: %w", err)
	}
	if _, err := d.ExecContext(ctx, `DROP TABLE chats`); err != nil {
		return fmt.Errorf("drop chats: %w", err)
	}
	if _, err := d.ExecContext(ctx, `ALTER TABLE chats_new RENAME TO chats`); err != nil {
		return fmt.Errorf("rename chats_new: %w", err)
	}
	if _, err := d.ExecContext(ctx, `PRAGMA foreign_keys=ON`); err != nil {
		return err
	}
	logutil.Info("migrated chats table: removed type CHECK constraint, added notify support")
	return nil
}
