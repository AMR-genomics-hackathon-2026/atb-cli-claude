# atb sketch — Find closest ATB genomes via sketch distances

## Problem

Users have a genome assembly (FASTA) and want to find the closest matches in the AllTheBacteria database (~3.2M genomes). Currently there's no way to do this from the CLI — users must manually install sketchlib, download the sketch database from OSF, figure out the k-mer parameters, run the distance calculation, and then cross-reference sample accessions with ATB metadata.

## Solution

`atb sketch` wraps the `sketchlib` Rust binary to provide a single-command workflow: sketch the user's input, query it against a pre-built ATB sketch database, and return enriched results with species, quality, and MLST metadata from the local SQLite index.

## Architecture

`atb sketch` shells out to the `sketchlib` CLI binary (user installs separately via `cargo install sketchlib` or `conda install -c bioconda sketchlib`). This preserves atb-cli's pure-Go, no-CGO design while leveraging sketchlib's optimized Rust implementation.

### Data flow

```
user.fasta
  → sketchlib sketch (temp .skm/.skd for query)
  → sketchlib dist --knn N --ani (query vs ATB database)
  → parse tab-separated output
  → join sample accessions against SQLite index
  → enriched output table
```

### File layout

```
~/.atb/data/
├── assembly.parquet          # existing
├── atb.db                    # existing SQLite index
├── sketch/                   # NEW
│   ├── atb_sketchlib.skm     # metadata (122 MB)
│   └── atb_sketchlib.skd     # sketch data (4.1 GB)
```

## Subcommands

### `atb sketch fetch`

Downloads the aggregated ATB sketch database (.skm + .skd pair) from OSF.

```bash
atb sketch fetch              # download aggregated 2024-08 (default, 4.2 GB)
atb sketch fetch --force      # re-download
atb sketch fetch --verify     # verify MD5 after download
```

OSF sources (aggregated 2024-08):
- `.skm`: https://osf.io/download/nwfkc/ (122 MB, MD5 from index)
- `.skd`: https://osf.io/download/92qmr/ (4.1 GB, MD5 from index)

Uses existing `download.Downloader` with `FileTask` structs.

### `atb sketch query`

Sketches user input, finds N closest ATB genomes, enriches with metadata.

```bash
# Single file
atb sketch query my_genome.fasta

# Multiple files
atb sketch query sample1.fasta sample2.fasta

# Batch from file list
atb sketch query -f input_list.txt

# Options
atb sketch query genome.fasta --knn 10         # top 10 closest (default)
atb sketch query genome.fasta --threads 4      # parallel
atb sketch query genome.fasta --format json    # output format
atb sketch query genome.fasta --raw            # plain distances, no enrichment
```

**Output columns (enriched, default):**

| Column | Source |
|--------|--------|
| query | input filename |
| sample_accession | sketchlib dist output |
| ani | sketchlib dist output |
| species | SQLite index (sylph_species) |
| N50 | SQLite index |
| completeness | SQLite index (Completeness_General) |
| mlst_st | SQLite index |

Missing metadata shows as `-`.

**Under the hood — two sketchlib calls:**
1. `sketchlib sketch -o <tmpdir>/query --k-vals <from_db> <inputs>` — sketch query
2. `sketchlib dist <db>.skm <tmpdir>/query.skm --knn N --ani` — find neighbors

K-mer values are read from the ATB `.skm` via `sketchlib info` to ensure they match.

### `atb sketch info`

Shows info about the local sketch database.

```bash
atb sketch info
# Sketch database: ~/.atb/data/sketch/atb_sketchlib.skm
# Samples:         3,218,671
# K-mer sizes:     17, 29
# Sketch size:     1000
# Database size:   4.2 GB
```

## Error handling

| Condition | Behavior |
|-----------|----------|
| sketchlib not in PATH | Error with install instructions (cargo/conda) |
| Sketch database not fetched | Error: "Run 'atb sketch fetch' first" |
| Invalid FASTA input | Forward sketchlib's stderr |
| Very low ANI (<80%) | Warning: query may not be bacterial |
| Sample not in local index | Show `-` for missing metadata columns |
| K-mer mismatch | Auto-detect from .skm, never a user concern |

Temp files created in `os.MkdirTemp`, cleaned up via `defer os.RemoveAll`.

## New code

| File | Action | Purpose |
|------|--------|---------|
| `internal/sketch/sketch.go` | create | sketchlib wrapper: find binary, run commands, parse output |
| `internal/sketch/sketch_test.go` | create | Tests parsing logic with mock output |
| `internal/cli/sketch_cmd.go` | create | fetch, query, info subcommands |
| `internal/cli/root.go` | modify | Register `newSketchCmd()` |

### `internal/sketch/sketch.go` API

```go
func FindBinary() (string, error)

type DatabaseInfo struct {
    Samples    int
    KmerSizes  []int
    SketchSize int
}

func Info(skmPath string) (*DatabaseInfo, error)
func SketchQuery(inputs []string, kmerSizes []int, threads int) (tmpDir string, err error)

type Match struct {
    Query           string
    SampleAccession string
    ANI             float64
}

func QueryDist(refSkm, querySkm string, knn, threads int) ([]Match, error)
```

### Reuse from existing code

- `download.Downloader` / `download.FileTask` — fetch with MD5 verification
- `index.DB.InfoRow()` — look up sample metadata for enrichment
- `output.Format()` / `output.ResolveFormat()` — TSV/table/JSON output
- `cli.loadConfig()` — data-dir resolution

## Not in scope

- Building sketch databases from scratch (users use `sketchlib sketch` directly)
- Inverted index queries (`.ski` files) — future optimization for sub-second queries
- Bundling sketchlib binary in atb releases — user installs separately
- Sketch database version selection — hardcoded to aggregated 2024-08 for now
