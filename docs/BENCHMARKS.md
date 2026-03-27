# Benchmarks

Performance benchmarks for `atb-cli`, measured on Linux x86_64 (Intel Xeon Platinum 8275CL @ 3.00GHz, 8 cores, 15 GB RAM).

Database: 3,227,665 genomes, 25,599,620 AMR rows across 2,420 genera.

Reproduce with:

```bash
# Go benchmarks (synthetic data)
go test ./internal/parquet/ -bench=. -benchmem -count=1
go test ./internal/amr/ -bench=. -benchmem -count=1

# Real-data benchmarks (requires atb fetch first)
bash benchmark.sh
```

## Metadata queries (SQLite index)

The SQLite index is built automatically during `atb fetch`. All `atb query` commands use it.

| Operation | Time | Peak RAM |
|-----------|------|----------|
| Single sample info (`atb info`) | <10ms | 14 MB |
| Species query (E. coli, HQ, limit 100) | <10ms | 15 MB |
| Species query (S. aureus, HQ, limit 100) | <10ms | 17 MB |
| Species + completeness + N50 filter | <10ms | 15 MB |

### Improvement over raw parquet scan

| Operation | Before (parquet) | After (SQLite) | Speedup | RAM reduction |
|-----------|-----------------|----------------|---------|---------------|
| Single sample info | 39.5s / 2.2 GB | <10ms / 14 MB | ~4,000x | 99.4% |
| E. coli species query | 7.3s / 2.1 GB | <10ms / 15 MB | ~700x | 99.3% |
| S. aureus species query | 6.4s / 1.5 GB | <10ms / 17 MB | ~600x | 98.9% |
| QC join query | 10.9s / 2.2 GB | <10ms / 15 MB | ~1,000x | 99.3% |

### ENA queries (parquet scan, no index)

Queries involving ENA metadata (country, platform, collection date) still use parquet scanning because ENA data is not included in the index.

| Query | Time | Peak RAM |
|-------|------|----------|
| Species + country filter | ~35s | 2.4 GB |
| Species + country + platform + dates | ~42s | 2.7 GB |

## AMR queries

AMR queries go through a three-tier lookup: SQLite index (fastest), parquet partition (fast), monolithic file (slowest). The tier is selected automatically based on what's available.

### Query tiers compared (500K synthetic rows, Escherichia = 60%)

| Query | Monolithic | Parquet partition | SQLite index | Best speedup |
|-------|-----------|------------------|-------------|-------------|
| Full genus scan (no limit) | 1,057 ms | 710 ms | 1,156 ms | 1.5x (parquet) |
| Genus + limit 100 | 2.3 ms | 2.2 ms | **0.71 ms** | **3.2x** |
| Genus + class filter + limit 100 | 2.4 ms | - | **1.0 ms** | **2.4x** |
| Genus + gene pattern + limit 100 | 3.7 ms | - | **1.7 ms** | **2.2x** |
| Multi-genus + limit 100 | 2.4 ms | - | **0.70 ms** | **3.4x** |

### Query tiers compared (1M synthetic rows, Escherichia = 60%)

| Query | Monolithic | Parquet partition | SQLite index | Best speedup |
|-------|-----------|------------------|-------------|-------------|
| Full genus scan (no limit) | 1,857 ms | 1,371 ms | 2,334 ms | 1.4x (parquet) |
| Genus + limit 100 | 2.4 ms | - | **0.71 ms** | **3.4x** |

### Memory usage

| Query | Monolithic | Parquet partition | SQLite index |
|-------|-----------|------------------|-------------|
| Full scan (500K) | 664 MB | 624 MB | 428 MB |
| Full scan (1M) | 1,442 MB | 1,358 MB | 847 MB |
| Limit 100 (500K) | 3.3 MB | 3.3 MB | **0.11 MB** |
| Limit 100 (1M) | 3.3 MB | - | **0.11 MB** |

### Performance by tier

