// Package diff provides utilities for computing and reporting differences
// between Kubernetes resource snapshots.
package diff

// FieldChange describes a single changed field between two resource versions.
type FieldChange struct {
	// Path is the dot-separated JSON path, e.g. "spec.replicas".
	Path     string
	OldValue interface{}
	NewValue interface{}
	Severity ChangeSeverity
}

// ChangeSeverity classifies the impact of a field change.
type ChangeSeverity string

const (
	SeverityInfo     ChangeSeverity = "info"
	SeverityWarning  ChangeSeverity = "warning"
	SeverityCritical ChangeSeverity = "critical"
)

// DiffResult holds all field changes between two resource snapshots.
type DiffResult struct {
	Kind      string
	Name      string
	Namespace string
	Changes   []FieldChange
}
