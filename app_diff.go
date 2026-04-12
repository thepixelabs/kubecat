package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DiffSource identifies where to fetch resource data
type DiffSource struct {
	Context  string `json:"context"`            // Cluster context name
	Snapshot string `json:"snapshot,omitempty"` // RFC3339 timestamp for historical
	IsLive   bool   `json:"isLive"`             // true = live cluster, false = snapshot
}

// DiffRequest specifies what to compare
type DiffRequest struct {
	Kind      string     `json:"kind"`
	Namespace string     `json:"namespace"`
	Name      string     `json:"name"`
	Left      DiffSource `json:"left"`
	Right     DiffSource `json:"right"`
}

// DiffResult contains the comparison output
type DiffResult struct {
	Request       DiffRequest       `json:"request"`
	LeftYAML      string            `json:"leftYaml"`
	RightYAML     string            `json:"rightYaml"`
	LeftExists    bool              `json:"leftExists"`
	RightExists   bool              `json:"rightExists"`
	Differences   []FieldDifference `json:"differences"`
	FilteredPaths []string          `json:"filteredPaths"` // Paths that were ignored
	ComputedAt    string            `json:"computedAt"`
}

// FieldDifference represents a single changed field
type FieldDifference struct {
	Path       string `json:"path"`       // JSONPath e.g., "spec.replicas"
	LeftValue  string `json:"leftValue"`  // Value in left source (empty if not present)
	RightValue string `json:"rightValue"` // Value in right source (empty if not present)
	Category   string `json:"category"`   // replicas, image, env, limits, config, labels, annotations, other
	Severity   string `json:"severity"`   // info, warning, critical
	ChangeType string `json:"changeType"` // added, removed, modified
}

// ApplyResult contains the result of applying a resource
type ApplyResult struct {
	Success  bool     `json:"success"`
	DryRun   bool     `json:"dryRun"`
	Message  string   `json:"message"`
	Changes  []string `json:"changes"`  // What would change / changed
	Warnings []string `json:"warnings"` // Potential issues
}

// DiffReport for export functionality
type DiffReport struct {
	Format   string `json:"format"`   // "markdown" or "json"
	Content  string `json:"content"`  // The generated report content
	Filename string `json:"filename"` // Suggested filename
}

// managedAnnotations lists annotations to ignore during diff
var managedAnnotations = []string{
	"kubectl.kubernetes.io/last-applied-configuration",
	"deployment.kubernetes.io/revision",
}

// filterManagedFields removes Kubernetes-managed fields from a resource object
func filterManagedFields(obj map[string]interface{}) []string {
	var filteredPaths []string

	// Remove top-level managed fields
	if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
		for _, field := range []string{"resourceVersion", "uid", "creationTimestamp", "generation", "managedFields", "selfLink"} {
			if _, exists := metadata[field]; exists {
				delete(metadata, field)
				filteredPaths = append(filteredPaths, "metadata."+field)
			}
		}

		// Filter managed annotations
		if annotations, ok := metadata["annotations"].(map[string]interface{}); ok {
			for _, ann := range managedAnnotations {
				if _, exists := annotations[ann]; exists {
					delete(annotations, ann)
					filteredPaths = append(filteredPaths, "metadata.annotations."+ann)
				}
			}
			// Remove empty annotations
			if len(annotations) == 0 {
				delete(metadata, "annotations")
			}
		}
	}

	// Remove status (cluster-specific, not config)
	if _, ok := obj["status"]; ok {
		delete(obj, "status")
		filteredPaths = append(filteredPaths, "status")
	}

	return filteredPaths
}

// GetResourceFromContext fetches a resource's YAML from a specific cluster context
func (a *App) GetResourceFromContext(contextName, kind, namespace, name string) (string, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	manager := a.nexus.Clusters.Manager()
	cl, err := manager.Get(contextName)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster %s: %w", contextName, err)
	}

	resource, err := cl.Get(ctx, kind, namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to get resource: %w", err)
	}

	// Parse and filter managed fields
	var obj map[string]interface{}
	if err := json.Unmarshal(resource.Raw, &obj); err != nil {
		return "", fmt.Errorf("failed to parse resource: %w", err)
	}

	filterManagedFields(obj)

	// Convert to YAML
	yamlBytes, err := yaml.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("failed to convert to YAML: %w", err)
	}

	return string(yamlBytes), nil
}

