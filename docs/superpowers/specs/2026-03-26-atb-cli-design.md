# atb-cli Design Specification

> A cross-platform CLI tool for querying the AllTheBacteria (ATB) genomics database and downloading genome assemblies.

**Date:** 2026-03-26
**Status:** Approved

---

## 1. Overview

`atb-cli` is a standalone Go binary that lets bioinformaticians and researchers query ~3.2 million bacterial genome assemblies from the AllTheBacteria project. It reads parquet files locally using pure Go libraries (no CGO), supports flexible filtering via CLI flags or TOML files, and downloads genome FASTA files from AWS S3 or OSF.

### Design goals

- **Standalone**: single binary, no runtime dependencies, runs on Linux and macOS (amd64/arm64)
- **Fast**: predicate pushdown on parquet reads, parallel downloads, lazy table loading
- **Reproducible**: TOML filter files, download manifests, versioned database
- **Accessible**: simple commands for beginners, power-user flags for experts
- **Testable**: high test coverage with synthetic parquet fixtures and golden file tests

---

## 2. Command Structure

```
atb
├── config            Manage configuration
│   ├── init          Create default config at ~/.config/atb/config.toml
│   ├── set           Set a config value
│   ├── get           Get a config value
│   └── show          Print full config
│
├── fetch             Download parquet database from OSF
│                     --all, --tables <list>, --force
│
├── update            Check for and apply database updates
│
├── query             Query the database
│                     Filters via flags or -f filters.toml
│
├── summarise         Generate summary statistics
│                     --by <dimensions> or default report
│                     --from <file> for previous query results
│
├── download          Download genome FASTA files
│                     --from <query-result>, --urls <file>, --url <single>
│                     --parallel N, --dry-run, --force
│
├── info              Get all info on a specific sample
│                     atb info <sample_accession>
│
└── version           Print CLI version, database version, config path
```

### Pipeline design

Commands are designed to compose:

```bash
# Query then download
atb query --species "Escherichia coli" --hq-only --output results.tsv
atb download --from results.tsv --output-dir ./ecoli_genomes

# Pipe query to summarise
atb query --genus Salmonella --hq-only | atb summarise --from -

# Pipe query to download
atb query --species "E. coli" --hq-only --format csv | atb download --from - --output-dir ./out
```

---

## 3. Data Architecture

### Config file

Location: `~/.config/atb/config.toml`

```toml
[general]
data_dir = "~/atb/metadata/parquet"
default_format = "auto"       # auto|tsv|csv|json|table

[fetch]
base_url = "https://osf.io"
parallel = 4

[download]
parallel = 4
output_dir = "./genomes"
check_disk_space = true
min_free_space_gb = 5
```

Created by `atb config init` with sensible defaults.

### Parquet tables

**Core tables** (fetched by default, ~540MB):

| Table | Rows | Size | Primary Key |
|-------|------|------|-------------|
| `assembly.parquet` | 3,227,665 | 95MB | `sample_accession` |
| `assembly_stats.parquet` | 2,778,266 | 111MB | `sample_accession` |
| `checkm2.parquet` | 2,767,044 | 74MB | `sample_accession` |
| `sylph.parquet` | 3,815,195 | 109MB | `sample_accession` + `run_accession` |
| `run.parquet` | 3,530,512 | 148MB | `run_accession` + `sample_accession` |

**ENA tables** (on-demand, ~2.5GB):

| Table | Rows | Size | Primary Key |
|-------|------|------|-------------|
| `ena_20250506.parquet` | 3,514,266 | 856MB | `run_accession` |
| `ena_20240801.parquet` | 3,112,706 | 757MB | `run_accession` |
| `ena_20240625.parquet` | 3,055,816 | 752MB | `run_accession` |
| `ena_202505_used.parquet` | 340,016 | 72MB | `run_accession` |
| `ena_661k.parquet` | — | 121MB | `run_accession` |

### Key columns in assembly.parquet

| Column | Type | Description |
|--------|------|-------------|
| `sample_accession` | string | Primary key (e.g., SAMN02391170) |
| `run_accession` | string | SRA run accession |
| `assembly_accession` | string | ENA assembly accession |
| `sylph_species` | string | GTDB species call |
| `hq_filter` | string | "PASS" or failure reason |
| `asm_fasta_on_osf` | int64 | 1 if FASTA available on OSF |
| `dataset` | string | Release batch (661k, r0.2, etc.) |
| `aws_url` | string | Direct S3 download URL for genome FASTA |
| `osf_tarball_url` | string | OSF tar.xz download URL |

### Query execution model

1. Always start with `assembly.parquet` - apply species, HQ, dataset filters with predicate pushdown
2. Collect matching `sample_accession` values into a hash set
3. Join secondary tables only if their columns are requested (filter or output)
4. Scan secondary tables with column projection, keeping only rows in the hash set

**Join decision tree:**

