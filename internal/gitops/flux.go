// SPDX-License-Identifier: Apache-2.0

package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
)

// FluxProvider implements the Provider interface for Flux CD.
type FluxProvider struct {
	client client.ClusterClient
}

// NewFluxProvider creates a new Flux provider.
func NewFluxProvider(cl client.ClusterClient) *FluxProvider {
	return &FluxProvider{client: cl}
}

// Type returns the provider type.
func (p *FluxProvider) Type() ProviderType {
	return ProviderFlux
}

// ListApplications returns all Flux applications (Kustomizations and HelmReleases).
func (p *FluxProvider) ListApplications(ctx context.Context) ([]Application, error) {
	var apps []Application

	// Get Kustomizations
	ksList, err := p.client.List(ctx, "kustomizations", client.ListOptions{Limit: 1000})
	if err == nil {
		for _, r := range ksList.Items {
			app, err := p.parseKustomization(r)
			if err == nil {
				apps = append(apps, *app)
			}
		}
	}

	// Get HelmReleases
	hrList, err := p.client.List(ctx, "helmreleases", client.ListOptions{Limit: 1000})
	if err == nil {
		for _, r := range hrList.Items {
			app, err := p.parseHelmRelease(r)
			if err == nil {
				apps = append(apps, *app)
			}
		}
	}

	return apps, nil
}

// GetApplication returns a specific Flux application.
func (p *FluxProvider) GetApplication(ctx context.Context, namespace, name string) (*Application, error) {
	// Try Kustomization first
	ks, err := p.client.Get(ctx, "kustomizations", namespace, name)
	if err == nil {
		return p.parseKustomization(*ks)
	}

	// Try HelmRelease
	hr, err := p.client.Get(ctx, "helmreleases", namespace, name)
	if err == nil {
		return p.parseHelmRelease(*hr)
	}

	return nil, fmt.Errorf("application not found: %s/%s", namespace, name)
}

