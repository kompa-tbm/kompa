package config

import (
	"testing"

	"github.com/kompa-tbm/kompa/internal/platform"
)

func testDirs(t *testing.T) platform.Dirs {
	t.Helper()
	tmp := t.TempDir()
	return platform.GetDirsFromRoot(tmp)
}

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.GithubRepo == "" {
		t.Error("Defaults().GithubRepo is empty")
	}
	if cfg.CacheTTL <= 0 {
		t.Error("Defaults().CacheTTL should be positive")
	}
	if cfg.MaxDownloadRetries <= 0 {
		t.Error("Defaults().MaxDownloadRetries should be positive")
	}
	if cfg.DownloadTimeoutSecs <= 0 {
		t.Error("Defaults().DownloadTimeoutSecs should be positive")
	}
}

func TestLoad_NoFile(t *testing.T) {
	dirs := testDirs(t)
	cfg, err := Load(dirs)
	if err != nil {
		t.Fatalf("Load() on missing file: unexpected error: %v", err)
	}
	def := Defaults()
	if cfg.GithubRepo != def.GithubRepo {
		t.Errorf("Load() GithubRepo = %q, want %q", cfg.GithubRepo, def.GithubRepo)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dirs := testDirs(t)

	cfg := Defaults()
	cfg.GithubRepo = "my-org/my-repo"
	cfg.Verbose = true
	cfg.CacheTTL = 9999

	if err := Save(dirs, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load(dirs)
	if err != nil {
		t.Fatalf("Load() after Save() error = %v", err)
	}

	if loaded.GithubRepo != "my-org/my-repo" {
		t.Errorf("GithubRepo = %q, want my-org/my-repo", loaded.GithubRepo)
	}
	if !loaded.Verbose {
		t.Error("Verbose should be true after save/load")
	}
	if loaded.CacheTTL != 9999 {
		t.Errorf("CacheTTL = %d, want 9999", loaded.CacheTTL)
	}
}

func TestGet(t *testing.T) {
	cfg := Defaults()
	cfg.GithubRepo = "test/repo"
	cfg.Verbose = true

	val, err := Get(cfg, "github_repo")
	if err != nil {
		t.Fatalf("Get(github_repo) error = %v", err)
	}
	if val != "test/repo" {
		t.Errorf("Get(github_repo) = %q, want %q", val, "test/repo")
	}

	val, err = Get(cfg, "verbose")
	if err != nil {
		t.Fatalf("Get(verbose) error = %v", err)
	}
	if val != "true" {
		t.Errorf("Get(verbose) = %q, want true", val)
	}
}

func TestGet_Unknown(t *testing.T) {
	cfg := Defaults()
	_, err := Get(cfg, "nonexistent_key")
	if err == nil {
		t.Error("Get(unknown key) expected error, got nil")
	}
}

func TestSet(t *testing.T) {
	cfg := Defaults()

	tests := []struct {
		key   string
		value string
	}{
		{"github_repo", "other/repo"},
		{"verbose", "true"},
		{"no_color", "1"},
		{"cache_ttl", "7200"},
		{"max_download_retries", "5"},
		{"download_timeout_secs", "600"},
		{"parallel_downloads", "8"},
		{"kompa_home", "/tmp/kompa"},
	}

	for _, tt := range tests {
		if err := Set(&cfg, tt.key, tt.value); err != nil {
			t.Errorf("Set(%s, %s) error = %v", tt.key, tt.value, err)
		}
	}

	if cfg.GithubRepo != "other/repo" {
		t.Errorf("GithubRepo = %q after set", cfg.GithubRepo)
	}
	if !cfg.Verbose {
		t.Error("Verbose should be true after set")
	}
	if cfg.CacheTTL != 7200 {
		t.Errorf("CacheTTL = %d, want 7200", cfg.CacheTTL)
	}
}

func TestSet_Invalid(t *testing.T) {
	cfg := Defaults()
	if err := Set(&cfg, "cache_ttl", "not-a-number"); err == nil {
		t.Error("Set(cache_ttl, not-a-number) expected error, got nil")
	}
	if err := Set(&cfg, "unknown_key", "value"); err == nil {
		t.Error("Set(unknown_key) expected error, got nil")
	}
}

func TestAllKeys(t *testing.T) {
	keys := AllKeys()
	if len(keys) == 0 {
		t.Error("AllKeys() returned empty list")
	}
	required := []string{"github_repo", "verbose", "cache_ttl"}
	for _, k := range required {
		found := false
		for _, ak := range keys {
			if ak == k {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("AllKeys() missing key %q", k)
		}
	}
}
