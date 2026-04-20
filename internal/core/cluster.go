// SPDX-License-Identifier: Apache-2.0

// Package core provides shared business logic for both TUI and GUI frontends.
// This package is designed to be UI-agnostic and can be used by any frontend.
package core

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
)

var ErrManagerNotInitialized = errors.New("cluster manager not initialized")

// ClusterService manages cluster operations.
type ClusterService struct {
	manager client.Manager
	mu      sync.RWMutex

	// Cached state
	activeContext  string
	cachedContexts []string
	contextsCached time.Time
	cacheExpiry    time.Duration
}

// NewClusterService creates a new cluster service.
func NewClusterService() *ClusterService {
	manager, err := client.NewManager()
	if err != nil {
		// Log error but don't fail - allow app to start without cluster connection
		// Users can still view help, settings, etc.
		return &ClusterService{
			cacheExpiry: 5 * time.Minute,
		}
	}
	return &ClusterService{
		manager:     manager,
		cacheExpiry: 5 * time.Minute,
	}
}

// NewClusterServiceWithManager constructs a ClusterService backed by a
// caller-supplied Manager. Intended for tests that inject a fake Manager;
// production code should use NewClusterService.
func NewClusterServiceWithManager(mgr client.Manager) *ClusterService {
	s := &ClusterService{
		manager:     mgr,
		cacheExpiry: 5 * time.Minute,
	}
	if mgr != nil {
		s.activeContext = mgr.ActiveContext()
	}
	return s
}

// Manager returns the underlying cluster manager.
func (s *ClusterService) Manager() client.Manager {
	return s.manager
}

// GetContexts returns available kubeconfig contexts.
func (s *ClusterService) GetContexts(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	if time.Since(s.contextsCached) < s.cacheExpiry && len(s.cachedContexts) > 0 {
		contexts := s.cachedContexts
		s.mu.RUnlock()
		return contexts, nil
	}
	s.mu.RUnlock()

	if s.manager == nil {
		return nil, ErrManagerNotInitialized
	}

	contexts, err := s.manager.Contexts()
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cachedContexts = contexts
	s.contextsCached = time.Now()
	s.mu.Unlock()

	return contexts, nil
}

// RefreshContexts reloads the kubeconfig and returns available contexts.
func (s *ClusterService) RefreshContexts(ctx context.Context) ([]string, error) {
	if s.manager == nil {
		return nil, ErrManagerNotInitialized
	}

	contexts, err := s.manager.ReloadContexts()
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cachedContexts = contexts
	s.contextsCached = time.Now()
	s.mu.Unlock()

	return contexts, nil
}

// Connect connects to a cluster by context name.
func (s *ClusterService) Connect(ctx context.Context, contextName string) error {
	if s.manager == nil {
		return ErrManagerNotInitialized
	}

	if err := s.manager.Add(ctx, contextName); err != nil {
		if !errors.Is(err, client.ErrClusterAlreadyExists) {
			return err
		}
	}
	if err := s.manager.SetActive(contextName); err != nil {
		return err
	}

	s.mu.Lock()
	s.activeContext = contextName
	s.mu.Unlock()

	return nil
}

// Disconnect disconnects from a cluster.
func (s *ClusterService) Disconnect(contextName string) error {
	if s.manager == nil {
		return ErrManagerNotInitialized
	}
	return s.manager.Remove(contextName)
}

// ActiveContext returns the currently active context name.
func (s *ClusterService) ActiveContext() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeContext
}

// IsConnected returns true if connected to any cluster.
func (s *ClusterService) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Must have both a manager and an active context
	return s.manager != nil && s.activeContext != ""
}

// GetClusterInfo returns info about the active cluster.
func (s *ClusterService) GetClusterInfo(ctx context.Context) (*client.ClusterInfo, error) {
	if s.manager == nil {
		return nil, ErrManagerNotInitialized
	}
	c, err := s.manager.Active()
	if err != nil {
		return nil, err
	}
	return c.Info(ctx)
}

// Close closes all connections.
func (s *ClusterService) Close() error {
	if s.manager == nil {
		return nil
	}
	return s.manager.Close()
}
