package db

import (
	"context"
	"fmt"

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
