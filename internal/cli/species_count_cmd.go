package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	idx "github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/index"
	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/output"
)

// isGTDBPlaceholder reports whether a species name is an unnamed GTDB genomic
// cluster placeholder (e.g. "Escherichia sp001234567"). These are assembly bins
// that lack a formal species name and are typically unhelpful in exploratory
// queries.
func isGTDBPlaceholder(species string) bool {
	// GTDB placeholders contain "spNNNNNN" where N is a digit and the run is
	// at least 6 digits long — matching patterns like "sp000746275".
	lower := strings.ToLower(species)
	idx := strings.Index(lower, " sp")
	if idx == -1 {
		return false
	}
	rest := lower[idx+3:]
	digits := 0
	for _, c := range rest {
		if c >= '0' && c <= '9' {
			digits++
		} else {
			break
		}
	}
	return digits >= 6
}

func newSpeciesCountCmd() *cobra.Command {
	var (
		top    int
		hqOnly bool
		format string
		noPlaceholders bool
	)

	cmd := &cobra.Command{
		Use:   "species-count",
		Short: "Show genome counts per species in the ATB database",
		Long: `Show the number of genomes per species in the local ATB database,
sorted from most to fewest. Useful for discovering which species are
well-represented in the database.

By default, unnamed GTDB placeholder species (e.g. "Escherichia sp001234567")
are excluded. Use --all to include them.`,
		Example: `  # Top 20 species by genome count
  atb species-count --top 20

  # Only count high-quality genomes
  atb species-count --hq-only --top 30

  # Include GTDB unnamed placeholder species
  atb species-count --all

  # Output as CSV
  atb species-count --top 50 --format csv`,
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

			if !idx.Exists(dir) {
				return fmt.Errorf("index not found; run 'atb index' to build it first")
			}

			db, err := idx.Open(dir)
			if err != nil {
				return fmt.Errorf("opening index: %w", err)
			}
			defer db.Close()

			counts, err := db.SpeciesCountList(top*10, hqOnly)
			if err != nil {
				return fmt.Errorf("querying species counts: %w", err)
			}

			// Filter out GTDB placeholder species unless --all was requested.
			if noPlaceholders {
				filtered := counts[:0]
				for _, sc := range counts {
					if !isGTDBPlaceholder(sc.Species) {
						filtered = append(filtered, sc)
					}
				}
				counts = filtered
			}

			// Apply top cap after filtering.
			if top > 0 && len(counts) > top {
				counts = counts[:top]
			}

			w := cmd.OutOrStdout()

			resolvedFormat := format
			if resolvedFormat == "" {
				resolvedFormat = "table"
			}

			if resolvedFormat == "table" {
				return printSpeciesCountTable(w, counts, hqOnly)
			}

			// Convert to output rows for CSV/TSV/JSON output.
			rows := make([]output.Row, len(counts))
			for i, sc := range counts {
				rows[i] = output.Row{
					"species": sc.Species,
					"count":   fmt.Sprintf("%d", sc.Count),
				}
			}
			cols := []string{"species", "count"}
			return output.Format(w, rows, cols, resolvedFormat)
		},
	}

	cmd.Flags().IntVar(&top, "top", 20, "number of top species to show (0 = all)")
	cmd.Flags().BoolVar(&hqOnly, "hq-only", false, "only count high-quality genomes (hq_filter=PASS)")
	cmd.Flags().StringVar(&format, "format", "table", "output format: table, tsv, csv, json")
	cmd.Flags().BoolVar(&noPlaceholders, "no-placeholders", true, "exclude unnamed GTDB placeholder species (e.g. Genus sp001234567)")
	// --all is a convenience alias that sets no-placeholders=false
	cmd.Flags().BoolP("all", "a", false, "include GTDB unnamed placeholder species in output")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("all") {
			noPlaceholders = false
		}
		return nil
	}

	return cmd
}

func printSpeciesCountTable(w io.Writer, counts []idx.SpeciesCount, hqOnly bool) error {
	qualifier := ""
	if hqOnly {
		qualifier = " (HQ only)"
	}
	fmt.Fprintf(w, "%-55s %10s\n", "Species", "Count"+qualifier)
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 67))
	total := 0
	for _, sc := range counts {
		name := sc.Species
		if name == "" {
			name = "(unclassified)"
		}
		fmt.Fprintf(w, "%-55s %10d\n", name, sc.Count)
		total += sc.Count
	}
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 67))
	fmt.Fprintf(w, "%-55s %10d\n", fmt.Sprintf("Total (%d species shown)", len(counts)), total)
	return nil
}
