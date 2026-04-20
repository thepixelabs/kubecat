// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// containsShellMeta
// ---------------------------------------------------------------------------

func TestContainsShellMeta_ClearCommands(t *testing.T) {
	clear := []string{
		"kubectl get pods",
		"helm list -n default",
		"flux get sources git",
		"argocd app list",
		"kubectl -n kube-system get configmap",
		"", // empty has no meta
	}
	for _, c := range clear {
		t.Run(c, func(t *testing.T) {
			if containsShellMeta(c) {
				t.Errorf("containsShellMeta(%q) = true, want false", c)
			}
		})
	}
}

func TestContainsShellMeta_DetectsDangerous(t *testing.T) {
	dangerous := []string{
		"kubectl get pods; rm -rf /",
		"kubectl get pods && rm -rf /",
		"kubectl get pods || sh",
		"kubectl get pods | grep foo",
		"kubectl $(id) pods",
		"kubectl `id` pods",
		"kubectl > /tmp/out",
		"kubectl < /tmp/in",
		"kubectl\nrm",
		"kubectl\tfoo",
		"kubectl ~/foo",
		"kubectl foo*",
		"kubectl foo?",
		"kubectl (foo)",
		"kubectl {foo}",
		"kubectl #comment",
		"kubectl !sudo",
	}
	for _, c := range dangerous {
		t.Run(c, func(t *testing.T) {
			if !containsShellMeta(c) {
				t.Errorf("containsShellMeta(%q) = false, want true", c)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// allowedCommands allowlist shape
// ---------------------------------------------------------------------------

func TestAllowedCommands_ContainsExpectedBinaries(t *testing.T) {
	for _, bin := range []string{"kubectl", "helm", "flux", "argocd"} {
		if !allowedCommands[bin] {
			t.Errorf("allowedCommands missing %q", bin)
		}
	}
	// Shell interpreters MUST NOT be allowlisted.
	for _, forbid := range []string{"bash", "sh", "zsh", "python", "node", "perl", "ruby"} {
		if allowedCommands[forbid] {
			t.Errorf("allowedCommands contains forbidden binary %q", forbid)
		}
	}
}

// ---------------------------------------------------------------------------
// ExecuteCommand — denied paths (do NOT execute real commands)
// ---------------------------------------------------------------------------

func TestExecuteCommand_EmptyCommand(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	_, err := a.ExecuteCommand("")
	if err == nil {
		t.Fatal("empty command should be rejected")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty, got: %v", err)
	}
}

func TestExecuteCommand_WhitespaceOnly(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	_, err := a.ExecuteCommand("   \t  ")
	if err == nil {
		t.Fatal("whitespace-only command should be rejected")
	}
}

func TestExecuteCommand_ShellMetaRejected(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	cases := []string{
		"kubectl get pods; rm -rf /",
		"kubectl get pods && bash",
		"kubectl $(whoami)",
		"kubectl `id`",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			_, err := a.ExecuteCommand(c)
			if err == nil {
				t.Errorf("shell metachar command %q should be rejected", c)
			}
			if err != nil && !strings.Contains(err.Error(), "metacharacter") {
				t.Errorf("error should mention metacharacter: %v", err)
			}
		})
	}
}

func TestExecuteCommand_NonAllowlistedBinary(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)

	for _, cmd := range []string{"bash -c id", "python3 -c print", "rm -rf /tmp/foo"} {
		t.Run(cmd, func(t *testing.T) {
			_, err := a.ExecuteCommand(cmd)
			if err == nil {
				t.Errorf("non-allowlisted command %q should be rejected", cmd)
			}
			if err != nil && !strings.Contains(err.Error(), "allowed") {
				t.Errorf("error should mention allowlist: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AI query rejection when provider not configured
// ---------------------------------------------------------------------------

func TestAIQuery_AIDisabled_RejectsWithClearMessage(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	// Default config has AI.Enabled=false.
	_, err := a.AIQuery("anything")
	if err == nil {
		t.Fatal("AIQuery should reject when AI is disabled")
	}
	// Frontend depends on this exact substring.
	if !strings.Contains(err.Error(), "AI features are not enabled") {
		t.Errorf("error message changed — frontend may break: %v", err)
	}
}

func TestAIQuery_NoSelectedProvider(t *testing.T) {
	dir := isolateConfigDir(t)
	_ = dir
	a := newTestApp(t)
	// Enable AI but do not set a provider.
	if err := a.SaveAISettings(AISettings{
		Enabled:          true,
		SelectedProvider: "",
		Providers:        map[string]ProviderConfig{},
	}); err != nil {
		t.Fatalf("SaveAISettings: %v", err)
	}
	_, err := a.AIQuery("anything")
	if err == nil {
		t.Fatal("AIQuery should reject when no provider selected")
	}
	if !strings.Contains(err.Error(), "No AI provider selected") {
		t.Errorf("error message changed — frontend may break: %v", err)
	}
}

func TestAIQuery_ProviderNotEnabled(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	if err := a.SaveAISettings(AISettings{
		Enabled:          true,
		SelectedProvider: "anthropic",
		Providers: map[string]ProviderConfig{
			"anthropic": {Enabled: false, APIKey: "k", Endpoint: "https://api.anthropic.com/v1"},
		},
	}); err != nil {
		t.Fatalf("SaveAISettings: %v", err)
	}

	_, err := a.AIQuery("anything")
	if err == nil {
		t.Fatal("AIQuery should reject when selected provider not enabled")
	}
	if !strings.Contains(err.Error(), "not enabled or configured") {
		t.Errorf("error message changed — frontend may break: %v", err)
	}
}

func TestAIQueryWithContext_ProvidedProviderOverridesConfig(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	// AI enabled but no provider configured.
	if err := a.SaveAISettings(AISettings{
		Enabled:          true,
		SelectedProvider: "",
		Providers:        map[string]ProviderConfig{},
	}); err != nil {
		t.Fatalf("SaveAISettings: %v", err)
	}

	// Passing an unconfigured provider ID → should still fall into "not configured" branch
	// (i.e. provider lookup fails).
	_, err := a.AIQueryWithContext("q", nil, "anthropic", "", nil)
	if err == nil {
		t.Fatal("expected error when provider is not in config")
	}
	if !strings.Contains(err.Error(), "not enabled or configured") {
		t.Errorf("error message changed: %v", err)
	}
}

func TestAIQueryWithContext_AIDisabled(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	_, err := a.AIQueryWithContext("q", nil, "anthropic", "claude-3", nil)
	if err == nil {
		t.Fatal("expected error when AI is globally disabled")
	}
	if !strings.Contains(err.Error(), "AI features are not enabled") {
		t.Errorf("error message changed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// AIQueryStream error paths
// ---------------------------------------------------------------------------

func TestAIQueryStream_AIDisabled_SynchronousError(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	err := a.AIQueryStream("q", "conv-1", "", "")
	if err == nil {
		t.Fatal("AIQueryStream should reject when AI is disabled")
	}
	if !strings.Contains(err.Error(), "AI features are not enabled") {
		t.Errorf("error message changed: %v", err)
	}
}

func TestAIQueryStream_NoProviderSelected(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	if err := a.SaveAISettings(AISettings{Enabled: true}); err != nil {
		t.Fatalf("SaveAISettings: %v", err)
	}
	err := a.AIQueryStream("q", "conv-1", "", "")
	if err == nil {
		t.Fatal("AIQueryStream should reject when no provider selected")
	}
}

func TestAIQueryStream_ProviderNotConfigured(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	if err := a.SaveAISettings(AISettings{Enabled: true}); err != nil {
		t.Fatalf("SaveAISettings: %v", err)
	}
	err := a.AIQueryStream("q", "conv-1", "ghost-provider", "")
	if err == nil {
		t.Fatal("AIQueryStream should reject unknown provider")
	}
}

// ---------------------------------------------------------------------------
// AIAgentQuery error paths
// ---------------------------------------------------------------------------

func TestAIAgentQuery_AIDisabled(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	err := a.AIAgentQuery("q", "sess-1", "default", "", "")
	if err == nil {
		t.Fatal("AIAgentQuery should reject when AI is disabled")
	}
}

func TestAIAgentQuery_NoProvider(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	if err := a.SaveAISettings(AISettings{Enabled: true}); err != nil {
		t.Fatalf("SaveAISettings: %v", err)
	}
	err := a.AIAgentQuery("q", "sess-1", "default", "", "")
	if err == nil {
		t.Fatal("AIAgentQuery should reject when no provider selected")
	}
}

// ---------------------------------------------------------------------------
// Session approval / stop on unknown sessions
// ---------------------------------------------------------------------------

func TestApproveAgentAction_UnknownSession(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	a.agentSessions = map[string]*agentSession{} // ensure non-nil empty map
	if err := a.ApproveAgentAction("nonexistent", "tc1"); err == nil {
		t.Error("ApproveAgentAction should fail for unknown session")
	}
}

func TestRejectAgentAction_UnknownSession(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	a.agentSessions = map[string]*agentSession{}
	if err := a.RejectAgentAction("nonexistent", "tc1"); err == nil {
		t.Error("RejectAgentAction should fail for unknown session")
	}
}

func TestStopAgentSession_UnknownSession(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	a.agentSessions = map[string]*agentSession{}
	if err := a.StopAgentSession("nonexistent"); err == nil {
		t.Error("StopAgentSession should fail for unknown session")
	}
}
