# atb-cli-claude vs atb-cli-codex: Comparison

> Fair comparison of two independent implementations of the AllTheBacteria CLI tool,
> both built during the AMR Genomics Hackathon 2026.

## Summary

| Dimension | atb-cli-claude (ours) | atb-cli-codex |
|-----------|----------------------|---------------|
| **Language** | Go | Go |
| **CLI framework** | cobra | cobra |
| **Parquet library** | parquet-go v0.29.0 | parquet-go v0.25.1 |
| **Config** | BurntSushi/toml | BurntSushi/toml |
| **Total Go files** | 44 | 19 |
| **Test files** | 15 | 7 |
| **Total lines of Go** | ~5,256 | ~4,507 |
| **Test lines** | ~1,500+ | ~1,166 |
| **CI/CD** | GitHub Actions (test + release) | None |
| **Release automation** | GoReleaser (6 binaries) | Manual build.sh |
| **Install script** | curl \| bash | None |
| **Self-update** | Built-in (`atb update`) | None |
| **README** | Comprehensive with benchmarks | One line |

---

## Architecture

### atb-cli-claude

Flat, straightforward architecture. Each internal package has one job:
- `internal/parquet/` - Generic typed parquet reader (ReadAll, ReadFiltered)
- `internal/query/` - Filters, planner (decides which tables to join), executor
- `internal/download/` - Parallel HTTP downloader with resume, disk space checks
- `internal/output/` - TSV/CSV/JSON/table formatters
- `internal/config/` - TOML config
- `internal/suggest/` - Fuzzy species matching
- `internal/summarise/` - Statistics engine
- `internal/fetch/` - OSF data fetcher
- `internal/selfupdate/` - Self-update from GitHub releases

Query approach: assembly.parquet is always the starting table. Secondary tables are joined lazily based on what filters/columns the user requests.

### atb-cli-codex

More layered, interface-driven architecture:
- `internal/store/` - Store interface with LocalStore implementation (parquet + JSON cache)
- `internal/query/` - Service layer with typed Record model, filtering, sampling
- `internal/source/` - Discovery system (OSF API + GitHub API for auto-fetching)
- `internal/download/` - Download planner with AWS vs OSF-tarball strategy selection
- `internal/cache/` - Cache layout management
- `internal/output/` - Output formatting
- `internal/model/` - Shared domain types

Query approach: builds a unified Record model by joining ENA + assembly + checkm2 parquet files upfront, then filters in memory. Uses a lookup index (SQLite) for faster repeated queries.

**Verdict:** Codex has a more sophisticated architecture with better separation of concerns and interfaces. Claude has a simpler, more direct approach that's easier to understand but less extensible.

---

## Feature Comparison

### Commands

| Command | atb-cli-claude | atb-cli-codex |
|---------|---------------|---------------|
| `query` | Full filter flags + TOML file | Full filter flags + TOML file |
| `info` | All tables, formatted sections | `--sample` flag, optional ENA |
| `download` | Parallel HTTP with resume/retry | AWS + OSF-tarball strategy |
| `fetch` | Core/all tables from OSF | Metadata + AMR, auto-discovery |
| `summarise`/`stats` | Default report + group-by + `--from` | Per-species/genus/QC breakdown |
| `config` | init/set/get/show | Not present (env vars + flags) |
| `update` (data) | Not fully implemented | Refresh cache, show diff |
| `update` (binary) | Self-update from GitHub | Not present |
| `amr` | Not present | First-class AMR query command |
| `mlst` | Not present | First-class MLST query command |
| `version` | Full version info | Not present |

### Unique to atb-cli-codex
- **AMR as a first-class command** (`atb amr --species "E. coli"`) - queries genus-partitioned AMR parquet files from the atb-amr-shiny repo
- **MLST command** (`atb mlst --species "E. coli"`) - returns sequence type info
- **Even sampling** (`--sample-strategy even --limit 100`) - deterministic sampling across sequence types with configurable seed
- **Download strategy selection** (`--strategy auto|aws|osf-tarball`) - automatically chooses AWS for small sets, OSF tarballs for large sets
- **OSF tarball extraction** - downloads tar.xz archives and extracts specific genomes
- **`--print-command`** - prints equivalent curl commands instead of downloading
- **`--emit-query-toml`** / `--save-query` - reproducibility features for saving/loading exact query specs
- **Auto-discovery** - dynamically discovers parquet files from OSF API and AMR files from GitHub API (no hardcoded URLs)
- **SQLite lookup index** - builds an indexed cache for faster repeated queries
- **Typed Record model** - strongly typed domain model with JSON/TOML serialization
- **Shell completion** - registered completion functions for flag values

