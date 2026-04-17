package query

import "testing"

func TestBuildENASampleSetInactive(t *testing.T) {
	set, err := BuildENASampleSet(fixturesDir, ENAFilter{})
	if err != nil {
		t.Fatalf("BuildENASampleSet returned error for inactive filter: %v", err)
	}
	if set != nil {
		t.Errorf("expected nil map for inactive filter, got %d entries", len(set))
	}
}

func TestBuildENASampleSetCountry(t *testing.T) {
	set, err := BuildENASampleSet(fixturesDir, ENAFilter{Country: "UK"})
	if err != nil {
		t.Fatalf("BuildENASampleSet failed: %v", err)
	}
	if len(set) != 2 {
		t.Errorf("expected 2 UK samples, got %d", len(set))
	}
}

func TestBuildENASampleSetPlatform(t *testing.T) {
	set, err := BuildENASampleSet(fixturesDir, ENAFilter{Platform: "OXFORD_NANOPORE"})
	if err != nil {
		t.Fatalf("BuildENASampleSet failed: %v", err)
	}
	if len(set) != 4 {
		t.Errorf("expected 4 OXFORD_NANOPORE samples, got %d", len(set))
	}
}

func TestBuildENASampleSetDateRange(t *testing.T) {
	set, err := BuildENASampleSet(fixturesDir, ENAFilter{
		CollectionDateFrom: "2020-01-01",
		CollectionDateTo:   "2023-12-31",
	})
	if err != nil {
		t.Fatalf("BuildENASampleSet failed: %v", err)
	}
	if len(set) != 16 {
		t.Errorf("expected 16 samples in 2020-2023 range, got %d", len(set))
	}
}

func TestBuildENASampleSetCombined(t *testing.T) {
	set, err := BuildENASampleSet(fixturesDir, ENAFilter{
		Country:            "UK",
		Platform:           "ILLUMINA",
		CollectionDateFrom: "2020-01-01",
	})
	if err != nil {
		t.Fatalf("BuildENASampleSet failed: %v", err)
	}
	// Fixture: SAMN00000001 (UK, 2019) is excluded by date; the second UK sample
	// must satisfy country, platform, and date for the intersection to hit.
	for acc := range set {
		if acc == "SAMN00000001" {
			t.Errorf("SAMN00000001 should be excluded by date filter")
		}
	}
}

func TestBuildENASampleSetInvalidDate(t *testing.T) {
	_, err := BuildENASampleSet(fixturesDir, ENAFilter{CollectionDateFrom: "2020/01/01"})
	if err == nil {
		t.Fatal("expected error for malformed --collection-date-from")
	}
}

func TestENAFilterActive(t *testing.T) {
	cases := []struct {
		name string
		f    ENAFilter
		want bool
	}{
		{"empty", ENAFilter{}, false},
		{"country", ENAFilter{Country: "UK"}, true},
		{"platform", ENAFilter{Platform: "ILLUMINA"}, true},
		{"from", ENAFilter{CollectionDateFrom: "2020-01-01"}, true},
		{"to", ENAFilter{CollectionDateTo: "2020-01-01"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.f.Active(); got != tc.want {
				t.Errorf("Active() = %v, want %v", got, tc.want)
			}
		})
	}
}
