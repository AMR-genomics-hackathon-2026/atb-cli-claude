package cli

import (
	"bytes"
	"strings"
	"testing"
)

const fixtureDir = "../../testdata/fixtures"

func runCmd(args ...string) (string, string, error) {
	cmd := NewRootCmd("test")
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestQuerySpecies(t *testing.T) {
	stdout, _, err := runCmd("query", "--data-dir", fixtureDir, "--species", "Escherichia coli", "--format", "tsv")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if !strings.Contains(stdout, "SAMN00000001") {
		t.Errorf("expected SAMN00000001 in output, got:\n%s", stdout)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 6 {
		t.Errorf("expected 6 lines (header + 5 results), got %d lines:\n%s", len(lines), stdout)
	}
}

func TestQueryHQOnly(t *testing.T) {
	stdout, _, err := runCmd("query", "--data-dir", fixtureDir, "--hq-only", "--format", "tsv")
	if err != nil {
		t.Fatalf("query --hq-only failed: %v", err)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	// 1 header + 17 HQ results = 18 lines
	dataRows := len(lines) - 1
	if dataRows != 17 {
		t.Errorf("expected 17 HQ results, got %d", dataRows)
	}
}

func TestQueryWithN50(t *testing.T) {
	stdout, _, err := runCmd("query", "--data-dir", fixtureDir, "--species", "Escherichia coli", "--min-n50", "240000", "--format", "tsv")
	if err != nil {
		t.Fatalf("query with min-n50 failed: %v", err)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 4 {
		t.Errorf("expected 4 lines (header + 3 results), got %d lines:\n%s", len(lines), stdout)
	}
}

func TestInfoCommand(t *testing.T) {
	stdout, _, err := runCmd("info", "--data-dir", fixtureDir, "SAMN00000001")
	if err != nil {
		t.Fatalf("info command failed: %v", err)
	}

	if !strings.Contains(stdout, "Escherichia coli") {
		t.Errorf("expected 'Escherichia coli' in output, got:\n%s", stdout)
	}

	if !strings.Contains(stdout, "PASS") {
		t.Errorf("expected 'PASS' in output, got:\n%s", stdout)
	}
}

func TestInfoNotFound(t *testing.T) {
	_, _, err := runCmd("info", "--data-dir", fixtureDir, "NONEXISTENT")
	if err == nil {
		t.Fatal("expected an error for non-existent sample, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error message, got: %v", err)
	}
}

func TestVersionCommand(t *testing.T) {
	stdout, _, err := runCmd("version")
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	if !strings.Contains(stdout, "atb") {
		t.Errorf("expected 'atb' in version output, got:\n%s", stdout)
	}
}

func TestSummariseFromTSV(t *testing.T) {
	stdout, _, err := runCmd("summarise", "--from", "../../testdata/sample_results.tsv")
	if err != nil {
		t.Fatalf("summarise --from tsv failed: %v", err)
	}

	if !strings.Contains(stdout, "Total genomes:") {
		t.Errorf("expected 'Total genomes:' in output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "5") {
		t.Errorf("expected count 5 in output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Klebsiella pneumoniae") {
		t.Errorf("expected 'Klebsiella pneumoniae' in output, got:\n%s", stdout)
	}
}

func TestSummariseFromCSV(t *testing.T) {
	stdout, _, err := runCmd("summarise", "--from", "../../testdata/sample_results.csv")
	if err != nil {
		t.Fatalf("summarise --from csv failed: %v", err)
	}

	if !strings.Contains(stdout, "Total genomes:") {
		t.Errorf("expected 'Total genomes:' in output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Klebsiella pneumoniae") {
		t.Errorf("expected 'Klebsiella pneumoniae' in output, got:\n%s", stdout)
	}
}

func TestSummariseFromNonExistent(t *testing.T) {
	_, _, err := runCmd("summarise", "--from", "/nonexistent/path/results.csv")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

func TestQueryMissingDatabaseNonInteractive(t *testing.T) {
	// When stdin is not a terminal (test environment), should get error message
	dir := t.TempDir()
	_, stderr, err := runCmd("query", "--data-dir", dir, "--species", "E. coli")
	if err == nil {
		t.Error("expected error for missing database")
	}
	// Should mention "database" or "fetch" in the error
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	combined := stderr + errMsg
	if !strings.Contains(strings.ToLower(combined), "database") && !strings.Contains(strings.ToLower(combined), "fetch") {
		t.Errorf("error should mention database or fetch: %s / %s", errMsg, stderr)
	}
}

func TestConfigShow(t *testing.T) {
	stdout, _, err := runCmd("config", "show")
	if err != nil {
		t.Fatalf("config show failed: %v", err)
	}

	if !strings.Contains(stdout, "data_dir") {
		t.Errorf("expected 'data_dir' in config show output, got:\n%s", stdout)
	}
}
