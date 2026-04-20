// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"strings"
	"testing"
)

// Tests for app_agent_executor.go focusing on input validation and dispatch
// branches that do not require a live cluster connection. Branches that
// require a real cluster client (agentGetResourceYAML, agentListResources,
// etc.) are intentionally left to integration coverage.

// ---------------------------------------------------------------------------
// ExecuteTool dispatch
// ---------------------------------------------------------------------------

func TestExecuteTool_UnknownTool(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	_, err := a.ExecuteTool(context.Background(), "mystery_tool", nil)
	if err == nil {
		t.Fatal("unknown tool should return error")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("error should mention unknown tool, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Required-param validation (these branches reject before any cluster call)
// ---------------------------------------------------------------------------

func TestAgentGetResourceYAML_MissingRequiredParams(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	cases := []map[string]string{
		{},              // no kind, no name
		{"kind": "Pod"}, // missing name
		{"name": "foo"}, // missing kind
	}
	for i, p := range cases {
		_, err := a.agentGetResourceYAML(context.Background(), p)
		if err == nil {
			t.Errorf("case %d: expected error for missing required params", i)
		}
	}
}

func TestAgentGetPodLogs_MissingRequiredParams(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	cases := []map[string]string{
		{},
		{"namespace": "default"}, // missing pod
		{"pod": "web"},           // missing namespace
	}
	for i, p := range cases {
		_, err := a.agentGetPodLogs(context.Background(), p)
		if err == nil {
			t.Errorf("case %d: expected error for missing required params", i)
		}
	}
}

func TestAgentDescribeResource_MissingRequiredParams(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	if _, err := a.agentDescribeResource(context.Background(), map[string]string{}); err == nil {
		t.Error("expected error when kind/name missing")
	}
}

func TestAgentListResources_MissingKind(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	if _, err := a.agentListResources(context.Background(), map[string]string{}); err == nil {
		t.Error("expected error when kind missing")
	}
}

func TestAgentScaleDeployment_ValidationErrors(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	// Missing required fields.
	if _, err := a.agentScaleDeployment(context.Background(), map[string]string{}); err == nil {
		t.Error("expected error when fields missing")
	}
	// Non-numeric replicas.
	p := map[string]string{"namespace": "default", "name": "web", "replicas": "abc"}
	if _, err := a.agentScaleDeployment(context.Background(), p); err == nil {
		t.Error("expected error for non-numeric replicas")
	}
	// Negative replicas.
	p = map[string]string{"namespace": "default", "name": "web", "replicas": "-1"}
	if _, err := a.agentScaleDeployment(context.Background(), p); err == nil {
		t.Error("expected error for negative replicas")
	}
}

func TestAgentRestartDeployment_MissingFields(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	if _, err := a.agentRestartDeployment(context.Background(), map[string]string{}); err == nil {
		t.Error("expected error when namespace/name missing")
	}
}

func TestAgentRestartDeployment_Success(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	// No cluster call inside restartDeployment — it just returns instructions.
	out, err := a.agentRestartDeployment(context.Background(), map[string]string{
		"namespace": "default",
		"name":      "web",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "kubectl rollout restart") {
		t.Errorf("output should contain kubectl rollout restart, got %q", out)
	}
	if !strings.Contains(out, "deployment/web") {
		t.Errorf("output should reference the deployment name, got %q", out)
	}
}

func TestAgentDeleteResource_MissingFields(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	if _, err := a.agentDeleteResource(context.Background(), map[string]string{}); err == nil {
		t.Error("expected error when kind/name missing")
	}
}

func TestAgentExecCommand_RequiresCommand(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	if _, err := a.agentExecCommand(context.Background(), map[string]string{}); err == nil {
		t.Error("expected error when command missing")
	}
}

func TestAgentExecCommand_DelegatesToExecuteCommand(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	// Empty command after trim — must hit the "empty" path of ExecuteCommand.
	_, err := a.agentExecCommand(context.Background(), map[string]string{"command": "   "})
	if err == nil {
		t.Error("expected error for empty command")
	}
	// Non-allowlisted binary routed through ExecuteCommand.
	_, err = a.agentExecCommand(context.Background(), map[string]string{"command": "rm -rf /"})
	if err == nil {
		t.Error("expected rejection by allowlist")
	}
}

func TestAgentApplyManifest_RequiresManifest(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	if _, err := a.agentApplyManifest(context.Background(), map[string]string{}); err == nil {
		t.Error("expected error when manifest missing")
	}
}

func TestAgentApplyManifest_ReturnsKubectlInvocation(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	manifest := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: foo"
	out, err := a.agentApplyManifest(context.Background(), map[string]string{"manifest": manifest})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "kubectl apply -f -") {
		t.Errorf("output should recommend kubectl apply, got %q", out)
	}
	if !strings.Contains(out, manifest) {
		t.Error("output should embed the user manifest so they can review it before running")
	}
}

func TestAgentPatchResource_MissingFields(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	if _, err := a.agentPatchResource(context.Background(), map[string]string{}); err == nil {
		t.Error("expected error when kind/name/patch missing")
	}
}

func TestAgentPatchResource_ReturnsKubectlPatch(t *testing.T) {
	isolateConfigDir(t)
	a := newTestApp(t)
	out, err := a.agentPatchResource(context.Background(), map[string]string{
		"kind":      "Deployment",
		"namespace": "default",
		"name":      "web",
		"patch":     `{"spec":{"replicas":2}}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "kubectl patch deployment web") {
		t.Errorf("output should contain kubectl patch invocation, got %q", out)
	}
}

// ---------------------------------------------------------------------------
// activeCluster — returns error when nexus has no manager
// ---------------------------------------------------------------------------

func TestActiveCluster_NoManager(t *testing.T) {
	a := newTestApp(t)
	// newTestApp uses NewClusterService which tries NewManager; in most test
	// environments this will fail (no kubeconfig), resulting in a nil manager.
	_, err := a.activeCluster()
	if err == nil {
		// Skip: CI machine might have a kubeconfig that produces a valid manager;
		// in that case this test is not meaningful. We still assert that the
		// function did not panic.
		t.Skip("active cluster is set up on this host; skipping assertion")
	}
	if !strings.Contains(err.Error(), "cluster") {
		t.Errorf("error should mention cluster, got: %v", err)
	}
}
