package cli

import (
	"fmt"
	"os"
	"sync"

	"github.com/spf13/cobra"

	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/amr"
	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/fetch"
	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/index"
)

func newFetchCmd() *cobra.Command {
	var (
		all      bool
		tables   []string
		force    bool
		parallel int
	)

	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Download ATB parquet metadata tables",
		Long: `Download parquet metadata tables from AllTheBacteria.

By default the core tables are downloaded, which includes amrfinderplus.parquet.

Use --all to download all available tables.
Use --tables to specify exact tables by name.`,
		Example: `  # Download core tables (includes AMR data)
  atb fetch

  # Download all tables including ENA metadata
  atb fetch --all

  # Force re-download
  atb fetch --force

  # Download specific tables
  atb fetch --tables assembly.parquet,amrfinderplus.parquet`,
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

			// Determine which tables to fetch
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

			fmt.Fprintf(os.Stderr, "All tables downloaded successfully.\n\n")

			stderrLog := func(format string, args ...any) {
				fmt.Fprintf(os.Stderr, format+"\n", args...)
			}

			// Build genus partitions for faster AMR queries.
			if err := amr.BuildPartitions(dir, stderrLog); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: AMR partition build failed: %v\n", err)
			}

			// Auto-build the SQLite index after a successful fetch.
			fmt.Fprintf(os.Stderr, "\nBuilding query index...\n")
			if err := index.Build(dir, stderrLog); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: index build failed: %v\n", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "download all available tables")
	cmd.Flags().StringSliceVar(&tables, "tables", nil, "specific tables to download (comma-separated names)")
	cmd.Flags().BoolVar(&force, "force", false, "re-download even if table already exists")
	cmd.Flags().IntVar(&parallel, "parallel", 0, "parallel downloads (default from config)")

	return cmd
}
