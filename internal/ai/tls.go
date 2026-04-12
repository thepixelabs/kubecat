package ai

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// NewCloudHTTPClient returns a hardened *http.Client suitable for outbound
// connections to cloud AI provider APIs.
//
// Security posture (Mozilla Intermediate profile):
//   - TLS 1.2 minimum (TLS 1.3 preferred by Go's default negotiation)
//   - ECDHE key exchange only
//   - AESGCM and ChaCha20-Poly1305 AEAD ciphers
//   - No RC4, no 3DES, no CBC-mode suites
func NewCloudHTTPClient(timeout time.Duration) *http.Client {
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			// TLS 1.2 ECDHE + AEAD suites (Mozilla Intermediate).
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		},
	}

	transport := &http.Transport{
		TLSClientConfig: tlsCfg,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}
