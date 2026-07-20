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
