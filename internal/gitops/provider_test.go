// SPDX-License-Identifier: Apache-2.0

package gitops

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/thepixelabs/kubecat/internal/client"
)

// ---------------------------------------------------------------------------
// fakeClient
// ---------------------------------------------------------------------------

// fakeClient is a minimal client.ClusterClient for gitops tests.
type fakeClient struct {
	resources map[string][]client.Resource
	listErrs  map[string]error // kind -> error
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		resources: make(map[string][]client.Resource),
		listErrs:  make(map[string]error),
	}
}

func (f *fakeClient) addResource(kind string, obj map[string]interface{}) {
	raw, _ := json.Marshal(obj)
	var name, namespace string
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		name, _ = meta["name"].(string)
		namespace, _ = meta["namespace"].(string)
	}
	f.resources[kind] = append(f.resources[kind], client.Resource{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
		Raw:       raw,
		Object:    obj,
	})
}

func (f *fakeClient) Info(_ context.Context) (*client.ClusterInfo, error) {
	return &client.ClusterInfo{Name: "fake"}, nil
}
func (f *fakeClient) List(_ context.Context, kind string, _ client.ListOptions) (*client.ResourceList, error) {
	if err, ok := f.listErrs[kind]; ok {
		return nil, err
	}
	items := f.resources[kind]
	return &client.ResourceList{Items: items, Total: len(items)}, nil
}
func (f *fakeClient) Get(_ context.Context, kind, namespace, name string) (*client.Resource, error) {
	for _, r := range f.resources[kind] {
		if r.Name == name && r.Namespace == namespace {
			return &r, nil
		}
	}
	return nil, client.ErrResourceNotFound
}
func (f *fakeClient) Delete(_ context.Context, _, _, _ string) error { return nil }
func (f *fakeClient) Watch(_ context.Context, _ string, _ client.WatchOptions) (<-chan client.WatchEvent, error) {
	ch := make(chan client.WatchEvent)
	close(ch)
	return ch, nil
}
func (f *fakeClient) Logs(_ context.Context, _, _, _ string, _ bool, _ int64) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}
func (f *fakeClient) Exec(_ context.Context, _, _, _ string, _ []string) error { return nil }
func (f *fakeClient) PortForward(_ context.Context, _, _ string, _, _ int) (client.PortForwarder, error) {
	return nil, nil
}
func (f *fakeClient) Close() error { return nil }

// ---------------------------------------------------------------------------
// DetectProvider
// ---------------------------------------------------------------------------

// TestDetectProvider_Flux returns a FluxProvider when Kustomizations list.
func TestDetectProvider_Flux(t *testing.T) {
	cl := newFakeClient()
	cl.addResource("kustomizations", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "infra", "namespace": "flux-system"},
	})
	p, err := DetectProvider(context.Background(), cl)
	if err != nil {
		t.Fatalf("DetectProvider: %v", err)
	}
	if p == nil || p.Type() != ProviderFlux {
		t.Errorf("expected Flux provider, got %v", p)
	}
}

// TestDetectProvider_Flux_ViaHelmReleases detects Flux via HelmReleases when
// the Kustomizations list path errors out.
func TestDetectProvider_Flux_ViaHelmReleases(t *testing.T) {
	cl := newFakeClient()
	cl.listErrs["kustomizations"] = errors.New("no such crd")
	cl.addResource("helmreleases", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "prom", "namespace": "monitoring"},
	})
	p, err := DetectProvider(context.Background(), cl)
	if err != nil {
		t.Fatalf("DetectProvider: %v", err)
	}
	if p == nil || p.Type() != ProviderFlux {
		t.Errorf("expected Flux via HelmReleases, got %v", p)
	}
}

// TestDetectProvider_ArgoCD returns an ArgoCDProvider when Applications list
// and neither Flux CRD responds.
func TestDetectProvider_ArgoCD(t *testing.T) {
	cl := newFakeClient()
	cl.listErrs["kustomizations"] = errors.New("nope")
	cl.listErrs["helmreleases"] = errors.New("nope")
	cl.addResource("applications", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "app", "namespace": "argocd"},
	})
	p, err := DetectProvider(context.Background(), cl)
	if err != nil {
		t.Fatalf("DetectProvider: %v", err)
	}
	if p == nil || p.Type() != ProviderArgoCD {
		t.Errorf("expected ArgoCD provider, got %v", p)
	}
}

