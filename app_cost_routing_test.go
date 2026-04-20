// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/thepixelabs/kubecat/internal/cost"
)

// ---------------------------------------------------------------------------
// OpenCost-endpoint routing — these tests pin that when a non-empty
// openCostEndpoint is configured, GetNamespaceCostSummary and GetWorkloadCost
// route through QueryOpenCost. On error or no-match they fall back to the
// heuristic and the Source field reflects which path served the response.
// ---------------------------------------------------------------------------

// setOpenCostEndpoint writes a config.yaml that configures the OpenCost
// endpoint and isolates the config dir for the duration of the test.
func setOpenCostEndpoint(t *testing.T, endpoint string) {
	t.Helper()
	dir := isolateConfigDir(t)
	content := "kubecat:\n  cost:\n    openCostEndpoint: \"" + endpoint + "\"\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0600); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
}

// TestGetNamespaceCostSummary_RoutesToOpenCost_WhenEndpointSet verifies that a
// configured endpoint is queried and estimates come back with Source=opencost.
func TestGetNamespaceCostSummary_RoutesToOpenCost_WhenEndpointSet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{"my-pod-abc": map[string]interface{}{
					"totalCost": 0.10,
					"cpuCost":   0.06,
					"ramCost":   0.04,
				}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	setOpenCostEndpoint(t, srv.URL)
	cl := newFakeClusterClient()
	a := newAppWithFakes(cl)

	sum, err := a.GetNamespaceCostSummary("any", "team-a")
	if err != nil {
		t.Fatalf("GetNamespaceCostSummary: %v", err)
	}
	if sum.Source != cost.SourceOpenCost {
		t.Errorf("Source = %q, want opencost", sum.Source)
	}
	if len(sum.Workloads) != 1 {
		t.Fatalf("expected 1 workload from OpenCost, got %d", len(sum.Workloads))
	}
}

// TestGetNamespaceCostSummary_FallsBackOnOpenCostError ensures that when
// OpenCost responds with non-JSON (parse error), the heuristic path serves
// the response with Source=heuristic.
func TestGetNamespaceCostSummary_FallsBackOnOpenCostError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	setOpenCostEndpoint(t, srv.URL)
	cl := newFakeClusterClient()
	cl.addResource("pods", podCostObj("nginx-abc-111", "ns", "100m", "128Mi"))
	a := newAppWithFakes(cl)

	sum, err := a.GetNamespaceCostSummary("any", "ns")
	if err != nil {
		t.Fatalf("GetNamespaceCostSummary: %v", err)
	}
	if sum.Source != cost.SourceHeuristic {
		t.Errorf("Source = %q, want heuristic (fell back from OpenCost parse error)", sum.Source)
	}
}

// TestGetNamespaceCostSummary_FallsBackOnEmptyOpenCostResponse ensures that
// when OpenCost returns {"data": []} (no workloads), the heuristic serves the
// response so the namespace summary is never empty when pods exist.
func TestGetNamespaceCostSummary_FallsBackOnEmptyOpenCostResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	setOpenCostEndpoint(t, srv.URL)
	cl := newFakeClusterClient()
	cl.addResource("pods", podCostObj("nginx-abc-111", "ns", "100m", "128Mi"))
	a := newAppWithFakes(cl)

	sum, err := a.GetNamespaceCostSummary("any", "ns")
	if err != nil {
		t.Fatalf("GetNamespaceCostSummary: %v", err)
	}
	if sum.Source != cost.SourceHeuristic {
		t.Errorf("Source = %q, want heuristic (empty OpenCost response must fall back)", sum.Source)
	}
	if len(sum.Workloads) == 0 {
		t.Error("expected heuristic fallback to surface the nginx pod")
	}
}

// TestGetWorkloadCost_RoutesToOpenCost_WhenEndpointSet verifies a specific
// workload name is matched by prefix in OpenCost response.
func TestGetWorkloadCost_RoutesToOpenCost_WhenEndpointSet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{"nginx-7d9f6b8c4-abc12": map[string]interface{}{
					"totalCost": 0.40,
					"cpuCost":   0.30,
					"ramCost":   0.10,
				}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	setOpenCostEndpoint(t, srv.URL)
	cl := newFakeClusterClient()
	a := newAppWithFakes(cl)

	got, err := a.GetWorkloadCost("any", "ns", "nginx")
	if err != nil {
		t.Fatalf("GetWorkloadCost: %v", err)
	}
	if got.Source != cost.SourceOpenCost {
		t.Errorf("Source = %q, want opencost", got.Source)
	}
	if got.TotalCost != 0.40 {
		t.Errorf("TotalCost = %v, want 0.40", got.TotalCost)
	}
}

// TestGetWorkloadCost_FallsBackToHeuristic_WhenOpenCostMissesWorkload verifies
// that if OpenCost doesn't report the specific workload, the heuristic runs.
func TestGetWorkloadCost_FallsBackToHeuristic_WhenOpenCostMissesWorkload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// OpenCost returns data for a DIFFERENT workload only.
		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{"other-wl-aaa": map[string]interface{}{"totalCost": 0.01}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	setOpenCostEndpoint(t, srv.URL)
	cl := newFakeClusterClient()
	cl.addResource("pods", podCostObj("target-wl-xxx-111", "ns", "500m", "512Mi"))
	a := newAppWithFakes(cl)

	got, err := a.GetWorkloadCost("any", "ns", "target-wl")
	if err != nil {
		t.Fatalf("GetWorkloadCost: %v", err)
	}
	if got.Source != cost.SourceHeuristic {
		t.Errorf("Source = %q, want heuristic (OpenCost missed the workload)", got.Source)
	}
}

// ---------------------------------------------------------------------------
// Settings CRUD
// ---------------------------------------------------------------------------

// TestGetCostSettings_AppliesDefaultsWhenEmpty pins the default-substitution
// behavior on fresh config.
func TestGetCostSettings_AppliesDefaultsWhenEmpty(t *testing.T) {
	isolateConfigDir(t)
	a := &App{ctx: context.Background()}
	got, err := a.GetCostSettings()
	if err != nil {
		t.Fatalf("GetCostSettings: %v", err)
	}
	if got.CPUCostPerCoreHour != 0.048 {
		t.Errorf("CPU = %v, want 0.048", got.CPUCostPerCoreHour)
	}
	if got.MemCostPerGBHour != 0.006 {
		t.Errorf("Mem = %v, want 0.006", got.MemCostPerGBHour)
	}
	if got.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", got.Currency)
	}
}