// GetResourceFromSnapshot fetches a resource from a historical snapshot
func (a *App) GetResourceFromSnapshot(contextName, snapshotTimestamp, kind, namespace, name string) (string, error) {
	if a.snapshotter == nil {
		return "", fmt.Errorf("snapshots not available: history database not initialized")
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// Parse the timestamp
	ts, err := time.Parse(time.RFC3339, snapshotTimestamp)
	if err != nil {
		return "", fmt.Errorf("invalid snapshot timestamp: %w", err)
	}

	// Get the snapshot data
	snapshotData, err := a.snapshotter.GetSnapshot(ctx, contextName, ts)
	if err != nil {
		return "", fmt.Errorf("failed to get snapshot: %w", err)
	}

	// Find the resource in the snapshot
	resources, ok := snapshotData.Resources[kind]
	if !ok {
		return "", fmt.Errorf("resource kind %s not found in snapshot", kind)
	}

	for _, r := range resources {
		if r.Namespace == namespace && r.Name == name {
			// We only have minimal info in snapshots, construct a basic YAML
			obj := map[string]interface{}{
				"apiVersion": "v1", // Simplified - snapshots don't store full API version
				"kind":       kind,
				"metadata": map[string]interface{}{
					"name":      r.Name,
					"namespace": r.Namespace,
					"labels":    r.Labels,
				},
				"_snapshotInfo": map[string]interface{}{
					"status":          r.Status,
					"resourceVersion": r.ResourceVersion,
					"snapshotTime":    snapshotData.Timestamp.Format(time.RFC3339),
				},
			}

			yamlBytes, err := yaml.Marshal(obj)
			if err != nil {
				return "", fmt.Errorf("failed to convert to YAML: %w", err)
			}
			return string(yamlBytes), nil
		}
	}

	return "", fmt.Errorf("resource %s/%s not found in snapshot", namespace, name)
}

// ComputeDiff computes the differences between two resource sources
func (a *App) ComputeDiff(req DiffRequest) (*DiffResult, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	result := &DiffResult{
		Request:       req,
		Differences:   []FieldDifference{},
		FilteredPaths: []string{},
		ComputedAt:    time.Now().Format(time.RFC3339),
	}

	// Fetch left resource
	var leftObj map[string]interface{}
	if req.Left.IsLive {
		manager := a.nexus.Clusters.Manager()
		cl, err := manager.Get(req.Left.Context)
		if err != nil {
			// Try to connect to the cluster if not already connected
			if err := manager.Add(ctx, req.Left.Context); err != nil {
				result.LeftExists = false
			} else {
				cl, err = manager.Get(req.Left.Context)
				if err != nil {
					result.LeftExists = false
				}
			}
		}
		if cl != nil {
			resource, err := cl.Get(ctx, req.Kind, req.Namespace, req.Name)
			if err != nil {
				result.LeftExists = false
			} else {
				result.LeftExists = true
				if err := json.Unmarshal(resource.Raw, &leftObj); err == nil {
					result.FilteredPaths = append(result.FilteredPaths, filterManagedFields(leftObj)...)
				}
			}
		}
	} else if req.Left.Snapshot != "" {
		// Historical snapshot - limited data available
		result.LeftExists = false // Snapshots have limited data
	}

	// Fetch right resource
	var rightObj map[string]interface{}
	if req.Right.IsLive {
		manager := a.nexus.Clusters.Manager()
		cl, err := manager.Get(req.Right.Context)
		if err != nil {
			// Try to connect to the cluster if not already connected
			if err := manager.Add(ctx, req.Right.Context); err != nil {
				result.RightExists = false
			} else {
				cl, err = manager.Get(req.Right.Context)
				if err != nil {
					result.RightExists = false
				}
			}
		}
		if cl != nil {
			resource, err := cl.Get(ctx, req.Kind, req.Namespace, req.Name)
			if err != nil {
				result.RightExists = false
			} else {
				result.RightExists = true
				if err := json.Unmarshal(resource.Raw, &rightObj); err == nil {
					filterManagedFields(rightObj)
				}
			}
		}
	} else if req.Right.Snapshot != "" {
		result.RightExists = false // Snapshots have limited data
	}

	// Convert to YAML for display
	if leftObj != nil {
		yamlBytes, _ := yaml.Marshal(leftObj)
		result.LeftYAML = string(yamlBytes)
	}
	if rightObj != nil {
		yamlBytes, _ := yaml.Marshal(rightObj)
		result.RightYAML = string(yamlBytes)
	}

	// Compute field differences
	if result.LeftExists && result.RightExists {
		result.Differences = computeFieldDifferences("", leftObj, rightObj)
	} else if result.LeftExists && !result.RightExists {
		result.Differences = []FieldDifference{{
			Path:       "(root)",
			LeftValue:  "exists",
			RightValue: "not found",
			Category:   "existence",
			Severity:   "critical",
			ChangeType: "removed",
		}}
	} else if !result.LeftExists && result.RightExists {
		result.Differences = []FieldDifference{{
			Path:       "(root)",
			LeftValue:  "not found",
			RightValue: "exists",
			Category:   "existence",
			Severity:   "critical",
			ChangeType: "added",
		}}
	}

	return result, nil
}

// computeFieldDifferences recursively compares two objects and returns differences
func computeFieldDifferences(prefix string, left, right map[string]interface{}) []FieldDifference {
	var diffs []FieldDifference

	// Check all keys in left
	for key, leftVal := range left {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		rightVal, exists := right[key]
		if !exists {
			diffs = append(diffs, FieldDifference{
				Path:       path,
				LeftValue:  formatValue(leftVal),
				RightValue: "",
				Category:   categorizeField(path),
				Severity:   assessSeverity(path, leftVal, nil),
				ChangeType: "removed",
			})
			continue
		}

		// Both exist, compare
		if !valuesEqual(leftVal, rightVal) {
			// Check if both are maps for recursive comparison
			leftMap, leftIsMap := leftVal.(map[string]interface{})
			rightMap, rightIsMap := rightVal.(map[string]interface{})
			if leftIsMap && rightIsMap {
				diffs = append(diffs, computeFieldDifferences(path, leftMap, rightMap)...)
			} else {
				diffs = append(diffs, FieldDifference{
					Path:       path,
					LeftValue:  formatValue(leftVal),
					RightValue: formatValue(rightVal),
					Category:   categorizeField(path),
					Severity:   assessSeverity(path, leftVal, rightVal),
					ChangeType: "modified",
				})
			}
		}
	}

	// Check keys only in right
	for key, rightVal := range right {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		if _, exists := left[key]; !exists {
			diffs = append(diffs, FieldDifference{
				Path:       path,
				LeftValue:  "",
				RightValue: formatValue(rightVal),
				Category:   categorizeField(path),
				Severity:   assessSeverity(path, nil, rightVal),
				ChangeType: "added",
			})
		}
	}

	return diffs
}

// formatValue converts a value to a string representation
func formatValue(val interface{}) string {
	if val == nil {
		return "<nil>"
	}
	switch v := val.(type) {
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case []interface{}:
		if len(v) == 0 {
			return "[]"
		}
		return fmt.Sprintf("[%d items]", len(v))
	case map[string]interface{}:
		return fmt.Sprintf("{%d fields}", len(v))
	default:
		return fmt.Sprintf("%v", v)
	}
}

// valuesEqual checks if two values are equal
func valuesEqual(a, b interface{}) bool {
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}

// categorizeField determines the category of a field path
func categorizeField(path string) string {
	switch {
	case strings.Contains(path, "replicas"):
		return "replicas"
	case strings.Contains(path, "image"):
		return "image"
	case strings.Contains(path, "env"):
		return "env"
	case strings.Contains(path, "limits") || strings.Contains(path, "requests"):
		return "limits"
	case strings.Contains(path, "labels"):
		return "labels"
	case strings.Contains(path, "annotations"):
		return "annotations"
	case strings.HasPrefix(path, "spec.template.spec.containers"):
		return "container"
	case strings.HasPrefix(path, "spec"):
		return "config"
	default:
		return "other"
	}
}

// assessSeverity determines the severity of a field difference
func assessSeverity(path string, leftVal, rightVal interface{}) string {
	category := categorizeField(path)

	switch category {
	case "replicas":
		return "warning"
	case "image":
		return "warning"
	case "limits":
		return "warning"
	case "env":
		return "info"
	case "labels", "annotations":
		return "info"
	default:
		return "info"
	}
}

// ApplyResourceToCluster applies a resource YAML to a target cluster
func (a *App) ApplyResourceToCluster(contextName, kind, namespace, name, yamlContent string, dryRun bool) (*ApplyResult, error) {
	// Dry-run is a simulation only — allow it even in read-only mode.
	// Actual applies that modify cluster state are blocked.
	if !dryRun {
		if err := a.checkReadOnly(); err != nil {
			return &ApplyResult{
				Success:  false,
				DryRun:   dryRun,
				Message:  err.Error(),
				Changes:  []string{},
				Warnings: []string{},
			}, nil
		}
	}

	result := &ApplyResult{
		DryRun:   dryRun,
		Changes:  []string{},
		Warnings: []string{},
	}

	// Parse the YAML to validate
	var obj map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &obj); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Invalid YAML: %v", err)
		return result, nil
	}

	// Add warning for production clusters
	if strings.Contains(strings.ToLower(contextName), "prod") {
		result.Warnings = append(result.Warnings, "This is a production cluster - proceed with caution")
	}

	if dryRun {
		// In dry-run mode, just show what would change
		result.Success = true
		result.Message = "Dry run completed - no changes applied"
		result.Changes = append(result.Changes, fmt.Sprintf("Would apply %s %s/%s to %s", kind, namespace, name, contextName))
		return result, nil
	}

	// For actual apply, use kubectl
	cmd := exec.Command("kubectl", "apply", "-f", "-", "--context", contextName)
	cmd.Stdin = strings.NewReader(yamlContent)
	out, err := cmd.CombinedOutput()

	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Failed to apply resource: %s\nOutput: %s", err, string(out))
		return result, nil
	}

	result.Success = true
	result.Message = fmt.Sprintf("Resource applied successfully to %s", contextName)
	result.Changes = append(result.Changes, fmt.Sprintf("Applied %s %s/%s", kind, namespace, name))

	return result, nil
}

