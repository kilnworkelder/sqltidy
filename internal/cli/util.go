package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kilnworkelder/sqltidy"
)

func openInput(path string) (io.Reader, func() error, error) {
	if path == "" || path == "-" {
		return os.Stdin, func() error { return nil }, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

func openOutput(path string) (io.Writer, func() error, error) {
	if path == "" || path == "-" {
		return os.Stdout, func() error { return nil }, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

func resolveInputFormat(explicit, path string) (string, error) {
	if explicit != "" {
		switch strings.ToLower(strings.TrimSpace(explicit)) {
		case "csv":
			return "csv", nil
		case "tsv", "tab":
			return "tsv", nil
		case "json":
			return "json", nil
		case "jsonl", "ndjson":
			return "jsonl", nil
		default:
			return "", fmt.Errorf("unsupported input format %q (want csv, tsv, json, or jsonl)", explicit)
		}
	}
	if path == "" || path == "-" {
		return "csv", nil
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv":
		return "csv", nil
	case ".tsv", ".tab":
		return "tsv", nil
	case ".json":
		return "json", nil
	case ".jsonl", ".ndjson":
		return "jsonl", nil
	default:
		return "", fmt.Errorf("cannot infer format from %q; use --format", path)
	}
}

func tableFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func parseComma(s string) (rune, error) {
	if s == "" {
		return 0, nil
	}
	r := []rune(s)
	if len(r) != 1 {
		return 0, fmt.Errorf("--comma must be a single character")
	}
	return r[0], nil
}

func writeFormatted(w io.Writer, rs *sqltidy.ResultSet, name string) error {
	f, err := sqltidy.ParseFormat(name)
	if err != nil {
		return err
	}
	return sqltidy.WriteResult(w, rs, f)
}

func splitPair(s string) (string, string, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return "", "", fmt.Errorf("expected NAME:VALUE, got %q", s)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}
