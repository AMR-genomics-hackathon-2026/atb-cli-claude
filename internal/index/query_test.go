package index

import (
	"os"
	"path/filepath"
	"testing"
)

// buildTestIndex creates a fresh index from test fixtures in a temp dir.
func buildTestIndex(t *testing.T) *DB {
	t.Helper()

	dir := t.TempDir()
	fixtures := "../../testdata/fixtures"
	for _, name := range []string{"assembly.parquet", "assembly_stats.parquet", "checkm2.parquet", "mlst.parquet"} {
		data, err := os.ReadFile(filepath.Join(fixtures, name))
		if err != nil {
			t.Fatalf("reading fixture %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatalf("writing fixture %s: %v", name, err)
		}
	}

	if err := Build(dir, func(string, ...any) {}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestQuerySpecies(t *testing.T) {
	db := buildTestIndex(t)

	rows, err := db.Query(QueryParams{Species: "Escherichia coli"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 5 {
		t.Errorf("expected 5 E. coli rows, got %d", len(rows))
	}
	for _, r := range rows {
		if r["sylph_species"] != "Escherichia coli" {
			t.Errorf("unexpected species %q", r["sylph_species"])
		}
	}
}

func TestQueryHQOnly(t *testing.T) {
	db := buildTestIndex(t)

	rows, err := db.Query(QueryParams{HQOnly: true})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 17 {
		t.Errorf("expected 17 HQ rows, got %d", len(rows))
	}
	for _, r := range rows {
		if r["hq_filter"] != "PASS" {
			t.Errorf("row %s has hq_filter=%q, expected PASS", r["sample_accession"], r["hq_filter"])
		}
	}
}

func TestQueryWithCompleteness(t *testing.T) {
	db := buildTestIndex(t)

	// completeness >= 99: SAMN00000001(99.5), SAMN00000002(99.2), SAMN00000006(99.0),
	// SAMN00000011(99.3), SAMN00000016(99.1), SAMN00000017(99.0), SAMN00000019(99.4) = 7
	rows, err := db.Query(QueryParams{MinCompleteness: 99})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 7 {
		t.Errorf("expected 7 rows with completeness >= 99, got %d", len(rows))
	}
}

func TestQueryWithN50(t *testing.T) {
	db := buildTestIndex(t)

	// N50 >= 240000: SAMN00000002(245000), SAMN00000007(260000), SAMN00000009(260000),
	// SAMN00000010(300000), SAMN00000011(250000), SAMN00000013(300000),
	// SAMN00000016(350000), SAMN00000017(350000), SAMN00000019(240000) = 9
	rows, err := db.Query(QueryParams{MinN50: 240000})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 9 {
		t.Errorf("expected 9 rows with N50 >= 240000, got %d", len(rows))
	}
}

func TestInfoRow(t *testing.T) {
	db := buildTestIndex(t)

	row, err := db.InfoRow("SAMN00000001")
	if err != nil {
		t.Fatalf("InfoRow: %v", err)
	}

	if row["sylph_species"] != "Escherichia coli" {
		t.Errorf("expected sylph_species=%q, got %q", "Escherichia coli", row["sylph_species"])
	}
	if row["hq_filter"] != "PASS" {
		t.Errorf("expected hq_filter=PASS, got %q", row["hq_filter"])
	}
}

func TestInfoRowNotFound(t *testing.T) {
	db := buildTestIndex(t)

	_, err := db.InfoRow("NONEXISTENT_SAMPLE")
	if err == nil {
		t.Fatal("expected error for nonexistent sample, got nil")
	}
}

func TestQueryMLSTColumns(t *testing.T) {
	db := buildTestIndex(t)

	rows, err := db.Query(QueryParams{
		Species: "Escherichia coli",
		Columns: []string{"sample_accession", "mlst_scheme", "mlst_st", "mlst_status", "mlst_score"},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("expected 5 E. coli rows, got %d", len(rows))
	}
	for _, r := range rows {
		if r["mlst_scheme"] != "ecoli_achtman_4" {
			t.Errorf("sample %s: expected mlst_scheme=ecoli_achtman_4, got %q", r["sample_accession"], r["mlst_scheme"])
		}
		if r["mlst_st"] == "" {
			t.Errorf("sample %s: mlst_st is empty", r["sample_accession"])
		}
		if r["mlst_status"] == "" {
			t.Errorf("sample %s: mlst_status is empty", r["sample_accession"])
		}
	}
}

func TestQueryMLSTFilterByST(t *testing.T) {
	db := buildTestIndex(t)

	// ST131 E. coli: SAMN1, SAMN2, SAMN19 (3 samples)
	rows, err := db.Query(QueryParams{SequenceType: "131"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("expected 3 ST131 rows, got %d", len(rows))
	}
	for _, r := range rows {
		if r["mlst_st"] != "131" {
			t.Errorf("expected mlst_st=131, got %q", r["mlst_st"])
		}
	}
}

func TestQueryMLSTFilterByScheme(t *testing.T) {
	db := buildTestIndex(t)

	rows, err := db.Query(QueryParams{Scheme: "salmonella"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	// Salmonella enterica samples: SAMN5, SAMN6, SAMN18 = 3
	if len(rows) != 3 {
		t.Errorf("expected 3 salmonella scheme rows, got %d", len(rows))
	}
}

func TestQueryMLSTFilterByStatus(t *testing.T) {
	db := buildTestIndex(t)

	rows, err := db.Query(QueryParams{MLSTStatus: "NONE"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	// NONE: M. tuberculosis (SAMN16, SAMN17) + unknown (SAMN20) = 3
	if len(rows) != 3 {
		t.Errorf("expected 3 NONE status rows, got %d", len(rows))
	}
}

func TestMLSTForSample(t *testing.T) {
	db := buildTestIndex(t)

	mlst, err := db.MLSTForSample("SAMN00000001")
	if err != nil {
		t.Fatalf("MLSTForSample: %v", err)
	}
	if mlst == nil {
		t.Fatal("expected non-nil MLST result")
	}
	if mlst["mlst_scheme"] != "ecoli_achtman_4" {
		t.Errorf("expected mlst_scheme=ecoli_achtman_4, got %q", mlst["mlst_scheme"])
	}
}

func TestMLSTInInfoRow(t *testing.T) {
	db := buildTestIndex(t)

	row, err := db.InfoRow("SAMN00000001")
	if err != nil {
		t.Fatalf("InfoRow: %v", err)
	}
	if row["mlst_scheme"] != "ecoli_achtman_4" {
		t.Errorf("InfoRow: expected mlst_scheme=ecoli_achtman_4, got %q", row["mlst_scheme"])
	}
	if row["mlst_st"] == "" {
		t.Error("InfoRow: mlst_st is empty")
	}
}

func TestSpeciesCountList(t *testing.T) {
	db := buildTestIndex(t)

	counts, err := db.SpeciesCountList(0, false)
	if err != nil {
		t.Fatalf("SpeciesCountList: %v", err)
	}
	if len(counts) == 0 {
		t.Fatal("expected non-empty species counts")
	}

	// Results should be sorted descending by count.
	for i := 1; i < len(counts); i++ {
		if counts[i].Count > counts[i-1].Count {
			t.Errorf("results not sorted descending: counts[%d]=%d > counts[%d]=%d",
				i, counts[i].Count, i-1, counts[i-1].Count)
		}
	}

	// Verify E. coli appears with count 5.
	var ecoliCount int
	for _, sc := range counts {
		if sc.Species == "Escherichia coli" {
			ecoliCount = sc.Count
			break
		}
	}
	if ecoliCount != 5 {
		t.Errorf("expected 5 E. coli samples, got %d", ecoliCount)
	}
}

func TestSpeciesCountListHQOnly(t *testing.T) {
	db := buildTestIndex(t)

	allCounts, err := db.SpeciesCountList(0, false)
	if err != nil {
		t.Fatalf("SpeciesCountList(all): %v", err)
	}

	hqCounts, err := db.SpeciesCountList(0, true)
	if err != nil {
		t.Fatalf("SpeciesCountList(hq): %v", err)
	}

	// HQ total should be <= total.
	totalAll := 0
	for _, sc := range allCounts {
		totalAll += sc.Count
	}
	totalHQ := 0
	for _, sc := range hqCounts {
		totalHQ += sc.Count
	}
	if totalHQ > totalAll {
		t.Errorf("HQ total %d > all total %d", totalHQ, totalAll)
	}
}

func TestSpeciesCountListLimit(t *testing.T) {
	db := buildTestIndex(t)

	counts, err := db.SpeciesCountList(2, false)
	if err != nil {
		t.Fatalf("SpeciesCountList: %v", err)
	}
	if len(counts) > 2 {
		t.Errorf("expected at most 2 results with limit=2, got %d", len(counts))
	}
}
