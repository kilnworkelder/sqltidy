package main

import (
	"os"

	"github.com/kilnworkelder/sqltidy/internal/cli"
)

// Injected by GoReleaser via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.SetBuildInfo(cli.BuildInfo{Version: version, Commit: commit, Date: date})
	os.Exit(cli.Execute())
}
