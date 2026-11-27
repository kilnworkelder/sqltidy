package cli

import (
	"fmt"

	"github.com/kilnworkelder/sqltidy"
	"github.com/spf13/cobra"
)

func newTransformCmd() *cobra.Command {
	var renames, adds, drops []string
	cmd := &cobra.Command{
		Use:   "transform <table>",
		Short: "Rename, add, or drop columns",
		Long: "Reshape a table in place. Operations run in the order --rename,\n" +
			"--add, --drop. Types are restricted to SQLite storage classes.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(renames)+len(adds)+len(drops) == 0 {
				return fmt.Errorf("nothing to do; use --rename, --add, or --drop")
			}
			db, err := openDB()
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			table := args[0]
			w := cmd.OutOrStdout()
			for _, spec := range renames {
				oldName, newName, err := splitPair(spec)
				if err != nil {
					return fmt.Errorf("--rename %s: %w", spec, err)
				}
				if newName == "" {
					return fmt.Errorf("--rename %s: new name is required", spec)
				}
				if err := db.RenameColumn(cmd.Context(), table, oldName, newName); err != nil {
					return err
				}
				_, _ = fmt.Fprintf(w, "renamed %s.%s -> %s\n", table, oldName, newName)
			}
			for _, spec := range adds {
				name, typ, err := splitPair(spec)
				if err != nil {
					return fmt.Errorf("--add %s: %w", spec, err)
				}
				if err := db.AddColumn(cmd.Context(), table, name, typ); err != nil {
					return err
				}
				_, _ = fmt.Fprintf(w, "added %s.%s\n", table, name)
			}
			for _, name := range drops {
				if err := db.DropColumn(cmd.Context(), table, name); err != nil {
					return err
				}
				_, _ = fmt.Fprintf(w, "dropped %s.%s\n", table, name)
			}
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&renames, "rename", nil, "rename a column (old:new)")
	cmd.Flags().StringArrayVar(&adds, "add", nil, "add a column (name:type, type defaults to TEXT)")
	cmd.Flags().StringArrayVar(&drops, "drop", nil, "drop a column")
	return cmd
}

func newFTSCmd() *cobra.Command {
	fts := &cobra.Command{
		Use:   "fts",
		Short: "Full-text search with SQLite FTS5",
	}
	fts.AddCommand(newFTSEnableCmd(), newFTSSearchCmd())
	return fts
}

func newFTSEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <table>",
		Short: "Create and populate an FTS5 index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB()
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			table := args[0]
			if err := db.EnableFTS(cmd.Context(), table); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "enabled FTS for %s (index %s)\n", table, sqltidy.FTSName(table))
			return nil
		},
	}
}

func newFTSSearchCmd() *cobra.Command {
	var limit int
	var format string
	cmd := &cobra.Command{
		Use:   "search <table> <query>",
		Short: "Search an FTS5 index",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB()
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			rs, err := db.SearchFTS(cmd.Context(), args[0], args[1], limit)
			if err != nil {
				return err
			}
			return writeFormatted(cmd.OutOrStdout(), rs, format)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum rows (0 = no limit)")
	cmd.Flags().StringVar(&format, "format", "table", "output format: table, csv, json, jsonl, markdown")
	return cmd
}
