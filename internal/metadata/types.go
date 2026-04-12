// Package metadata provides helpers for extracting and formatting structured
// metadata from raw Kubernetes resource JSON.
package metadata

// ResourceMetadata holds distilled metadata for a single Kubernetes resource.
type ResourceMetadata struct {
	Kind      string
	Name      string
	Namespace string

	// Status is a simplified human-readable status string.
	Status string

	// Labels are the resource labels.
	Labels map[string]string

	// Annotations are the resource annotations.
	Annotations map[string]string

	// Pod-specific fields.
	PodPhase        string
	PodIP           string
	NodeName        string
	RestartCount    int
	ReadyContainers string // "2/3"

	// Deployment/StatefulSet/DaemonSet fields.
	Replicas          int32
	ReadyReplicas     int32
	AvailableReplicas int32
	UpdatedReplicas   int32

	// Service fields.
	ServiceType string
	ClusterIP   string
	ExternalIPs []string
	Ports       []string

	// PVC fields.
	StorageClass string
	Capacity     string
	AccessModes  []string

	// Ingress fields.
	IngressClass string
	Hosts        []string
	TLSHosts     []string
	Backends     []string

	// Owner reference.
	OwnerKind string
	OwnerName string

	// Security.
	SecurityIssues []string

	// Raw extra fields not covered above.
	Extra map[string]string
}
