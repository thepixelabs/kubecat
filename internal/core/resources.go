package core

import (
	"context"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
)

// ResourceService provides resource operations.
type ResourceService struct {
	clusterService *ClusterService
}

// NewResourceService creates a new resource service.
func NewResourceService(cs *ClusterService) *ResourceService {
	return &ResourceService{
		clusterService: cs,
	}
}

// ListResources lists resources of a given kind.
func (s *ResourceService) ListResources(ctx context.Context, kind, namespace string) ([]client.Resource, error) {
	c, err := s.clusterService.Manager().Active()
	if err != nil {
		return nil, err
	}

	// Add timeout to prevent hanging indefinitely
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	list, err := c.List(timeoutCtx, kind, client.ListOptions{
		Namespace: namespace,
		Limit:     1000,
	})
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

// GetResource gets a single resource.
func (s *ResourceService) GetResource(ctx context.Context, kind, namespace, name string) (*client.Resource, error) {
	c, err := s.clusterService.Manager().Active()
	if err != nil {
		return nil, err
	}

	// Add timeout to prevent hanging indefinitely
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return c.Get(timeoutCtx, kind, namespace, name)
}

// DeleteResource deletes a resource.
func (s *ResourceService) DeleteResource(ctx context.Context, kind, namespace, name string) error {
	c, err := s.clusterService.Manager().Active()
	if err != nil {
		return err
	}

	return c.Delete(ctx, kind, namespace, name)
}

// WatchResources watches for resource changes.
func (s *ResourceService) WatchResources(ctx context.Context, kind, namespace string) (<-chan client.WatchEvent, error) {
	c, err := s.clusterService.Manager().Active()
	if err != nil {
		return nil, err
	}

	return c.Watch(ctx, kind, client.WatchOptions{
		Namespace: namespace,
	})
}

// ResourceInfo provides summary information about a resource.
type ResourceInfo struct {
	Kind      string
	Name      string
	Namespace string
	Status    string
	Age       time.Duration
	Labels    map[string]string
}

// GetResourceInfo returns summary info about a resource.
func (s *ResourceService) GetResourceInfo(r *client.Resource) ResourceInfo {
	return ResourceInfo{
		Kind:      r.Kind,
		Name:      r.Name,
		Namespace: r.Namespace,
		Status:    r.Status,
		Age:       time.Since(r.CreatedAt),
		Labels:    r.Labels,
	}
}
