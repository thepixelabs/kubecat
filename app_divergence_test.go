// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/thepixelabs/kubecat/internal/client"
	"github.com/thepixelabs/kubecat/internal/security"
)

// ---------------------------------------------------------------------------
// Scanner duplicate divergence — pin the differences between the two parallel
// implementations:
//
//   * app_security.go (this package)         — the ORIGINAL, still consumed by
//                                               the Wails bindings.
//   * internal/security/scanner.go            — a newer structured scanner
//                                               that is NOT wired into the App.
//
// Until one is retired, both must be tested independently. Two documented
// divergences:
//
//   D1. Missing-NetworkPolicy severity:
//        - app_security.scanNetworkPolicyGaps     → Medium
//        - internal/security/scanner               → High
//
//   D2. Cluster-admin RBAC finding:
//        - app_security.checkDangerousAccess      → reason "Has cluster-admin
//                                                     or equivalent privileges"
//        - internal/security.analyzeBinding        → reason "Has cluster-admin
//                                                     equivalent permissions
//                                                     (all verbs on all
//                                                     resources)"
//
// These tests pin both so future convergence work is visible in diff review.
// ---------------------------------------------------------------------------

// TestDivergence_MissingNetPolicy_SeverityDiffers asserts both classifiers are
// exercised in the same scenario and their severities remain DIFFERENT
// (Medium vs High). If a follow-up unifies them, this test flips.
func TestDivergence_MissingNetPolicy_SeverityDiffers(t *testing.T) {
	// --- internal/security scanner path ---
	cl1 := newSecFakeClient()
	cl1.addNS("prod")
	appIssues := newAppSideScan(t, cl1, "prod")

	// --- app_security path ---
	// Use the main-package fake.
	cl2 := newFakeClusterClient()
	cl2.addResource("namespaces", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "prod"},
	})
	a := newAppWithFakes(cl2)
	appSummary, err := a.GetSecuritySummary("prod")
	if err != nil {
		t.Fatalf("GetSecuritySummary: %v", err)
	}

	// Assert app path produces a Medium issue (pin current behavior).
	if appSummary.MediumCount == 0 {
		t.Errorf("app_security.scanNetworkPolicyGaps must flag missing netpols as Medium; got MediumCount=0")
	}
	// Assert internal path produces a High issue (pin current behavior).
	var highCount int
	for _, i := range appIssues {
		if i.Severity == security.SeverityHigh && i.Category == security.CategoryNetwork {
			highCount++
		}
	}
	if highCount == 0 {
		t.Errorf("internal/security scanner must flag missing netpols as High; got 0")
	}
}

// TestDivergence_ClusterAdmin_ReasonDiffers verifies the two implementations
// use materially different reason strings for the cluster-admin finding. This
// is locked in because the frontend may display the reason verbatim.
func TestDivergence_ClusterAdmin_ReasonDiffers(t *testing.T) {
	// --- app_security path ---
	a := &App{}
	appSum := &RBACSummary{}
	a.checkDangerousAccess(appSum, &RBACBinding{
		Name:     "admin-binding",
		RoleName: "cluster-admin",
		Subjects: []RBACSubject{{Kind: "User", Name: "alice"}},
	})
	if len(appSum.DangerousAccess) != 1 {
		t.Fatalf("app side: expected 1 finding, got %d", len(appSum.DangerousAccess))
	}
	appReason := appSum.DangerousAccess[0].Reason

	// --- internal/security path ---
	cl := newSecFakeClient()
	cl.addCRB("admin", "cluster-admin", []map[string]interface{}{
		{"kind": "User", "name": "alice"},
	})
	cl.addCR("cluster-admin", []map[string]interface{}{
		{"verbs": []string{"*"}, "resources": []string{"*"}, "apiGroups": []string{"*"}},
	})
	s := security.NewScanner(cl)
	analysis, err := s.AnalyzeRBAC(context.Background())
	if err != nil {
		t.Fatalf("AnalyzeRBAC: %v", err)
	}
	if len(analysis.DangerousAccess) == 0 {
		t.Fatal("internal side: expected at least one DangerousAccess")
	}
	internalReason := analysis.DangerousAccess[0].Reason

	// Both mention cluster-admin, but strings are distinct. Pin both.
	if !strings.Contains(appReason, "cluster-admin") {
		t.Errorf("app reason lost 'cluster-admin': %q", appReason)
	}
	if !strings.Contains(internalReason, "cluster-admin") {
		t.Errorf("internal reason lost 'cluster-admin': %q", internalReason)
	}
	if appReason == internalReason {
		t.Errorf("reasons should currently differ between implementations; if unified, flip this assertion. Got:\n  app:      %q\n  internal: %q",
			appReason, internalReason)
	}
}

