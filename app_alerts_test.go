// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"
)

// TestGetAlertSettings_AppliesDefaultsForZeroFields pins the contract that
// GetAlertSettings fills in sensible defaults when the config has zero-valued
// scan interval or cooldown. The 60s/30m defaults are surfaced in the UI and
// changing them silently would be a visible regression.
func TestGetAlertSettings_AppliesDefaultsForZeroFields(t *testing.T) {
	// Use a temp config dir so no real config bleeds in.
	t.Setenv("KUBECAT_CONFIG_DIR", t.TempDir())

	a := &App{}
	got, err := a.GetAlertSettings()
	if err != nil {
		t.Fatalf("GetAlertSettings: %v", err)
	}
	if got.ScanIntervalSeconds != 60 {
		t.Errorf("default ScanIntervalSeconds = %d, want 60", got.ScanIntervalSeconds)
	}
	if got.CooldownMinutes != 30 {
		t.Errorf("default CooldownMinutes = %d, want 30", got.CooldownMinutes)
	}
}

func TestSaveAndGetAlertSettings_RoundTrip(t *testing.T) {
	t.Setenv("KUBECAT_CONFIG_DIR", t.TempDir())

	a := &App{}
	in := AlertSettings{
		Enabled:             true,
		ScanIntervalSeconds: 120,
		CooldownMinutes:     45,
		IgnoredNamespaces:   []string{"kube-system", "istio-system"},
	}
	if err := a.SaveAlertSettings(in); err != nil {
		t.Fatalf("SaveAlertSettings: %v", err)
	}

	got, err := a.GetAlertSettings()
	if err != nil {
		t.Fatalf("GetAlertSettings: %v", err)
	}
	if !got.Enabled {
		t.Error("Enabled not round-tripped")
	}
	if got.ScanIntervalSeconds != in.ScanIntervalSeconds {
		t.Errorf("ScanIntervalSeconds = %d, want %d", got.ScanIntervalSeconds, in.ScanIntervalSeconds)
	}
	if got.CooldownMinutes != in.CooldownMinutes {
		t.Errorf("CooldownMinutes = %d, want %d", got.CooldownMinutes, in.CooldownMinutes)
	}
	if len(got.IgnoredNamespaces) != len(in.IgnoredNamespaces) {
		t.Errorf("IgnoredNamespaces len = %d, want %d", len(got.IgnoredNamespaces), len(in.IgnoredNamespaces))
	}
}
