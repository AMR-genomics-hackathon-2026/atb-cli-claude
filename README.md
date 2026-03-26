# atb-cli

A command-line tool for querying the [AllTheBacteria](https://osf.io/xv7q9/) genomics database (~3.2M bacterial genomes), searching AMR/stress/virulence genes, and downloading genome assemblies.

Single binary, no dependencies.

**Supported platforms:** Linux, macOS, Windows (amd64 and arm64)

## Download

Pre-built binaries for all platforms:

| Platform | Architecture | File |
|----------|-------------|------|
| Linux | x86_64 (amd64) | `atb-cli_<version>_linux_amd64.tar.gz` |
| Linux | ARM64 | `atb-cli_<version>_linux_arm64.tar.gz` |
| macOS | Intel (amd64) | `atb-cli_<version>_darwin_amd64.tar.gz` |
| macOS | Apple Silicon (arm64) | `atb-cli_<version>_darwin_arm64.tar.gz` |
| Windows | x86_64 (amd64) | `atb-cli_<version>_windows_amd64.zip` |
| Windows | ARM64 | `atb-cli_<version>_windows_arm64.zip` |

**Latest release:** [github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/latest](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/latest)

Download the file for your platform, extract, and place the `atb` binary (or `atb.exe` on Windows) somewhere in your `PATH`.

## Install

**One-line install** (Linux/macOS):

```bash
curl -fsSL https://raw.githubusercontent.com/AMR-genomics-hackathon-2026/atb-cli-claude/main/install.sh | bash
```

This detects your OS and architecture, downloads the latest release, and installs to `/usr/local/bin`.

```bash
# Install a specific version
ATB_VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/AMR-genomics-hackathon-2026/atb-cli-claude/main/install.sh | bash

# Install to a custom directory
ATB_INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/AMR-genomics-hackathon-2026/atb-cli-claude/main/install.sh | bash
```

**Windows:** Download the `.zip` from the [Download](#download) table above, extract, and add `atb.exe` to your PATH.

**Other methods:**

```bash
# Go install (requires Go 1.23+)
go install github.com/AMR-genomics-hackathon-2026/atb-cli-claude/cmd/atb@latest

# From source
git clone https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude.git
cd atb-cli-claude
make build    # binary at ./bin/atb
```

## Quick Start

```bash
# 1. Download the database (~540 MB core tables)
atb fetch

# 2. Query
atb query --species "Escherichia coli" --hq-only --limit 10
```

If you don't have the parquet files yet:

```bash
# Download core tables from OSF (~540MB)
./bin/atb fetch

# Or download all tables including ENA metadata (~3GB)
./bin/atb fetch --all
```

## Updating

`atb` checks for new versions automatically in the background (once every 24 hours). If a newer release is found, you'll see a notice:

```
  A new version of atb is available: v0.2.0 (current: v0.1.0)
  Run 'atb update' to upgrade.
```

To update:

```bash
# Interactive update (asks for confirmation)
atb update

# Non-interactive (for scripts/CI)
atb update --force
```

The updater downloads the correct binary for your OS/architecture from GitHub Releases and replaces the current binary in place.

## Usage Examples

### Query genomes by species

```bash
# Get 10 high-quality E. coli genomes
atb query --species "Escherichia coli" --hq-only --limit 10

# With quality filters
atb query --species "Escherichia coli" \
  --hq-only \
  --min-completeness 99.5 \
  --max-contamination 0.5 \
  --min-n50 200000 \
  --sort-by N50 --sort-desc \
  --limit 20

# Select specific columns
atb query --species "Escherichia coli" --hq-only --limit 5 \
  --columns sample_accession,sylph_species,N50,Completeness_General,aws_url

# Search by genus
atb query --genus Salmonella --hq-only --limit 20

# Wildcard species search
atb query --species-like "Streptococcus%" --hq-only --limit 10
```

### Filter by geography and platform (requires ENA tables)

```bash
# Salmonella from the UK, Illumina only
atb query --species "Salmonella enterica" \
  --country "United Kingdom" \
  --platform "ILLUMINA" \
  --limit 20

# Genomes collected between 2020-2023
atb query --species "Escherichia coli" \
  --collection-date-from 2020-01-01 \
  --collection-date-to 2023-12-31 \
  --limit 50
```

### Use a TOML filter file (reproducible queries)

```bash
# Create a filter file
cat > my_query.toml <<'EOF'
[filter]
species = "Escherichia coli"
hq_only = true
min_completeness = 99.0
max_contamination = 2.0
min_n50 = 100000

[output]
columns = ["sample_accession", "sylph_species", "N50", "Completeness_General", "aws_url"]
sort_by = "N50"
sort_desc = true
limit = 100
format = "tsv"
output = "ecoli_results.tsv"
EOF

# Run the query
atb query --filter my_query.toml

# CLI flags override TOML values
atb query --filter my_query.toml --limit 10
```

### Get sample details

```bash
atb info SAMD00000355
```

Output:
```
=== Assembly ===
  sample_accession:   SAMD00000355
  sylph_species:      Streptococcus pyogenes
  hq_filter:          PASS
  dataset:            661k
  aws_url:            https://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/SAMD00000355.fa.gz

=== Assembly Stats ===
  total_length: 1868526
  N50:          148451

=== CheckM2 Quality ===
  completeness_general:  99.06
  contamination:         0.03

=== ENA Metadata ===
  country:             Japan:Aichi
  collection_date:     1994
  instrument_platform: ILLUMINA
```

### Download genome assemblies

```bash
# Query, then download
atb query --species "Klebsiella pneumoniae" --hq-only --limit 10 \
  --columns sample_accession,aws_url -o results.tsv
atb download --from results.tsv --output-dir ./genomes

# Pipe query directly to download
atb query --species "Escherichia coli" --hq-only --limit 5 \
  --columns sample_accession,aws_url --format csv | \
  atb download --from - --output-dir ./ecoli_genomes

# Preview what would be downloaded
atb download --from results.tsv --dry-run

# Download from a URL list
atb download --urls my_urls.txt --output-dir ./genomes --parallel 8

# Download a single file
atb download --url https://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/SAMD00000355.fa.gz \
  --output-dir ./genomes
```

### Summary statistics

```bash
# Default summary of the full database
atb summarise

# Group by species (top 20)
atb summarise --by sylph_species --top 20

# Summarise a previous query result
atb query --genus Salmonella --hq-only --limit 100 \
  --columns sample_accession,sylph_species,hq_filter,dataset -o salmonella.tsv
atb summarise --from salmonella.tsv

# Pipe query to summarise
atb query --species "Escherichia coli" --hq-only --limit 200 \
  --columns sample_accession,sylph_species,dataset --format csv | \
  atb summarise --from -
```

### Query AMR genes

AMR data comes from [AMRFinderPlus](https://github.com/ncbi/amr) results run across all ATB genomes. Data is organized by genus and includes three categories: AMR resistance genes, stress response genes, and virulence factors.

```bash
# First, fetch AMR data for the genus you need
atb fetch --amr --genus Escherichia

# Get all AMR gene hits for E. coli (high-quality genomes only)
atb amr --species "Escherichia coli" --hq-only --limit 100

# Filter by drug class
atb amr --species "Escherichia coli" --hq-only --class "BETA-LACTAM"

# Wildcard gene search (all beta-lactamase genes)
atb amr --species "Escherichia coli" --gene "bla%"

# Filter by detection quality
atb amr --species "Escherichia coli" --min-coverage 95 --min-identity 98

# Query stress response genes
atb amr --species "Escherichia coli" --type stress

# Query virulence factors
atb amr --species "Escherichia coli" --type virulence

# Query all three categories at once
atb amr --species "Escherichia coli" --type all

# Output to file
atb amr --species "Klebsiella pneumoniae" --hq-only --format csv -o kpn_amr.csv

# Fetch AMR data for multiple genera
atb fetch --amr --genus Escherichia,Salmonella,Klebsiella

# Fetch all three element types
atb fetch --amr --genus Escherichia --amr-types amr,stress,virulence
```

AMR output columns: `sample_accession`, `gene_symbol`, `element_type`, `element_subtype`, `class`, `subclass`, `method`, `coverage`, `identity`, `species`

### Fetch the database

```bash
# Download core metadata tables (~540MB: assembly, assembly_stats, checkm2, sylph, run)
atb fetch

# Download all metadata tables including ENA (~3GB)
atb fetch --all

# Download specific metadata tables
atb fetch --tables ena_20250506.parquet

# Download AMR gene data by genus
atb fetch --amr --genus Escherichia
atb fetch --amr --genus Salmonella,Klebsiella,Pseudomonas

# Download AMR + stress + virulence data
atb fetch --amr --genus Escherichia --amr-types amr,stress,virulence

# Force re-download
atb fetch --force
```

### Configuration

```bash
# Create default config
atb config init

# View config
atb config show

# Set data directory
atb config set general.data_dir /path/to/parquet/files

# Set default download parallelism
atb config set download.parallel 8
```

Config is stored at `~/.config/atb/config.toml`.

## LLM Integration (MCP)

`atb` includes a built-in [Model Context Protocol](https://modelcontextprotocol.io) server, allowing LLMs to query the AllTheBacteria database directly through natural language.

**Two transport modes:**
- **stdio** (default) - for Claude Code, Claude Desktop, Cursor, VS Code Copilot, Windsurf, OpenAI Codex CLI
- **HTTP/SSE** (`--http :8080`) - for ChatGPT, OpenAI Responses API, remote clients

**Tools exposed:**

| Tool | Description |
|------|-------------|
| `atb_query` | Search genomes by species, genus, quality, N50 |
| `atb_amr` | Query AMR resistance genes by species and drug class |
| `atb_info` | Get full metadata for a specific sample |
| `atb_stats` | Database summary statistics |
| `atb_species_list` | List available species with genome counts |

### Quick try (no install needed, requires Go)

```bash
# Claude Code - runs directly from GitHub, no install
claude mcp add atb -- go run github.com/AMR-genomics-hackathon-2026/atb-cli-claude/cmd/atb@latest mcp
```

First call takes ~10s to compile; cached after that.

### Setup

```bash
# 1. Install atb
curl -fsSL https://raw.githubusercontent.com/AMR-genomics-hackathon-2026/atb-cli-claude/main/install.sh | bash

# 2. Fetch the database and build the index
atb fetch
```

### stdio mode (Claude, Cursor, Codex CLI)

```bash
# Claude Code
claude mcp add atb -- atb mcp

# Claude Desktop (add to claude_desktop_config.json)
{
  "mcpServers": {
    "atb": {
      "command": "atb",
      "args": ["mcp"]
    }
  }
}

# Cursor (Settings > MCP Servers > Add)
# Command: atb mcp

# OpenAI Codex CLI (~/.codex/config.toml)
[mcp_servers.atb]
command = "atb"
args = ["mcp"]
```

### HTTP/SSE mode (ChatGPT, OpenAI API, remote clients)

```bash
# Start the HTTP/SSE server
atb mcp --http :8080
```

Then configure your client with the SSE endpoint URL:
- **ChatGPT:** Settings > Connected apps > Add MCP server > `http://your-host:8080/sse`
- **OpenAI Responses API:** Use `server_url: "http://your-host:8080/sse"` in the MCP tool config

For public access, use a reverse proxy (nginx/caddy) or tunnel (ngrok):
```bash
# Quick public URL with ngrok
ngrok http 8080
# Use the ngrok URL as your SSE endpoint
```

> **Note:** If your data is in a non-default location, add `--data-dir /your/path` to all commands above.

### What you can ask your LLM

Once connected, you can ask natural language questions like:

- "How many Salmonella genomes are in the database?"
- "Find me 20 high-quality E. coli genomes with N50 > 200000"
- "What beta-lactam resistance genes does Klebsiella pneumoniae have?"
- "Show me all metadata for sample SAMD00000355"
- "What are the top 10 species by genome count?"

The LLM will call the appropriate `atb` tools and interpret the results for you.

## Output Formats

By default, output is a pretty table when writing to a terminal, and TSV when piped. Override with `--format`:

```bash
atb query --species "E. coli" --limit 5 --format tsv     # tab-separated
atb query --species "E. coli" --limit 5 --format csv     # comma-separated
atb query --species "E. coli" --limit 5 --format json    # JSON array
atb query --species "E. coli" --limit 5 --format table   # pretty table
```

## Available Columns

### assembly.parquet (always loaded)
`sample_accession`, `run_accession`, `assembly_accession`, `sylph_species`, `scientific_name`, `hq_filter`, `dataset`, `asm_fasta_on_osf`, `aws_url`, `osf_tarball_url`

### assembly_stats.parquet (loaded when N50/length columns requested)
`total_length`, `number`, `mean_length`, `longest`, `shortest`, `N50`, `N90`

### checkm2.parquet (loaded when quality columns requested)
`Completeness_General`, `Contamination`, `Completeness_Specific`, `Genome_Size`, `GC_Content`

### ena_20250506.parquet (loaded when geography/platform columns requested)
`country`, `collection_date`, `instrument_platform`, `instrument_model`, `read_count`, `base_count`, `library_strategy`, `study_accession`, `fastq_ftp`

## Performance

Benchmarked on Linux x86_64, 8 cores, 15 GB RAM. Database: 3,227,665 genomes.

Run `bash benchmark.sh` to reproduce on your machine.

### With SQLite index (default after `atb fetch`)

The index is built automatically during `atb fetch` (one-time cost: ~4 minutes, produces a 1.2 GB SQLite file). All subsequent queries use the index.

| Operation | Time | Peak RAM |
|-----------|------|----------|
| Single sample info (`atb info`) | **<10ms** | **14 MB** |
| Species query (E. coli, HQ, limit 100) | **<10ms** | **15 MB** |
| Species query (S. aureus, HQ, limit 100) | **<10ms** | **17 MB** |
| Species + completeness + N50 filter | **<10ms** | **15 MB** |

Queries are effectively instant. The SQLite index pre-joins assembly, assembly_stats, and checkm2 data into one indexed table, so no parquet scanning is needed.

### Improvement over parquet-only scan

| Operation | Before (parquet) | After (SQLite) | Speedup | RAM reduction |
|-----------|-----------------|----------------|---------|---------------|
| Single sample info | 39.5s / 2.2 GB | <10ms / 14 MB | **~4,000x** | **99.4%** |
| E. coli species query | 7.3s / 2.1 GB | <10ms / 15 MB | **~700x** | **99.3%** |
| S. aureus species query | 6.4s / 1.5 GB | <10ms / 17 MB | **~600x** | **98.9%** |
| QC join query | 10.9s / 2.2 GB | <10ms / 15 MB | **~1,000x** | **99.3%** |

### ENA queries (parquet scan, no index)

Queries involving ENA metadata (country, platform, collection date) still use parquet scanning because ENA data is not included in the index (it would add 856 MB and slow index builds significantly).

| Query | Time | Peak RAM |
|-------|------|----------|
| Species + country filter | ~35s | 2.4 GB |
| Species + country + platform + dates | ~42s | 2.7 GB |

### Download (genome FASTA from AWS S3, parallel=4)

| Files | Time | Peak RAM | Total size | Throughput |
|-------|------|----------|------------|------------|
| 10 | 2.2s | 18 MB | 16 MB | 7 MB/s |
| 20 | 2.5s | 18 MB | 31 MB | 13 MB/s |
| 30 | 3.1s | 18 MB | 46 MB | 15 MB/s |
| 40 | 3.2s | 18 MB | 62 MB | 20 MB/s |
| 50 | 3.8s | 18 MB | 77 MB | 21 MB/s |

Downloads are memory-efficient (18 MB flat). Average genome assembly is ~1.5 MB compressed.

### Index build cost

| Step | Time |
|------|------|
| Read assembly.parquet (3.2M rows) + insert | ~2 min |
| Read assembly_stats.parquet (2.8M rows) + update | ~1 min |
| Read checkm2.parquet (2.8M rows) + update | ~1 min |
| **Total** | **~4 min** |

Index file size: 1.2 GB. Built once, used for all subsequent queries. Rebuild with `atb index --force`.

### Disk usage

| Component | Size |
|-----------|------|
| Core metadata parquet (5 tables) | 540 MB |
| ENA parquet (5 tables, optional) | 2.5 GB |
| SQLite index | 1.2 GB |
| AMR data (per genus, e.g. Escherichia) | ~19 MB |
| **Typical install (core + index)** | **~1.7 GB** |

### Notes on species names

The database uses GTDB taxonomy (not NCBI). Some species names differ from common usage. If a query returns 0 results, the tool suggests close matches. Example: *Enterococcus faecium* in GTDB may be *Enterococcus_B faecium*. Use `--species-like "Enterococcus%faecium"` to search across GTDB naming variants.

## Building

```bash
# Build for current platform
make build

# Run tests
make test

# Cross-compile for all supported platforms
GOOS=linux   GOARCH=amd64 go build -o bin/atb-linux-amd64   ./cmd/atb
GOOS=linux   GOARCH=arm64 go build -o bin/atb-linux-arm64   ./cmd/atb
GOOS=darwin  GOARCH=amd64 go build -o bin/atb-darwin-amd64  ./cmd/atb
GOOS=darwin  GOARCH=arm64 go build -o bin/atb-darwin-arm64  ./cmd/atb
GOOS=windows GOARCH=amd64 go build -o bin/atb-windows-amd64.exe ./cmd/atb
GOOS=windows GOARCH=arm64 go build -o bin/atb-windows-arm64.exe ./cmd/atb
```

Requires Go 1.23+. Pure Go, no CGO - cross-compilation works out of the box.

## Data Sources

**Metadata** (assembly, QC, ENA): [AllTheBacteria](https://allthebacteria.org) project on [OSF (h7wzy)](https://osf.io/h7wzy/files/osfstorage), path: `Aggregated/Latest_2025-05/atb.metadata.202505.parquet/`

**AMR/Stress/Virulence genes**: [AMRFinderPlus](https://github.com/ncbi/amr) results from the [atb-amr-shiny](https://github.com/immem-hackathon-2025/atb-amr-shiny) project, Hive-partitioned by genus under `data/amr_by_genus/`, `data/stress_by_genus/`, `data/virulence_by_genus/`

## License

[MIT](LICENSE)