// TestDetectProvider_None returns (nil, nil) when no CRDs match.
func TestDetectProvider_None(t *testing.T) {
	cl := newFakeClient()
	cl.listErrs["kustomizations"] = errors.New("nope")
	cl.listErrs["helmreleases"] = errors.New("nope")
	cl.listErrs["applications"] = errors.New("nope")

	p, err := DetectProvider(context.Background(), cl)
	if err != nil {
		t.Fatalf("DetectProvider: %v", err)
	}
	if p != nil {
		t.Errorf("expected nil provider when no CRDs match, got %v", p)
	}
}

// ---------------------------------------------------------------------------
// FluxProvider
// ---------------------------------------------------------------------------

func fluxKs(name, namespace, ready string) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{"name": name, "namespace": namespace},
		"spec": map[string]interface{}{
			"path":      "./",
			"sourceRef": map[string]interface{}{"kind": "GitRepository", "name": "infra"},
		},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": ready},
			},
			"lastAppliedRevision":    "abc",
			"lastHandledReconcileAt": "2024-01-01T00:00:00Z",
		},
	}
}

func fluxHr(name, namespace, ready string) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{"name": name, "namespace": namespace},
		"spec": map[string]interface{}{
			"chart": map[string]interface{}{
				"spec": map[string]interface{}{
					"chart":     "my-chart",
					"version":   "1.0",
					"sourceRef": map[string]interface{}{"kind": "HelmRepository", "name": "repo"},
				},
			},
		},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": ready},
			},
		},
	}
}

// TestFluxProvider_ListApplications_ReadsBothCRDs pulls both Kustomizations
// and HelmReleases.
func TestFluxProvider_ListApplications_ReadsBothCRDs(t *testing.T) {
	cl := newFakeClient()
	cl.addResource("kustomizations", fluxKs("app", "flux-system", "True"))
	cl.addResource("helmreleases", fluxHr("chart", "flux-system", "True"))

	p := NewFluxProvider(cl)
	apps, err := p.ListApplications(context.Background())
	if err != nil {
		t.Fatalf("ListApplications: %v", err)
	}
	if len(apps) != 2 {
		t.Errorf("expected 2 applications, got %d", len(apps))
	}
}

// TestFluxProvider_SyncSuspendResume_ReturnGuidance verifies the not-yet-
// implemented ops return descriptive errors and don't mutate.
func TestFluxProvider_SyncSuspendResume_ReturnGuidance(t *testing.T) {
	p := NewFluxProvider(newFakeClient())
	ctx := context.Background()

	cases := []struct {
		op   string
		err  error
		want string
	}{
		{"sync", p.Sync(ctx, "ns", "a"), "flux reconcile"},
		{"suspend", p.Suspend(ctx, "ns", "a"), "flux suspend"},
		{"resume", p.Resume(ctx, "ns", "a"), "flux resume"},
	}
	for _, c := range cases {
		if c.err == nil {
			t.Errorf("%s should return guidance error", c.op)
			continue
		}
		if !strings.Contains(c.err.Error(), c.want) {
			t.Errorf("%s err = %q, want substring %q", c.op, c.err.Error(), c.want)
		}
	}
}

// TestFluxProvider_GetApplication_Kustomization returns a parsed Application.
func TestFluxProvider_GetApplication_Kustomization(t *testing.T) {
	cl := newFakeClient()
	cl.addResource("kustomizations", fluxKs("my-app", "flux-system", "True"))

	p := NewFluxProvider(cl)
	app, err := p.GetApplication(context.Background(), "flux-system", "my-app")
	if err != nil {
		t.Fatalf("GetApplication: %v", err)
	}
	if app.Name != "my-app" || app.Kind != "Kustomization" {
		t.Errorf("got %+v, want name=my-app kind=Kustomization", app)
	}
	if app.SyncStatus != SyncStatusSynced {
		t.Errorf("sync = %q, want Synced", app.SyncStatus)
	}
}

