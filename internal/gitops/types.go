package gitops

import (
	"time"
)

// ProviderType represents the type of GitOps provider.
type ProviderType string

const (
	ProviderFlux   ProviderType = "flux"
	ProviderArgoCD ProviderType = "argocd"
	ProviderNone   ProviderType = "none"
)

// SyncStatus represents the sync state of an application.
type SyncStatus string

const (
	SyncStatusSynced      SyncStatus = "Synced"
	SyncStatusOutOfSync   SyncStatus = "OutOfSync"
	SyncStatusUnknown     SyncStatus = "Unknown"
	SyncStatusProgressing SyncStatus = "Progressing"
)

// HealthStatus represents the health state of an application.
type HealthStatus string

const (
	HealthStatusHealthy     HealthStatus = "Healthy"
	HealthStatusDegraded    HealthStatus = "Degraded"
	HealthStatusProgressing HealthStatus = "Progressing"
	HealthStatusSuspended   HealthStatus = "Suspended"
	HealthStatusMissing     HealthStatus = "Missing"
	HealthStatusUnknown     HealthStatus = "Unknown"
)

// Application represents a GitOps application (Flux Kustomization/HelmRelease or ArgoCD Application).
type Application struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Provider     ProviderType      `json:"provider"`
	Kind         string            `json:"kind"` // Kustomization, HelmRelease, Application
	Source       Source            `json:"source"`
	SyncStatus   SyncStatus        `json:"syncStatus"`
	HealthStatus HealthStatus      `json:"healthStatus"`
	Message      string            `json:"message,omitempty"`
	LastSyncTime *time.Time        `json:"lastSyncTime,omitempty"`
	Revision     string            `json:"revision,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

// Source represents the source configuration for a GitOps application.
type Source struct {
	Type       string `json:"type"` // git, helm
	URL        string `json:"url"`
	Path       string `json:"path,omitempty"`
	Branch     string `json:"branch,omitempty"`
	Revision   string `json:"revision,omitempty"`
	Chart      string `json:"chart,omitempty"`
	Version    string `json:"version,omitempty"`
	Repository string `json:"repository,omitempty"`
}

// Drift represents differences between desired and live state.
type Drift struct {
	Application string       `json:"application"`
	HasDrift    bool         `json:"hasDrift"`
	Resources   []DriftEntry `json:"resources"`
}

// DriftEntry represents a single resource difference.
type DriftEntry struct {
	Kind      string   `json:"kind"`
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	Status    string   `json:"status"` // added, removed, modified
	Changes   []Change `json:"changes,omitempty"`
}

// Change represents a specific field change.
type Change struct {
	Path     string `json:"path"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

// Commit represents a Git commit.
type Commit struct {
	SHA       string    `json:"sha"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	Timestamp time.Time `json:"timestamp"`
}

// GitOpsStatus represents the overall GitOps status for a cluster.
type GitOpsStatus struct {
	Provider     ProviderType  `json:"provider"`
	Detected     bool          `json:"detected"`
	Applications []Application `json:"applications"`
	Summary      StatusSummary `json:"summary"`
}

// StatusSummary provides counts of applications by status.
type StatusSummary struct {
	Total       int `json:"total"`
	Synced      int `json:"synced"`
	OutOfSync   int `json:"outOfSync"`
	Healthy     int `json:"healthy"`
	Degraded    int `json:"degraded"`
	Progressing int `json:"progressing"`
}