### Unique to atb-cli-claude
- **Self-update** (`atb update`) - checks GitHub releases, replaces binary in-place
- **Background version check** - non-blocking 24h check, shows notice on next run
- **Interactive DB prompt** - when database is missing, offers to download with a nice UI
- **Fuzzy species suggestion** - Levenshtein distance matching for typos
- **Result sorting** (`--sort-by N50 --sort-desc`) - sort by any column with numeric detection
- **Disk space checking** - verifies free space before downloads, human-readable sizes
- **Download resume** - HTTP Range headers + .part files for interrupted downloads
- **Download retry** - exponential backoff on 429/5xx
- **Download manifest** - writes manifest.json tracking what was downloaded
- **Multi-format output** - TSV/CSV/JSON/table with auto-detection (table for TTY, TSV for pipe)
- **TOML config file** - persistent config at OS-standard paths
- **Wildcard species search** (`--species-like "Salmonella%"`)
- **Comprehensive README** - usage examples, benchmarks, install instructions
- **CI/CD pipeline** - GitHub Actions with 3-OS test matrix + GoReleaser
- **One-line install** - `curl | bash` installer
- **Benchmark suite** - reproducible performance benchmarks

---

## Query Filters

| Filter | atb-cli-claude | atb-cli-codex |
|--------|---------------|---------------|
| `--species` | Exact, case-insensitive | Exact, case-insensitive |
| `--species-like` | Wildcard with % | Not present |
| `--genus` | Derived from species | Not present (uses species) |
| `--sample` / `--sample-id` | Comma-separated list | Single ID |
| `--sample-file` | File with accessions | `--input` (CSV/TSV/plain text) |
| `--hq-only` | hq_filter == "PASS" | completeness >= 90 && contamination <= 5 |
| `--min-completeness` | CheckM2 threshold | `--checkm2-min` |
| `--max-contamination` | CheckM2 threshold | `--checkm2-max-contamination` |
| `--min-n50` | Assembly stats join | Not present |
| `--dataset` | Filter by release batch | Not present |
| `--country` | ENA join | Not present (ENA only in info) |
| `--platform` | ENA join | Not present |
| `--collection-date-from/to` | Date range | Not present |
| `--sequence-type` | Not present | MLST sequence type filter |
| `--sample-strategy` | Not present | `all` or `even` sampling |
| `--seed` | Not present | Deterministic sampling seed |
| `--limit` | Post-filter limit | Post-filter limit |
| `--offset` | Skip rows | Not present |
| `--sort-by` / `--sort-desc` | Any column, numeric-aware | Not present |

---

## Download System

| Feature | atb-cli-claude | atb-cli-codex |
|---------|---------------|---------------|
| Parallel downloads | Yes (configurable workers) | Sequential |
| Resume support | .part files + HTTP Range | No |
| Retry with backoff | Yes (3 attempts) | No |
| Disk space check | Yes | No |
| AWS direct | Yes (default) | Yes (for <= 5 samples) |
| OSF tarball extraction | No | Yes (for > 5 samples) |
| Strategy selection | No (always AWS) | auto/aws/osf-tarball |
| Print commands | No | `--print-command` outputs curl lines |
| Dry run | `--dry-run` (list URLs) | `--dry-run` (JSON plan) |
| Download manifest | manifest.json | No |
| Progress reporting | To stderr | No |

**Verdict:** Claude is better for reliability (resume, retry, parallelism, disk checks). Codex is better for large-scale downloads (OSF tarball batching saves bandwidth when downloading many genomes from the same archive).

---

## Data Source Discovery

