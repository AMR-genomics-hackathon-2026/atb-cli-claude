package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/amr"
	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/output"
	pq "github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/parquet"
)

func newAMRCmd() *cobra.Command {
	var (
		species     string
		elementType string
		class       string
		gene        string
		hqOnly      bool
		minCoverage float64
		minIdentity float64
		limit       int
		format      string
		outputFile  string
	)

	cmd := &cobra.Command{
		Use:   "amr",
		Short: "Query AMR gene data for a species",
		Long: `Query AMRFinderPlus gene hits from the merged amrfinderplus.parquet file.

Run 'atb fetch' to download the data before querying.`,
		Example: `  # Get AMR gene hits for E. coli (HQ only)
  atb amr --species "Escherichia coli" --hq-only --limit 100

  # Filter by drug class
  atb amr --species "Escherichia coli" --class "BETA-LACTAM"

  # Search for beta-lactamase genes
  atb amr --species "Escherichia coli" --gene "bla%"

  # Query stress response genes
  atb amr --species "Escherichia coli" --type stress

  # Query all element types (AMR + stress + virulence)
  atb amr --species "Klebsiella pneumoniae" --type all --hq-only`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := dataDir
			if dir == "" {
				dir = cfg.General.DataDir
			}

			if species == "" {
				return fmt.Errorf("--species is required")
			}

			genus := pq.GenusFromSpecies(species)
			if genus == "" {
				return fmt.Errorf("could not derive genus from species %q", species)
			}

			// Check if amrfinderplus.parquet exists
			amrPath := filepath.Join(dir, amr.AMRFileName)
			if _, statErr := os.Stat(amrPath); statErr != nil {
				return fmt.Errorf("AMR data not found — run 'atb fetch' to download %s", amr.AMRFileName)
			}

			// Optionally build HQ sample set from assembly.parquet
			var sampleSet map[string]struct{}
			if hqOnly {
				assemblyPath := filepath.Join(dir, "assembly.parquet")
				hqRows, hqErr := pq.ReadStreamFiltered[pq.AssemblyRow](assemblyPath, func(r pq.AssemblyRow) bool {
					if r.HQFilter != "PASS" {
						return false
					}
					return strings.EqualFold(pq.GenusFromSpecies(r.SylphSpecies), genus)
				}, 0)
				if hqErr != nil {
					return fmt.Errorf("loading HQ samples: %w", hqErr)
				}
				sampleSet = make(map[string]struct{}, len(hqRows))
				for _, r := range hqRows {
					sampleSet[r.SampleAccession] = struct{}{}
				}
			}

			filters := amr.Filters{
				Samples:     sampleSet,
				Class:       class,
				GenePattern: gene,
				MinCoverage: minCoverage,
				MinIdentity: minIdentity,
				ElementType: elementType,
				Genus:       genus,
				Limit:       limit,
			}

			results, err := amr.Query(dir, filters)
			if err != nil {
				return fmt.Errorf("AMR query failed: %w", err)
			}

			rows := amrResultsToOutputRows(results)
			cols := amrColumns()

			// Resolve output format
			resolvedFormat := format
			if resolvedFormat == "" {
				resolvedFormat = cfg.General.DefaultFormat
			}
			resolvedFormat = output.ResolveFormat(resolvedFormat)

			var w io.Writer = cmd.OutOrStdout()
			if outputFile != "" {
				f, err := os.Create(outputFile)
				if err != nil {
					return fmt.Errorf("opening output file: %w", err)
				}
				defer f.Close()
				w = f
			}

			return output.Format(w, rows, cols, resolvedFormat)
		},
	}

	cmd.Flags().StringVar(&species, "species", "", "species to query AMR data for (required)")
	cmd.Flags().StringVar(&elementType, "type", "", "element type: amr (default), stress, virulence, all")
	cmd.Flags().StringVar(&class, "class", "", "filter by drug class (case-insensitive, substring match)")
	cmd.Flags().StringVar(&gene, "gene", "", "filter by gene symbol (supports % wildcards)")
	cmd.Flags().BoolVar(&hqOnly, "hq-only", false, "only include HQ samples (hq_filter=PASS)")
	cmd.Flags().Float64Var(&minCoverage, "min-coverage", 0, "minimum coverage %")
	cmd.Flags().Float64Var(&minIdentity, "min-identity", 0, "minimum identity %")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of results")
	cmd.Flags().StringVar(&format, "format", "", "output format: tsv, csv, json, table, auto")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "write output to file instead of stdout")

	_ = cmd.MarkFlagRequired("species")

	return cmd
}

// amrColumns returns the fixed column order for AMR output.
func amrColumns() []string {
	return []string{
		"sample_accession",
		"gene_symbol",
		"element_type",
		"element_subtype",
		"class",
		"subclass",
		"method",
		"coverage",
		"identity",
		"species",
		"genus",
	}
}

func amrResultsToOutputRows(results []amr.Result) []output.Row {
	rows := make([]output.Row, len(results))
	for i, r := range results {
		rows[i] = output.Row{
			"sample_accession": r.SampleAccession,
			"gene_symbol":      r.GeneSymbol,
			"element_type":     r.ElementType,
			"element_subtype":  r.ElementSubtype,
			"class":            r.Class,
			"subclass":         r.Subclass,
			"method":           r.Method,
			"coverage":         strconv.FormatFloat(r.Coverage, 'f', 2, 64),
			"identity":         strconv.FormatFloat(r.Identity, 'f', 2, 64),
			"species":          r.Species,
			"genus":            r.Genus,
		}
	}
	return rows
}
