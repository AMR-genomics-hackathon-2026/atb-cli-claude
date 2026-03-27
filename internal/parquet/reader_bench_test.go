package parquet

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	parquetgo "github.com/parquet-go/parquet-go"
)

// generateBenchParquet creates a synthetic AMR parquet file with the given row
// count and genus distribution. Returns the file path and a cleanup function.
func generateBenchParquet(b *testing.B, nRows int) string {
	b.Helper()

	dir := b.TempDir()
	path := filepath.Join(dir, "bench_amr.parquet")

	f, err := os.Create(path)
	if err != nil {
		b.Fatalf("create bench file: %v", err)
	}

	w := parquetgo.NewGenericWriter[AMRRow](f)

	genera := []string{"Escherichia", "Salmonella", "Staphylococcus", "Klebsiella", "Pseudomonas"}
	types := []string{"AMR", "STRESS", "VIRULENCE"}
	classes := []string{"BETA-LACTAM", "AMINOGLYCOSIDE", "EFFLUX", "TETRACYCLINE"}

	buf := make([]AMRRow, 0, 1000)
	for i := 0; i < nRows; i++ {
		row := AMRRow{
			Name:           fmt.Sprintf("SAMN%08d", i),
			GeneSymbol:     fmt.Sprintf("gene_%d", i%200),
			HierarchyNode:  "node",
			ElementType:    types[i%len(types)],
			ElementSubtype: "subtype",
			Coverage:       95.0 + float64(i%50)/10.0,
			Identity:       98.0 + float64(i%20)/10.0,
			Method:         "BLAST",
			Class:          classes[i%len(classes)],
			Subclass:       "sub",
			Species:        genera[i%len(genera)] + " sp.",
			Genus:          genera[i%len(genera)],
		}
		buf = append(buf, row)
		if len(buf) == 1000 {
			if _, err := w.Write(buf); err != nil {
				b.Fatalf("write rows: %v", err)
			}
			buf = buf[:0]
		}
	}
	if len(buf) > 0 {
		if _, err := w.Write(buf); err != nil {
			b.Fatalf("write remaining rows: %v", err)
		}
	}

	if err := w.Close(); err != nil {
		b.Fatalf("close writer: %v", err)
	}
	if err := f.Close(); err != nil {
		b.Fatalf("close file: %v", err)
	}

	return path
}

// BenchmarkReadFiltered_500K benchmarks the old read-all-then-filter approach.
func BenchmarkReadFiltered_500K(b *testing.B) {
	path := generateBenchParquet(b, 500_000)
	predicate := func(row AMRRow) bool {
		return row.Genus == "Escherichia"
	}

	b.ResetTimer()
	for b.Loop() {
		rows, err := ReadFiltered[AMRRow](path, predicate)
		if err != nil {
			b.Fatal(err)
		}
		if len(rows) == 0 {
			b.Fatal("no results")
		}
	}
}

// BenchmarkReadStreamFiltered_500K benchmarks stream-filter (no limit).
func BenchmarkReadStreamFiltered_500K(b *testing.B) {
	path := generateBenchParquet(b, 500_000)
	predicate := func(row AMRRow) bool {
		return row.Genus == "Escherichia"
	}

	b.ResetTimer()
	for b.Loop() {
		rows, err := ReadStreamFiltered[AMRRow](path, predicate, 0)
		if err != nil {
			b.Fatal(err)
		}
		if len(rows) == 0 {
			b.Fatal("no results")
		}
	}
}

// BenchmarkReadStreamFiltered_500K_Limit100 benchmarks stream-filter with early exit.
func BenchmarkReadStreamFiltered_500K_Limit100(b *testing.B) {
	path := generateBenchParquet(b, 500_000)
	predicate := func(row AMRRow) bool {
		return row.Genus == "Escherichia"
	}

	b.ResetTimer()
	for b.Loop() {
		rows, err := ReadStreamFiltered[AMRRow](path, predicate, 100)
		if err != nil {
			b.Fatal(err)
		}
		if len(rows) != 100 {
			b.Fatalf("expected 100, got %d", len(rows))
		}
	}
}

// BenchmarkReadFiltered_1M benchmarks read-all-then-filter at 1M rows.
func BenchmarkReadFiltered_1M(b *testing.B) {
	path := generateBenchParquet(b, 1_000_000)
	predicate := func(row AMRRow) bool {
		return row.Genus == "Escherichia"
	}

	b.ResetTimer()
	for b.Loop() {
		rows, err := ReadFiltered[AMRRow](path, predicate)
		if err != nil {
			b.Fatal(err)
		}
		if len(rows) == 0 {
			b.Fatal("no results")
		}
	}
}

// BenchmarkReadStreamFiltered_1M benchmarks stream-filter at 1M rows.
func BenchmarkReadStreamFiltered_1M(b *testing.B) {
	path := generateBenchParquet(b, 1_000_000)
	predicate := func(row AMRRow) bool {
		return row.Genus == "Escherichia"
	}

	b.ResetTimer()
	for b.Loop() {
		rows, err := ReadStreamFiltered[AMRRow](path, predicate, 0)
		if err != nil {
			b.Fatal(err)
		}
		if len(rows) == 0 {
			b.Fatal("no results")
		}
	}
}

// BenchmarkReadStreamFiltered_1M_Limit100 benchmarks stream-filter with early exit at 1M rows.
func BenchmarkReadStreamFiltered_1M_Limit100(b *testing.B) {
	path := generateBenchParquet(b, 1_000_000)
	predicate := func(row AMRRow) bool {
		return row.Genus == "Escherichia"
	}

	b.ResetTimer()
	for b.Loop() {
		rows, err := ReadStreamFiltered[AMRRow](path, predicate, 100)
		if err != nil {
			b.Fatal(err)
		}
		if len(rows) != 100 {
			b.Fatalf("expected 100, got %d", len(rows))
		}
	}
}
