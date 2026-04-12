package analyzer

import (
	"context"
	"sync"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
)

// Registry manages registered analyzers and coordinates analysis.
type Registry struct {
	mu        sync.RWMutex
	analyzers []Analyzer
}

// NewRegistry creates a new analyzer registry.
func NewRegistry() *Registry {
	return &Registry{
		analyzers: make([]Analyzer, 0),
	}
}

// Register registers an analyzer with the registry.
func (r *Registry) Register(a Analyzer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.analyzers = append(r.analyzers, a)
}

// Analyzers returns all registered analyzers.
func (r *Registry) Analyzers() []Analyzer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Analyzer, len(r.analyzers))
	copy(result, r.analyzers)
	return result
}

// AnalyzersByCategory returns analyzers for a specific category.
func (r *Registry) AnalyzersByCategory(cat Category) []Analyzer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Analyzer
	for _, a := range r.analyzers {
		if a.Category() == cat {
			result = append(result, a)
		}
	}
	return result
}

// Analyze runs all applicable analyzers on a resource.
func (r *Registry) Analyze(ctx context.Context, cl client.ClusterClient, resource client.Resource) (*AnalysisResult, error) {
	r.mu.RLock()
	analyzers := make([]Analyzer, len(r.analyzers))
	copy(analyzers, r.analyzers)
	r.mu.RUnlock()

	result := &AnalysisResult{
		Resource:   resource,
		Issues:     make([]Issue, 0),
		AnalyzedAt: time.Now(),
	}

	for _, a := range analyzers {
		issues, err := a.Analyze(ctx, cl, resource)
		if err != nil {
			// Log error but continue with other analyzers
			continue
		}
		result.Issues = append(result.Issues, issues...)
	}

	return result, nil
}

// Scan runs all analyzers to scan for issues across the cluster.
func (r *Registry) Scan(ctx context.Context, cl client.ClusterClient, namespace string) (*ScanSummary, error) {
	r.mu.RLock()
	analyzers := make([]Analyzer, len(r.analyzers))
	copy(analyzers, r.analyzers)
	r.mu.RUnlock()

	summary := &ScanSummary{
		IssuesByCategory: make(map[Category][]Issue),
		ScannedAt:        time.Now(),
	}

	for _, a := range analyzers {
		issues, err := a.Scan(ctx, cl, namespace)
		if err != nil {
			// Log error but continue with other analyzers
			continue
		}

		for _, issue := range issues {
			// Count by severity
			switch issue.Severity {
			case SeverityCritical:
				summary.Critical++
			case SeverityWarning:
				summary.Warning++
			case SeverityInfo:
				summary.Info++
			}

			// Group by category
			summary.IssuesByCategory[issue.Category] = append(
				summary.IssuesByCategory[issue.Category],
				issue,
			)
		}
	}

	return summary, nil
}

// ScanCategory runs analyzers for a specific category.
func (r *Registry) ScanCategory(ctx context.Context, cl client.ClusterClient, category Category, namespace string) ([]Issue, error) {
	analyzers := r.AnalyzersByCategory(category)

	var allIssues []Issue
	for _, a := range analyzers {
		issues, err := a.Scan(ctx, cl, namespace)
		if err != nil {
			continue
		}
		allIssues = append(allIssues, issues...)
	}

	return allIssues, nil
}

// DefaultRegistry is the default global registry.
var DefaultRegistry = NewRegistry()

// Register registers an analyzer with the default registry.
func Register(a Analyzer) {
	DefaultRegistry.Register(a)
}
