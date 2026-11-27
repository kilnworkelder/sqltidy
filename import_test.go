package sqltidy

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestImportCSVInfersTypes(t *testing.T) {
	db := newTestDB(t)
	data := "id,name,score,active,zip\n1,Ada,9.5,true,00123\n2,Grace,8,false,00456\n"
	res := importText(t, db, data, "csv", "people")
	if res.Rows != 2 {
		t.Fatalf("rows = %d, want 2", res.Rows)
	}
	if len(res.Columns) != 5 {
		t.Fatalf("columns = %v", res.Columns)
	}

	info, err := db.TableInfo(context.Background(), "people")
	if err != nil {
		t.Fatal(err)
	}
	wantTypes := map[string]string{
		"id":     "INTEGER",
		"name":   "TEXT",
		"score":  "REAL",
		"active": "INTEGER",
		"zip":    "TEXT",
	}
	for _, c := range info.Columns {
		if wantTypes[c.Name] != c.Type {
			t.Errorf("%s type = %q, want %q", c.Name, c.Type, wantTypes[c.Name])
		}
	}

	rs, err := db.Query(context.Background(),
		"SELECT name, score, active, zip FROM people ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	if got := rs.Rows[0][0]; got != "Ada" {
		t.Errorf("name = %v", got)
	}
	if got := rs.Rows[0][1]; got != 9.5 {
		t.Errorf("score = %v (%T), want 9.5", got, got)
	}
	if got := rs.Rows[0][2]; got != int64(1) {
		t.Errorf("active = %v (%T), want int64(1)", got, got)
	}
	if got := rs.Rows[0][3]; got != "00123" {
		t.Errorf("zip = %v, want \"00123\"", got)
	}
}

func TestImportCSVNoHeader(t *testing.T) {
	db := newTestDB(t)
	res, err := db.Import(context.Background(), strings.NewReader("1,Ada\n2,Grace\n"),
		ImportOptions{Table: "t", Format: "csv", NoHeader: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Rows != 2 {
		t.Fatalf("rows = %d", res.Rows)
	}
	if got := strings.Join(res.Columns, ","); got != "column_1,column_2" {
		t.Fatalf("columns = %s", got)
	}
}

func TestImportCSVHeaderOnly(t *testing.T) {
	db := newTestDB(t)
	res := importText(t, db, "id,name\n", "csv", "empty")
	if res.Rows != 0 {
		t.Fatalf("rows = %d, want 0", res.Rows)
	}
	info, err := db.TableInfo(context.Background(), "empty")
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Columns) != 2 {
		t.Fatalf("columns = %d", len(info.Columns))
	}
}

func TestImportJSONL(t *testing.T) {
	db := newTestDB(t)
	data := `{"id":1,"tags":["a","b"],"meta":{"ok":true}}
{"id":2,"tags":[],"meta":null}
`
	res := importText(t, db, data, "jsonl", "items")
	if res.Rows != 2 {
		t.Fatalf("rows = %d", res.Rows)
	}
	rs, err := db.Query(context.Background(), "SELECT id, tags, meta FROM items ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	if rs.Rows[0][0] != int64(1) {
		t.Errorf("id = %v (%T)", rs.Rows[0][0], rs.Rows[0][0])
	}
	if got := rs.Rows[0][1]; got != `["a","b"]` {
		t.Errorf("tags = %v", got)
	}
	if got := rs.Rows[0][2]; got != `{"ok":true}` {
		t.Errorf("meta = %v", got)
	}
	if rs.Rows[1][2] != nil {
		t.Errorf("meta null = %v", rs.Rows[1][2])
	}
}

func TestImportJSONArray(t *testing.T) {
	db := newTestDB(t)
	res := importText(t, db, `[{"a":1,"b":"x"},{"a":2,"b":"y"}]`, "json", "j")
	if res.Rows != 2 {
		t.Fatalf("rows = %d", res.Rows)
	}
	rs, err := db.Query(context.Background(), "SELECT a, b FROM j ORDER BY a")
	if err != nil {
		t.Fatal(err)
	}
	if rs.Rows[1][1] != "y" {
		t.Errorf("b = %v", rs.Rows[1][1])
	}
}

func TestImportAlterAddsColumns(t *testing.T) {
	db := newTestDB(t)
	importText(t, db, "id,name\n1,Ada\n", "csv", "people")
	_, err := db.Import(context.Background(), strings.NewReader("id,name,age\n2,Grace,30\n"),
		ImportOptions{Table: "people", Format: "csv", Alter: true})
	if err != nil {
		t.Fatal(err)
	}
	info, err := db.TableInfo(context.Background(), "people")
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Columns) != 3 {
		t.Fatalf("columns = %d, want 3", len(info.Columns))
	}
	rs, err := db.Query(context.Background(), "SELECT age FROM people ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	if rs.Rows[0][0] != nil {
		t.Errorf("old row age = %v, want NULL", rs.Rows[0][0])
	}
	if rs.Rows[1][0] != int64(30) {
		t.Errorf("new row age = %v (%T)", rs.Rows[1][0], rs.Rows[1][0])
	}
}

func TestImportMissingColumnWithoutAlter(t *testing.T) {
	db := newTestDB(t)
	importText(t, db, "id,name\n1,Ada\n", "csv", "people")
	_, err := db.Import(context.Background(), strings.NewReader("id,name,age\n2,Grace,30\n"),
		ImportOptions{Table: "people", Format: "csv"})
	if err == nil || !strings.Contains(err.Error(), "--alter") {
		t.Fatalf("err = %v, want an --alter hint", err)
	}
}

func TestImportRejectsInconsistentJSONLKeys(t *testing.T) {
	db := newTestDB(t)
	var b strings.Builder
	for i := 0; i < inferenceSample; i++ {
		fmt.Fprintf(&b, "{\"a\":%d}\n", i)
	}
	b.WriteString("{\"a\":1000,\"b\":3}\n")

	_, err := db.Import(context.Background(), strings.NewReader(b.String()),
		ImportOptions{Table: "t", Format: "jsonl"})
	if err == nil || !strings.Contains(err.Error(), "unexpected field") {
		t.Fatalf("err = %v, want unexpected field", err)
	}
}
