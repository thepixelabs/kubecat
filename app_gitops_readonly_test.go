// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

// TestSyncGitOpsApplication_ReadOnlyBlocks_BeforeGuidance pins that when the
// app is running in read-only mode, SyncGitOpsApplication returns the
// read-only error BEFORE the kind-specific guidance string — even for
// supported kinds. This guards the property that SyncGitOps never mutates
// cluster state even to produce a hint.
func TestSyncGitOpsApplication_ReadOnlyBlocks_BeforeGuidance(t *testing.T) {
	withReadOnlyConfig(t, true)
	a := newAppWithFakes(newFakeClusterClient())

	err := a.SyncGitOpsApplication("ns", "name", "Kustomization")
	if err == nil {
		t.Fatal("expected read-only error")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("err = %q, expected substring 'read-only'", err.Error())
	}
	// Guidance string must NOT be returned when read-only blocks first.
	if strings.Contains(err.Error(), "flux reconcile") {
		t.Errorf("read-only should short-circuit before guidance; err = %q", err.Error())
	}
}

// TestSyncGitOpsApplication_NotReadOnly_ReturnsGuidance pins the happy-path:
// with readOnly=false the guidance string is returned. This is the contract
// the UI depends on to render a copy-paste CLI instruction.
func TestSyncGitOpsApplication_NotReadOnly_ReturnsGuidance(t *testing.T) {
	withReadOnlyConfig(t, false)
	a := newAppWithFakes(newFakeClusterClient())

	err := a.SyncGitOpsApplication("ns", "name", "HelmRelease")
	if err == nil {
		t.Fatal("expected guidance error even with readOnly=false")
	}
	if !strings.Contains(err.Error(), "flux reconcile") {
		t.Errorf("err = %q, want substring 'flux reconcile'", err.Error())
	}
}

// TestGetGitOpsStatus_HelmReleasesAloneDetectsFlux pins that a cluster with
// ONLY HelmReleases (no Kustomizations) still reports provider=flux. This is
// relevant for Flux v2 clusters that use HelmRelease without
// Kustomization-driven directory layouts.
func TestGetGitOpsStatus_HelmReleasesAloneDetectsFlux(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("helmreleases", fluxHelmReleaseObj("prom", "monitoring", "prom", "1.0", "True"))
	a := newAppWithFakes(cl)

	status, err := a.GetGitOpsStatus()
	if err != nil {
		t.Fatalf("GetGitOpsStatus: %v", err)
	}
	if status.Provider != "flux" || !status.Detected {
		t.Errorf("expected flux/Detected=true from HelmReleases alone, got provider=%q detected=%v",
			status.Provider, status.Detected)
	}
}

// TestGetGitOpsStatus_FluxPreemptsArgoCD documents the ordering: when BOTH
// Flux and ArgoCD CRDs are present, the Flux branch fires first and ArgoCD
// applications are ignored. This pins the current policy.
func TestGetGitOpsStatus_FluxPreemptsArgoCD(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("kustomizations", fluxKustomizationObj("k", "flux-system", "True", ""))
	cl.addResource("applications", argoApplicationObj("a", "argocd", "Synced", "Healthy", ""))
	a := newAppWithFakes(cl)

	status, err := a.GetGitOpsStatus()
	if err != nil {
		t.Fatalf("GetGitOpsStatus: %v", err)
	}
	if status.Provider != "flux" {
		t.Errorf("provider = %q, want flux (Flux should preempt ArgoCD detection)", status.Provider)
	}
	// Only the Flux Kustomization should be reported.
	if len(status.Applications) != 1 {
		t.Errorf("expected 1 Flux app, got %d (ArgoCD apps leaked?)", len(status.Applications))
	}
}

// TestUpdateGitOpsSummary_ProgressingAndHealthyCounters exercises the
// small counter switch in updateGitOpsSummary that isn't hit by the
// happy-path tests (Progressing health + Synced sync).
func TestUpdateGitOpsSummary_ProgressingAndHealthyCounters(t *testing.T) {
	var s GitOpsSummary
	updateGitOpsSummary(&s, GitOpsApplicationInfo{SyncStatus: "Synced", HealthStatus: "Progressing"})
	updateGitOpsSummary(&s, GitOpsApplicationInfo{SyncStatus: "OutOfSync", HealthStatus: "Healthy"})
	updateGitOpsSummary(&s, GitOpsApplicationInfo{SyncStatus: "Unknown", HealthStatus: "Degraded"})
	updateGitOpsSummary(&s, GitOpsApplicationInfo{SyncStatus: "Synced", HealthStatus: "Suspended"}) // Suspended not counted

	if s.Synced != 2 {
		t.Errorf("Synced = %d, want 2", s.Synced)
	}
	if s.OutOfSync != 1 {
		t.Errorf("OutOfSync = %d, want 1", s.OutOfSync)
	}
	if s.Healthy != 1 {
		t.Errorf("Healthy = %d, want 1", s.Healthy)
	}
	if s.Degraded != 1 {
		t.Errorf("Degraded = %d, want 1", s.Degraded)
	}
	if s.Progressing != 1 {
		t.Errorf("Progressing = %d, want 1", s.Progressing)
	}
}

// TestGetGitOpsApplication_HelmRelease_SourceFields verifies chart+version+
// repository propagate to the GitOpsSource.
func TestGetGitOpsApplication_HelmRelease_SourceFields(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("helmreleases", fluxHelmReleaseObj("prom", "mon", "prometheus", "25.0.0", "True"))
	a := newAppWithFakes(cl)

	got, err := a.GetGitOpsApplication("mon", "prom", "HelmRelease")
	if err != nil {
		t.Fatalf("GetGitOpsApplication: %v", err)
	}
	if got.Source.Type != "helm" {
		t.Errorf("Source.Type = %q, want helm", got.Source.Type)
	}
	if got.Source.Chart != "prometheus" || got.Source.Version != "25.0.0" {
		t.Errorf("Source chart/version = %q/%q, want prometheus/25.0.0",
			got.Source.Chart, got.Source.Version)
	}
	if got.Source.Repository != "repo" {
		t.Errorf("Source.Repository = %q, want repo", got.Source.Repository)
	}
}

// TestGetGitOpsApplication_ApplicationMissingInNamespace_ReturnsError pins the
// not-found error propagation for Application kind specifically.
func TestGetGitOpsApplication_ApplicationMissingInNamespace_ReturnsError(t *testing.T) {
	cl := newFakeClusterClient()
	a := newAppWithFakes(cl)

	_, err := a.GetGitOpsApplication("argocd", "ghost", "Application")
	if err == nil {
		t.Error("expected error for non-existent ArgoCD Application")
	}
}
