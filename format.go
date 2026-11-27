package sqltidy

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Format is an output format.
type Format string

const (
	FormatTable    Format = "table"
	FormatCSV      Format = "csv"
	FormatJSON     Format = "json"
	FormatJSONL    Format = "jsonl"
	FormatMarkdown Format = "markdown"
)

// ParseFormat parses a user-supplied format name.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "table":
		return FormatTable, nil
	case "csv":
		return FormatCSV, nil
	case "json":
		return FormatJSON, nil
	case "jsonl", "ndjson":
		return FormatJSONL, nil
	case "markdown", "md":
		return FormatMarkdown, nil
	default:
		return "", fmt.Errorf("unsupported output format %q (want table, csv, json, jsonl, or markdown)", s)
	}
}

// WriteResult writes rs to w in the requested format.
func WriteResult(w io.Writer, rs *ResultSet, format Format) error {
	switch format {
	case FormatTable:
		return writeTable(w, rs)
	case FormatCSV:
		return writeCSV(w, rs)
	case FormatJSON:
		return writeJSONArray(w, rs)
	case FormatJSONL:
		return writeJSONLines(w, rs)
	case FormatMarkdown:
		return writeMarkdown(w, rs)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func cellAt(row []any, i int) any {
	if i < len(row) {
		return row[i]
	}
	return nil
}

func cellString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(x)
	case time.Time:
		return x.Format(time.RFC3339Nano)
	case float32:
		return strconv.FormatFloat(float64(x), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case json.Number:
		return string(x)
	default:
		return fmt.Sprint(x)
	}
}

func jsonValue(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	case time.Time:
		return x.Format(time.RFC3339Nano)
	default:
		return v
	}
}

func writeCSV(w io.Writer, rs *ResultSet) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(rs.Columns); err != nil {
		return err
	}
	record := make([]string, len(rs.Columns))
	for _, row := range rs.Rows {
		for i := range record {
			record[i] = cellString(cellAt(row, i))
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func writeJSONArray(w io.Writer, rs *ResultSet) error {
	var b bytes.Buffer
	b.WriteString("[\n")
	for r, row := range rs.Rows {
		b.WriteString("  ")
		if err := writeJSONObject(&b, rs.Columns, row); err != nil {
			return err
		}
		if r < len(rs.Rows)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString("]\n")
	_, err := w.Write(b.Bytes())
	return err
}

func writeJSONLines(w io.Writer, rs *ResultSet) error {
	var b bytes.Buffer
	for _, row := range rs.Rows {
		if err := writeJSONObject(&b, rs.Columns, row); err != nil {
			return err
		}
		b.WriteByte('\n')
	}
	_, err := w.Write(b.Bytes())
	return err
}

func writeJSONObject(b *bytes.Buffer, cols []string, row []any) error {
	b.WriteByte('{')
	for i, c := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		key, err := marshalJSON(c)
		if err != nil {
			return err
		}
		b.Write(key)
		b.WriteString(": ")
		val, err := marshalJSON(jsonValue(cellAt(row, i)))
		if err != nil {
			return err
		}
		b.Write(val)
	}
	b.WriteByte('}')
	return nil
}

// marshalJSON encodes a value without escaping <, >, and &, which would
// otherwise turn ordinary data into \u003c sequences.
func marshalJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

func writeTable(w io.Writer, rs *ResultSet) error {
	cells := make([][]string, len(rs.Rows))
	widths := make([]int, len(rs.Columns))
	for i, c := range rs.Columns {
		widths[i] = utf8.RuneCountInString(c)
	}
	for r, row := range rs.Rows {
		cells[r] = make([]string, len(rs.Columns))
		for i := range rs.Columns {
			s := truncate(tableCell(cellAt(row, i)), 60)
			cells[r][i] = s
			if n := utf8.RuneCountInString(s); n > widths[i] {
				widths[i] = n
			}
		}
	}

	var b strings.Builder
	for i, c := range rs.Columns {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(padRight(c, widths[i]))
	}
	b.WriteByte('\n')
	for i := range rs.Columns {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(strings.Repeat("-", widths[i]))
	}
	b.WriteByte('\n')
	for _, row := range cells {
		for i, s := range row {
			if i > 0 {
				b.WriteString("  ")
			}
			b.WriteString(padRight(s, widths[i]))
		}
		b.WriteByte('\n')
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func writeMarkdown(w io.Writer, rs *ResultSet) error {
	var b strings.Builder
	b.WriteString("| ")
	for i, c := range rs.Columns {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(markdownCell(c))
	}
	b.WriteString(" |\n| ")
	for i := range rs.Columns {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString("---")
	}
	b.WriteString(" |\n")
	for _, row := range rs.Rows {
		b.WriteString("| ")
		for i := range rs.Columns {
			if i > 0 {
				b.WriteString(" | ")
			}
			b.WriteString(markdownCell(cellString(cellAt(row, i))))
		}
		b.WriteString(" |\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func tableCell(v any) string {
	s := cellString(v)
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return s
}

func markdownCell(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\n", "<br>")
	return s
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

func padRight(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}
