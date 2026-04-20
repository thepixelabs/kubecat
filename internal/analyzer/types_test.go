// SPDX-License-Identifier: Apache-2.0

package analyzer

import "testing"

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		name string
		in   Severity
		want string
	}{
		{"info", SeverityInfo, "Info"},
		{"warning", SeverityWarning, "Warning"},
		{"critical", SeverityCritical, "Critical"},
		{"unknown_fallback", Severity(99), "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.String(); got != tt.want {
				t.Errorf("Severity(%d).String() = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSeverity_Symbol(t *testing.T) {
	// Symbol must return a non-empty string for every defined level. The exact
	// character is rendered in the UI, so we pin concrete values.
	tests := []struct {
		name string
		in   Severity
		want string
	}{
		{"info", SeverityInfo, "ℹ"},
		{"warning", SeverityWarning, "⚠"},
		{"critical", SeverityCritical, "✖"},
		{"unknown_fallback", Severity(-1), "?"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.Symbol(); got != tt.want {
				t.Errorf("Severity(%d).Symbol() = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSeverity_Ordering_CriticalGreaterThanWarning(t *testing.T) {
	// Callers sort/filter issues by severity — the numeric ordering is
	// load-bearing and must remain Info < Warning < Critical.
	if !(SeverityInfo < SeverityWarning) {
		t.Errorf("SeverityInfo(%d) must be < SeverityWarning(%d)", SeverityInfo, SeverityWarning)
	}
	if !(SeverityWarning < SeverityCritical) {
		t.Errorf("SeverityWarning(%d) must be < SeverityCritical(%d)", SeverityWarning, SeverityCritical)
	}
}
