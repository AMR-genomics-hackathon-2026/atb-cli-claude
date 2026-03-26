package cli

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/fetch"
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
	)

	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Download ATB parquet metadata tables",
		Long: `Download parquet metadata tables from AllTheBacteria.

By default only the five core tables are downloaded:
  assembly, assembly_stats, checkm2, sylph, run

Use --all to download all ten available tables.
Use --tables to specify exact tables by name.`,
		Example: `  # Download core metadata tables (~540 MB)
  atb fetch

  # Download all tables including ENA metadata (~3 GB)
  atb fetch --all

  # Download AMR data for a specific genus
  atb fetch --amr --genus Escherichia

  # Download AMR + stress + virulence data
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
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "download all ten available tables")
	cmd.Flags().StringSliceVar(&tables, "tables", nil, "specific tables to download (comma-separated names)")
	cmd.Flags().BoolVar(&force, "force", false, "re-download even if table already exists")
	cmd.Flags().IntVar(&parallel, "parallel", 0, "parallel downloads (default from config)")
	cmd.Flags().BoolVar(&fetchAMR, "amr", false, "fetch AMR gene data from GitHub")
	cmd.Flags().StringSliceVar(&genera, "genus", nil, "genus (or genera) to fetch AMR data for (comma-separated)")
	cmd.Flags().StringSliceVar(&amrTypes, "amr-types", nil, "AMR types to fetch: amr,stress,virulence (default: amr)")

	return cmd
}
