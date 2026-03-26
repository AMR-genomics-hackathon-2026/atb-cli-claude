package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/query"
	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/summarise"
)

func newSummariseCmd() *cobra.Command {
	var (
		by   []string
		topN int
		from string
	)

	cmd := &cobra.Command{
		Use:     "summarise",
		Aliases: []string{"summarize"},
		Short:   "Print summary statistics for the ATB database",
		Long: `Print summary statistics for all genomes in the local ATB database.

By default prints total counts, HQ fraction, top species, and dataset breakdown.
Use --by to group results by a specific column (e.g. --by sylph_species).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var rows []query.ResultRow

			if from != "" {
				var err error
				rows, err = readResultsFromFile(from)
				if err != nil {
					return fmt.Errorf("reading --from file: %w", err)
				}
			} else {
				cfg, err := loadConfig()
				if err != nil {
					return err
				}

				dir := dataDir
				if dir == "" {
					dir = cfg.General.DataDir
				}

				if _, err := os.Stat(dir); os.IsNotExist(err) {
					return fmt.Errorf("data directory does not exist: %s\n\nRun 'atb fetch' to download parquet tables.", dir)
				}

				// Run a full query with no filters to get all rows
				rows, err = query.Execute(dir, query.Filters{}, nil)
				if err != nil {
					return fmt.Errorf("reading database: %w", err)
				}
			}

			w := cmd.OutOrStdout()

			if len(by) > 0 {
				for _, dim := range by {
					dim = strings.TrimSpace(dim)
					groups := summarise.GroupBy(rows, dim)
					summarise.PrintGroupBy(w, groups, dim, topN)
					fmt.Fprintln(w)
				}
				return nil
			}

			s := summarise.DefaultSummary(rows)
			summarise.PrintSummary(w, s, topN)
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&by, "by", nil, "group results by column(s) (e.g. --by sylph_species)")
	cmd.Flags().IntVar(&topN, "top", 10, "number of top entries to show per group")
	cmd.Flags().StringVar(&from, "from", "", "read rows from a CSV/TSV query result file (use \"-\" for stdin)")

	return cmd
}
