package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/mcpserver"
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server for LLM integration",
		Long: `Start a Model Context Protocol (MCP) server over stdio.

This allows LLMs like Claude to query the AllTheBacteria database
through structured tool calls.

The server exposes 5 tools:
  atb_query        Search bacterial genomes by species, genus, quality, etc.
  atb_amr          Query AMR resistance genes for a species
  atb_info         Get all metadata for a specific sample accession
  atb_stats        Database summary statistics
  atb_species_list List available species sorted by sample count

Usage with Claude Code:
  claude mcp add atb -- atb mcp --data-dir ~/atb/metadata/parquet

Usage with Claude Desktop (add to claude_desktop_config.json):
  {
    "mcpServers": {
      "atb": {
        "command": "atb",
        "args": ["mcp", "--data-dir", "/path/to/data"]
      }
    }
  }`,
		Example: `  # Start MCP server (requires built index)
  atb mcp --data-dir ~/atb/metadata/parquet

  # Add to Claude Code
  claude mcp add atb -- atb mcp --data-dir ~/atb/metadata/parquet`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := dataDir
			if dir == "" {
				dir = cfg.General.DataDir
			}
			if dir == "" {
				home, _ := os.UserHomeDir()
				dir = filepath.Join(home, ".atb", "data")
			}

			if err := mcpserver.Serve(cmd.Context(), dir); err != nil {
				return fmt.Errorf("MCP server error: %w", err)
			}
			return nil
		},
	}
	return cmd
}