// TestFluxProvider_GetApplication_NotFound returns an error.
func TestFluxProvider_GetApplication_NotFound(t *testing.T) {
	p := NewFluxProvider(newFakeClient())
	_, err := p.GetApplication(context.Background(), "ns", "ghost")
	if err == nil {
		t.Error("expected error for missing application")
	}
}

// TestFluxProvider_GetDrift_DetectsOutOfSync verifies HasDrift=true when the
// Ready condition is False.
func TestFluxProvider_GetDrift_DetectsOutOfSync(t *testing.T) {
	cl := newFakeClient()
	cl.addResource("kustomizations", fluxKs("a", "flux-system", "False"))
	p := NewFluxProvider(cl)
	drift, err := p.GetDrift(context.Background(), "flux-system", "a")
	if err != nil {
		t.Fatalf("GetDrift: %v", err)
	}
	if !drift.HasDrift {
		t.Error("expected HasDrift=true for False Ready condition")
	}
}

// TestFluxProvider_ParseKustomization_Suspend overrides HealthStatus.
func TestFluxProvider_ParseKustomization_Suspend(t *testing.T) {
	cl := newFakeClient()
	obj := fluxKs("s", "flux-system", "True")
	obj["spec"].(map[string]interface{})["suspend"] = true
	cl.addResource("kustomizations", obj)

	p := NewFluxProvider(cl)
	app, _ := p.GetApplication(context.Background(), "flux-system", "s")
	if app.HealthStatus != HealthStatusSuspended {
		t.Errorf("health = %q, want Suspended", app.HealthStatus)
	}
}

// TestFluxProvider_Reconciling_MapsToProgressing when Reconciling=True.
func TestFluxProvider_Reconciling_MapsToProgressing(t *testing.T) {
	cl := newFakeClient()
	obj := fluxKs("r", "flux-system", "Unknown")
	obj["status"].(map[string]interface{})["conditions"] = []interface{}{
		map[string]interface{}{"type": "Reconciling", "status": "True"},
	}
	cl.addResource("kustomizations", obj)

	p := NewFluxProvider(cl)
	app, _ := p.GetApplication(context.Background(), "flux-system", "r")
	if app.SyncStatus != SyncStatusProgressing {
		t.Errorf("sync = %q, want Progressing", app.SyncStatus)
	}
}

// ---------------------------------------------------------------------------
// ArgoCDProvider
// ---------------------------------------------------------------------------

func argoApp(name, namespace, sync, health string) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{"name": name, "namespace": namespace},
		"spec": map[string]interface{}{
			"source": map[string]interface{}{
				"repoURL":        "https://example.com/repo",
				"path":           "apps/foo",
				"targetRevision": "main",
			},
		},
		"status": map[string]interface{}{
			"sync":   map[string]interface{}{"status": sync, "revision": "deadbeef"},
			"health": map[string]interface{}{"status": health, "message": ""},
		},
	}
}

// TestArgoCDProvider_ListApplications enumerates Applications.
func TestArgoCDProvider_ListApplications(t *testing.T) {
	cl := newFakeClient()
	cl.addResource("applications", argoApp("a", "argocd", "Synced", "Healthy"))
	cl.addResource("applications", argoApp("b", "argocd", "OutOfSync", "Degraded"))

	p := NewArgoCDProvider(cl)
	apps, err := p.ListApplications(context.Background())
	if err != nil {
		t.Fatalf("ListApplications: %v", err)
	}
	if len(apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(apps))
	}
}

// TestArgoCDProvider_GetApplication_StatusMappingTable pins the mapping of
// ArgoCD sync and health strings to our typed constants.
func TestArgoCDProvider_GetApplication_StatusMappingTable(t *testing.T) {
	cases := []struct {
		sync       string
		health     string
		wantSync   SyncStatus
		wantHealth HealthStatus
	}{
		{"Synced", "Healthy", SyncStatusSynced, HealthStatusHealthy},
		{"OutOfSync", "Degraded", SyncStatusOutOfSync, HealthStatusDegraded},
		{"Unknown", "Progressing", SyncStatusUnknown, HealthStatusProgressing},
		{"", "Suspended", SyncStatusUnknown, HealthStatusSuspended}, // default
		{"Synced", "Missing", SyncStatusSynced, HealthStatusMissing},
		{"Synced", "Unknown", SyncStatusSynced, HealthStatusUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.sync+"/"+tc.health, func(t *testing.T) {
			cl := newFakeClient()
			cl.addResource("applications", argoApp("app", "argocd", tc.sync, tc.health))
			p := NewArgoCDProvider(cl)
			app, err := p.GetApplication(context.Background(), "argocd", "app")
			if err != nil {
				t.Fatalf("GetApplication: %v", err)
			}
			if app.SyncStatus != tc.wantSync {
				t.Errorf("sync = %q, want %q", app.SyncStatus, tc.wantSync)
			}
			if app.HealthStatus != tc.wantHealth {
				t.Errorf("health = %q, want %q", app.HealthStatus, tc.wantHealth)
			}
		})
	}
}

