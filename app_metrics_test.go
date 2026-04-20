// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"
)

// nodeAllocObj builds a Node object with only the allocatable fields set.
func nodeAllocObj(name, cpu, mem string) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{"name": name},
		"status": map[string]interface{}{
			"allocatable": map[string]interface{}{
				"cpu":    cpu,
				"memory": mem,
			},
		},
	}
}

// nodePodObj builds a Pod with requests/limits and a nodeName. Phase is the
// pod phase (Running, Pending, Succeeded, Failed).
func nodePodObj(name, nodeName, phase, reqCPU, reqMem, limCPU, limMem string) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{"name": name, "namespace": "default"},
		"spec": map[string]interface{}{
			"nodeName": nodeName,
			"containers": []interface{}{
				map[string]interface{}{
					"name": "c",
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{"cpu": reqCPU, "memory": reqMem},
						"limits":   map[string]interface{}{"cpu": limCPU, "memory": limMem},
					},
				},
			},
		},
		"status": map[string]interface{}{"phase": phase},
	}
}

// TestGetNodeAllocation_RequestsVsAllocatable seeds a node with 4000m cores
// and one pod using 1000m. The CPURequestPct must be 25%.
//
// NOTE: The test uses the 'm' suffix for pod requests because the App's
// parseResourceQuantity returns millicores directly for "m"-suffixed
// values, and the node path multiplies bare integers by 1000 but the pod
// path does not — using the suffix avoids that asymmetry. See
// TestGetNodeAllocation_AsymmetricCPUParsing_PinsBug for the documented
// asymmetry.
func TestGetNodeAllocation_RequestsVsAllocatable(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("nodes", nodeAllocObj("node-1", "4", "8Gi"))
	cl.addResource("pods", nodePodObj("p1", "node-1", "Running",
		"1000m", "1Gi", "2000m", "2Gi"))

	a := newAppWithFakes(cl)
	nodes, err := a.GetNodeAllocation()
	if err != nil {
		t.Fatalf("GetNodeAllocation: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	n := nodes[0]
	if n.NodeName != "node-1" {
		t.Errorf("NodeName = %q, want node-1", n.NodeName)
	}
	// 1000m / 4000m = 25%
	if n.CPURequestPct != 25 {
		t.Errorf("CPURequestPct = %d, want 25", n.CPURequestPct)
	}
	// 2000m / 4000m = 50%
	if n.CPULimitPct != 50 {
		t.Errorf("CPULimitPct = %d, want 50", n.CPULimitPct)
	}
	// 1 Gi / 8 Gi = 12.5% → truncated to 12%
	if n.MemRequestPct != 12 {
		t.Errorf("MemRequestPct = %d, want 12", n.MemRequestPct)
	}
	if n.PodCount != 1 {
		t.Errorf("PodCount = %d, want 1", n.PodCount)
	}
}

// TestGetNodeAllocation_AsymmetricCPUParsing_PinsBug documents an asymmetry
// in parseResourceQuantity integration: when node allocatable is "4"
// (bare integer cores), GetNodeAllocation multiplies by 1000 to get
// millicores. When a pod request is also "1" (bare integer), the code
// does NOT multiply, so 1 millicore is charged against 4000 millicores.
// A reader expects 25% but gets 0% (floor).
//
// Flip this assertion when GetNodeAllocation is fixed to normalize pod
// requests the same way node allocatable is normalized.
func TestGetNodeAllocation_AsymmetricCPUParsing_PinsBug(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("nodes", nodeAllocObj("node-1", "4", "8Gi"))
	// Pod requests "1" bare integer (meaning 1 core), but the current
	// app parses this as 1 millicore against a 4000 millicore node.
	cl.addResource("pods", nodePodObj("p1", "node-1", "Running",
		"1", "1Gi", "", ""))
	a := newAppWithFakes(cl)
	nodes, _ := a.GetNodeAllocation()
	if nodes[0].CPURequestPct != 0 {
		t.Errorf("PIN: asymmetric parser currently returns 0%% for 1-core pod on 4-core node; got %d (bug may be fixed — flip this assertion to 25)",
			nodes[0].CPURequestPct)
	}
}

// TestGetNodeAllocation_ZeroAllocatable_NaNSafe verifies that a node with
// zero cpu/memory allocatable does not divide-by-zero — percentages stay 0.
func TestGetNodeAllocation_ZeroAllocatable_NaNSafe(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("nodes", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "dead"},
		"status": map[string]interface{}{
			"allocatable": map[string]interface{}{
				"cpu":    "0",
				"memory": "0",
			},
		},
	})
	cl.addResource("pods", nodePodObj("p", "dead", "Running",
		"100m", "128Mi", "200m", "256Mi"))

	a := newAppWithFakes(cl)
	nodes, err := a.GetNodeAllocation()
	if err != nil {
		t.Fatalf("GetNodeAllocation: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	n := nodes[0]
	// Zero allocatable → percentages must not be computed (stay 0).
	if n.CPURequestPct != 0 || n.MemRequestPct != 0 {
		t.Errorf("zero allocatable node: expected zero pct, got cpu=%d mem=%d",
			n.CPURequestPct, n.MemRequestPct)
	}
}

// TestGetNodeAllocation_OnlyRunningAndPendingCounted verifies Succeeded/
// Failed pods are excluded from the aggregation.
func TestGetNodeAllocation_OnlyRunningAndPendingCounted(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("nodes", nodeAllocObj("n", "4", "8Gi"))
	cl.addResource("pods", nodePodObj("running", "n", "Running", "100m", "128Mi", "", ""))
	cl.addResource("pods", nodePodObj("pending", "n", "Pending", "100m", "128Mi", "", ""))
	cl.addResource("pods", nodePodObj("succeeded", "n", "Succeeded", "100m", "128Mi", "", ""))
	cl.addResource("pods", nodePodObj("failed", "n", "Failed", "100m", "128Mi", "", ""))

	a := newAppWithFakes(cl)
	nodes, err := a.GetNodeAllocation()
	if err != nil {
		t.Fatalf("GetNodeAllocation: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].PodCount != 2 {
		t.Errorf("PodCount = %d, want 2 (only Running+Pending counted)", nodes[0].PodCount)
	}
}

// TestGetNodeAllocation_PodsWithoutNodeNameSkipped ensures unscheduled pods
// (empty nodeName) are not attributed to any node.
func TestGetNodeAllocation_PodsWithoutNodeNameSkipped(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("nodes", nodeAllocObj("n", "4", "8Gi"))
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "unscheduled", "namespace": "default"},
		"spec": map[string]interface{}{
			"nodeName": "",
			"containers": []interface{}{
				map[string]interface{}{
					"name": "c",
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{"cpu": "100m", "memory": "128Mi"},
					},
				},
			},
		},
		"status": map[string]interface{}{"phase": "Pending"},
	})

	a := newAppWithFakes(cl)
	nodes, err := a.GetNodeAllocation()
	if err != nil {
		t.Fatalf("GetNodeAllocation: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].PodCount != 0 {
		t.Errorf("PodCount = %d, want 0 (unscheduled pod excluded)", nodes[0].PodCount)
	}
}

// TestGetNodeAllocation_NoClusterConnection_Error exercises the
// no-active-cluster branch.
func TestGetNodeAllocation_NoClusterConnection_Error(t *testing.T) {
	a := newAppWithFakes(nil)
	if _, err := a.GetNodeAllocation(); err == nil {
		t.Error("expected error when no active cluster is available")
	}
}

// TestGetNodeAllocation_EmptyCluster_NoPanic ensures that with no nodes or
// pods the function returns an empty slice without panicking.
func TestGetNodeAllocation_EmptyCluster_NoPanic(t *testing.T) {
	cl := newFakeClusterClient()
	a := newAppWithFakes(cl)
	nodes, err := a.GetNodeAllocation()
	if err != nil {
		t.Fatalf("GetNodeAllocation: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("empty cluster should return 0 nodes, got %d", len(nodes))
	}
}
