package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	_ "modernc.org/sqlite"
)

func isDupColumnErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate column name")
}

//go:embed migrations/*.sql
var migrationFS embed.FS

var versionRe = regexp.MustCompile(`^V(\d{3})__`)

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
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, n := range names {
		m := versionRe.FindStringSubmatch(n)
		if m == nil {
			// non-versioned (init.sql) — run idempotently
			b, err := migrationFS.ReadFile("migrations/" + n)
			if err != nil {
				return fmt.Errorf("read %s: %w", n, err)
			}
			if _, err := d.ExecContext(context.Background(), string(b)); err != nil {
				if !isDupColumnErr(err) {
					return fmt.Errorf("apply %s: %w", n, err)
				}
			}
			logutil.Debug("applied base migration: %s", n)
			continue
		}

		version := m[1]
		var exists int
		d.QueryRowContext(context.Background(),
			`SELECT 1 FROM schema_migrations WHERE version = ?`, version).Scan(&exists)
		if exists == 1 {
			continue
		}

		b, err := migrationFS.ReadFile("migrations/" + n)
		if err != nil {
			return fmt.Errorf("read %s: %w", n, err)
		}
		if _, err := d.ExecContext(context.Background(), string(b)); err != nil {
			return fmt.Errorf("apply %s: %w", n, err)
		}
		if _, err := d.ExecContext(context.Background(),
			`INSERT INTO schema_migrations (version) VALUES (?)`, version); err != nil {
			return fmt.Errorf("record %s: %w", n, err)
		}
		logutil.Info("applied migration: %s", n)
	}
	return nil
}
