# AMR Query Performance Optimization

## Problem

`atb amr` reads all 25.6M rows from the monolithic `amrfinderplus.parquet` (81 MB)
into memory, then filters in Go. A typical Escherichia query takes ~2.5 minutes.

**Root cause:** `ReadFiltered` calls `ReadAll` first (deserializes every row into a
Go struct), then applies the predicate. For a genus that represents 35% of the data,
we still pay 100% of the deserialization cost.

### Data profile

| Genus | Rows | % of total |
|---|---|---|
| Escherichia | 9,085,983 | 35.5% |
| Salmonella | 6,169,753 | 24.1% |
| Staphylococcus | 3,098,867 | 12.1% |
| Klebsiella | 2,264,731 | 8.8% |
| Other (2,420 genera) | 4,980,286 | 19.5% |
| **Total** | **25,599,620** | |

File structure: 25 row groups of ~1M rows each, 12 columns, no row-group-level
statistics exploitable for predicate pushdown on genus.

## Proposed design

Two-layer optimization, each independent and incrementally shippable.

### Layer 1: Streaming filter with early termination

**What:** Add `ReadStreamFiltered[T]` to `internal/parquet/reader.go` that applies
the predicate during deserialization (not after) and accepts an optional max-results
limit for early exit.

**Why:** Avoids allocating ~25.6M `AMRRow` structs (each ~200 bytes, totalling ~5 GB
of heap pressure) just to throw most away.

**Expected gain:** ~2x faster for large result sets (halves allocation), much faster
with a small `--limit` (early exit).

**Changes:**
- `internal/parquet/reader.go` — new `ReadStreamFiltered` function
- `internal/amr/amr.go` — switch from `ReadFiltered` to `ReadStreamFiltered`
- Existing `ReadFiltered` callers are unaffected (function stays, just not used by AMR)

### Layer 2: Genus-partitioned parquet files

**What:** After `atb fetch` downloads the monolithic `amrfinderplus.parquet`, a
post-processing step reads it once and writes genus-partitioned files:

```
data/amr/Escherichia.parquet    (~28 MB, 9.1M rows)
data/amr/Salmonella.parquet     (~19 MB, 6.2M rows)
data/amr/Staphylococcus.parquet (~10 MB, 3.1M rows)
...
```

When `amr.Query` has a genus filter (always — `--species` is required), it reads
only the relevant partition. Falls back to full-file scan if partitions don't exist
(backward-compatible).

**Why:** Eliminates 65-96% of I/O depending on genus. Escherichia (worst case)
reads ~28 MB instead of 81 MB, and deserializes 9.1M rows instead of 25.6M.

**Expected gain:** ~3-5x on its own. Combined with Layer 1, Escherichia queries
should drop from ~2.5min to ~15-30s. Smaller genera under 5 seconds.

**Changes:**
- `internal/amr/partition.go` — new file: `BuildPartitions(dataDir)` reads the
  monolithic file, groups by genus, writes per-genus parquet files to `amr/` subdir
- `internal/amr/amr.go` — `Query` checks for `amr/<Genus>.parquet` first, falls
  back to `amrfinderplus.parquet`
- `internal/cli/fetch_cmd.go` — call `amr.BuildPartitions` after download (alongside
  index build)
- `internal/amr/partition_test.go` — tests for partition creation and fallback

### Interaction between layers

| Scenario | Layer 1 alone | Layer 2 alone | Both |
|---|---|---|---|
| Escherichia, no limit | ~1.5min | ~45s | ~15-30s |
| Escherichia, limit 100 | ~5s | ~30s | ~2-5s |
| Staphylococcus, no limit | ~1.5min | ~15s | ~5-10s |
| No genus (internal/fallback) | ~1.5min | ~2.5min | ~1.5min |

### Test strategy

- Unit tests for `ReadStreamFiltered` (predicate correctness, limit behavior, EOF)
- Unit tests for `BuildPartitions` (correct genus grouping, file creation)
- Existing `amr_test.go` tests pass unchanged (they use fixture data with the
  monolithic file layout)
- Add integration test that partitions fixture data and queries via partition path

### Migration / backward compatibility

- Monolithic `amrfinderplus.parquet` stays in place (not deleted)
- Partitions are additive — old installs without partitions still work
- `atb fetch --force` re-downloads and re-partitions
- Partition directory: `<data-dir>/amr/` (alongside existing parquet files)

### Risks

- **Partition build time:** Reading 81 MB and writing ~2,400 genus files adds ~1-2min
  to `atb fetch`. Acceptable since fetch is infrequent.
- **Disk usage:** Partitioned files add ~81 MB (roughly same total size as original).
  Total disk for AMR goes from 81 MB to ~162 MB.
- **parquet-go writer:** Need to verify `parquet-go` supports writing parquet files
  (it does — `parquetgo.NewGenericWriter`). If writing is problematic, Layer 1 alone
  still provides meaningful improvement.
