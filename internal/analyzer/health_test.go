// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thepixelabs/kubecat/internal/client"
)

// -----------------------------------------------------------------------------
// HealthAnalyzer — pod-level issues
// -----------------------------------------------------------------------------

func TestHealthAnalyzer_HealthyPod_NoIssues(t *testing.T) {
	cl := newFakeClient()
	a := NewHealthAnalyzer()

	healthy := cl.addResourceRaw("Pod", newPod("ok", "default",
		addContainerStatus(map[string]interface{}{
			"name": "app", "image": "app:1", "restartCount": float64(0),
			"state": map[string]interface{}{"running": map[string]interface{}{}},
		}),
	))

	issues, err := a.Analyze(context.Background(), cl, healthy)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("healthy pod should produce 0 issues, got %d: %+v", len(issues), issues)
	}
}

func TestHealthAnalyzer_CrashLoopBackOff_ReturnsCritical(t *testing.T) {
	cl := newFakeClient()
	a := NewHealthAnalyzer()

	r := cl.addResourceRaw("Pod", newPod("crash", "default", withCrashLoop("app", 7)))
	issues, err := a.Analyze(context.Background(), cl, r)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	issue := findIssue(issues, "health.crashloop")
	if issue == nil {
		t.Fatalf("expected health.crashloop issue, got %+v", issues)
	}
	if issue.Severity != SeverityCritical {
		t.Errorf("CrashLoopBackOff severity = %v, want Critical", issue.Severity)
	}
	if got := issue.Details["restart_count"]; got != 7 {
		t.Errorf("restart_count = %v, want 7", got)
	}
	if len(issue.Fixes) == 0 {
		t.Error("CrashLoopBackOff issue should include fix hints")
	}
}

func TestHealthAnalyzer_ImagePullBackOff_ReturnsCritical(t *testing.T) {
	cl := newFakeClient()
	a := NewHealthAnalyzer()

	r := cl.addResourceRaw("Pod", newPod("bad-image", "default", withImagePullBackOff("app")))
	issues, _ := a.Analyze(context.Background(), cl, r)

	issue := findIssue(issues, "health.imagepull")
	if issue == nil {
		t.Fatalf("expected health.imagepull issue, got %+v", issues)
	}
	if issue.Severity != SeverityCritical {
		t.Errorf("ImagePullBackOff severity = %v, want Critical", issue.Severity)
	}
}

func TestHealthAnalyzer_OOMKilled_ReturnsWarningWithHints(t *testing.T) {
	cl := newFakeClient()
	a := NewHealthAnalyzer()

	r := cl.addResourceRaw("Pod", newPod("oom", "prod", withOOMKilled("app")))
	issues, _ := a.Analyze(context.Background(), cl, r)

	issue := findIssue(issues, "health.oomkilled")
	if issue == nil {
		t.Fatalf("expected health.oomkilled issue, got %+v", issues)
	}
	if issue.Severity != SeverityWarning {
		t.Errorf("OOMKilled severity = %v, want Warning", issue.Severity)
	}
	if got := issue.Details["exit_code"]; got != 137 {
		t.Errorf("exit_code = %v, want 137", got)
	}
	if len(issue.Fixes) == 0 {
		t.Error("OOMKilled issue should have remediation fixes")
	}
}

func TestHealthAnalyzer_HighRestarts_Warning(t *testing.T) {
	cl := newFakeClient()
	a := NewHealthAnalyzer()

	r := cl.addResourceRaw("Pod", newPod("flappy", "default", withHighRestarts("app", 6)))
	issues, _ := a.Analyze(context.Background(), cl, r)

	issue := findIssue(issues, "health.highrestarts")
	if issue == nil {
		t.Fatalf("expected health.highrestarts issue, got %+v", issues)
	}
	if issue.Severity != SeverityWarning {
		t.Errorf("highrestarts severity = %v, want Warning", issue.Severity)
	}
}

func TestHealthAnalyzer_FailedPhase_Critical(t *testing.T) {
	cl := newFakeClient()
	a := NewHealthAnalyzer()

	r := cl.addResourceRaw("Pod", newPod("dead", "default",
		withPodPhase("Failed"),
		withPodReason("Evicted", "The node was low on resource: memory"),
	))
	issues, _ := a.Analyze(context.Background(), cl, r)

	issue := findIssue(issues, "health.failed")
	if issue == nil {
		t.Fatalf("expected health.failed, got %+v", issues)
	}
	if issue.Severity != SeverityCritical {
		t.Errorf("Failed phase severity = %v, want Critical", issue.Severity)
	}
	if got, _ := issue.Details["reason"].(string); got != "Evicted" {
		t.Errorf("reason = %q, want Evicted", got)
	}
}

func TestHealthAnalyzer_NonPodKind_ReturnsNil(t *testing.T) {
	cl := newFakeClient()
	a := NewHealthAnalyzer()

	svc := cl.addResourceRaw("Service", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "svc", "namespace": "default"},
	})
	issues, err := a.Analyze(context.Background(), cl, svc)
	if err != nil {
		t.Fatalf("Analyze non-pod: %v", err)
	}
	if issues != nil {
		t.Errorf("non-pod kind should return nil issues, got %+v", issues)
	}
}

func TestHealthAnalyzer_InvalidJSON_ReturnsError(t *testing.T) {
	// Unmarshal error must surface; a broken pod should not silently
	// produce zero issues.
	a := NewHealthAnalyzer()
	bad := client.Resource{Kind: "Pod", Raw: []byte(`{not json`)}
	issues, err := a.analyzePod(bad)
	if err == nil {
		t.Fatal("expected unmarshal error for invalid JSON, got nil")
	}
	if issues != nil {
		t.Errorf("expected nil issues on error, got %+v", issues)
	}
}

func TestHealthAnalyzer_Scan_AggregatesIssuesAcrossPods(t *testing.T) {
	cl := newFakeClient()
	a := NewHealthAnalyzer()

	cl.addResourceRaw("pods", newPod("a", "default", withCrashLoop("c", 3)))
	cl.addResourceRaw("pods", newPod("b", "default", withOOMKilled("c")))
	cl.addResourceRaw("pods", newPod("c", "default")) // healthy

	issues, err := a.Scan(context.Background(), cl, "default")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !hasIssueID(issues, "health.crashloop") {
		t.Error("expected health.crashloop in aggregated scan")
	}
	if !hasIssueID(issues, "health.oomkilled") {
		t.Error("expected health.oomkilled in aggregated scan")
	}
	if len(issues) < 2 {
		t.Errorf("expected at least 2 issues across 3 pods, got %d", len(issues))
	}
}

func TestHealthAnalyzer_Metadata(t *testing.T) {
	a := NewHealthAnalyzer()
	if a.Name() != "health" {
		t.Errorf("Name() = %q, want health", a.Name())
	}
	if a.Category() != CategoryConfig {
		t.Errorf("Category() = %q, want %q", a.Category(), CategoryConfig)
	}
}

// -----------------------------------------------------------------------------
// JSON wiring
// -----------------------------------------------------------------------------

func TestHealthAnalyzer_OOM_DetailsShape(t *testing.T) {
	cl := newFakeClient()
	r := cl.addResourceRaw("Pod", newPod("oom2", "x", withOOMKilled("worker")))
	issues, _ := NewHealthAnalyzer().Analyze(context.Background(), cl, r)
	i := findIssue(issues, "health.oomkilled")
	if i == nil {
		t.Fatal("missing oom issue")
	}
	b, _ := json.Marshal(i.Details)
	if len(b) == 0 {
		t.Error("Details must be marshal-able JSON")
	}
}
