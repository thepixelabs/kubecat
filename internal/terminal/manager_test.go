package terminal

import (
	"strings"
	"testing"
)

// TestValidateCommand_Allowlist verifies that only the approved shell names are
// accepted and that everything else is rejected, including absolute paths that
// resolve to approved binaries and bare names of unapproved binaries.
func TestValidateCommand_Allowlist(t *testing.T) {
	t.Parallel()

	allowedCases := []string{"bash", "zsh", "sh"}
	for _, cmd := range allowedCases {
		cmd := cmd
		t.Run("allowed_"+cmd, func(t *testing.T) {
			t.Parallel()
			resolved, err := validateCommand(cmd)
			if err != nil {
				// The shell might simply not be installed on the test host.
				// That is acceptable — the function correctly attempted the
				// lookup. What is NOT acceptable is a security bypass.
				if strings.Contains(err.Error(), "not found on this system") {
					t.Skipf("shell %q not installed, skipping: %v", cmd, err)
				}
				t.Errorf("validateCommand(%q) returned unexpected error: %v", cmd, err)
			}
			if resolved == "" {
				t.Errorf("validateCommand(%q) returned empty resolved path", cmd)
			}
		})
	}
}

// TestValidateCommand_Rejected verifies that disallowed commands are rejected
// with a clear error before any PTY is created.
func TestValidateCommand_Rejected(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		command string
		wantMsg string
	}{
		{
			name:    "arbitrary binary",
			command: "nc",
			wantMsg: "not in the approved shell allowlist",
		},
		{
			name:    "absolute path to allowed shell",
			command: "/bin/bash",
			wantMsg: "command must be a bare name, not a path",
		},
		{
			name:    "absolute path to arbitrary binary",
			command: "/usr/bin/nc",
			wantMsg: "command must be a bare name, not a path",
		},
		{
			name:    "path traversal attempt",
			command: "../../bin/bash",
			wantMsg: "command must be a bare name, not a path",
		},
		{
			name:    "empty string",
			command: "",
			wantMsg: "not in the approved shell allowlist",
		},
		{
			name:    "python",
			command: "python3",
			wantMsg: "not in the approved shell allowlist",
		},
		{
			name:    "curl",
			command: "curl",
			wantMsg: "not in the approved shell allowlist",
		},
		{
			name:    "windows-style backslash path",
			command: "C:\\Windows\\System32\\cmd.exe",
			wantMsg: "command must be a bare name, not a path",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := validateCommand(tc.command)
			if err == nil {
				t.Fatalf("validateCommand(%q) expected error containing %q, got nil", tc.command, tc.wantMsg)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("validateCommand(%q) error = %q, want substring %q", tc.command, err.Error(), tc.wantMsg)
			}
		})
	}
}

// TestValidateArgs_Clean verifies that benign argument lists are accepted.
func TestValidateArgs_Clean(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{name: "no args", args: []string{}},
		{name: "login flag", args: []string{"-l"}},
		{name: "interactive flag", args: []string{"-i"}},
		{name: "norc flag", args: []string{"--norc"}},
		{name: "multiple clean flags", args: []string{"-l", "-i", "--norc"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := validateArgs(tc.args); err != nil {
				t.Errorf("validateArgs(%v) unexpected error: %v", tc.args, err)
			}
		})
	}
}

// TestValidateArgs_Metacharacters verifies that every shell metacharacter is
// caught in args, regardless of which position in the arg slice it appears.
func TestValidateArgs_Metacharacters(t *testing.T) {
	t.Parallel()

	// Each entry is a single character (or short sequence) that must be
	// rejected when present anywhere in an arg value.
	badArgs := []struct {
		name string
		arg  string
	}{
		{name: "semicolon", arg: "-c;id"},
		{name: "ampersand_pipe", arg: "&&cat /etc/passwd"},
		{name: "pipe", arg: "foo|bar"},
		{name: "backtick", arg: "`id`"},
		{name: "dollar_paren", arg: "$(id)"},
		{name: "open_paren", arg: "foo(bar"},
		{name: "close_paren", arg: "foo)bar"},
		{name: "less_than", arg: "</etc/passwd"},
		{name: "greater_than", arg: ">/tmp/x"},
		{name: "backslash", arg: "foo\\bar"},
		{name: "exclamation", arg: "!foo"},
		{name: "open_brace", arg: "{foo"},
		{name: "close_brace", arg: "foo}"},
	}

	for _, tc := range badArgs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateArgs([]string{tc.arg})
			if err == nil {
				t.Errorf("validateArgs([%q]) expected error for metacharacter, got nil", tc.arg)
			}
		})
	}

	// Also test that a bad arg in the second position (not index 0) is still
	// caught.
	t.Run("bad_arg_in_second_position", func(t *testing.T) {
		t.Parallel()
		err := validateArgs([]string{"-l", "$(evil)"})
		if err == nil {
			t.Error("validateArgs with metacharacter in second arg should return error, got nil")
		}
		if !strings.Contains(err.Error(), "arg[1]") {
			t.Errorf("error should reference arg index 1, got: %v", err)
		}
	})
}
