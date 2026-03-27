package fetch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// amrBaseURL is the root of Hive-partitioned AMR parquet files on GitHub.
const amrBaseURL = "https://raw.githubusercontent.com/immem-hackathon-2025/atb-amr-shiny/main/data"

// amrAPIBase is the GitHub Contents API for listing genera directories.
const amrAPIBase = "https://api.github.com/repos/immem-hackathon-2025/atb-amr-shiny/contents/data"

// AllAMRTypes lists every element-type directory in the AMR dataset.
var AllAMRTypes = []string{"amr", "stress", "virulence"}

// tableURLs maps parquet table filenames to their OSF download URLs.
// Source: https://osf.io/h7wzy/files/osfstorage
// Path: Aggregated/Latest_2025-05/atb.metadata.202505.parquet/
var tableURLs = map[string]string{
	"assembly.parquet":       "https://osf.io/download/4ku2n/",
	"assembly_stats.parquet": "https://osf.io/download/69c51e86801fecc5d6146396/",
	"checkm2.parquet":        "https://osf.io/download/69c51e93cba7111bb21d27f2/",
	"sylph.parquet":          "https://osf.io/download/69c51f90cba7111bb21d2905/",
	"run.parquet":            "https://osf.io/download/69c51f68376eb79a651d2d85/",
	"ena_20250506.parquet":   "https://osf.io/download/69c51f3ab4f99c692d54cf73/",
	"ena_20240801.parquet":   "https://osf.io/download/69c51f002e72f67915145d0e/",
	"ena_20240625.parquet":   "https://osf.io/download/69c51ec99ce80b96ac54cd08/",
	"ena_202505_used.parquet": "https://osf.io/download/69c51f475eedad376954ce7b/",
	"ena_661k.parquet":       "https://osf.io/download/69c51f57376eb79a651d2d83/",
	"mlst.parquet":           "https://osf.io/download/69c66d33fa3d973d94254f46/",
}

// coreTables lists the essential tables for basic ATB operations.
var coreTables = []string{
	"assembly.parquet",
	"assembly_stats.parquet",
	"checkm2.parquet",
	"sylph.parquet",
	"run.parquet",
	"mlst.parquet",
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

// FetchAMRGenus downloads AMR parquet files for a specific genus from GitHub.
// elementTypes may contain any combination of "amr", "stress", "virulence".
// For each type it tries data_0.parquet, data_1.parquet, ... until a 404 is received.
// Files are written atomically using a .tmp rename. Existing files are skipped unless force is true.
func (f *Fetcher) FetchAMRGenus(genus string, elementTypes []string, force bool) error {
	for _, et := range elementTypes {
		dirName := et + "_by_genus"
		// The '=' in the Hive partition dir name must be URL-encoded in GitHub raw URLs.
		partitionSegment := "Genus%3D" + url.PathEscape(genus)
		localDir := filepath.Join(f.cfg.DataDir, "amr", dirName, "Genus="+genus)

		if err := os.MkdirAll(localDir, 0755); err != nil {
			return fmt.Errorf("create amr dir %q: %w", localDir, err)
		}

		for n := 0; ; n++ {
			filename := fmt.Sprintf("data_%d.parquet", n)
			finalPath := filepath.Join(localDir, filename)

			if !force {
				if _, err := os.Stat(finalPath); err == nil {
					continue // already present; keep probing for higher-numbered parts
				}
			}

			rawURL := fmt.Sprintf("%s/%s/%s/%s", amrBaseURL, dirName, partitionSegment, filename)

			resp, err := f.client.Get(rawURL)
			if err != nil {
				return fmt.Errorf("fetching %s: %w", rawURL, err)
			}

			if resp.StatusCode == http.StatusNotFound {
				resp.Body.Close()
				break // no more parts for this type/genus
			}

			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return fmt.Errorf("fetching %s: HTTP %d", rawURL, resp.StatusCode)
			}

			tmp := finalPath + ".tmp"
			out, err := os.Create(tmp)
			if err != nil {
				resp.Body.Close()
				return fmt.Errorf("creating temp file for %s: %w", filename, err)
			}

			if _, err := io.Copy(out, resp.Body); err != nil {
				out.Close()
				resp.Body.Close()
				os.Remove(tmp)
				return fmt.Errorf("writing %s: %w", filename, err)
			}
			out.Close()
			resp.Body.Close()

			if err := os.Rename(tmp, finalPath); err != nil {
				os.Remove(tmp)
				return fmt.Errorf("renaming %s: %w", filename, err)
			}
		}
	}

	return nil
}

// ghEntry is the minimal JSON shape returned by the GitHub Contents API.
type ghEntry struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ListAMRGenera queries the GitHub API to discover all genus directories
// under a given element type (e.g. "amr_by_genus"). Directory names have the
// form "Genus=Escherichia"; this function returns just the genus part.
func (f *Fetcher) ListAMRGenera(elementType string) ([]string, error) {
	dirName := elementType + "_by_genus"
	apiURL := fmt.Sprintf("%s/%s", amrAPIBase, dirName)

	var genera []string
	for page := 1; ; page++ {
		reqURL := fmt.Sprintf("%s?per_page=100&page=%d", apiURL, page)
		resp, err := f.client.Get(reqURL)
		if err != nil {
			return nil, fmt.Errorf("listing genera for %s: %w", elementType, err)
		}

		var entries []ghEntry
		if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decoding genera list for %s: %w", elementType, err)
		}
		resp.Body.Close()

		for _, e := range entries {
			if e.Type != "dir" {
				continue
			}
			if g, ok := strings.CutPrefix(e.Name, "Genus="); ok {
				genera = append(genera, g)
			}
		}

		if len(entries) < 100 {
			break
		}
	}
	return genera, nil
}

// FetchAllAMR discovers every genus for each element type and downloads all
// partitions. It calls logf to report progress. Existing files are skipped
// unless force is true.
func (f *Fetcher) FetchAllAMR(types []string, force bool, logf func(string, ...any)) error {
	for _, et := range types {
		logf("Listing genera for %s...", et)
		genera, err := f.ListAMRGenera(et)
		if err != nil {
			return err
		}
		logf("Found %d genera for %s", len(genera), et)

		for i, genus := range genera {
			logf("  [%d/%d] fetching %s/%s", i+1, len(genera), et, genus)
			if err := f.FetchAMRGenus(genus, []string{et}, force); err != nil {
				logf("  warning: %s/%s: %v", et, genus, err)
			}
		}
	}
	return nil
}
