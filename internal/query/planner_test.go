package query

import (
	"reflect"
	"testing"
)

func TestPlan(t *testing.T) {
	tests := []struct {
		name    string
		filters Filters
		columns []string
		want    []string
	}{
		{
			name:    "species_only",
			filters: Filters{Species: "Escherichia coli"},
			columns: nil,
			want:    []string{"assembly"},
		},
		{
			name:    "with_completeness",
			filters: Filters{MinCompleteness: 99.0},
			columns: nil,
			want:    []string{"assembly", "checkm2"},
		},
		{
			name:    "with_n50",
			filters: Filters{MinN50: 200000},
			columns: nil,
			want:    []string{"assembly", "assembly_stats"},
		},
		{
			name:    "with_country",
			filters: Filters{Country: "UK"},
			columns: nil,
			want:    []string{"assembly", "run", "ena_20250506"},
		},
		{
			name:    "columns_need_checkm2",
			filters: Filters{},
			columns: []string{"sample_accession", "Completeness_General"},
			want:    []string{"assembly", "checkm2"},
		},
		{
			name:    "full_join",
			filters: Filters{MinCompleteness: 99.0, MinN50: 200000, Country: "UK"},
			columns: nil,
			want:    []string{"assembly", "checkm2", "assembly_stats", "run", "ena_20250506"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Plan(tt.filters, tt.columns)
			if !reflect.DeepEqual(got.Tables, tt.want) {
				t.Errorf("Plan() tables = %v, want %v", got.Tables, tt.want)
			}
		})
	}
}
