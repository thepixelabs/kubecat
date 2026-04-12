// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ErrReadOnly sentinel
// ---------------------------------------------------------------------------

func TestErrReadOnly_IsDistinctError(t *testing.T) {
	if ErrReadOnly == nil {
		t.Fatal("ErrReadOnly must not be nil")
	}
	if ErrReadOnly.Error() == "" {
		t.Error("ErrReadOnly must have a non-empty message")
	}
}

func TestGuardReadOnly_ReadOnlyTrue_ReturnsErrReadOnly(t *testing.T) {
	err := guardReadOnly(true)
	if !errors.Is(err, ErrReadOnly) {
		t.Errorf("guardReadOnly(true) = %v, want ErrReadOnly", err)
	}
}

func TestGuardReadOnly_ReadOnlyFalse_ReturnsNil(t *testing.T) {
	if err := guardReadOnly(false); err != nil {
		t.Errorf("guardReadOnly(false) = %v, want nil", err)
	}
}

func TestGuardReadOnly_ErrorWrapsErrReadOnly(t *testing.T) {
	err := guardReadOnly(true)
	if !errors.Is(err, ErrReadOnly) {
		t.Error("guardReadOnly(true) result must satisfy errors.Is(err, ErrReadOnly)")
	}
}

// ---------------------------------------------------------------------------
// ValidateResourceName
// ---------------------------------------------------------------------------

func TestValidateResourceName_Valid(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"simple lowercase", "my-pod"},
		{"alphanumeric only", "pod1"},
		{"single char", "a"},
		{"with dot", "my.pod"},
		{"max length exactly", strings.Repeat("a", 253)},
		{"kubernetes style", "nginx-deployment-5dc4c4d8b5"},
		{"starts and ends alphanumeric", "a-b-c"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateResourceName(tc.input); err != nil {
				t.Errorf("ValidateResourceName(%q) unexpected error: %v", tc.input, err)
			}
		})
	}
}

func TestValidateResourceName_Invalid(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantMsg string
	}{
		{"empty string", "", "must not be empty"},
		{"uppercase letters", "MyPod", "invalid"},
		{"starts with hyphen", "-pod", "invalid"},
		{"ends with hyphen", "pod-", "invalid"},
		{"space inside", "my pod", "invalid"},
		{"underscore", "my_pod", "invalid"},
		{"too long", strings.Repeat("a", 254), "exceeds maximum"},
		{"slash injection", "ns/pod", "invalid"},
		{"dot start", ".pod", "invalid"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateResourceName(tc.input)
			if err == nil {
				t.Fatalf("ValidateResourceName(%q) expected error, got nil", tc.input)
			}
			if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("ValidateResourceName(%q) error = %q, want substring %q", tc.input, err.Error(), tc.wantMsg)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ValidateNamespace
// ---------------------------------------------------------------------------

func TestValidateNamespace_Valid(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"simple", "default"},
		{"with hyphen", "my-namespace"},
		{"single char", "a"},
		{"max length exactly", strings.Repeat("a", 63)},
		{"kube-system", "kube-system"},
		{"numeric suffix", "app-123"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateNamespace(tc.input); err != nil {
				t.Errorf("ValidateNamespace(%q) unexpected error: %v", tc.input, err)
			}
		})
	}
}

func TestValidateNamespace_Invalid(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantMsg string
	}{
		{"empty", "", "must not be empty"},
		{"uppercase", "MyNamespace", "invalid"},
		{"starts with hyphen", "-ns", "invalid"},
		{"ends with hyphen", "ns-", "invalid"},
		{"dot not allowed", "my.ns", "invalid"},
		{"underscore not allowed", "my_ns", "invalid"},
		{"too long", strings.Repeat("a", 64), "exceeds maximum"},
		{"space inside", "my ns", "invalid"},
		{"slash injection", "myns/etc", "invalid"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateNamespace(tc.input)
			if err == nil {
				t.Fatalf("ValidateNamespace(%q) expected error, got nil", tc.input)
			}
			if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("ValidateNamespace(%q) error = %q, want substring %q", tc.input, err.Error(), tc.wantMsg)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isSystemNamespace
// ---------------------------------------------------------------------------

func TestIsSystemNamespace(t *testing.T) {
	cases := []struct {
		ns   string
		want bool
	}{
		{"kube-system", true},
		{"kube-public", true},
		{"kube-node-lease", true},
		{"default", false},
		{"my-app", false},
		{"KUBE-SYSTEM", true}, // case-insensitive
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.ns, func(t *testing.T) {
			got := isSystemNamespace(tc.ns)
			if got != tc.want {
				t.Errorf("isSystemNamespace(%q) = %v, want %v", tc.ns, got, tc.want)
			}
		})
	}
}
