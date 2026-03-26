package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/amr"
	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/fetch"
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
		Long: `Query AMRFinderPlus gene hits from Hive-partitioned parquet files.

Data is organized by genus and element type (amr, stress, virulence).
Use 'atb fetch --amr --genus <Genus>' to download data before querying.`,
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

			// Check if AMR data exists for this genus
			amrDir := amrDirForType(dir, genus, elementType)
			if _, statErr := os.Stat(amrDir); statErr != nil {
				if !offerFetchAMR(dir, genus, elementType) {
					return fmt.Errorf("AMR data not found for genus %q — run 'atb fetch --amr --genus %s' to download", genus, genus)
				}
				types := amrTypesForElementType(elementType)
				f := fetch.New(fetch.Config{DataDir: dir})
				fmt.Fprintf(os.Stderr, "Fetching AMR data for genus %q...\n", genus)
				if fetchErr := f.FetchAMRGenus(genus, types, false); fetchErr != nil {
					return fmt.Errorf("fetching AMR data: %w", fetchErr)
				}
			}

			// Optionally build HQ sample set from assembly.parquet
			var sampleSet map[string]struct{}
			if hqOnly {
				assemblyPath := filepath.Join(dir, "assembly.parquet")
				hqRows, hqErr := pq.ReadFiltered[pq.AssemblyRow](assemblyPath, func(r pq.AssemblyRow) bool {
					if r.HQFilter != "PASS" {
						return false
					}
					return strings.EqualFold(pq.GenusFromSpecies(r.SylphSpecies), genus)
				})
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
			}

			results, err := amr.Query(dir, genus, elementType, filters)
			if err != nil {
				return fmt.Errorf("AMR query failed: %w", err)
			}

			// Apply limit
			if limit > 0 && limit < len(results) {
				results = results[:limit]
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

// amrDirForType returns the genus directory for the given element type to check existence.
func amrDirForType(dataDir, genus, elementType string) string {
	var dirName string
	switch strings.ToUpper(elementType) {
	case "STRESS":
		dirName = "stress_by_genus"
	case "VIRULENCE":
		dirName = "virulence_by_genus"
	default:
		dirName = "amr_by_genus"
	}
	return filepath.Join(dataDir, "amr", dirName, "Genus="+genus)
}

// amrTypesForElementType returns the lowercase type strings for FetchAMRGenus.
func amrTypesForElementType(elementType string) []string {
	switch strings.ToLower(elementType) {
	case "stress":
		return []string{"stress"}
	case "virulence":
		return []string{"virulence"}
	case "all":
		return []string{"amr", "stress", "virulence"}
	default:
		return []string{"amr"}
	}
}

// offerFetchAMR prompts the user interactively to fetch missing AMR data.
// Returns true if the user accepted and we should proceed with fetching.
func offerFetchAMR(dataDir, genus, elementType string) bool {
	stat, _ := os.Stdin.Stat()
	if stat.Mode()&os.ModeCharDevice == 0 {
		return false // non-interactive
	}

	types := strings.Join(amrTypesForElementType(elementType), ",")
	fmt.Fprintf(os.Stderr, "\nAMR data not found for genus %q (types: %s).\n", genus, types)
	fmt.Fprintf(os.Stderr, "Download it now? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	answer := strings.TrimSpace(strings.ToLower(input))
	return answer == "y" || answer == "yes"
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
		}
	}
	return rows
}

