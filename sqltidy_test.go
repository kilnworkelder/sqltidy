package sqltidy

import (
	"context"
	"path/filepath"
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func importText(t *testing.T, db *DB, data, format, table string) ImportResult {
	t.Helper()
	res, err := db.Import(context.Background(), newReader(data), ImportOptions{Table: table, Format: format})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	return res
}
