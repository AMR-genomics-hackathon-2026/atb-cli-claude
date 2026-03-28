package download

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// Config holds configuration for the Downloader.
type Config struct {
	OutputDir      string
	Parallel       int
	CheckDiskSpace bool
	MinFreeSpaceGB int
	MaxRetries     int
}

// FileTask describes a single file to download, with optional metadata
// for integrity verification and proper naming.
type FileTask struct {
	URL      string
	Filename string // saved as this name; falls back to filepath.Base(URL)
	MD5      string // expected MD5 hex digest; empty to skip verification
}

// Downloader manages parallel HTTP downloads with resume and retry support.
type Downloader struct {
	cfg    Config
	client *http.Client

	// OnProgress is called periodically during downloads if set.
	// Arguments: filename, bytesWritten, totalBytes (-1 if unknown).
	OnProgress func(filename string, written, total int64)
}

// Result summarises the outcome of a DownloadAll call.
type Result struct {
	Total     int
	Completed int
	Failed    int
	Bytes     int64
	Errors    []DownloadError
}

// DownloadError records which URL failed and why.
type DownloadError struct {
	URL   string
	Error string
}

// Manifest is written as manifest.json after a batch download.
type Manifest struct {
	Query      string    `json:"query"`
	Timestamp  time.Time `json:"timestamp"`
	Total      int       `json:"total_files"`
	Completed  int       `json:"completed"`
	Failed     int       `json:"failed"`
	TotalBytes int64     `json:"total_bytes"`
}

// New creates a Downloader using cfg. Defaults are applied for zero values.
func New(cfg Config) *Downloader {
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.Parallel == 0 {
		cfg.Parallel = 4
	}

	// Use transport-level timeouts instead of a blanket client timeout.
	// This avoids killing large file transfers that take longer than N minutes.
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   cfg.Parallel,
	}

	return &Downloader{
		cfg:    cfg,
		client: &http.Client{Transport: transport},
	}
}

// DownloadFile fetches url and saves it as filename inside the output directory.
// It skips the download when the final file already exists, and resumes from a
// .part file when one is present. Retries are performed with exponential backoff
// for 429 and 5xx responses; 4xx (except 429) fail immediately.
func (d *Downloader) DownloadFile(url, filename string) error {
	return d.DownloadFileVerified(url, filename, "")
}

// DownloadFileVerified is like DownloadFile but also verifies the MD5 checksum
// of the downloaded file when expectedMD5 is non-empty. If verification fails,
// the file is removed and an error is returned.
func (d *Downloader) DownloadFileVerified(url, filename, expectedMD5 string) error {
	if err := os.MkdirAll(d.cfg.OutputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	finalPath := filepath.Join(d.cfg.OutputDir, filename)
	if _, err := os.Stat(finalPath); err == nil {
		if expectedMD5 != "" {
			if ok, _ := verifyMD5(finalPath, expectedMD5); ok {
				return nil // already complete and verified
			}
			// Checksum mismatch — re-download
			os.Remove(finalPath)
		} else {
			return nil // already complete
		}
	}

	partPath := finalPath + ".part"

	var attempt int
	for {
		err := d.attemptDownload(url, filename, finalPath, partPath)
		if err == nil {
			break
		}

		var de *downloadErr
		if asDownloadErr(err, &de) && de.fatal {
			return err
		}

		attempt++
		if attempt >= d.cfg.MaxRetries {
			return err
		}

		backoff := time.Duration(math.Pow(2, float64(attempt))) * 500 * time.Millisecond
		time.Sleep(backoff)
	}

	if expectedMD5 != "" {
		ok, got := verifyMD5(finalPath, expectedMD5)
		if !ok {
			os.Remove(finalPath)
			return fmt.Errorf("MD5 mismatch for %s: expected %s, got %s", filename, expectedMD5, got)
		}
	}

	return nil
}

// attemptDownload performs one HTTP request to download url into partPath and
// renames it to finalPath on success. Returns a *downloadErr with fatal=true
// for permanent failures (4xx except 429).
func (d *Downloader) attemptDownload(url, filename, finalPath, partPath string) error {
	var partSize int64
	if info, err := os.Stat(partPath); err == nil {
		partSize = info.Size()
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return &downloadErr{msg: err.Error(), fatal: true}
	}

	if partSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", partSize))
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return &downloadErr{msg: err.Error(), fatal: false}
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return &downloadErr{msg: fmt.Sprintf("HTTP %d", resp.StatusCode), fatal: false}
	case resp.StatusCode >= 500:
		return &downloadErr{msg: fmt.Sprintf("HTTP %d", resp.StatusCode), fatal: false}
	case resp.StatusCode >= 400:
		return &downloadErr{msg: fmt.Sprintf("HTTP %d", resp.StatusCode), fatal: true}
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		// Range not satisfiable — part file may be corrupt or complete; start fresh
		os.Remove(partPath)
		partSize = 0
	case resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent:
		return &downloadErr{msg: fmt.Sprintf("unexpected HTTP %d", resp.StatusCode), fatal: true}
	}

	// If the server returned 200 (not 206), discard the partial file and start fresh.
	flags := os.O_WRONLY | os.O_CREATE
	if resp.StatusCode == http.StatusPartialContent {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
		partSize = 0
	}

	f, err := os.OpenFile(partPath, flags, 0644)
	if err != nil {
		return &downloadErr{msg: err.Error(), fatal: true}
	}

	var writer io.Writer = f
	if d.OnProgress != nil {
		totalSize := resp.ContentLength
		if totalSize > 0 {
			totalSize += partSize
		}
		writer = &progressWriter{
			w:        f,
			filename: filename,
			written:  partSize,
			total:    totalSize,
			callback: d.OnProgress,
			interval: 500 * time.Millisecond,
		}
	}

	if _, err := io.Copy(writer, resp.Body); err != nil {
		f.Close()
		return &downloadErr{msg: err.Error(), fatal: false}
	}
	f.Close()

	if err := os.Rename(partPath, finalPath); err != nil {
		return &downloadErr{msg: err.Error(), fatal: true}
	}

	return nil
}

