// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/thepixelabs/kubecat/internal/analyzer"
)

// AnalyzerIssue is a JSON-friendly analyzer issue.
type AnalyzerIssue struct {
	ID         string                 `json:"id"`
	Category   string                 `json:"category"`
	Severity   string                 `json:"severity"`
	Title      string                 `json:"title"`
	Message    string                 `json:"message"`
	Resource   string                 `json:"resource"`
	Namespace  string                 `json:"namespace"`
	Kind       string                 `json:"kind"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Fixes      []AnalyzerFix          `json:"fixes,omitempty"`
	DetectedAt string                 `json:"detectedAt"`
}

// AnalyzerFix is a JSON-friendly fix suggestion.
type AnalyzerFix struct {
	Description string `json:"description"`
	YAML        string `json:"yaml,omitempty"`
	Command     string `json:"command,omitempty"`
}

// AnalyzerSummary is a JSON-friendly scan summary.
type AnalyzerSummary struct {
	Critical         int                        `json:"critical"`
	Warning          int                        `json:"warning"`
	Info             int                        `json:"info"`
	IssuesByCategory map[string][]AnalyzerIssue `json:"issuesByCategory"`
	ScannedAt        string                     `json:"scannedAt"`
}

// GetResourceYAML returns the full YAML representation of a resource.
func (a *App) GetResourceYAML(kind, namespace, name string) (string, error) {
	cl, err := a.nexus.Clusters.Manager().Active()
	if err != nil {
		return "", fmt.Errorf("no active cluster: %w", err)
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	resource, err := cl.Get(ctx, kind, namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to get resource: %w", err)
	}

	// Convert to YAML
	var obj map[string]interface{}
	if err := json.Unmarshal(resource.Raw, &obj); err != nil {
		return "", fmt.Errorf("failed to parse resource: %w", err)
	}

	yamlBytes, err := yaml.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("failed to convert to YAML: %w", err)
	}

	return string(yamlBytes), nil
}

// ScanCluster runs all analyzers and returns a summary of issues.
func (a *App) ScanCluster(namespace string) (*AnalyzerSummary, error) {
	cl, err := a.nexus.Clusters.Manager().Active()
	if err != nil {
		return nil, fmt.Errorf("no active cluster: %w", err)
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	summary, err := analyzer.DefaultRegistry.Scan(ctx, cl, namespace)
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	result := &AnalyzerSummary{
		Critical:         summary.Critical,
		Warning:          summary.Warning,
		Info:             summary.Info,
		IssuesByCategory: make(map[string][]AnalyzerIssue),
		ScannedAt:        summary.ScannedAt.Format(time.RFC3339),
	}

	for cat, issues := range summary.IssuesByCategory {
		catIssues := make([]AnalyzerIssue, len(issues))
		for i, issue := range issues {
			catIssues[i] = convertIssue(issue)
		}
		result.IssuesByCategory[string(cat)] = catIssues
	}

	return result, nil
}

// AnalyzeResource analyzes a specific resource for issues.
func (a *App) AnalyzeResource(kind, namespace, name string) ([]AnalyzerIssue, error) {
	cl, err := a.nexus.Clusters.Manager().Active()
	if err != nil {
		return nil, fmt.Errorf("no active cluster: %w", err)
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	resource, err := cl.Get(ctx, kind, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}

	analysisResult, err := analyzer.DefaultRegistry.Analyze(ctx, cl, *resource)
	if err != nil {
		return nil, fmt.Errorf("analysis failed: %w", err)
	}

	issues := make([]AnalyzerIssue, len(analysisResult.Issues))
	for i, issue := range analysisResult.Issues {
		issues[i] = convertIssue(issue)
	}

	return issues, nil
}

func convertIssue(issue analyzer.Issue) AnalyzerIssue {
	fixes := make([]AnalyzerFix, len(issue.Fixes))
	for i, fix := range issue.Fixes {
		fixes[i] = AnalyzerFix{
			Description: fix.Description,
			YAML:        fix.YAML,
			Command:     fix.Command,
		}
	}

	return AnalyzerIssue{
		ID:         issue.ID,
		Category:   string(issue.Category),
		Severity:   issue.Severity.String(),
		Title:      issue.Title,
		Message:    issue.Message,
		Resource:   issue.Resource.Name,
		Namespace:  issue.Resource.Namespace,
		Kind:       issue.Resource.Kind,
		Details:    issue.Details,
		Fixes:      fixes,
		DetectedAt: issue.DetectedAt.Format(time.RFC3339),
	}
}
