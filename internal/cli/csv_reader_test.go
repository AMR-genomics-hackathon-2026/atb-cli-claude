package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadResultsFromFileTSV(t *testing.T) {
	content := "sample_accession\thq_filter\tsylph_species\tdataset\nSAMEA001\tPASS\tEscherichia coli\talpha\nSAMEA002\tFAIL\tSalmonella enterica\tbeta\n"
	path := writeTempFile(t, "results-*.tsv", content)

	rows, err := readResultsFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["sample_accession"] != "SAMEA001" {
		t.Errorf("rows[0][sample_accession]: got %q, want %q", rows[0]["sample_accession"], "SAMEA001")
	}
	if rows[0]["hq_filter"] != "PASS" {
		t.Errorf("rows[0][hq_filter]: got %q, want %q", rows[0]["hq_filter"], "PASS")
	}
	if rows[1]["sylph_species"] != "Salmonella enterica" {
		t.Errorf("rows[1][sylph_species]: got %q, want %q", rows[1]["sylph_species"], "Salmonella enterica")
	}
}

func TestReadResultsFromFileCSV(t *testing.T) {
	content := "sample_accession,hq_filter,sylph_species,dataset\nSAMEA001,PASS,Escherichia coli,alpha\nSAMEA002,FAIL,Salmonella enterica,beta\n"
	path := writeTempFile(t, "results-*.csv", content)

	rows, err := readResultsFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["dataset"] != "alpha" {
		t.Errorf("rows[0][dataset]: got %q, want %q", rows[0]["dataset"], "alpha")
	}
}

func TestReadResultsFromFileEmpty(t *testing.T) {
	content := "sample_accession,hq_filter\n"
	path := writeTempFile(t, "results-*.csv", content)

	rows, err := readResultsFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
}

func TestReadResultsFromFileMissing(t *testing.T) {
	_, err := readResultsFromFile("/nonexistent/path/results.csv")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

func writeTempFile(t *testing.T, pattern, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), pattern)
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return filepath.Clean(f.Name())
}
