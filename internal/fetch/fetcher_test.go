package fetch_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/fetch"
)

func TestCoreTables(t *testing.T) {
	tables := fetch.CoreTables()
	if len(tables) != 5 {
		t.Fatalf("expected 5 core tables, got %d", len(tables))
	}

	want := map[string]bool{
		"assembly.parquet":       true,
		"assembly_stats.parquet": true,
		"checkm2.parquet":        true,
		"sylph.parquet":          true,
		"run.parquet":             true,
	}

	for _, name := range tables {
		if !want[name] {
			t.Errorf("unexpected core table: %s", name)
		}
	}
}

func TestAllTables(t *testing.T) {
	tables := fetch.AllTables()
	if len(tables) != 10 {
		t.Fatalf("expected 10 tables, got %d", len(tables))
	}
}

func TestFetchTable(t *testing.T) {
	const payload = "fake parquet data"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(payload))
	}))
	defer srv.Close()

	dir := t.TempDir()

	f := fetch.New(fetch.Config{
		DataDir:  dir,
		Parallel: 1,
	})

	name := "test.parquet"
	if err := f.FetchTable(name, srv.URL+"/test.parquet", false); err != nil {
		t.Fatalf("FetchTable failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}

	if string(data) != payload {
		t.Errorf("got %q, want %q", string(data), payload)
	}
}

func TestFetchTableSkipsExisting(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data"))
	}))
	defer srv.Close()

	dir := t.TempDir()

	// Pre-create the file
	name := "existing.parquet"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	f := fetch.New(fetch.Config{DataDir: dir})
	if err := f.FetchTable(name, srv.URL+"/"+name, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls != 0 {
		t.Errorf("expected 0 HTTP calls for existing file, got %d", calls)
	}
}

func TestFetchTableForce(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("new data"))
	}))
	defer srv.Close()

	dir := t.TempDir()

	name := "existing.parquet"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	f := fetch.New(fetch.Config{DataDir: dir})
	if err := f.FetchTable(name, srv.URL+"/"+name, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls != 1 {
		t.Errorf("expected 1 HTTP call with force=true, got %d", calls)
	}
}
