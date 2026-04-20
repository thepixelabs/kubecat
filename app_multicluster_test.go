// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

// TestStringToLower pins the custom ASCII-only lowercase helper. It does NOT
// lowercase non-ASCII characters (unlike strings.ToLower) — that is by design
// because these searches are against k8s-conformant resource names.
func TestStringToLower(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"abc", "abc"},
		{"ABC", "abc"},
		{"aBcDeF", "abcdef"},
		{"123-XYZ", "123-xyz"},
		// Non-ASCII pass through unchanged — pinning existing behavior.
		{"café", "café"},
	}
	for _, tt := range tests {
		if got := stringToLower(tt.in); got != tt.want {
			t.Errorf("stringToLower(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStringContains(t *testing.T) {
	tests := []struct {
		s, sub string
		want   bool
	}{
		{"", "", true},
		{"abc", "", true},
		{"abc", "b", true},
		{"abc", "abc", true},
		{"abc", "d", false},
		{"short", "longer", false},
	}
	for _, tt := range tests {
		if got := stringContains(tt.s, tt.sub); got != tt.want {
			t.Errorf("stringContains(%q, %q) = %v, want %v", tt.s, tt.sub, got, tt.want)
		}
	}
}

func TestStringContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s, sub string
		want   bool
	}{
		{"my-pod-xyz", "POD", true},
		{"Production-Service", "prod", true},
		{"short", "longer", false},
		{"abc", "", true},
	}
	for _, tt := range tests {
		if got := stringContainsIgnoreCase(tt.s, tt.sub); got != tt.want {
			t.Errorf("stringContainsIgnoreCase(%q, %q) = %v, want %v", tt.s, tt.sub, got, tt.want)
		}
	}
}
