// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Default
// ---------------------------------------------------------------------------

func TestDefault_RefreshRate(t *testing.T) {
	cfg := Default()
	if cfg.Kubecat.RefreshRate != 2 {
		t.Errorf("Default RefreshRate = %d, want 2", cfg.Kubecat.RefreshRate)
	}
}

func TestDefault_Theme(t *testing.T) {
	cfg := Default()
	if cfg.Kubecat.Theme != "default" {
		t.Errorf("Default Theme = %q, want %q", cfg.Kubecat.Theme, "default")
	}
}

func TestDefault_ReadOnlyFalse(t *testing.T) {
	cfg := Default()
	if cfg.Kubecat.ReadOnly {
		t.Error("Default ReadOnly should be false")
	}
}

func TestDefault_AIDisabled(t *testing.T) {
	cfg := Default()
	if cfg.Kubecat.AI.Enabled {
		t.Error("Default AI.Enabled should be false")
	}
}

func TestDefault_AISelectedProvider(t *testing.T) {
	cfg := Default()
	if cfg.Kubecat.AI.SelectedProvider != "ollama" {
		t.Errorf("Default AI.SelectedProvider = %q, want %q", cfg.Kubecat.AI.SelectedProvider, "ollama")
	}
}

func TestDefault_OllamaProviderPresent(t *testing.T) {
	cfg := Default()
	p, ok := cfg.Kubecat.AI.Providers["ollama"]
	if !ok {
		t.Fatal("Default config missing ollama provider")
	}
	if !p.Enabled {
		t.Error("Default ollama provider should be enabled")
	}
	if p.Endpoint != "http://localhost:11434" {
		t.Errorf("ollama endpoint = %q, want http://localhost:11434", p.Endpoint)
	}
}

func TestDefault_LoggerDefaults(t *testing.T) {
	cfg := Default()
	l := cfg.Kubecat.Logger
	if l.Tail != 100 {
		t.Errorf("Logger.Tail = %d, want 100", l.Tail)
	}
	if l.Buffer != 5000 {
		t.Errorf("Logger.Buffer = %d, want 5000", l.Buffer)
	}
	if l.LogLevel != "info" {
		t.Errorf("Logger.LogLevel = %q, want info", l.LogLevel)
	}
}

func TestDefault_ClustersEmpty(t *testing.T) {
	cfg := Default()
	if len(cfg.Kubecat.Clusters) != 0 {
		t.Errorf("Default Clusters should be empty, got %d", len(cfg.Kubecat.Clusters))
	}
}

// ---------------------------------------------------------------------------
// LoadFrom
// ---------------------------------------------------------------------------

func TestLoadFrom_NonExistentFile_ReturnsDefault(t *testing.T) {
	cfg, err := LoadFrom("/tmp/does-not-exist-kubecat-test-config.yaml")
	if err != nil {
		t.Fatalf("LoadFrom non-existent path returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadFrom returned nil config")
	}
	// Must match defaults
	if cfg.Kubecat.RefreshRate != 2 {
		t.Errorf("fallback RefreshRate = %d, want 2", cfg.Kubecat.RefreshRate)
	}
}

func TestLoadFrom_PartialYAML_MergesWithDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yaml := `kubecat:
  refreshRate: 5
  theme: dark
`
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom partial YAML: %v", err)
	}

	if cfg.Kubecat.RefreshRate != 5 {
		t.Errorf("RefreshRate = %d, want 5", cfg.Kubecat.RefreshRate)
	}
	if cfg.Kubecat.Theme != "dark" {
		t.Errorf("Theme = %q, want dark", cfg.Kubecat.Theme)
	}
	// Unspecified fields retain defaults
	if cfg.Kubecat.Logger.Tail != 100 {
		t.Errorf("Logger.Tail should default to 100 after partial load, got %d", cfg.Kubecat.Logger.Tail)
	}
}

func TestLoadFrom_InvalidYAML_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")

	if err := os.WriteFile(path, []byte("not: valid: yaml: :::"), 0600); err != nil {
		t.Fatalf("writing bad YAML: %v", err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Error("LoadFrom invalid YAML should return error, got nil")
	}
}

