// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// filterManagedFields
// ---------------------------------------------------------------------------

func TestFilterManagedFields_RemovesManagedMetadataFields(t *testing.T) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":              "app",
			"namespace":         "default",
			"resourceVersion":   "12345",
			"uid":               "uid-abc",
			"creationTimestamp": "2020-01-01T00:00:00Z",
			"generation":        float64(7),
			"managedFields":     []interface{}{},
			"selfLink":          "/api/v1/whatever",
		},
		"spec":   map[string]interface{}{"replicas": float64(3)},
		"status": map[string]interface{}{"readyReplicas": float64(3)},
	}

	filtered := filterManagedFields(obj)

	// All managed fields must be gone.
	meta := obj["metadata"].(map[string]interface{})
	for _, f := range []string{"resourceVersion", "uid", "creationTimestamp", "generation", "managedFields", "selfLink"} {
		if _, ok := meta[f]; ok {
			t.Errorf("metadata.%s not removed", f)
		}
	}
	// status should be removed entirely
	if _, ok := obj["status"]; ok {
		t.Error("status not removed")
	}
	// name and namespace must remain — they're load-bearing.
	if meta["name"] != "app" || meta["namespace"] != "default" {
		t.Errorf("name/namespace dropped: %+v", meta)
	}
	// filteredPaths must include status + every metadata field removed.
	pathSet := make(map[string]bool, len(filtered))
	for _, p := range filtered {
		pathSet[p] = true
	}
	if !pathSet["status"] {
		t.Error("status should appear in filteredPaths")
	}
	if !pathSet["metadata.resourceVersion"] {
		t.Error("metadata.resourceVersion should appear in filteredPaths")
	}
}

func TestFilterManagedFields_DropsManagedAnnotations(t *testing.T) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"kubectl.kubernetes.io/last-applied-configuration": "...",
				"deployment.kubernetes.io/revision":                "2",
				"app.kubernetes.io/version":                        "1.0",
			},
		},
	}

	filtered := filterManagedFields(obj)
	annotations := obj["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})
	if _, ok := annotations["kubectl.kubernetes.io/last-applied-configuration"]; ok {
		t.Error("last-applied-configuration annotation not removed")
	}
	if _, ok := annotations["deployment.kubernetes.io/revision"]; ok {
		t.Error("revision annotation not removed")
	}
	// Non-managed annotation must remain.
	if annotations["app.kubernetes.io/version"] != "1.0" {
		t.Error("non-managed annotation was removed")
	}

	// Check that the filtered path list includes the managed annotations.
	found := false
	for _, p := range filtered {
		if strings.Contains(p, "last-applied-configuration") {
			found = true
		}
	}
	if !found {
		t.Error("filtered paths should include removed annotation")
	}
}

func TestFilterManagedFields_EmptyAnnotationsPurged(t *testing.T) {
	// After removing the only managed annotation, the parent annotations map
	// should be purged entirely.
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"kubectl.kubernetes.io/last-applied-configuration": "...",
			},
		},
	}
	_ = filterManagedFields(obj)
	meta := obj["metadata"].(map[string]interface{})
	if _, ok := meta["annotations"]; ok {
		t.Error("empty annotations map should be removed after purging managed entries")
	}
}

// ---------------------------------------------------------------------------
// computeFieldDifferences — the core diff walk
// ---------------------------------------------------------------------------

func TestComputeFieldDifferences_IdenticalObjects_EmptyDiff(t *testing.T) {
	o := map[string]interface{}{
		"a": "1",
		"b": map[string]interface{}{"c": float64(2)},
	}
	// Round-trip through JSON so the comparison values are identical types.
	b, _ := json.Marshal(o)
	var l, r map[string]interface{}
	_ = json.Unmarshal(b, &l)
	_ = json.Unmarshal(b, &r)

	diffs := computeFieldDifferences("", l, r)
	if len(diffs) != 0 {
		t.Errorf("identical objects should produce empty diff, got %+v", diffs)
	}
}

func TestComputeFieldDifferences_AddedAndRemoved(t *testing.T) {
	left := map[string]interface{}{"a": "1", "only_left": "x"}
	right := map[string]interface{}{"a": "1", "only_right": "y"}

	diffs := computeFieldDifferences("", left, right)
	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d: %+v", len(diffs), diffs)
	}
	types := make(map[string]string) // path -> changeType
	for _, d := range diffs {
		types[d.Path] = d.ChangeType
	}
	if types["only_left"] != "removed" {
		t.Errorf("only_left changeType = %q, want removed", types["only_left"])
	}
	if types["only_right"] != "added" {
		t.Errorf("only_right changeType = %q, want added", types["only_right"])
	}
}

