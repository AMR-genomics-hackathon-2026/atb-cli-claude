package parquet

import (
	"testing"
)

const fixturesDir = "../../testdata/fixtures"

func TestReadAssembly(t *testing.T) {
	rows, err := ReadAll[AssemblyRow](fixturesDir + "/assembly.parquet")
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(rows) != 20 {
		t.Errorf("expected 20 rows, got %d", len(rows))
	}
	if rows[0].SampleAccession != "SAMN00000001" {
		t.Errorf("expected first sample = SAMN00000001, got %q", rows[0].SampleAccession)
	}
	if rows[0].SylphSpecies != "Escherichia coli" {
		t.Errorf("expected first species = Escherichia coli, got %q", rows[0].SylphSpecies)
	}
}

func TestReadAssemblyStats(t *testing.T) {
	rows, err := ReadAll[AssemblyStatsRow](fixturesDir + "/assembly_stats.parquet")
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(rows) != 20 {
		t.Errorf("expected 20 rows, got %d", len(rows))
	}
	if rows[0].N50 != 234000 {
		t.Errorf("expected first N50 = 234000, got %d", rows[0].N50)
	}
}

func TestReadCheckM2(t *testing.T) {
	rows, err := ReadAll[CheckM2Row](fixturesDir + "/checkm2.parquet")
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(rows) != 20 {
		t.Errorf("expected 20 rows, got %d", len(rows))
	}
	if rows[0].CompletenessGeneral != 99.5 {
		t.Errorf("expected first completeness = 99.5, got %f", rows[0].CompletenessGeneral)
	}
}

func TestReadNonexistent(t *testing.T) {
	_, err := ReadAll[AssemblyRow](fixturesDir + "/does_not_exist.parquet")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestReadFiltered(t *testing.T) {
	rows, err := ReadFiltered[AssemblyRow](
		fixturesDir+"/assembly.parquet",
		func(row AssemblyRow) bool {
			return row.SylphSpecies == "Escherichia coli" && row.HQFilter == "PASS"
		},
	)
	if err != nil {
		t.Fatalf("ReadFiltered failed: %v", err)
	}
	if len(rows) != 5 {
		t.Errorf("expected 5 HQ E. coli rows, got %d", len(rows))
	}
}