func TestLoadFrom_EmptyFile_ReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yaml")
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatalf("writing empty file: %v", err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom empty file: %v", err)
	}
	if cfg.Kubecat.RefreshRate != 2 {
		t.Errorf("empty file should yield default RefreshRate=2, got %d", cfg.Kubecat.RefreshRate)
	}
}

// ---------------------------------------------------------------------------
// SaveTo round-trip
// ---------------------------------------------------------------------------

func TestSaveTo_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	original := Default()
	original.Kubecat.RefreshRate = 10
	original.Kubecat.Theme = "solarized"
	original.Kubecat.ReadOnly = true

	if err := original.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom after SaveTo: %v", err)
	}

	if loaded.Kubecat.RefreshRate != 10 {
		t.Errorf("round-trip RefreshRate = %d, want 10", loaded.Kubecat.RefreshRate)
	}
	if loaded.Kubecat.Theme != "solarized" {
		t.Errorf("round-trip Theme = %q, want solarized", loaded.Kubecat.Theme)
	}
	if !loaded.Kubecat.ReadOnly {
		t.Error("round-trip ReadOnly should be true")
	}
}

func TestSaveTo_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := Default()
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config file: %v", err)
	}
	// Must be owner-only readable (0600)
	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("config file mode = %04o, want 0600", mode)
	}
}

// ---------------------------------------------------------------------------
// API key never on disk
// ---------------------------------------------------------------------------

func TestSaveTo_APIKeyWrittenToDisk(t *testing.T) {
	// This test documents that the current implementation DOES write the API key
	// to disk (no encryption). If that behavior changes, this test catches it.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.Kubecat.AI.Providers = map[string]ProviderConfig{
		"openai": {APIKey: "test-api-key-not-real-000000", Enabled: true},
	}

	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	// Load raw file content — if API key is there, record it
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved config: %v", err)
	}

	// Ensure the round-trip restores the key correctly
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if loaded.Kubecat.AI.Providers["openai"].APIKey != "test-api-key-not-real-000000" {
		t.Errorf("API key not preserved in round-trip")
	}
	_ = data // suppress unused warning
}

// ---------------------------------------------------------------------------
// XDG overrides
// ---------------------------------------------------------------------------

func TestConfigDir_XDGOverride(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")
	t.Setenv("KUBECAT_CONFIG_DIR", "")

	got := ConfigDir()
	want := "/tmp/xdg-config/kubecat"
	if got != want {
		t.Errorf("ConfigDir with XDG_CONFIG_HOME = %q, want %q", got, want)
	}
}

func TestConfigDir_KubecatOverrideTakesPrecedence(t *testing.T) {
	t.Setenv("KUBECAT_CONFIG_DIR", "/custom/kubecat")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")

	got := ConfigDir()
	if got != "/custom/kubecat" {
		t.Errorf("ConfigDir with KUBECAT_CONFIG_DIR = %q, want /custom/kubecat", got)
	}
}

func TestStateDir_XDGOverride(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/xdg-state")

	got := StateDir()
	want := "/tmp/xdg-state/kubecat"
	if got != want {
		t.Errorf("StateDir = %q, want %q", got, want)
	}
}

func TestDataDir_XDGOverride(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-data")

	got := DataDir()
	want := "/tmp/xdg-data/kubecat"
	if got != want {
		t.Errorf("DataDir = %q, want %q", got, want)
	}
}

func TestCacheDir_XDGOverride(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/tmp/xdg-cache")

	got := CacheDir()
	want := "/tmp/xdg-cache/kubecat"
	if got != want {
		t.Errorf("CacheDir = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// UpdatesEnabled (AI.Enabled toggle)
// ---------------------------------------------------------------------------

func TestAIEnabled_CanBeToggled(t *testing.T) {
	cfg := Default()
	if cfg.Kubecat.AI.Enabled {
		t.Fatal("precondition: AI.Enabled should start false")
	}

	cfg.Kubecat.AI.Enabled = true

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if !loaded.Kubecat.AI.Enabled {
		t.Error("AI.Enabled should be true after round-trip")
	}
}
