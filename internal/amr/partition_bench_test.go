package amr_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	parquetgo "github.com/parquet-go/parquet-go"

	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/amr"
	pq "github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/parquet"
)

// setupBenchData generates a synthetic monolithic parquet file, partitions it,
// and returns the data directory. The genus distribution mimics real data:
// one large genus (60%), one medium (25%), several small (15% total).
func setupBenchData(b *testing.B, nRows int) string {
	b.Helper()
	dir := b.TempDir()
	path := filepath.Join(dir, amr.AMRFileName)
	f, err := os.Create(path)
	if err != nil {
		b.Fatalf("create: %v", err)
	}

	w := parquetgo.NewGenericWriter[pq.AMRRow](f)

	// Distribution: Escherichia 60%, Salmonella 25%, rest 15% across 3 small genera
	type genDist struct {
		genus string
		pct   float64
	}
	dist := []genDist{
		{"Escherichia", 0.60},
		{"Salmonella", 0.25},
		{"Staphylococcus", 0.06},
		{"Klebsiella", 0.05},
		{"Pseudomonas", 0.04},
	}

	buf := make([]pq.AMRRow, 0, 1000)
	var gi int
	for _, d := range dist {
		count := int(float64(nRows) * d.pct)
		for i := 0; i < count; i++ {
			row := pq.AMRRow{
				Name:           fmt.Sprintf("SAMN%08d", gi),
				GeneSymbol:     fmt.Sprintf("gene_%d", gi%200),
				HierarchyNode:  "node",
				ElementType:    "AMR",
				ElementSubtype: "subtype",
				Coverage:       99.0,
				Identity:       99.5,
				Method:         "BLAST",
				Class:          "BETA-LACTAM",
				Subclass:       "sub",
				Species:        d.genus + " sp.",
				Genus:          d.genus,
			}
			buf = append(buf, row)
			if len(buf) == 1000 {
				if _, err := w.Write(buf); err != nil {
					b.Fatal(err)
				}
				buf = buf[:0]
			}
			gi++
		}
	}
	if len(buf) > 0 {
		if _, err := w.Write(buf); err != nil {
			b.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		b.Fatal(err)
	}
	if err := f.Close(); err != nil {
		b.Fatal(err)
	}

	if err := amr.BuildPartitions(dir, nil); err != nil {
		b.Fatalf("BuildPartitions: %v", err)
	}

	return dir
}

// BenchmarkQuery_Monolithic_500K queries the monolithic file (no partitions).
func BenchmarkQuery_Monolithic_500K(b *testing.B) {
	dir := setupBenchData(b, 500_000)
	// Remove partitions so Query falls back to monolithic
	os.RemoveAll(filepath.Join(dir, amr.PartitionDir))

	b.ResetTimer()
	for b.Loop() {
		results, err := amr.Query(dir, amr.Filters{Genus: "Escherichia"})
		if err != nil {
			b.Fatal(err)
		}
		if len(results) == 0 {
			b.Fatal("no results")
		}
	}
}

// BenchmarkQuery_Partitioned_500K queries the genus partition file.
func BenchmarkQuery_Partitioned_500K(b *testing.B) {
	dir := setupBenchData(b, 500_000)

	b.ResetTimer()
	for b.Loop() {
		results, err := amr.Query(dir, amr.Filters{Genus: "Escherichia"})
		if err != nil {
			b.Fatal(err)
		}
		if len(results) == 0 {
			b.Fatal("no results")
		}
	}
}

// BenchmarkQuery_Monolithic_500K_Limit100 queries monolithic with limit.
func BenchmarkQuery_Monolithic_500K_Limit100(b *testing.B) {
	dir := setupBenchData(b, 500_000)
	os.RemoveAll(filepath.Join(dir, amr.PartitionDir))

	b.ResetTimer()
	for b.Loop() {
		results, err := amr.Query(dir, amr.Filters{Genus: "Escherichia", Limit: 100})
		if err != nil {
			b.Fatal(err)
		}
		if len(results) != 100 {
			b.Fatalf("expected 100, got %d", len(results))
		}
	}
}

// BenchmarkQuery_Partitioned_500K_Limit100 queries partition with limit.
func BenchmarkQuery_Partitioned_500K_Limit100(b *testing.B) {
	dir := setupBenchData(b, 500_000)

	b.ResetTimer()
	for b.Loop() {
		results, err := amr.Query(dir, amr.Filters{Genus: "Escherichia", Limit: 100})
		if err != nil {
			b.Fatal(err)
		}
		if len(results) != 100 {
			b.Fatalf("expected 100, got %d", len(results))
		}
	}
}

// BenchmarkQuery_Monolithic_1M queries 1M rows from monolithic file.
func BenchmarkQuery_Monolithic_1M(b *testing.B) {
	dir := setupBenchData(b, 1_000_000)
	os.RemoveAll(filepath.Join(dir, amr.PartitionDir))

	b.ResetTimer()
	for b.Loop() {
		results, err := amr.Query(dir, amr.Filters{Genus: "Escherichia"})
		if err != nil {
			b.Fatal(err)
		}
		if len(results) == 0 {
			b.Fatal("no results")
		}
	}
}

// BenchmarkQuery_Partitioned_1M queries 1M rows from partition file.
func BenchmarkQuery_Partitioned_1M(b *testing.B) {
	dir := setupBenchData(b, 1_000_000)

	b.ResetTimer()
	for b.Loop() {
		results, err := amr.Query(dir, amr.Filters{Genus: "Escherichia"})
		if err != nil {
			b.Fatal(err)
		}
		if len(results) == 0 {
			b.Fatal("no results")
		}
	}
}
