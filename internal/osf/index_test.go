package osf

import (
	"strings"
	"testing"
)

const testTSV = `project	project_id	filename	url	md5	size(MB)
AllTheBacteria/AMR/AMRFinderPlus	7nwrx	v3.12.8/AMRFP_results.tsv.gz	https://osf.io/download/zgexh/	0629c681b3068eec	519.83
AllTheBacteria/AMR/AMRFinderPlus	7nwrx	v3.12.8/AMRFP_status.tsv.gz	https://osf.io/download/br9fv/	f5e03173115c2723	8.5
AllTheBacteria/Assembly	xv7q9	0.2/Set_1/batch1.tar.xz	https://osf.io/download/abc/	aaa111	1234.5
AllTheBacteria/Assembly	xv7q9	0.2/Set_1/batch2.tar.xz	https://osf.io/download/def/	bbb222	567.8
AllTheBacteria/MLST	h7wzy	mlst_results.tsv.gz	https://osf.io/download/ghi/	ccc333	12.3
`

func TestParseIndex(t *testing.T) {
	idx, err := ParseIndex(strings.NewReader(testTSV))
	if err != nil {
		t.Fatalf("ParseIndex: %v", err)
	}
	if len(idx.Entries) != 5 {
		t.Fatalf("got %d entries, want 5", len(idx.Entries))
	}

	e := idx.Entries[0]
	if e.Project != "AllTheBacteria/AMR/AMRFinderPlus" {
		t.Errorf("Project = %q", e.Project)
	}
	if e.Filename != "v3.12.8/AMRFP_results.tsv.gz" {
		t.Errorf("Filename = %q", e.Filename)
	}
	if e.SizeMB != 519.83 {
		t.Errorf("SizeMB = %v", e.SizeMB)
	}
	if e.URL != "https://osf.io/download/zgexh/" {
		t.Errorf("URL = %q", e.URL)
	}
}

func TestProjects(t *testing.T) {
	idx, _ := ParseIndex(strings.NewReader(testTSV))
	projects := idx.Projects()

	if len(projects) != 3 {
		t.Fatalf("got %d projects, want 3", len(projects))
	}

	// Sorted alphabetically
	if projects[0].Project != "AllTheBacteria/AMR/AMRFinderPlus" {
		t.Errorf("first project = %q", projects[0].Project)
	}
	if projects[0].FileCount != 2 {
		t.Errorf("AMR file count = %d, want 2", projects[0].FileCount)
	}

	if projects[1].Project != "AllTheBacteria/Assembly" {
		t.Errorf("second project = %q", projects[1].Project)
	}
	if projects[1].FileCount != 2 {
		t.Errorf("Assembly file count = %d, want 2", projects[1].FileCount)
	}
}

func TestFilterByProject(t *testing.T) {
	idx, _ := ParseIndex(strings.NewReader(testTSV))

	entries, err := idx.Filter("AllTheBacteria/AMR", "")
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
}

func TestFilterByPattern(t *testing.T) {
	idx, _ := ParseIndex(strings.NewReader(testTSV))

	entries, err := idx.Filter("", "batch.*tar")
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
}

func TestFilterByProjectAndPattern(t *testing.T) {
	idx, _ := ParseIndex(strings.NewReader(testTSV))

	entries, err := idx.Filter("AllTheBacteria/AMR", "results")
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Filename != "v3.12.8/AMRFP_results.tsv.gz" {
		t.Errorf("got %q", entries[0].Filename)
	}
}

func TestFilterInvalidRegex(t *testing.T) {
	idx, _ := ParseIndex(strings.NewReader(testTSV))
	_, err := idx.Filter("", "[invalid")
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestMatchProject(t *testing.T) {
	idx, _ := ParseIndex(strings.NewReader(testTSV))

	entries := idx.MatchProject("amr")
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	entries = idx.MatchProject("MLST")
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
}

func TestParseEmptyIndex(t *testing.T) {
	_, err := ParseIndex(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for empty index")
	}
}

func TestParseHeaderOnly(t *testing.T) {
	idx, err := ParseIndex(strings.NewReader("project\tproject_id\tfilename\turl\tmd5\tsize(MB)\n"))
	if err != nil {
		t.Fatalf("ParseIndex: %v", err)
	}
	if len(idx.Entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(idx.Entries))
	}
}
