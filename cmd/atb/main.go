package main

import (
	"fmt"
	"os"

	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/cli"
)

// version is set via ldflags at build time.
var version = "dev"

func main() {
	cli.RootCmd.Version = version
	if err := cli.RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
