#!/usr/bin/env bash
set -euo pipefail

ATB="./bin/atb --data-dir $HOME/atb/metadata/parquet"
DOWNLOAD_DIR="/tmp/atb-bench-downloads"
LIMITS=(10 20 30 40 50)

echo "============================================"
echo "atb-cli Benchmark — $(date -Iseconds)"
echo "============================================"
echo ""

# System info
echo "System: $(uname -s) $(uname -m)"
echo "CPU: $(nproc) cores"
echo "RAM: $(free -h | awk '/Mem:/{print $2}') total"
echo ""

# ─── QUERY BENCHMARKS ───────────────────────────────────────────────

echo "┌──────────────────────────────────────────────────────────────┐"
echo "│  QUERY BENCHMARKS (species filter only — assembly.parquet)  │"
echo "└──────────────────────────────────────────────────────────────┘"
echo ""
printf "%-8s  %-10s  %-12s  %-12s\n" "LIMIT" "TIME(s)" "PEAK_RSS(MB)" "CPU(%)"
printf "%-8s  %-10s  %-12s  %-12s\n" "-----" "-------" "-----------" "------"

for N in "${LIMITS[@]}"; do
    result=$( /usr/bin/time -v $ATB query \
        --species "Escherichia coli" --hq-only \
        --limit "$N" \
        --columns sample_accession,sylph_species,dataset \
        --format tsv -o /dev/null 2>&1 )

    wall=$(echo "$result" | grep "Elapsed (wall clock)" | awk '{print $NF}')
    rss=$(echo "$result" | grep "Maximum resident set size" | awk '{print $NF}')
    cpu=$(echo "$result" | grep "Percent of CPU" | awk '{print $NF}')
    rss_mb=$(echo "scale=1; $rss / 1024" | bc)

    printf "%-8s  %-10s  %-12s  %-12s\n" "$N" "$wall" "$rss_mb" "$cpu"
done

echo ""
echo "┌──────────────────────────────────────────────────────────────┐"
echo "│  QUERY + JOIN BENCHMARKS (assembly + checkm2 + stats)       │"
echo "└──────────────────────────────────────────────────────────────┘"
echo ""
printf "%-8s  %-10s  %-12s  %-12s\n" "LIMIT" "TIME(s)" "PEAK_RSS(MB)" "CPU(%)"
printf "%-8s  %-10s  %-12s  %-12s\n" "-----" "-------" "-----------" "------"

for N in "${LIMITS[@]}"; do
    result=$( /usr/bin/time -v $ATB query \
        --species "Escherichia coli" --hq-only \
        --min-completeness 99.0 --max-contamination 2.0 --min-n50 100000 \
        --limit "$N" --sort-by N50 --sort-desc \
        --columns sample_accession,sylph_species,N50,Completeness_General,Contamination \
        --format tsv -o /dev/null 2>&1 )

    wall=$(echo "$result" | grep "Elapsed (wall clock)" | awk '{print $NF}')
    rss=$(echo "$result" | grep "Maximum resident set size" | awk '{print $NF}')
    cpu=$(echo "$result" | grep "Percent of CPU" | awk '{print $NF}')
    rss_mb=$(echo "scale=1; $rss / 1024" | bc)

    printf "%-8s  %-10s  %-12s  %-12s\n" "$N" "$wall" "$rss_mb" "$cpu"
done

# ─── DOWNLOAD BENCHMARKS ────────────────────────────────────────────

echo ""
echo "┌──────────────────────────────────────────────────────────────┐"
echo "│  DOWNLOAD BENCHMARKS (actual genome FASTA from AWS S3)      │"
echo "└──────────────────────────────────────────────────────────────┘"
echo ""

# First, generate query results with URLs for download
$ATB query \
    --species "Escherichia coli" --hq-only --has-assembly \
    --limit 50 \
    --columns sample_accession,aws_url \
    --format csv -o /tmp/atb-bench-urls.csv 2>/dev/null

printf "%-8s  %-10s  %-12s  %-12s  %-12s\n" "FILES" "TIME(s)" "PEAK_RSS(MB)" "TOTAL_SIZE" "SPEED"
printf "%-8s  %-10s  %-12s  %-12s  %-12s\n" "-----" "-------" "-----------" "----------" "-----"

for N in "${LIMITS[@]}"; do
    rm -rf "$DOWNLOAD_DIR"
    mkdir -p "$DOWNLOAD_DIR"

    # Take first N+1 lines (header + N data rows)
    head -n $((N + 1)) /tmp/atb-bench-urls.csv > /tmp/atb-bench-urls-n.csv

    result=$( /usr/bin/time -v $ATB download \
        --from /tmp/atb-bench-urls-n.csv \
        --output-dir "$DOWNLOAD_DIR" \
        --parallel 4 2>&1 )

    wall=$(echo "$result" | grep "Elapsed (wall clock)" | awk '{print $NF}')
    rss=$(echo "$result" | grep "Maximum resident set size" | awk '{print $NF}')
    rss_mb=$(echo "scale=1; $rss / 1024" | bc)

    # Calculate total downloaded size
    total_bytes=$(du -sb "$DOWNLOAD_DIR" 2>/dev/null | awk '{print $1}')
    if [ -z "$total_bytes" ] || [ "$total_bytes" = "0" ]; then
        total_size="0 B"
        speed="n/a"
    else
        total_size=$(numfmt --to=iec-i --suffix=B "$total_bytes" 2>/dev/null || echo "${total_bytes}B")
        # Parse wall time to seconds for speed calc
        if [[ "$wall" == *:* ]]; then
            mins=$(echo "$wall" | cut -d: -f1)
            secs=$(echo "$wall" | cut -d: -f2)
            total_secs=$(echo "$mins * 60 + $secs" | bc)
        else
            total_secs="$wall"
        fi
        if [ "$(echo "$total_secs > 0" | bc)" = "1" ]; then
            bps=$(echo "scale=0; $total_bytes / $total_secs" | bc)
            speed=$(numfmt --to=iec-i --suffix=B/s "$bps" 2>/dev/null || echo "${bps}B/s")
        else
            speed="n/a"
        fi
    fi

    printf "%-8s  %-10s  %-12s  %-12s  %-12s\n" "$N" "$wall" "$rss_mb" "$total_size" "$speed"
done

# Cleanup
rm -rf "$DOWNLOAD_DIR" /tmp/atb-bench-urls.csv /tmp/atb-bench-urls-n.csv

echo ""
echo "Done."
