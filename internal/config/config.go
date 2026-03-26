package config

import (
	"errors"
	"os"
	"path/filepath"
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

// Default returns a Config populated with sensible defaults.
func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		General: GeneralConfig{
			DataDir:       filepath.Join(home, "atb", "metadata", "parquet"),
			DefaultFormat: "auto",
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

// DefaultPath returns the default config file path (~/.config/atb/config.toml).
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "atb", "config.toml")
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
