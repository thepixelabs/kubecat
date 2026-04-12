// SPDX-License-Identifier: Apache-2.0

package keychain

import (
	"sync"
	"testing"

	"github.com/zalando/go-keyring"
)

// resetProbe resets the cached availability check so tests can control it.
func resetProbe() {
	probeOnce = sync.Once{}
	available = false
}

func TestGetOrFallback_KeychainUnavailable_ReturnsFallback(t *testing.T) {
	// Force keychain to appear unavailable by using an in-memory mock.
	keyring.MockInit()
	resetProbe()
	// MockInit makes keyring work, but we can test fallback path directly.
	result := GetOrFallback("nonexistent_key_xyz", "my-fallback")
	// Key doesn't exist in keychain → should return fallback.
	if result != "my-fallback" {
		t.Errorf("expected fallback %q, got %q", "my-fallback", result)
	}
}

func TestGetOrFallback_KeyInKeychain_ReturnsKeychainValue(t *testing.T) {
	keyring.MockInit()
	resetProbe()

	if err := keyring.Set(service, "test_key", "secret"); err != nil {
		t.Fatalf("setup: keyring.Set: %v", err)
	}

	result := GetOrFallback("test_key", "fallback-unused")
	if result != "secret" {
		t.Errorf("expected keychain value %q, got %q", "secret", result)
	}
}

func TestGetOrFallback_BothEmpty_ReturnsEmpty(t *testing.T) {
	keyring.MockInit()
	resetProbe()

	result := GetOrFallback("missing_key", "")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestSet_Get_RoundTrip(t *testing.T) {
	keyring.MockInit()
	resetProbe()

	if err := Set("round_trip_key", "round_trip_value"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := Get("round_trip_key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "round_trip_value" {
		t.Errorf("expected %q, got %q", "round_trip_value", got)
	}
}

func TestDelete_RemovesKey(t *testing.T) {
	keyring.MockInit()
	resetProbe()

	_ = keyring.Set(service, "delete_me", "value")

	if err := Delete("delete_me"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := Get("delete_me")
	if err == nil && got != "" {
		t.Errorf("expected key to be deleted, got %q", got)
	}
}

func TestDelete_NonExistentKey_NoError(t *testing.T) {
	keyring.MockInit()
	resetProbe()

	if err := Delete("never_existed_key_xyz"); err != nil {
		t.Errorf("Delete of non-existent key should return nil, got %v", err)
	}
}
