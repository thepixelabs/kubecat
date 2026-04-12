// SPDX-License-Identifier: Apache-2.0

package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
)

// ArgoCDProvider implements the Provider interface for ArgoCD.
type ArgoCDProvider struct {
	client client.ClusterClient
}

// NewArgoCDProvider creates a new ArgoCD provider.
func NewArgoCDProvider(cl client.ClusterClient) *ArgoCDProvider {
	return &ArgoCDProvider{client: cl}
}

// Type returns the provider type.
func (p *ArgoCDProvider) Type() ProviderType {
	return ProviderArgoCD
}

// ListApplications returns all ArgoCD applications.
func (p *ArgoCDProvider) ListApplications(ctx context.Context) ([]Application, error) {
	var apps []Application

	// Get Applications
	appList, err := p.client.List(ctx, "applications", client.ListOptions{Limit: 1000})
	if err != nil {
		return nil, err
	}

	for _, r := range appList.Items {
		app, err := p.parseApplication(r)
		if err == nil {
			apps = append(apps, *app)
		}
	}

	return apps, nil
}

// GetApplication returns a specific ArgoCD application.
func (p *ArgoCDProvider) GetApplication(ctx context.Context, namespace, name string) (*Application, error) {
	resource, err := p.client.Get(ctx, "applications", namespace, name)
	if err != nil {
		return nil, err
	}

	return p.parseApplication(*resource)
}

// GetDrift returns drift information for an ArgoCD application.
func (p *ArgoCDProvider) GetDrift(ctx context.Context, namespace, name string) (*Drift, error) {
	resource, err := p.client.Get(ctx, "applications", namespace, name)
	if err != nil {
		return nil, err
	}

	var argoApp argoApplication
	if err := json.Unmarshal(resource.Raw, &argoApp); err != nil {
		return nil, err
	}

	drift := &Drift{
		Application: name,
		HasDrift:    argoApp.Status.Sync.Status != "Synced",
		Resources:   []DriftEntry{},
	}

	// Parse resource statuses for drift details
	for _, res := range argoApp.Status.Resources {
		if res.Status != "Synced" {
			entry := DriftEntry{
				Kind:      res.Kind,
				Name:      res.Name,
				Namespace: res.Namespace,
				Status:    res.Status,
			}
			drift.Resources = append(drift.Resources, entry)
		}
	}

	return drift, nil
}

// Sync triggers a sync for an ArgoCD application.
func (p *ArgoCDProvider) Sync(ctx context.Context, namespace, name string) error {
	// ArgoCD sync requires creating an Operation on the Application
	// This would require a Patch operation which isn't in the current client interface
	return fmt.Errorf("sync not yet implemented - use 'argocd app sync %s' CLI", name)
}

// Suspend suspends auto-sync for an ArgoCD application.
func (p *ArgoCDProvider) Suspend(ctx context.Context, namespace, name string) error {
	return fmt.Errorf("suspend not yet implemented - disable auto-sync in ArgoCD UI or CLI")
}

// Resume resumes auto-sync for an ArgoCD application.
func (p *ArgoCDProvider) Resume(ctx context.Context, namespace, name string) error {
	return fmt.Errorf("resume not yet implemented - enable auto-sync in ArgoCD UI or CLI")
}

func (p *ArgoCDProvider) parseApplication(r client.Resource) (*Application, error) {
	var argoApp argoApplication
	if err := json.Unmarshal(r.Raw, &argoApp); err != nil {
		return nil, err
	}

	app := &Application{
		Name:      argoApp.Metadata.Name,
		Namespace: argoApp.Metadata.Namespace,
		Provider:  ProviderArgoCD,
		Kind:      "Application",
		Labels:    argoApp.Metadata.Labels,
	}

	// Parse source
	if argoApp.Spec.Source.RepoURL != "" {
		app.Source = Source{
			URL:      argoApp.Spec.Source.RepoURL,
			Path:     argoApp.Spec.Source.Path,
			Branch:   argoApp.Spec.Source.TargetRevision,
			Revision: argoApp.Spec.Source.TargetRevision,
		}

		if argoApp.Spec.Source.Chart != "" {
			app.Source.Type = "helm"
			app.Source.Chart = argoApp.Spec.Source.Chart
		} else {
			app.Source.Type = "git"
		}
	}

	// Parse sync status
	switch argoApp.Status.Sync.Status {
	case "Synced":
		app.SyncStatus = SyncStatusSynced
	case "OutOfSync":
		app.SyncStatus = SyncStatusOutOfSync
	case "Unknown":
		app.SyncStatus = SyncStatusUnknown
	default:
		app.SyncStatus = SyncStatusUnknown
	}

	// Parse health status
	switch argoApp.Status.Health.Status {
	case "Healthy":
		app.HealthStatus = HealthStatusHealthy
	case "Degraded":
		app.HealthStatus = HealthStatusDegraded
	case "Progressing":
		app.HealthStatus = HealthStatusProgressing
	case "Suspended":
		app.HealthStatus = HealthStatusSuspended
	case "Missing":
		app.HealthStatus = HealthStatusMissing
	default:
		app.HealthStatus = HealthStatusUnknown
	}

	app.Message = argoApp.Status.Health.Message
	app.Revision = argoApp.Status.Sync.Revision

	if argoApp.Status.ReconciledAt != "" {
		t, err := time.Parse(time.RFC3339, argoApp.Status.ReconciledAt)
		if err == nil {
			app.LastSyncTime = &t
		}
	}

	return app, nil
}

// ArgoCD CRD types for JSON parsing

type argoApplication struct {
	Metadata struct {
		Name      string            `json:"name"`
		Namespace string            `json:"namespace"`
		Labels    map[string]string `json:"labels"`
	} `json:"metadata"`
	Spec struct {
		Source struct {
			RepoURL        string `json:"repoURL"`
			Path           string `json:"path"`
			TargetRevision string `json:"targetRevision"`
			Chart          string `json:"chart"`
			Helm           struct {
				ValueFiles []string `json:"valueFiles"`
			} `json:"helm"`
		} `json:"source"`
		Destination struct {
			Server    string `json:"server"`
			Namespace string `json:"namespace"`
		} `json:"destination"`
		SyncPolicy struct {
			Automated struct {
				Prune    bool `json:"prune"`
				SelfHeal bool `json:"selfHeal"`
			} `json:"automated"`
		} `json:"syncPolicy"`
	} `json:"spec"`
	Status struct {
		Sync struct {
			Status   string `json:"status"`
			Revision string `json:"revision"`
		} `json:"sync"`
		Health struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"health"`
		OperationState struct {
			Phase   string `json:"phase"`
			Message string `json:"message"`
		} `json:"operationState"`
		ReconciledAt string `json:"reconciledAt"`
		Resources    []struct {
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
			Status    string `json:"status"`
			Health    struct {
				Status  string `json:"status"`
				Message string `json:"message"`
			} `json:"health"`
		} `json:"resources"`
	} `json:"status"`
}
