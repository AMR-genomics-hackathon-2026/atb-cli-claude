# Assembly Download from AMR/MLST Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--download` flag to `atb amr` and `atb mlst` commands so users can download matching genome assemblies in a single command.

**Architecture:** A shared `downloadAssemblies()` helper in `internal/cli/download_helpers.go` handles deduplication, URL construction, and download orchestration using the existing `internal/download` package. Both `amr_cmd.go` and `mlst_cmd.go` call this helper after rendering query output.

**Tech Stack:** Go, cobra (CLI), existing `internal/download.Downloader`, `internal/sources.AssemblyBaseURL`

---

### Task 1: Create the shared download helper with tests

**Files:**
- Create: `internal/cli/download_helpers.go`
- Create: `internal/cli/download_helpers_test.go`

- [ ] **Step 1: Write the test file**

```go
// internal/cli/download_helpers_test.go
package cli

import (
	"testing"
)

func TestDeduplicateAccessions(t *testing.T) {
	input := []string{"SAMN001", "SAMN002", "SAMN001", "SAMN003", "SAMN002"}
	got := deduplicateAccessions(input)

	want := []string{"SAMN001", "SAMN002", "SAMN003"}
	if len(got) != len(want) {
		t.Fatalf("expected %d accessions, got %d: %v", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("index %d: expected %q, got %q", i, w, got[i])
		}
	}
}

func TestDeduplicateAccessionsEmpty(t *testing.T) {
	got := deduplicateAccessions(nil)
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestBuildAssemblyURL(t *testing.T) {
	url := buildAssemblyURL("SAMN00000355")
	want := "https://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/SAMN00000355.fa.gz"
	if url != want {
		t.Errorf("expected %q, got %q", want, url)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/ubuntu/atb-cli && go test ./internal/cli/ -run "TestDeduplicate|TestBuildAssembly" -v`
Expected: compilation error — `deduplicateAccessions` and `buildAssemblyURL` not defined

- [ ] **Step 3: Write the helper implementation**

```go
// internal/cli/download_helpers.go
package cli

import (
	"fmt"
	"os"

	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/download"
	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/sources"
)

// AssemblyDownloadConfig holds options for downloading genome assemblies.
type AssemblyDownloadConfig struct {
	SampleAccessions []string
	OutputDir        string
	Parallel         int
	DryRun           bool
	MaxSamples       int
	Force            bool
	MinFreeSpaceGB   int
}

// downloadAssemblies deduplicates accessions, constructs S3 URLs, and downloads
// the corresponding FASTA assemblies. It prints progress and summary to stderr.
func downloadAssemblies(cfg AssemblyDownloadConfig) error {
	accessions := deduplicateAccessions(cfg.SampleAccessions)
	if len(accessions) == 0 {
		return nil
	}

	if cfg.MaxSamples > 0 && len(accessions) > cfg.MaxSamples {
		fmt.Fprintf(os.Stderr, "Capping to %d assemblies (--max-samples)\n", cfg.MaxSamples)
		accessions = accessions[:cfg.MaxSamples]
	}

	urls := make([]string, len(accessions))
	for i, acc := range accessions {
		urls[i] = buildAssemblyURL(acc)
	}

	if cfg.DryRun {
		fmt.Fprintf(os.Stderr, "Would download %d assembly file(s) to %s\n", len(urls), cfg.OutputDir)
		for _, u := range urls {
			fmt.Fprintf(os.Stderr, "  %s\n", u)
		}
		return nil
	}

	fmt.Fprintf(os.Stderr, "Downloading %d assembly file(s) to %s\n", len(urls), cfg.OutputDir)

	dl := download.New(download.Config{
		OutputDir:      cfg.OutputDir,
		Parallel:       cfg.Parallel,
		CheckDiskSpace: !cfg.Force,
		MinFreeSpaceGB: cfg.MinFreeSpaceGB,
		MaxRetries:     3,
	})

	result := dl.DownloadAll(urls)

	fmt.Fprintf(os.Stderr, "Completed: %d/%d  Failed: %d  Bytes: %s\n",
		result.Completed, result.Total, result.Failed, formatSize(result.Bytes))

	for _, e := range result.Errors {
		fmt.Fprintf(os.Stderr, "  error: %s: %s\n", e.URL, e.Error)
	}

	if err := dl.WriteManifest("atb assembly download", result); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to write manifest: %v\n", err)
	}

	if result.Failed > 0 {
		return fmt.Errorf("%d download(s) failed", result.Failed)
	}

	return nil
}

// deduplicateAccessions removes duplicate accessions while preserving order.
func deduplicateAccessions(accessions []string) []string {
	seen := make(map[string]struct{}, len(accessions))
	out := make([]string, 0, len(accessions))
	for _, a := range accessions {
		if _, ok := seen[a]; !ok {
			seen[a] = struct{}{}
			out = append(out, a)
		}
	}
	return out
}

// buildAssemblyURL constructs the S3 download URL for a genome assembly.
func buildAssemblyURL(sampleAccession string) string {
	return sources.AssemblyBaseURL + sampleAccession + ".fa.gz"
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/ubuntu/atb-cli && go test ./internal/cli/ -run "TestDeduplicate|TestBuildAssembly" -v`
Expected: all 3 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/download_helpers.go internal/cli/download_helpers_test.go
git commit -m "feat(cli): add shared assembly download helper"
```

---

### Task 2: Add --download flags to atb amr

**Files:**
- Modify: `internal/cli/amr_cmd.go`

- [ ] **Step 1: Add flag variables to the var block at the top of newAMRCmd()**

Add these four variables to the existing `var` block (after `outputFile string`):

```go
		downloadFlag bool
		downloadDir  string
		dryRun       bool
		maxSamples   int
