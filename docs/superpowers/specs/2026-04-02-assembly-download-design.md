# Assembly Download from AMR/MLST Commands

**Date**: 2026-04-02
**Status**: Approved

## Problem

`atb amr` and `atb mlst` query and return results but provide no way to download the matching genome assemblies. Users must manually extract sample accessions, construct S3 URLs, and pipe them to `atb download` — a tedious multi-step process.

## Solution

Add a built-in `--download` flag to both `atb amr` and `atb mlst` commands. When set, the command runs the query, prints results as usual, then downloads the corresponding FASTA assemblies from the AllTheBacteria S3 bucket.

## Design

### New Flags

Both `atb amr` and `atb mlst` gain these flags:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--download` | bool | false | Download FASTA assemblies for matching samples |
| `--download-dir` / `-d` | string | from config | Directory to save downloaded assemblies |
| `--dry-run` | bool | false | Print download URLs without downloading |
| `--max-samples` | int | 0 | Cap number of assemblies to download (0 = unlimited) |

`--parallel` and `--force` are inherited from the user's config (`cfg.Download.*`), consistent with `atb download`.

### Shared Helper

A new file `internal/cli/download_helpers.go` containing:

```go
type AssemblyDownloadConfig struct {
    SampleAccessions []string
    OutputDir        string
    Parallel         int
    DryRun           bool
    MaxSamples       int
    Force            bool
    MinFreeSpaceGB   int
}

func downloadAssemblies(cfg AssemblyDownloadConfig) error
```

The function:

1. Deduplicates `SampleAccessions` (preserving insertion order)
2. Applies `MaxSamples` cap after dedup
3. Constructs URLs: `sources.AssemblyBaseURL + accession + ".fa.gz"`
4. If `DryRun`, prints URLs to stderr and returns nil
5. Otherwise, creates a `download.Downloader` and calls `DownloadAll`
6. Prints summary to stderr: `Downloaded: X/Y  Failed: Z  Bytes: N`
7. Writes `manifest.json` via existing `WriteManifest`
8. Returns error if any downloads failed

### Command Integration

In both `amr_cmd.go` and `mlst_cmd.go`, after the existing `output.Format()` call:

- Extract `sample_accession` values from results
  - AMR: from `[]amr.Result` field `SampleAccession`
  - MLST: from `[]map[string]string` key `"sample_accession"`
- Call `downloadAssemblies()` with config derived from flags and `cfg.Download.*`
- Return the download error if one occurs (query output has already been written)

### URL Construction

Assembly FASTA files follow a predictable pattern on S3:

```
https://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/{sample_accession}.fa.gz
```

This base URL is already defined in `sources.AssemblyBaseURL`.

### What Does NOT Change

- `internal/download/` — reused as-is
- `internal/amr/` — query logic unchanged
- `internal/osf/` — not involved
- `internal/sources/` — only read, not modified
- `internal/mcpserver/` — MCP tools left as-is (LLM clients handle downloads differently)

## Behavior

- Normal query output (table/tsv/csv/json to stdout) is always produced
- Downloads happen after output is rendered
- If query returns 0 results, no downloads are attempted and no error is returned
- Samples are automatically deduplicated (AMR results often have many hits per sample)
- Existing files are skipped (existing `DownloadFile` behavior)
- Progress and summary are printed to stderr

## Examples

```bash
# Download assemblies with beta-lactam resistance
atb amr --species "Escherichia coli" --class "BETA-LACTAM" --hq-only --download -d ./genomes

# Preview what would be downloaded
atb amr --species "Klebsiella pneumoniae" --gene "blaCTX-M-15" --download --dry-run

# Download assemblies for ST131 E. coli, cap at 20
atb mlst --species "Escherichia coli" --st 131 --download --max-samples 20 -d ./st131

# MLST query with download
atb mlst --species "Salmonella enterica" --status PERFECT --hq-only --download -d ./salmonella
```
