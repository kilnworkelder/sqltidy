package cli

import (
	"fmt"

	"github.com/kilnworkelder/sqltidy"
	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	var format, output string
	cmd := &cobra.Command{
		Use:   "export <table>",
		Short: "Export a table as CSV, JSON, JSONL, or Markdown",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := sqltidy.ParseFormat(format)
			if err != nil {
				return err
			}
			w, closeFn, err := openOutput(output)
			if err != nil {
				return err
			}
			defer func() { _ = closeFn() }()

			db, err := openDB()
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			return db.Export(cmd.Context(), args[0], w, f)
		},
	}
	cmd.Flags().StringVar(&format, "format", "csv", "output format: csv, json, jsonl, markdown, table")
	cmd.Flags().StringVarP(&output, "output", "o", "-", "output file (default: stdout)")
	return cmd
}

func newUpsertCmd() *cobra.Command {
	var (
		opts     sqltidy.ImportOptions
		noHeader bool
		comma    string
	)
	cmd := &cobra.Command{
		Use:   "upsert <table> <file>",
		Short: "Insert rows, updating rows that collide on the primary key",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Table = args[0]
			path := args[1]
			if len(opts.PK) == 0 {
				return fmt.Errorf("--pk is required")
			}
			format, err := resolveInputFormat(opts.Format, path)
			if err != nil {
				return err
			}
			delim, err := parseComma(comma)
			if err != nil {
				return err
			}
			opts.Format = format
			opts.NoHeader = noHeader
			opts.Comma = delim

			r, closeFn, err := openInput(path)
			if err != nil {
				return err
			}
			defer func() { _ = closeFn() }()

			db, err := openDB()
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			res, err := db.Upsert(cmd.Context(), r, opts)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "upserted %d rows into %s\n", res.Rows, res.Table)
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Format, "format", "", "input format: csv, tsv, json, jsonl (default: from extension)")
	cmd.Flags().StringSliceVar(&opts.PK, "pk", nil, "primary key column(s), comma-separated")
	cmd.Flags().BoolVar(&noHeader, "no-header", false, "treat the first row as data")
	cmd.Flags().StringVar(&comma, "comma", "", "CSV delimiter (single character)")
	return cmd
}
