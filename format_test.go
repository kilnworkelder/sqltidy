package sqltidy

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func sampleResult() *ResultSet {
	return &ResultSet{
		Columns: []string{"id", "name", "note"},
		Rows: [][]any{
			{int64(1), "Ada", "a|b\nc"},
			{int64(2), nil, ""},
		},
	}
}

func TestWriteResultCSV(t *testing.T) {
	var b bytes.Buffer
	if err := WriteResult(&b, sampleResult(), FormatCSV); err != nil {
		t.Fatal(err)
	}
	want := "id,name,note\n1,Ada,\"a|b\nc\"\n2,,\n"
	if b.String() != want {
		t.Fatalf("csv =\n%q\nwant\n%q", b.String(), want)
	}
}

func TestWriteResultJSON(t *testing.T) {
	var b bytes.Buffer
	if err := WriteResult(&b, sampleResult(), FormatJSON); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{`"id": 1`, `"name": "Ada"`, `"note": "a|b\nc"`, `"name": null`} {
		if !strings.Contains(out, want) {
			t.Errorf("json missing %q in:\n%s", want, out)
		}
	}
}

func TestWriteResultJSONL(t *testing.T) {
	var b bytes.Buffer
	if err := WriteResult(&b, sampleResult(), FormatJSONL); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(b.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2:\n%s", len(lines), b.String())
	}
	if !strings.HasPrefix(lines[0], `{"id": 1`) {
		t.Errorf("line 1 = %q", lines[0])
	}
}

func TestWriteResultMarkdown(t *testing.T) {
	var b bytes.Buffer
	if err := WriteResult(&b, sampleResult(), FormatMarkdown); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, `| 1 | Ada | a\|b<br>c |`) {
		t.Errorf("markdown =\n%s", out)
	}
}

func TestWriteResultTable(t *testing.T) {
	var b bytes.Buffer
	if err := WriteResult(&b, sampleResult(), FormatTable); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"id", "name", "note", "---", "Ada"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
}

func TestParseFormat(t *testing.T) {
	for in, want := range map[string]Format{
		"":         FormatTable,
		"table":    FormatTable,
		"CSV":      FormatCSV,
		"json":     FormatJSON,
		"ndjson":   FormatJSONL,
		"md":       FormatMarkdown,
		"markdown": FormatMarkdown,
	} {
		got, err := ParseFormat(in)
		if err != nil || got != want {
			t.Errorf("ParseFormat(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := ParseFormat("xml"); err == nil {
		t.Error("expected an error for xml")
	}
}

func TestExportCSV(t *testing.T) {
	db := newTestDB(t)
	importText(t, db, "id,name\n1,Ada\n", "csv", "people")
	var b bytes.Buffer
	if err := db.Export(context.Background(), "people", &b, FormatCSV); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(b.String(), "id,name\n1,Ada\n") {
		t.Fatalf("export = %q", b.String())
	}
}