// GetDrift returns drift information (Flux doesn't expose detailed drift, return basic status).
func (p *FluxProvider) GetDrift(ctx context.Context, namespace, name string) (*Drift, error) {
	app, err := p.GetApplication(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	drift := &Drift{
		Application: app.Name,
		HasDrift:    app.SyncStatus == SyncStatusOutOfSync,
		Resources:   []DriftEntry{},
	}

	return drift, nil
}

// Sync triggers a reconciliation for a Flux application.
func (p *FluxProvider) Sync(ctx context.Context, namespace, name string) error {
	// In Flux, we trigger reconciliation by annotating the resource
	// This would require an Update/Patch operation which isn't in the current client interface
	// For now, return an informational error
	return fmt.Errorf("sync via annotation not yet implemented - use 'flux reconcile' CLI")
}

// Suspend suspends reconciliation for a Flux application.
func (p *FluxProvider) Suspend(ctx context.Context, namespace, name string) error {
	return fmt.Errorf("suspend not yet implemented - use 'flux suspend' CLI")
}

// Resume resumes reconciliation for a Flux application.
func (p *FluxProvider) Resume(ctx context.Context, namespace, name string) error {
	return fmt.Errorf("resume not yet implemented - use 'flux resume' CLI")
}

func (p *FluxProvider) parseKustomization(r client.Resource) (*Application, error) {
	var ks fluxKustomization
	if err := json.Unmarshal(r.Raw, &ks); err != nil {
		return nil, err
	}

	app := &Application{
		Name:      ks.Metadata.Name,
		Namespace: ks.Metadata.Namespace,
		Provider:  ProviderFlux,
		Kind:      "Kustomization",
		Labels:    ks.Metadata.Labels,
		Source: Source{
			Type: "git",
			Path: ks.Spec.Path,
		},
	}

	// Parse source reference
	if ks.Spec.SourceRef.Kind == "GitRepository" {
		app.Source.Repository = ks.Spec.SourceRef.Name
	}

	// Parse status
	app.SyncStatus = SyncStatusUnknown
	app.HealthStatus = HealthStatusUnknown

	for _, cond := range ks.Status.Conditions {
		switch cond.Type {
		case "Ready":
			if cond.Status == "True" {
				app.SyncStatus = SyncStatusSynced
				app.HealthStatus = HealthStatusHealthy
			} else {
				app.SyncStatus = SyncStatusOutOfSync
				app.HealthStatus = HealthStatusDegraded
				app.Message = cond.Message
			}
		case "Reconciling":
			if cond.Status == "True" {
				app.SyncStatus = SyncStatusProgressing
				app.HealthStatus = HealthStatusProgressing
			}
		}
	}

	if ks.Spec.Suspend {
		app.HealthStatus = HealthStatusSuspended
	}

	app.Revision = ks.Status.LastAppliedRevision
	if ks.Status.LastHandledReconcileAt != "" {
		t, err := time.Parse(time.RFC3339, ks.Status.LastHandledReconcileAt)
		if err == nil {
			app.LastSyncTime = &t
		}
	}

	return app, nil
}

func (p *FluxProvider) parseHelmRelease(r client.Resource) (*Application, error) {
	var hr fluxHelmRelease
	if err := json.Unmarshal(r.Raw, &hr); err != nil {
		return nil, err
	}

	app := &Application{
		Name:      hr.Metadata.Name,
		Namespace: hr.Metadata.Namespace,
		Provider:  ProviderFlux,
		Kind:      "HelmRelease",
		Labels:    hr.Metadata.Labels,
		Source: Source{
			Type:    "helm",
			Chart:   hr.Spec.Chart.Spec.Chart,
			Version: hr.Spec.Chart.Spec.Version,
		},
	}

	if hr.Spec.Chart.Spec.SourceRef.Kind == "HelmRepository" {
		app.Source.Repository = hr.Spec.Chart.Spec.SourceRef.Name
	}

	// Parse status
	app.SyncStatus = SyncStatusUnknown
	app.HealthStatus = HealthStatusUnknown

	for _, cond := range hr.Status.Conditions {
		switch cond.Type {
		case "Ready":
			if cond.Status == "True" {
				app.SyncStatus = SyncStatusSynced
				app.HealthStatus = HealthStatusHealthy
			} else {
				app.SyncStatus = SyncStatusOutOfSync
				app.HealthStatus = HealthStatusDegraded
				app.Message = cond.Message
			}
		case "Reconciling":
			if cond.Status == "True" {
				app.SyncStatus = SyncStatusProgressing
				app.HealthStatus = HealthStatusProgressing
			}
		}
	}

	if hr.Spec.Suspend {
		app.HealthStatus = HealthStatusSuspended
	}

	app.Revision = hr.Status.LastAppliedRevision
	if hr.Status.LastHandledReconcileAt != "" {
		t, err := time.Parse(time.RFC3339, hr.Status.LastHandledReconcileAt)
		if err == nil {
			app.LastSyncTime = &t
		}
	}

	return app, nil
}

// Flux CRD types for JSON parsing

type fluxKustomization struct {
	Metadata struct {
		Name      string            `json:"name"`
		Namespace string            `json:"namespace"`
		Labels    map[string]string `json:"labels"`
	} `json:"metadata"`
	Spec struct {
		Path      string `json:"path"`
		Interval  string `json:"interval"`
		Suspend   bool   `json:"suspend"`
		SourceRef struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		} `json:"sourceRef"`
	} `json:"spec"`
	Status struct {
		Conditions []struct {
			Type    string `json:"type"`
			Status  string `json:"status"`
			Reason  string `json:"reason"`
			Message string `json:"message"`
		} `json:"conditions"`
		LastAppliedRevision    string `json:"lastAppliedRevision"`
		LastHandledReconcileAt string `json:"lastHandledReconcileAt"`
	} `json:"status"`
}

type fluxHelmRelease struct {
	Metadata struct {
		Name      string            `json:"name"`
		Namespace string            `json:"namespace"`
		Labels    map[string]string `json:"labels"`
	} `json:"metadata"`
	Spec struct {
		Interval string `json:"interval"`
		Suspend  bool   `json:"suspend"`
		Chart    struct {
			Spec struct {
				Chart     string `json:"chart"`
				Version   string `json:"version"`
				SourceRef struct {
					Kind string `json:"kind"`
					Name string `json:"name"`
				} `json:"sourceRef"`
			} `json:"spec"`
		} `json:"chart"`
	} `json:"spec"`
	Status struct {
		Conditions []struct {
			Type    string `json:"type"`
			Status  string `json:"status"`
			Reason  string `json:"reason"`
			Message string `json:"message"`
		} `json:"conditions"`
		LastAppliedRevision    string `json:"lastAppliedRevision"`
		LastHandledReconcileAt string `json:"lastHandledReconcileAt"`
	} `json:"status"`
}