```

- [ ] **Step 2: Add download logic after the output.Format() call in RunE**

Replace the bare `return output.Format(w, rows, cols, resolvedFormat)` with:

```go
			if err := output.Format(w, rows, cols, resolvedFormat); err != nil {
				return err
			}

			if downloadFlag && len(results) > 0 {
				accessions := make([]string, len(results))
				for i, r := range results {
					accessions[i] = r.SampleAccession
				}

				outDir := downloadDir
				if outDir == "" {
					outDir = cfg.Download.OutputDir
				}

				return downloadAssemblies(AssemblyDownloadConfig{
					SampleAccessions: accessions,
					OutputDir:        outDir,
					Parallel:         cfg.Download.Parallel,
					DryRun:           dryRun,
					MaxSamples:       maxSamples,
					Force:            false,
					MinFreeSpaceGB:   cfg.Download.MinFreeSpaceGB,
				})
			}

			return nil
```

`cfg` is already loaded at line 69 of the existing RunE — reuse it directly.

- [ ] **Step 3: Register the flags on the command**

Add after the existing `cmd.Flags()` lines:

```go
	cmd.Flags().BoolVar(&downloadFlag, "download", false, "download FASTA assemblies for matching samples")
	cmd.Flags().StringVarP(&downloadDir, "download-dir", "d", "", "directory to save downloaded assemblies (default from config)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print download URLs without downloading")
	cmd.Flags().IntVar(&maxSamples, "max-samples", 0, "limit number of assemblies to download")
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /home/ubuntu/atb-cli && go build ./...`
Expected: no errors

- [ ] **Step 5: Run existing AMR tests to verify no regressions**

Run: `cd /home/ubuntu/atb-cli && go test ./internal/cli/ -run "AMR|Amr" -v`
Expected: all existing tests pass

- [ ] **Step 6: Commit**

```bash
git add internal/cli/amr_cmd.go
git commit -m "feat(amr): add --download flag for assembly download"
```

---

### Task 3: Add --download flags to atb mlst

**Files:**
- Modify: `internal/cli/mlst_cmd.go`

- [ ] **Step 1: Add flag variables to the var block at the top of newMLSTCmd()**

Add these four variables to the existing `var` block (after `outputFile string`):

```go
		downloadFlag bool
		downloadDir  string
		dryRun       bool
		maxSamples   int
```

- [ ] **Step 2: Add download logic after the output.Format() call in RunE**

Replace the bare `return output.Format(...)` with:

```go
			if err := output.Format(w, outRows, mlstCols, resolvedFormat); err != nil {
				return err
			}

			if downloadFlag && len(rows) > 0 {
				accessions := make([]string, len(rows))
				for i, r := range rows {
					accessions[i] = r["sample_accession"]
				}

				outDir := downloadDir
				if outDir == "" {
					outDir = cfg.Download.OutputDir
				}

				return downloadAssemblies(AssemblyDownloadConfig{
					SampleAccessions: accessions,
					OutputDir:        outDir,
					Parallel:         cfg.Download.Parallel,
					DryRun:           dryRun,
					MaxSamples:       maxSamples,
					Force:            false,
					MinFreeSpaceGB:   cfg.Download.MinFreeSpaceGB,
				})
			}

			return nil
