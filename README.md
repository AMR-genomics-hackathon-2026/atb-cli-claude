# atb-cli

A command-line tool for querying the [AllTheBacteria](https://osf.io/xv7q9/) genomics database (~3.2M bacterial genomes) and downloading genome assemblies.

Single binary, no dependencies. Runs on Linux and macOS (amd64/arm64).

## Quick Start

```bash
# 1. Build (or download a release binary)
make build

# 2. Point to your parquet database
./bin/atb config init
./bin/atb config set general.data_dir ~/atb/metadata/parquet

# 3. Query
./bin/atb query --species "Escherichia coli" --hq-only --limit 10
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

Querying 3.2M rows on a typical machine:

| Operation | Time |
|-----------|------|
| Species filter (assembly only) | ~7s |
| Species + QC + N50 (3-table join) | ~14s |
| Sample info (all tables) | ~40s |
| Genus filter | ~8s |

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
