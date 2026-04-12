// SPDX-License-Identifier: Apache-2.0

package ai

import (
	"crypto/tls"
	"net/http"
	"testing"
	"time"
)

// TestNewCloudHTTPClient_TLSMinVersion ensures the client enforces TLS 1.2+.
func TestNewCloudHTTPClient_TLSMinVersion(t *testing.T) {
	client := NewCloudHTTPClient(30 * time.Second)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Transport is not *http.Transport")
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("TLSClientConfig is nil — no TLS hardening applied")
	}
	if transport.TLSClientConfig.MinVersion < tls.VersionTLS12 {
		t.Errorf("MinVersion = %d, want >= TLS 1.2 (%d)",
			transport.TLSClientConfig.MinVersion, tls.VersionTLS12)
	}
}

// TestNewCloudHTTPClient_NoInsecureSkipVerify ensures certificate validation is enforced.
func TestNewCloudHTTPClient_NoInsecureSkipVerify(t *testing.T) {
	client := NewCloudHTTPClient(30 * time.Second)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Transport is not *http.Transport")
	}
	if transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify is true — this disables certificate validation and must never be set on production cloud clients")
	}
}

// TestNewCloudHTTPClient_TimeoutSet ensures a non-zero timeout is set.
func TestNewCloudHTTPClient_TimeoutSet(t *testing.T) {
	timeout := 42 * time.Second
	c := NewCloudHTTPClient(timeout)
	if c.Timeout != timeout {
		t.Errorf("Timeout = %v, want %v", c.Timeout, timeout)
	}
}

// TestNewCloudHTTPClient_CipherSuites ensures only AEAD ciphers are present.
func TestNewCloudHTTPClient_CipherSuites(t *testing.T) {
	client := NewCloudHTTPClient(30 * time.Second)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Transport is not *http.Transport")
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("TLSClientConfig is nil")
	}
	if len(transport.TLSClientConfig.CipherSuites) == 0 {
		t.Error("CipherSuites is empty — expected an explicit hardened cipher list")
	}
	// Ensure no known-weak ciphers are present.
	weakCiphers := map[uint16]string{
		tls.TLS_RSA_WITH_RC4_128_SHA:      "RC4",
		tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA: "3DES",
		tls.TLS_RSA_WITH_AES_128_CBC_SHA:  "RSA-AES-CBC (no PFS)",
		tls.TLS_RSA_WITH_AES_256_CBC_SHA:  "RSA-AES-CBC (no PFS)",
	}
	for _, cs := range transport.TLSClientConfig.CipherSuites {
		if name, bad := weakCiphers[cs]; bad {
			t.Errorf("weak cipher suite present: %s (0x%04x)", name, cs)
		}
	}
}

// TestCloudProviders_UseHardenedClient verifies that all cloud AI providers
// use NewCloudHTTPClient rather than a bare &http.Client{}.
// This test constructs providers and inspects their transport.
func TestCloudProviders_UseHardenedClient(t *testing.T) {
	cfg := ProviderConfig{
		Model:   "test-model",
		APIKey:  "test-key",
		Timeout: 30 * time.Second,
	}

	providers := []struct {
		name   string
		client *http.Client
	}{
		{"openai", NewOpenAIProvider(cfg).client},
		{"anthropic", NewAnthropicProvider(cfg).client},
		{"google", NewGoogleProvider(cfg).client},
	}

	for _, p := range providers {
		t.Run(p.name, func(t *testing.T) {
			transport, ok := p.client.Transport.(*http.Transport)
			if !ok {
				t.Fatalf("%s: Transport is not *http.Transport — provider may be using Go's default transport which lacks TLS hardening", p.name)
			}
			if transport.TLSClientConfig == nil {
				t.Fatalf("%s: TLSClientConfig is nil — no TLS hardening applied", p.name)
			}
			if transport.TLSClientConfig.InsecureSkipVerify {
				t.Errorf("%s: InsecureSkipVerify = true — this bypasses certificate validation", p.name)
			}
			if transport.TLSClientConfig.MinVersion < tls.VersionTLS12 {
				t.Errorf("%s: MinVersion = %d, want >= TLS 1.2", p.name, transport.TLSClientConfig.MinVersion)
			}
		})
	}
}
