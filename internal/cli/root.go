package cli

import (
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	dataDir string
)

// RootCmd is the base command for atb.
var RootCmd = &cobra.Command{
	Use:          "atb",
	Short:        "Query and download AllTheBacteria genomes",
	SilenceUsage: true,
}

func init() {
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default $HOME/.atb/config.toml)")
	RootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", "data directory for downloaded files (default $HOME/.atb/data)")
}
