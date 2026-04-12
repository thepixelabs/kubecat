// Package core provides the shared business logic layer for Kubecat.
// This package is UI-agnostic and can be used by both TUI and GUI frontends.
package core

// Kubecat is the main application service container.
// It provides access to all core services.
type Kubecat struct {
	// Clusters manages cluster connections.
	Clusters *ClusterService

	// Resources provides resource operations.
	Resources *ResourceService

	// Logs provides log streaming.
	Logs *LogService

	// PortForwards manages port forwarding.
	PortForwards *PortForwardService
}

// New creates a new Kubecat service container.
func New() *Kubecat {
	clusters := NewClusterService()

	return &Kubecat{
		Clusters:     clusters,
		Resources:    NewResourceService(clusters),
		Logs:         NewLogService(clusters),
		PortForwards: NewPortForwardService(clusters),
	}
}

// Close cleans up all services.
func (k *Kubecat) Close() error {
	k.Logs.StopStreaming()
	k.PortForwards.StopAll()
	return k.Clusters.Close()
}
