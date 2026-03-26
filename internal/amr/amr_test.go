package amr_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/amr"
)

func fixturesDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file path")
	}
	return filepath.Join(filepath.Dir(file), "testdata", "fixtures")
}

func TestReadGenusParts(t *testing.T) {
	genusDir := filepath.Join(fixturesDir(t), "amr", "amr_by_genus", "Genus=Escherichia")
	rows, err := amr.ReadGenusParts(genusDir)
	if err != nil {
		t.Fatalf("ReadGenusParts: %v", err)
	}
	if len(rows) != 10 {
		t.Errorf("expected 10 rows, got %d", len(rows))
	}
	// Verify first row has expected columns populated
	first := rows[0]
	if first.Name == "" {
		t.Error("expected non-empty Name")
	}
	if first.GeneSymbol == "" {
		t.Error("expected non-empty GeneSymbol")
	}
	if first.Class == "" {
		t.Error("expected non-empty Class")
	}
}

func TestQueryAMRByGenus(t *testing.T) {
	results, err := amr.Query(fixturesDir(t), "Escherichia", "AMR", amr.Filters{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results, got none")
	}
	for _, r := range results {
		if r.SampleAccession == "" {
			t.Error("expected non-empty SampleAccession")
		}
	}
}

func TestQueryAMRFilterBySamples(t *testing.T) {
	wanted := map[string]struct{}{
		"SAMN00000001": {},
		"SAMN00000002": {},
	}
	results, err := amr.Query(fixturesDir(t), "Escherichia", "AMR", amr.Filters{
		Samples: wanted,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	for _, r := range results {
		if _, ok := wanted[r.SampleAccession]; !ok {
			t.Errorf("unexpected sample %q in results", r.SampleAccession)
		}
	}
	if len(results) == 0 {
		t.Error("expected at least one result for filtered samples")
	}
}

func TestQueryAMRFilterByClass(t *testing.T) {
	results, err := amr.Query(fixturesDir(t), "Escherichia", "AMR", amr.Filters{
		Class: "EFFLUX",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results for EFFLUX class")
	}
	for _, r := range results {
		if r.Class != "EFFLUX" {
			t.Errorf("expected Class=EFFLUX, got %q", r.Class)
		}
	}
}

func TestQueryAMRFilterByGenePattern(t *testing.T) {
	// Pattern "bla%" should match blaTEM-1, blaEC, blaOXA-1, blaCTX-M-15
	results, err := amr.Query(fixturesDir(t), "Escherichia", "AMR", amr.Filters{
		GenePattern: "bla%",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results matching bla% pattern")
	}
	for _, r := range results {
		gene := r.GeneSymbol
		if len(gene) < 3 || gene[:3] != "bla" {
			t.Errorf("gene %q does not match bla%% pattern", gene)
		}
	}
}

func TestQueryAMRFilterByMinCoverage(t *testing.T) {
	results, err := amr.Query(fixturesDir(t), "Escherichia", "AMR", amr.Filters{
		MinCoverage: 99.0,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	for _, r := range results {
		if r.Coverage < 99.0 {
			t.Errorf("expected coverage >= 99.0, got %v for gene %q", r.Coverage, r.GeneSymbol)
		}
	}
}

func TestQueryStress(t *testing.T) {
	results, err := amr.Query(fixturesDir(t), "Escherichia", "STRESS", amr.Filters{})
	if err != nil {
		t.Fatalf("Query stress: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("expected 5 stress results, got %d", len(results))
	}
	for _, r := range results {
		if r.ElementType != "STRESS" {
			t.Errorf("expected ElementType=STRESS, got %q", r.ElementType)
		}
	}
}

func TestQueryAll(t *testing.T) {
	results, err := amr.Query(fixturesDir(t), "Escherichia", "all", amr.Filters{})
	if err != nil {
		t.Fatalf("Query all: %v", err)
	}
	// 10 AMR + 5 stress = 15 (virulence dir doesn't exist; ReadGenusParts returns 0 rows for missing dir)
	if len(results) < 10 {
		t.Errorf("expected at least 10 results from 'all' query, got %d", len(results))
	}
}
