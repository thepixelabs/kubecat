// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thepixelabs/kubecat/internal/cost"
)

// TestDetectBackend_OpenCost verifies that a Service whose name contains
// "opencost" in the opencost namespace is detected as BackendOpenCost.
func TestDetectBackend_OpenCost(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("services", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "opencost", "namespace": "opencost"},
	})

	got := cost.DetectBackend(context.Background(), cl)
	if got != cost.BackendOpenCost {
		t.Errorf("DetectBackend = %q, want %q", got, cost.BackendOpenCost)
	}
}

// TestDetectBackend_Kubecost verifies that a kubecost-cost-analyzer Service
// is detected as BackendKubecost.
func TestDetectBackend_Kubecost(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("services", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "kubecost-cost-analyzer", "namespace": "monitoring"},
	})

	got := cost.DetectBackend(context.Background(), cl)
	if got != cost.BackendKubecost {
		t.Errorf("DetectBackend = %q, want %q", got, cost.BackendKubecost)
	}
}

// TestDetectBackend_None_EmptyCluster returns BackendNone when no known
// services exist in the probed namespaces.
func TestDetectBackend_None_EmptyCluster(t *testing.T) {
	cl := newFakeClusterClient()
	got := cost.DetectBackend(context.Background(), cl)
	if got != cost.BackendNone {
		t.Errorf("DetectBackend = %q, want %q on empty cluster", got, cost.BackendNone)
	}
}

