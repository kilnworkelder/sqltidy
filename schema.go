package sqltidy

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// ColumnInfo describes one table column.
type ColumnInfo struct {
	Name    string
	Type    string
	NotNull bool
	Default *string
	PK      bool
}

// TableInfo describes a table and its indexes.
type TableInfo struct {
	Name    string
	Columns []ColumnInfo
	Indexes []string
	Rows    int64
}

// Tables lists user tables, excluding SQLite internals and FTS shadow
// tables.
func (db *DB) Tables(ctx context.Context) ([]string, error) {
	rows, err := db.sql.QueryContext(ctx, "PRAGMA table_list")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var names []string
	for rows.Next() {
		var schemaName, name, typ string
		var ncol, wr, strict int
		if err := rows.Scan(&schemaName, &name, &typ, &ncol, &wr, &strict); err != nil {
			return nil, err
		}
		if typ != "table" && typ != "virtual" {
			continue
		}
		if strings.HasPrefix(name, "sqlite_") {
			continue
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

// TableInfo returns the schema and row count for table.
func (db *DB) TableInfo(ctx context.Context, table string) (*TableInfo, error) {
	exists, err := db.TableExists(ctx, table)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("table %s does not exist", table)
	}
	info := &TableInfo{Name: table}

	rows, err := db.sql.QueryContext(ctx, "PRAGMA table_info("+quoteIdent(table)+")")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			_ = rows.Close()
			return nil, err
		}
		col := ColumnInfo{Name: name, Type: ctype, NotNull: notnull != 0, PK: pk > 0}
		if dflt.Valid {
			v := dflt.String
			col.Default = &v
		}
		info.Columns = append(info.Columns, col)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()

	idx, err := db.sql.QueryContext(ctx, "PRAGMA index_list("+quoteIdent(table)+")")
	if err != nil {
		return nil, err
	}
	for idx.Next() {
		var (
			seq     int
			name    string
			unique  int
			origin  string
			partial int
		)
		if err := idx.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			_ = idx.Close()
			return nil, err
		}
		info.Indexes = append(info.Indexes, name)
	}
	if err := idx.Err(); err != nil {
		_ = idx.Close()
		return nil, err
	}
	_ = idx.Close()

	if err := db.sql.QueryRowContext(ctx, "SELECT count(*) FROM "+quoteIdent(table)).Scan(&info.Rows); err != nil {
		return nil, err
	}
	return info, nil
}

// Schema returns the schema of every user table.
func (db *DB) Schema(ctx context.Context) ([]TableInfo, error) {
	names, err := db.Tables(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]TableInfo, 0, len(names))
	for _, name := range names {
		info, err := db.TableInfo(ctx, name)
		if err != nil {
			return nil, err
		}
		out = append(out, *info)
	}
	return out, nil
}

func (db *DB) columnNames(ctx context.Context, table string) ([]string, error) {
	rows, err := db.sql.QueryContext(ctx, "PRAGMA table_info("+quoteIdent(table)+")")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var names []string
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return names, nil
}
