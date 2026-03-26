package suggest

import (
	"testing"
)

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"kitten", "sitting", 3},
	}

	for _, c := range cases {
		got := levenshtein(c.a, c.b)
		if got != c.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestSuggest(t *testing.T) {
	species := []string{
		"Escherichia coli",
		"Staphylococcus aureus",
		"Salmonella enterica",
		"Klebsiella pneumoniae",
		"Pseudomonas aeruginosa",
	}

	cases := []struct {
		input string
		want  string
	}{
		{"Escherichia col", "Escherichia coli"},
		{"staphylococcus aureas", "Staphylococcus aureus"},
		{"salmonela enterica", "Salmonella enterica"},
	}

	for _, c := range cases {
		got := Suggest(c.input, species, 3)
		if len(got) == 0 {
			t.Errorf("Suggest(%q) returned no results", c.input)
			continue
		}
		if got[0] != c.want {
			t.Errorf("Suggest(%q)[0] = %q, want %q", c.input, got[0], c.want)
		}
	}
}
