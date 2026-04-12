package gitops

import (
	"context"

	"github.com/thepixelabs/kubecat/internal/client"
)

// Provider defines the interface for GitOps providers.
type Provider interface {
	// Type returns the provider type.
	Type() ProviderType

	// ListApplications returns all GitOps applications.
	ListApplications(ctx context.Context) ([]Application, error)

	// GetApplication returns a specific application by name.
	GetApplication(ctx context.Context, namespace, name string) (*Application, error)

	// GetDrift returns drift information for an application.
	GetDrift(ctx context.Context, namespace, name string) (*Drift, error)

	// Sync triggers a sync for an application.
	Sync(ctx context.Context, namespace, name string) error

	// Suspend suspends reconciliation for an application.
	Suspend(ctx context.Context, namespace, name string) error

	// Resume resumes reconciliation for an application.
	Resume(ctx context.Context, namespace, name string) error
}

// DetectProvider detects which GitOps provider is installed in the cluster.
func DetectProvider(ctx context.Context, cl client.ClusterClient) (Provider, error) {
	// Check for Flux first
	if hasFluxCRDs(ctx, cl) {
		return NewFluxProvider(cl), nil
	}

	// Check for ArgoCD
	if hasArgoCDCRDs(ctx, cl) {
		return NewArgoCDProvider(cl), nil
	}

	return nil, nil
}

// hasFluxCRDs checks if Flux CRDs are installed.
func hasFluxCRDs(ctx context.Context, cl client.ClusterClient) bool {
	// Try to list Kustomizations
	_, err := cl.List(ctx, "kustomizations", client.ListOptions{Limit: 1})
	if err == nil {
		return true
	}

	// Try to list HelmReleases
	_, err = cl.List(ctx, "helmreleases", client.ListOptions{Limit: 1})
	return err == nil
}

// hasArgoCDCRDs checks if ArgoCD CRDs are installed.
func hasArgoCDCRDs(ctx context.Context, cl client.ClusterClient) bool {
	// Try to list ArgoCD Applications
	_, err := cl.List(ctx, "applications", client.ListOptions{Limit: 1})
	return err == nil
}