```
assembly.parquet (always first)
    ├── completeness/contamination? → join checkm2.parquet on sample_accession
    ├── N50/total_length/contigs?   → join assembly_stats.parquet on sample_accession
    ├── sylph ANI/abundance?        → join sylph.parquet on sample_accession
    └── country/date/platform?      → join run.parquet → ena_20250506.parquet (latest, default)
                                      (prompt to fetch ENA if missing)

**ENA table selection:** Always use `ena_20250506.parquet` (the latest snapshot with best coverage). The older ENA files exist for historical reproducibility but are not needed for standard queries. The `--ena-version` flag can override this in future.
```

---

## 4. Query Filters

### CLI flags

```
# Species/taxonomy
--species "Escherichia coli"       Exact match (case-insensitive)
--species-like "Salmonella%"       Wildcard match
--genus "Escherichia"              Filter by genus (derived from species)

# Sample selection
--sample SAMN02391170              Specific sample(s), comma-separated
--sample-file samples.txt          File with one accession per line

# Quality filters
--hq-only                          hq_filter == "PASS"
--min-completeness 95.0            checkm2 Completeness_General >= N
--max-contamination 5.0            checkm2 Contamination <= N
--min-n50 50000                    assembly_stats N50 >= N

# Assembly filters
--dataset "661k"                   Filter by dataset
--has-assembly                     asm_fasta_on_osf == 1

# ENA metadata filters (trigger ENA table fetch if needed)
--country "United Kingdom"
--platform "ILLUMINA"
--collection-date-from 2020-01-01
--collection-date-to 2023-12-31

# Output control
--columns "sample_accession,sylph_species,N50,aws_url"
--limit 100
--offset 0
--sort-by N50 --sort-desc
--format tsv|csv|json|parquet|table
--output results.tsv
```

### TOML filter file

```toml
[filter]
species = "Escherichia coli"
hq_only = true
min_completeness = 95.0
max_contamination = 5.0
min_n50 = 50000
country = "United Kingdom"

[output]
columns = ["sample_accession", "sylph_species", "N50", "Completeness_General", "aws_url"]
sort_by = "N50"
sort_desc = true
limit = 100
format = "tsv"
output = "results.tsv"
```

**Precedence:** CLI flags override TOML values.

---

## 5. Download System

### Two modes

**Mode 1: From query results**
```bash
atb download --from results.tsv --output-dir ./genomes
```
The `--from` file must contain an `aws_url` or `sample_accession` column. If only `sample_accession`, URLs are looked up from `assembly.parquet`.

**Mode 2: From URL list**
```bash
atb download --urls urls.txt --output-dir ./genomes
atb download --url https://example.com/genome.fa.gz --output-dir ./genomes
```

### Features

| Feature | Detail |
|---------|--------|
| Parallel downloads | `--parallel N` (default from config) |
| Resume support | `.part` files + HTTP Range headers |
| Disk space check | HEAD requests for Content-Length, compare with available space |
| Progress reporting | Per-file and overall progress bar to stderr |
| Integrity verification | Verify `.fa.gz` is valid gzip |
| Rate limiting | Exponential backoff on 429/503 |
| Max samples | `--max-samples N` safety cap |
| Dry run | `--dry-run` shows what would be downloaded + estimated size |

### Disk space check flow

1. Send HTTP HEAD requests (parallel) to get `Content-Length` per URL
2. Sum total expected bytes
3. Query available disk space on target filesystem
4. If `total > available - min_free_space_gb` → warn and abort
5. Override with `--force`

### Download manifest

Written to output directory as `manifest.json`:

```json
{
  "query": "species=Escherichia coli, hq_only=true",
  "timestamp": "2026-03-26T14:30:00Z",
  "total_files": 150,
  "completed": 150,
  "failed": 0,
  "total_bytes": 1073741824
}
```

---

## 6. Summarise Command

### Default report

`atb summarise` outputs a fixed report: total genomes, HQ count, top 10 species, dataset breakdown, median assembly stats, median QC metrics.

### Custom breakdowns

```bash
atb summarise --by species --top 20
atb summarise --by species,country --top 10
atb summarise --from results.tsv
```

**Available dimensions:**

| Dimension | Source Table |
|-----------|-------------|
| `species` | assembly (`sylph_species`) |
| `genus` | assembly (derived) |
| `dataset` | assembly |
| `hq_status` | assembly (`hq_filter`) |
| `country` | ena_20250506 |
| `platform` | ena_20250506 (`instrument_platform`) |
| `collection_year` | ena_20250506 (from `collection_date`) |

`--from` mode computes stats from columns present in the input file.

---

## 7. Info Command

```bash
atb info SAMN02391170
```

Prints all available data for a sample from all locally available tables. If a table is not fetched, reports that rather than silently omitting.

---

## 8. Version & Update Management

### `atb version`

Prints CLI version, Go version, OS/arch, database version, fetched tables, data directory, config path.