// TestArgoCDProvider_GetApplication_HelmSource detects chart source type.
func TestArgoCDProvider_GetApplication_HelmSource(t *testing.T) {
	cl := newFakeClient()
	obj := argoApp("helm-app", "argocd", "Synced", "Healthy")
	obj["spec"].(map[string]interface{})["source"].(map[string]interface{})["chart"] = "my-chart"
	cl.addResource("applications", obj)

	p := NewArgoCDProvider(cl)
	app, err := p.GetApplication(context.Background(), "argocd", "helm-app")
	if err != nil {
		t.Fatalf("GetApplication: %v", err)
	}
	if app.Source.Type != "helm" {
		t.Errorf("source.Type = %q, want helm", app.Source.Type)
	}
}

// TestArgoCDProvider_GetDrift_OutOfSync returns HasDrift=true with a resource
// entry for each non-synced resource.
func TestArgoCDProvider_GetDrift_OutOfSync(t *testing.T) {
	cl := newFakeClient()
	obj := argoApp("a", "argocd", "OutOfSync", "Degraded")
	obj["status"].(map[string]interface{})["resources"] = []interface{}{
		map[string]interface{}{"kind": "Deployment", "name": "web", "status": "OutOfSync"},
		map[string]interface{}{"kind": "Service", "name": "svc", "status": "Synced"}, // skipped
	}
	cl.addResource("applications", obj)

	p := NewArgoCDProvider(cl)
	drift, err := p.GetDrift(context.Background(), "argocd", "a")
	if err != nil {
		t.Fatalf("GetDrift: %v", err)
	}
	if !drift.HasDrift {
		t.Error("expected HasDrift=true for OutOfSync status")
	}
	if len(drift.Resources) != 1 {
		t.Errorf("expected 1 drift resource (non-synced only), got %d", len(drift.Resources))
	}
	if drift.Resources[0].Kind != "Deployment" {
		t.Errorf("drift[0].Kind = %q, want Deployment", drift.Resources[0].Kind)
	}
}

// TestArgoCDProvider_SyncSuspendResume_ReturnGuidance verifies those ops
// return informational errors.
func TestArgoCDProvider_SyncSuspendResume_ReturnGuidance(t *testing.T) {
	p := NewArgoCDProvider(newFakeClient())
	ctx := context.Background()
	if err := p.Sync(ctx, "ns", "a"); err == nil || !strings.Contains(err.Error(), "argocd app sync") {
		t.Errorf("Sync = %v, want guidance to argocd app sync", err)
	}
	if err := p.Suspend(ctx, "ns", "a"); err == nil {
		t.Errorf("Suspend should return not-yet-implemented error")
	}
	if err := p.Resume(ctx, "ns", "a"); err == nil {
		t.Errorf("Resume should return not-yet-implemented error")
	}
}

// TestArgoCDProvider_Type returns the correct ProviderType.
func TestArgoCDProvider_Type(t *testing.T) {
	p := NewArgoCDProvider(newFakeClient())
	if p.Type() != ProviderArgoCD {
		t.Errorf("Type = %q, want %q", p.Type(), ProviderArgoCD)
	}
}

// TestFluxProvider_Type returns the correct ProviderType.
func TestFluxProvider_Type(t *testing.T) {
	p := NewFluxProvider(newFakeClient())
	if p.Type() != ProviderFlux {
		t.Errorf("Type = %q, want %q", p.Type(), ProviderFlux)
	}
}
