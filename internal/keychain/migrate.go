// SPDX-License-Identifier: Apache-2.0

package keychain

import (
	"log/slog"

	"github.com/thepixelabs/kubecat/internal/config"
)

// MigrateAPIKeys walks every configured AI provider.  For each provider whose
// APIKey is non-empty and not already in the keychain, it moves the key into
// the OS keychain, blanks the in-config value, and saves the config.
//
// This is intentionally a best-effort operation: if any step fails we log and
// continue rather than aborting startup.
func MigrateAPIKeys() {
	if !IsAvailable() {
		slog.Warn("keychain: skipping API key migration — OS keyring unavailable")
		return
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Warn("keychain: migration skipped — could not load config", slog.Any("error", err))
		return
	}

	changed := false
	providers := cfg.Kubecat.AI.Providers

	for name, pcfg := range providers {
		if pcfg.APIKey == "" {
			continue
		}

		keychainKey := providerKeychainKey(name)

		// Check whether a value is already stored; avoid overwriting.
		existing, getErr := Get(keychainKey)
		if getErr == nil && existing != "" {
			// Already in keychain — just blank the config copy.
			pcfg.APIKey = ""
			providers[name] = pcfg
			changed = true
			slog.Info("keychain: API key already migrated, clearing config copy",
				slog.String("provider", name))
			continue
		}

		if err := Set(keychainKey, pcfg.APIKey); err != nil {
			slog.Warn("keychain: failed to store API key in keychain",
				slog.String("provider", name),
				slog.Any("error", err))
			continue
		}

		pcfg.APIKey = ""
		providers[name] = pcfg
		changed = true
		slog.Info("keychain: API key migrated to OS keyring",
			slog.String("provider", name))
	}

	if !changed {
		return
	}

	cfg.Kubecat.AI.Providers = providers
	if err := cfg.Save(); err != nil {
		slog.Warn("keychain: migration succeeded but config save failed", slog.Any("error", err))
	}
}

// GetProviderAPIKey returns the API key for a provider, checking the keychain
// first and the config value second.
func GetProviderAPIKey(providerName, configFallback string) string {
	return GetOrFallback(providerKeychainKey(providerName), configFallback)
}

// providerKeychainKey returns the keychain account name for a provider.
func providerKeychainKey(providerName string) string {
	return "provider_apikey_" + providerName
}
