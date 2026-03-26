package cli

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	pq "github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/parquet"
	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/output"
	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/query"
	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/suggest"
)

func newQueryCmd() *cobra.Command {
	var (
		filterFile string
		columns    []string
		sortBy     string
		sortDesc   bool
		limit      int
		offset     int
		format     string
		outputFile string

		// filter flags
		species            string
		speciesLike        string
		genus              string
		samples            []string
		sampleFile         string
		hqOnly             bool
		minCompleteness    float64
		maxContamination   float64
		minN50             int64
		dataset            string
		hasAssembly        bool
		country            string
		platform           string
		collectionDateFrom string
		collectionDateTo   string
	)

	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query the ATB database",
		Long:  "Query bacterial genome metadata from local parquet tables.",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			// Build filters - start from TOML file if provided
			filters := query.Filters{}
			outCfg := query.OutputConfig{}

			if filterFile != "" {
				ff, err := query.LoadFilterFile(filterFile)
				if err != nil {
					return fmt.Errorf("loading filter file: %w", err)
				}
				filters = ff.Filter
				outCfg = ff.Output
			}

			// CLI flags override TOML values
			if cmd.Flags().Changed("species") {
				filters.Species = species
			}
			if cmd.Flags().Changed("species-like") {
				filters.SpeciesLike = speciesLike
			}
			if cmd.Flags().Changed("genus") {
				filters.Genus = genus
			}
			if cmd.Flags().Changed("samples") {
				filters.Samples = samples
			}
			if cmd.Flags().Changed("sample-file") {
				filters.SampleFile = sampleFile
			}
			if cmd.Flags().Changed("hq-only") {
				filters.HQOnly = hqOnly
			}
			if cmd.Flags().Changed("min-completeness") {
				filters.MinCompleteness = minCompleteness
			}
			if cmd.Flags().Changed("max-contamination") {
				filters.MaxContamination = maxContamination
			}
			if cmd.Flags().Changed("min-n50") {
				filters.MinN50 = minN50
			}
			if cmd.Flags().Changed("dataset") {
				filters.Dataset = dataset
			}
			if cmd.Flags().Changed("has-assembly") {
				filters.HasAssembly = hasAssembly
			}
			if cmd.Flags().Changed("country") {
				filters.Country = country
			}
			if cmd.Flags().Changed("platform") {
				filters.Platform = platform
			}
			if cmd.Flags().Changed("collection-date-from") {
				filters.CollectionDateFrom = collectionDateFrom
			}
			if cmd.Flags().Changed("collection-date-to") {
				filters.CollectionDateTo = collectionDateTo
			}

			// Load sample file if specified
			if filters.SampleFile != "" {
				if err := filters.LoadSampleFile(); err != nil {
					return fmt.Errorf("loading sample file: %w", err)
				}
			}

			// Output config overrides
			if cmd.Flags().Changed("columns") {
				outCfg.Columns = columns
			}
			if cmd.Flags().Changed("sort-by") {
				outCfg.SortBy = sortBy
			}
			if cmd.Flags().Changed("sort-desc") {
				outCfg.SortDesc = sortDesc
			}
			if cmd.Flags().Changed("limit") {
				outCfg.Limit = limit
			}
			if cmd.Flags().Changed("offset") {
				outCfg.Offset = offset
			}
			if cmd.Flags().Changed("format") {
				outCfg.Format = format
			}
			if cmd.Flags().Changed("output") {
				outCfg.Output = outputFile
			}

			results, err := query.Execute(dir, filters, outCfg.Columns)
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}

			// Suggest species if no results and a species filter was set
			if len(results) == 0 && filters.Species != "" {
				assemblies, aErr := pq.ReadAll[pq.AssemblyRow](filepath.Join(dir, "assembly.parquet"))
				if aErr == nil {
					speciesSet := make(map[string]struct{}, len(assemblies))
					for _, a := range assemblies {
						if a.SylphSpecies != "" {
							speciesSet[a.SylphSpecies] = struct{}{}
						}
					}
					allSpecies := make([]string, 0, len(speciesSet))
					for s := range speciesSet {
						allSpecies = append(allSpecies, s)
					}
					suggestions := suggest.Suggest(filters.Species, allSpecies, 5)
					if len(suggestions) > 0 {
						fmt.Fprintf(os.Stderr, "No results for species %q. Did you mean:\n", filters.Species)
						for _, s := range suggestions {
							fmt.Fprintf(os.Stderr, "  %s\n", s)
						}
					}
				}
			}

			// Sort if requested
			if outCfg.SortBy != "" {
				sortKey := outCfg.SortBy
				desc := outCfg.SortDesc
				sort.SliceStable(results, func(i, j int) bool {
					vi, vj := results[i][sortKey], results[j][sortKey]
					if desc {
						return vi > vj
					}
					return vi < vj
				})
			}

			// Apply offset
			if outCfg.Offset > 0 {
				if outCfg.Offset >= len(results) {
					results = nil
				} else {
					results = results[outCfg.Offset:]
				}
			}

			// Apply limit
			if outCfg.Limit > 0 && outCfg.Limit < len(results) {
				results = results[:outCfg.Limit]
			}

			// Determine columns
			cols := outCfg.Columns
			if len(cols) == 0 {
				outRows := queryToOutputRows(results)
				cols = output.InferColumns(outRows)
			}

			// Resolve format
			resolvedFormat := outCfg.Format
			if resolvedFormat == "" {
				resolvedFormat = cfg.General.DefaultFormat
			}
			resolvedFormat = output.ResolveFormat(resolvedFormat)

			// Write output
			outRows := queryToOutputRows(results)

			var w io.Writer = cmd.OutOrStdout()
			if outCfg.Output != "" {
				f, err := os.Create(outCfg.Output)
				if err != nil {
					return fmt.Errorf("opening output file: %w", err)
				}
				defer f.Close()
				w = f
			}

			return output.Format(w, outRows, cols, resolvedFormat)
		},
	}

	// Filter flags
	cmd.Flags().StringVar(&filterFile, "filter", "", "TOML filter file")
	cmd.Flags().StringVar(&species, "species", "", "exact species name (case-insensitive)")
	cmd.Flags().StringVar(&speciesLike, "species-like", "", "wildcard species match (use % for wildcards)")
	cmd.Flags().StringVar(&genus, "genus", "", "filter by genus")
	cmd.Flags().StringSliceVar(&samples, "samples", nil, "comma-separated sample accessions")
	cmd.Flags().StringVar(&sampleFile, "sample-file", "", "file with one sample accession per line")
	cmd.Flags().BoolVar(&hqOnly, "hq-only", false, "only return high-quality genomes (HQFilter=PASS)")
	cmd.Flags().Float64Var(&minCompleteness, "min-completeness", 0, "minimum CheckM2 completeness")
	cmd.Flags().Float64Var(&maxContamination, "max-contamination", 0, "maximum CheckM2 contamination")
	cmd.Flags().Int64Var(&minN50, "min-n50", 0, "minimum assembly N50")
	cmd.Flags().StringVar(&dataset, "dataset", "", "filter by dataset name")
	cmd.Flags().BoolVar(&hasAssembly, "has-assembly", false, "only samples with assembly FASTA on OSF")
	cmd.Flags().StringVar(&country, "country", "", "filter by country of origin (ENA metadata)")
	cmd.Flags().StringVar(&platform, "platform", "", "filter by sequencing platform (ENA metadata)")
	cmd.Flags().StringVar(&collectionDateFrom, "collection-date-from", "", "collection date lower bound (YYYY-MM-DD)")
	cmd.Flags().StringVar(&collectionDateTo, "collection-date-to", "", "collection date upper bound (YYYY-MM-DD)")

	// Output flags
	cmd.Flags().StringSliceVar(&columns, "columns", nil, "columns to include in output")
	cmd.Flags().StringVar(&sortBy, "sort-by", "", "column to sort by")
	cmd.Flags().BoolVar(&sortDesc, "sort-desc", false, "sort in descending order")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of rows to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "number of rows to skip")
	cmd.Flags().StringVar(&format, "format", "", "output format: tsv, csv, json, table, auto")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "write output to file instead of stdout")

	return cmd
}

