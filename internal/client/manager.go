package client

import (
	"context"
	"sync"
)

// manager implements Manager for multi-cluster management.
type manager struct {
	mu sync.RWMutex

	// loader handles kubeconfig loading.
	loader *KubeConfigLoader

	// clients maps context names to cluster clients.
	clients map[string]ClusterClient

	// active is the currently active cluster context.
	active string

	// infos caches cluster information.
	infos map[string]*ClusterInfo
}

// NewManager creates a new cluster manager.
func NewManager() (Manager, error) {
	// Ensure common CLI tool paths are in PATH (important for GUI apps)
	EnsureCommonPathsForCLITools()

	loader, err := NewKubeConfigLoader()
	if err != nil {
		return nil, err
	}

	return &manager{
		loader:  loader,
		clients: make(map[string]ClusterClient),
		infos:   make(map[string]*ClusterInfo),
		active:  loader.CurrentContext(),
	}, nil
}

// Add adds a cluster by context name.
func (m *manager) Add(ctx context.Context, contextName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already exists
	if _, exists := m.clients[contextName]; exists {
		return ErrClusterAlreadyExists
	}

	// Get REST config for context
	config, err := m.loader.ClientConfig(contextName)
	if err != nil {
		return FormatConnectionError(err)
	}

	// Create cluster client
	client, err := NewCluster(contextName, config)
	if err != nil {
		return FormatConnectionError(err)
	}

	m.clients[contextName] = client

	// Get initial cluster info to verify connection works
	info, err := client.Info(ctx)
	if err != nil {
		// Connection failed - remove the client and return formatted error
		delete(m.clients, contextName)
		_ = client.Close()
		return FormatConnectionError(err)
	}

	m.infos[contextName] = info

	return nil
}

// Remove removes a cluster.
func (m *manager) Remove(contextName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[contextName]
	if !exists {
		return ErrContextNotFound
	}

	// Close the client
	if err := client.Close(); err != nil {
		return err
	}

	delete(m.clients, contextName)
	delete(m.infos, contextName)

	// If this was the active cluster, clear active
	if m.active == contextName {
		m.active = ""
	}

	return nil
}

// Get returns the client for a cluster.
func (m *manager) Get(contextName string) (ClusterClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[contextName]
	if !exists {
		return nil, ErrContextNotFound
	}

	return client, nil
}

// Active returns the currently active cluster client.
func (m *manager) Active() (ClusterClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.active == "" {
		return nil, ErrNoActiveCluster
	}

	client, exists := m.clients[m.active]
	if !exists {
		return nil, ErrNoActiveCluster
	}

	return client, nil
}

// SetActive sets the active cluster.
func (m *manager) SetActive(contextName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[contextName]; !exists {
		return ErrContextNotFound
	}

	m.active = contextName
	return nil
}

// List returns all managed clusters.
func (m *manager) List() []ClusterInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Include all connected clients (even if Info() hasn't succeeded yet).
	infos := make([]ClusterInfo, 0, len(m.clients))
	for contextName := range m.clients {
		if info, ok := m.infos[contextName]; ok && info != nil {
			infos = append(infos, *info)
			continue
		}
		infos = append(infos, ClusterInfo{
			Name:    contextName,
			Context: contextName,
			Status:  StatusUnknown,
		})
	}
	return infos
}

// Contexts returns available kubeconfig contexts.
func (m *manager) Contexts() ([]string, error) {
	return m.loader.Contexts(), nil
}

// Close closes all clients.
func (m *manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for _, client := range m.clients {
		if err := client.Close(); err != nil {
			lastErr = err
		}
	}

	m.clients = make(map[string]ClusterClient)
	m.infos = make(map[string]*ClusterInfo)
	m.active = ""

	return lastErr
}

// ActiveContext returns the name of the active context.
func (m *manager) ActiveContext() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

// RefreshInfo refreshes cluster info for a context.
func (m *manager) RefreshInfo(ctx context.Context, contextName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[contextName]
	if !exists {
		return ErrContextNotFound
	}

	info, err := client.Info(ctx)
	if err != nil {
		return err
	}

	m.infos[contextName] = info
	return nil
}

// ReloadContexts reloads the kubeconfig and returns available contexts.
func (m *manager) ReloadContexts() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.loader.Reload(); err != nil {
		return nil, err
	}

	return m.loader.Contexts(), nil
}
