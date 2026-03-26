package query

import (
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
