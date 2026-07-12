package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
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
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, n := range names {
		b, err := migrationFS.ReadFile("migrations/" + n)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", n, err)
		}
		if _, err := d.ExecContext(context.Background(), string(b)); err != nil {
			if !isDupColumnErr(err) {
				return fmt.Errorf("apply migration %s: %w", n, err)
			}
		}
		logutil.Debug("applied migration: %s", n)
	}
	return nil
}
