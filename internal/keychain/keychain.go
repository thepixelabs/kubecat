// Package keychain wraps the OS credential store via go-keyring.
// On systems where the keychain is unavailable (headless servers, CI), it
// falls back to config-file storage with a logged warning.
package keychain

import (
	"errors"
	"log/slog"
	"sync"

	"github.com/zalando/go-keyring"
)

const service = "kubecat"

var (
	// probeOnce ensures the availability check runs exactly once.
	probeOnce sync.Once
	available bool
)

// IsAvailable returns true when the OS keychain can be used on this system.
// The result is cached after the first call.
func IsAvailable() bool {
	probeOnce.Do(func() {
		const probeKey = "__kubecat_probe__"
		// Attempt a round-trip to determine real availability.
		err := keyring.Set(service, probeKey, "1")
		if err != nil {
			slog.Warn("keychain: OS keyring unavailable, falling back to config file",
				slog.Any("error", err))
			available = false
			return
		}
		_ = keyring.Delete(service, probeKey)
		available = true
	})
	return available
}

// Set stores a secret in the OS keychain under the given key.
// Returns an error if the keychain is unavailable.
func Set(key, value string) error {
	if !IsAvailable() {
		return errors.New("keychain: OS keyring not available on this system")
	}
	return keyring.Set(service, key, value)
}

// Get retrieves a secret from the OS keychain.
// Returns ("", ErrNotFound) when the key does not exist.
func Get(key string) (string, error) {
	if !IsAvailable() {
		return "", errors.New("keychain: OS keyring not available on this system")
	}
	return keyring.Get(service, key)
}

// Delete removes a key from the OS keychain.
// Returns nil if the key did not exist.
func Delete(key string) error {
	if !IsAvailable() {
		return nil
	}
	err := keyring.Delete(service, key)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

// GetOrFallback tries the OS keychain first; if the key is missing or the
// keychain is unavailable it returns fallback instead.
// A warning is logged when the keychain is unavailable and fallback is non-empty.
func GetOrFallback(key, fallback string) string {
	if !IsAvailable() {
		if fallback != "" {
			slog.Warn("keychain: using config-file API key because OS keyring is unavailable",
				slog.String("key", key))
		}
		return fallback
	}

	val, err := keyring.Get(service, key)
	if err != nil {
		// Key not in keychain yet — use fallback silently.
		return fallback
	}
	return val
}
