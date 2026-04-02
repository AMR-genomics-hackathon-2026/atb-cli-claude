package cli

import (
	"testing"
)

func TestDeduplicateAccessions(t *testing.T) {
	input := []string{"SAMN001", "SAMN002", "SAMN001", "SAMN003", "SAMN002"}
	got := deduplicateAccessions(input)

	want := []string{"SAMN001", "SAMN002", "SAMN003"}
	if len(got) != len(want) {
		t.Fatalf("expected %d accessions, got %d: %v", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("index %d: expected %q, got %q", i, w, got[i])
		}
	}
}

func TestDeduplicateAccessionsEmpty(t *testing.T) {
	got := deduplicateAccessions(nil)
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestBuildAssemblyURL(t *testing.T) {
	url := buildAssemblyURL("SAMN00000355")
	want := "https://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/SAMN00000355.fa.gz"
	if url != want {
		t.Errorf("expected %q, got %q", want, url)
	}
}
