package sqltidy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// inferenceSample is how many leading rows are buffered to infer column
// types and JSON keys. CSV data after this window is streamed.
const inferenceSample = 1000

// ImportOptions controls how a source is loaded into a table.
type ImportOptions struct {
	// Table is the destination table. Required.
	Table string
	// Format is one of "csv", "tsv", "json", or "jsonl".
	Format string
	// PK lists primary-key columns. Used when creating a new table.
	PK []string
	// Alter adds columns that are missing from an existing table.
	Alter bool
	// NoHeader treats the first CSV/TSV row as data.
	NoHeader bool
	// Comma overrides the CSV delimiter. Zero means "," (or tab for TSV).
	Comma rune
}

// ImportResult describes a completed import.
type ImportResult struct {
	Table   string
	Rows    int
	Columns []string
}

type importPlan struct {
	src    rowSource
	cols   []string
	types  []columnType
	sample [][]any
}

func newImportPlan(src rowSource) (*importPlan, error) {
	p := &importPlan{src: src, cols: src.Columns()}
	if len(p.cols) == 0 {
		return nil, errors.New("input has no columns")
	}
	for len(p.sample) < inferenceSample {
		row, err := src.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		p.sample = append(p.sample, row)
	}
	p.types = inferColumnTypes(p.sample, len(p.cols))
	return p, nil
}

// forEach replays the sample and then streams the remainder of the source.
func (p *importPlan) forEach(ctx context.Context, fn func(row []any, n int) error) error {
	n := 0
	for _, row := range p.sample {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := fn(row, n); err != nil {
			return err
		}
		n++
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		row, err := p.src.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := fn(row, n); err != nil {
			return err
		}
		n++
	}
}

// Import streams CSV/TSV/JSON/JSONL into table, creating it when needed.
func (db *DB) Import(ctx context.Context, r io.Reader, opts ImportOptions) (ImportResult, error) {
	if strings.TrimSpace(opts.Table) == "" {
		return ImportResult{}, errors.New("table name is required")
	}
	src, err := openSource(r, opts)
	if err != nil {
		return ImportResult{}, err
	}
	p, err := newImportPlan(src)
	if err != nil {
		return ImportResult{}, err
	}
	if err := validatePK(opts.PK, p.cols); err != nil {
		return ImportResult{}, err
	}

	exists, err := db.TableExists(ctx, opts.Table)
	if err != nil {
		return ImportResult{}, err
	}
	if !exists {
		if _, err := db.sql.ExecContext(ctx, createTableSQL(opts.Table, p.cols, p.types, opts.PK)); err != nil {
			return ImportResult{}, fmt.Errorf("create table %s: %w", opts.Table, err)
		}
	} else {
		existing, err := db.columnSet(ctx, opts.Table)
		if err != nil {
			return ImportResult{}, err
		}
		for i, c := range p.cols {
			if existing[c] {
				continue
			}
			if !opts.Alter {
				return ImportResult{}, fmt.Errorf(
					"table %s is missing column %q (use --alter to add missing columns)",
					opts.Table, c)
			}
			if err := db.AddColumn(ctx, opts.Table, c, p.types[i].sql()); err != nil {
				return ImportResult{}, err
			}
		}
	}

	insertSQL := insertStatement(opts.Table, p.cols)
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return ImportResult{}, err
	}
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		_ = tx.Rollback()
		return ImportResult{}, err
	}
	count := 0
	err = p.forEach(ctx, func(row []any, n int) error {
		args, err := convertRow(row, p.types)
		if err != nil {
			return fmt.Errorf("row %d: %w", n+1, err)
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			return fmt.Errorf("row %d: %w", n+1, err)
		}
		count++
		return nil
	})
	_ = stmt.Close()
	if err != nil {
		_ = tx.Rollback()
		return ImportResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Table: opts.Table, Rows: count, Columns: p.cols}, nil
}

func validatePK(pk, cols []string) error {
	if len(pk) == 0 {
		return nil
	}
	have := make(map[string]bool, len(cols))
	for _, c := range cols {
		have[c] = true
	}
	for _, p := range pk {
		if !have[p] {
			return fmt.Errorf("primary key column %q is not present in the input", p)
		}
	}
	return nil
}

func createTableSQL(table string, cols []string, types []columnType, pk []string) string {
	defs := make([]string, 0, len(cols)+1)
	for i, c := range cols {
		defs = append(defs, quoteIdent(c)+" "+types[i].sql())
	}
	if len(pk) > 0 {
		quoted := make([]string, len(pk))
		for i, p := range pk {
			quoted[i] = quoteIdent(p)
		}
		defs = append(defs, "PRIMARY KEY ("+strings.Join(quoted, ", ")+")")
	}
	return "CREATE TABLE " + quoteIdent(table) + " (" + strings.Join(defs, ", ") + ")"
}

func insertStatement(table string, cols []string) string {
	quoted := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = quoteIdent(c)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",")
	return "INSERT INTO " + quoteIdent(table) + " (" + strings.Join(quoted, ", ") + ") VALUES (" + placeholders + ")"
}

func (db *DB) columnSet(ctx context.Context, table string) (map[string]bool, error) {
	names, err := db.columnNames(ctx, table)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set, nil
}