```

Here `cfg` is already loaded at line 48 and `rows` is the `[]map[string]string` from the query at line 77.

- [ ] **Step 3: Register the flags on the command**

Add after the existing `cmd.Flags()` lines:

```go
	cmd.Flags().BoolVar(&downloadFlag, "download", false, "download FASTA assemblies for matching samples")
	cmd.Flags().StringVarP(&downloadDir, "download-dir", "d", "", "directory to save downloaded assemblies (default from config)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print download URLs without downloading")
	cmd.Flags().IntVar(&maxSamples, "max-samples", 0, "limit number of assemblies to download")
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /home/ubuntu/atb-cli && go build ./...`
Expected: no errors

- [ ] **Step 5: Run existing MLST tests to verify no regressions**

Run: `cd /home/ubuntu/atb-cli && go test ./internal/cli/ -run "MLST|Mlst" -v`
Expected: all existing tests pass

- [ ] **Step 6: Commit**

```bash
git add internal/cli/mlst_cmd.go
git commit -m "feat(mlst): add --download flag for assembly download"
```

---

### Task 4: Add integration tests for --download --dry-run

**Files:**
- Modify: `internal/cli/cli_test.go`

- [ ] **Step 1: Write dry-run integration tests**

Append to `internal/cli/cli_test.go`:

```go
func TestAMRDownloadDryRun(t *testing.T) {
	// Build the index first so the AMR command can find HQ samples
	stdout, stderr, err := runCmd("amr", "--data-dir", fixtureDir,
		"--species", "Escherichia coli", "--limit", "5",
		"--download", "--dry-run")
	if err != nil {
		t.Fatalf("amr --download --dry-run failed: %v\nstderr: %s", err, stderr)
	}

	// Normal query output should still appear on stdout
	if !strings.Contains(stdout, "sample_accession") {
		t.Errorf("expected tabular output on stdout, got:\n%s", stdout)
	}

	// Dry-run messages should appear on stderr
	if !strings.Contains(stderr, "Would download") {
		t.Errorf("expected dry-run message on stderr, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, ".fa.gz") {
		t.Errorf("expected .fa.gz URLs in dry-run output, got:\n%s", stderr)
	}
}

func TestMLSTDownloadDryRun(t *testing.T) {
	stdout, stderr, err := runCmd("mlst", "--data-dir", fixtureDir,
		"--species", "Escherichia coli", "--limit", "5",
		"--download", "--dry-run")
	if err != nil {
		t.Fatalf("mlst --download --dry-run failed: %v\nstderr: %s", err, stderr)
	}

	if !strings.Contains(stdout, "sample_accession") {
		t.Errorf("expected tabular output on stdout, got:\n%s", stdout)
	}

	if !strings.Contains(stderr, "Would download") {
		t.Errorf("expected dry-run message on stderr, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, ".fa.gz") {
		t.Errorf("expected .fa.gz URLs in dry-run output, got:\n%s", stderr)
	}
}

func TestAMRDownloadMaxSamples(t *testing.T) {
	_, stderr, err := runCmd("amr", "--data-dir", fixtureDir,
		"--species", "Escherichia coli",
		"--download", "--dry-run", "--max-samples", "2")
	if err != nil {
		t.Fatalf("amr --download --max-samples failed: %v\nstderr: %s", err, stderr)
	}

	if !strings.Contains(stderr, "Capping to 2") {
		// Only expect capping message if there are more than 2 unique samples
		// Count .fa.gz lines to verify cap was applied
		count := strings.Count(stderr, ".fa.gz")
		if count > 2 {
			t.Errorf("expected at most 2 URLs in dry-run output, got %d\nstderr: %s", count, stderr)
		}
	}
}
```

- [ ] **Step 2: Run the new tests**

Run: `cd /home/ubuntu/atb-cli && go test ./internal/cli/ -run "TestAMRDownload|TestMLSTDownload" -v`
Expected: all 3 tests PASS

- [ ] **Step 3: Run the full test suite to check for regressions**

Run: `cd /home/ubuntu/atb-cli && go test ./...`
Expected: all tests pass

- [ ] **Step 4: Commit**

```bash
git add internal/cli/cli_test.go
git commit -m "test(cli): add integration tests for --download --dry-run"
```

---

### Task 5: Update command examples and README

**Files:**
- Modify: `internal/cli/amr_cmd.go` (Example string)
- Modify: `internal/cli/mlst_cmd.go` (Example string)

- [ ] **Step 1: Add download examples to AMR command**

Append to the `Example` string in `newAMRCmd()`:

```
  # Download assemblies with beta-lactam resistance
  atb amr --species "Escherichia coli" --class "BETA-LACTAM" --hq-only --download -d ./genomes

  # Preview assemblies that would be downloaded
  atb amr --species "Klebsiella pneumoniae" --gene "blaCTX-M-15" --download --dry-run
```

- [ ] **Step 2: Add download examples to MLST command**

Append to the `Example` string in `newMLSTCmd()`:

```
  # Download assemblies for ST131 E. coli
  atb mlst --species "Escherichia coli" --st 131 --download -d ./st131

  # Preview download, cap at 20 assemblies
  atb mlst --species "Salmonella enterica" --status PERFECT --download --dry-run --max-samples 20
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /home/ubuntu/atb-cli && go build ./...`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/cli/amr_cmd.go internal/cli/mlst_cmd.go
git commit -m "docs(cli): add --download examples to amr and mlst commands"
```
