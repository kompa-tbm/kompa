// Package config manages Kompa's persistent configuration.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kompa-tbm/kompa/internal/platform"
)

const configFileName = "config.json"

// Config holds user-configurable settings for Kompa.
type Config struct {
	// GithubRepo is the GitHub repository (owner/repo) that hosts Kompa package releases.
	GithubRepo string `json:"github_repo"`
	// GithubToken is an optional GitHub personal access token (for higher API rate limits).
	GithubToken string `json:"github_token,omitempty"`
	// CacheTTL is the number of seconds that remote metadata is cached.
	CacheTTL int `json:"cache_ttl"`
	// Verbose enables verbose logging by default.
	Verbose bool `json:"verbose"`
	// NoColor disables ANSI color output.
	NoColor bool `json:"no_color"`
	// MaxDownloadRetries is the number of times to retry a failed download.
	MaxDownloadRetries int `json:"max_download_retries"`
	// DownloadTimeoutSecs is the per-request HTTP timeout in seconds.
	DownloadTimeoutSecs int `json:"download_timeout_secs"`
	// ParallelDownloads sets the maximum number of simultaneous downloads.
	ParallelDownloads int `json:"parallel_downloads"`
	// KompaHome overrides the root Kompa data directory at runtime.
	KompaHome string `json:"kompa_home,omitempty"`
}

// Defaults returns a Config pre-filled with sensible defaults.
func Defaults() Config {
	return Config{
		GithubRepo:          "kompa-tbm/kompa",
		CacheTTL:            3600,
		Verbose:             false,
		NoColor:             false,
		MaxDownloadRetries:  3,
		DownloadTimeoutSecs: 300,
		ParallelDownloads:   4,
	}
}

// Load reads the configuration file from the Kompa root directory.
// If the file does not exist, defaults are returned.
func Load(dirs platform.Dirs) (Config, error) {
	cfg := Defaults()
	path := filepath.Join(dirs.Root, configFileName)

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("reading config file %s: %w", path, err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config file %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the configuration to the Kompa root directory.
func Save(dirs platform.Dirs, cfg Config) error {
	if err := os.MkdirAll(dirs.Root, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	path := filepath.Join(dirs.Root, configFileName)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config file %s: %w", path, err)
	}
	return nil
}

// Get retrieves a single configuration value by key as a string.
func Get(cfg Config, key string) (string, error) {
	switch key {
	case "github_repo":
		return cfg.GithubRepo, nil
	case "github_token":
		return cfg.GithubToken, nil
	case "verbose":
		return fmt.Sprintf("%v", cfg.Verbose), nil
	case "no_color":
		return fmt.Sprintf("%v", cfg.NoColor), nil
	case "cache_ttl":
		return fmt.Sprintf("%d", cfg.CacheTTL), nil
	case "max_download_retries":
		return fmt.Sprintf("%d", cfg.MaxDownloadRetries), nil
	case "download_timeout_secs":
		return fmt.Sprintf("%d", cfg.DownloadTimeoutSecs), nil
	case "parallel_downloads":
		return fmt.Sprintf("%d", cfg.ParallelDownloads), nil
	case "kompa_home":
		return cfg.KompaHome, nil
	default:
		return "", fmt.Errorf("unknown config key %q", key)
	}
}

// Set updates a single configuration value by key.
func Set(cfg *Config, key, value string) error {
	switch key {
	case "github_repo":
		cfg.GithubRepo = value
	case "github_token":
		cfg.GithubToken = value
	case "verbose":
		cfg.Verbose = value == "true" || value == "1" || value == "yes"
	case "no_color":
		cfg.NoColor = value == "true" || value == "1" || value == "yes"
	case "cache_ttl":
		n := 0
		if _, err := fmt.Sscanf(value, "%d", &n); err != nil {
			return fmt.Errorf("invalid integer value for cache_ttl: %s", value)
		}
		cfg.CacheTTL = n
	case "max_download_retries":
		n := 0
		if _, err := fmt.Sscanf(value, "%d", &n); err != nil {
			return fmt.Errorf("invalid integer value for max_download_retries: %s", value)
		}
		cfg.MaxDownloadRetries = n
	case "download_timeout_secs":
		n := 0
		if _, err := fmt.Sscanf(value, "%d", &n); err != nil {
			return fmt.Errorf("invalid integer value for download_timeout_secs: %s", value)
		}
		cfg.DownloadTimeoutSecs = n
	case "parallel_downloads":
		n := 0
		if _, err := fmt.Sscanf(value, "%d", &n); err != nil {
			return fmt.Errorf("invalid integer value for parallel_downloads: %s", value)
		}
		cfg.ParallelDownloads = n
	case "kompa_home":
		cfg.KompaHome = value
	default:
		return fmt.Errorf("unknown config key %q", key)
	}
	return nil
}

// AllKeys returns all valid configuration key names.
func AllKeys() []string {
	return []string{
		"github_repo",
		"github_token",
		"verbose",
		"no_color",
		"cache_ttl",
		"max_download_retries",
		"download_timeout_secs",
		"parallel_downloads",
		"kompa_home",
	}
}
