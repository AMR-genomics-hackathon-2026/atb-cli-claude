# atb sketch — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `atb sketch` subcommand to find closest ATB genomes for a user's FASTA input via sketch distances.

**Architecture:** Shell out to the `sketchlib` Rust binary for sketching and distance calculation. New `internal/sketch` package wraps sketchlib CLI calls and parses output. New `internal/cli/sketch_cmd.go` provides `fetch`, `query`, and `info` subcommands. Results enriched with ATB metadata from the existing SQLite index.

**Tech Stack:** Go (CLI), sketchlib Rust binary (external dependency), existing download/index/output packages.

**Key sketchlib CLI facts (verified):**
- `sketchlib sketch -o <prefix> --k-vals 17,29 file.fasta` — creates `<prefix>.skm` + `<prefix>.skd`
- `sketchlib sketch -o <prefix> --k-vals 17 -f filelist.txt` — file list format: `name\tpath` per line
- `sketchlib dist <ref_prefix> <query_prefix> -k 17 --ani` — output: `ref_name\tquery_name\tani_value` per line
- `sketchlib dist` takes **prefixes** (no `.skm` extension), not file paths
- `--knn` only works in self-distance mode (no query db), NOT in ref-vs-query mode
- `sketchlib info <prefix>.skm` — output: key=value lines including `kmers=[17, 29]`
- `sketchlib info <prefix>.skm --sample-info` — tab-separated: `Name\tSequence length\t...`

---

### Task 1: Create `internal/sketch/sketch.go` — FindBinary and Info

**Files:**
- Create: `internal/sketch/sketch.go`
- Create: `internal/sketch/sketch_test.go`

- [ ] **Step 1: Write tests for FindBinary and ParseInfo**

Create `internal/sketch/sketch_test.go`:

```go
package sketch

import (
	"strings"
	"testing"
)

func TestParseInfo(t *testing.T) {
	output := `sketch_version=0.2.4
sequence_type=DNA
sketch_size=1024
n_samples=3218671
kmers=[17, 29]
inverted=false`

	info, err := ParseInfo(strings.NewReader(output))
	if err != nil {
		t.Fatalf("ParseInfo: %v", err)
	}
	if info.Samples != 3218671 {
		t.Errorf("Samples = %d, want 3218671", info.Samples)
	}
	if len(info.KmerSizes) != 2 || info.KmerSizes[0] != 17 || info.KmerSizes[1] != 29 {
		t.Errorf("KmerSizes = %v, want [17, 29]", info.KmerSizes)
	}
	if info.SketchSize != 1024 {
		t.Errorf("SketchSize = %d, want 1024", info.SketchSize)
	}
}

func TestParseInfoMissingFields(t *testing.T) {
	output := `sketch_version=0.2.4
sequence_type=DNA`

	info, err := ParseInfo(strings.NewReader(output))
	if err != nil {
		t.Fatalf("ParseInfo: %v", err)
	}
	if info.Samples != 0 {
		t.Errorf("Samples = %d, want 0", info.Samples)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/sketch/... -v`
Expected: FAIL — package doesn't exist yet.

- [ ] **Step 3: Implement FindBinary, ParseInfo, and DatabaseInfo**

Create `internal/sketch/sketch.go`:

```go
package sketch

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

const BinaryName = "sketchlib"

type DatabaseInfo struct {
	Samples    int
	KmerSizes  []int
	SketchSize int
}

// FindBinary returns the path to sketchlib or an error with install instructions.
func FindBinary() (string, error) {
	path, err := exec.LookPath(BinaryName)
	if err != nil {
		return "", fmt.Errorf("sketchlib not found in PATH.\nInstall with: cargo install sketchlib\n         or: conda install -c bioconda sketchlib")
	}
	return path, nil
}

// Info runs sketchlib info on a .skm file and returns parsed metadata.
func Info(skmPath string) (*DatabaseInfo, error) {
	bin, err := FindBinary()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, "info", skmPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("sketchlib info: %w", err)
	}
	return ParseInfo(strings.NewReader(string(out)))
}

// ParseInfo parses the key=value output from sketchlib info.
func ParseInfo(r io.Reader) (*DatabaseInfo, error) {
	info := &DatabaseInfo{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		switch key {
		case "n_samples":
			info.Samples, _ = strconv.Atoi(val)
		case "sketch_size":
			info.SketchSize, _ = strconv.Atoi(val)
		case "kmers":
			// Parse "[17, 29]"
			val = strings.Trim(val, "[] ")
			for _, s := range strings.Split(val, ",") {
				s = strings.TrimSpace(s)
				if n, err := strconv.Atoi(s); err == nil {
					info.KmerSizes = append(info.KmerSizes, n)
				}
			}
		}
	}
	return info, scanner.Err()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/sketch/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/sketch/sketch.go internal/sketch/sketch_test.go
git commit -m "feat(sketch): add sketchlib binary detection and info parsing"
```