// GenerateDiffReport generates a markdown or JSON report from diff results
func (a *App) GenerateDiffReport(result *DiffResult, format string) (*DiffReport, error) {
	report := &DiffReport{
		Format: format,
	}

	timestamp := time.Now().Format("2006-01-02-150405")
	resourceName := fmt.Sprintf("%s-%s-%s", result.Request.Kind, result.Request.Namespace, result.Request.Name)

	switch format {
	case "json":
		report.Filename = fmt.Sprintf("diff-%s-%s.json", resourceName, timestamp)
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to generate JSON: %w", err)
		}
		report.Content = string(jsonBytes)

	case "markdown":
		fallthrough
	default:
		report.Format = "markdown"
		report.Filename = fmt.Sprintf("diff-%s-%s.md", resourceName, timestamp)
		report.Content = generateMarkdownReport(result)
	}

	return report, nil
}

// generateMarkdownReport creates a markdown diff report
func generateMarkdownReport(result *DiffResult) string {
	var sb strings.Builder

	sb.WriteString("# Cluster Diff Report\n\n")
	sb.WriteString(fmt.Sprintf("**Generated:** %s\n", result.ComputedAt))
	sb.WriteString(fmt.Sprintf("**Resource:** %s/%s/%s\n\n", result.Request.Kind, result.Request.Namespace, result.Request.Name))

	sb.WriteString("## Comparison\n\n")
	sb.WriteString("| Source | Cluster | Type |\n")
	sb.WriteString("|--------|---------|------|\n")

	leftType := "live"
	if !result.Request.Left.IsLive {
		leftType = fmt.Sprintf("snapshot (%s)", result.Request.Left.Snapshot)
	}
	rightType := "live"
	if !result.Request.Right.IsLive {
		rightType = fmt.Sprintf("snapshot (%s)", result.Request.Right.Snapshot)
	}

	sb.WriteString(fmt.Sprintf("| Left | %s | %s |\n", result.Request.Left.Context, leftType))
	sb.WriteString(fmt.Sprintf("| Right | %s | %s |\n\n", result.Request.Right.Context, rightType))

	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Left exists:** %t\n", result.LeftExists))
	sb.WriteString(fmt.Sprintf("- **Right exists:** %t\n", result.RightExists))
	sb.WriteString(fmt.Sprintf("- **Differences found:** %d\n\n", len(result.Differences)))

	if len(result.Differences) > 0 {
		sb.WriteString("## Differences\n\n")
		sb.WriteString("| Field | Left Value | Right Value | Category | Severity |\n")
		sb.WriteString("|-------|------------|-------------|----------|----------|\n")

		for _, diff := range result.Differences {
			leftVal := diff.LeftValue
			if leftVal == "" {
				leftVal = "_(not set)_"
			}
			rightVal := diff.RightValue
			if rightVal == "" {
				rightVal = "_(not set)_"
			}
			sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s |\n",
				diff.Path, leftVal, rightVal, diff.Category, diff.Severity))
		}
		sb.WriteString("\n")
	}

	if result.LeftYAML != "" || result.RightYAML != "" {
		sb.WriteString("## Full YAML Comparison\n\n")

		if result.LeftYAML != "" {
			sb.WriteString(fmt.Sprintf("<details>\n<summary>%s</summary>\n\n```yaml\n%s```\n</details>\n\n",
				result.Request.Left.Context, result.LeftYAML))
		}

		if result.RightYAML != "" {
			sb.WriteString(fmt.Sprintf("<details>\n<summary>%s</summary>\n\n```yaml\n%s```\n</details>\n\n",
				result.Request.Right.Context, result.RightYAML))
		}
	}

	if len(result.FilteredPaths) > 0 {
		sb.WriteString("## Filtered Paths\n\n")
		sb.WriteString("The following Kubernetes-managed fields were ignored:\n\n")
		for _, path := range result.FilteredPaths {
			sb.WriteString(fmt.Sprintf("- `%s`\n", path))
		}
	}

	return sb.String()
}
