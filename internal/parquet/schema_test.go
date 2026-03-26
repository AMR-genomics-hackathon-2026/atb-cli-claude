package parquet

import "testing"

func TestGenusFromSpecies(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Escherichia coli", "Escherichia"},
		{"unknown", "unknown"},
		{"", ""},
	}

	for _, tt := range tests {
		got := GenusFromSpecies(tt.input)
		if got != tt.want {
			t.Errorf("GenusFromSpecies(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
