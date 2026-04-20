// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"strings"
	"testing"

	"github.com/thepixelabs/kubecat/internal/terminal"
)

// newAppWithTerminal constructs a minimal *App wired to a real
// terminal.Manager so the shell allowlist / read-only path can be exercised.
// The terminal manager's ctx is deliberately set to a background context so
// the PTY reader goroutine does not explode on Wails EventsEmit.
func newAppWithTerminal(t *testing.T) *App {
	t.Helper()
	a := newAppWithFakes(nil)
	a.terminalManager = terminal.NewManager()
	a.terminalManager.SetContext(context.Background())
	return a
}

// TestStartTerminal_ReadOnlyBlocks verifies the safety net: if the app is
// configured in read-only mode, StartTerminal refuses before touching the
// terminal manager. This protects against a kubeconfig-exec side-effect
// path that might spawn a shell.
func TestStartTerminal_ReadOnlyBlocks(t *testing.T) {
	withReadOnlyConfig(t, true)
	a := newAppWithTerminal(t)

	err := a.StartTerminal("s1", "bash", nil)
	if err == nil {
		t.Fatal("expected read-only mode to block StartTerminal, got nil")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("error should mention read-only, got: %v", err)
	}
}

// TestStartTerminal_RejectsDisallowedShell pins the allowlist enforcement at
// the *App* layer: anything outside {bash,zsh,sh} must be refused before a
// PTY is allocated.
func TestStartTerminal_RejectsDisallowedShell(t *testing.T) {
	isolateConfigDir(t)
	a := newAppWithTerminal(t)

	err := a.StartTerminal("s1", "python3", nil)
	if err == nil {
		t.Fatal("expected shell allowlist to reject python3, got nil")
	}
	if !strings.Contains(err.Error(), "allowlist") {
		t.Errorf("error should mention allowlist, got: %v", err)
	}
}

// TestStartTerminal_RejectsPathForm ensures an attacker cannot bypass the
// allowlist by supplying an absolute path whose basename is "bash".
func TestStartTerminal_RejectsPathForm(t *testing.T) {
	isolateConfigDir(t)
	a := newAppWithTerminal(t)

	err := a.StartTerminal("s1", "/bin/bash", nil)
	if err == nil {
		t.Fatal("expected path-form command to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "bare name") {
		t.Errorf("error should mention bare-name requirement, got: %v", err)
	}
}

// TestStartTerminal_RejectsArgMetacharacters confirms args containing shell
// metacharacters are refused even when the command itself is approved.
func TestStartTerminal_RejectsArgMetacharacters(t *testing.T) {
	isolateConfigDir(t)
	a := newAppWithTerminal(t)

	err := a.StartTerminal("s1", "bash", []string{"-c", "$(id)"})
	if err == nil {
		t.Fatal("expected metacharacter args to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "metacharacter") {
		t.Errorf("error should mention metacharacter, got: %v", err)
	}
}

// Note: full PTY-lifecycle tests (Start -> Write -> Close -> exit) live in
// internal/terminal and cannot run at the *App* layer without a real Wails
// context because Manager.Start's reader goroutine unconditionally calls
// runtime.EventsEmit on exit. See the "testability gap" comment in the
// report. We keep coverage here focused on the validation paths that run
// before any PTY is allocated.

// TestCloseTerminal_UnknownID_NoError mirrors the internal manager's contract:
// closing a never-started session is a silent no-op rather than an error.
func TestCloseTerminal_UnknownID_NoError(t *testing.T) {
	a := newAppWithTerminal(t)
	if err := a.CloseTerminal("never-started"); err != nil {
		t.Errorf("CloseTerminal for unknown id should not error, got: %v", err)
	}
}

// TestResizeTerminal_UnknownID_Errors verifies resize for a missing session
// returns a clean error.
func TestResizeTerminal_UnknownID_Errors(t *testing.T) {
	a := newAppWithTerminal(t)
	err := a.ResizeTerminal("ghost", 24, 80)
	if err == nil {
		t.Fatal("expected error on resize of missing session, got nil")
	}
}

// TestWriteTerminal_UnknownID_Errors verifies write for a missing session
// returns a clean error.
func TestWriteTerminal_UnknownID_Errors(t *testing.T) {
	a := newAppWithTerminal(t)
	err := a.WriteTerminal("ghost", "data")
	if err == nil {
		t.Fatal("expected error on write to missing session, got nil")
	}
}
