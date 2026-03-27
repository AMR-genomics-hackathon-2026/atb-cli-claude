package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds all configuration for atb.
type Config struct {
	General  GeneralConfig  `toml:"general"`
	Fetch    FetchConfig    `toml:"fetch"`
	Download DownloadConfig `toml:"download"`
}

// GeneralConfig holds general application settings.
type GeneralConfig struct {
	DataDir       string `toml:"data_dir"`
	DefaultFormat string `toml:"default_format"`
}

// FetchConfig holds settings for metadata fetching.
type FetchConfig struct {
	BaseURL  string `toml:"base_url"`
	Parallel int    `toml:"parallel"`
}

// DownloadConfig holds settings for genome downloads.
type DownloadConfig struct {
	Parallel       int    `toml:"parallel"`
	OutputDir      string `toml:"output_dir"`
	CheckDiskSpace bool   `toml:"check_disk_space"`
	MinFreeSpaceGB int    `toml:"min_free_space_gb"`
}

// DefaultDataDir returns the OS-standard data directory for ATB.
// On Windows: %LOCALAPPDATA%\atb\data
// On macOS: ~/Library/Application Support/atb/data
// On Linux/other: $XDG_DATA_HOME/atb/data or ~/.local/share/atb/data
func DefaultDataDir() string {
	switch runtime.GOOS {
	case "windows":
		if appdata := os.Getenv("LOCALAPPDATA"); appdata != "" {
			return filepath.Join(appdata, "atb", "data")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AppData", "Local", "atb", "data")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "atb", "data")
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "atb", "data")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", "atb", "data")
	}
}

// Default returns a Config populated with sensible defaults.
func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		General: GeneralConfig{
			DataDir:       DefaultDataDir(),
			DefaultFormat: "tsv",
		},
		Fetch: FetchConfig{
			BaseURL:  "https://ftp.ebi.ac.uk/pub/databases/AllTheBacteria",
			Parallel: 4,
		},
		Download: DownloadConfig{
			Parallel:       4,
			OutputDir:      filepath.Join(home, "atb", "genomes"),
			CheckDiskSpace: true,
			MinFreeSpaceGB: 10,
		},
	}
}

// DefaultPath returns the default config file path.
// On Windows: %LOCALAPPDATA%\atb\config.toml
// On macOS: ~/Library/Application Support/atb/config.toml
// On Linux/other: $XDG_CONFIG_HOME/atb/config.toml or ~/.config/atb/config.toml
func DefaultPath() string {
	switch runtime.GOOS {
	case "windows":
		if appdata := os.Getenv("LOCALAPPDATA"); appdata != "" {
			return filepath.Join(appdata, "atb", "config.toml")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AppData", "Local", "atb", "config.toml")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "atb", "config.toml")
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "atb", "config.toml")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "atb", "config.toml")
	}
}

// Load reads a TOML config from path. If the file does not exist, it returns
// the default config without error.
func Load(path string) (Config, error) {
	cfg := Default()

	_, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}

	return cfg, nil
}

// Save writes cfg to path as TOML, creating parent directories as needed.
func Save(cfg Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(cfg)
}

// ExpandPath expands a leading ~ to the user's home directory.
func ExpandPath(path string) (string, error) {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home, nil
	}

	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}

	return path, nil
}