func TestComputeFieldDifferences_NestedModification(t *testing.T) {
	left := map[string]interface{}{
		"spec": map[string]interface{}{"replicas": float64(3)},
	}
	right := map[string]interface{}{
		"spec": map[string]interface{}{"replicas": float64(5)},
	}

	diffs := computeFieldDifferences("", left, right)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 nested diff, got %d: %+v", len(diffs), diffs)
	}
	d := diffs[0]
	if d.Path != "spec.replicas" {
		t.Errorf("path = %q, want spec.replicas", d.Path)
	}
	if d.ChangeType != "modified" {
		t.Errorf("changeType = %q, want modified", d.ChangeType)
	}
	if d.Category != "replicas" {
		t.Errorf("category = %q, want replicas", d.Category)
	}
	if d.Severity != "warning" {
		t.Errorf("severity = %q, want warning", d.Severity)
	}
}

// ---------------------------------------------------------------------------
// formatValue
// ---------------------------------------------------------------------------

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want string
	}{
		{"nil", nil, "<nil>"},
		{"string", "hello", "hello"},
		{"whole_float", float64(5), "5"},
		{"bool_true", true, "true"},
		{"empty_slice", []interface{}{}, "[]"},
		{"slice_items", []interface{}{1, 2, 3}, "[3 items]"},
		{"map", map[string]interface{}{"a": 1, "b": 2}, "{2 fields}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatValue(tt.in); got != tt.want {
				t.Errorf("formatValue(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// categorizeField + assessSeverity
// ---------------------------------------------------------------------------

func TestCategorizeField(t *testing.T) {
	tests := []struct {
		path, want string
	}{
		{"spec.replicas", "replicas"},
		// NB: the implementation checks keywords in order, so "image" wins
		// over "container" even for a container.image path. Pin the current
		// behavior so a reorder doesn't sneak through.
		{"spec.template.spec.containers[0].image", "image"},
		{"spec.template.spec.containers[0].env", "env"},
		{"spec.template.spec.containers[0].resources.limits.cpu", "limits"},
		{"metadata.labels.app", "labels"},
		{"metadata.annotations.foo", "annotations"},
		{"spec.template.spec.containers[0]", "container"},
		{"spec.serviceAccountName", "config"},
		{"other.thing", "other"},
	}
	for _, tt := range tests {
		if got := categorizeField(tt.path); got != tt.want {
			t.Errorf("categorizeField(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestAssessSeverity_SurfacesWarningsForRiskyPaths(t *testing.T) {
	warningPaths := []string{"spec.replicas", "containers.image", "spec.resources.limits"}
	for _, p := range warningPaths {
		if got := assessSeverity(p, "a", "b"); got != "warning" {
			t.Errorf("assessSeverity(%q) = %q, want warning", p, got)
		}
	}

	// labels/annotations/env are informational.
	infoPaths := []string{"metadata.labels.app", "metadata.annotations.foo", "containers.env"}
	for _, p := range infoPaths {
		if got := assessSeverity(p, "a", "b"); got != "info" {
			t.Errorf("assessSeverity(%q) = %q, want info", p, got)
		}
	}
}

// ---------------------------------------------------------------------------
// valuesEqual
// ---------------------------------------------------------------------------

func TestValuesEqual(t *testing.T) {
	if !valuesEqual("x", "x") {
		t.Error("equal strings should be equal")
	}
	if valuesEqual("x", "y") {
		t.Error("different strings should not be equal")
	}
	// Equal maps
	if !valuesEqual(
		map[string]interface{}{"a": float64(1)},
		map[string]interface{}{"a": float64(1)},
	) {
		t.Error("equal maps should be equal")
	}
}

// ---------------------------------------------------------------------------
// ApplyResourceToCluster — naive "prod" detection + YAML validation + dry-run
// ---------------------------------------------------------------------------

func TestApplyResourceToCluster_DryRun_ValidYAML_ReportsWouldApply(t *testing.T) {
	a := &App{}
	res, err := a.ApplyResourceToCluster(
		"my-dev-cluster", "Deployment", "ns", "app",
		"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: app\n",
		true, // dry-run
	)
	if err != nil {
		t.Fatalf("ApplyResourceToCluster: %v", err)
	}
	if !res.Success {
		t.Errorf("dry-run with valid YAML should succeed, got %+v", res)
	}
	if !res.DryRun {
		t.Error("DryRun flag not propagated to result")
	}
	if len(res.Changes) == 0 {
		t.Error("dry-run should report what it would apply")
	}
	if len(res.Warnings) != 0 {
		t.Errorf("non-prod context should emit no warnings, got %v", res.Warnings)
	}
}

func TestApplyResourceToCluster_DryRun_ProductionContext_EmitsWarning(t *testing.T) {
	// This pins the current naive "prod substring" safety rail so a future
	// replacement (env-tag based) doesn't silently lose the warning.
	a := &App{}
	res, err := a.ApplyResourceToCluster(
		"gke-production-us", "Deployment", "ns", "app",
		"kind: Deployment\n",
		true,
	)
	if err != nil {
		t.Fatalf("ApplyResourceToCluster: %v", err)
	}
	if !res.Success {
		t.Fatalf("dry-run must succeed, got %+v", res)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("prod context should emit at least one warning")
	}
	if !strings.Contains(strings.ToLower(res.Warnings[0]), "prod") {
		t.Errorf("warning should mention prod, got %q", res.Warnings[0])
	}
}

func TestApplyResourceToCluster_NonProdNames_DoNotEmitWarning(t *testing.T) {
	// Pin the substring behavior to document its limits. "dev", "stg", etc.
	// must NOT be flagged.
	nonProd := []string{"dev", "staging", "kind-local", "minikube"}
	a := &App{}
	for _, name := range nonProd {
		res, _ := a.ApplyResourceToCluster(name, "Deployment", "ns", "app", "kind: Deployment\n", true)
		if len(res.Warnings) != 0 {
			t.Errorf("context %q should not trigger prod warning, got %+v", name, res.Warnings)
		}
	}
}

func TestApplyResourceToCluster_InvalidYAML_ReportsFailure(t *testing.T) {
	a := &App{}
	res, err := a.ApplyResourceToCluster(
		"dev", "Deployment", "ns", "app",
		"::::not yaml",
		true,
	)
	if err != nil {
		t.Fatalf("ApplyResourceToCluster: %v", err)
	}
	if res.Success {
		t.Error("invalid YAML should not succeed")
	}
	if !strings.Contains(strings.ToLower(res.Message), "yaml") {
		t.Errorf("message should mention YAML, got %q", res.Message)
	}
}

// ---------------------------------------------------------------------------
// GenerateDiffReport
// ---------------------------------------------------------------------------

func TestGenerateDiffReport_JSON(t *testing.T) {
	a := &App{}
	in := &DiffResult{
		Request:     DiffRequest{Kind: "Deployment", Namespace: "ns", Name: "app"},
		LeftExists:  true,
		RightExists: true,
		ComputedAt:  "2024-01-01T00:00:00Z",
	}
	report, err := a.GenerateDiffReport(in, "json")
	if err != nil {
		t.Fatalf("GenerateDiffReport: %v", err)
	}
	if report.Format != "json" {
		t.Errorf("format = %q, want json", report.Format)
	}
	if !strings.HasSuffix(report.Filename, ".json") {
		t.Errorf("filename = %q, want *.json", report.Filename)
	}
	var parsed DiffResult
	if err := json.Unmarshal([]byte(report.Content), &parsed); err != nil {
		t.Errorf("generated JSON must round-trip, got: %v\n%s", err, report.Content)
	}
}

func TestGenerateDiffReport_MarkdownDefault(t *testing.T) {
	a := &App{}
	in := &DiffResult{
		Request: DiffRequest{
			Kind: "Deployment", Namespace: "ns", Name: "app",
			Left:  DiffSource{Context: "dev", IsLive: true},
			Right: DiffSource{Context: "prod", IsLive: true},
		},
		LeftExists:  true,
		RightExists: true,
		Differences: []FieldDifference{{Path: "spec.replicas", LeftValue: "3", RightValue: "5", Category: "replicas", Severity: "warning"}},
		ComputedAt:  "2024-01-01T00:00:00Z",
	}
	report, err := a.GenerateDiffReport(in, "")
	if err != nil {
		t.Fatalf("GenerateDiffReport: %v", err)
	}
	if report.Format != "markdown" {
		t.Errorf("format = %q, want markdown", report.Format)
	}
	if !strings.HasSuffix(report.Filename, ".md") {
		t.Errorf("filename = %q, want *.md", report.Filename)
	}
	if !strings.Contains(report.Content, "spec.replicas") {
		t.Error("markdown should mention the changed path")
	}
	if !strings.Contains(report.Content, "# Cluster Diff Report") {
		t.Error("markdown should have a report heading")
	}
}
