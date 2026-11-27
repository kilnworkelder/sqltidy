package sqltidy

import (
	"context"
	"strings"
	"testing"
)

func TestUpsertUpdatesAndInserts(t *testing.T) {
	db := newTestDB(t)
	_, err := db.Import(context.Background(), strings.NewReader("id,name\n1,Ada\n2,Grace\n"),
		ImportOptions{Table: "people", Format: "csv", PK: []string{"id"}})
	if err != nil {
		t.Fatal(err)
	}

	res, err := db.Upsert(context.Background(),
		strings.NewReader("id,name\n2,Grace Hopper\n3,Linus\n"),
		ImportOptions{Table: "people", Format: "csv", PK: []string{"id"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Rows != 2 {
		t.Fatalf("rows = %d, want 2", res.Rows)
	}

	rs, err := db.Query(context.Background(), "SELECT id, name FROM people ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rows) != 3 {
		t.Fatalf("row count = %d, want 3", len(rs.Rows))
	}
	if rs.Rows[1][1] != "Grace Hopper" {
		t.Errorf("updated name = %v", rs.Rows[1][1])
	}
	if rs.Rows[2][1] != "Linus" {
		t.Errorf("inserted name = %v", rs.Rows[2][1])
	}
}

func TestUpsertRequiresPK(t *testing.T) {
	db := newTestDB(t)
	importText(t, db, "id\n1\n", "csv", "t")
	_, err := db.Upsert(context.Background(), strings.NewReader("id\n2\n"),
		ImportOptions{Table: "t", Format: "csv"})
	if err == nil || !strings.Contains(err.Error(), "--pk") {
		t.Fatalf("err = %v, want --pk error", err)
	}
}

func TestUpsertRequiresExistingTable(t *testing.T) {
	db := newTestDB(t)
	_, err := db.Upsert(context.Background(), strings.NewReader("id\n1\n"),
		ImportOptions{Table: "missing", Format: "csv", PK: []string{"id"}})
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("err = %v", err)
	}
}