### `atb update`

1. Fetches a version manifest JSON from OSF (small file with latest version + checksums)
2. Compares with local `~/.config/atb/metadata.json`
3. Shows changes, asks confirmation, downloads only changed files

### Local metadata tracking

`~/.config/atb/metadata.json`:

```json
{
  "version": "202505",
  "fetched_at": "2026-03-26T10:00:00Z",
  "tables": {
    "assembly.parquet": {"size": 95163613, "md5": "abc123..."},
    "assembly_stats.parquet": {"size": 111162176, "md5": "def456..."}
  }
}
```

---

## 9. Output Formatting

**Auto-detection:** Pretty table when stdout is a terminal, TSV when piped.

Override with `--format tsv|csv|json|parquet|table`.

---

## 10. Error Handling

| Scenario | Behavior |
|----------|----------|
| Database not found | `Database not found. Run 'atb fetch' first.` |
| Species not found | Suggest closest matches (Levenshtein distance) |
| No query results | Print filter summary + `0 results found`, exit 0 |
| Network failure | Retry with exponential backoff (3 attempts), resume partial |
| Disk full mid-download | Detect, abort gracefully, report progress |
| Invalid TOML | Parse error with line number and field name |
| Corrupt parquet | Detect on read, suggest `atb fetch --force` |
| ENA table not fetched | `This query needs ENA data (856MB). Fetch now? [y/N]` |

---

## 11. Testing Strategy

| Layer | Approach |
|-------|----------|
| Unit tests | Filter parsing, TOML parsing, config, column mapping, disk space, species matching |
| Integration tests | Synthetic parquet fixtures (100-1000 rows), full query-to-output pipeline |
| Table-driven tests | All filter combinations, edge cases |
| Golden file tests | Expected output for known queries, regression detection |
| Download tests | Mock HTTP server, parallel downloads, resume, disk checks |
| CLI tests | Flag parsing, TOML override, help output, exit codes |

Test fixtures generated by `testdata/generate.py` (Python script using pyarrow).

---

## 12. Project Structure

```
atb-cli/
├── cmd/atb/main.go                  Entry point
├── internal/
│   ├── cli/                         Cobra command definitions
│   │   ├── root.go
│   │   ├── config.go
│   │   ├── fetch.go
│   │   ├── query.go
│   │   ├── summarise.go
│   │   ├── download.go
│   │   ├── info.go
│   │   ├── update.go
│   │   └── version.go
│   ├── config/                      TOML config management
│   │   ├── config.go
│   │   └── config_test.go
│   ├── parquet/                     Parquet reading, filtering, joins
│   │   ├── reader.go
│   │   ├── filter.go
│   │   ├── join.go
│   │   └── reader_test.go
│   ├── query/                       Query planning & execution
│   │   ├── planner.go
│   │   ├── executor.go
│   │   ├── filters.go
│   │   └── planner_test.go
│   ├── download/                    File download engine
│   │   ├── downloader.go
│   │   ├── disk.go
│   │   ├── progress.go
│   │   └── downloader_test.go
│   ├── fetch/                       Database fetch from OSF
│   │   ├── fetcher.go
│   │   └── fetcher_test.go
│   ├── summarise/                   Summary statistics
│   │   ├── stats.go
│   │   └── stats_test.go
│   ├── output/                      Formatting (table, tsv, csv, json)
│   │   ├── formatter.go
│   │   ├── detect.go
│   │   └── formatter_test.go
│   └── suggest/                     Fuzzy species name matching
│       ├── suggest.go
│       └── suggest_test.go
├── testdata/
│   ├── generate.py                  Generate test parquet fixtures
│   ├── fixtures/                    Small parquet files for tests
│   └── golden/                      Expected outputs
├── go.mod
├── go.sum
├── .goreleaser.yml
├── Makefile
└── README.md
```

---

## 13. Key Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/BurntSushi/toml` | TOML config parsing |
| `github.com/apache/arrow-go` | Arrow columnar memory format |
| `github.com/parquet-go/parquet-go` | Pure Go parquet reader with predicate pushdown |
| `github.com/olekukonez/tablewriter` | Pretty table output |
| `github.com/schollz/progressbar/v3` | Download progress bars |
| `golang.org/x/term` | Terminal detection for auto-format |

---

## 14. Distribution

| Channel | Method |
|---------|--------|
| GitHub Releases | `goreleaser` for linux/macos amd64/arm64 |
| Homebrew | Tap formula |
| Bioconda | Future |
| Go install | `go install github.com/AMR-genomics-hackathon-2026/atb-cli-claude@latest` |

---

## 15. Future Work (out of scope for v1)

- MLST as a first-class query target
- Closest genome search (mash/sourmash distance)
- AMRFinderPlus results integration (Hive-partitioned parquet from Shiny app)
- Interactive TUI mode
- Bioconda packaging
