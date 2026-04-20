// SPDX-License-Identifier: Apache-2.0

package security

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/thepixelabs/kubecat/internal/client"
)

// addNamed inserts an object into the fake cluster and also sets the
// client.Resource's Name/Namespace fields so Get() lookups succeed. The
// existing fakeClusterClient.addResource only stores Raw, which is sufficient
// for List but not for Get-by-name paths exercised here.
func addNamed(f *fakeClusterClient, kind string, obj map[string]interface{}) {
	raw, _ := json.Marshal(obj)
	var name, namespace string
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		name, _ = meta["name"].(string)
		namespace, _ = meta["namespace"].(string)
	}
	f.resources[kind] = append(f.resources[kind], client.Resource{
		Kind: kind, Name: name, Namespace: namespace, Raw: raw, Object: obj,
	})
}

// ---------------------------------------------------------------------------
// AnalyzeRBAC — dangerous/wildcard access analysis
// ---------------------------------------------------------------------------

// TestAnalyzeRBAC_WildcardVerbAndResource_FlagsDangerousAndWildcard pins the
// critical code path: a ClusterRoleBinding granting verbs=["*"] resources=["*"]
// must surface:
//  1. A DangerousAccess entry with reason "cluster-admin equivalent..."
//  2. A WildcardAccess entry
func TestAnalyzeRBAC_WildcardVerbAndResource_FlagsDangerousAndWildcard(t *testing.T) {
	cl := newFakeClient()
	addNamed(cl, "clusterrolebindings", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "allpowerful"},
		"roleRef":  map[string]interface{}{"kind": "ClusterRole", "name": "allpowerful"},
		"subjects": []interface{}{
			map[string]interface{}{"kind": "User", "name": "alice"},
		},
	})
	addNamed(cl, "clusterroles", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "allpowerful"},
		"rules": []interface{}{
			map[string]interface{}{
				"verbs":     []string{"*"},
				"resources": []string{"*"},
				"apiGroups": []string{"*"},
			},
		},
	})

	s := NewScanner(cl)
	analysis, err := s.AnalyzeRBAC(context.Background())
	if err != nil {
		t.Fatalf("AnalyzeRBAC: %v", err)
	}
	if len(analysis.DangerousAccess) == 0 {
		t.Fatal("expected at least one DangerousAccess finding for wildcard CRB")
	}
	if !strings.Contains(analysis.DangerousAccess[0].Reason, "cluster-admin") {
		t.Errorf("reason = %q, want substring 'cluster-admin'", analysis.DangerousAccess[0].Reason)
	}
	if len(analysis.WildcardAccess) == 0 {
		t.Error("expected WildcardAccess to also be populated")
	}
}

// TestAnalyzeRBAC_SecretsReadFlaggedAsDangerous verifies read access to
// secrets is flagged separately from wildcard access.
func TestAnalyzeRBAC_SecretsReadFlaggedAsDangerous(t *testing.T) {
	cl := newFakeClient()
	addNamed(cl, "rolebindings", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "reader", "namespace": "app"},
		"roleRef":  map[string]interface{}{"kind": "Role", "name": "reader"},
		"subjects": []interface{}{
			map[string]interface{}{"kind": "User", "name": "bob"},
		},
	})
	addNamed(cl, "roles", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "reader", "namespace": "app"},
		"rules": []interface{}{
			map[string]interface{}{
				"verbs":     []string{"get", "list"},
				"resources": []string{"secrets"},
				"apiGroups": []string{""},
			},
		},
	})

	s := NewScanner(cl)
	analysis, err := s.AnalyzeRBAC(context.Background())
	if err != nil {
		t.Fatalf("AnalyzeRBAC: %v", err)
	}
	// Should include a "Can read secrets" finding.
	sawSecrets := false
	for _, da := range analysis.DangerousAccess {
		if strings.Contains(da.Reason, "Can read secrets") {
			sawSecrets = true
		}
	}
	if !sawSecrets {
		t.Errorf("expected 'Can read secrets' finding in %+v", analysis.DangerousAccess)
	}
	// Secrets read should NOT incur a WildcardAccess entry.
	if len(analysis.WildcardAccess) != 0 {
		t.Errorf("non-wildcard rule must not produce WildcardAccess, got %d", len(analysis.WildcardAccess))
	}
}

