// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"context"
	"testing"
)

// -----------------------------------------------------------------------------
// StorageAnalyzer
// -----------------------------------------------------------------------------

func TestStorageAnalyzer_Metadata(t *testing.T) {
	a := NewStorageAnalyzer()
	if a.Name() != "storage" {
		t.Errorf("Name() = %q, want storage", a.Name())
	}
	if a.Category() != CategoryStorage {
		t.Errorf("Category() = %q, want Storage", a.Category())
	}
}

func TestStorageAnalyzer_PendingPVC_Critical(t *testing.T) {
	cl := newFakeClient()
	a := NewStorageAnalyzer()

	r := cl.addResourceRaw("PersistentVolumeClaim", newPVC("orphan", "app", "Pending"))
	issues, _ := a.Analyze(context.Background(), cl, r)

	i := findIssue(issues, "storage.pvc.pending")
	if i == nil {
		t.Fatalf("expected pvc.pending issue, got %+v", issues)
	}
	if i.Severity != SeverityCritical {
		t.Errorf("severity = %v, want Critical", i.Severity)
	}
	if sc := i.Details["storage_class"]; sc != "standard" {
		t.Errorf("storage_class detail = %v, want standard", sc)
	}
}

func TestStorageAnalyzer_LostPVC_Critical(t *testing.T) {
	cl := newFakeClient()
	a := NewStorageAnalyzer()

	r := cl.addResourceRaw("PersistentVolumeClaim", newPVC("lost-data", "db", "Lost"))
	issues, _ := a.Analyze(context.Background(), cl, r)

	i := findIssue(issues, "storage.pvc.lost")
	if i == nil {
		t.Fatalf("expected pvc.lost issue, got %+v", issues)
	}
	if i.Severity != SeverityCritical {
		t.Errorf("severity = %v, want Critical", i.Severity)
	}
}

func TestStorageAnalyzer_BoundPVC_NoIssues(t *testing.T) {
	cl := newFakeClient()
	a := NewStorageAnalyzer()

	r := cl.addResourceRaw("PersistentVolumeClaim", newPVC("ok", "default", "Bound"))
	issues, _ := a.Analyze(context.Background(), cl, r)
	if len(issues) != 0 {
		t.Errorf("Bound PVC should have 0 issues, got %+v", issues)
	}
}

func TestStorageAnalyzer_NonPVCKind_ReturnsNil(t *testing.T) {
	cl := newFakeClient()
	a := NewStorageAnalyzer()
	r := cl.addResourceRaw("Secret", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "s"},
	})
	issues, err := a.Analyze(context.Background(), cl, r)
	if err != nil || issues != nil {
		t.Errorf("non-PVC should be nil/nil, got %v / %v", issues, err)
	}
}

func TestStorageAnalyzer_Scan(t *testing.T) {
	cl := newFakeClient()
	a := NewStorageAnalyzer()
	cl.addResourceRaw("persistentvolumeclaims", newPVC("a", "ns", "Pending"))
	cl.addResourceRaw("persistentvolumeclaims", newPVC("b", "ns", "Bound"))

	issues, err := a.Scan(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue (pending), got %d (%+v)", len(issues), issues)
	}
}

// -----------------------------------------------------------------------------
// NodeAnalyzer
// -----------------------------------------------------------------------------

func TestNodeAnalyzer_Metadata(t *testing.T) {
	a := NewNodeAnalyzer()
	if a.Name() != "node" {
		t.Errorf("Name() = %q, want node", a.Name())
	}
	if a.Category() != CategoryNode {
		t.Errorf("Category() = %q, want Node", a.Category())
	}
}

func TestNodeAnalyzer_Ready_NoIssues(t *testing.T) {
	cl := newFakeClient()
	a := NewNodeAnalyzer()
	r := cl.addResourceRaw("Node", newNode("node-1"))
	issues, _ := a.Analyze(context.Background(), cl, r)
	if len(issues) != 0 {
		t.Errorf("ready node should have 0 issues, got %+v", issues)
	}
}

func TestNodeAnalyzer_NotReady_Critical(t *testing.T) {
	cl := newFakeClient()
	a := NewNodeAnalyzer()
	r := cl.addResourceRaw("Node", newNode("down",
		withNodeCondition("Ready", "False"),
	))
	issues, _ := a.Analyze(context.Background(), cl, r)
	i := findIssue(issues, "node.notready")
	if i == nil {
		t.Fatalf("expected node.notready, got %+v", issues)
	}
	if i.Severity != SeverityCritical {
		t.Errorf("severity = %v, want Critical", i.Severity)
	}
}

func TestNodeAnalyzer_PressureConditions(t *testing.T) {
	tests := []struct {
		name, condType, issueID string
	}{
		{"memory_pressure", "MemoryPressure", "node.memorypressure"},
		{"disk_pressure", "DiskPressure", "node.diskpressure"},
		{"pid_pressure", "PIDPressure", "node.pidpressure"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := newFakeClient()
			a := NewNodeAnalyzer()
			r := cl.addResourceRaw("Node", newNode("pressure-node",
				withNodeCondition(tt.condType, "True"),
			))
			issues, _ := a.Analyze(context.Background(), cl, r)
			i := findIssue(issues, tt.issueID)
			if i == nil {
				t.Fatalf("expected %s, got %+v", tt.issueID, issues)
			}
			if i.Severity != SeverityWarning {
				t.Errorf("%s severity = %v, want Warning", tt.issueID, i.Severity)
			}
		})
	}
}

func TestNodeAnalyzer_NetworkUnavailable_Critical(t *testing.T) {
	cl := newFakeClient()
	a := NewNodeAnalyzer()
	r := cl.addResourceRaw("Node", newNode("net-down",
		withNodeCondition("NetworkUnavailable", "True"),
	))
	issues, _ := a.Analyze(context.Background(), cl, r)
	i := findIssue(issues, "node.networkunavailable")
	if i == nil {
		t.Fatalf("expected networkunavailable, got %+v", issues)
	}
	if i.Severity != SeverityCritical {
		t.Errorf("severity = %v, want Critical", i.Severity)
	}
}

func TestNodeAnalyzer_NonNodeKind_ReturnsNil(t *testing.T) {
	cl := newFakeClient()
	a := NewNodeAnalyzer()
	r := cl.addResourceRaw("Pod", newPod("p", "ns"))
	issues, err := a.Analyze(context.Background(), cl, r)
	if err != nil || issues != nil {
		t.Errorf("non-node should be nil/nil, got %v / %v", issues, err)
	}
}

func TestNodeAnalyzer_Scan(t *testing.T) {
	cl := newFakeClient()
	a := NewNodeAnalyzer()
	cl.addResourceRaw("nodes", newNode("n1")) // healthy
	cl.addResourceRaw("nodes", newNode("n2", withNodeCondition("MemoryPressure", "True")))
	cl.addResourceRaw("nodes", newNode("n3", withNodeCondition("Ready", "False")))

	issues, err := a.Scan(context.Background(), cl, "")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !hasIssueID(issues, "node.memorypressure") {
		t.Error("missing memory pressure issue")
	}
	if !hasIssueID(issues, "node.notready") {
		t.Error("missing notready issue")
	}
}