// queryToOutputRows converts []query.ResultRow to []output.Row.
// Both types are map[string]string so this is a simple cast loop.
func queryToOutputRows(rows []query.ResultRow) []output.Row {
	out := make([]output.Row, len(rows))
	for i, r := range rows {
		out[i] = output.Row(r)
	}
	return out
}

// parseCSVURLs reads a CSV or TSV file and extracts the aws_url column.
// Falls back to sample_accession if aws_url is absent.
func parseCSVURLs(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Detect separator from extension
	sep := ','
	if strings.HasSuffix(strings.ToLower(path), ".tsv") {
		sep = '\t'
	}

	r := csv.NewReader(f)
	r.Comma = rune(sep)

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}

	awsIdx := -1
	sampleIdx := -1
	for i, h := range header {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "aws_url":
			awsIdx = i
		case "sample_accession":
			sampleIdx = i
		}
	}

	if awsIdx == -1 && sampleIdx == -1 {
		return nil, fmt.Errorf("CSV/TSV file must contain an aws_url or sample_accession column")
	}

	var urls []string
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		col := awsIdx
		if col == -1 {
			col = sampleIdx
		}
		if col < len(record) {
			v := strings.TrimSpace(record[col])
			if v != "" {
				urls = append(urls, v)
			}
		}
	}

	return urls, nil
}
