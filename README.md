# atb-cli

A command-line tool for querying the [AllTheBacteria](https://osf.io/xv7q9/) genomics database (~3.2M bacterial genomes) and downloading genome assemblies.

Single binary, no dependencies. Runs on Linux, macOS, and Windows (amd64/arm64).

## Install

**One-line install** (Linux/macOS):

```bash
curl -fsSL https://raw.githubusercontent.com/AMR-genomics-hackathon-2026/atb-cli-claude/main/install.sh | bash
```

This detects your OS and architecture, downloads the latest release binary, and installs it to `/usr/local/bin`.

**Options:**

```bash
# Install a specific version
ATB_VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/AMR-genomics-hackathon-2026/atb-cli-claude/main/install.sh | bash

# Install to a custom directory
ATB_INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/AMR-genomics-hackathon-2026/atb-cli-claude/main/install.sh | bash
```

**Other install methods:**

```bash
# Go install
go install github.com/AMR-genomics-hackathon-2026/atb-cli-claude/cmd/atb@latest

# From source
git clone https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude.git
cd atb-cli-claude
make build
./bin/atb --help

# Manual download
# Visit https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases
# Download the archive for your platform, extract, and move `atb` to your PATH
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

### Fetch the database

```bash
# Download core tables (~540MB: assembly, assembly_stats, checkm2, sylph, run)
atb fetch

# Download all tables including ENA metadata (~3GB)
atb fetch --all

# Download specific tables
atb fetch --tables ena_20250506.parquet

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

Run `bash benchmark.sh` to reproduce these results on your machine.

### Query (single table - assembly.parquet only)

| Limit | Wall time | Peak RAM | CPU |
|-------|-----------|----------|-----|
| 10 | 7.1s | 2.1 GB | 117% |
| 20 | 7.1s | 2.1 GB | 116% |
| 30 | 7.1s | 2.1 GB | 116% |
| 40 | 7.1s | 2.1 GB | 116% |
| 50 | 7.1s | 2.1 GB | 116% |

Query time is constant regardless of `--limit` because the full parquet file is scanned and filtered, then the limit is applied. RAM usage is ~2.1 GB (the assembly.parquet file decompressed in memory).

### Query with joins (assembly + checkm2 + assembly_stats)

| Limit | Wall time | Peak RAM | CPU |
|-------|-----------|----------|-----|
| 10 | 15.9s | 2.2 GB | 126% |
| 20 | 15.9s | 2.2 GB | 125% |
| 30 | 16.0s | 2.2 GB | 126% |
| 40 | 15.9s | 2.1 GB | 127% |
| 50 | 15.8s | 2.2 GB | 125% |

Adding QC and assembly stats joins roughly doubles query time. RAM stays near 2.2 GB.

### Download (genome FASTA from AWS S3, parallel=4)

| Files | Wall time | Peak RAM | Total size | Throughput |
|-------|-----------|----------|------------|------------|
| 10 | 2.2s | 18 MB | 16 MB | 7 MB/s |
| 20 | 2.5s | 18 MB | 31 MB | 13 MB/s |
| 30 | 3.1s | 18 MB | 46 MB | 15 MB/s |
| 40 | 3.2s | 18 MB | 62 MB | 20 MB/s |
| 50 | 3.8s | 18 MB | 77 MB | 21 MB/s |

Downloads are memory-efficient (18 MB flat) and throughput scales with parallelism. Average genome assembly is ~1.5 MB compressed.

### Query by species (single table)

| Species | Time | Peak RAM | CPU |
|---------|------|----------|-----|
| Escherichia coli | 7.2s | 2.1 GB | 116% |
| Staphylococcus aureus | 6.5s | 1.5 GB | 118% |
| Salmonella enterica | 7.9s | 2.2 GB | 134% |
| Klebsiella pneumoniae | 6.3s | 1.5 GB | 119% |
| Pseudomonas aeruginosa | 6.3s | 1.4 GB | 119% |
| Mycobacterium tuberculosis | 6.5s | 1.7 GB | 118% |
| Streptococcus pneumoniae | 6.4s | 1.6 GB | 118% |
| Acinetobacter baumannii | 6.2s | 1.4 GB | 118% |
| Clostridioides difficile | 6.2s | 1.4 GB | 119% |

RAM varies from 1.4-2.2 GB depending on how much of the parquet file is scanned. Rare species with fewer row groups to read use less memory. CPU is ~118% (parquet decompression is the bottleneck, uses ~1.2 cores).

### Query by genus

| Genus | Time | Peak RAM | CPU |
|-------|------|----------|-----|
| Salmonella | 7.9s | 2.2 GB | 137% |
| Streptococcus | 6.8s | 1.9 GB | 118% |
| Staphylococcus | 6.6s | 1.7 GB | 118% |
| Escherichia | 7.3s | 2.2 GB | 121% |
| Mycobacterium | 6.7s | 1.8 GB | 118% |

### Multi-table join cost

| Query | Time | Peak RAM | Tables joined |
|-------|------|----------|---------------|
| Species only | 7s | 2.1 GB | assembly |
| + CheckM2 completeness | 11s | 2.2 GB | + checkm2 |
| + N50 assembly stats | 11s | 2.1 GB | + assembly_stats |
| + CheckM2 + N50 | 14s | 2.2 GB | + both |
| + ENA country filter | 35s | 2.4 GB | + run + ena |
| All filters combined | 42s | 2.7 GB | all 4 tables |

ENA joins are the most expensive (+25s, +300 MB) because `ena_20250506.parquet` is 856 MB. Use ENA filters only when you need geographic or platform metadata.

### Notes on species names

The database uses GTDB taxonomy (not NCBI). Some species names differ from common usage. If a query returns 0 results, the tool suggests close matches. Example: *Enterococcus faecium* in GTDB may be *Enterococcus_B faecium*. Use `--species-like "Enterococcus%faecium"` to search across GTDB naming variants.

## Building

```bash
# Build
make build

# Run tests
make test

# Cross-compile check
GOOS=darwin GOARCH=arm64 go build -o /dev/null ./cmd/atb
```

Requires Go 1.22+.

## Data Source

The parquet files are from the [AllTheBacteria](https://allthebacteria.org) project, hosted at [OSF (h7wzy)](https://osf.io/h7wzy/files/osfstorage), path: `Aggregated/Latest_2025-05/atb.metadata.202505.parquet/`.

## License

TBD
