# sqltidy

[![CI](https://github.com/kilnworkelder/sqltidy/actions/workflows/ci.yml/badge.svg)](https://github.com/kilnworkelder/sqltidy/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/kilnworkelder/sqltidy.svg)](https://pkg.go.dev/github.com/kilnworkelder/sqltidy)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A single-binary SQLite data toolkit: import messy CSV/TSV/JSON/JSONL, infer
types, reshape the schema, add full-text search, query, and export — without
Python, a runtime, or a C toolchain.

```console
$ sqltidy import sales.csv --table sales --pk id --db shop.db
imported 3 rows into sales (6 columns)

$ sqltidy query "select region, sum(amount) from sales group by 1" --db shop.db
region  sum(amount)
------  -----------
EU      19.75
US      99
```

## Why

SQLite is the best default store for small and medium datasets, but getting
data *into* it and back *out* usually means Python's `sqlite-utils`, a shell
script around the `sqlite3` binary, or a pile of `INSERT` statements. sqltidy
is a focused, cross-platform alternative that installs with `go install` and
runs the same way on Linux, macOS, and Windows.

It is deliberately not a database client (use `usql`) or an ephemeral SQL
engine (use `trdsql`). It owns the curation workflow: import → transform →
index → query → export.

## Install

Requires Go 1.26 or newer.

```console
go install github.com/kilnworkelder/sqltidy/cmd/sqltidy@latest
```

Prebuilt archives for Linux, macOS, and Windows (amd64/arm64) are attached to
each [release](https://github.com/kilnworkelder/sqltidy/releases).

## Quickstart

```console
# Create a table with inferred INTEGER/REAL/TEXT columns.
sqltidy import sales.csv --table sales --pk id --db shop.db

# Add a column and rename another in place.
sqltidy transform sales --add currency:TEXT --rename amount:total --db shop.db

# Full-text search over every column.
sqltidy fts enable sales --db shop.db
sqltidy fts search sales "compiler pioneer" --db shop.db --limit 5

# Query with any output format.
sqltidy query "select * from sales" --db shop.db --format markdown

# Insert new rows and update rows that collide on the primary key.
sqltidy upsert sales new_sales.csv --pk id --db shop.db

# Export the table.
sqltidy export sales --format jsonl -o sales.jsonl --db shop.db
```

`-` means stdin/stdout, so sqltidy composes in pipelines:

```console
$ curl -s https://example.com/data.csv | sqltidy import - --table data --format csv
$ sqltidy query "select * from data" --format csv | gzip > data.csv.gz
```

## Commands

| Command | What it does |
| --- | --- |
| `import [file]` | Import CSV, TSV, JSON, or JSONL. Infers column types, creates the table, supports `--pk`, `--alter`, `--no-header`, `--comma`. |
| `query "SQL"` | Run a query. `--format table\|csv\|json\|jsonl\|markdown`. |
| `schema [table]` | List tables, columns, indexes, and row counts. |
| `transform <table>` | `--rename old:new`, `--add name:type`, `--drop name`. |
| `fts enable <table>` | Build an external-content FTS5 index and keep-in-sync triggers. |
| `fts search <table> <query>` | FTS5 search ordered by relevance. |
| `upsert <table> <file>` | `INSERT ... ON CONFLICT DO UPDATE`, requires `--pk`. |
| `export <table>` | Write a table as CSV, JSON, JSONL, Markdown, or a text table. |
| `version` | Print version, commit, and build date. |

Every command accepts `--db` (default `sqltidy.db`).

## Type inference

- Integer-looking values become `INTEGER`; decimals and exponents become
  `REAL`; `true`/`false` become `1`/`0` in an `INTEGER` column.
- Leading zeros keep a column as `TEXT`, so ZIP codes and phone numbers
  round-trip unchanged.
- Empty CSV fields become `NULL` in numeric columns and `""` in text columns.
- Nested JSON objects and arrays are stored as compact JSON text.
- The first 1,000 rows are sampled to infer types. CSV/TSV data after that
  window is streamed; JSONL columns are fixed once the sample is read and a
  later unknown key is an error.

## Library

The CLI is a thin wrapper around the root package:

```go
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/kilnworkelder/sqltidy"
)

func main() {
	db, err := sqltidy.Open("shop.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	res, err := db.Import(context.Background(),
		strings.NewReader("id,name,amount\n1,Ada,12.5\n"),
		sqltidy.ImportOptions{Table: "sales", Format: "csv", PK: []string{"id"}})
	if err != nil {
		panic(err)
	}
	fmt.Println(res.Rows)

	rs, err := db.Query(context.Background(), "select name from sales")
	if err != nil {
		panic(err)
	}
	fmt.Println(rs.Rows)
}
```

## Development

```console
go test ./...
go vet ./...
go build ./...
```

Tests cover type inference, all input formats, schema alteration, transforms,
upserts, FTS5 triggers and rebuilds, and every output format.

## Release

1. Update `CHANGELOG.md` and tag a semver version:

   ```console
   git tag -a v0.1.0 -m "v0.1.0"
   git push origin main --tags
   ```

2. The `Release` workflow runs GoReleaser, which cross-compiles, adds
   checksums, and attaches archives to the GitHub release. Tagging also makes
   the module available through `proxy.golang.org`, so `go install
   github.com/kilnworkelder/sqltidy/cmd/sqltidy@v0.1.0` starts working and pkg.go.dev
   indexes the library automatically.

3. Optional: create `kilnworkelder/homebrew-tap` and `kilnworkelder/scoop-bucket`, uncomment
   the matching blocks in `.goreleaser.yaml`, and add the repository tokens.

## License

MIT — see [LICENSE](LICENSE).
