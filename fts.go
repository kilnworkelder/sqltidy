package sqltidy

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// FTSName is the FTS5 companion table created for table.
func FTSName(table string) string { return table + "_fts" }

// EnableFTS creates an external-content FTS5 index over every column of
// table, installs keep-in-sync triggers, and rebuilds the index.
func (db *DB) EnableFTS(ctx context.Context, table string) error {
	exists, err := db.TableExists(ctx, table)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("table %s does not exist", table)
	}
	cols, err := db.columnNames(ctx, table)
	if err != nil {
		return err
	}
	if len(cols) == 0 {
		return fmt.Errorf("table %s has no columns", table)
	}

	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = quoteIdent(c)
	}
	newCols := make([]string, len(cols))
	oldCols := make([]string, len(cols))
	for i, c := range cols {
		newCols[i] = "new." + quoteIdent(c)
		oldCols[i] = "old." + quoteIdent(c)
	}
	fts := FTSName(table)

	create := "CREATE VIRTUAL TABLE IF NOT EXISTS " + quoteIdent(fts) +
		" USING fts5(" + strings.Join(quotedCols, ", ") +
		", content=" + quoteLiteral(table) + ", content_rowid='rowid')"
	if _, err := db.sql.ExecContext(ctx, create); err != nil {
		return fmt.Errorf("create FTS index: %w", err)
	}

	insertTrigger := "CREATE TRIGGER IF NOT EXISTS " + quoteIdent(fts+"_ai") +
		" AFTER INSERT ON " + quoteIdent(table) + " BEGIN " +
		"INSERT INTO " + quoteIdent(fts) + "(rowid, " + strings.Join(quotedCols, ", ") + ") VALUES (new.rowid, " +
		strings.Join(newCols, ", ") + "); END"
	deleteTrigger := "CREATE TRIGGER IF NOT EXISTS " + quoteIdent(fts+"_ad") +
		" AFTER DELETE ON " + quoteIdent(table) + " BEGIN " +
		"INSERT INTO " + quoteIdent(fts) + "(" + quoteIdent(fts) + ", rowid, " + strings.Join(quotedCols, ", ") +
		") VALUES ('delete', old.rowid, " + strings.Join(oldCols, ", ") + "); END"
	updateTrigger := "CREATE TRIGGER IF NOT EXISTS " + quoteIdent(fts+"_au") +
		" AFTER UPDATE ON " + quoteIdent(table) + " BEGIN " +
		"INSERT INTO " + quoteIdent(fts) + "(" + quoteIdent(fts) + ", rowid, " + strings.Join(quotedCols, ", ") +
		") VALUES ('delete', old.rowid, " + strings.Join(oldCols, ", ") + "); " +
		"INSERT INTO " + quoteIdent(fts) + "(rowid, " + strings.Join(quotedCols, ", ") + ") VALUES (new.rowid, " +
		strings.Join(newCols, ", ") + "); END"

	for _, trigger := range []string{insertTrigger, deleteTrigger, updateTrigger} {
		if _, err := db.sql.ExecContext(ctx, trigger); err != nil {
			return fmt.Errorf("create FTS trigger: %w", err)
		}
	}
	return db.RebuildFTS(ctx, table)
}

// RebuildFTS reindexes every row of table.
func (db *DB) RebuildFTS(ctx context.Context, table string) error {
	fts := FTSName(table)
	exists, err := db.TableExists(ctx, fts)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("FTS is not enabled for %s (run: sqltidy fts enable %s)", table, table)
	}
	_, err = db.sql.ExecContext(ctx,
		"INSERT INTO "+quoteIdent(fts)+"("+quoteIdent(fts)+") VALUES('rebuild')")
	return err
}

// SearchFTS runs an FTS5 query and returns matching base-table rows ordered
// by relevance. limit <= 0 means no limit.
func (db *DB) SearchFTS(ctx context.Context, table, query string, limit int) (*ResultSet, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("search query is required")
	}
	fts := FTSName(table)
	exists, err := db.TableExists(ctx, fts)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("FTS is not enabled for %s (run: sqltidy fts enable %s)", table, table)
	}
	q := "SELECT t.* FROM " + quoteIdent(table) + " AS t JOIN " + quoteIdent(fts) +
		" AS f ON f.rowid = t.rowid WHERE " + quoteIdent(fts) + " MATCH ? ORDER BY f.rank"
	args := []any{query}
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	return db.Query(ctx, q, args...)
}
