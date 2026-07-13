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
	if _, err := d.ExecContext(context.Background(),
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

	type migration struct {
		version int
		name    string
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
		d.QueryRowContext(context.Background(),
			`SELECT 1 FROM schema_migrations WHERE version = ?`, m.version).Scan(&exists)
		if exists == 1 {
			continue
		}

		b, err := migrationFS.ReadFile("migrations/" + m.name)
		if err != nil {
			return fmt.Errorf("read %s: %w", m.name, err)
		}
		if _, err := d.ExecContext(context.Background(), string(b)); err != nil {
			return fmt.Errorf("apply %s: %w", m.name, err)
		}
		if _, err := d.ExecContext(context.Background(),
			`INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
			return fmt.Errorf("record %s: %w", m.name, err)
		}
		logutil.Info("applied migration: %s", m.name)
	}
	return nil
}

