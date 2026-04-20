// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

// fluxKustomizationObj builds a Flux v1 Kustomization CRD object with
// the provided Ready-condition status ("True" or "False").
func fluxKustomizationObj(name, namespace, readyStatus, message string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"labels":    map[string]interface{}{"team": "platform"},
		},
		"spec": map[string]interface{}{
			"path": "./base",
			"sourceRef": map[string]interface{}{
				"kind": "GitRepository",
				"name": "infra",
			},
		},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{
					"type":    "Ready",
					"status":  readyStatus,
					"message": message,
				},
			},
			"lastAppliedRevision":    "abc123",
			"lastHandledReconcileAt": "2024-01-01T00:00:00Z",
		},
	}
}

// fluxHelmReleaseObj builds a Flux v2 HelmRelease object.
func fluxHelmReleaseObj(name, namespace, chart, version, readyStatus string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "helm.toolkit.fluxcd.io/v2",
		"kind":       "HelmRelease",
		"metadata":   map[string]interface{}{"name": name, "namespace": namespace},
		"spec": map[string]interface{}{
			"chart": map[string]interface{}{
				"spec": map[string]interface{}{
					"chart":   chart,
					"version": version,
					"sourceRef": map[string]interface{}{
						"kind": "HelmRepository",
						"name": "repo",
					},
				},
			},
		},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": readyStatus},
			},
			"lastAppliedRevision": "1.2.3",
		},
	}
}

// argoApplicationObj builds an ArgoCD Application object.
func argoApplicationObj(name, namespace, syncStatus, healthStatus, message string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]interface{}{"name": name, "namespace": namespace},
		"spec": map[string]interface{}{
			"source": map[string]interface{}{
				"repoURL":        "https://github.com/acme/infra",
				"path":           "apps/foo",
				"targetRevision": "main",
			},
		},
		"status": map[string]interface{}{
			"sync": map[string]interface{}{
				"status":   syncStatus,
				"revision": "deadbeef",
			},
			"health": map[string]interface{}{
				"status":  healthStatus,
				"message": message,
			},
			"reconciledAt": "2024-01-02T00:00:00Z",
		},
	}
}

// TestGetGitOpsStatus_DetectsFluxKustomizations pins Flux detection via
// Kustomization CRD + correct sync status mapping.
func TestGetGitOpsStatus_DetectsFluxKustomizations(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("kustomizations", fluxKustomizationObj("app-a", "flux-system", "True", ""))
	cl.addResource("kustomizations", fluxKustomizationObj("app-b", "flux-system", "False", "timeout"))

	a := newAppWithFakes(cl)
	status, err := a.GetGitOpsStatus()
	if err != nil {
		t.Fatalf("GetGitOpsStatus: %v", err)
	}

	if status.Provider != "flux" || !status.Detected {
		t.Errorf("provider=%q detected=%v, want flux/true", status.Provider, status.Detected)
	}
	if status.Summary.Total != 2 {
		t.Errorf("Total = %d, want 2", status.Summary.Total)
	}
	if status.Summary.Synced != 1 {
		t.Errorf("Synced = %d, want 1", status.Summary.Synced)
	}
	if status.Summary.OutOfSync != 1 {
		t.Errorf("OutOfSync = %d, want 1", status.Summary.OutOfSync)
	}
	// The degraded one carries the Ready condition message.
	var foundMsg bool
	for _, app := range status.Applications {
		if app.Name == "app-b" && app.Message == "timeout" {
			foundMsg = true
		}
	}
	if !foundMsg {
		t.Error("expected degraded app-b to carry Ready condition message")
	}
}

// TestGetGitOpsStatus_DetectsFluxHelmReleases pins HelmRelease detection.
func TestGetGitOpsStatus_DetectsFluxHelmReleases(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("helmreleases", fluxHelmReleaseObj("prometheus", "monitoring", "prometheus", "25.8.0", "True"))

	a := newAppWithFakes(cl)
	status, err := a.GetGitOpsStatus()
	if err != nil {
		t.Fatalf("GetGitOpsStatus: %v", err)
	}
	if status.Provider != "flux" {
		t.Errorf("provider = %q, want flux", status.Provider)
	}
	if len(status.Applications) != 1 {
		t.Fatalf("expected 1 application, got %d", len(status.Applications))
	}
	app := status.Applications[0]
	if app.Kind != "HelmRelease" {
		t.Errorf("kind = %q, want HelmRelease", app.Kind)
	}
	if app.Source.Chart != "prometheus" || app.Source.Version != "25.8.0" {
		t.Errorf("source = %+v, want chart=prometheus version=25.8.0", app.Source)
	}
}

