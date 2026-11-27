package sqltidy

import (
	"context"
)

// ResultSet is a materialised query result.
type ResultSet struct {
	Columns []string
	Rows    [][]any
}

// Query runs query and returns all rows. It is intended for the CLI and for
// modest result sets; large exports should use Export, which still
// materialises but avoids extra copies.
func (db *DB) Query(ctx context.Context, query string, args ...any) (*ResultSet, error) {
	rows, err := db.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	rs := &ResultSet{Columns: cols}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		rs.Rows = append(rs.Rows, vals)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rs, nil
}