// TestAnalyzeRBAC_SubjectAccessTracksNamespaces verifies SubjectAccess records
// the namespaces each subject is bound in.
func TestAnalyzeRBAC_SubjectAccessTracksNamespaces(t *testing.T) {
	cl := newFakeClient()
	for _, ns := range []string{"team-a", "team-b"} {
		addNamed(cl, "rolebindings", map[string]interface{}{
			"metadata": map[string]interface{}{"name": "viewer-" + ns, "namespace": ns},
			"roleRef":  map[string]interface{}{"kind": "Role", "name": "viewer"},
			"subjects": []interface{}{
				map[string]interface{}{"kind": "User", "name": "alice"},
			},
		})
	}
	addNamed(cl, "clusterroles", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "viewer"},
		"rules": []interface{}{
			map[string]interface{}{"verbs": []string{"get"}, "resources": []string{"pods"}, "apiGroups": []string{""}},
		},
	})

	s := NewScanner(cl)
	analysis, err := s.AnalyzeRBAC(context.Background())
	if err != nil {
		t.Fatalf("AnalyzeRBAC: %v", err)
	}
	key := "User:alice"
	namespaces := analysis.SubjectAccess[key]
	if len(namespaces) != 2 {
		t.Errorf("expected alice to show up in 2 namespaces, got %v", namespaces)
	}
}

// ---------------------------------------------------------------------------
// Gatekeeper & Kyverno policy parsing
// ---------------------------------------------------------------------------

// TestGetPolicySummary_Gatekeeper parses a Gatekeeper constraint and sums
// violations.
func TestGetPolicySummary_Gatekeeper(t *testing.T) {
	cl := newFakeClient()
	cl.addResource("constraints", map[string]interface{}{
		"kind": "K8sRequiredLabels",
		"metadata": map[string]interface{}{
			"name": "ns-must-have-owner",
		},
		"spec": map[string]interface{}{
			"enforcementAction": "deny",
			"match": map[string]interface{}{
				"kinds": []interface{}{
					map[string]interface{}{"kinds": []string{"Namespace"}},
				},
			},
		},
		"status": map[string]interface{}{"totalViolations": 3},
	})

	s := NewScanner(cl)
	summary, err := s.GetPolicySummary(context.Background())
	if err != nil {
		t.Fatalf("GetPolicySummary: %v", err)
	}
	if summary.Provider != "gatekeeper" {
		t.Errorf("Provider = %q, want gatekeeper", summary.Provider)
	}
	if summary.TotalPolicies != 1 {
		t.Errorf("TotalPolicies = %d, want 1", summary.TotalPolicies)
	}
	if summary.TotalViolations != 3 {
		t.Errorf("TotalViolations = %d, want 3", summary.TotalViolations)
	}
}

// TestGetPolicySummary_Gatekeeper_DefaultEnforcement covers the code path
// where spec.enforcementAction is empty → default "deny".
func TestGetPolicySummary_Gatekeeper_DefaultEnforcement(t *testing.T) {
	cl := newFakeClient()
	cl.addResource("constraints", map[string]interface{}{
		"kind":     "K8sRequiredLabels",
		"metadata": map[string]interface{}{"name": "c1"},
		"spec": map[string]interface{}{
			"match": map[string]interface{}{
				"kinds": []interface{}{
					map[string]interface{}{"kinds": []string{"Pod"}},
				},
			},
		},
	})
	s := NewScanner(cl)
	summary, err := s.GetPolicySummary(context.Background())
	if err != nil {
		t.Fatalf("GetPolicySummary: %v", err)
	}
	if len(summary.Policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(summary.Policies))
	}
	if summary.Policies[0].Enforcement != "deny" {
		t.Errorf("default enforcement = %q, want deny", summary.Policies[0].Enforcement)
	}
}