---

### Task 2: Add SketchQuery and QueryDist to sketch package

**Files:**
- Modify: `internal/sketch/sketch.go`
- Modify: `internal/sketch/sketch_test.go`

- [ ] **Step 1: Write tests for ParseDistOutput and SketchQuery/QueryDist signatures**

Append to `internal/sketch/sketch_test.go`:

```go
func TestParseDistOutput(t *testing.T) {
	output := "SAMD00123456\tmy_genome.fasta\t0.9982\nSAMD00789012\tmy_genome.fasta\t0.9971\nSAMD00345678\tmy_genome.fasta\t0.9965\n"

	matches, err := ParseDistOutput(strings.NewReader(output))
	if err != nil {
		t.Fatalf("ParseDistOutput: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("got %d matches, want 3", len(matches))
	}

	if matches[0].RefName != "SAMD00123456" {
		t.Errorf("match[0].RefName = %q", matches[0].RefName)
	}
	if matches[0].QueryName != "my_genome.fasta" {
		t.Errorf("match[0].QueryName = %q", matches[0].QueryName)
	}
	if matches[0].ANI != 0.9982 {
		t.Errorf("match[0].ANI = %v", matches[0].ANI)
	}
}

func TestParseDistOutputSorted(t *testing.T) {
	output := "ref1\tq1\t0.80\nref2\tq1\t0.99\nref3\tq1\t0.90\n"

	matches, err := ParseDistOutput(strings.NewReader(output))
	if err != nil {
		t.Fatalf("ParseDistOutput: %v", err)
	}

	// Should be sorted by ANI descending
	if matches[0].ANI != 0.99 {
		t.Errorf("expected highest ANI first, got %v", matches[0].ANI)
	}
	if matches[2].ANI != 0.80 {
		t.Errorf("expected lowest ANI last, got %v", matches[2].ANI)
	}
}

func TestParseDistOutputEmpty(t *testing.T) {
	matches, err := ParseDistOutput(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseDistOutput: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/sketch/... -v`
Expected: FAIL — ParseDistOutput not defined.

- [ ] **Step 3: Implement Match type, ParseDistOutput, SketchQuery, QueryDist**

Append to `internal/sketch/sketch.go`:

