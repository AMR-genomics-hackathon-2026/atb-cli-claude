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
		Short: "Query AMR gene data",
		Long: `Query AMRFinderPlus gene hits from the merged amrfinderplus.parquet file.

Use --species to filter by one or more species (comma-separated).
When --species is omitted, all genera are scanned (requires --gene or --class).

Run 'atb fetch' to download the data before querying.`,
		Example: `  # Get AMR gene hits for E. coli (HQ only)
  atb amr --species "Escherichia coli" --hq-only --limit 100

  # Filter by drug class
  atb amr --species "Escherichia coli" --class "BETA-LACTAM"

  # Search for beta-lactamase genes in E. coli
  atb amr --species "Escherichia coli" --gene "bla%"

  # Compare beta-lactam resistance across species
  atb amr --species "Escherichia coli,Klebsiella pneumoniae" --class "BETA-LACTAM"

  # Find a gene across ALL genera (no species filter)
  atb amr --gene "blaCTX-M-15" --limit 100

  # Query stress response genes
  atb amr --species "Escherichia coli" --type stress

  # Query all element types (AMR + stress + virulence)
  atb amr --species "Klebsiella pneumoniae" --type all --hq-only`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}

			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := dataDir
			if dir == "" {
				dir = cfg.General.DataDir
			}

			// Parse species into genera (supports comma-separated)
			var genera []string
			if species != "" {
				for _, sp := range strings.Split(species, ",") {
					sp = strings.TrimSpace(sp)
					if sp == "" {
						continue
					}
					g := pq.GenusFromSpecies(sp)
					if g == "" {
						return fmt.Errorf("could not derive genus from species %q", sp)
					}
					genera = append(genera, g)
				}
			}

			// Require either --species or --gene/--class to avoid accidental full scans
			if len(genera) == 0 && gene == "" && class == "" {
				return fmt.Errorf("--species is required (or use --gene/--class to search across all genera)")
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
				generaSet := make(map[string]bool, len(genera))
				for _, g := range genera {
					generaSet[strings.ToLower(g)] = true
				}
				hqRows, hqErr := pq.ReadStreamFiltered[pq.AssemblyRow](assemblyPath, func(r pq.AssemblyRow) bool {
					if r.HQFilter != "PASS" {
						return false
					}
					if len(generaSet) > 0 {
						return generaSet[strings.ToLower(pq.GenusFromSpecies(r.SylphSpecies))]
					}
					return true
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
				Genera:      genera,
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

	cmd.Flags().StringVar(&species, "species", "", "species to query (comma-separated for multiple)")
	cmd.Flags().StringVar(&elementType, "type", "", "element type: amr (default), stress, virulence, all")
	cmd.Flags().StringVar(&class, "class", "", "filter by drug class (case-insensitive, substring match)")
	cmd.Flags().StringVar(&gene, "gene", "", "filter by gene symbol (supports % wildcards)")
	cmd.Flags().BoolVar(&hqOnly, "hq-only", false, "only include HQ samples (hq_filter=PASS)")
	cmd.Flags().Float64Var(&minCoverage, "min-coverage", 0, "minimum coverage %")
	cmd.Flags().Float64Var(&minIdentity, "min-identity", 0, "minimum identity %")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of results")
	cmd.Flags().StringVar(&format, "format", "", "output format: tsv, csv, json, table, auto")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "write output to file instead of stdout")

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