// TestGetPolicySummary_Kyverno parses a ClusterPolicy when Gatekeeper is absent.
func TestGetPolicySummary_Kyverno(t *testing.T) {
	cl := newFakeClient()
	cl.addResource("clusterpolicies", map[string]interface{}{
		"kind":     "ClusterPolicy",
		"metadata": map[string]interface{}{"name": "require-labels"},
		"spec": map[string]interface{}{
			"validationFailureAction": "enforce",
			"rules": []interface{}{
				map[string]interface{}{
					"match": map[string]interface{}{
						"resources": map[string]interface{}{
							"kinds": []string{"Pod", "Deployment"},
						},
					},
				},
			},
		},
	})

	s := NewScanner(cl)
	summary, err := s.GetPolicySummary(context.Background())
	if err != nil {
		t.Fatalf("GetPolicySummary: %v", err)
	}
	if summary.Provider != "kyverno" {
		t.Errorf("Provider = %q, want kyverno", summary.Provider)
	}
	if len(summary.Policies) != 1 || summary.Policies[0].Enforcement != "enforce" {
		t.Errorf("expected 1 policy with enforcement=enforce, got %+v", summary.Policies)
	}
}

// TestGetPolicySummary_Kyverno_DefaultEnforcementAudit pins the fallback to
// "audit" when validationFailureAction is unset.
func TestGetPolicySummary_Kyverno_DefaultEnforcementAudit(t *testing.T) {
	cl := newFakeClient()
	cl.addResource("clusterpolicies", map[string]interface{}{
		"kind":     "ClusterPolicy",
		"metadata": map[string]interface{}{"name": "bare"},
		"spec": map[string]interface{}{
			"rules": []interface{}{},
		},
	})
	s := NewScanner(cl)
	summary, err := s.GetPolicySummary(context.Background())
	if err != nil {
		t.Fatalf("GetPolicySummary: %v", err)
	}
	if summary.Policies[0].Enforcement != "audit" {
		t.Errorf("default enforcement = %q, want audit", summary.Policies[0].Enforcement)
	}
}

// TestGetPolicySummary_NoneWhenBothAbsent returns provider=none.
func TestGetPolicySummary_NoneWhenBothAbsent(t *testing.T) {
	cl := newFakeClient()
	s := NewScanner(cl)
	summary, err := s.GetPolicySummary(context.Background())
	if err != nil {
		t.Fatalf("GetPolicySummary: %v", err)
	}
	if summary.Provider != "none" {
		t.Errorf("Provider = %q, want none", summary.Provider)
	}
}

// ---------------------------------------------------------------------------
// GetNetworkPolicyAnalysis
// ---------------------------------------------------------------------------

// TestGetNetworkPolicyAnalysis_MatchesPodByLabel verifies a NetworkPolicy
// whose podSelector matches the pod's labels appears under IngressPolicies.
func TestGetNetworkPolicyAnalysis_MatchesPodByLabel(t *testing.T) {
	cl := newFakeClient()
	addNamed(cl, "pods", map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "web",
			"namespace": "app",
			"labels":    map[string]interface{}{"role": "web"},
		},
	})
	addNamed(cl, "networkpolicies", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "web-policy", "namespace": "app"},
		"spec": map[string]interface{}{
			"podSelector": map[string]interface{}{
				"matchLabels": map[string]interface{}{"role": "web"},
			},
			"policyTypes": []string{"Ingress"},
		},
	})

	s := NewScanner(cl)
	analysis, err := s.GetNetworkPolicyAnalysis(context.Background(), "app", "web")
	if err != nil {
		t.Fatalf("GetNetworkPolicyAnalysis: %v", err)
	}
	if !analysis.HasPolicies {
		t.Error("expected HasPolicies=true when a netpol matches the pod")
	}
	if len(analysis.IngressPolicies) != 1 {
		t.Errorf("expected 1 ingress policy, got %d", len(analysis.IngressPolicies))
	}
}

