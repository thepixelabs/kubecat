// SPDX-License-Identifier: Apache-2.0

// Package client provides multi-cluster Kubernetes client management.
package client

import (
	"context"
	"io"
	"time"
)

// ClusterStatus represents the connection status of a cluster.
type ClusterStatus int

const (
	// StatusUnknown indicates the cluster status is not yet determined.
	StatusUnknown ClusterStatus = iota
	// StatusConnected indicates the cluster is reachable.
	StatusConnected
	// StatusDisconnected indicates the cluster is not reachable.
	StatusDisconnected
	// StatusError indicates an error occurred while connecting.
	StatusError
)

// String returns the string representation of the status.
func (s ClusterStatus) String() string {
	switch s {
	case StatusConnected:
		return "Connected"
	case StatusDisconnected:
		return "Disconnected"
	case StatusError:
		return "Error"
	default:
		return "Unknown"
	}
}

// ClusterInfo contains metadata about a connected cluster.
type ClusterInfo struct {
	// Name is the friendly name of the cluster.
	Name string
	// Context is the kubeconfig context name.
	Context string
	// Server is the API server URL.
	Server string
	// Version is the Kubernetes version.
	Version string
	// Status is the current connection status.
	Status ClusterStatus
	// LastCheck is when the cluster was last checked.
	LastCheck time.Time
	// Error contains any error message if Status is StatusError.
	Error string
	// NodeCount is the number of nodes in the cluster.
	NodeCount int
	// PodCount is the total number of pods.
	PodCount int
	// NamespaceCount is the number of namespaces.
	NamespaceCount int
}

// Resource represents a generic Kubernetes resource.
type Resource struct {
	// Kind is the resource kind (e.g., "Pod", "Deployment").
	Kind string
	// APIVersion is the API version (e.g., "v1", "apps/v1").
	APIVersion string
	// Name is the resource name.
	Name string
	// Namespace is the resource namespace (empty for cluster-scoped).
	Namespace string
	// Labels are the resource labels.
	Labels map[string]string
	// Annotations are the resource annotations.
	Annotations map[string]string
	// CreatedAt is when the resource was created.
	CreatedAt time.Time
	// Status is a simplified status string.
	Status string
	// Raw is the raw JSON representation.
	Raw []byte
	// Object is the already-parsed unstructured representation. Populated by
	// unstructuredToResource so downstream consumers (e.g. metadata extraction)
	// can avoid re-unmarshaling Raw. May be nil for resources synthesized
	// outside the cluster client.
	Object map[string]interface{}
}

// ResourceList is a list of resources with metadata.
type ResourceList struct {
	// Items are the resources.
	Items []Resource
	// Total is the total count (may differ from len(Items) if paginated).
	Total int
	// Continue is the pagination token for the next page.
	Continue string
}

// ListOptions configures resource listing.
type ListOptions struct {
	// Namespace filters by namespace (empty for all namespaces).
	Namespace string
	// LabelSelector filters by labels.
	LabelSelector string
	// FieldSelector filters by fields.
	FieldSelector string
	// Limit is the maximum number of items to return.
	Limit int64
	// Continue is the pagination token from a previous request.
	Continue string
}

// WatchEvent represents a change to a resource.
type WatchEvent struct {
	// Type is the event type (Added, Modified, Deleted).
	Type string
	// Resource is the affected resource.
	Resource Resource
}

// WatchOptions configures resource watching.
type WatchOptions struct {
	// Namespace filters by namespace.
	Namespace string
	// LabelSelector filters by labels.
	LabelSelector string
	// ResourceVersion is the version to start watching from.
	ResourceVersion string
}

// ClusterClient is the interface for interacting with a single cluster.
type ClusterClient interface {
	// Info returns information about the cluster.
	Info(ctx context.Context) (*ClusterInfo, error)

	// List lists resources of a given kind.
	List(ctx context.Context, kind string, opts ListOptions) (*ResourceList, error)

	// Get retrieves a single resource.
	Get(ctx context.Context, kind, namespace, name string) (*Resource, error)

	// Delete deletes a resource.
	Delete(ctx context.Context, kind, namespace, name string) error

	// Watch watches for resource changes.
	Watch(ctx context.Context, kind string, opts WatchOptions) (<-chan WatchEvent, error)

	// Logs streams logs from a pod.
	Logs(ctx context.Context, namespace, pod, container string, follow bool, tailLines int64) (<-chan string, error)

	// Exec executes a command in a container.
	Exec(ctx context.Context, namespace, pod, container string, command []string) error

	// PortForward creates a port forward to a pod.
	PortForward(ctx context.Context, namespace, pod string, localPort, remotePort int) (PortForwarder, error)

	// Close closes the client connection.
	Close() error
}

// ExecClient provides extended exec capabilities with I/O control.
type ExecClient interface {
	ClusterClient
	// ExecInteractive executes a command with interactive I/O.
	ExecInteractive(ctx context.Context, namespace, pod, container string, command []string, stdin io.Reader, stdout, stderr io.Writer, tty bool) error
}

// PortForwarder manages a port forwarding session.
type PortForwarder interface {
	// LocalPort returns the local port being forwarded.
	LocalPort() int
	// Stop stops the port forwarding.
	Stop()
	// Done returns a channel that's closed when the port forward ends.
	Done() <-chan struct{}
	// Error returns any error that occurred.
	Error() error
}

// Manager manages multiple cluster clients.
type Manager interface {
	// Add adds a cluster by context name.
	Add(ctx context.Context, contextName string) error

	// Remove removes a cluster.
	Remove(contextName string) error

	// Get returns the client for a cluster.
	Get(contextName string) (ClusterClient, error)

	// Active returns the currently active cluster client.
	Active() (ClusterClient, error)

	// SetActive sets the active cluster.
	SetActive(contextName string) error

	// List returns all managed clusters.
	List() []ClusterInfo

	// Contexts returns available kubeconfig contexts.
	Contexts() ([]string, error)

	// Close closes all clients.
	Close() error

	// ActiveContext returns the name of the active context.
	ActiveContext() string

	// RefreshInfo refreshes cached cluster info for a context.
	RefreshInfo(ctx context.Context, contextName string) error

	// ReloadContexts reloads the kubeconfig and returns available contexts.
	ReloadContexts() ([]string, error)
}
