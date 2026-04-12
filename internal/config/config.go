// Package config provides configuration management for Kubecat.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the main Kubecat configuration.
type Config struct {
	Kubecat KubecatConfig `yaml:"kubecat"`
}

// KubecatConfig holds all Kubecat-specific settings.
type KubecatConfig struct {
	// RefreshRate is the UI refresh interval in seconds.
	RefreshRate int `yaml:"refreshRate"`

	// Theme is the name of the active theme.
	Theme string `yaml:"theme"`

	// ReadOnly disables all modification commands.
	ReadOnly bool `yaml:"readOnly"`

	// UI contains user interface settings.
	UI UIConfig `yaml:"ui"`

	// AI contains AI/LLM configuration.
	AI AIConfig `yaml:"ai"`

	// ActiveContext is the last active kubeconfig context used by Kubecat.
	// This is separate from kubeconfig's "current-context".
	ActiveContext string `yaml:"activeContext"`

	// Clusters is a list of cluster configurations.
	Clusters []ClusterConfig `yaml:"clusters"`

	// Logger contains logging configuration.
	Logger LoggerConfig `yaml:"logger"`

	// Alerts contains proactive alert monitor configuration.
	Alerts AlertsConfig `yaml:"alerts"`

	// Telemetry contains anonymous usage telemetry settings.
	Telemetry TelemetryConfig `yaml:"telemetry"`

	// AgentGuardrails configures the AI agent safety guardrails.
	AgentGuardrails AgentGuardrailsConfig `yaml:"agentGuardrails"`

	// Storage configures data retention for the history database.
	Storage StorageConfig `yaml:"storage"`

	// CheckForUpdates enables automatic GitHub release checks.
	CheckForUpdates bool `yaml:"checkForUpdates"`

	// MCP configures the Model Context Protocol server.
	MCP MCPConfig `yaml:"mcp"`

	// Cost configures resource cost estimation.
	Cost CostConfig `yaml:"cost"`
}

// MCPConfig configures the MCP server.
type MCPConfig struct {
	// Enabled starts the MCP stdio server on app launch.
	Enabled bool `yaml:"enabled"`
}

// CostConfig configures resource cost estimation heuristics.
type CostConfig struct {
	// CPUCostPerCoreHour is the $/core/hour for CPU requests (default 0.048).
	CPUCostPerCoreHour float64 `yaml:"cpuCostPerCoreHour"`
	// MemCostPerGBHour is the $/GB/hour for memory requests (default 0.006).
	MemCostPerGBHour float64 `yaml:"memCostPerGBHour"`
	// Currency is the display currency (default USD).
	Currency string `yaml:"currency"`
}

// AlertsConfig configures the proactive alert monitor.
type AlertsConfig struct {
	// Enabled controls whether the alert monitor runs.
	Enabled bool `yaml:"enabled"`
	// ScanIntervalSeconds is the number of seconds between cluster scans.
	ScanIntervalSeconds int `yaml:"scanIntervalSeconds"`
	// CooldownMinutes is the minimum minutes between repeated alerts for the
	// same issue.
	CooldownMinutes int `yaml:"cooldownMinutes"`
	// IgnoredNamespaces lists namespaces that should not trigger alerts.
	IgnoredNamespaces []string `yaml:"ignoredNamespaces"`
}

// TelemetryConfig configures anonymous usage telemetry.
type TelemetryConfig struct {
	// Enabled controls whether telemetry data is collected.
	Enabled bool `yaml:"enabled"`
	// AnonymousID is the anonymous installation identifier.
	AnonymousID string `yaml:"anonymousId"`
}

// AgentGuardrailsConfig configures AI agent safety limits.
type AgentGuardrailsConfig struct {
	// AllowedNamespaces restricts tool calls to these namespaces (empty = all).
	AllowedNamespaces []string `yaml:"allowedNamespaces"`
	// ProtectedNamespaces blocks write/destructive tools in these namespaces.
	ProtectedNamespaces []string `yaml:"protectedNamespaces"`
	// BlockDestructive disallows all destructive tools.
	BlockDestructive bool `yaml:"blockDestructive"`
	// RequireDoubleConfirm requires a second approval for destructive tools.
	RequireDoubleConfirm bool `yaml:"requireDoubleConfirm"`
	// SessionRateLimit is the maximum tool calls per 60s window (0 = unlimited).
	SessionRateLimit int `yaml:"sessionRateLimit"`
	// SessionToolCap is the maximum total tool calls per agent session (0 = unlimited).
	SessionToolCap int `yaml:"sessionToolCap"`
	// TokenBudget is the maximum tokens the agent may consume (0 = unlimited).
	TokenBudget int `yaml:"tokenBudget"`
	// AllowProductionDestructive enables destructive operations on clusters whose
	// context name matches *prod*. Defaults to false for safety.
	AllowProductionDestructive bool `yaml:"allowProductionDestructive"`
}

// StorageConfig configures data retention for the history database.
type StorageConfig struct {
	// EventsRetentionDays sets how many days of events to keep (default 30).
	EventsRetentionDays int `yaml:"eventsRetentionDays"`
	// SnapshotsRetentionDays sets how many days of snapshots to keep (default 7).
	SnapshotsRetentionDays int `yaml:"snapshotsRetentionDays"`
	// CorrelationsRetentionDays sets how many days of correlations to keep (default 30).
	CorrelationsRetentionDays int `yaml:"correlationsRetentionDays"`
}