// TestGetNetworkPolicyAnalysis_PolicyDoesNotMatch ensures a non-matching
// policy does NOT appear in the analysis.
func TestGetNetworkPolicyAnalysis_PolicyDoesNotMatch(t *testing.T) {
	cl := newFakeClient()
	addNamed(cl, "pods", map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "web",
			"namespace": "app",
			"labels":    map[string]interface{}{"role": "web"},
		},
	})
	addNamed(cl, "networkpolicies", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "db-policy", "namespace": "app"},
		"spec": map[string]interface{}{
			"podSelector": map[string]interface{}{
				"matchLabels": map[string]interface{}{"role": "db"},
			},
			"policyTypes": []string{"Ingress"},
		},
	})

	s := NewScanner(cl)
	analysis, err := s.GetNetworkPolicyAnalysis(context.Background(), "app", "web")
	if err != nil {
		t.Fatalf("GetNetworkPolicyAnalysis: %v", err)
	}
	if analysis.HasPolicies {
		t.Error("HasPolicies should be false when no policy matches the pod")
	}
}

// TestGetNetworkPolicyAnalysis_PodNotFound surfaces Get error.
func TestGetNetworkPolicyAnalysis_PodNotFound(t *testing.T) {
	cl := newFakeClient()
	s := NewScanner(cl)
	_, err := s.GetNetworkPolicyAnalysis(context.Background(), "app", "ghost")
	if err == nil {
		t.Error("expected error when pod does not exist")
	}
}

// TestGetNetworkPolicyAnalysis_EgressDirection verifies that a policy with
// policyTypes starting with "Egress" is routed to EgressPolicies.
func TestGetNetworkPolicyAnalysis_EgressDirection(t *testing.T) {
	cl := newFakeClient()
	addNamed(cl, "pods", map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "web", "namespace": "app",
			"labels": map[string]interface{}{"role": "web"},
		},
	})
	addNamed(cl, "networkpolicies", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "deny-egress", "namespace": "app"},
		"spec": map[string]interface{}{
			"podSelector": map[string]interface{}{
				"matchLabels": map[string]interface{}{"role": "web"},
			},
			"policyTypes": []string{"Egress"},
		},
	})

	s := NewScanner(cl)
	analysis, err := s.GetNetworkPolicyAnalysis(context.Background(), "app", "web")
	if err != nil {
		t.Fatalf("GetNetworkPolicyAnalysis: %v", err)
	}
	if len(analysis.EgressPolicies) != 1 {
		t.Errorf("expected 1 egress policy, got %d", len(analysis.EgressPolicies))
	}
}

// ---------------------------------------------------------------------------
// scanNetworkPolicyIssues — severity pin (High, not Medium)
// ---------------------------------------------------------------------------

// TestScanNetworkPolicyIssues_NoPolicyIsHigh pins the internal-scanner
// severity for missing network policies. This is the explicit DIVERGENCE
// from app_security.scanNetworkPolicyGaps which flags Medium.
func TestScanNetworkPolicyIssues_NoPolicyIsHigh(t *testing.T) {
	cl := newFakeClient()
	addNamed(cl, "namespaces", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "prod"},
	})

	s := NewScanner(cl)
	issues, err := s.scanNetworkPolicyIssues(context.Background(), "")
	if err != nil {
		t.Fatalf("scanNetworkPolicyIssues: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for empty-netpol ns, got %d", len(issues))
	}
	if issues[0].Severity != SeverityHigh {
		t.Errorf("DIVERGENCE PIN: internal/security flags missing netpols High; got %q", issues[0].Severity)
	}
	if issues[0].Category != CategoryNetwork {
		t.Errorf("Category = %q, want Network", issues[0].Category)
	}
}

// TestScanNetworkPolicyIssues_SystemNamespacesExempt mirrors the app-side
// exemption rule. kube-system, kube-public and kube-node-lease must not be
// flagged.
func TestScanNetworkPolicyIssues_SystemNamespacesExempt(t *testing.T) {
	cl := newFakeClient()
	for _, ns := range []string{"kube-system", "kube-public", "kube-node-lease", "prod"} {
		addNamed(cl, "namespaces", map[string]interface{}{
			"metadata": map[string]interface{}{"name": ns},
		})
	}
	s := NewScanner(cl)
	issues, err := s.scanNetworkPolicyIssues(context.Background(), "")
	if err != nil {
		t.Fatalf("scanNetworkPolicyIssues: %v", err)
	}
	var flagged []string
	for _, i := range issues {
		flagged = append(flagged, i.Namespace)
	}
	for _, f := range flagged {
		switch f {
		case "kube-system", "kube-public", "kube-node-lease":
			t.Errorf("%q must be exempt", f)
		}
	}
	if len(flagged) != 1 || flagged[0] != "prod" {
		t.Errorf("expected only 'prod' flagged, got %v", flagged)
	}
}

