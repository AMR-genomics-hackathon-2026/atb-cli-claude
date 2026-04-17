package query

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	pq "github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/parquet"
)

// ENAFileName is the name of the ENA metadata parquet file on disk.
const ENAFileName = "ena_20250506.parquet"

// ENAFilter holds optional filters that require joining against the ENA
// metadata table. It is the subset of Filters that any command can reuse
// without pulling in the full query Filters struct.
type ENAFilter struct {
	Country            string
	Platform           string
	CollectionDateFrom string
	CollectionDateTo   string
}

// Active reports whether any ENA filter is set.
func (f ENAFilter) Active() bool {
	return f.Country != "" ||
		f.Platform != "" ||
		f.CollectionDateFrom != "" ||
		f.CollectionDateTo != ""
}

// BuildENASampleSet streams ena_20250506.parquet and returns the set of
// sample_accessions matching every provided filter. When no filter is
// active it returns (nil, nil) so callers can skip the intersection step.
func BuildENASampleSet(dir string, f ENAFilter) (map[string]struct{}, error) {
	if !f.Active() {
		return nil, nil
	}

	var (
		fromTime, toTime time.Time
		haveFrom, haveTo bool
	)
	if f.CollectionDateFrom != "" {
		t, err := time.Parse("2006-01-02", f.CollectionDateFrom)
		if err != nil {
			return nil, fmt.Errorf("invalid --collection-date-from %q: expected YYYY-MM-DD", f.CollectionDateFrom)
		}
		fromTime, haveFrom = t, true
	}
	if f.CollectionDateTo != "" {
		t, err := time.Parse("2006-01-02", f.CollectionDateTo)
		if err != nil {
			return nil, fmt.Errorf("invalid --collection-date-to %q: expected YYYY-MM-DD", f.CollectionDateTo)
		}
		toTime, haveTo = t, true
	}

	enaPath := filepath.Join(dir, ENAFileName)
	rows, err := pq.ReadStreamFiltered[pq.ENARow](enaPath, func(r pq.ENARow) bool {
		if f.Country != "" && !strings.EqualFold(r.Country, f.Country) {
			return false
		}
		if f.Platform != "" && !strings.EqualFold(r.InstrumentPlatform, f.Platform) {
			return false
		}
		if haveFrom || haveTo {
			rowStart, rowEnd, ok := parseCollectionDate(r.CollectionDate)
			if !ok {
				return false
			}
			if haveFrom && rowEnd.Before(fromTime) {
				return false
			}
			if haveTo && rowStart.After(toTime) {
				return false
			}
		}
		return true
	}, 0)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", ENAFileName, err)
	}

	set := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		set[r.SampleAccession] = struct{}{}
	}
	return set, nil
}