// UIConfig contains user interface settings.
type UIConfig struct {
	// EnableMouse enables mouse support.
	EnableMouse bool `yaml:"enableMouse"`

	// Headless hides the header.
	Headless bool `yaml:"headless"`

	// NoIcons disables icon display.
	NoIcons bool `yaml:"noIcons"`
}

// AIConfig contains AI/LLM settings.
type AIConfig struct {
	// Enabled enables AI features.
	Enabled bool `yaml:"enabled"`

	// SelectedProvider is the currently selected provider ID.
	SelectedProvider string `yaml:"selectedProvider"`

	// SelectedModel is the currently selected model name.
	SelectedModel string `yaml:"selectedModel"`

	// Providers contains configuration for each provider.
	Providers map[string]ProviderConfig `yaml:"providers"`

	// ConsentGiven records whether the user has acknowledged the
	// cloud AI data-transmission disclosure for the currently selected
	// non-Ollama provider. False or absent means a fresh consent prompt
	// must be shown before any cloud query is dispatched.
	ConsentGiven bool `yaml:"aiConsentGiven"`

	// ConsentDate is the ISO-8601 date the user gave consent.
	// Stored for audit / re-prompt logic on policy changes.
	ConsentDate string `yaml:"aiConsentDate"`

	// ConsentProvider records which provider the consent was granted for,
	// so switching providers re-prompts even if a previous consent exists.
	ConsentProvider string `yaml:"aiConsentProvider"`
}

// ProviderConfig contains settings for a specific AI provider.
type ProviderConfig struct {
	// Enabled enables this specific provider.
	Enabled bool `yaml:"enabled"`

	// APIKey is the API key for this provider.
	APIKey string `yaml:"apiKey"`

	// Endpoint is a custom API endpoint.
	Endpoint string `yaml:"endpoint"`

	// Models is a list of enabled models for this provider.
	Models []string `yaml:"models"`
}

// ClusterConfig represents a cluster configuration.
type ClusterConfig struct {
	// Name is a friendly name for the cluster.
	Name string `yaml:"name"`

	// Context is the kubeconfig context name.
	Context string `yaml:"context"`

	// Namespace is the default namespace.
	Namespace string `yaml:"namespace"`

	// ReadOnly makes this cluster read-only.
	ReadOnly bool `yaml:"readOnly"`
}

// LoggerConfig contains logging settings.
type LoggerConfig struct {
	// Tail is the number of log lines to display.
	Tail int `yaml:"tail"`

	// Buffer is the maximum log buffer size.
	Buffer int `yaml:"buffer"`

	// ShowTime displays timestamps in logs.
	ShowTime bool `yaml:"showTime"`

	// TextWrap enables log line wrapping.
	TextWrap bool `yaml:"textWrap"`

	// LogLevel controls the minimum log level written to the log file.
	// Accepted values: "debug", "info", "warn", "error". Default: "info".
	LogLevel string `yaml:"logLevel"`
}

// Default returns the default configuration.
func Default() *Config {
	return &Config{
		Kubecat: KubecatConfig{
			RefreshRate: 2,
			Theme:       "default",
			ReadOnly:    false,
			UI: UIConfig{
				EnableMouse: false,
				Headless:    false,
				NoIcons:     false,
			},
			AI: AIConfig{
				Enabled:          false,
				SelectedProvider: "ollama",
				SelectedModel:    "llama3.2",
				Providers: map[string]ProviderConfig{
					"ollama": {
						Enabled:  true,
						Endpoint: "http://localhost:11434",
						Models:   []string{"llama3.2"},
					},
				},
			},
			ActiveContext:   "",
			Clusters:        []ClusterConfig{},
			CheckForUpdates: false,
			Logger: LoggerConfig{
				Tail:     100,
				Buffer:   5000,
				ShowTime: false,
				TextWrap: false,
				LogLevel: "info",
			},
		},
	}
}

// ConfigDir returns the XDG-compliant configuration directory.
func ConfigDir() string {
	// Support KUBECAT_CONFIG_DIR as a top-level override.
	if dir := strings.TrimSpace(os.Getenv("KUBECAT_CONFIG_DIR")); dir != "" {
		return dir
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "kubecat")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "kubecat")
}

// DataDir returns the XDG-compliant data directory.
func DataDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "kubecat")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "kubecat")
}

// StateDir returns the XDG-compliant state directory.
func StateDir() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "kubecat")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "kubecat")
}

// CacheDir returns the XDG-compliant cache directory.
func CacheDir() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "kubecat")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "kubecat")
}

// Load reads configuration from the default location.
func Load() (*Config, error) {
	configPath := filepath.Join(ConfigDir(), "config.yaml")
	return LoadFrom(configPath)
}

// LoadFrom reads configuration from a specific path.
func LoadFrom(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Return default config if file doesn't exist
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes the configuration to the default location.
func (c *Config) Save() error {
	configDir := ConfigDir()
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}

	configPath := filepath.Join(configDir, "config.yaml")
	return c.SaveTo(configPath)
}

// SaveTo writes the configuration to a specific path.
func (c *Config) SaveTo(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}
