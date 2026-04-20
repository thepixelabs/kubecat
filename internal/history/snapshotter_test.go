// SPDX-License-Identifier: Apache-2.0

package history

import (
	"testing"
	"time"

	"github.com/thepixelabs/kubecat/internal/storage"
)

// ---------------------------------------------------------------------------
// CompareSnapshots — the core diff contract
// ---------------------------------------------------------------------------

func TestCompareSnapshots_Added(t *testing.T) {
	before := &storage.SnapshotData{
		Timestamp: time.Now().Add(-1 * time.Minute),
		Resources: map[string][]storage.ResourceInfo{
			"pods": {
				{Name: "a", Namespace: "ns", Status: "Running", ResourceVersion: "1"},
			},
		},
	}
	after := &storage.SnapshotData{
		Timestamp: time.Now(),
		Resources: map[string][]storage.ResourceInfo{
			"pods": {
				{Name: "a", Namespace: "ns", Status: "Running", ResourceVersion: "1"},
				{Name: "b", Namespace: "ns", Status: "Running", ResourceVersion: "1"},
			},
		},
	}

	diff := CompareSnapshots(before, after)
	if len(diff.Added) != 1 || diff.Added[0].Name != "b" {
		t.Errorf("expected [b] added, got %+v", diff.Added)
	}
	if len(diff.Removed) != 0 {
		t.Errorf("expected no removed, got %+v", diff.Removed)
	}
}

func TestCompareSnapshots_Removed(t *testing.T) {
	before := &storage.SnapshotData{
		Resources: map[string][]storage.ResourceInfo{
			"pods": {{Name: "gone", Namespace: "ns", ResourceVersion: "1"}},
		},
	}
	after := &storage.SnapshotData{
		Resources: map[string][]storage.ResourceInfo{"pods": {}},
	}

	diff := CompareSnapshots(before, after)
	if len(diff.Removed) != 1 || diff.Removed[0].Name != "gone" {
		t.Errorf("expected [gone] removed, got %+v", diff.Removed)
	}
}

func TestCompareSnapshots_Modified_OnStatusChange(t *testing.T) {
	before := &storage.SnapshotData{
		Resources: map[string][]storage.ResourceInfo{
			"pods": {{Name: "p", Namespace: "ns", Status: "Pending", ResourceVersion: "1"}},
		},
	}
	after := &storage.SnapshotData{
		Resources: map[string][]storage.ResourceInfo{
			"pods": {{Name: "p", Namespace: "ns", Status: "Running", ResourceVersion: "1"}},
		},
	}

	diff := CompareSnapshots(before, after)
	if len(diff.Modified) != 1 {
		t.Fatalf("expected 1 modified, got %d (%+v)", len(diff.Modified), diff.Modified)
	}
	m := diff.Modified[0]
	if m.OldStatus != "Pending" || m.NewStatus != "Running" {
		t.Errorf("unexpected status transition: %+v", m)
	}
}

func TestCompareSnapshots_Modified_OnResourceVersionBump(t *testing.T) {
	// Same status but resourceVersion changed → still counted as modified.
	before := &storage.SnapshotData{
		Resources: map[string][]storage.ResourceInfo{
			"pods": {{Name: "p", Namespace: "ns", Status: "Running", ResourceVersion: "1"}},
		},
	}
	after := &storage.SnapshotData{
		Resources: map[string][]storage.ResourceInfo{
			"pods": {{Name: "p", Namespace: "ns", Status: "Running", ResourceVersion: "42"}},
		},
	}

	diff := CompareSnapshots(before, after)
	if len(diff.Modified) != 1 {
		t.Errorf("rv-only change should count as modified, got %d", len(diff.Modified))
	}
}

func TestCompareSnapshots_NoChanges(t *testing.T) {
	snap := &storage.SnapshotData{
		Resources: map[string][]storage.ResourceInfo{
			"pods": {{Name: "p", Namespace: "ns", Status: "Running", ResourceVersion: "1"}},
		},
	}
	diff := CompareSnapshots(snap, snap)
	if len(diff.Added)+len(diff.Removed)+len(diff.Modified) != 0 {
		t.Errorf("identical snapshots should produce zero changes, got %+v", diff)
	}
}

func TestCompareSnapshots_AcrossMultipleKinds(t *testing.T) {
	before := &storage.SnapshotData{
		Resources: map[string][]storage.ResourceInfo{
			"pods":     {{Name: "p", Namespace: "ns", ResourceVersion: "1"}},
			"services": {{Name: "svc", Namespace: "ns", ResourceVersion: "1"}},
		},
	}
	after := &storage.SnapshotData{
		Resources: map[string][]storage.ResourceInfo{
			"pods":     {{Name: "p", Namespace: "ns", ResourceVersion: "1"}},
			"services": {{Name: "svc", Namespace: "ns", ResourceVersion: "2"}},
		},
	}

	diff := CompareSnapshots(before, after)
	if len(diff.Modified) != 1 || diff.Modified[0].Kind != "services" {
		t.Errorf("expected service modified, got %+v", diff.Modified)
	}
}

func TestCompareSnapshots_NamespaceInKeyPreventsCollision(t *testing.T) {
	// Two pods with the same name in different namespaces must not collide.
	before := &storage.SnapshotData{
		Resources: map[string][]storage.ResourceInfo{
			"pods": {{Name: "p", Namespace: "ns-a", ResourceVersion: "1"}},
		},
	}
	after := &storage.SnapshotData{
		Resources: map[string][]storage.ResourceInfo{
			"pods": {{Name: "p", Namespace: "ns-b", ResourceVersion: "1"}},
		},
	}

	diff := CompareSnapshots(before, after)
	if len(diff.Added) != 1 || diff.Added[0].Namespace != "ns-b" {
		t.Errorf("expected pod in ns-b added, got %+v", diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0].Namespace != "ns-a" {
		t.Errorf("expected pod in ns-a removed, got %+v", diff.Removed)
	}
}

// ---------------------------------------------------------------------------
// getResourceVersion — raw JSON parsing
// ---------------------------------------------------------------------------

func TestGetResourceVersion(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"present", `{"metadata":{"resourceVersion":"42"}}`, "42"},
		{"missing", `{"metadata":{}}`, ""},
		{"empty_raw", "", ""},
		{"malformed", `{not json`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getResourceVersion([]byte(tt.in)); got != tt.want {
				t.Errorf("getResourceVersion(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DefaultSnapshotterConfig
// ---------------------------------------------------------------------------

func TestDefaultSnapshotterConfig_ReasonableDefaults(t *testing.T) {
	cfg := DefaultSnapshotterConfig()
	if cfg.Interval <= 0 {
		t.Errorf("Interval = %v, want > 0", cfg.Interval)
	}
	if cfg.Retention <= 0 {
		t.Errorf("Retention = %v, want > 0", cfg.Retention)
	}
	if len(cfg.ResourceKinds) == 0 {
		t.Error("ResourceKinds must include at least one kind")
	}
	// "pods" must be included; the entire feature hinges on snapshotting pods.
	found := false
	for _, k := range cfg.ResourceKinds {
		if k == "pods" {
			found = true
			break
		}
	}
	if !found {
		t.Error("DefaultSnapshotterConfig.ResourceKinds missing 'pods'")
	}
}
