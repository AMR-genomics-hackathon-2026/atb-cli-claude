package amr

import (
	"fmt"
	"path/filepath"
	"strings"

	pq "github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/parquet"
)

// AMRFileName is the single merged parquet file containing all AMR data.
const AMRFileName = "amrfinderplus.parquet"

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
	// Genus restricts results to a specific bacterial genus (case-insensitive). Empty means all.
	Genus string
	// Limit caps the number of returned results. 0 means no limit.
	Limit int
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
	Genus           string
}

// Query reads AMR data from dataDir, applies filters, and returns matching results.
// If a genus partition file exists for the requested genus, it reads only that file.
// Otherwise it falls back to the monolithic amrfinderplus.parquet.
func Query(dataDir string, filters Filters) ([]Result, error) {
	amrPath := filepath.Join(dataDir, AMRFileName)

	// Try genus partition first (much smaller file)
	if filters.Genus != "" {
		if partPath := PartitionPath(dataDir, filters.Genus); partPath != "" {
			amrPath = partPath
		}
	}

	rows, err := pq.ReadStreamFiltered[pq.AMRRow](amrPath, func(row pq.AMRRow) bool {
		return matchesFilters(row, filters)
	}, filters.Limit)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filepath.Base(amrPath), err)
	}

	results := make([]Result, 0, len(rows))
	for _, row := range rows {
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
			Genus:           row.Genus,
		})
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
	if f.Genus != "" && !strings.EqualFold(row.Genus, f.Genus) {
		return false
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
