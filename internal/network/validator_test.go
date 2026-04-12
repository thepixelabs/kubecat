package network

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Layer 1 — canonical allowlist
// ---------------------------------------------------------------------------

func TestValidate_CanonicalCloudURLs_Allowed(t *testing.T) {
	canonicalCases := []struct {
		provider string
		url      string
	}{
		{"openai", "https://api.openai.com/v1/models"},
		{"anthropic", "https://api.anthropic.com/v1/messages"},
		{"google", "https://generativelanguage.googleapis.com/v1beta/models"},
		{"groq", "https://api.groq.com/openai/v1/models"},
		{"openrouter", "https://openrouter.ai/api/v1/models"},
		{"mistral", "https://api.mistral.ai/v1/models"},
		{"cohere", "https://api.cohere.ai/v1/generate"},
		{"together", "https://api.together.xyz/v1/models"},
		{"perplexity", "https://api.perplexity.ai/chat/completions"},
	}

	for _, tc := range canonicalCases {
		t.Run(tc.provider, func(t *testing.T) {
			if err := Validate(tc.url, tc.provider); err != nil {
				t.Errorf("Validate(%q, %q) = %v, want nil (canonical URL must pass)", tc.url, tc.provider, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Layer 2 — Ollama loopback constraint
// ---------------------------------------------------------------------------

func TestValidate_Ollama_LoopbackAllowed(t *testing.T) {
	loopbackURLs := []string{
		"http://localhost:11434",
		"http://127.0.0.1:11434",
		"http://::1:11434",
	}
	for _, u := range loopbackURLs {
		t.Run(u, func(t *testing.T) {
			if err := Validate(u, "ollama"); err != nil {
				t.Errorf("Validate(%q, ollama) = %v, want nil", u, err)
			}
		})
	}
}

func TestValidate_Ollama_NonLoopback_Rejected(t *testing.T) {
	nonLoopback := []string{
		"http://10.0.0.5:11434",
		"http://192.168.1.1:11434",
		"http://evil.example.com:11434",
	}
	for _, u := range nonLoopback {
		t.Run(u, func(t *testing.T) {
			if err := Validate(u, "ollama"); err == nil {
				t.Errorf("Validate(%q, ollama) = nil, want error (non-loopback must be rejected)", u)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Layer 3 — litellm:// scheme rejected
// ---------------------------------------------------------------------------

func TestValidate_LiteLLMScheme_Rejected(t *testing.T) {
	if err := Validate("litellm://some-server/api", "custom"); err == nil {
		t.Error("Validate(litellm://, custom) = nil, want error")
	}
}

// ---------------------------------------------------------------------------
// Layer 4 — metadata hostname blocklist (pre-DNS)
// ---------------------------------------------------------------------------

func TestValidate_MetadataHostnames_Rejected(t *testing.T) {
	metadataURLs := []struct {
		name string
		url  string
	}{
		{"aws-imds", "http://169.254.169.254/latest/meta-data"},
		{"gcp-metadata", "http://metadata.google.internal/computeMetadata/v1/"},
		{"ecs-metadata", "http://169.254.170.2/v2/metadata"},
		{"bare-metadata", "http://metadata/something"},
	}
	for _, tc := range metadataURLs {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.url, "litellm")
			if err == nil {
				t.Errorf("Validate(%q) = nil, want error (metadata address must be blocked)", tc.url)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Layer 5 — blocked CIDR ranges
// ---------------------------------------------------------------------------

func TestValidate_RFC1918Addresses_Rejected(t *testing.T) {
	// Use raw IP addresses (no DNS lookup needed) that fall into blocked CIDRs.
	// We can't make real DNS calls in unit tests, so test the IP-literal path
	// by having the hostname be a dotted-decimal IP that net.LookupHost will
	// return as-is on all platforms.
	blockedIPs := []struct {
		name string
		url  string
	}{
		{"10.x.x.x/8", "https://10.10.10.10/api"},
		{"172.16.x.x/12", "https://172.20.0.1/api"},
		{"192.168.x.x/16", "https://192.168.0.1/api"},
		{"loopback-127.0.0.1", "https://127.0.0.1/api"},
		{"loopback-127.x.x.x", "https://127.99.99.99/api"},
		{"link-local", "https://169.254.1.1/api"},
	}
	for _, tc := range blockedIPs {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.url, "litellm")
			if err == nil {
				t.Errorf("Validate(%q) = nil, want error (blocked IP range)", tc.url)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Invalid URL — should return an error
// ---------------------------------------------------------------------------

func TestValidate_InvalidURL_Rejected(t *testing.T) {
	if err := Validate("://not-a-valid-url", "litellm"); err == nil {
		t.Error("Validate(invalid URL) = nil, want error")
	}
}

// ---------------------------------------------------------------------------
// isLoopback helper
// ---------------------------------------------------------------------------

func TestIsLoopback(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"127.50.50.50", true},
		{"10.0.0.1", false},
		{"192.168.1.1", false},
		{"api.openai.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			got := isLoopback(tc.host)
			if got != tc.want {
				t.Errorf("isLoopback(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isCanonicalURL helper
// ---------------------------------------------------------------------------

func TestIsCanonicalURL(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://api.openai.com/v1/models", true},
		{"https://api.anthropic.com/v1/messages", true},
		{"https://generativelanguage.googleapis.com/v1beta/models", true},
		{"http://api.openai.com/v1", false},         // wrong scheme
		{"https://evil.com/api.openai.com/", false}, // prefix spoofing
		{"https://api.openai.com.evil.com/", false}, // subdomain spoofing
	}
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			got := isCanonicalURL(tc.url)
			if got != tc.want {
				t.Errorf("isCanonicalURL(%q) = %v, want %v", tc.url, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DNS rebinding guard — metadata IPs are blocked even via IP literals
// ---------------------------------------------------------------------------

func TestValidate_MetadataIPLiteral_Rejected(t *testing.T) {
	// 169.254.169.254 is the AWS/GCP/Azure IMDS address.
	// It must be blocked even when specified as a raw IP (no DNS lookup needed).
	err := Validate("http://169.254.169.254/latest/meta-data", "litellm")
	if err == nil {
		t.Error("Validate(169.254.169.254) = nil, want error")
	}
	if !strings.Contains(err.Error(), "ssrf") && !strings.Contains(err.Error(), "blocked") {
		t.Errorf("error message should mention ssrf/blocked, got: %v", err)
	}
}
