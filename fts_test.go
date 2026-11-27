package sqltidy

import (
	"context"
	"strings"
	"testing"
)

func TestFTSIndexesExistingAndNewRows(t *testing.T) {
	db := newTestDB(t)
	importText(t, db,
		"id,name,notes\n1,Ada,first programmer\n2,Grace,compiler pioneer\n3,Linus,kernel\n",
		"csv", "people")
	ctx := context.Background()

	if err := db.EnableFTS(ctx, "people"); err != nil {
		t.Fatal(err)
	}

	rs, err := db.SearchFTS(ctx, "people", "Ada", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rows) != 1 {
		t.Fatalf("Ada matches = %d, want 1", len(rs.Rows))
	}
	if rs.Rows[0][1] != "Ada" {
		t.Errorf("matched row = %v", rs.Rows[0])
	}

	if _, err := db.SQL().ExecContext(ctx,
		"INSERT INTO people (id, name, notes) VALUES (4, 'Barbara', 'compiler pioneer')"); err != nil {
		t.Fatal(err)
	}
	rs, err = db.SearchFTS(ctx, "people", "compiler", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rows) != 2 {
		t.Fatalf("compiler matches = %d, want 2", len(rs.Rows))
	}

	rs, err = db.SearchFTS(ctx, "people", "compiler", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rows) != 1 {
		t.Fatalf("limited matches = %d, want 1", len(rs.Rows))
	}

	if err := db.RebuildFTS(ctx, "people"); err != nil {
		t.Fatal(err)
	}
	rs, err = db.SearchFTS(ctx, "people", "compiler", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rows) != 2 {
		t.Fatalf("after rebuild = %d, want 2", len(rs.Rows))
	}
}

func TestFTSNotEnabled(t *testing.T) {
	db := newTestDB(t)
	importText(t, db, "id,name\n1,Ada\n", "csv", "people")
	_, err := db.SearchFTS(context.Background(), "people", "Ada", 10)
	if err == nil || !strings.Contains(err.Error(), "not enabled") {
		t.Fatalf("err = %v, want not enabled", err)
	}
}

func TestFTSEmptyQuery(t *testing.T) {
	db := newTestDB(t)
	importText(t, db, "id,name\n1,Ada\n", "csv", "people")
	if err := db.EnableFTS(context.Background(), "people"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SearchFTS(context.Background(), "people", "  ", 10); err == nil {
		t.Fatal("expected an error for an empty query")
	}
}