func convertRow(row []any, types []columnType) ([]any, error) {
	args := make([]any, len(row))
	for i := range row {
		t := typeText
		if i < len(types) {
			t = types[i]
		}
		v, err := convertValue(row[i], t)
		if err != nil {
			return nil, err
		}
		args[i] = v
	}
	return args, nil
}

func convertValue(v any, t columnType) (any, error) {
	if v == nil {
		return nil, nil
	}
	switch x := v.(type) {
	case string:
		switch t {
		case typeInteger:
			s := strings.TrimSpace(x)
			if s == "" {
				return nil, nil
			}
			if isBoolString(s) {
				return boolInt(s), nil
			}
			if isIntString(s) {
				return strconv.ParseInt(s, 10, 64)
			}
			if isFloatString(s) {
				return strconv.ParseFloat(s, 64)
			}
			return x, nil
		case typeReal:
			s := strings.TrimSpace(x)
			if s == "" {
				return nil, nil
			}
			if isIntString(s) {
				n, err := strconv.ParseInt(s, 10, 64)
				if err != nil {
					return nil, err
				}
				return float64(n), nil
			}
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return f, nil
			}
			return x, nil
		default:
			return x, nil
		}
	case bool:
		if t == typeReal {
			if x {
				return float64(1), nil
			}
			return float64(0), nil
		}
		if x {
			return int64(1), nil
		}
		return int64(0), nil
	case int:
		if t == typeReal {
			return float64(x), nil
		}
		return int64(x), nil
	case int8:
		return int64(x), nil
	case int16:
		return int64(x), nil
	case int32:
		return int64(x), nil
	case int64:
		if t == typeReal {
			return float64(x), nil
		}
		return x, nil
	case uint:
		if t == typeReal {
			return float64(x), nil
		}
		return int64(x), nil
	case uint64:
		if t == typeReal {
			return float64(x), nil
		}
		return int64(x), nil
	case float32:
		return float64(x), nil
	case float64:
		if t == typeInteger && x == math.Trunc(x) && !math.IsInf(x, 0) {
			return int64(x), nil
		}
		return x, nil
	case json.Number:
		if t == typeInteger {
			if n, err := x.Int64(); err == nil {
				return n, nil
			}
		}
		if f, err := x.Float64(); err == nil {
			return f, nil
		}
		return string(x), nil
	case []byte:
		return string(x), nil
	case map[string]any, []any:
		b, err := json.Marshal(x)
		if err != nil {
			return nil, err
		}
		return string(b), nil
	default:
		return fmt.Sprint(x), nil
	}
}

// Upsert inserts rows and updates existing rows that collide on the primary
// key. The destination table must already exist.
func (db *DB) Upsert(ctx context.Context, r io.Reader, opts ImportOptions) (ImportResult, error) {
	if strings.TrimSpace(opts.Table) == "" {
		return ImportResult{}, errors.New("table name is required")
	}
	if len(opts.PK) == 0 {
		return ImportResult{}, errors.New("upsert requires at least one --pk column")
	}
	src, err := openSource(r, opts)
	if err != nil {
		return ImportResult{}, err
	}
	p, err := newImportPlan(src)
	if err != nil {
		return ImportResult{}, err
	}
	if err := validatePK(opts.PK, p.cols); err != nil {
		return ImportResult{}, err
	}
	exists, err := db.TableExists(ctx, opts.Table)
	if err != nil {
		return ImportResult{}, err
	}
	if !exists {
		return ImportResult{}, fmt.Errorf("table %s does not exist; import it first", opts.Table)
	}
	existing, err := db.columnSet(ctx, opts.Table)
	if err != nil {
		return ImportResult{}, err
	}
	for _, c := range p.cols {
		if !existing[c] {
			return ImportResult{}, fmt.Errorf("table %s is missing column %q; import it first", opts.Table, c)
		}
	}

	upsertSQL := upsertStatement(opts.Table, p.cols, opts.PK)
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return ImportResult{}, err
	}
	stmt, err := tx.PrepareContext(ctx, upsertSQL)
	if err != nil {
		_ = tx.Rollback()
		return ImportResult{}, err
	}
	count := 0
	err = p.forEach(ctx, func(row []any, n int) error {
		args, err := convertRow(row, p.types)
		if err != nil {
			return fmt.Errorf("row %d: %w", n+1, err)
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			return fmt.Errorf("row %d: %w", n+1, err)
		}
		count++
		return nil
	})
	_ = stmt.Close()
	if err != nil {
		_ = tx.Rollback()
		return ImportResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Table: opts.Table, Rows: count, Columns: p.cols}, nil
}

func upsertStatement(table string, cols, pk []string) string {
	quoted := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = quoteIdent(c)
	}
	quotedPK := make([]string, len(pk))
	inPK := make(map[string]bool, len(pk))
	for i, c := range pk {
		quotedPK[i] = quoteIdent(c)
		inPK[c] = true
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",")
	var sets []string
	for _, c := range cols {
		if inPK[c] {
			continue
		}
		sets = append(sets, quoteIdent(c)+"=excluded."+quoteIdent(c))
	}
	sql := "INSERT INTO " + quoteIdent(table) + " (" + strings.Join(quoted, ", ") +
		") VALUES (" + placeholders + ") ON CONFLICT (" + strings.Join(quotedPK, ", ") + ") "
	if len(sets) == 0 {
		return sql + "DO NOTHING"
	}
	return sql + "DO UPDATE SET " + strings.Join(sets, ", ")
}