// TestQueryOpenCost_ParsesAllocationResponse wires a fake OpenCost server and
// asserts the response JSON is parsed into []CostEstimate.
func TestQueryOpenCost_ParsesAllocationResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sanity check: URL contains allocation path
		if r.URL.Path != "/model/allocation" {
			t.Errorf("unexpected request path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"my-pod-abc123": map[string]interface{}{
						"totalCost": 0.50,
						"cpuCost":   0.30,
						"ramCost":   0.20,
					},
					"my-pod-def456": map[string]interface{}{
						"totalCost": 0.25,
						"cpuCost":   0.15,
						"ramCost":   0.10,
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	estimates, err := cost.QueryOpenCost(context.Background(), server.URL, "team-a")
	if err != nil {
		t.Fatalf("QueryOpenCost: %v", err)
	}
	if len(estimates) != 2 {
		t.Fatalf("expected 2 estimates, got %d", len(estimates))
	}
	// Each estimate is SourceOpenCost, period hour.
	for _, e := range estimates {
		if e.Source != cost.SourceOpenCost {
			t.Errorf("source = %q, want %q", e.Source, cost.SourceOpenCost)
		}
		if e.Namespace != "team-a" {
			t.Errorf("namespace = %q, want team-a", e.Namespace)
		}
		if e.Period != "hour" {
			t.Errorf("period = %q, want hour", e.Period)
		}
		if e.MonthlyTotal == 0 {
			t.Errorf("MonthlyTotal should be derived from totalCost × 730, got 0")
		}
	}
}

// TestQueryOpenCost_InvalidJSON_ReturnsError ensures non-JSON response
// returns an error.
func TestQueryOpenCost_InvalidJSON_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	_, err := cost.QueryOpenCost(context.Background(), server.URL, "ns")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestQueryOpenCost_TransportError returns an error when the endpoint is
// unreachable.
func TestQueryOpenCost_TransportError(t *testing.T) {
	// Start, then close — the URL is now dead.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	_, err := cost.QueryOpenCost(context.Background(), server.URL, "ns")
	if err == nil {
		t.Error("expected transport error from closed server")
	}
}

// TestQueryOpenCost_CanceledContext_ReturnsError ensures a canceled
// context propagates to the HTTP client.
func TestQueryOpenCost_CanceledContext_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than we'll wait.
		fmt.Fprintln(w, `{"data": []}`)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before issuing request

	_, err := cost.QueryOpenCost(ctx, server.URL, "ns")
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

// ---------------------------------------------------------------------------
// Heuristic calibration — verify specific dollar amounts with seeded inputs
// ---------------------------------------------------------------------------

// TestEstimator_HeuristicCalibration_KnownAmount locks the heuristic arithmetic:
// 2 cores × 0.048 + 4 GB × 0.006 = 0.096 + 0.024 = 0.12 $/hour.
// Over 730 h = 87.6 $/month.
func TestEstimator_HeuristicCalibration_KnownAmount(t *testing.T) {
	// Seed pods that aggregate to 2 cores + 4 GB and compute against them.
	cl := newFakeClusterClient()
	cl.addResource("pods", podCostObj("my-app-abc-111", "ns", "2", "4Gi"))
	est := cost.New(cl, 0.048, 0.006, "USD")

	sum, err := est.GetNamespaceCost(context.Background(), "ns")
	if err != nil {
		t.Fatalf("GetNamespaceCost: %v", err)
	}
	if len(sum.Workloads) != 1 {
		t.Fatalf("expected 1 workload, got %d", len(sum.Workloads))
	}
	wl := sum.Workloads[0]

	// Allow a small floating-point / round2 tolerance.
	wantHour := 0.12
	if approxEqual(wl.TotalCost, wantHour, 0.01) == false {
		t.Errorf("TotalCost/hour = %v, want %v", wl.TotalCost, wantHour)
	}
	wantMonth := wantHour * 730.0
	if approxEqual(wl.MonthlyTotal, wantMonth, 1.0) == false {
		t.Errorf("MonthlyTotal = %v, want ~%v", wl.MonthlyTotal, wantMonth)
	}
	if sum.Source != cost.SourceHeuristic {
		t.Errorf("source = %q, want %q", sum.Source, cost.SourceHeuristic)
	}
}

// TestInferWorkload_StatefulSetMisAttribution pins the documented
// StatefulSet bug — `inferWorkload` strips off two suffixes, so a
// StatefulSet pod like "kafka-0" (single hyphen) is NOT attributed to
// "kafka" but to "kafka-0". Future fix should flip this assertion.
func TestInferWorkload_StatefulSetMisAttribution(t *testing.T) {
	cl := newFakeClusterClient()
	// StatefulSet pods use `<workload>-<ordinal>`.
	cl.addResource("pods", podCostObj("kafka-0", "ns", "100m", "128Mi"))
	cl.addResource("pods", podCostObj("kafka-1", "ns", "100m", "128Mi"))

	est := cost.New(cl, 0.048, 0.006, "USD")
	sum, err := est.GetNamespaceCost(context.Background(), "ns")
	if err != nil {
		t.Fatalf("GetNamespaceCost: %v", err)
	}

	// CURRENT BEHAVIOR: inferWorkload splits "kafka-0" into ["kafka","0"]
	// and returns len<=2 → unchanged. So each pod becomes its own
	// "workload" rather than aggregating to "kafka".
	// We assert this so a future fix (flip the <=2 check to detect
	// StatefulSet ordinals) is surfaced in review.
	if len(sum.Workloads) != 2 {
		t.Errorf("PIN: StatefulSet pods should currently mis-attribute to 2 workloads; got %d (bug may be fixed — update test)",
			len(sum.Workloads))
	}
}

// TestInferWorkload_DeploymentAttribution verifies a standard Deployment
// pod name collapses to the deployment name.
func TestInferWorkload_DeploymentAttribution(t *testing.T) {
	cl := newFakeClusterClient()
	// Deployment: <name>-<rs-hash>-<pod-hash>
	cl.addResource("pods", podCostObj("nginx-7d9f6b8c4-abc12", "ns", "100m", "128Mi"))
	cl.addResource("pods", podCostObj("nginx-7d9f6b8c4-def34", "ns", "100m", "128Mi"))

	est := cost.New(cl, 0.048, 0.006, "USD")
	sum, err := est.GetNamespaceCost(context.Background(), "ns")
	if err != nil {
		t.Fatalf("GetNamespaceCost: %v", err)
	}
	if len(sum.Workloads) != 1 {
		t.Fatalf("expected 1 workload from Deployment pods, got %d", len(sum.Workloads))
	}
	if sum.Workloads[0].WorkloadName != "nginx" {
		t.Errorf("workload = %q, want nginx", sum.Workloads[0].WorkloadName)
	}
}

// TestInferWorkload_JobPodAttribution pins the behavior for Job pods where
// the name is `<job>-<pod-hash>`. Since the splitter strips 2 trailing
// parts, a single-hyphen job pod like "backup-abc" becomes its own
// workload — matching the StatefulSet quirk.
func TestInferWorkload_JobPodAttribution(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("pods", podCostObj("backup-abcd", "ns", "100m", "128Mi"))
	est := cost.New(cl, 0.048, 0.006, "USD")
	sum, err := est.GetNamespaceCost(context.Background(), "ns")
	if err != nil {
		t.Fatalf("GetNamespaceCost: %v", err)
	}
	// Two hyphens required → single-hyphen pod is its own workload (PIN).
	if len(sum.Workloads) != 1 {
		t.Fatalf("expected 1 workload, got %d", len(sum.Workloads))
	}
	if sum.Workloads[0].WorkloadName != "backup-abcd" {
		t.Errorf("workload = %q, want backup-abcd (PIN: short names are not stripped)",
			sum.Workloads[0].WorkloadName)
	}
}

// TestInferWorkload_DaemonSetAttribution pins a DaemonSet pod name like
// `kube-proxy-<node-hash>-<pod-hash>` collapses to "kube-proxy".
func TestInferWorkload_DaemonSetAttribution(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("pods", podCostObj("kube-proxy-abcd-12345", "ns", "100m", "128Mi"))
	est := cost.New(cl, 0.048, 0.006, "USD")
	sum, err := est.GetNamespaceCost(context.Background(), "ns")
	if err != nil {
		t.Fatalf("GetNamespaceCost: %v", err)
	}
	if len(sum.Workloads) != 1 {
		t.Fatalf("expected 1 workload, got %d", len(sum.Workloads))
	}
	if sum.Workloads[0].WorkloadName != "kube-proxy" {
		t.Errorf("workload = %q, want kube-proxy", sum.Workloads[0].WorkloadName)
	}
}

// TestEstimator_EmptyCluster_NoPanic verifies getting cost on an empty
// namespace returns a valid (empty) summary without panicking.
func TestEstimator_EmptyCluster_NoPanic(t *testing.T) {
	cl := newFakeClusterClient()
	est := cost.New(cl, 0.048, 0.006, "USD")
	sum, err := est.GetNamespaceCost(context.Background(), "empty")
	if err != nil {
		t.Fatalf("GetNamespaceCost: %v", err)
	}
	if sum == nil {
		t.Fatal("summary is nil")
	}
	if sum.TotalPerHour != 0 {
		t.Errorf("empty cluster TotalPerHour = %v, want 0", sum.TotalPerHour)
	}
}

// TestGetWorkloadCost_WorkloadNotFound surfaces a descriptive error when
// no pods prefix-match the workload name.
func TestGetWorkloadCost_WorkloadNotFound(t *testing.T) {
	cl := newFakeClusterClient()
	est := cost.New(cl, 0.048, 0.006, "USD")
	_, err := est.GetWorkloadCost(context.Background(), "ns", "ghost")
	if err == nil {
		t.Error("expected error when no matching pods exist")
	}
}

// ---------------------------------------------------------------------------
// App.GetNamespaceCostSummary + App.GetWorkloadCost — heuristic fallback
// ---------------------------------------------------------------------------

// TestGetNamespaceCostSummary_HeuristicWhenEndpointEmpty verifies that with
// no OpenCost endpoint configured the heuristic path runs and returns
// SourceHeuristic.
func TestGetNamespaceCostSummary_HeuristicWhenEndpointEmpty(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("pods", podCostObj("nginx-7d9f-abc12", "ns", "500m", "512Mi"))
	a := newAppWithFakes(cl)

	sum, err := a.GetNamespaceCostSummary("any", "ns")
	if err != nil {
		t.Fatalf("GetNamespaceCostSummary: %v", err)
	}
	if sum.Source != cost.SourceHeuristic {
		t.Errorf("source = %q, want heuristic", sum.Source)
	}
}

// TestGetNamespaceCostSummary_EmptyCluster_NoPanic pins safety on an empty
// namespace.
func TestGetNamespaceCostSummary_EmptyCluster_NoPanic(t *testing.T) {
	cl := newFakeClusterClient()
	a := newAppWithFakes(cl)

	sum, err := a.GetNamespaceCostSummary("any", "empty")
	if err != nil {
		t.Fatalf("GetNamespaceCostSummary: %v", err)
	}
	if sum == nil {
		t.Fatal("summary is nil")
	}
}

// TestDetectCostBackend_ReturnsDetectedValue exercises the App bridge —
// empty cluster returns "none".
func TestDetectCostBackend_ReturnsDetectedValue(t *testing.T) {
	a := newAppWithFakes(newFakeClusterClient())
	got, err := a.DetectCostBackend()
	if err != nil {
		t.Fatalf("DetectCostBackend: %v", err)
	}
	if got != "none" {
		t.Errorf("got %q, want none", got)
	}
}

// TestFirstCurrency_PreferenceOrder pins the helper behavior — takes first
// non-empty currency from the estimates, else fallback, else USD.
func TestFirstCurrency_PreferenceOrder(t *testing.T) {
	got := firstCurrency([]cost.CostEstimate{{Currency: "EUR"}, {Currency: "USD"}}, "GBP")
	if got != "EUR" {
		t.Errorf("first-winning currency = %q, want EUR", got)
	}
	got = firstCurrency([]cost.CostEstimate{{Currency: ""}}, "GBP")
	if got != "GBP" {
		t.Errorf("fallback currency = %q, want GBP", got)
	}
	got = firstCurrency(nil, "")
	if got != "USD" {
		t.Errorf("default currency = %q, want USD", got)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func podCostObj(name, namespace, cpu, mem string) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{"name": name, "namespace": namespace},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name": "c",
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu":    cpu,
							"memory": mem,
						},
					},
				},
			},
		},
	}
}

func approxEqual(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}
