package download

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDownloadSingleFile(t *testing.T) {
	content := "hello world download content"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, content)
	}))
	defer srv.Close()

	dir := t.TempDir()
	d := New(Config{OutputDir: dir, Parallel: 1, MaxRetries: 1})

	err := d.DownloadFile(srv.URL+"/file.txt", "file.txt")
	if err != nil {
		t.Fatalf("DownloadFile error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "file.txt"))
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(got) != content {
		t.Errorf("file content = %q, want %q", string(got), content)
	}
}

func TestDownloadMultiple(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "content of %s", r.URL.Path)
	}))
	defer srv.Close()

	dir := t.TempDir()
	d := New(Config{OutputDir: dir, Parallel: 2, MaxRetries: 1})

	urls := []string{
		srv.URL + "/a.txt",
		srv.URL + "/b.txt",
		srv.URL + "/c.txt",
	}

	result := d.DownloadAll(urls)
	if result.Completed != 3 {
		t.Errorf("completed = %d, want 3", result.Completed)
	}
	if result.Failed != 0 {
		t.Errorf("failed = %d, want 0 (errors: %v)", result.Failed, result.Errors)
	}

	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected file %s to exist: %v", name, err)
		}
	}
}

func TestDownloadResume(t *testing.T) {
	fullContent := "0123456789abcdefghij"
	partialContent := "0123456789"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader != "" {
			// Parse "bytes=N-"
			var start int
			fmt.Sscanf(rangeHeader, "bytes=%d-", &start)
			remaining := fullContent[start:]
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(fullContent)-1, len(fullContent)))
			w.Header().Set("Content-Length", strconv.Itoa(len(remaining)))
			w.WriteHeader(http.StatusPartialContent)
			fmt.Fprint(w, remaining)
		} else {
			fmt.Fprint(w, fullContent)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	// Write partial .part file simulating a previous interrupted download
	partPath := filepath.Join(dir, "resume.txt.part")
	if err := os.WriteFile(partPath, []byte(partialContent), 0644); err != nil {
		t.Fatalf("WriteFile part: %v", err)
	}

	d := New(Config{OutputDir: dir, Parallel: 1, MaxRetries: 1})

	err := d.DownloadFile(srv.URL+"/resume.txt", "resume.txt")
	if err != nil {
		t.Fatalf("DownloadFile resume error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "resume.txt"))
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(got) != fullContent {
		t.Errorf("file content = %q, want %q", string(got), fullContent)
	}
}

func TestDownload404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	dir := t.TempDir()
	d := New(Config{OutputDir: dir, Parallel: 1, MaxRetries: 3})

	err := d.DownloadFile(srv.URL+"/missing.txt", "missing.txt")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error %q should mention 404", err.Error())
	}
}

func TestWriteManifest(t *testing.T) {
	dir := t.TempDir()
	d := New(Config{OutputDir: dir})

	result := Result{
		Total:     3,
		Completed: 2,
		Failed:    1,
		Bytes:     1024,
		Errors:    []DownloadError{{URL: "http://example.com/x", Error: "404"}},
	}

	err := d.WriteManifest("test query", result)
	if err != nil {
		t.Fatalf("WriteManifest error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("ReadFile manifest: %v", err)
	}
	if !strings.Contains(string(data), "test query") {
		t.Errorf("manifest missing query string")
	}
	if !strings.Contains(string(data), `"total_files": 3`) {
		t.Errorf("manifest missing total_files, got: %s", string(data))
	}
}

func TestDownloadSkipsExistingFile(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		fmt.Fprint(w, "fresh content")
	}))
	defer srv.Close()

	dir := t.TempDir()
	// Pre-create the final file
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("old content"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	d := New(Config{OutputDir: dir, Parallel: 1, MaxRetries: 1})
	err := d.DownloadFile(srv.URL+"/existing.txt", "existing.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 0 {
		t.Errorf("expected 0 HTTP calls for existing file, got %d", callCount)
	}

	// File should still have the old content
	got, _ := os.ReadFile(filepath.Join(dir, "existing.txt"))
	if string(got) != "old content" {
		t.Errorf("file was overwritten, expected %q got %q", "old content", string(got))
	}
}

func TestDownloadAllAggregatesErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad.txt" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	dir := t.TempDir()
	d := New(Config{OutputDir: dir, Parallel: 2, MaxRetries: 1})

	urls := []string{
		srv.URL + "/good.txt",
		srv.URL + "/bad.txt",
	}

	result := d.DownloadAll(urls)
	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	if result.Completed != 1 {
		t.Errorf("completed = %d, want 1", result.Completed)
	}
	if result.Failed != 1 {
		t.Errorf("failed = %d, want 1", result.Failed)
	}
	if len(result.Errors) != 1 {
		t.Errorf("errors count = %d, want 1", len(result.Errors))
	}
}

func TestDownloadFileVerifiedGoodMD5(t *testing.T) {
	content := "hello world"
	// MD5 of "hello world" = 5eb63bbbe01eeed093cb22bb8f5acdc3
	expectedMD5 := "5eb63bbbe01eeed093cb22bb8f5acdc3"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, content)
	}))
	defer srv.Close()

	dir := t.TempDir()
	d := New(Config{OutputDir: dir, MaxRetries: 1})

	err := d.DownloadFileVerified(srv.URL+"/file.txt", "file.txt", expectedMD5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(dir, "file.txt"))
	if string(got) != content {
		t.Errorf("content = %q, want %q", string(got), content)
	}
}

func TestDownloadFileVerifiedBadMD5(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hello world")
	}))
	defer srv.Close()

	dir := t.TempDir()
	d := New(Config{OutputDir: dir, MaxRetries: 1})

	err := d.DownloadFileVerified(srv.URL+"/file.txt", "file.txt", "0000000000000000000000000000dead")
	if err == nil {
		t.Fatal("expected MD5 mismatch error")
	}
	if !strings.Contains(err.Error(), "MD5 mismatch") {
		t.Errorf("error = %q, want MD5 mismatch", err.Error())
	}

	// File should be removed after mismatch
	if _, err := os.Stat(filepath.Join(dir, "file.txt")); err == nil {
		t.Error("file should have been removed after MD5 mismatch")
	}
}

func TestDownloadFileVerifiedRedownloadsOnBadExisting(t *testing.T) {
	content := "correct content"
	expectedMD5 := "ee000912423a8bf8e7c5018cfed1574b" // md5("correct content")

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		fmt.Fprint(w, content)
	}))
	defer srv.Close()

	dir := t.TempDir()
	// Pre-create a file with wrong content (simulating corruption)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("corrupted"), 0644)

	d := New(Config{OutputDir: dir, MaxRetries: 1})
	err := d.DownloadFileVerified(srv.URL+"/file.txt", "file.txt", expectedMD5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 HTTP call to re-download, got %d", callCount)
	}

	got, _ := os.ReadFile(filepath.Join(dir, "file.txt"))
	if string(got) != content {
		t.Errorf("content = %q, want %q", string(got), content)
	}
}

func TestDownloadAllFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "content of %s", r.URL.Path)
	}))
	defer srv.Close()

	dir := t.TempDir()
	d := New(Config{OutputDir: dir, Parallel: 2, MaxRetries: 1})

	tasks := []FileTask{
		{URL: srv.URL + "/abc123/", Filename: "results.tsv.gz"},
		{URL: srv.URL + "/def456/", Filename: "status.tsv.gz"},
	}

	result := d.DownloadAllFiles(tasks)
	if result.Completed != 2 {
		t.Errorf("completed = %d, want 2", result.Completed)
	}
	if result.Failed != 0 {
		t.Errorf("failed = %d, want 0 (errors: %v)", result.Failed, result.Errors)
	}

	// Verify files are saved with the specified filenames, not URL basenames
	for _, name := range []string{"results.tsv.gz", "status.tsv.gz"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected file %s to exist: %v", name, err)
		}
	}
}

func TestProgressCallback(t *testing.T) {
	content := strings.Repeat("x", 1000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		fmt.Fprint(w, content)
	}))
	defer srv.Close()

	dir := t.TempDir()
	d := New(Config{OutputDir: dir, MaxRetries: 1})

	var progressCalled int64
	d.OnProgress = func(filename string, written, total int64) {
		atomic.AddInt64(&progressCalled, 1)
		if filename != "progress.txt" {
			t.Errorf("filename = %q, want progress.txt", filename)
		}
	}

	err := d.DownloadFile(srv.URL+"/progress.txt", "progress.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