// ---------------------------------------------------------------------------
// Small in-package fake targeting the security.ClusterClient interface
// ---------------------------------------------------------------------------

type secFakeClient struct {
	resources map[string][]client.Resource
}

func newSecFakeClient() *secFakeClient {
	return &secFakeClient{resources: map[string][]client.Resource{}}
}
func (f *secFakeClient) addNS(name string) {
	obj := map[string]interface{}{"metadata": map[string]interface{}{"name": name}}
	raw, _ := json.Marshal(obj)
	f.resources["namespaces"] = append(f.resources["namespaces"],
		client.Resource{Kind: "namespaces", Name: name, Raw: raw, Object: obj})
}
func (f *secFakeClient) addCRB(name, roleName string, subjects []map[string]interface{}) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{"name": name},
		"roleRef":  map[string]interface{}{"kind": "ClusterRole", "name": roleName},
		"subjects": subjects,
	}
	raw, _ := json.Marshal(obj)
	f.resources["clusterrolebindings"] = append(f.resources["clusterrolebindings"],
		client.Resource{Kind: "clusterrolebindings", Name: name, Raw: raw, Object: obj})
}
func (f *secFakeClient) addCR(name string, rules []map[string]interface{}) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{"name": name},
		"rules":    rules,
	}
	raw, _ := json.Marshal(obj)
	f.resources["clusterroles"] = append(f.resources["clusterroles"],
		client.Resource{Kind: "clusterroles", Name: name, Raw: raw, Object: obj})
}

func (f *secFakeClient) Info(_ context.Context) (*client.ClusterInfo, error) {
	return &client.ClusterInfo{}, nil
}
func (f *secFakeClient) List(_ context.Context, kind string, _ client.ListOptions) (*client.ResourceList, error) {
	items := f.resources[kind]
	return &client.ResourceList{Items: items, Total: len(items)}, nil
}
func (f *secFakeClient) Get(_ context.Context, kind, _, name string) (*client.Resource, error) {
	for _, r := range f.resources[kind] {
		if r.Name == name {
			return &r, nil
		}
	}
	return nil, client.ErrResourceNotFound
}
func (f *secFakeClient) Delete(_ context.Context, _, _, _ string) error { return nil }
func (f *secFakeClient) Watch(_ context.Context, _ string, _ client.WatchOptions) (<-chan client.WatchEvent, error) {
	ch := make(chan client.WatchEvent)
	close(ch)
	return ch, nil
}
func (f *secFakeClient) Logs(_ context.Context, _, _, _ string, _ bool, _ int64) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}
func (f *secFakeClient) Exec(_ context.Context, _, _, _ string, _ []string) error { return nil }
func (f *secFakeClient) PortForward(_ context.Context, _, _ string, _, _ int) (client.PortForwarder, error) {
	return nil, nil
}
func (f *secFakeClient) Close() error { return nil }

// newAppSideScan invokes the internal scanner's scanNetworkPolicyIssues via
// GetSecuritySummary (public surface). It returns the top issues for assertion.
func newAppSideScan(t *testing.T, cl *secFakeClient, namespace string) []security.SecurityIssue {
	t.Helper()
	s := security.NewScanner(cl)
	sum, err := s.GetSecuritySummary(context.Background(), namespace)
	if err != nil {
		t.Fatalf("internal GetSecuritySummary: %v", err)
	}
	return sum.TopIssues
}
