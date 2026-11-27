// Package sqltidy is a small, embeddable toolkit for importing, transforming,
// querying, and exporting SQLite databases.
//
// It is the library behind the sqltidy command. Everything the CLI does is
// available through the DB type, so the same behaviour can be embedded in
// other Go programs.
package sqltidy

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// DB wraps an open SQLite database.
type DB struct {
	sql *sql.DB
}

// Open opens path, creating the database if it does not exist. An empty path
// or ":memory:" opens a private in-memory database.
func Open(path string) (*DB, error) {
	if path == "" {
		path = ":memory:"
	}
	dsn := path
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		dsn = path + sep + "_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	}
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	// SQLite serialises writers anyway; one connection avoids surprising
	// "database is locked" errors while the CLI runs sequential commands.
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return &DB{sql: sqlDB}, nil
}

// Close closes the underlying database.
func (db *DB) Close() error { return db.sql.Close() }

// SQL exposes the underlying handle for advanced use.
func (db *DB) SQL() *sql.DB { return db.sql }

// TableExists reports whether table (or view) exists.
func (db *DB) TableExists(ctx context.Context, table string) (bool, error) {
	var n int
	const q = `SELECT count(*) FROM sqlite_master WHERE type IN ('table','view') AND name = ?`
	if err := db.sql.QueryRowContext(ctx, q, table).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
