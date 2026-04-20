// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"context"
	"errors"
	"testing"

	"github.com/thepixelabs/kubecat/internal/client"
)

// stubAnalyzer is a test double whose behavior we can program. We avoid
// mocking anything we don't own — this is just an Analyzer we've implemented
// ourselves to exercise Registry behavior in isolation.
type stubAnalyzer struct {
	name      string
	cat       Category
	analyzeFn func() ([]Issue, error)
	scanFn    func() ([]Issue, error)
	calls     int
	scanCalls int
}

func (s *stubAnalyzer) Name() string       { return s.name }
func (s *stubAnalyzer) Category() Category { return s.cat }
func (s *stubAnalyzer) Analyze(_ context.Context, _ client.ClusterClient, _ client.Resource) ([]Issue, error) {
	s.calls++
	if s.analyzeFn != nil {
		return s.analyzeFn()
	}
	return nil, nil
}
func (s *stubAnalyzer) Scan(_ context.Context, _ client.ClusterClient, _ string) ([]Issue, error) {
	s.scanCalls++
	if s.scanFn != nil {
		return s.scanFn()
	}
	return nil, nil
}

func TestRegistry_RegisterAndList(t *testing.T) {
	r := NewRegistry()
	a1 := &stubAnalyzer{name: "a1", cat: CategoryNode}
	a2 := &stubAnalyzer{name: "a2", cat: CategoryStorage}
	r.Register(a1)
	r.Register(a2)

	got := r.Analyzers()
	if len(got) != 2 {
		t.Fatalf("Analyzers() returned %d, want 2", len(got))
	}
	// Order is preserved.
	if got[0].Name() != "a1" || got[1].Name() != "a2" {
		t.Errorf("order changed: got %s,%s", got[0].Name(), got[1].Name())
	}

	// Returned slice is a copy — mutating it must not affect the registry.
	got[0] = nil
	if r.Analyzers()[0] == nil {
		t.Error("Analyzers() returned internal slice, not a copy")
	}
}

func TestRegistry_AnalyzersByCategory_FiltersCorrectly(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubAnalyzer{name: "n", cat: CategoryNode})
	r.Register(&stubAnalyzer{name: "s1", cat: CategoryStorage})
	r.Register(&stubAnalyzer{name: "s2", cat: CategoryStorage})

	got := r.AnalyzersByCategory(CategoryStorage)
	if len(got) != 2 {
		t.Errorf("AnalyzersByCategory(Storage) returned %d, want 2", len(got))
	}
	for _, a := range got {
		if a.Category() != CategoryStorage {
			t.Errorf("got category %v in Storage filter", a.Category())
		}
	}
}

func TestRegistry_Analyze_RunsAllAnalyzersAndAggregatesIssues(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubAnalyzer{name: "a", analyzeFn: func() ([]Issue, error) {
		return []Issue{{ID: "x1"}}, nil
	}})
	r.Register(&stubAnalyzer{name: "b", analyzeFn: func() ([]Issue, error) {
		return []Issue{{ID: "x2"}, {ID: "x3"}}, nil
	}})

	result, err := r.Analyze(context.Background(), newFakeClient(), client.Resource{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Issues) != 3 {
		t.Errorf("aggregated %d issues, want 3", len(result.Issues))
	}
}

func TestRegistry_Analyze_IgnoresAnalyzerErrorsAndContinues(t *testing.T) {
	r := NewRegistry()
	broken := &stubAnalyzer{name: "broken", analyzeFn: func() ([]Issue, error) {
		return nil, errors.New("intentional")
	}}
	working := &stubAnalyzer{name: "working", analyzeFn: func() ([]Issue, error) {
		return []Issue{{ID: "ok"}}, nil
	}}
	r.Register(broken)
	r.Register(working)

	result, err := r.Analyze(context.Background(), newFakeClient(), client.Resource{})
	if err != nil {
		t.Fatalf("Analyze must not fail when a sub-analyzer errors; got %v", err)
	}
	if len(result.Issues) != 1 {
		t.Errorf("expected issues from working analyzer only, got %+v", result.Issues)
	}
	if broken.calls != 1 {
		t.Error("broken analyzer should still have been invoked")
	}
}

func TestRegistry_Scan_GroupsByCategoryAndCounts(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubAnalyzer{name: "n", cat: CategoryNode, scanFn: func() ([]Issue, error) {
		return []Issue{
			{ID: "n1", Severity: SeverityCritical, Category: CategoryNode},
			{ID: "n2", Severity: SeverityWarning, Category: CategoryNode},
		}, nil
	}})
	r.Register(&stubAnalyzer{name: "s", cat: CategoryStorage, scanFn: func() ([]Issue, error) {
		return []Issue{
			{ID: "s1", Severity: SeverityInfo, Category: CategoryStorage},
		}, nil
	}})

	summary, err := r.Scan(context.Background(), newFakeClient(), "")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if summary.Critical != 1 {
		t.Errorf("Critical = %d, want 1", summary.Critical)
	}
	if summary.Warning != 1 {
		t.Errorf("Warning = %d, want 1", summary.Warning)
	}
	if summary.Info != 1 {
		t.Errorf("Info = %d, want 1", summary.Info)
	}
	if len(summary.IssuesByCategory[CategoryNode]) != 2 {
		t.Errorf("Node category should have 2 issues")
	}
	if len(summary.IssuesByCategory[CategoryStorage]) != 1 {
		t.Errorf("Storage category should have 1 issue")
	}
}

func TestRegistry_ScanCategory_RunsOnlyMatchingAnalyzers(t *testing.T) {
	r := NewRegistry()
	node := &stubAnalyzer{name: "n", cat: CategoryNode, scanFn: func() ([]Issue, error) {
		return []Issue{{ID: "n1"}}, nil
	}}
	storage := &stubAnalyzer{name: "s", cat: CategoryStorage, scanFn: func() ([]Issue, error) {
		return []Issue{{ID: "s1"}}, nil
	}}
	r.Register(node)
	r.Register(storage)

	issues, err := r.ScanCategory(context.Background(), newFakeClient(), CategoryNode, "")
	if err != nil {
		t.Fatalf("ScanCategory: %v", err)
	}
	if len(issues) != 1 || issues[0].ID != "n1" {
		t.Errorf("expected only node issues, got %+v", issues)
	}
	if storage.scanCalls != 0 {
		t.Error("storage analyzer should not have been invoked for Node category")
	}
}

func TestRegistry_Scan_SwallowsScanErrors(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubAnalyzer{name: "broken", scanFn: func() ([]Issue, error) {
		return nil, errors.New("boom")
	}})
	r.Register(&stubAnalyzer{name: "good", scanFn: func() ([]Issue, error) {
		return []Issue{{ID: "g", Severity: SeverityWarning, Category: CategoryConfig}}, nil
	}})

	summary, err := r.Scan(context.Background(), newFakeClient(), "")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if summary.Warning != 1 {
		t.Errorf("Warning count = %d, want 1 (from good analyzer)", summary.Warning)
	}
}

func TestDefaultRegistry_HasBuiltinAnalyzersRegistered(t *testing.T) {
	// The init() functions in health/scheduling register built-ins. Verify the
	// expected names are present so a rename accidentally removing registration
	// doesn't silently disable a whole category.
	names := make(map[string]bool)
	for _, a := range DefaultRegistry.Analyzers() {
		names[a.Name()] = true
	}
	for _, want := range []string{"health", "workload", "storage", "node", "scheduling"} {
		if !names[want] {
			t.Errorf("DefaultRegistry missing analyzer %q", want)
		}
	}
}
