// Package cli wires the sqltidy commands to the library.
package cli

import (
	"fmt"
	"os"

	"github.com/kilnworkelder/sqltidy"
	"github.com/spf13/cobra"
)

// BuildInfo is injected at release time via -ldflags.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

var build = BuildInfo{Version: "dev", Commit: "none", Date: "unknown"}

// SetBuildInfo sets the values printed by the version command.
func SetBuildInfo(b BuildInfo) { build = b }

var dbPath string

// Execute runs the CLI and returns a process exit code.
func Execute() int {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "sqltidy:", err)
		return 1
	}
	return 0
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "sqltidy",
		Short: "A single-binary SQLite data toolkit",
		Long: "sqltidy imports messy CSV, TSV, JSON, and JSONL into SQLite, lets you\n" +
			"reshape and index it, and exports it back out. It needs no Python, no\n" +
			"runtime, and no C toolchain.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&dbPath, "db", "sqltidy.db", "SQLite database file")
	root.AddCommand(
		newImportCmd(),
		newQueryCmd(),
		newSchemaCmd(),
		newTransformCmd(),
		newFTSCmd(),
		newExportCmd(),
		newUpsertCmd(),
		newVersionCmd(),
	)
	return root
}

func openDB() (*sqltidy.DB, error) { return sqltidy.Open(dbPath) }
