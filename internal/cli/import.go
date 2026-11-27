package cli

import (
	"fmt"

	"github.com/kilnworkelder/sqltidy"
	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	var (
		opts     sqltidy.ImportOptions
		noHeader bool
		comma    string
	)
	cmd := &cobra.Command{
		Use:   "import [file]",
		Short: "Import CSV, TSV, JSON, or JSONL into a table",
		Long: "Import a file (or stdin with '-') into a table. The table is created\n" +
			"with inferred column types; pass --alter to add new columns to an\n" +
			"existing table.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "-"
			if len(args) == 1 {
				path = args[0]
			}
			format, err := resolveInputFormat(opts.Format, path)
			if err != nil {
				return err
			}
			delim, err := parseComma(comma)
			if err != nil {
				return err
			}
			if opts.Table == "" {
				if path == "-" {
					return fmt.Errorf("--table is required when reading from stdin")
				}
				opts.Table = tableFromPath(path)
			}
			if opts.Table == "" {
				return fmt.Errorf("could not derive a table name; use --table")
			}
			opts.Format = format
			opts.NoHeader = noHeader
			opts.Comma = delim

			r, closeFn, err := openInput(path)
			if err != nil {
				return err
			}
			defer closeFn()

			db, err := openDB()
			if err != nil {
				return err
			}
			defer db.Close()

			res, err := db.Import(cmd.Context(), r, opts)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "imported %d rows into %s (%d columns)\n",
				res.Rows, res.Table, len(res.Columns))
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Table, "table", "", "target table (default: file name without extension)")
	cmd.Flags().StringVar(&opts.Format, "format", "", "input format: csv, tsv, json, jsonl (default: from extension)")
	cmd.Flags().StringSliceVar(&opts.PK, "pk", nil, "primary key column(s), comma-separated")
	cmd.Flags().BoolVar(&opts.Alter, "alter", false, "add missing columns to an existing table")
	cmd.Flags().BoolVar(&noHeader, "no-header", false, "treat the first row as data")
	cmd.Flags().StringVar(&comma, "comma", "", "CSV delimiter (single character)")
	return cmd
}
