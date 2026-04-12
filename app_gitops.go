// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thepixelabs/kubecat/internal/client"
)

// GitOpsApplicationInfo is a JSON-friendly GitOps application.
type GitOpsApplicationInfo struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Provider     string            `json:"provider"`
	Kind         string            `json:"kind"`
	Source       GitOpsSource      `json:"source"`
	SyncStatus   string            `json:"syncStatus"`
	HealthStatus string            `json:"healthStatus"`
	Message      string            `json:"message,omitempty"`
	LastSyncTime string            `json:"lastSyncTime,omitempty"`
	Revision     string            `json:"revision,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

// GitOpsSource contains source information.
type GitOpsSource struct {
	Type       string `json:"type"`
	URL        string `json:"url,omitempty"`
	Path       string `json:"path,omitempty"`
	Branch     string `json:"branch,omitempty"`
	Revision   string `json:"revision,omitempty"`
	Chart      string `json:"chart,omitempty"`
	Version    string `json:"version,omitempty"`
	Repository string `json:"repository,omitempty"`
}

// GitOpsStatusInfo contains overall GitOps status.
type GitOpsStatusInfo struct {
	Provider     string                  `json:"provider"`
	Detected     bool                    `json:"detected"`
	Applications []GitOpsApplicationInfo `json:"applications"`
	Summary      GitOpsSummary           `json:"summary"`
}

// GitOpsSummary provides application counts.
type GitOpsSummary struct {
	Total       int `json:"total"`
	Synced      int `json:"synced"`
	OutOfSync   int `json:"outOfSync"`
	Healthy     int `json:"healthy"`
	Degraded    int `json:"degraded"`
	Progressing int `json:"progressing"`
}

// GetGitOpsStatus returns the GitOps status for the active cluster.
func (a *App) GetGitOpsStatus() (*GitOpsStatusInfo, error) {
	cl, err := a.nexus.Clusters.Manager().Active()
	if err != nil {
		return nil, fmt.Errorf("no active cluster: %w", err)
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	status := &GitOpsStatusInfo{
		Provider:     "none",
		Detected:     false,
		Applications: []GitOpsApplicationInfo{},
		Summary:      GitOpsSummary{},
	}

	// Try to detect Flux
	ksList, err := cl.List(ctx, "kustomizations", client.ListOptions{Limit: 1000})
	if err == nil && len(ksList.Items) > 0 {
		status.Provider = "flux"
		status.Detected = true
		for _, ks := range ksList.Items {
			app := parseFluxKustomization(ks)
			status.Applications = append(status.Applications, app)
			updateGitOpsSummary(&status.Summary, app)
		}
	}

	hrList, err := cl.List(ctx, "helmreleases", client.ListOptions{Limit: 1000})
	if err == nil && len(hrList.Items) > 0 {
		status.Provider = "flux"
		status.Detected = true
		for _, hr := range hrList.Items {
			app := parseFluxHelmRelease(hr)
			status.Applications = append(status.Applications, app)
			updateGitOpsSummary(&status.Summary, app)
		}
	}

	// Try to detect ArgoCD if Flux not found
	if !status.Detected {
		appList, err := cl.List(ctx, "applications", client.ListOptions{Limit: 1000})
		if err == nil && len(appList.Items) > 0 {
			status.Provider = "argocd"
			status.Detected = true
			for _, app := range appList.Items {
				appInfo := parseArgoCDApplication(app)
				status.Applications = append(status.Applications, appInfo)
				updateGitOpsSummary(&status.Summary, appInfo)
			}
		}
	}

	status.Summary.Total = len(status.Applications)
	return status, nil
}

// GetGitOpsApplication returns a specific GitOps application.
func (a *App) GetGitOpsApplication(namespace, name, kind string) (*GitOpsApplicationInfo, error) {
	cl, err := a.nexus.Clusters.Manager().Active()
	if err != nil {
		return nil, fmt.Errorf("no active cluster: %w", err)
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	var resourceKind string
	switch kind {
	case "Kustomization":
		resourceKind = "kustomizations"
	case "HelmRelease":
		resourceKind = "helmreleases"
	case "Application":
		resourceKind = "applications"
	default:
		return nil, fmt.Errorf("unknown GitOps kind: %s", kind)
	}

	resource, err := cl.Get(ctx, resourceKind, namespace, name)
	if err != nil {
		return nil, err
	}

	var app GitOpsApplicationInfo
	switch kind {
	case "Kustomization":
		app = parseFluxKustomization(*resource)
	case "HelmRelease":
		app = parseFluxHelmRelease(*resource)
	case "Application":
		app = parseArgoCDApplication(*resource)
	}

	return &app, nil
}

// SyncGitOpsApplication triggers a sync for a GitOps application.
func (a *App) SyncGitOpsApplication(namespace, name, kind string) error {
	if err := a.checkReadOnly(); err != nil {
		return err
	}
	switch kind {
	case "Kustomization", "HelmRelease":
		return fmt.Errorf("use 'flux reconcile %s %s -n %s' to trigger sync", kind, name, namespace)
	case "Application":
		return fmt.Errorf("use 'argocd app sync %s' to trigger sync", name)
	}
	return fmt.Errorf("unknown GitOps kind: %s", kind)
}

func parseFluxKustomization(r client.Resource) GitOpsApplicationInfo {
	var ks struct {
		Metadata struct {
			Name      string            `json:"name"`
			Namespace string            `json:"namespace"`
			Labels    map[string]string `json:"labels"`
		} `json:"metadata"`
		Spec struct {
			Path      string `json:"path"`
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
				Message string `json:"message"`
			} `json:"conditions"`
			LastAppliedRevision    string `json:"lastAppliedRevision"`
			LastHandledReconcileAt string `json:"lastHandledReconcileAt"`
		} `json:"status"`
	}
	_ = json.Unmarshal(r.Raw, &ks)

	app := GitOpsApplicationInfo{
		Name:      ks.Metadata.Name,
		Namespace: ks.Metadata.Namespace,
		Provider:  "flux",
		Kind:      "Kustomization",
		Labels:    ks.Metadata.Labels,
		Source: GitOpsSource{
			Type:       "git",
			Path:       ks.Spec.Path,
			Repository: ks.Spec.SourceRef.Name,
		},
		SyncStatus:   "Unknown",
		HealthStatus: "Unknown",
		Revision:     ks.Status.LastAppliedRevision,
	}

	if ks.Status.LastHandledReconcileAt != "" {
		app.LastSyncTime = ks.Status.LastHandledReconcileAt
	}

	for _, cond := range ks.Status.Conditions {
		if cond.Type == "Ready" {
			if cond.Status == "True" {
				app.SyncStatus = "Synced"
				app.HealthStatus = "Healthy"
			} else {
				app.SyncStatus = "OutOfSync"
				app.HealthStatus = "Degraded"
				app.Message = cond.Message
			}
		}
	}

	if ks.Spec.Suspend {
		app.HealthStatus = "Suspended"
	}

	return app
}

func parseFluxHelmRelease(r client.Resource) GitOpsApplicationInfo {
	var hr struct {
		Metadata struct {
			Name      string            `json:"name"`
			Namespace string            `json:"namespace"`
			Labels    map[string]string `json:"labels"`
		} `json:"metadata"`
		Spec struct {
			Suspend bool `json:"suspend"`
			Chart   struct {
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
				Message string `json:"message"`
			} `json:"conditions"`
			LastAppliedRevision    string `json:"lastAppliedRevision"`
			LastHandledReconcileAt string `json:"lastHandledReconcileAt"`
		} `json:"status"`
	}
	_ = json.Unmarshal(r.Raw, &hr)

	app := GitOpsApplicationInfo{
		Name:      hr.Metadata.Name,
		Namespace: hr.Metadata.Namespace,
		Provider:  "flux",
		Kind:      "HelmRelease",
		Labels:    hr.Metadata.Labels,
		Source: GitOpsSource{
			Type:       "helm",
			Chart:      hr.Spec.Chart.Spec.Chart,
			Version:    hr.Spec.Chart.Spec.Version,
			Repository: hr.Spec.Chart.Spec.SourceRef.Name,
		},
		SyncStatus:   "Unknown",
		HealthStatus: "Unknown",
		Revision:     hr.Status.LastAppliedRevision,
	}

	if hr.Status.LastHandledReconcileAt != "" {
		app.LastSyncTime = hr.Status.LastHandledReconcileAt
	}

	for _, cond := range hr.Status.Conditions {
		if cond.Type == "Ready" {
			if cond.Status == "True" {
				app.SyncStatus = "Synced"
				app.HealthStatus = "Healthy"
			} else {
				app.SyncStatus = "OutOfSync"
				app.HealthStatus = "Degraded"
				app.Message = cond.Message
			}
		}
	}

	if hr.Spec.Suspend {
		app.HealthStatus = "Suspended"
	}

	return app
}

func parseArgoCDApplication(r client.Resource) GitOpsApplicationInfo {
	var argoApp struct {
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
			} `json:"source"`
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
			ReconciledAt string `json:"reconciledAt"`
		} `json:"status"`
	}
	_ = json.Unmarshal(r.Raw, &argoApp)

	sourceType := "git"
	if argoApp.Spec.Source.Chart != "" {
		sourceType = "helm"
	}

	app := GitOpsApplicationInfo{
		Name:      argoApp.Metadata.Name,
		Namespace: argoApp.Metadata.Namespace,
		Provider:  "argocd",
		Kind:      "Application",
		Labels:    argoApp.Metadata.Labels,
		Source: GitOpsSource{
			Type:     sourceType,
			URL:      argoApp.Spec.Source.RepoURL,
			Path:     argoApp.Spec.Source.Path,
			Branch:   argoApp.Spec.Source.TargetRevision,
			Revision: argoApp.Spec.Source.TargetRevision,
			Chart:    argoApp.Spec.Source.Chart,
		},
		SyncStatus:   argoApp.Status.Sync.Status,
		HealthStatus: argoApp.Status.Health.Status,
		Message:      argoApp.Status.Health.Message,
		Revision:     argoApp.Status.Sync.Revision,
		LastSyncTime: argoApp.Status.ReconciledAt,
	}

	if app.SyncStatus == "" {
		app.SyncStatus = "Unknown"
	}
	if app.HealthStatus == "" {
		app.HealthStatus = "Unknown"
	}

	return app
}

func updateGitOpsSummary(s *GitOpsSummary, app GitOpsApplicationInfo) {
	switch app.SyncStatus {
	case "Synced":
		s.Synced++
	case "OutOfSync":
		s.OutOfSync++
	}
	switch app.HealthStatus {
	case "Healthy":
		s.Healthy++
	case "Degraded":
		s.Degraded++
	case "Progressing":
		s.Progressing++
	}
}
