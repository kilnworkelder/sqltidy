# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] - 2026-11-26

### Added

- `import` for CSV, TSV, JSON, and JSONL with INTEGER/REAL/TEXT inference,
  primary keys, and `--alter` schema evolution.
- `query` with table, CSV, JSON, JSONL, and Markdown output.
- `schema` for tables, columns, indexes, and row counts.
- `transform` for renaming, adding, and dropping columns.
- `fts enable` / `fts search` built on SQLite FTS5 with external-content
  triggers and rebuild support.
- `upsert` using `INSERT ... ON CONFLICT DO UPDATE`.
- `export` to CSV, JSON, JSONL, Markdown, and text tables.
- CGO-free single binary built on `modernc.org/sqlite`.
