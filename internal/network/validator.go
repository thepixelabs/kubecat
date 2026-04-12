// Package network provides centralized SSRF protection for outbound HTTP
// requests made by Kubecat.  All outbound AI provider calls must pass through
// Validate before the request is dispatched.
package network

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// canonicalAllowlist is the set of official cloud provider base-URL prefixes
// that are always permitted regardless of other checks.
var canonicalAllowlist = []string{
	"https://api.anthropic.com/",
	"https://api.openai.com/",
	"https://generativelanguage.googleapis.com/",
	"https://api.groq.com/",
	"https://openrouter.ai/",
	"https://api.mistral.ai/",
	"https://api.cohere.ai/",
	"https://api.together.xyz/",
	"https://api.perplexity.ai/",
}

// metadataHostnames is the set of cloud-metadata hostnames that must never
// be reached (checked before DNS resolution).
var metadataHostnames = map[string]struct{}{
	"169.254.169.254":          {}, // AWS/GCP/Azure IMDS
	"metadata.google.internal": {},
	"metadata":                 {},
	"169.254.170.2":            {}, // AWS ECS metadata
}

// blockedCIDRs lists IP ranges that must never be reached.
var blockedCIDRs []*net.IPNet

func init() {
	rawCIDRs := []string{
		"10.0.0.0/8",     // RFC 1918 private
		"172.16.0.0/12",  // RFC 1918 private
		"192.168.0.0/16", // RFC 1918 private
		"127.0.0.0/8",    // Loopback (IPv4)
		"::1/128",        // Loopback (IPv6)
		"fc00::/7",       // IPv6 ULA
		"169.254.0.0/16", // Link-local / IMDS
		"0.0.0.0/8",      // This network
		"240.0.0.0/4",    // Reserved
		"224.0.0.0/4",    // Multicast
	}
	for _, cidr := range rawCIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			blockedCIDRs = append(blockedCIDRs, ipNet)
		}
	}
}

// Validate checks a raw endpoint URL against five SSRF-protection layers and
// returns an error if the URL should not be contacted.
//
// Layers (in order):
//  1. Provider canonical URL allowlist — cloud URLs always pass.
//  2. Ollama loopback constraint — only localhost/127.x allowed.
//  3. LiteLLM split — litellm:// scheme rejected (not supported).
//  4. Metadata hostname blocklist — pre-DNS check by hostname.
//  5. DNS pre-resolution + blocked CIDR check.
func Validate(rawURL string, providerName string) error {
	// Layer 1: canonical allowlist — short-circuit for known-good cloud URLs.
	if isCanonicalURL(rawURL) {
		return nil
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("ssrf: invalid URL %q: %w", rawURL, err)
	}

	// Layer 2: Ollama must only connect to loopback.
	if strings.EqualFold(providerName, "ollama") {
		host := parsed.Hostname()
		if !isLoopback(host) {
			return fmt.Errorf("ssrf: ollama endpoint must be loopback, got %q", host)
		}
		return nil
	}

	// Layer 3: litellm:// scheme is not supported.
	if strings.EqualFold(parsed.Scheme, "litellm") {
		return fmt.Errorf("ssrf: litellm:// scheme is not supported as a direct endpoint")
	}

	// Layer 4: metadata hostname blocklist (pre-DNS).
	host := strings.ToLower(parsed.Hostname())
	if _, blocked := metadataHostnames[host]; blocked {
		return fmt.Errorf("ssrf: endpoint hostname %q is a blocked metadata address", host)
	}

	// Layer 5: DNS pre-resolution + CIDR check.
	addrs, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("ssrf: DNS resolution failed for %q: %w", host, err)
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		for _, cidr := range blockedCIDRs {
			if cidr.Contains(ip) {
				return fmt.Errorf("ssrf: endpoint %q resolves to blocked IP %s (%s)", host, addr, cidr)
			}
		}
	}

	return nil
}

// isCanonicalURL returns true when rawURL has a prefix matching one of the
// pre-approved cloud provider base URLs.
func isCanonicalURL(rawURL string) bool {
	for _, prefix := range canonicalAllowlist {
		if strings.HasPrefix(rawURL, prefix) {
			return true
		}
	}
	return false
}

// isLoopback returns true for hostnames that resolve to 127.x.x.x or ::1.
func isLoopback(host string) bool {
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	// Check if it parses as a loopback IP directly.
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}