// DownloadAllFiles downloads tasks in parallel. Each FileTask carries a URL,
// filename, and optional MD5 for verification. This is the preferred method
// when callers have file metadata (e.g., from an index).
func (d *Downloader) DownloadAllFiles(tasks []FileTask) Result {
	result := Result{Total: len(tasks)}

	sem := make(chan struct{}, d.cfg.Parallel)
	var mu sync.Mutex
	var bytesTotal int64
	var wg sync.WaitGroup
	var completed, failed int64

	for _, task := range tasks {
		wg.Add(1)
		go func(t FileTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			filename := t.Filename
			if filename == "" {
				filename = filepath.Base(t.URL)
			}

			err := d.DownloadFileVerified(t.URL, filename, t.MD5)
			if err != nil {
				atomic.AddInt64(&failed, 1)
				mu.Lock()
				result.Errors = append(result.Errors, DownloadError{URL: t.URL, Error: err.Error()})
				mu.Unlock()
			} else {
				atomic.AddInt64(&completed, 1)
				if info, statErr := os.Stat(filepath.Join(d.cfg.OutputDir, filename)); statErr == nil {
					atomic.AddInt64(&bytesTotal, info.Size())
				}
			}
		}(task)
	}

	wg.Wait()

	result.Completed = int(atomic.LoadInt64(&completed))
	result.Failed = int(atomic.LoadInt64(&failed))
	result.Bytes = atomic.LoadInt64(&bytesTotal)
	return result
}

// DownloadAll downloads all urls in parallel using a goroutine pool limited by
// cfg.Parallel. It returns a Result with counts and any per-URL errors.
func (d *Downloader) DownloadAll(urls []string) Result {
	tasks := make([]FileTask, len(urls))
	for i, u := range urls {
		tasks[i] = FileTask{URL: u, Filename: filepath.Base(u)}
	}
	return d.DownloadAllFiles(tasks)
}

// WriteManifest writes a manifest.json file summarising the download batch.
func (d *Downloader) WriteManifest(queryDesc string, result Result) error {
	if err := os.MkdirAll(d.cfg.OutputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	m := Manifest{
		Query:      queryDesc,
		Timestamp:  time.Now().UTC(),
		Total:      result.Total,
		Completed:  result.Completed,
		Failed:     result.Failed,
		TotalBytes: result.Bytes,
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	path := filepath.Join(d.cfg.OutputDir, "manifest.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

// verifyMD5 computes the MD5 of the file at path and compares against expected.
// Returns (match, actualHex).
func verifyMD5(path, expected string) (bool, string) {
	f, err := os.Open(path)
	if err != nil {
		return false, ""
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, ""
	}
	actual := hex.EncodeToString(h.Sum(nil))
	return actual == expected, actual
}

// progressWriter wraps an io.Writer and calls a callback at regular intervals.
type progressWriter struct {
	w        io.Writer
	filename string
	written  int64
	total    int64
	callback func(string, int64, int64)
	interval time.Duration
	lastCall time.Time
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p)
	pw.written += int64(n)

	now := time.Now()
	if now.Sub(pw.lastCall) >= pw.interval {
		pw.lastCall = now
		pw.callback(pw.filename, pw.written, pw.total)
	}
	return n, err
}

// downloadErr is an internal error type that carries fatality information.
type downloadErr struct {
	msg   string
	fatal bool
}

func (e *downloadErr) Error() string { return e.msg }

// asDownloadErr is a helper that tries to assign err to target as *downloadErr.
func asDownloadErr(err error, target **downloadErr) bool {
	if de, ok := err.(*downloadErr); ok {
		*target = de
		return true
	}
	return false
}