| Aspect | atb-cli-claude | atb-cli-codex |
|--------|---------------|---------------|
| Metadata URLs | Hardcoded (10 OSF download links) | Dynamic (OSF API traversal) |
| AMR data | Not supported | GitHub API tree traversal |
| Assembly manifest | Not supported | Downloads file_list.all.latest.tsv.gz |
| Version tracking | metadata.json (local) | state.json with source URLs |
| Cache reuse | Skip if file exists | Skip if file exists + size check |

**Verdict:** Codex's auto-discovery is more robust and future-proof - it won't break when a new ATB version is released. Claude's hardcoded URLs are simpler but need manual updates.

---

## Testing

| Metric | atb-cli-claude | atb-cli-codex |
|--------|---------------|---------------|
| Test files | 15 | 7 |
| Test lines | ~1,500+ | ~1,166 |
| Test fixtures | Python-generated parquet (20 rows) | In-memory test helpers |
| CLI integration tests | Yes (7 tests) | Yes (~180 lines) |
| Query logic tests | Yes (table-driven) | Yes (table-driven) |
| Download tests | Mock HTTP server | Mock HTTP server |
| Source discovery tests | N/A | Mock OSF/GitHub API |
| Coverage (estimated) | ~60% | ~55% |
| Race detector | CI runs with `-race` | Not specified |

---

## Distribution & UX

| Aspect | atb-cli-claude | atb-cli-codex |
|--------|---------------|---------------|
| Install methods | curl \| bash, go install, source, manual | Source only (build.sh) |
| CI/CD | GitHub Actions (3 OS) | None |
| Release automation | GoReleaser (6 binaries + checksums) | Manual build.sh |
| README | 300+ lines, examples, benchmarks | 1 line |
| AGENTS.md / design doc | Design spec + implementation plan | Detailed AGENTS.md (427 lines) |
| Help text examples | Basic --help | Rich examples in every command |
| Error messages | Actionable with suggestions | Actionable with "run this command" hints |
| Config persistence | TOML file at OS-standard path | None (flags + env vars only) |
| Missing DB prompt | Interactive box with options | "Run `atb fetch --metadata` first" |

---

## Performance

Benchmarked head-to-head on the same machine: Linux x86_64, 8 cores, 15 GB RAM, 3,227,665 genomes.
Each test run 3 times (2 for stats). Same parquet files, both warmed up (codex SQLite index pre-built).

### Binary size

| Tool | Size |
|------|------|
| atb-cli-claude | 17 MB |
| atb-cli-codex | 24 MB |

Codex is 41% larger, mostly due to the SQLite dependency (modernc.org/sqlite).

### Species query (E. coli, HQ, limit 100)

| Tool | Time (avg) | Peak RAM | CPU |
|------|-----------|----------|-----|
| **claude** | **7.3s** | 2,128 MB | 114% |
| **codex** | **7.1s** | 1,016 MB | 111% |

Nearly identical speed. Codex uses **52% less RAM** thanks to its SQLite index which avoids loading the full assembly.parquet into memory.

### Species query (S. aureus, HQ, limit 100)

| Tool | Time (avg) | Peak RAM | CPU |
|------|-----------|----------|-----|
| **claude** | **6.4s** | 1,531 MB | 119% |
| **codex** | **3.4s** | 339 MB | 109% |

Codex is **1.9x faster** and uses **78% less RAM**. The SQLite index allows targeted lookup without scanning the full parquet file.

### Single sample lookup (atb info SAMD00000355)

| Tool | Time (avg) | Peak RAM | CPU |
|------|-----------|----------|-----|
| **claude** | **39.5s** | 2,174 MB | 105% |
| **codex** | **2.6s** | 44 MB | 101% |

Codex is **15x faster** and uses **98% less RAM**. This is where the SQLite index pays off the most - claude must scan multiple full parquet files to find one sample, while codex does an indexed lookup.

### Query with QC join (E. coli, completeness >= 99, limit 100)

| Tool | Time (avg) | Peak RAM | CPU |
|------|-----------|----------|-----|
| **claude** | **10.9s** | 2,176 MB | 131% |
| **codex** | **7.1s** | 1,136 MB | 113% |

Codex is **1.5x faster** with **48% less RAM**. Claude loads both assembly + checkm2 tables; codex's pre-joined index already has completeness data.

