// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"context"
	"testing"

	"github.com/thepixelabs/kubecat/internal/client"
)

func TestWorkloadAnalyzer_Metadata(t *testing.T) {
	a := NewWorkloadAnalyzer()
	if a.Name() != "workload" {
		t.Errorf("Name() = %q, want workload", a.Name())
	}
	if a.Category() != CategoryConfig {
		t.Errorf("Category() = %q, want Config", a.Category())
	}
}

func TestWorkloadAnalyzer_NonWorkloadKind_ReturnsNil(t *testing.T) {
	cl := newFakeClient()
	a := NewWorkloadAnalyzer()
	r := cl.addResourceRaw("ConfigMap", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "cm"},
	})
	issues, err := a.Analyze(context.Background(), cl, r)
	if err != nil || issues != nil {
		t.Errorf("non-workload should be no-op, got issues=%v err=%v", issues, err)
	}
}

// ---- Deployment ------------------------------------------------------------

func TestWorkloadAnalyzer_Deployment_UnavailableReplicas_Warning(t *testing.T) {
	cl := newFakeClient()
	a := NewWorkloadAnalyzer()
	r := cl.addResourceRaw("Deployment", newDeployment("api", "prod", 3, 1, 2))
	issues, _ := a.Analyze(context.Background(), cl, r)

	i := findIssue(issues, "workload.unavailable")
	if i == nil {
		t.Fatalf("expected workload.unavailable issue, got %+v", issues)
	}
	if i.Severity != SeverityWarning {
		t.Errorf("severity = %v, want Warning", i.Severity)
	}
	if got := i.Details["unavailable"]; got != 2 {
		t.Errorf("unavailable = %v, want 2", got)
	}
}

func TestWorkloadAnalyzer_Deployment_RolloutStuck_Critical(t *testing.T) {
	cl := newFakeClient()
	a := NewWorkloadAnalyzer()

	dep := newDeployment("api", "prod", 3, 3, 0)
	withDeploymentCondition(dep, "Progressing", "False", "ProgressDeadlineExceeded",
		"deployment timeout")
	r := cl.addResourceRaw("Deployment", dep)

	issues, _ := a.Analyze(context.Background(), cl, r)
	i := findIssue(issues, "workload.rollout.stuck")
	if i == nil {
		t.Fatalf("expected rollout stuck, got %+v", issues)
	}
	if i.Severity != SeverityCritical {
		t.Errorf("severity = %v, want Critical", i.Severity)
	}
}

func TestWorkloadAnalyzer_Deployment_ZeroReplicas_Info(t *testing.T) {
	cl := newFakeClient()
	a := NewWorkloadAnalyzer()
	r := cl.addResourceRaw("Deployment", newDeployment("paused", "dev", 0, 0, 0))
	issues, _ := a.Analyze(context.Background(), cl, r)

	i := findIssue(issues, "workload.zeroreplicas")
	if i == nil {
		t.Fatalf("expected zeroreplicas issue, got %+v", issues)
	}
	if i.Severity != SeverityInfo {
		t.Errorf("severity = %v, want Info", i.Severity)
	}
}

func TestWorkloadAnalyzer_Deployment_Healthy_NoIssues(t *testing.T) {
	cl := newFakeClient()
	a := NewWorkloadAnalyzer()
	r := cl.addResourceRaw("Deployment", newDeployment("healthy", "prod", 3, 3, 0))
	issues, _ := a.Analyze(context.Background(), cl, r)
	if len(issues) != 0 {
		t.Errorf("healthy deployment should have 0 issues, got %+v", issues)
	}
}

// ---- StatefulSet -----------------------------------------------------------

func TestWorkloadAnalyzer_StatefulSet_NotFullyReady(t *testing.T) {
	cl := newFakeClient()
	a := NewWorkloadAnalyzer()

	sts := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "db", "namespace": "data"},
		"spec":     map[string]interface{}{"replicas": float64(3)},
		"status":   map[string]interface{}{"replicas": float64(3), "readyReplicas": float64(1)},
	}
	r := cl.addResourceRaw("StatefulSet", sts)
	issues, _ := a.Analyze(context.Background(), cl, r)
	i := findIssue(issues, "workload.statefulset.notready")
	if i == nil {
		t.Fatalf("expected statefulset.notready, got %+v", issues)
	}
	if i.Severity != SeverityWarning {
		t.Errorf("severity = %v, want Warning", i.Severity)
	}
}

func TestWorkloadAnalyzer_DaemonSet_Unavailable(t *testing.T) {
	cl := newFakeClient()
	a := NewWorkloadAnalyzer()
	ds := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "node-agent", "namespace": "kube-system"},
		"status": map[string]interface{}{
			"desiredNumberScheduled": float64(5),
			"numberReady":            float64(3),
			"numberUnavailable":      float64(2),
		},
	}
	r := cl.addResourceRaw("DaemonSet", ds)
	issues, _ := a.Analyze(context.Background(), cl, r)
	i := findIssue(issues, "workload.daemonset.unavailable")
	if i == nil {
		t.Fatalf("expected daemonset.unavailable, got %+v", issues)
	}
	if i.Severity != SeverityWarning {
		t.Errorf("severity = %v, want Warning", i.Severity)
	}
}

// ---- Scan aggregation ------------------------------------------------------

func TestWorkloadAnalyzer_Scan_CombinesAllKinds(t *testing.T) {
	cl := newFakeClient()
	a := NewWorkloadAnalyzer()

	cl.addResourceRaw("deployments", newDeployment("d", "ns", 3, 1, 2))
	cl.addResourceRaw("statefulsets", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "s", "namespace": "ns"},
		"spec":     map[string]interface{}{"replicas": float64(2)},
		"status":   map[string]interface{}{"readyReplicas": float64(0), "replicas": float64(2)},
	})
	cl.addResourceRaw("daemonsets", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "n", "namespace": "ns"},
		"status": map[string]interface{}{
			"desiredNumberScheduled": float64(3), "numberReady": float64(1), "numberUnavailable": float64(2),
		},
	})

	issues, err := a.Scan(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !hasIssueID(issues, "workload.unavailable") {
		t.Error("missing workload.unavailable")
	}
	if !hasIssueID(issues, "workload.statefulset.notready") {
		t.Error("missing workload.statefulset.notready")
	}
	if !hasIssueID(issues, "workload.daemonset.unavailable") {
		t.Error("missing workload.daemonset.unavailable")
	}
}

func TestWorkloadAnalyzer_InvalidJSON_Deployment_ReturnsError(t *testing.T) {
	a := NewWorkloadAnalyzer()
	bad := client.Resource{Kind: "Deployment", Raw: []byte("not-json")}
	_, err := a.analyzeDeployment(bad)
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}
