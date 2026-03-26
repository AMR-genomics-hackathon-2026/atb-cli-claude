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
	for _, name := range []string{"assembly.parquet", "assembly_stats.parquet", "checkm2.parquet"} {
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
