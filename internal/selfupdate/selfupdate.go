package selfupdate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	repo           = "AMR-genomics-hackathon-2026/atb-cli-claude"
	checkInterval  = 24 * time.Hour
	stateFileName  = "update-state.json"
)

type Release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type UpdateState struct {
	LastChecked   time.Time `json:"last_checked"`
	LatestVersion string    `json:"latest_version"`
	NotifiedUser  bool      `json:"notified_user"`
}

func statePath() string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "atb", stateFileName)
	}
	switch runtime.GOOS {
	case "windows":
		if appdata := os.Getenv("LOCALAPPDATA"); appdata != "" {
			return filepath.Join(appdata, "atb", stateFileName)
		}
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "atb", stateFileName)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "atb", stateFileName)
}

func loadState() UpdateState {
	data, err := os.ReadFile(statePath())
	if err != nil {
		return UpdateState{}
	}
	var state UpdateState
	json.Unmarshal(data, &state)
	return state
}

func saveState(state UpdateState) {
	path := statePath()
	os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := json.Marshal(state)
	os.WriteFile(path, data, 0o644)
}

func FetchLatestRelease() (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}
	return &release, nil
}

// CompareVersions returns true if remote is newer than current.
// Both should be semver strings like "v0.1.0" or "0.1.0".
func CompareVersions(current, remote string) bool {
	current = strings.TrimPrefix(current, "v")
	remote = strings.TrimPrefix(remote, "v")

	if current == "dev" || current == "" {
		return false // dev builds don't auto-update
	}

	return remote != current && remote > current
}

func assetName() string {
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("atb-cli_*_%s_%s.%s", runtime.GOOS, runtime.GOARCH, ext)
}

// FindAsset finds the matching release asset for the current platform.
func FindAsset(release *Release) *Asset {
	target := fmt.Sprintf("_%s_%s.", runtime.GOOS, runtime.GOARCH)
	for i := range release.Assets {
		if strings.Contains(release.Assets[i].Name, target) {
			return &release.Assets[i]
		}
	}
	return nil
}

// Apply downloads the release asset and replaces the current binary.
func Apply(asset *Asset) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(asset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "atb-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("write update: %w", err)
	}
	tmp.Close()

	// Extract binary from archive
	binaryPath, err := extractBinary(tmpPath, asset.Name)
	if err != nil {
		return err
	}
	defer os.Remove(binaryPath)

	// Replace current binary
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find current binary: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve binary path: %w", err)
	}

	// Atomic replace: rename new over old
	if err := os.Chmod(binaryPath, 0o755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	// On Windows, can't replace a running binary directly.
	// Move old binary aside first.
	backupPath := execPath + ".old"
	os.Remove(backupPath) // clean up any previous backup
	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}

	if err := copyFile(binaryPath, execPath); err != nil {
		// Restore backup on failure
		os.Rename(backupPath, execPath)
		return fmt.Errorf("install new binary: %w", err)
	}

	os.Remove(backupPath)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// CheckInBackground performs a non-blocking update check and prints a notice
// if a newer version is available. Call from root command PersistentPreRun.
func CheckInBackground(currentVersion string, w io.Writer) {
	state := loadState()
	if time.Since(state.LastChecked) < checkInterval {
		// Already checked recently, just show notice if we found one
		if state.LatestVersion != "" && CompareVersions(currentVersion, state.LatestVersion) && !state.NotifiedUser {
			fmt.Fprintf(w, "\n  A new version of atb is available: %s (current: %s)\n", state.LatestVersion, currentVersion)
			fmt.Fprintf(w, "  Run 'atb update' to upgrade.\n\n")
			state.NotifiedUser = true
			saveState(state)
		}
		return
	}

	// Check in background goroutine - don't block the command
	go func() {
		release, err := FetchLatestRelease()
		if err != nil {
			return // silently fail
		}
		state.LastChecked = time.Now()
		state.LatestVersion = release.TagName
		state.NotifiedUser = false
		saveState(state)
	}()
}