**SQLite index** is best for filtered queries (limit, class, gene pattern, multi-genus). It uses SQL WHERE clauses with indexes, returning only matching rows. Memory usage is minimal (~110 KB for limited queries).

**Parquet partition** is best for full genus scans without limit. It reads only the relevant genus file (e.g., 28 MB for Escherichia instead of 81 MB for the full file), but must deserialize every row.

**Monolithic fallback** is used when no partitions exist (backward compatibility) or for cross-genus queries without a species filter.

### Projected performance on real data (25.6M rows)

| Scenario | Estimated time | Notes |
|----------|---------------|-------|
| E. coli + limit 100 (SQLite) | <10ms | SQL query on indexed genus file |
| E. coli full scan (parquet) | ~15-30s | Streams 9.1M rows from partition |
| E. coli full scan (SQLite) | ~20-40s | Reads all 9.1M rows from SQLite |
| Staphylococcus + limit 100 (SQLite) | <10ms | Same for any genus |
| Cross-genus gene search + limit 100 | ~2ms | Streaming parquet with early exit |

## Parquet reader internals

Comparison of the two parquet reading strategies.

### ReadFiltered (old: read all, then filter)

| Dataset | Time/op | Memory/op | Allocs/op |
|---------|---------|-----------|-----------|
| 500K rows | 592 ms | 305 MB | 5.0M |
| 1M rows | 1,228 ms | 605 MB | 10.0M |

### ReadStreamFiltered (new: filter during read)

| Dataset | Limit | Time/op | Memory/op | Allocs/op | vs ReadFiltered |
|---------|-------|---------|-----------|-----------|----------------|
| 500K | none | 539 ms | 218 MB | 5.0M | 1.1x faster, 28% less RAM |
| 500K | 100 | **2.1 ms** | **3.2 MB** | 8.2K | **282x faster, 99% less RAM** |
| 1M | none | 1,032 ms | 434 MB | 10.0M | 1.2x faster, 28% less RAM |
| 1M | 100 | **2.4 ms** | **3.3 MB** | 8.2K | **512x faster, 99.5% less RAM** |

## Download performance

Genome FASTA files from AWS S3 (parallel=4):

| Files | Time | Peak RAM | Total size | Throughput |
|-------|------|----------|------------|------------|
| 10 | 2.2s | 18 MB | 16 MB | 7 MB/s |
| 20 | 2.5s | 18 MB | 31 MB | 13 MB/s |
| 30 | 3.1s | 18 MB | 46 MB | 15 MB/s |
| 40 | 3.2s | 18 MB | 62 MB | 20 MB/s |
| 50 | 3.8s | 18 MB | 77 MB | 21 MB/s |

Downloads are memory-efficient (18 MB flat). Average genome assembly is ~1.5 MB compressed.

## Build costs

### Post-fetch index build

| Step | Time |
|------|------|
| AMR genus partitioning (25.6M rows) | ~1-2 min |
| AMR SQLite index build (parallel, 8 workers) | ~3-5 min |
| Read assembly.parquet (3.2M rows) + insert | ~2 min |
| Merge assembly_stats (2.8M rows) | ~1 min |
| Merge checkm2 (2.8M rows) | ~1 min |
| Merge mlst (2.4M rows) | ~30s |
| **Total** | **~8-11 min** |

### Disk usage

| Component | Size |
|-----------|------|
| Core metadata parquet (7 tables) | 700 MB |
| ENA parquet (5 tables, optional) | 2.5 GB |
| Metadata SQLite index | 1.2 GB |
| AMR parquet partitions | ~81 MB |
| AMR SQLite indexes | ~1.5 GB |
| **Typical install (core + all indexes)** | **~3.5 GB** |

## Environment

```
goos: linux
goarch: amd64
cpu: Intel(R) Xeon(R) Platinum 8275CL CPU @ 3.00GHz
go: 1.25.0
sqlite: modernc.org/sqlite v1.47.0
parquet: github.com/parquet-go/parquet-go v0.29.0
```
