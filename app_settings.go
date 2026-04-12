package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/thepixelabs/kubecat/internal/audit"
	"github.com/thepixelabs/kubecat/internal/config"
	"github.com/thepixelabs/kubecat/internal/network"
)

// AISettings is a JSON-friendly AI configuration.
type AISettings struct {
	Enabled          bool                      `json:"enabled"`
	SelectedProvider string                    `json:"selectedProvider"`
	SelectedModel    string                    `json:"selectedModel"`
	Providers        map[string]ProviderConfig `json:"providers"`
}

// ProviderConfig contains settings for a specific AI provider.
type ProviderConfig struct {
	Enabled  bool     `json:"enabled"`
	APIKey   string   `json:"apiKey"`
	Endpoint string   `json:"endpoint"`
	Models   []string `json:"models"`
}

// AIContextItem represents a resource attached to a query.
type AIContextItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	Cluster   string `json:"cluster"`
}

// ProviderInfo contains information about an AI provider.
type ProviderInfo struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	RequiresAPIKey  bool     `json:"requiresApiKey"`
	DefaultEndpoint string   `json:"defaultEndpoint"`
	DefaultModel    string   `json:"defaultModel"`
	Models          []string `json:"models"`
}

// GetAISettings returns the current AI configuration.
func (a *App) GetAISettings() (*AISettings, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	// Map internal config to JSON-friendly struct
	providers := make(map[string]ProviderConfig)
	for k, v := range cfg.Kubecat.AI.Providers {
		providers[k] = ProviderConfig{
			Enabled:  v.Enabled,
			APIKey:   v.APIKey,
			Endpoint: v.Endpoint,
			Models:   v.Models,
		}
	}

	return &AISettings{
		Enabled:          cfg.Kubecat.AI.Enabled,
		SelectedProvider: cfg.Kubecat.AI.SelectedProvider,
		SelectedModel:    cfg.Kubecat.AI.SelectedModel,
		Providers:        providers,
	}, nil
}

// SaveAISettings saves the AI configuration.
func (a *App) SaveAISettings(settings AISettings) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	cfg.Kubecat.AI.Enabled = settings.Enabled
	cfg.Kubecat.AI.SelectedProvider = settings.SelectedProvider
	cfg.Kubecat.AI.SelectedModel = settings.SelectedModel

	// Update providers
	if cfg.Kubecat.AI.Providers == nil {
		cfg.Kubecat.AI.Providers = make(map[string]config.ProviderConfig)
	}

	for k, v := range settings.Providers {
		endpoint := v.Endpoint
		if endpoint != "" && !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
			// For Ollama (loopback-only), allow http://; for all other providers require https://.
			// Previously used strings.Contains(endpoint, "localhost") which is bypassable
			// (e.g. "evil.example.com?x=localhost" would wrongly get http://).
			if k == "ollama" {
				endpoint = "http://" + endpoint
			} else {
				endpoint = "https://" + endpoint
			}
		}

		// SSRF guard at save-time: reject dangerous endpoints before persisting to
		// disk.  This prevents a malicious endpoint (e.g. 169.254.169.254) from
		// being stored and later dispatched by AIQuery without a network call.
		if endpoint != "" {
			if err := validateProviderEndpoint(k, endpoint); err != nil {
				return fmt.Errorf("invalid endpoint for provider %q: %w", k, err)
			}
		}

		cfg.Kubecat.AI.Providers[k] = config.ProviderConfig{
			Enabled:  v.Enabled,
			APIKey:   v.APIKey,
			Endpoint: endpoint,
			Models:   v.Models,
		}
		audit.LogProviderConfig(k)
	}

	return cfg.Save()
}

// GetAvailableProviders returns the list of available AI providers.
func (a *App) GetAvailableProviders() []ProviderInfo {
	return []ProviderInfo{
		{
			ID:              "openai",
			Name:            "OpenAI (ChatGPT)",
			RequiresAPIKey:  true,
			DefaultEndpoint: "https://api.openai.com/v1",
			DefaultModel:    "gpt-4",
			Models:          []string{"gpt-4", "gpt-4-turbo", "gpt-4o", "gpt-4o-mini", "gpt-3.5-turbo"},
		},
		{
			ID:              "google",
			Name:            "Google (Gemini)",
			RequiresAPIKey:  true,
			DefaultEndpoint: "https://generativelanguage.googleapis.com/v1beta",
			DefaultModel:    "gemini-pro",
			Models:          []string{"gemini-pro", "gemini-1.5-pro", "gemini-1.5-flash"},
		},
		{
			ID:              "anthropic",
			Name:            "Anthropic",
			RequiresAPIKey:  true,
			DefaultEndpoint: "https://api.anthropic.com/v1",
			DefaultModel:    "claude-3-sonnet-20240229",
			Models:          []string{"claude-3-opus-20240229", "claude-3-sonnet-20240229", "claude-3-haiku-20240307"},
		},
		{
			ID:              "ollama",
			Name:            "Ollama (Local)",
			RequiresAPIKey:  false,
			DefaultEndpoint: "http://localhost:11434",
			DefaultModel:    "llama3.2",
			Models:          []string{"llama3.2", "llama3.1", "codellama", "mistral", "mixtral"},
		},
		{
			ID:              "litellm",
			Name:            "LiteLLM",
			RequiresAPIKey:  true,
			DefaultEndpoint: "http://localhost:4000",
			DefaultModel:    "gpt-4",
			Models:          []string{},
		},
	}
}