### Full database summary (stats/summarise)

| Tool | Time (avg) | Peak RAM | CPU |
|------|-----------|----------|-----|
| **claude** | **15.9s** | 3,620 MB | 178% |
| **codex** | **47.6s** | 6,915 MB | 114% |

Claude is **3x faster** with **48% less RAM**. This reverses the trend - for a full-database scan, claude's lazy parquet reading is more efficient than codex's approach of loading all ENA records into memory to build the full record set.

### Summary table

| Operation | Claude time | Codex time | Winner | Claude RAM | Codex RAM | Winner |
|-----------|-----------|-----------|--------|-----------|-----------|--------|
| E. coli query | 7.3s | 7.1s | Tie | 2.1 GB | 1.0 GB | Codex |
| S. aureus query | 6.4s | 3.4s | Codex | 1.5 GB | 339 MB | Codex |
| Single sample info | 39.5s | 2.6s | Codex | 2.2 GB | 44 MB | Codex |
| QC join query | 10.9s | 7.1s | Codex | 2.2 GB | 1.1 GB | Codex |
| Full DB summary | 15.9s | 47.6s | Claude | 3.6 GB | 6.9 GB | Claude |

### Analysis

Codex wins most query benchmarks decisively, especially single-sample lookups (15x faster). Its SQLite index strategy trades one-time index-building cost for dramatically faster queries and lower memory usage.

Claude wins the full-database summary because its lazy parquet reader only loads what's needed, while codex's approach of building a full ENA-based record set is expensive when you need all 3.5M rows.

**Key takeaway:** The SQLite index is the single biggest performance differentiator. If claude adopted a similar indexing strategy, it would match or beat codex across the board while keeping its advantages in download reliability and user experience.

---

## Dependencies

| Dependency | atb-cli-claude | atb-cli-codex |
|------------|---------------|---------------|
| parquet-go | v0.29.0 (latest) | v0.25.1 (older) |
| cobra | v1.10.2 | v1.8.1 |
| BurntSushi/toml | v1.6.0 | v1.4.0 |
| tablewriter | v1.1.4 | Not used |
| progressbar | v3 (in go.mod) | Not used |
| x/term | Yes (TTY detection) | Not used |
| x/sys | Yes (Windows disk space) | Not used |
| ulikunitz/xz | Not used | Yes (tar.xz extraction) |
| modernc.org/sqlite | Not used | Yes (lookup index) |
| Go version | 1.25 | 1.26.1 |

---

## Strengths Summary

### atb-cli-claude excels at:
- Distribution and installation (curl | bash, GoReleaser, CI/CD)
- User experience (interactive prompts, fuzzy suggestions, auto-format)
- Download reliability (resume, retry, parallelism, disk checks)
- Documentation (README, benchmarks, design docs)
- Self-maintenance (auto-update, background version check)
- Broad query filters (N50, country, platform, date range, wildcard species)

### atb-cli-codex excels at:
- Biological domain modeling (AMR, MLST as first-class concepts)
- Data source discovery (dynamic OSF/GitHub API traversal)
- Download strategy intelligence (AWS vs OSF-tarball auto-selection)
- Reproducibility (save/load/emit query TOML)
- Sampling semantics (deterministic even sampling across STs)
- Architecture quality (interfaces, dependency injection, testability)
- Help text design (rich examples in every command)
- OSF tarball extraction for bulk downloads

---

## What Each Could Learn From the Other

### atb-cli-claude should adopt from codex:
1. AMR and MLST as first-class commands
2. OSF tarball extraction for bulk downloads
3. `--emit-query-toml` / `--save-query` for reproducibility
4. `--print-command` to output download scripts
5. Dynamic source discovery via OSF/GitHub APIs
6. Even sampling with deterministic seed
7. SQLite lookup index for faster repeated queries

### atb-cli-codex should adopt from claude:
1. CI/CD pipeline (GitHub Actions + GoReleaser)
2. One-line install script
3. Self-update mechanism
4. Download parallelism, resume, and retry
5. Disk space checking
6. Comprehensive README with examples and benchmarks
7. Interactive missing-database prompt
8. Fuzzy species name suggestion
9. N50/country/platform/date query filters
10. Result sorting
