package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/fetch"
)

// ensureDatabase checks if the parquet database exists at dir.
// If not, it shows a friendly message and offers to download it.
// Returns nil if database is available (either existed or was downloaded).
// Returns error if user declines or download fails.
func ensureDatabase(dir string) error {
	assemblyPath := filepath.Join(dir, "assembly.parquet")
	if _, err := os.Stat(assemblyPath); err == nil {
		return nil // database exists
	}

	entries, dirErr := os.ReadDir(dir)
	hasParquet := false
	if dirErr == nil {
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".parquet" {
				hasParquet = true
				break
			}
		}
	}

	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  ╭─────────────────────────────────────────────────────╮\n")
	fmt.Fprintf(os.Stderr, "  │          ATB Database Not Found                     │\n")
	fmt.Fprintf(os.Stderr, "  ╰─────────────────────────────────────────────────────╯\n")
	fmt.Fprintf(os.Stderr, "\n")

	if dirErr != nil {
		fmt.Fprintf(os.Stderr, "  Data directory does not exist:\n")
	} else if !hasParquet {
		fmt.Fprintf(os.Stderr, "  No parquet files found in:\n")
	} else {
		fmt.Fprintf(os.Stderr, "  Missing required file (assembly.parquet) in:\n")
	}
	fmt.Fprintf(os.Stderr, "    %s\n\n", dir)

	fmt.Fprintf(os.Stderr, "  The AllTheBacteria database is required to run queries.\n")
	fmt.Fprintf(os.Stderr, "  Core tables are ~540 MB (5 files).\n\n")

	fmt.Fprintf(os.Stderr, "  Options:\n")
	fmt.Fprintf(os.Stderr, "    [d] Download core tables now to %s\n", dir)
	fmt.Fprintf(os.Stderr, "    [a] Download ALL tables (~3 GB) to %s\n", dir)
	fmt.Fprintf(os.Stderr, "    [p] Specify a different path\n")
	fmt.Fprintf(os.Stderr, "    [q] Quit\n\n")

	// Check if stdin is a terminal (can prompt)
	stat, _ := os.Stdin.Stat()
	if stat.Mode()&os.ModeCharDevice == 0 {
		// Not a terminal (piped input), can't prompt
		return fmt.Errorf("database not found at %s\nRun 'atb fetch' to download, or set --data-dir", dir)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Fprintf(os.Stderr, "  Choice [d/a/p/q]: ")
	input, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(strings.ToLower(input))

	switch choice {
	case "d":
		return doFetch(dir, false)
	case "a":
		return doFetch(dir, true)
	case "p":
		fmt.Fprintf(os.Stderr, "  Enter path: ")
		pathInput, _ := reader.ReadString('\n')
		newDir := strings.TrimSpace(pathInput)
		if newDir == "" {
			return fmt.Errorf("no path provided")
		}
		fmt.Fprintf(os.Stderr, "  Downloading to %s ...\n", newDir)
		return doFetch(newDir, false)
	default:
		return fmt.Errorf("database required — run 'atb fetch' to download")
	}
}

func doFetch(dir string, all bool) error {
	fmt.Fprintf(os.Stderr, "\n  Downloading to %s ...\n\n", dir)

	var tables []string
	if all {
		tables = fetch.AllTables()
	} else {
		tables = fetch.CoreTables()
	}

	f := fetch.New(fetch.Config{
		DataDir:  dir,
		Parallel: 4,
	})

	var failed int
	for _, name := range tables {
		url, ok := fetch.URLForTable(name)
		if !ok {
			continue
		}
		fmt.Fprintf(os.Stderr, "  Fetching %s ... ", name)
		if err := f.FetchTable(name, url, false); err != nil {
			fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
			failed++
		} else {
			fmt.Fprintf(os.Stderr, "OK\n")
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d table(s) failed to download", failed)
	}

	fmt.Fprintf(os.Stderr, "\n  Database ready.\n\n")
	return nil
}