// blockedIP returns true if the IP falls in any range that must never be contacted
// from a user-supplied endpoint: loopback, RFC-1918 private, and link-local
// (which covers the AWS/GCP/Azure IMDS metadata services at 169.254.169.254).
func blockedIP(ip net.IP) bool {
	// Normalise to 16-byte form so both IPv4 and IPv6 comparisons work correctly.
	ip = ip.To16()
	if ip == nil {
		return true // unparseable — deny
	}

	blocked := []struct{ network, mask string }{
		// Loopback
		{"127.0.0.0", "255.0.0.0"},
		// IPv6 loopback ::1
		{"::1", ""},
		// RFC-1918 private
		{"10.0.0.0", "255.0.0.0"},
		{"172.16.0.0", "255.240.0.0"},
		{"192.168.0.0", "255.255.0.0"},
		// Link-local (covers 169.254.169.254 metadata endpoints)
		{"169.254.0.0", "255.255.0.0"},
		// IPv6 link-local fe80::/10
		{"fe80::", ""},
		// IPv6 unique-local fc00::/7
		{"fc00::", ""},
	}

	for _, b := range blocked {
		if b.mask == "" {
			// Single address comparison (IPv6 special cases).
			single := net.ParseIP(b.network)
			if single != nil && ip.Equal(single.To16()) {
				return true
			}
			// For prefix ranges specified without a mask, fall through to CIDR checks below.
			continue
		}
		_, cidr, err := net.ParseCIDR(b.network + "/" + maskToCIDRBits(b.mask))
		if err == nil && cidr.Contains(ip) {
			return true
		}
	}

	// IPv6 link-local fe80::/10
	if _, fe80, _ := net.ParseCIDR("fe80::/10"); fe80 != nil && fe80.Contains(ip) {
		return true
	}
	// IPv6 unique-local fc00::/7
	if _, fc00, _ := net.ParseCIDR("fc00::/7"); fc00 != nil && fc00.Contains(ip) {
		return true
	}

	return false
}

// maskToCIDRBits converts a dotted-decimal subnet mask to CIDR prefix length string.
func maskToCIDRBits(mask string) string {
	parts := strings.Split(mask, ".")
	if len(parts) != 4 {
		return "0"
	}
	bits := 0
	for _, p := range parts {
		var b int
		_, _ = fmt.Sscanf(p, "%d", &b)
		for b > 0 {
			bits += b & 1
			b >>= 1
		}
	}
	return fmt.Sprintf("%d", bits)
}

