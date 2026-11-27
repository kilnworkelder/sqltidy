package sqltidy

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// rowSource is the common shape of every supported input format.
type rowSource interface {
	Columns() []string
	Next() ([]any, error)
}

func openSource(r io.Reader, opts ImportOptions) (rowSource, error) {
	switch strings.ToLower(strings.TrimSpace(opts.Format)) {
	case "csv", "tsv":
		comma := opts.Comma
		if comma == 0 {
			if strings.EqualFold(opts.Format, "tsv") {
				comma = '\t'
			} else {
				comma = ','
			}
		}
		return newCSVSource(r, comma, !opts.NoHeader)
	case "jsonl", "ndjson":
		return newJSONLSource(r)
	case "json":
		return newJSONSource(r)
	case "":
		return nil, fmt.Errorf("input format is required (csv, tsv, json, or jsonl)")
	default:
		return nil, fmt.Errorf("unsupported format %q (want csv, tsv, json, or jsonl)", opts.Format)
	}
}

// csvSource streams records and never rewinds the underlying reader.
type csvSource struct {
	r       *csv.Reader
	header  []string
	pending [][]string
}

func newCSVSource(r io.Reader, comma rune, hasHeader bool) (*csvSource, error) {
	cr := csv.NewReader(r)
	cr.Comma = comma
	cr.FieldsPerRecord = -1 // tolerate ragged rows
	cr.LazyQuotes = true
	s := &csvSource{r: cr}
	if hasHeader {
		h, err := cr.Read()
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("input is empty: expected a header row")
			}
			return nil, fmt.Errorf("read header: %w", err)
		}
		if len(h) > 0 {
			h[0] = strings.TrimPrefix(h[0], "\ufeff")
		}
		s.header = normalizeColumns(h)
		return s, nil
	}
	first, err := cr.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("input is empty")
		}
		return nil, fmt.Errorf("read first row: %w", err)
	}
	s.header = make([]string, len(first))
	for i := range first {
		s.header[i] = fmt.Sprintf("column_%d", i+1)
	}
	s.pending = append(s.pending, first)
	return s, nil
}

func (s *csvSource) Columns() []string { return s.header }

func (s *csvSource) Next() ([]any, error) {
	var rec []string
	if len(s.pending) > 0 {
		rec = s.pending[0]
		s.pending = s.pending[1:]
	} else {
		r, err := s.r.Read()
		if err != nil {
			return nil, err // io.EOF is meaningful to the caller
		}
		rec = r
	}
	row := make([]any, len(s.header))
	for i := range row {
		if i < len(rec) {
			row[i] = rec[i]
		}
	}
	return row, nil
}

// normalizeColumns trims names, fills blanks, removes a UTF-8 BOM from the
// first column, and de-duplicates collisions.
func normalizeColumns(header []string) []string {
	out := make([]string, len(header))
	used := make(map[string]bool, len(header))
	for i, h := range header {
		h = strings.TrimSpace(h)
		if h == "" {
			h = fmt.Sprintf("column_%d", i+1)
		}
		base := h
		for n := 2; used[h]; n++ {
			h = fmt.Sprintf("%s_%d", base, n)
		}
		used[h] = true
		out[i] = h
	}
	return out
}

// jsonlSource reads newline-delimited JSON objects. The first
// inferenceSample rows are buffered to discover the column union.
type jsonlSource struct {
	sc      *bufio.Scanner
	cols    []string
	known   map[string]bool
	pending []map[string]any
	line    int
}

func newJSONLSource(r io.Reader) (*jsonlSource, error) {
	s := &jsonlSource{sc: bufio.NewScanner(r)}
	s.sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	var sample []map[string]any
	for len(sample) < inferenceSample {
		m, err := s.nextLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		sample = append(sample, m)
	}
	if len(sample) == 0 {
		return nil, fmt.Errorf("input is empty: expected JSON objects")
	}
	s.cols = unionColumns(sample)
	s.known = make(map[string]bool, len(s.cols))
	for _, c := range s.cols {
		s.known[c] = true
	}
	s.pending = sample
	return s, nil
}

func (s *jsonlSource) Columns() []string { return s.cols }

func (s *jsonlSource) Next() ([]any, error) {
	var m map[string]any
	if len(s.pending) > 0 {
		m = s.pending[0]
		s.pending = s.pending[1:]
	} else {
		var err error
		m, err = s.nextLine()
		if err != nil {
			return nil, err
		}
		for k := range m {
			if !s.known[k] {
				return nil, fmt.Errorf("line %d: unexpected field %q (JSONL columns are fixed after the first %d rows)",
					s.line, k, inferenceSample)
			}
		}
	}
	row := make([]any, len(s.cols))
	for i, c := range s.cols {
		if v, ok := m[c]; ok {
			row[i] = v
		}
	}
	return row, nil
}

func (s *jsonlSource) nextLine() (map[string]any, error) {
	for s.sc.Scan() {
		s.line++
		line := strings.TrimSpace(s.sc.Text())
		if line == "" {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(line))
		dec.UseNumber()
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			return nil, fmt.Errorf("line %d: %w", s.line, err)
		}
		if m == nil {
			return nil, fmt.Errorf("line %d: expected a JSON object", s.line)
		}
		return m, nil
	}
	if err := s.sc.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

// jsonSource decodes a JSON array (or a single object) eagerly.
type jsonSource struct {
	cols []string
	rows []map[string]any
	i    int
}

func newJSONSource(r io.Reader) (*jsonSource, error) {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	var raw any
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}
	var rows []map[string]any
	switch v := raw.(type) {
	case []any:
		for i, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("item %d is not a JSON object", i+1)
			}
			rows = append(rows, m)
		}
		if len(rows) == 0 {
			return nil, fmt.Errorf("JSON array is empty")
		}
	case map[string]any:
		rows = []map[string]any{v}
	default:
		return nil, fmt.Errorf("JSON input must be an array of objects or an object")
	}
	return &jsonSource{cols: unionColumns(rows), rows: rows}, nil
}

func (s *jsonSource) Columns() []string { return s.cols }

func (s *jsonSource) Next() ([]any, error) {
	if s.i >= len(s.rows) {
		return nil, io.EOF
	}
	m := s.rows[s.i]
	s.i++
	row := make([]any, len(s.cols))
	for i, c := range s.cols {
		if v, ok := m[c]; ok {
			row[i] = v
		}
	}
	return row, nil
}

// unionColumns returns a deterministic column list: keys of the first row in
// sorted order, followed by newly seen keys in sorted order.
func unionColumns(rows []map[string]any) []string {
	seen := make(map[string]bool)
	var cols []string
	for _, m := range rows {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if !seen[k] {
				seen[k] = true
				cols = append(cols, k)
			}
		}
	}
	return cols
}