```go
// Match represents a single distance result from sketchlib dist.
type Match struct {
	RefName   string  // sample name from reference database
	QueryName string  // sample name from query
	ANI       float64 // average nucleotide identity (0-1)
}

// ParseDistOutput parses the tab-separated output from sketchlib dist.
// Output format: ref_name \t query_name \t ani_value
// Results are sorted by ANI descending (closest first).
func ParseDistOutput(r io.Reader) ([]Match, error) {
	var matches []Match
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Skip sketchlib status lines (emoji prefix)
		if strings.Contains(line, "sketchlib done") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		ani, err := strconv.ParseFloat(fields[2], 64)
		if err != nil {
			continue
		}
		matches = append(matches, Match{
			RefName:   fields[0],
			QueryName: fields[1],
			ANI:       ani,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].ANI > matches[j].ANI
	})

	return matches, scanner.Err()
}

// SketchQuery creates a temporary sketch database from input FASTA files.
// Returns the temp directory (caller must clean up) and the sketch prefix.
func SketchQuery(inputs []string, kmerSizes []int, threads int) (tmpDir, prefix string, err error) {
	bin, err := FindBinary()
	if err != nil {
		return "", "", err
	}

	tmpDir, err = os.MkdirTemp("", "atb-sketch-*")
	if err != nil {
		return "", "", fmt.Errorf("create temp dir: %w", err)
	}

	prefix = filepath.Join(tmpDir, "query")

	kStrs := make([]string, len(kmerSizes))
	for i, k := range kmerSizes {
		kStrs[i] = strconv.Itoa(k)
	}

	args := []string{"sketch", "-o", prefix, "--k-vals", strings.Join(kStrs, ",")}
	if threads > 0 {
		args = append(args, "--threads", strconv.Itoa(threads))
	}
	args = append(args, inputs...)

	cmd := exec.Command(bin, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return "", "", fmt.Errorf("sketchlib sketch: %w", err)
	}

	return tmpDir, prefix, nil
}

// QueryDist runs sketchlib dist in ref-vs-query mode and returns matches
// sorted by ANI descending. The kmer parameter selects which k-mer size
// to compute ANI with. Results are truncated to topN (0 = all).
func QueryDist(refPrefix, queryPrefix string, kmer, threads, topN int) ([]Match, error) {
	bin, err := FindBinary()
	if err != nil {
		return nil, err
	}

	args := []string{"dist", refPrefix, queryPrefix, "-k", strconv.Itoa(kmer), "--ani"}
	if threads > 0 {
		args = append(args, "--threads", strconv.Itoa(threads))
	}

	cmd := exec.Command(bin, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("sketchlib dist: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("sketchlib dist: %w", err)
	}

	matches, err := ParseDistOutput(strings.NewReader(string(out)))
	if err != nil {
		return nil, err
	}

	if topN > 0 && len(matches) > topN {
		matches = matches[:topN]
	}

	return matches, nil
}
```

Also add these imports to the file header:

```go
import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/sketch/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/sketch/sketch.go internal/sketch/sketch_test.go
git commit -m "feat(sketch): add query sketching and distance parsing"
```

---

### Task 3: Create `internal/cli/sketch_cmd.go` — fetch subcommand

**Files:**
- Create: `internal/cli/sketch_cmd.go`

- [ ] **Step 1: Create the sketch command tree and fetch subcommand**

Create `internal/cli/sketch_cmd.go`:

```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/download"
	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/sketch"
)

const (
	sketchSubdir = "sketch"
	sketchSkmName = "atb_sketchlib.skm"
	sketchSkdName = "atb_sketchlib.skd"

	sketchSkmURL = "https://osf.io/download/nwfkc/"
	sketchSkdURL = "https://osf.io/download/92qmr/"
	sketchSkmMD5 = "" // filled when known; empty skips verification
	sketchSkdMD5 = ""
)

func sketchDir(dataDir string) string {
	return filepath.Join(dataDir, sketchSubdir)
}

func sketchSkmPath(dataDir string) string {
	return filepath.Join(sketchDir(dataDir), sketchSkmName)
}

func sketchDbPrefix(dataDir string) string {
	return filepath.Join(sketchDir(dataDir), "atb_sketchlib")
}

func sketchDbExists(dataDir string) bool {
	skm := filepath.Join(sketchDir(dataDir), sketchSkmName)
	skd := filepath.Join(sketchDir(dataDir), sketchSkdName)
	_, err1 := os.Stat(skm)
	_, err2 := os.Stat(skd)
	return err1 == nil && err2 == nil
}

func newSketchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sketch",
		Short: "Find closest ATB genomes using sketch distances",
		Long: `Find the closest genomes in the AllTheBacteria database to your input
sequences using MinHash sketch distances (via sketchlib).

Requires the sketchlib binary in PATH:
  cargo install sketchlib
  conda install -c bioconda sketchlib`,
	}

	cmd.AddCommand(newSketchFetchCmd())
	cmd.AddCommand(newSketchQueryCmd())
	cmd.AddCommand(newSketchInfoCmd())

	return cmd
}

func newSketchFetchCmd() *cobra.Command {
	var (
		force  bool
		verify bool
	)

	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Download the ATB sketch database from OSF",
		Long: `Download the AllTheBacteria sketch database (~4.2 GB) from OSF.
