package sqltidy

import (
	"context"
	"strings"
	"testing"
)

func TestSchemaAndTables(t *testing.T) {
	db := newTestDB(t)
	importText(t, db, "id,name\n1,Ada\n2,Grace\n", "csv", "people")
	ctx := context.Background()
	if err := db.EnableFTS(ctx, "people"); err != nil {
		t.Fatal(err)
	}

	names, err := db.Tables(ctx)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "people") {
		t.Fatalf("tables missing people: %v", names)
	}
	for _, n := range names {
		for _, shadow := range []string{"_data", "_idx", "_content", "_docsize", "_config"} {
			if strings.HasSuffix(n, shadow) {
				t.Errorf("shadow table leaked into Tables: %s", n)
			}
		}
	}

	schema, err := db.Schema(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var base *TableInfo
	for i := range schema {
		if schema[i].Name == "people" {
			base = &schema[i]
		}
	}
	if base == nil {
		t.Fatal("people missing from Schema")
	}
	if base.Rows != 2 {
		t.Errorf("rows = %d, want 2", base.Rows)
	}
	if len(base.Columns) != 2 {
		t.Errorf("columns = %d, want 2", len(base.Columns))
	}
}

func TestTableInfoMissing(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.TableInfo(context.Background(), "nope"); err == nil {
		t.Fatal("expected an error")
	}
}
