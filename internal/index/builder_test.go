package index

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuild(t *testing.T) {
	dir := t.TempDir()

	// Copy fixtures into temp dir so Build can find them.
	fixtures := "../../testdata/fixtures"
	for _, name := range []string{"assembly.parquet", "assembly_stats.parquet", "checkm2.parquet", "mlst.parquet"} {
		src := filepath.Join(fixtures, name)
		dst := filepath.Join(dir, name)
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("reading fixture %s: %v", name, err)
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			t.Fatalf("writing fixture %s: %v", name, err)
		}
	}

	var logs []string
	logf := func(format string, args ...any) {
		// Collect log messages but don't print them during tests.
		_ = format
		_ = args
	}

	if err := Build(dir, logf); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Index file must exist.
	idxPath := filepath.Join(dir, IndexFileName)
	if _, err := os.Stat(idxPath); err != nil {
		t.Fatalf("index file not created: %v", err)
	}

	// Open and verify row count.
	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.db.QueryRow("SELECT COUNT(*) FROM samples").Scan(&count); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	if count != 20 {
		t.Errorf("expected 20 rows, got %d", count)
	}

	// Verify stats were joined (n50 should be populated).
	var n50 int64
	if err := db.db.QueryRow("SELECT n50 FROM samples WHERE sample_accession = 'SAMN00000001'").Scan(&n50); err != nil {
		t.Fatalf("querying N50: %v", err)
	}
	if n50 != 234000 {
		t.Errorf("expected N50=234000 for SAMN00000001, got %d", n50)
	}

	// Verify checkm2 was joined (completeness should be populated).
	var completeness float64
	if err := db.db.QueryRow("SELECT completeness FROM samples WHERE sample_accession = 'SAMN00000001'").Scan(&completeness); err != nil {
		t.Fatalf("querying completeness: %v", err)
	}
	if completeness != 99.5 {
		t.Errorf("expected completeness=99.5 for SAMN00000001, got %f", completeness)
	}

	// Verify MLST was joined (mlst_scheme should be populated for E. coli).
	var mlstScheme string
	if err := db.db.QueryRow("SELECT mlst_scheme FROM samples WHERE sample_accession = 'SAMN00000001'").Scan(&mlstScheme); err != nil {
		t.Fatalf("querying mlst_scheme: %v", err)
	}
	if mlstScheme != "ecoli_achtman_4" {
		t.Errorf("expected mlst_scheme=ecoli_achtman_4 for SAMN00000001, got %q", mlstScheme)
	}

	_ = logs
}
