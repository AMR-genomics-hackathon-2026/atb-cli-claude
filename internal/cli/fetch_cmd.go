package cli

import (
	"fmt"
	"os"
	"sync"

	"github.com/spf13/cobra"

	"github.com/immem-hackathon-2025/atb-cli/internal/fetch"
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

By default only the five core tables are downloaded:
  assembly, assembly_stats, checkm2, sylph, run

Use --all to download all ten available tables.
Use --tables to specify exact tables by name.`,
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

			f := fetch.New(fetch.Config{
				DataDir:  dir,
				Parallel: par,
			})

			sem := make(chan struct{}, par)
			var mu sync.Mutex
			var wg sync.WaitGroup
			var failed int

			for _, name := range targets {
				wg.Add(1)
				url, _ := fetch.URLForTable(name)
				go func(tableName, tableURL string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					fmt.Fprintf(os.Stderr, "  fetching %s\n", tableName)
					if err := f.FetchTable(tableName, tableURL, force); err != nil {
						fmt.Fprintf(os.Stderr, "  error: %s: %v\n", tableName, err)
						mu.Lock()
						failed++
						mu.Unlock()
					} else {
						fmt.Fprintf(os.Stderr, "  done: %s\n", tableName)
					}
				}(name, url)
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

	return cmd
}