// ---------------------------------------------------------------------------
// analyzePodSecurity — invalid JSON paths exercised via AnalyzeRBAC dispatch
// ---------------------------------------------------------------------------

// TestAnalyzeRBAC_InvalidCRBJSON_Skipped ensures corrupt ClusterRoleBindings
// don't break analysis.
func TestAnalyzeRBAC_InvalidCRBJSON_Skipped(t *testing.T) {
	cl := newFakeClient()
	cl.resources["clusterrolebindings"] = []client.Resource{
		{Kind: "clusterrolebindings", Raw: []byte("not-json")},
	}
	s := NewScanner(cl)
	analysis, err := s.AnalyzeRBAC(context.Background())
	if err != nil {
		t.Fatalf("AnalyzeRBAC: %v", err)
	}
	if len(analysis.Bindings) != 0 {
		t.Errorf("malformed CRB must be skipped, got %d bindings", len(analysis.Bindings))
	}
}

// TestAnalyzeRBAC_InvalidRBJSON_Skipped mirrors the above for RoleBindings.
func TestAnalyzeRBAC_InvalidRBJSON_Skipped(t *testing.T) {
	cl := newFakeClient()
	cl.resources["rolebindings"] = []client.Resource{
		{Kind: "rolebindings", Raw: []byte("not-json")},
	}
	s := NewScanner(cl)
	_, err := s.AnalyzeRBAC(context.Background())
	if err != nil {
		t.Fatalf("AnalyzeRBAC: %v", err)
	}
}

// TestAnalyzeRBAC_CRBListErrorReturnsEmpty covers the graceful-error branch.
func TestAnalyzeRBAC_CRBListErrorReturnsEmpty(t *testing.T) {
	// For this test we need a List that errors; re-use the existing fake
	// but override via an intercepting wrapper.
	cl := &listErrClient{
		fakeClusterClient: newFakeClient(),
		listErr:           map[string]error{"clusterrolebindings": errors.New("denied")},
	}

	s := NewScanner(cl)
	analysis, err := s.AnalyzeRBAC(context.Background())
	if err != nil {
		t.Fatalf("AnalyzeRBAC should not error when CRB list fails, got %v", err)
	}
	if len(analysis.Bindings) != 0 {
		t.Errorf("expected no bindings when CRB list errors, got %d", len(analysis.Bindings))
	}
}

// listErrClient wraps fakeClusterClient to return errors for specific kinds.
type listErrClient struct {
	*fakeClusterClient
	listErr map[string]error
}

func (l *listErrClient) List(ctx context.Context, kind string, opts client.ListOptions) (*client.ResourceList, error) {
	if l.listErr == nil {
		l.listErr = map[string]error{}
	}
	if err, ok := l.listErr[kind]; ok {
		return nil, err
	}
	return l.fakeClusterClient.List(ctx, kind, opts)
}

// ---------------------------------------------------------------------------
// sortBySeverity
// ---------------------------------------------------------------------------

// TestSortBySeverity_Order pins the severity order: Critical→High→Medium→Low→Info.
func TestSortBySeverity_Order(t *testing.T) {
	in := []SecurityIssue{
		{Severity: SeverityLow},
		{Severity: SeverityCritical},
		{Severity: SeverityMedium},
		{Severity: SeverityInfo},
		{Severity: SeverityHigh},
	}
	got := sortBySeverity(in)
	want := []Severity{
		SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo,
	}
	for i, w := range want {
		if got[i].Severity != w {
			t.Errorf("got[%d] = %q, want %q", i, got[i].Severity, w)
		}
	}
}

