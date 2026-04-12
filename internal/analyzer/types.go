// Package analyzer provides a pluggable framework for analyzing Kubernetes resources.
package analyzer

import (
	"context"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
)

// Severity represents the severity level of an issue.
type Severity int

const (
	// SeverityInfo is for informational issues.
	SeverityInfo Severity = iota
	// SeverityWarning is for warning issues that may need attention.
	SeverityWarning
	// SeverityCritical is for critical issues that need immediate attention.
	SeverityCritical
)

// String returns the string representation of severity.
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "Info"
	case SeverityWarning:
		return "Warning"
	case SeverityCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}

// Symbol returns a symbol for the severity.
func (s Severity) Symbol() string {
	switch s {
	case SeverityInfo:
		return "ℹ"
	case SeverityWarning:
		return "⚠"
	case SeverityCritical:
		return "✖"
	default:
		return "?"
	}
}

// Category represents the category of an issue.
type Category string

const (
	// CategoryScheduling is for scheduling-related issues.
	CategoryScheduling Category = "Scheduling"
	// CategoryStorage is for storage-related issues.
	CategoryStorage Category = "Storage"
	// CategoryStuck is for stuck/terminating resources.
	CategoryStuck Category = "Stuck"
	// CategoryNode is for node-related issues.
	CategoryNode Category = "Node"
	// CategoryCRD is for CRD/operator issues.
	CategoryCRD Category = "CRD"
	// CategoryConfig is for configuration issues.
	CategoryConfig Category = "Config"
)

// Issue represents a detected issue with a resource.
type Issue struct {
	// ID is a unique identifier for this issue type.
	ID string
	// Category is the issue category.
	Category Category
	// Severity is the issue severity.
	Severity Severity
	// Title is a short description of the issue.
	Title string
	// Message is a detailed description of the issue.
	Message string
	// Resource is the affected resource.
	Resource client.Resource
	// Details contains additional structured information about the issue.
	Details map[string]interface{}
	// Fixes contains suggested fixes for the issue.
	Fixes []Fix
	// DetectedAt is when the issue was detected.
	DetectedAt time.Time
}

// Fix represents a suggested fix for an issue.
type Fix struct {
	// Description is a human-readable description of the fix.
	Description string
	// YAML is the YAML snippet to apply (if applicable).
	YAML string
	// Command is a kubectl command to run (if applicable).
	Command string
}

// OwnerRef represents an owner reference in a chain.
type OwnerRef struct {
	// Kind is the resource kind.
	Kind string
	// Name is the resource name.
	Name string
	// Namespace is the resource namespace.
	Namespace string
	// APIVersion is the API version.
	APIVersion string
	// UID is the unique identifier.
	UID string
	// Resource is the full resource if loaded.
	Resource *client.Resource
}

// OwnerChain represents the ownership chain for a resource.
type OwnerChain struct {
	// Resource is the resource being analyzed.
	Resource client.Resource
	// Owners is the chain of owner references (from immediate parent to root).
	Owners []OwnerRef
}

// RelatedEvent represents an event related to a resource.
type RelatedEvent struct {
	// Type is the event type (Normal, Warning).
	Type string
	// Reason is the event reason.
	Reason string
	// Message is the event message.
	Message string
	// Count is how many times this event occurred.
	Count int
	// FirstTimestamp is when the event first occurred.
	FirstTimestamp time.Time
	// LastTimestamp is when the event last occurred.
	LastTimestamp time.Time
}

// AnalysisResult contains the full analysis of a resource.
type AnalysisResult struct {
	// Resource is the analyzed resource.
	Resource client.Resource
	// Issues are the detected issues.
	Issues []Issue
	// OwnerChain is the ownership chain.
	OwnerChain *OwnerChain
	// Events are related events.
	Events []RelatedEvent
	// AnalyzedAt is when the analysis was performed.
	AnalyzedAt time.Time
}

// ScanSummary contains a summary of a cluster scan.
type ScanSummary struct {
	// Critical is the count of critical issues.
	Critical int
	// Warning is the count of warning issues.
	Warning int
	// Info is the count of info issues.
	Info int
	// IssuesByCategory groups issues by category.
	IssuesByCategory map[Category][]Issue
	// ScannedResources is the total number of resources scanned.
	ScannedResources int
	// ScannedAt is when the scan was performed.
	ScannedAt time.Time
}

// Analyzer is the interface that all analyzers must implement.
type Analyzer interface {
	// Name returns the analyzer name.
	Name() string
	// Category returns the issue category this analyzer handles.
	Category() Category
	// Analyze analyzes a single resource and returns any issues found.
	Analyze(ctx context.Context, client client.ClusterClient, resource client.Resource) ([]Issue, error)
	// Scan scans all relevant resources and returns issues.
	Scan(ctx context.Context, client client.ClusterClient, namespace string) ([]Issue, error)
}

// AnalysisContext provides context for analysis operations.
type AnalysisContext struct {
	// Client is the cluster client.
	Client client.ClusterClient
	// Namespace limits analysis to a specific namespace (empty for all).
	Namespace string
	// IncludeInfo includes info-level issues in results.
	IncludeInfo bool
}
