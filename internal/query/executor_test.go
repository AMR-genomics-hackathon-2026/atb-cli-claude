package query

import (
	"strconv"
	"testing"
)

const fixturesDir = "../../testdata/fixtures"

func TestExecuteSpeciesFilter(t *testing.T) {
	filters := Filters{Species: "Escherichia coli"}
	rows, err := Execute(fixturesDir, filters, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(rows) != 5 {
		t.Errorf("expected 5 results for E.coli, got %d", len(rows))
	}
}

func TestExecuteHQOnly(t *testing.T) {
	filters := Filters{HQOnly: true}
	rows, err := Execute(fixturesDir, filters, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(rows) != 17 {
		t.Errorf("expected 17 HQ results, got %d", len(rows))
	}
}

func TestExecuteSpeciesAndHQ(t *testing.T) {
	filters := Filters{Species: "Escherichia coli", HQOnly: true}
	rows, err := Execute(fixturesDir, filters, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(rows) != 5 {
		t.Errorf("expected 5 results for E.coli + HQ, got %d", len(rows))
	}
}

func TestExecuteWithCheckM2Join(t *testing.T) {
	filters := Filters{
		Species:         "Escherichia coli",
		HQOnly:          true,
		MinCompleteness: 99.0,
	}
	rows, err := Execute(fixturesDir, filters, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(rows) != 4 {
		t.Errorf("expected 4 results (SAMN12 excluded with completeness=97.0), got %d", len(rows))
	}
}

func TestExecuteWithN50Join(t *testing.T) {
	filters := Filters{
		Species: "Escherichia coli",
		MinN50:  240000,
	}
	rows, err := Execute(fixturesDir, filters, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("expected 3 results (SAMN2=245k, SAMN11=250k, SAMN19=240k), got %d", len(rows))
	}
}

func TestExecuteWithColumns(t *testing.T) {
	filters := Filters{Species: "Escherichia coli", HQOnly: true}
	cols := []string{"sample_accession", "sylph_species", "N50"}
	rows, err := Execute(fixturesDir, filters, cols)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(rows) != 5 {
		t.Errorf("expected 5 results, got %d", len(rows))
	}
	for _, row := range rows {
		for _, col := range cols {
			if _, ok := row[col]; !ok {
				t.Errorf("expected column %q in result row, but it was missing", col)
			}
		}
		// Ensure no extra columns
		if len(row) != len(cols) {
			t.Errorf("expected row to have exactly %d columns, got %d", len(cols), len(row))
		}
	}
}

func TestExecuteGenusFilter(t *testing.T) {
	filters := Filters{Genus: "Salmonella"}
	rows, err := Execute(fixturesDir, filters, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("expected 3 Salmonella results (SAMN5, SAMN6, SAMN18), got %d", len(rows))
	}
}

func TestExecuteDatasetFilter(t *testing.T) {
	filters := Filters{Dataset: "661k"}
	rows, err := Execute(fixturesDir, filters, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(rows) != 7 {
		t.Errorf("expected 7 results for dataset=661k (SAMN1-4, SAMN15-17), got %d", len(rows))
	}
}

func TestExecuteNoResults(t *testing.T) {
	filters := Filters{Species: "Nonexistent"}
	rows, err := Execute(fixturesDir, filters, nil)
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 results for nonexistent species, got %d", len(rows))
	}
}

func TestSortResultsNumeric(t *testing.T) {
	rows := []ResultRow{
		{"N50": "100000"},
		{"N50": "250000"},
		{"N50": "50000"},
	}
	SortResults(rows, "N50", true) // descending
	first, _ := strconv.ParseFloat(rows[0]["N50"], 64)
	last, _ := strconv.ParseFloat(rows[len(rows)-1]["N50"], 64)
	if first < last {
		t.Errorf("expected descending numeric sort: first=%v should be >= last=%v", first, last)
	}
}

func TestSortResultsString(t *testing.T) {
	rows := []ResultRow{
		{"sylph_species": "Staphylococcus aureus"},
		{"sylph_species": "Acinetobacter baumannii"},
		{"sylph_species": "Escherichia coli"},
	}
	SortResults(rows, "sylph_species", false) // ascending
	for i := 1; i < len(rows); i++ {
		if rows[i-1]["sylph_species"] > rows[i]["sylph_species"] {
			t.Errorf("expected ascending string sort at index %d: %q > %q",
				i, rows[i-1]["sylph_species"], rows[i]["sylph_species"])
		}
	}
}

func TestSortResultsEmpty(t *testing.T) {
	rows := []ResultRow{
		{"N50": "300000"},
		{"N50": "100000"},
	}
	original := []string{rows[0]["N50"], rows[1]["N50"]}
	SortResults(rows, "", false) // empty sortBy: no-op
	for i, row := range rows {
		if row["N50"] != original[i] {
			t.Errorf("expected no-op sort with empty sortBy: row %d changed from %q to %q",
				i, original[i], row["N50"])
		}
	}
}

func TestShuffleResultsDeterministic(t *testing.T) {
	makeRows := func() []ResultRow {
		rows := make([]ResultRow, 10)
		for i := range rows {
			rows[i] = ResultRow{"sample_accession": "SAMN" + strconv.Itoa(i)}
		}
		return rows
	}

	// Two shuffles with the same seed must produce the same order.
	r1 := makeRows()
	r2 := makeRows()
	ShuffleResults(r1, 42)
	ShuffleResults(r2, 42)
	for i := range r1 {
		if r1[i]["sample_accession"] != r2[i]["sample_accession"] {
			t.Errorf("shuffle with same seed gave different results at index %d: %q vs %q",
				i, r1[i]["sample_accession"], r2[i]["sample_accession"])
		}
	}
}

func TestShuffleResultsDifferentSeeds(t *testing.T) {
	makeRows := func() []ResultRow {
		rows := make([]ResultRow, 10)
		for i := range rows {
			rows[i] = ResultRow{"sample_accession": "SAMN" + strconv.Itoa(i)}
		}
		return rows
	}

	r1 := makeRows()
	r2 := makeRows()
	ShuffleResults(r1, 42)
	ShuffleResults(r2, 99)

	// Extremely unlikely to be identical by chance with 10 items.
	same := true
	for i := range r1 {
		if r1[i]["sample_accession"] != r2[i]["sample_accession"] {
			same = false
			break
		}
	}
	if same {
		t.Error("different seeds produced identical shuffle order (highly unlikely)")
	}
}

func TestShufflePreservesLength(t *testing.T) {
	rows := make([]ResultRow, 20)
	for i := range rows {
		rows[i] = ResultRow{"id": strconv.Itoa(i)}
	}
	ShuffleResults(rows, 1)
	if len(rows) != 20 {
		t.Errorf("shuffle changed row count: expected 20, got %d", len(rows))
	}
	// All original IDs must still be present.
	seen := make(map[string]bool)
	for _, r := range rows {
		seen[r["id"]] = true
	}
	for i := 0; i < 20; i++ {
		if !seen[strconv.Itoa(i)] {
			t.Errorf("id %d missing after shuffle", i)
		}
	}
}
