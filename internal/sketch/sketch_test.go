package sketch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseInfo(t *testing.T) {
	output := "sketch_version=0.2.4\nsequence_type=DNA\nsketch_size=1024\nn_samples=3218671\nkmers=[17, 29]\ninverted=false"
	info, err := ParseInfo(strings.NewReader(output))
	if err != nil {
		t.Fatalf("ParseInfo: %v", err)
	}
	if info.Samples != 3218671 {
		t.Errorf("Samples = %d, want 3218671", info.Samples)
	}
	if len(info.KmerSizes) != 2 || info.KmerSizes[0] != 17 || info.KmerSizes[1] != 29 {
		t.Errorf("KmerSizes = %v, want [17, 29]", info.KmerSizes)
	}
	if info.SketchSize != 1024 {
		t.Errorf("SketchSize = %d, want 1024", info.SketchSize)
	}
}

func TestParseInfoMissingFields(t *testing.T) {
	output := "sketch_version=0.2.4\nsequence_type=DNA"
	info, err := ParseInfo(strings.NewReader(output))
	if err != nil {
		t.Fatalf("ParseInfo: %v", err)
	}
	if info.Samples != 0 {
		t.Errorf("Samples = %d, want 0", info.Samples)
	}
}

func TestParseDistOutput(t *testing.T) {
	output := "SAMD00123456\tmy_genome.fasta\t0.9982\nSAMD00789012\tmy_genome.fasta\t0.9971\nSAMD00345678\tmy_genome.fasta\t0.9965\n"
	matches, err := ParseDistOutput(strings.NewReader(output))
	if err != nil {
		t.Fatalf("ParseDistOutput: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("got %d matches, want 3", len(matches))
	}
	if matches[0].RefName != "SAMD00123456" {
		t.Errorf("match[0].RefName = %q", matches[0].RefName)
	}
	if matches[0].QueryName != "my_genome.fasta" {
		t.Errorf("match[0].QueryName = %q", matches[0].QueryName)
	}
	if matches[0].ANI != 0.9982 {
		t.Errorf("match[0].ANI = %v", matches[0].ANI)
	}
}

func TestParseDistOutputSorted(t *testing.T) {
	output := "ref1\tq1\t0.80\nref2\tq1\t0.99\nref3\tq1\t0.90\n"
	matches, err := ParseDistOutput(strings.NewReader(output))
	if err != nil {
		t.Fatalf("ParseDistOutput: %v", err)
	}
	if matches[0].ANI != 0.99 {
		t.Errorf("expected highest ANI first, got %v", matches[0].ANI)
	}
	if matches[2].ANI != 0.80 {
		t.Errorf("expected lowest ANI last, got %v", matches[2].ANI)
	}
}

func TestParseDistOutputEmpty(t *testing.T) {
	matches, err := ParseDistOutput(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseDistOutput: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestParseDistOutputSkipsStatusLines(t *testing.T) {
	output := "ref1\tq1\t0.95\n🧬🖋️ sketchlib done in 0s\n"
	matches, err := ParseDistOutput(strings.NewReader(output))
	if err != nil {
		t.Fatalf("ParseDistOutput: %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("expected 1 match (status line skipped), got %d", len(matches))
	}
}

func TestIntegrationSketchAndDist(t *testing.T) {
	if _, err := FindBinary(); err != nil {
		t.Skip("sketchlib not installed, skipping integration test")
	}
	tmpDir := t.TempDir()
	fasta := filepath.Join(tmpDir, "test.fasta")
	os.WriteFile(fasta, []byte(">seq1\nACGTACGTACGTACGTACGTACGTACGTACGTACGTACGTACGTACGTACGT\n"), 0644)

	skDir, prefix, err := SketchQuery([]string{fasta}, []int{17}, 1)
	if err != nil {
		t.Fatalf("SketchQuery: %v", err)
	}
	defer os.RemoveAll(skDir)

	if _, err := os.Stat(prefix + ".skm"); err != nil {
		t.Fatalf("expected .skm file at %s.skm", prefix)
	}
	if _, err := os.Stat(prefix + ".skd"); err != nil {
		t.Fatalf("expected .skd file at %s.skd", prefix)
	}

	matches, err := QueryDist(prefix, prefix, 17, 1, 0)
	if err != nil {
		t.Fatalf("QueryDist: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 self-match, got %d", len(matches))
	}
	if matches[0].ANI != 1.0 {
		t.Errorf("self ANI = %v, want 1.0", matches[0].ANI)
	}
}