// TestAppendUnique_PreservesOrderAndDedupes pins the helper's behavior.
func TestAppendUnique_PreservesOrderAndDedupes(t *testing.T) {
	got := appendUnique(nil, "a")
	got = appendUnique(got, "b")
	got = appendUnique(got, "a") // dup
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("appendUnique = %v, want [a b]", got)
	}
}

// TestContainsString pins the tiny helper.
func TestContainsString(t *testing.T) {
	if !containsString([]string{"a", "b"}, "a") {
		t.Error("expected true for present")
	}
	if containsString([]string{"a", "b"}, "c") {
		t.Error("expected false for absent")
	}
	if containsString(nil, "a") {
		t.Error("expected false for nil slice")
	}
}

// TestMatchLabels_EmptySelector_MatchesAll pins the loose-selector semantics.
func TestMatchLabels_EmptySelector_MatchesAll(t *testing.T) {
	if !matchLabels(nil, map[string]string{"app": "x"}) {
		t.Error("empty selector must match any labels")
	}
	if !matchLabels(map[string]string{}, map[string]string{"app": "x"}) {
		t.Error("empty selector (empty map) must match any labels")
	}
}

// TestMatchLabels_Mismatch returns false when any selector key fails to match.
func TestMatchLabels_Mismatch(t *testing.T) {
	if matchLabels(map[string]string{"a": "1"}, map[string]string{"a": "2"}) {
		t.Error("value mismatch should return false")
	}
	if matchLabels(map[string]string{"missing": "x"}, map[string]string{"a": "1"}) {
		t.Error("missing label in targets should return false")
	}
}

// ---------------------------------------------------------------------------
// Score categorisation
// ---------------------------------------------------------------------------

// TestCalculateScore_CategoriesPopulated pins that category scores appear in
// the returned score with a 10-point deduction each.
func TestCalculateScore_CategoriesPopulated(t *testing.T) {
	cl := newFakeClient()
	s := NewScanner(cl)
	summary := &SecuritySummary{
		IssuesByCategory: map[Category]int{
			CategoryRBAC:    1,
			CategoryRuntime: 3,
		},
	}
	score := s.calculateScore(summary)
	if score.Categories[string(CategoryRBAC)] != 90 {
		t.Errorf("RBAC category = %d, want 90", score.Categories[string(CategoryRBAC)])
	}
	if score.Categories[string(CategoryRuntime)] != 70 {
		t.Errorf("Runtime category = %d, want 70", score.Categories[string(CategoryRuntime)])
	}
}

// TestCalculateScore_CategoryFloor pins that category score never goes below 0.
func TestCalculateScore_CategoryFloor(t *testing.T) {
	cl := newFakeClient()
	s := NewScanner(cl)
	summary := &SecuritySummary{
		IssuesByCategory: map[Category]int{CategoryRBAC: 100},
	}
	score := s.calculateScore(summary)
	if score.Categories[string(CategoryRBAC)] < 0 {
		t.Errorf("RBAC category = %d, must not be negative", score.Categories[string(CategoryRBAC)])
	}
}

// ---------------------------------------------------------------------------
// Summary: TopIssues truncation
// ---------------------------------------------------------------------------

// TestGetSecuritySummary_TopIssuesTruncatedAt10 seeds 12 privileged pods and
// asserts TopIssues is capped at 10.
func TestGetSecuritySummary_TopIssuesTruncatedAt10(t *testing.T) {
	cl := newFakeClient()
	priv := true
	for i := 0; i < 12; i++ {
		addNamed(cl, "pods", map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "p" + string(rune('a'+i)),
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":            "c",
						"securityContext": map[string]interface{}{"privileged": &priv},
					},
				},
			},
		})
	}
	s := NewScanner(cl)
	summary, err := s.GetSecuritySummary(context.Background(), "default")
	if err != nil {
		t.Fatalf("GetSecuritySummary: %v", err)
	}
	if summary.TotalIssues < 12 {
		t.Errorf("TotalIssues = %d, expected >= 12", summary.TotalIssues)
	}
	if len(summary.TopIssues) != 10 {
		t.Errorf("TopIssues = %d, want 10 (truncation)", len(summary.TopIssues))
	}
}
