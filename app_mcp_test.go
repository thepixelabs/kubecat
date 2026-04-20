// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"
)

// Unit tests for the thin MCP wrapper in app_mcp.go.  These exercise
// start/stop idempotency and the status reporter without spinning up an
// actual stdio server (the MCP server itself is exercised by internal/mcp).

func resetMCPGlobals() {
	// Package-level singletons are defined in app_mcp.go.  Tests that start
	// the server must reset them so subsequent tests see a clean slate.
	mcpMu.Lock()
	defer mcpMu.Unlock()
	if mcpCancel != nil {
		mcpCancel()
		mcpCancel = nil
	}
	mcpServer = nil
}

func TestGetMCPStatus_DefaultsToNotRunning(t *testing.T) {
	resetMCPGlobals()
	t.Cleanup(resetMCPGlobals)

	a := newTestApp(t)
	status := a.GetMCPStatus()

	if running, _ := status["running"].(bool); running {
		t.Error("expected running=false when server was never started")
	}
	if transport, _ := status["transport"].(string); transport != "stdio" {
		t.Errorf("transport = %q, want stdio", transport)
	}
}

func TestStartMCPServer_IsIdempotent(t *testing.T) {
	resetMCPGlobals()
	t.Cleanup(resetMCPGlobals)

	a := newTestApp(t)

	if err := a.StartMCPServer(); err != nil {
		t.Fatalf("first StartMCPServer: %v", err)
	}
	// Second call must be a no-op (already running).
	if err := a.StartMCPServer(); err != nil {
		t.Errorf("second StartMCPServer: %v, want nil (idempotent)", err)
	}

	status := a.GetMCPStatus()
	if running, _ := status["running"].(bool); !running {
		t.Error("status.running should be true after StartMCPServer")
	}
}

func TestStopMCPServer_BeforeStart_IsNoOp(t *testing.T) {
	resetMCPGlobals()
	t.Cleanup(resetMCPGlobals)

	a := newTestApp(t)
	// Must not panic when the server was never started.
	a.StopMCPServer()
}

func TestStartStopMCP_ClearsRunningFlag(t *testing.T) {
	resetMCPGlobals()
	t.Cleanup(resetMCPGlobals)

	a := newTestApp(t)
	if err := a.StartMCPServer(); err != nil {
		t.Fatalf("StartMCPServer: %v", err)
	}
	a.StopMCPServer()

	status := a.GetMCPStatus()
	if running, _ := status["running"].(bool); running {
		t.Error("running should be false after StopMCPServer")
	}
}
