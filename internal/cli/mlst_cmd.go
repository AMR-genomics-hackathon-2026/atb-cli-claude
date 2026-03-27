package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	idx "github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/index"
	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/output"
)

func newMLSTCmd() *cobra.Command {
	var (
		species      string
		sequenceType string
		scheme       string
		mlstStatus   string
		hqOnly       bool
		limit        int
		format       string
		outputFile   string
	)

	cmd := &cobra.Command{
		Use:   "mlst",
		Short: "Query MLST (Multi-Locus Sequence Typing) data for bacterial genomes",
		Example: `  # Get all STs for E. coli
  atb mlst --species "Escherichia coli" --hq-only --limit 20

  # Find ST131 E. coli
  atb mlst --species "Escherichia coli" --st 131

  # Query by scheme name
  atb mlst --scheme salmonella --limit 50

  # Only perfect MLST calls
  atb mlst --species "Escherichia coli" --status PERFECT --limit 20`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := dataDir
			if dir == "" {
				dir = cfg.General.DataDir
			}

			if err := ensureDatabase(dir); err != nil {
				return err
			}

			db, err := idx.Open(dir)
			if err != nil {
				return fmt.Errorf("opening index: %w", err)
			}
			defer db.Close()

			if limit <= 0 {
				limit = 100
			}

			mlstCols := []string{
				"sample_accession",
				"sylph_species",
				"mlst_scheme",
				"mlst_st",
				"mlst_status",
				"mlst_score",
				"mlst_alleles",
			}

			rows, err := db.Query(idx.QueryParams{
				Species:      species,
				HQOnly:       hqOnly,
				SequenceType: sequenceType,
				Scheme:       scheme,
				MLSTStatus:   mlstStatus,
				Columns:      mlstCols,
				Limit:        limit,
			})
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}

			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No results found.")
				return nil
			}

			outRows := make([]output.Row, len(rows))
			for i, r := range rows {
				outRows[i] = output.Row(r)
			}

			var w io.Writer = cmd.OutOrStdout()
			if outputFile != "" {
				f, ferr := os.Create(outputFile)
				if ferr != nil {
					return fmt.Errorf("opening output file: %w", ferr)
				}
				defer f.Close()
				w = f
			}

			resolvedFormat := format
			if resolvedFormat == "" {
				resolvedFormat = "tsv"
			}

			return output.Format(w, outRows, mlstCols, resolvedFormat)
		},
	}

	cmd.Flags().StringVar(&species, "species", "", "filter by species name (case-insensitive)")
	cmd.Flags().StringVar(&sequenceType, "sequence-type", "", "filter by sequence type (ST number)")
	cmd.Flags().StringVar(&sequenceType, "st", "", "filter by sequence type (shorthand for --sequence-type)")
	cmd.Flags().StringVar(&scheme, "scheme", "", "filter by MLST scheme name")
	cmd.Flags().StringVar(&mlstStatus, "status", "", "filter by MLST status (PERFECT, NOVEL, OK, MIXED, BAD, NONE, MISSING)")
	cmd.Flags().BoolVar(&hqOnly, "hq-only", false, "only include high-quality genomes (hq_filter=PASS)")
	cmd.Flags().IntVar(&limit, "limit", 100, "maximum number of results")
	cmd.Flags().StringVar(&format, "format", "tsv", "output format: tsv, csv, json, table")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "write output to file instead of stdout")

	return cmd
}
