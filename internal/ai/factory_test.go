// SPDX-License-Identifier: Apache-2.0

package ai

import (
	"strings"
	"testing"
)

// TestNewProvider_KnownProvidersReturnTypedImplementations verifies that
// NewProvider dispatches to the correct concrete type for every provider ID
// the configuration layer is expected to produce.
func TestNewProvider_KnownProvidersReturnTypedImplementations(t *testing.T) {
	cfg := ProviderConfig{Model: "m", APIKey: "k"}

	cases := []struct {
		id       string
		wantName string // value returned by provider.Name()
	}{
		{"openai", "openai"},
		{"litellm", "openai"}, // litellm is served by OpenAIProvider
		{"anthropic", "anthropic"},
		{"google", "google"},
		{"ollama", "ollama"},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			p, err := NewProvider(tc.id, cfg)
			if err != nil {
				t.Fatalf("NewProvider(%q) returned error: %v", tc.id, err)
			}
			if p == nil {
				t.Fatalf("NewProvider(%q) returned nil", tc.id)
			}
			if name := p.Name(); name != tc.wantName {
				t.Errorf("Name() = %q, want %q", name, tc.wantName)
			}
			if err := p.Close(); err != nil {
				t.Errorf("Close() = %v, want nil", err)
			}
		})
	}
}

// TestNewProvider_UnknownID returns an error rather than panicking.
func TestNewProvider_UnknownID(t *testing.T) {
	_, err := NewProvider("mystery-provider", ProviderConfig{})
	if err == nil {
		t.Fatal("NewProvider with unknown id should return error")
	}
	if !strings.Contains(err.Error(), "unknown AI provider") {
		t.Errorf("error should mention 'unknown AI provider', got: %v", err)
	}
}

// TestDefaultProviderConfig_SensibleDefaults ensures the default config does
// not silently zero out timeouts/token limits — a zero timeout would hang
// the UI indefinitely on a misbehaving provider.
func TestDefaultProviderConfig_SensibleDefaults(t *testing.T) {
	cfg := DefaultProviderConfig()
	if cfg.Timeout <= 0 {
		t.Errorf("DefaultProviderConfig.Timeout = %v, want > 0", cfg.Timeout)
	}
	if cfg.MaxTokens <= 0 {
		t.Errorf("DefaultProviderConfig.MaxTokens = %d, want > 0", cfg.MaxTokens)
	}
	if cfg.Temperature < 0 || cfg.Temperature > 1 {
		t.Errorf("DefaultProviderConfig.Temperature = %v, want in [0,1]", cfg.Temperature)
	}
}