// TestGetGitOpsStatus_DetectsArgoCDWhenNoFlux verifies that ArgoCD detection
// is the fallback when no Flux CRDs are present.
func TestGetGitOpsStatus_DetectsArgoCDWhenNoFlux(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("applications", argoApplicationObj("app-1", "argocd", "Synced", "Healthy", ""))
	cl.addResource("applications", argoApplicationObj("app-2", "argocd", "OutOfSync", "Degraded", "diff"))

	a := newAppWithFakes(cl)
	status, err := a.GetGitOpsStatus()
	if err != nil {
		t.Fatalf("GetGitOpsStatus: %v", err)
	}
	if status.Provider != "argocd" {
		t.Errorf("provider = %q, want argocd", status.Provider)
	}
	if status.Summary.Synced != 1 || status.Summary.OutOfSync != 1 {
		t.Errorf("summary = %+v, want Synced=1 OutOfSync=1", status.Summary)
	}
	if status.Summary.Healthy != 1 || status.Summary.Degraded != 1 {
		t.Errorf("summary = %+v, want Healthy=1 Degraded=1", status.Summary)
	}
}

// TestGetGitOpsStatus_EmptyCluster_ReturnsNone verifies the "no GitOps"
// branch — returns a valid struct with Detected=false.
func TestGetGitOpsStatus_EmptyCluster_ReturnsNone(t *testing.T) {
	cl := newFakeClusterClient()
	a := newAppWithFakes(cl)
	status, err := a.GetGitOpsStatus()
	if err != nil {
		t.Fatalf("GetGitOpsStatus: %v", err)
	}
	if status.Detected {
		t.Error("Detected should be false on empty cluster")
	}
	if status.Provider != "none" {
		t.Errorf("provider = %q, want none", status.Provider)
	}
	if len(status.Applications) != 0 {
		t.Errorf("Applications should be empty, got %d", len(status.Applications))
	}
}

// TestGetGitOpsApplication_ByKind_Kustomization retrieves a single
// Kustomization and asserts parsed fields.
func TestGetGitOpsApplication_ByKind_Kustomization(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("kustomizations", fluxKustomizationObj("app", "flux-system", "True", ""))
	a := newAppWithFakes(cl)

	got, err := a.GetGitOpsApplication("flux-system", "app", "Kustomization")
	if err != nil {
		t.Fatalf("GetGitOpsApplication: %v", err)
	}
	if got.SyncStatus != "Synced" || got.HealthStatus != "Healthy" {
		t.Errorf("sync/health = %s/%s, want Synced/Healthy", got.SyncStatus, got.HealthStatus)
	}
	if got.Revision != "abc123" {
		t.Errorf("revision = %q, want abc123", got.Revision)
	}
}

// TestGetGitOpsApplication_UnknownKind rejects unknown kinds.
func TestGetGitOpsApplication_UnknownKind(t *testing.T) {
	cl := newFakeClusterClient()
	a := newAppWithFakes(cl)
	_, err := a.GetGitOpsApplication("ns", "name", "Pod")
	if err == nil {
		t.Error("expected error for unknown kind 'Pod'")
	}
}

// TestSyncGitOpsApplication_GuidanceOnly pins the behavior — SyncGitOps
// returns an instructional error, it does NOT actually mutate.
func TestSyncGitOpsApplication_GuidanceOnly(t *testing.T) {
	// With empty config (readOnly unset), checkReadOnly returns nil; the
	// method should return the kind-specific guidance error.
	a := newAppWithFakes(newFakeClusterClient())

	cases := []struct {
		kind    string
		wantSub string
	}{
		{"Kustomization", "flux reconcile"},
		{"HelmRelease", "flux reconcile"},
		{"Application", "argocd app sync"},
		{"Pod", "unknown GitOps kind"},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			err := a.SyncGitOpsApplication("ns", "name", tc.kind)
			if err == nil {
				t.Fatalf("SyncGitOpsApplication(%s) must return guidance error", tc.kind)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("err = %q, expected substring %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// TestParseArgoCDApplication_DefaultsUnknown guarantees ArgoCD status fields
// without an explicit sync/health fall back to "Unknown".
func TestParseArgoCDApplication_DefaultsUnknown(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("applications", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "bare", "namespace": "argocd"},
		"spec":     map[string]interface{}{"source": map[string]interface{}{}},
		"status":   map[string]interface{}{},
	})
	a := newAppWithFakes(cl)
	got, err := a.GetGitOpsApplication("argocd", "bare", "Application")
	if err != nil {
		t.Fatalf("GetGitOpsApplication: %v", err)
	}
	if got.SyncStatus != "Unknown" || got.HealthStatus != "Unknown" {
		t.Errorf("defaults: sync=%s health=%s want Unknown/Unknown", got.SyncStatus, got.HealthStatus)
	}
}

// TestParseKustomization_SuspendOverridesHealth pins that Spec.Suspend sets
// HealthStatus to "Suspended" regardless of Ready condition.
func TestParseKustomization_SuspendOverridesHealth(t *testing.T) {
	cl := newFakeClusterClient()
	obj := fluxKustomizationObj("sus", "flux-system", "True", "")
	obj["spec"].(map[string]interface{})["suspend"] = true
	cl.addResource("kustomizations", obj)

	a := newAppWithFakes(cl)
	got, err := a.GetGitOpsApplication("flux-system", "sus", "Kustomization")
	if err != nil {
		t.Fatalf("GetGitOpsApplication: %v", err)
	}
	if got.HealthStatus != "Suspended" {
		t.Errorf("health = %q, want Suspended", got.HealthStatus)
	}
}
