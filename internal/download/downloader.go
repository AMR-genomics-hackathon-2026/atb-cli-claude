package download

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
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

// Downloader manages parallel HTTP downloads with resume and retry support.
type Downloader struct {
	cfg    Config
	client *http.Client
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
	return &Downloader{
		cfg:    cfg,
		client: &http.Client{Timeout: 5 * time.Minute},
	}
}

// DownloadFile fetches url and saves it as filename inside the output directory.
// It skips the download when the final file already exists, and resumes from a
// .part file when one is present. Retries are performed with exponential backoff
// for 429 and 5xx responses; 4xx (except 429) fail immediately.
func (d *Downloader) DownloadFile(url, filename string) error {
	if err := os.MkdirAll(d.cfg.OutputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	finalPath := filepath.Join(d.cfg.OutputDir, filename)
	if _, err := os.Stat(finalPath); err == nil {
		return nil // already complete
	}

	partPath := finalPath + ".part"

	var attempt int
	for {
		err := d.attemptDownload(url, finalPath, partPath)
		if err == nil {
			return nil
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
}

// attemptDownload performs one HTTP request to download url into partPath and
// renames it to finalPath on success. Returns a *downloadErr with fatal=true
// for permanent failures (4xx except 429).
func (d *Downloader) attemptDownload(url, finalPath, partPath string) error {
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
	case resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent:
		return &downloadErr{msg: fmt.Sprintf("unexpected HTTP %d", resp.StatusCode), fatal: true}
	}

	// If the server returned 200 (not 206), discard the partial file and start fresh.
	flags := os.O_WRONLY | os.O_CREATE
	if resp.StatusCode == http.StatusPartialContent {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	f, err := os.OpenFile(partPath, flags, 0644)
	if err != nil {
		return &downloadErr{msg: err.Error(), fatal: true}
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return &downloadErr{msg: err.Error(), fatal: false}
	}
	f.Close()

	if err := os.Rename(partPath, finalPath); err != nil {
		return &downloadErr{msg: err.Error(), fatal: true}
	}

	return nil
}

// DownloadAll downloads all urls in parallel using a goroutine pool limited by
// cfg.Parallel. It returns a Result with counts and any per-URL errors.
func (d *Downloader) DownloadAll(urls []string) Result {
	result := Result{Total: len(urls)}

	sem := make(chan struct{}, d.cfg.Parallel)
	var mu sync.Mutex
	var bytesTotal int64
	var wg sync.WaitGroup
	var completed, failed int64

	for _, rawURL := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			filename := filepath.Base(u)
			err := d.DownloadFile(u, filename)
			if err != nil {
				atomic.AddInt64(&failed, 1)
				mu.Lock()
				result.Errors = append(result.Errors, DownloadError{URL: u, Error: err.Error()})
				mu.Unlock()
			} else {
				atomic.AddInt64(&completed, 1)
				if info, statErr := os.Stat(filepath.Join(d.cfg.OutputDir, filename)); statErr == nil {
					atomic.AddInt64(&bytesTotal, info.Size())
				}
			}
		}(rawURL)
	}

	wg.Wait()

	result.Completed = int(atomic.LoadInt64(&completed))
	result.Failed = int(atomic.LoadInt64(&failed))
	result.Bytes = atomic.LoadInt64(&bytesTotal)
	return result
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