This is required before running 'atb sketch query'.`,
		Example: `  atb sketch fetch
  atb sketch fetch --force --verify`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := dataDir
			if dir == "" {
				dir = cfg.General.DataDir
			}

			sDir := sketchDir(dir)

			if !force && sketchDbExists(dir) {
				fmt.Fprintln(os.Stderr, "Sketch database already exists. Use --force to re-download.")
				return nil
			}

			fmt.Fprintln(os.Stderr, "Downloading ATB sketch database (~4.2 GB)...")

			tasks := []download.FileTask{
				{URL: sketchSkmURL, Filename: sketchSkmName},
				{URL: sketchSkdURL, Filename: sketchSkdName},
			}
			if verify {
				tasks[0].MD5 = sketchSkmMD5
				tasks[1].MD5 = sketchSkdMD5
			}

			dl := download.New(download.Config{
				OutputDir:  sDir,
				Parallel:   2,
				MaxRetries: 3,
			})

			dl.OnProgress = func(filename string, written, total int64) {
				if total > 0 {
					pct := float64(written) / float64(total) * 100
					fmt.Fprintf(os.Stderr, "\r  %s: %.1f%% (%s / %s)",
						filename, pct, formatSize(written), formatSize(total))
				} else {
					fmt.Fprintf(os.Stderr, "\r  %s: %s", filename, formatSize(written))
				}
			}

			result := dl.DownloadAllFiles(tasks)

			fmt.Fprintf(os.Stderr, "\r\033[K")
			fmt.Fprintf(os.Stderr, "Completed: %d/%d  Bytes: %s\n",
				result.Completed, result.Total, formatSize(result.Bytes))

			if result.Failed > 0 {
				for _, e := range result.Errors {
					fmt.Fprintf(os.Stderr, "  error: %s\n", e.Error)
				}
				return fmt.Errorf("%d download(s) failed", result.Failed)
			}

			fmt.Fprintln(os.Stderr, "Sketch database ready.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "re-download even if database exists")
	cmd.Flags().BoolVar(&verify, "verify", false, "verify MD5 checksums after download")

	return cmd
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/cli/...`
Expected: Success (sketch_cmd.go compiles but isn't registered yet).

- [ ] **Step 3: Commit**

```bash
git add internal/cli/sketch_cmd.go
git commit -m "feat(sketch): add fetch subcommand for sketch database download"
```

---

### Task 4: Add query and info subcommands to sketch_cmd.go

**Files:**
- Modify: `internal/cli/sketch_cmd.go`

- [ ] **Step 1: Add the query subcommand**

Append to `internal/cli/sketch_cmd.go`:

```go
func newSketchQueryCmd() *cobra.Command {
	var (
		fileList string
		knn      int
		threads  int
		format   string
		raw      bool
	)

	cmd := &cobra.Command{
		Use:   "query <fasta> [fasta...]",
		Short: "Find closest ATB genomes for your input sequences",
		Long: `Sketch your input FASTA file(s) and find the closest matches in the
AllTheBacteria sketch database. Results include ANI (Average Nucleotide
Identity) and metadata from the local ATB index.`,
		Example: `  # Find 10 closest genomes (default)
  atb sketch query my_genome.fasta

  # Top 50 closest
  atb sketch query my_genome.fasta --knn 50

  # Multiple input files
  atb sketch query sample1.fasta sample2.fasta

  # Batch from file list (one path per line)
  atb sketch query -f input_list.txt

  # Raw sketchlib distances without metadata enrichment
  atb sketch query my_genome.fasta --raw`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && fileList == "" {
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

			// Check sketchlib binary
			if _, err := sketch.FindBinary(); err != nil {
				return err
			}

			// Check sketch database
			if !sketchDbExists(dir) {
				return fmt.Errorf("sketch database not found. Run 'atb sketch fetch' first")
			}

			// Collect input files
			inputs := args
			if fileList != "" {
				lines, err := readLines(fileList)
				if err != nil {
					return fmt.Errorf("reading file list: %w", err)
				}
				inputs = append(inputs, lines...)
			}
			if len(inputs) == 0 {
				return fmt.Errorf("no input files provided")
			}

			// Validate inputs exist
			for _, f := range inputs {
				if _, err := os.Stat(f); err != nil {
					return fmt.Errorf("input file: %w", err)
				}
			}

			// Get database k-mer sizes
			dbInfo, err := sketch.Info(sketchSkmPath(dir))
			if err != nil {
				return fmt.Errorf("reading sketch database: %w", err)
			}
			if len(dbInfo.KmerSizes) == 0 {
				return fmt.Errorf("sketch database has no k-mer sizes")
			}

			fmt.Fprintf(os.Stderr, "Sketching %d input file(s)...\n", len(inputs))

			// Sketch query sequences
			tmpDir, queryPrefix, err := sketch.SketchQuery(inputs, dbInfo.KmerSizes, threads)
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmpDir)

			fmt.Fprintf(os.Stderr, "Querying ATB database (%d genomes)...\n", dbInfo.Samples)

			// Use the largest k-mer for ANI calculation
			kmer := dbInfo.KmerSizes[len(dbInfo.KmerSizes)-1]
			matches, err := sketch.QueryDist(sketchDbPrefix(dir), queryPrefix, kmer, threads, knn)
			if err != nil {
				return err
			}

			if len(matches) == 0 {
				fmt.Fprintln(os.Stderr, "No matches found.")
				return nil
			}

			// Warn on low ANI
			if matches[0].ANI < 0.80 {
				fmt.Fprintf(os.Stderr, "Warning: closest match has low ANI (%.1f%%) — query may not be bacterial or may be a novel species.\n",
					matches[0].ANI*100)
			}

			fmt.Fprintf(os.Stderr, "%d match(es) found.\n\n", len(matches))

			if raw {
				return printRawMatches(cmd, matches, format)
			}

			return printEnrichedMatches(cmd, matches, dir, format)
		},
	}

	cmd.Flags().StringVarP(&fileList, "file-list", "f", "", "file with one FASTA path per line")
	cmd.Flags().IntVar(&knn, "knn", 10, "number of closest matches to return")
	cmd.Flags().IntVar(&threads, "threads", 1, "CPU threads for sketching and distance calculation")
	cmd.Flags().StringVar(&format, "format", "", "output format: tsv, csv, json, table (default: auto)")
	cmd.Flags().BoolVar(&raw, "raw", false, "output raw distances without metadata enrichment")

	return cmd
}

func printRawMatches(cmd *cobra.Command, matches []sketch.Match, format string) error {
	resolved := output.ResolveFormat(format)
	columns := []string{"query", "sample_accession", "ani"}
	rows := make([]output.Row, len(matches))
	for i, m := range matches {
		rows[i] = output.Row{
			"query":              m.QueryName,
			"sample_accession":   m.RefName,
			"ani":                fmt.Sprintf("%.4f", m.ANI),
		}
	}
	return output.Format(cmd.OutOrStdout(), rows, columns, resolved)
}

func printEnrichedMatches(cmd *cobra.Command, matches []sketch.Match, dir, format string) error {
	resolved := output.ResolveFormat(format)

	// Try to enrich with metadata from SQLite index
	var db *idx.DB
	if idx.Exists(dir) {
		var err error
		db, err = idx.Open(dir)
		if err == nil {
			defer db.Close()
		}
	}

	columns := []string{"query", "sample_accession", "ani", "species", "N50", "completeness", "mlst_st"}
	rows := make([]output.Row, len(matches))
	for i, m := range matches {
		row := output.Row{
			"query":            m.QueryName,
			"sample_accession": m.RefName,
			"ani":              fmt.Sprintf("%.4f", m.ANI),
			"species":          "-",
			"N50":              "-",
			"completeness":     "-",
			"mlst_st":          "-",
		}

		if db != nil {
			if info, err := db.InfoRow(m.RefName); err == nil {
				if v := info["sylph_species"]; v != "" {
					row["species"] = v
				}
				if v := info["N50"]; v != "" {
					row["N50"] = v
				}
				if v := info["Completeness_General"]; v != "" {
					row["completeness"] = v
				}
				if v := info["mlst_st"]; v != "" {
					row["mlst_st"] = v
				}
			}
		}

		rows[i] = row
	}

	return output.Format(cmd.OutOrStdout(), rows, columns, resolved)
}
```

Also add these imports at the top of the file (update the existing import block):

```go
import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/download"
	idx "github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/index"
	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/output"
	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/sketch"
)
```

- [ ] **Step 2: Add the info subcommand**

Append to `internal/cli/sketch_cmd.go`:

```go
func newSketchInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show information about the local sketch database",
		Example: `  atb sketch info`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := dataDir
			if dir == "" {
				dir = cfg.General.DataDir
			}

			if !sketchDbExists(dir) {
				return fmt.Errorf("sketch database not found. Run 'atb sketch fetch' first")
			}

			info, err := sketch.Info(sketchSkmPath(dir))
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Sketch database: %s\n", sketchSkmPath(dir))
			fmt.Fprintf(w, "Samples:         %d\n", info.Samples)
			fmt.Fprintf(w, "K-mer sizes:     %v\n", info.KmerSizes)
			fmt.Fprintf(w, "Sketch size:     %d\n", info.SketchSize)

			// Show file sizes
			sDir := sketchDir(dir)
			var totalSize int64
			for _, name := range []string{sketchSkmName, sketchSkdName} {
				if fi, err := os.Stat(filepath.Join(sDir, name)); err == nil {
					totalSize += fi.Size()
				}
			}
			fmt.Fprintf(w, "Database size:   %s\n", formatSize(totalSize))

			return nil
		},
	}
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/cli/...`
Expected: Success.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/sketch_cmd.go
git commit -m "feat(sketch): add query and info subcommands"
```

---

### Task 5: Register sketch command and verify end-to-end

**Files:**
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Register newSketchCmd() in root.go**

Add `RootCmd.AddCommand(newSketchCmd())` after `newOSFCmd()` in both `init()` and `NewRootCmd()`.

- [ ] **Step 2: Build and run tests**

```bash
make build
go test ./...
```

Expected: All tests pass, binary builds.

- [ ] **Step 3: Manual verification**

```bash
# Help
./bin/atb sketch --help
./bin/atb sketch query --help
./bin/atb sketch fetch --help
./bin/atb sketch info --help
```

- [ ] **Step 4: Commit**

```bash
git add internal/cli/root.go
git commit -m "feat(sketch): register sketch command in root"
```

---

### Task 6: Integration test with real sketchlib

**Files:**
- Modify: `internal/sketch/sketch_test.go`

- [ ] **Step 1: Add integration test (skipped if sketchlib not installed)**

Append to `internal/sketch/sketch_test.go`:

```go
func TestIntegrationSketchAndDist(t *testing.T) {
	if _, err := FindBinary(); err != nil {
		t.Skip("sketchlib not installed, skipping integration test")
	}

	// Create a test FASTA
	tmpDir := t.TempDir()
	fasta := filepath.Join(tmpDir, "test.fasta")
	os.WriteFile(fasta, []byte(">seq1\nACGTACGTACGTACGTACGTACGTACGTACGTACGTACGTACGTACGTACGT\n"), 0644)

	// Sketch it
	skDir, prefix, err := SketchQuery([]string{fasta}, []int{17}, 1)
	if err != nil {
		t.Fatalf("SketchQuery: %v", err)
	}
	defer os.RemoveAll(skDir)

	// Verify sketch files exist
	if _, err := os.Stat(prefix + ".skm"); err != nil {
		t.Fatalf("expected .skm file at %s.skm", prefix)
	}
	if _, err := os.Stat(prefix + ".skd"); err != nil {
		t.Fatalf("expected .skd file at %s.skd", prefix)
	}

	// Self-distance should work
	matches, err := QueryDist(prefix, prefix, 17, 1, 0)
	if err != nil {
		t.Fatalf("QueryDist: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 self-match, got %d", len(matches))
	}
	if matches[0].ANI != 1.0 {
		t.Errorf("self ANI = %v, want 1.0", matches[0].ANI)
	}
}
```

Also add these imports:

```go
import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)
```

- [ ] **Step 2: Run integration test**

Run: `go test ./internal/sketch/... -v -run Integration`
Expected: PASS (sketchlib is installed from earlier in this session).

- [ ] **Step 3: Commit**

```bash
git add internal/sketch/sketch_test.go
git commit -m "test(sketch): add integration test with real sketchlib binary"
```

---
