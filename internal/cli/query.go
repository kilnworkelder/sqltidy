package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/kilnworkelder/sqltidy"
	"github.com/spf13/cobra"
)

func newQueryCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   `query "SQL"`,
		Short: "Run a SQL query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB()
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			rs, err := db.Query(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeFormatted(cmd.OutOrStdout(), rs, format)
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format: table, csv, json, jsonl, markdown")
	return cmd
}

func newSchemaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema [table]",
		Short: "Show tables, columns, and indexes",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB()
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			w := cmd.OutOrStdout()
			if len(args) == 1 {
				info, err := db.TableInfo(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				return printTableInfo(w, info)
			}
			schema, err := db.Schema(cmd.Context())
			if err != nil {
				return err
			}
			if len(schema) == 0 {
				_, err := fmt.Fprintln(w, "no tables")
				return err
			}
			for i := range schema {
				if err := printTableInfo(w, &schema[i]); err != nil {
					return err
				}
				if i < len(schema)-1 {
					if _, err := fmt.Fprintln(w); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
	return cmd
}

func printTableInfo(w io.Writer, info *sqltidy.TableInfo) error {
	if _, err := fmt.Fprintf(w, "%s (%d rows, %d columns)\n", info.Name, info.Rows, len(info.Columns)); err != nil {
		return err
	}
	if len(info.Indexes) > 0 {
		if _, err := fmt.Fprintf(w, "indexes: %s\n", strings.Join(info.Indexes, ", ")); err != nil {
			return err
		}
	}
	rs := &sqltidy.ResultSet{Columns: []string{"column", "type", "not null", "pk", "default"}}
	for _, c := range info.Columns {
		def := ""
		if c.Default != nil {
			def = *c.Default
		}
		rs.Rows = append(rs.Rows, []any{c.Name, c.Type, c.NotNull, c.PK, def})
	}
	return sqltidy.WriteResult(w, rs, sqltidy.FormatTable)
}
