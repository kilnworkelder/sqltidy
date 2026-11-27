package sqltidy

import (
	"context"
	"strings"
	"testing"
)

func TestTransform(t *testing.T) {
	db := newTestDB(t)
	importText(t, db, "id,name,score\n1,Ada,9.5\n", "csv", "people")
	ctx := context.Background()

	if err := db.RenameColumn(ctx, "people", "name", "full_name"); err != nil {
		t.Fatal(err)
	}
	if err := db.AddColumn(ctx, "people", "age", "INTEGER"); err != nil {
		t.Fatal(err)
	}
	if err := db.DropColumn(ctx, "people", "score"); err != nil {
		t.Fatal(err)
	}

	info, err := db.TableInfo(ctx, "people")
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(info.Columns))
	for i, c := range info.Columns {
		got[i] = c.Name
	}
	if strings.Join(got, ",") != "id,full_name,age" {
		t.Fatalf("columns = %v", got)
	}
	rs, err := db.Query(ctx, "SELECT full_name FROM people")
	if err != nil {
		t.Fatal(err)
	}
	if rs.Rows[0][0] != "Ada" {
		t.Fatalf("full_name = %v", rs.Rows[0][0])
	}
}

func TestTransformRejectsUnsafeType(t *testing.T) {
	db := newTestDB(t)
	importText(t, db, "id\n1\n", "csv", "t")
	err := db.AddColumn(context.Background(), "t", "x", "INTEGER; DROP TABLE t")
	if err == nil {
		t.Fatal("expected an error for an unsafe type")
	}
}

func TestRenameMissingColumnFails(t *testing.T) {
	db := newTestDB(t)
	importText(t, db, "id\n1\n", "csv", "t")
	if err := db.RenameColumn(context.Background(), "t", "nope", "x"); err == nil {
		t.Fatal("expected an error")
	}
}
