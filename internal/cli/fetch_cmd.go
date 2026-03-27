package cli

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/fetch"
	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/index"
)

func newFetchCmd() *cobra.Command {
	var (
		all      bool
		tables   []string
		force    bool
		parallel int
		fetchAMR bool
		genera   []string
		amrTypes []string
		noAMR    bool
	)

	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Download ATB parquet metadata tables",
		Long: `Download parquet metadata tables and AMR data from AllTheBacteria.

By default the core tables plus all AMR data (amr, stress, virulence
for every genus) are downloaded.

Use --all to download all ten available tables plus AMR data.
Use --no-amr to skip AMR data download.
Use --tables to specify exact tables by name.`,
		Example: `  # Download core tables + all AMR data
  atb fetch

  # Download all tables including ENA metadata + AMR data
  atb fetch --all

  # Download core tables only (skip AMR)
  atb fetch --no-amr

  # Download AMR data for a specific genus only
  atb fetch --amr --genus Escherichia

  # Download AMR + stress + virulence data for a genus
  atb fetch --amr --genus Escherichia --amr-types amr,stress,virulence

  # Force re-download
  atb fetch --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := dataDir
			if dir == "" {
				dir = cfg.General.DataDir
			}

			par := parallel
			if par == 0 {
				par = cfg.Fetch.Parallel
			}
			if par <= 0 {
				par = 4
			}

			f := fetch.New(fetch.Config{
				DataDir:  dir,
				Parallel: par,
			})

			// AMR fetch path
			if fetchAMR {
				if len(genera) == 0 {
					return fmt.Errorf("--genus is required when using --amr")
				}
				types := amrTypes
				if len(types) == 0 {
					types = []string{"amr"}
				}
				// Normalise to lowercase (directory names are lowercase)
				for i, t := range types {
					types[i] = strings.ToLower(t)
				}
				for _, genus := range genera {
					fmt.Fprintf(os.Stderr, "Fetching AMR data for genus %q (types: %s)\n", genus, strings.Join(types, ","))
					if err := f.FetchAMRGenus(genus, types, force); err != nil {
						return fmt.Errorf("fetching AMR for %q: %w", genus, err)
					}
					fmt.Fprintf(os.Stderr, "  done: %s\n", genus)
				}
				return nil
			}

			// Regular parquet table fetch
			var targets []string
			switch {
			case len(tables) > 0:
				targets = tables
			case all:
				targets = fetch.AllTables()
			default:
				targets = fetch.CoreTables()
			}

			// Validate table names
			for _, name := range targets {
				if _, ok := fetch.URLForTable(name); !ok {
					return fmt.Errorf("unknown table %q; run 'atb fetch --all' to see available tables", name)
				}
			}

			fmt.Fprintf(os.Stderr, "Fetching %d table(s) into %s\n", len(targets), dir)

			sem := make(chan struct{}, par)
			var mu sync.Mutex
			var wg sync.WaitGroup
			var failed int

			for _, name := range targets {
				wg.Add(1)
				tableURL, _ := fetch.URLForTable(name)
				go func(tableName, tURL string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					fmt.Fprintf(os.Stderr, "  fetching %s\n", tableName)
					if err := f.FetchTable(tableName, tURL, force); err != nil {
						fmt.Fprintf(os.Stderr, "  error: %s: %v\n", tableName, err)
						mu.Lock()
						failed++
						mu.Unlock()
					} else {
						fmt.Fprintf(os.Stderr, "  done: %s\n", tableName)
					}
				}(name, tableURL)
			}

			wg.Wait()

			if failed > 0 {
				return fmt.Errorf("%d table(s) failed to download", failed)
			}

			fmt.Fprintf(os.Stderr, "All tables downloaded successfully.\n")

			// Fetch all AMR data unless --no-amr is set
			if !noAMR {
				fmt.Fprintf(os.Stderr, "Fetching all AMR data (amr, stress, virulence)...\n")
				logf := func(format string, args ...any) {
					fmt.Fprintf(os.Stderr, format+"\n", args...)
				}
				if err := f.FetchAllAMR(fetch.AllAMRTypes, force, logf); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: AMR fetch encountered errors: %v\n", err)
				} else {
					fmt.Fprintf(os.Stderr, "All AMR data downloaded successfully.\n")
				}
			}

			// Auto-build the SQLite index after a successful fetch.
			fmt.Fprintf(os.Stderr, "Building query index...\n")
			stderrLog := func(format string, args ...any) {
				fmt.Fprintf(os.Stderr, format+"\n", args...)
			}
			if err := index.Build(dir, stderrLog); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: index build failed: %v\n", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "download all ten available tables")
	cmd.Flags().StringSliceVar(&tables, "tables", nil, "specific tables to download (comma-separated names)")
	cmd.Flags().BoolVar(&force, "force", false, "re-download even if table already exists")
	cmd.Flags().IntVar(&parallel, "parallel", 0, "parallel downloads (default from config)")
	cmd.Flags().BoolVar(&noAMR, "no-amr", false, "skip AMR data download")
	cmd.Flags().BoolVar(&fetchAMR, "amr", false, "fetch AMR gene data from GitHub (specific genus)")
	cmd.Flags().StringSliceVar(&genera, "genus", nil, "genus (or genera) to fetch AMR data for (comma-separated)")
	cmd.Flags().StringSliceVar(&amrTypes, "amr-types", nil, "AMR types to fetch: amr,stress,virulence (default: amr)")

	return cmd
}
