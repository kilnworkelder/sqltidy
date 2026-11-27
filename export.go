package sqltidy

import (
	"context"
	"fmt"
	"io"
)

// Export writes every row of table to w in the requested format.
func (db *DB) Export(ctx context.Context, table string, w io.Writer, format Format) error {
	if format == "" {
		format = FormatCSV
	}
	exists, err := db.TableExists(ctx, table)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("table %s does not exist", table)
	}
	rs, err := db.Query(ctx, "SELECT * FROM "+quoteIdent(table))
	if err != nil {
		return err
	}
	return WriteResult(w, rs, format)
}
