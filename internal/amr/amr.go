package amr

import (
	"fmt"
	"path/filepath"
	"strings"

	pq "github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/parquet"
)

// elementTypeDir maps an element type string to the Hive-partitioned directory name.
func elementTypeDir(elementType string) string {
	switch strings.ToUpper(elementType) {
	case "STRESS":
		return "stress_by_genus"
	case "VIRULENCE":
		return "virulence_by_genus"
	default:
		return "amr_by_genus"
	}
}

// ReadGenusParts reads all parquet files for a genus from a Hive-partitioned directory.
// genusDir should be like <dataDir>/amr/amr_by_genus/Genus=Escherichia/
func ReadGenusParts(genusDir string) ([]pq.AMRRow, error) {
	pattern := filepath.Join(genusDir, "data_*.parquet")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("globbing %q: %w", pattern, err)
	}

	var rows []pq.AMRRow
	for _, f := range files {
		part, err := pq.ReadAll[pq.AMRRow](f)
		if err != nil {
			return nil, fmt.Errorf("reading %q: %w", f, err)
		}
		rows = append(rows, part...)
	}
	return rows, nil
}

// Filters controls which AMR rows are returned by Query.
type Filters struct {
	// Samples restricts results to a specific set of sample accessions.
	// Nil or empty means no restriction.
	Samples map[string]struct{}
	// Class filters by drug class (case-insensitive substring match). Empty means all.
	Class string
	// GenePattern filters by gene symbol. Supports % wildcards (prefix/suffix/contains). Empty means all.
	GenePattern string
	// MinCoverage is the minimum coverage percentage (0 = no minimum).
	MinCoverage float64
	// MinIdentity is the minimum identity percentage (0 = no minimum).
	MinIdentity float64
	// ElementType restricts to a specific element type ("AMR", "STRESS", "VIRULENCE"). Empty means all.
	ElementType string
}

// Result is a single AMR gene hit associated with a sample.
type Result struct {
	SampleAccession string
	GeneSymbol      string
	ElementType     string
	ElementSubtype  string
	Coverage        float64
	Identity        float64
	Method          string
	Class           string
	Subclass        string
	Species         string
}

// Query loads AMR data for a genus, applies filters, and returns matching results.
//
// dataDir is the root data directory (e.g. ~/.atb/data).
// genus is the bacterial genus (e.g. "Escherichia").
// elementType controls which directory to query:
//
//	"AMR" or ""  -> amr_by_genus
//	"STRESS"     -> stress_by_genus
//	"VIRULENCE"  -> virulence_by_genus
//	"all"        -> all three directories
func Query(dataDir, genus, elementType string, filters Filters) ([]Result, error) {
	var types []string
	if strings.EqualFold(elementType, "all") {
		types = []string{"AMR", "STRESS", "VIRULENCE"}
	} else {
		types = []string{elementType}
	}

	var results []Result
	for _, et := range types {
		dir := elementTypeDir(et)
		genusDir := filepath.Join(dataDir, "amr", dir, "Genus="+genus)

		rows, err := ReadGenusParts(genusDir)
		if err != nil {
			return nil, fmt.Errorf("reading genus %q from %q: %w", genus, dir, err)
		}

		for _, row := range rows {
			if !matchesFilters(row, filters) {
				continue
			}
			results = append(results, Result{
				SampleAccession: row.Name,
				GeneSymbol:      row.GeneSymbol,
				ElementType:     row.ElementType,
				ElementSubtype:  row.ElementSubtype,
				Coverage:        row.Coverage,
				Identity:        row.Identity,
				Method:          row.Method,
				Class:           row.Class,
				Subclass:        row.Subclass,
				Species:         row.Species,
			})
		}
	}

	return results, nil
}

func matchesFilters(row pq.AMRRow, f Filters) bool {
	if len(f.Samples) > 0 {
		if _, ok := f.Samples[row.Name]; !ok {
			return false
		}
	}
	if f.Class != "" && !strings.Contains(strings.ToUpper(row.Class), strings.ToUpper(f.Class)) {
		return false
	}
	if f.GenePattern != "" && !matchesPattern(row.GeneSymbol, f.GenePattern) {
		return false
	}
	if f.MinCoverage > 0 && row.Coverage < f.MinCoverage {
		return false
	}
	if f.MinIdentity > 0 && row.Identity < f.MinIdentity {
		return false
	}
	if f.ElementType != "" && !strings.EqualFold(f.ElementType, "all") {
		if !strings.EqualFold(row.ElementType, f.ElementType) {
			return false
		}
	}
	return true
}

// matchesPattern performs a case-insensitive wildcard match using % as the wildcard character.
func matchesPattern(s, pattern string) bool {
	s = strings.ToLower(s)
	pattern = strings.ToLower(pattern)

	prefix := strings.HasPrefix(pattern, "%")
	suffix := strings.HasSuffix(pattern, "%")

	switch {
	case prefix && suffix:
		inner := pattern[1 : len(pattern)-1]
		return strings.Contains(s, inner)
	case prefix:
		inner := pattern[1:]
		return strings.HasSuffix(s, inner)
	case suffix:
		inner := pattern[:len(pattern)-1]
		return strings.HasPrefix(s, inner)
	default:
		return s == pattern
	}
}