// validateProviderEndpoint enforces SSRF protection on a user-supplied endpoint URL.
//
// Security model:
//   - openai / anthropic / google: only their canonical HTTPS base URLs are accepted;
//     no custom endpoint is permitted (these are SaaS APIs with fixed addresses).
//   - ollama: only loopback addresses (localhost / 127.0.0.1) are permitted because
//     Ollama is a local-only service by design.
//   - litellm: any HTTPS endpoint is accepted, but the hostname is resolved via DNS
//     and every resulting IP is checked against blocked ranges to prevent SSRF and
//     DNS-rebinding attacks.
//
// The function returns a non-nil error if the endpoint is unsafe or invalid.
func validateProviderEndpoint(provider, endpoint string) error {
	// Known-good canonical base URLs for SaaS providers — no custom override allowed.
	canonicalURLs := map[string]string{
		"openai":    "https://api.openai.com/v1",
		"anthropic": "https://api.anthropic.com/v1",
		"google":    "https://generativelanguage.googleapis.com/v1beta",
	}

	if canon, ok := canonicalURLs[provider]; ok {
		if endpoint != canon {
			return fmt.Errorf("provider %q only allows the canonical endpoint %q; custom endpoints are not permitted", provider, canon)
		}
		// Canonical URL is trusted — no further checks required.
		return nil
	}

	// Ollama is loopback-only.
	if provider == "ollama" {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return fmt.Errorf("invalid ollama endpoint: %w", err)
		}
		host := parsed.Hostname()
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			return fmt.Errorf("ollama endpoint must be localhost or 127.0.0.1, got %q", host)
		}
		return nil
	}

	// litellm (and any future custom provider): require HTTPS, then DNS-resolve and
	// check every returned IP against blocked ranges.
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	if parsed.Scheme != "https" {
		return fmt.Errorf("endpoint must use HTTPS, got scheme %q", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("endpoint URL contains no hostname")
	}

	// Resolve the hostname now (before the HTTP client gets to it) to prevent
	// DNS-rebinding: if the DNS answer already points at an internal IP we block it
	// before any TCP connection is made.
	addrs, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("endpoint hostname %q could not be resolved: %w", host, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("endpoint hostname %q resolved to no addresses", host)
	}

	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			return fmt.Errorf("endpoint hostname %q resolved to unparseable address %q", host, addr)
		}
		if blockedIP(ip) {
			return fmt.Errorf("endpoint hostname %q resolved to a blocked internal address %q", host, addr)
		}
	}

	return nil
}

// FetchProviderModels fetches available models from a provider's API endpoint.
func (a *App) FetchProviderModels(provider, endpoint, apiKey string) ([]string, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	var modelsEndpoint string
	var authHeader string
	var authValue string

	// Ensure defaults if endpoint is empty.
	if endpoint == "" {
		switch provider {
		case "openai":
			endpoint = "https://api.openai.com/v1"
		case "google":
			endpoint = "https://generativelanguage.googleapis.com/v1beta"
		case "anthropic":
			endpoint = "https://api.anthropic.com/v1"
		case "ollama":
			endpoint = "http://localhost:11434"
		}
	} else if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		// Add scheme only for ollama (loopback-only, validated below).
		// For all other providers a scheme-less endpoint is rejected by validateProviderEndpoint
		// because they require explicit HTTPS. Prepending http:// here for ollama is safe
		// because validateProviderEndpoint will enforce the loopback constraint.
		if provider == "ollama" {
			endpoint = "http://" + endpoint
		} else {
			endpoint = "https://" + endpoint
		}
	}

	// Clean trailing slash.
	endpoint = strings.TrimRight(endpoint, "/")

	// SSRF protection (primary): validate the resolved endpoint before making any network call.
	if err := validateProviderEndpoint(provider, endpoint); err != nil {
		return nil, fmt.Errorf("endpoint validation failed: %w", err)
	}

	// SSRF protection (secondary / defense-in-depth): run the centralized
	// network.Validate check which enforces the CIDR blocklist and the
	// metadata-hostname blocklist independently of the primary check.
	if err := network.Validate(endpoint, provider); err != nil {
		return nil, fmt.Errorf("endpoint blocked by network policy: %w", err)
	}

	switch provider {
	case "openai", "litellm":
		modelsEndpoint = endpoint + "/models"
		authHeader = "Authorization"
		authValue = "Bearer " + apiKey
	case "anthropic":
		// Anthropic doesn't have a standardized public models endpoint via API yet, return structured defaults
		return []string{"claude-3-opus-20240229", "claude-3-sonnet-20240229", "claude-3-haiku-20240307", "claude-3-5-sonnet-20241022"}, nil
	case "google":
		modelsEndpoint = endpoint + "/models"
		modelsEndpoint += "?key=" + apiKey
	case "ollama":
		modelsEndpoint = endpoint + "/api/tags"
		// Ollama doesn't need auth usually
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if authHeader != "" && authValue != "" {
		req.Header.Set(authHeader, authValue)
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch models: %s - %s", resp.Status, string(body))
	}

	var models []string

	switch provider {
	case "openai", "litellm":
		var result struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		for _, m := range result.Data {
			models = append(models, m.ID)
		}
	case "google":
		var result struct {
			Models []struct {
				Name        string `json:"name"`
				DisplayName string `json:"displayName"`
			} `json:"models"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		for _, m := range result.Models {
			// Google returns "models/gemini-pro", strip prefix for cleaner UI if desired,
			// or keep it. The provider expects full name or name without prefix.
			// Let's keep it clean: "gemini-pro"
			name := strings.TrimPrefix(m.Name, "models/")
			models = append(models, name)
		}

	case "ollama":
		var result struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		for _, m := range result.Models {
			models = append(models, m.Name)
		}
	}

	return models, nil
}
