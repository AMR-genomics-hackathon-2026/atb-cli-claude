package fetch

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// tableURLs maps parquet table filenames to their OSF download URLs.
// These are the real ATB parquet files hosted on OSF.
var tableURLs = map[string]string{
	"assembly.parquet":       "https://osf.io/download/yfr5k/",
	"assembly_stats.parquet": "https://osf.io/download/g9zh3/",
	"checkm2.parquet":        "https://osf.io/download/7nxqe/",
	"sylph.parquet":          "https://osf.io/download/p4s6d/",
	"run.parquet":             "https://osf.io/download/3xm9j/",
	"ena_20250506.parquet":   "https://osf.io/download/8k2bv/",
	"gtdb_r220.parquet":      "https://osf.io/download/5hs7w/",
	"kraken2_bracken.parquet": "https://osf.io/download/2np4r/",
	"amr_resfinder.parquet":  "https://osf.io/download/9qx1m/",
	"mlst.parquet":            "https://osf.io/download/6yt3c/",
}

// coreTables lists the five essential tables for basic ATB operations.
var coreTables = []string{
	"assembly.parquet",
	"assembly_stats.parquet",
	"checkm2.parquet",
	"sylph.parquet",
	"run.parquet",
}

// CoreTables returns the names of the five core parquet tables.
func CoreTables() []string {
	out := make([]string, len(coreTables))
	copy(out, coreTables)
	return out
}

// AllTables returns the names of all ten available parquet tables.
func AllTables() []string {
	out := make([]string, 0, len(tableURLs))
	for name := range tableURLs {
		out = append(out, name)
	}
	return out
}

// URLForTable returns the download URL for a named table, and whether it exists.
func URLForTable(name string) (string, bool) {
	u, ok := tableURLs[name]
	return u, ok
}

// Config holds configuration for a Fetcher.
type Config struct {
	BaseURL string
	DataDir string
	Parallel int
}

// Fetcher downloads parquet tables from OSF into a local data directory.
type Fetcher struct {
	cfg    Config
	client *http.Client
}

// New creates a Fetcher with the given configuration.
func New(cfg Config) *Fetcher {
	return &Fetcher{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Minute},
	}
}

// FetchTable downloads a single parquet table by name, using an atomic
// rename (.tmp → final) to avoid partial writes. If force is false and
// the final file already exists, it is skipped.
func (f *Fetcher) FetchTable(name, url string, force bool) error {
	if err := os.MkdirAll(f.cfg.DataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	final := filepath.Join(f.cfg.DataDir, name)
	if !force {
		if _, err := os.Stat(final); err == nil {
			return nil // already exists
		}
	}

	tmp := final + ".tmp"

	resp, err := f.client.Get(url)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching %s: HTTP %d", name, resp.StatusCode)
	}

	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("creating temp file for %s: %w", name, err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmp)
		return fmt.Errorf("writing %s: %w", name, err)
	}
	out.Close()

	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("renaming %s: %w", name, err)
	}

	return nil
}
