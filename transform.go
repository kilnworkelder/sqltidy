package sqltidy

import (
	"context"
	"fmt"
	"strings"
)

var allowedTypes = map[string]bool{
	"INTEGER":   true,
	"REAL":      true,
	"TEXT":      true,
	"BLOB":      true,
	"NUMERIC":   true,
	"BOOLEAN":   true,
	"DATE":      true,
	"DATETIME":  true,
	"TIMESTAMP": true,
}

func normalizeType(typ string) (string, error) {
	typ = strings.ToUpper(strings.TrimSpace(typ))
	if typ == "" {
		return "TEXT", nil
	}
	if !allowedTypes[typ] {
		return "", fmt.Errorf(
			"unsupported column type %q (use INTEGER, REAL, TEXT, BLOB, NUMERIC, BOOLEAN, DATE, DATETIME, or TIMESTAMP)",
			typ)
	}
	return typ, nil
}

// RenameColumn renames oldName to newName.
func (db *DB) RenameColumn(ctx context.Context, table, oldName, newName string) error {
	if strings.TrimSpace(oldName) == "" || strings.TrimSpace(newName) == "" {
		return fmt.Errorf("both old and new column names are required")
	}
	if oldName == newName {
		return nil
	}
	_, err := db.sql.ExecContext(ctx,
		"ALTER TABLE "+quoteIdent(table)+" RENAME COLUMN "+quoteIdent(oldName)+" TO "+quoteIdent(newName))
	return err
}

// AddColumn appends a column with the given SQLite type.
func (db *DB) AddColumn(ctx context.Context, table, name, typ string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("column name is required")
	}
	t, err := normalizeType(typ)
	if err != nil {
		return err
	}
	_, err = db.sql.ExecContext(ctx,
		"ALTER TABLE "+quoteIdent(table)+" ADD COLUMN "+quoteIdent(name)+" "+t)
	return err
}

// DropColumn removes a column. SQLite refuses if the column is part of a
// primary key or an index; the underlying error is returned unchanged.
func (db *DB) DropColumn(ctx context.Context, table, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("column name is required")
	}
	_, err := db.sql.ExecContext(ctx,
		"ALTER TABLE "+quoteIdent(table)+" DROP COLUMN "+quoteIdent(name))
	return err
}
