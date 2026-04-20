// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"strings"
	"testing"

	"github.com/thepixelabs/kubecat/internal/client"
)

// ---------------------------------------------------------------------------
// checkDangerousAccess — additional behavior pinned
// ---------------------------------------------------------------------------

// TestCheckDangerousAccess_WildcardVerbAndResource_FlaggedAsClusterAdminEquivalent
// verifies that a non-privileged role granting verbs=["*"] resources=["*"]
// is flagged. This is an "effective cluster-admin" — the implementation
// catches it via the dangerousVerbs["*"]+dangerousResources["*"] path,
// not the short-circuit, so it still emits one finding per (subject,verb,res).
func TestCheckDangerousAccess_WildcardVerbAndResource_FlaggedAsClusterAdminEquivalent(t *testing.T) {
	a := &App{}
	summary := &RBACSummary{}
	binding := &RBACBinding{
		Name:     "super-role",
		RoleName: "super-role", // NOT cluster-admin -> short-circuit skipped
		Subjects: []RBACSubject{{Kind: "User", Name: "eve"}},
		Permissions: []RBACPermission{{
			Verbs:     []string{"*"},
			Resources: []string{"*"},
		}},
	}
	a.checkDangerousAccess(summary, binding)

	if len(summary.DangerousAccess) == 0 {
		t.Fatalf("verbs=* resources=* must produce a dangerous access finding")
	}
	reason := summary.DangerousAccess[0].Reason
	if !strings.Contains(reason, "wildcard") && !strings.Contains(reason, "all") {
		t.Errorf("reason should mention wildcard/all-access, got %q", reason)
	}
}

// TestCheckDangerousAccess_DangerousVerbs_EachFlaggedIndividually pins that
// each of the specific dangerous verbs (impersonate, escalate, bind) is
// flagged when paired with a dangerous resource.
func TestCheckDangerousAccess_DangerousVerbs_EachFlaggedIndividually(t *testing.T) {
	cases := []struct {
		verb     string
		resource string
	}{
		{"impersonate", "users"},        // users is not in dangerousResources, so must pair with wildcard
		{"escalate", "roles"},           // can escalate + can manage roles
		{"bind", "clusterrolebindings"}, // can bind + manage cluster role bindings
	}
	// For verbs where resource isn't in dangerousResources, we need wildcard
	// resource to trip the flag.
	for _, tc := range cases {
		t.Run(tc.verb+"_"+tc.resource, func(t *testing.T) {
			a := &App{}
			summary := &RBACSummary{}
			res := tc.resource
			if _, ok := dangerousResources[res]; !ok {
				res = "*"
			}
			binding := &RBACBinding{
				Name:     "r-" + tc.verb,
				RoleName: "r-" + tc.verb,
				Subjects: []RBACSubject{{Kind: "User", Name: "alice"}},
				Permissions: []RBACPermission{{
					Verbs:     []string{tc.verb},
					Resources: []string{res},
				}},
			}
			a.checkDangerousAccess(summary, binding)
			if len(summary.DangerousAccess) == 0 {
				t.Fatalf("verb %q on %q should be flagged", tc.verb, res)
			}
			if !strings.Contains(summary.DangerousAccess[0].Reason, dangerousVerbs[tc.verb]) {
				t.Errorf("reason should include verb description %q, got %q",
					dangerousVerbs[tc.verb], summary.DangerousAccess[0].Reason)
			}
		})
	}
}

// TestCheckDangerousAccess_PodAttachPortforwardToken pins the flag behavior
// for pods/attach, pods/portforward, serviceaccounts/token (read on
// sensitive resource → flagged).
func TestCheckDangerousAccess_PodAttachPortforwardToken(t *testing.T) {
	cases := []struct {
		verb     string
		resource string
		keyword  string
	}{
		{"get", "pods/attach", "pods/attach"},
		{"get", "pods/portforward", "pods/portforward"},
		{"create", "serviceaccounts/token", "service account tokens"},
	}
	for _, tc := range cases {
		t.Run(tc.resource+"/"+tc.verb, func(t *testing.T) {
			a := &App{}
			summary := &RBACSummary{}
			binding := &RBACBinding{
				Name:     "r-" + tc.resource,
				RoleName: "r-" + tc.resource,
				Subjects: []RBACSubject{{Kind: "User", Name: "alice"}},
				Permissions: []RBACPermission{{
					Verbs:     []string{tc.verb},
					Resources: []string{tc.resource},
				}},
			}
			a.checkDangerousAccess(summary, binding)

			if len(summary.DangerousAccess) == 0 {
				t.Fatalf("%s %s should be flagged", tc.verb, tc.resource)
			}
			reason := summary.DangerousAccess[0].Reason
			if !strings.Contains(reason, tc.keyword) {
				t.Errorf("reason should mention %q, got %q", tc.keyword, reason)
			}
		})
	}
}

// TestCheckDangerousAccess_SecretsWatchFlagged verifies that `watch` on
// secrets is also treated as a read-of-sensitive and flagged.
func TestCheckDangerousAccess_SecretsWatchFlagged(t *testing.T) {
	a := &App{}
	summary := &RBACSummary{}
	binding := &RBACBinding{
		Name:     "watcher",
		RoleName: "watcher",
		Subjects: []RBACSubject{{Kind: "User", Name: "watch-user"}},
		Permissions: []RBACPermission{{
			Verbs:     []string{"watch"},
			Resources: []string{"secrets"},
		}},
	}
	a.checkDangerousAccess(summary, binding)

	if len(summary.DangerousAccess) != 1 {
		t.Fatalf("expected 1 finding for watch on secrets, got %d", len(summary.DangerousAccess))
	}
	if !strings.Contains(summary.DangerousAccess[0].Reason, "secrets") {
		t.Errorf("reason must mention secrets, got %q", summary.DangerousAccess[0].Reason)
	}
}

// TestCheckDangerousAccess_ClusterAdminEmitsOneFindingPerSubject pins the
// short-circuit: three subjects in a cluster-admin binding → three findings,
// not 50 (one per verb×resource tuple).
func TestCheckDangerousAccess_ClusterAdminEmitsOneFindingPerSubject(t *testing.T) {
	a := &App{}
	summary := &RBACSummary{}
	binding := &RBACBinding{
		Name:     "multi-cluster-admin",
		RoleName: "cluster-admin",
		Subjects: []RBACSubject{
			{Kind: "User", Name: "alice"},
			{Kind: "User", Name: "bob"},
			{Kind: "ServiceAccount", Namespace: "default", Name: "robot"},
		},
	}
	a.checkDangerousAccess(summary, binding)

	if len(summary.DangerousAccess) != 3 {
		t.Fatalf("expected exactly 3 findings (one per subject), got %d",
			len(summary.DangerousAccess))
	}
	for _, da := range summary.DangerousAccess {
		if len(da.Permissions) != 1 || da.Permissions[0] != "cluster-admin" {
			t.Errorf("cluster-admin finding must use Permissions=[cluster-admin], got %v",
				da.Permissions)
		}
	}
}

// TestCheckDangerousAccess_DeduplicatesSameVerbResource guarantees that when
// a role has overlapping rules emitting the same (verb, resource) for the
// same subject, only one finding is produced.
func TestCheckDangerousAccess_DeduplicatesSameVerbResource(t *testing.T) {
	a := &App{}
	summary := &RBACSummary{}
	binding := &RBACBinding{
		Name:     "dup-role",
		RoleName: "dup-role",
		Subjects: []RBACSubject{{Kind: "User", Name: "alice"}},
		Permissions: []RBACPermission{
			{Verbs: []string{"delete"}, Resources: []string{"secrets"}},
			{Verbs: []string{"delete"}, Resources: []string{"secrets"}}, // dup
			{Verbs: []string{"delete"}, Resources: []string{"secrets"}}, // dup
		},
	}
	a.checkDangerousAccess(summary, binding)

	if len(summary.DangerousAccess) != 1 {
		t.Fatalf("expected 1 de-duped finding, got %d", len(summary.DangerousAccess))
	}
}

// TestCheckDangerousAccess_SkipsSystemPrefixedSubjects verifies that a
// service account in kube-system with a system-prefixed binding is exempt.
func TestCheckDangerousAccess_SkipsSystemPrefixedSubjects(t *testing.T) {
	a := &App{}
	summary := &RBACSummary{}
	binding := &RBACBinding{
		Name:     "system:node",
		RoleName: "cluster-admin",
		Subjects: []RBACSubject{
			{Kind: "ServiceAccount", Namespace: "kube-system", Name: "system:node"},
		},
	}
	a.checkDangerousAccess(summary, binding)

	if len(summary.DangerousAccess) != 0 {
		t.Errorf("system:node in kube-system must be exempt, got %d findings",
			len(summary.DangerousAccess))
	}
}

// TestIsSystemSubject pins the system-subject identification rules.
func TestIsSystemSubject(t *testing.T) {
	a := &App{}
	cases := []struct {
		subject RBACSubject
		want    bool
	}{
		{RBACSubject{Kind: "User", Name: "system:anonymous"}, true},
		{RBACSubject{Kind: "User", Name: "kube-scheduler"}, true},
		{RBACSubject{Kind: "ServiceAccount", Namespace: "kube-system", Name: "default"}, true},
		{RBACSubject{Kind: "ServiceAccount", Namespace: "kube-public", Name: "default"}, true},
		{RBACSubject{Kind: "ServiceAccount", Namespace: "kube-node-lease", Name: "default"}, true},
		{RBACSubject{Kind: "User", Name: "alice"}, false},
		{RBACSubject{Kind: "ServiceAccount", Namespace: "default", Name: "robot"}, false},
	}
	for _, tc := range cases {
		got := a.isSystemSubject(tc.subject)
		if got != tc.want {
			t.Errorf("isSystemSubject(%+v) = %v, want %v", tc.subject, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// scanPodSecurityIssues — pin current behavior, document gaps
// ---------------------------------------------------------------------------

// TestScanPodSecurityIssues_HostNetworkFlagged ensures runtime.hostnetwork
// is flagged High.
func TestScanPodSecurityIssues_HostNetworkFlagged(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "hn", "namespace": "default"},
		"spec": map[string]interface{}{
			"hostNetwork": true,
			"containers":  []interface{}{},
		},
	})
	pod := cl.resources["pods"][0]
	issues := scanPodSecurityIssues(pod)
	found := false
	for _, i := range issues {
		if i.Severity == "High" && strings.Contains(strings.ToLower(i.Title), "host network") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected High/hostNetwork issue, got %+v", issues)
	}
}

// TestScanPodSecurityIssues_HostPIDFlagged mirrors the hostNetwork test.
func TestScanPodSecurityIssues_HostPIDFlagged(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "hp", "namespace": "default"},
		"spec": map[string]interface{}{
			"hostPID":    true,
			"containers": []interface{}{},
		},
	})
	pod := cl.resources["pods"][0]
	issues := scanPodSecurityIssues(pod)
	if len(issues) == 0 || issues[0].Severity != "High" {
		t.Errorf("expected High hostPID issue, got %+v", issues)
	}
}

// TestScanPodSecurityIssues_PrivilegedCriticalAndRootHigh verifies a
// container with both privileged=true and runAsUser=0 emits both findings.
func TestScanPodSecurityIssues_PrivilegedCriticalAndRootHigh(t *testing.T) {
	cl := newFakeClusterClient()
	priv := true
	uid := int64(0)
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "pr", "namespace": "default"},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name": "c",
					"securityContext": map[string]interface{}{
						"privileged": &priv,
						"runAsUser":  &uid,
					},
				},
			},
		},
	})
	pod := cl.resources["pods"][0]
	issues := scanPodSecurityIssues(pod)

	var sawPriv, sawRoot bool
	for _, i := range issues {
		if i.Severity == "Critical" && strings.Contains(i.Title, "Privileged") {
			sawPriv = true
		}
		if i.Severity == "High" && strings.Contains(i.Title, "root") {
			sawRoot = true
		}
	}
	if !sawPriv {
		t.Errorf("expected Critical privileged finding")
	}
	if !sawRoot {
		t.Errorf("expected High runAsUser=0 finding")
	}
}

// TestScanPodSecurityIssues_MissingChecks_PinCurrentGaps documents pod fields
// that the scanner currently ignores. This test will flip the assertion once
// coverage improves; for now it pins the behavior so a future fix is
// visible in diff review.
func TestScanPodSecurityIssues_MissingChecks_PinCurrentGaps(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "gap", "namespace": "default"},
		"spec": map[string]interface{}{
			// hostIPC is TODO per architect — currently ignored.
			"hostIPC": true,
			// volumes with hostPath — currently ignored
			"volumes": []interface{}{
				map[string]interface{}{
					"name":     "h",
					"hostPath": map[string]interface{}{"path": "/"},
				},
			},
			"containers": []interface{}{
				map[string]interface{}{
					"name": "c",
					// allowPrivilegeEscalation unset (default → true semantically)
					// capabilities.add ignored
					"securityContext": map[string]interface{}{
						"capabilities": map[string]interface{}{
							"add": []interface{}{"NET_ADMIN", "SYS_ADMIN"},
						},
					},
				},
			},
		},
	})
	pod := cl.resources["pods"][0]
	issues := scanPodSecurityIssues(pod)

	// CURRENT behavior: no findings for any of the above gaps.
	if len(issues) != 0 {
		t.Errorf("pinning current gap behavior: expected 0 findings for hostIPC/hostPath/capabilities, got %d: %+v",
			len(issues), issues)
	}
}

// TestScanPodSecurityIssues_InvalidJSON_NoCrash verifies malformed pod raw
// JSON produces an empty result rather than panicking.
func TestScanPodSecurityIssues_InvalidJSON_NoCrash(t *testing.T) {
	pod := clientResourceWithRaw([]byte("not-json"))
	issues := scanPodSecurityIssues(pod)
	if len(issues) != 0 {
		t.Errorf("invalid JSON should yield 0 issues, got %d", len(issues))
	}
}

// clientResourceWithRaw constructs a bare client.Resource with only Raw set.
func clientResourceWithRaw(raw []byte) client.Resource {
	return client.Resource{Kind: "Pod", Raw: raw}
}

// ---------------------------------------------------------------------------
// scanNetworkPolicyGaps — system namespace exemption
// ---------------------------------------------------------------------------

// TestScanNetworkPolicyGaps_SystemNamespaces_Exempt verifies kube-system,
// kube-public and kube-node-lease are never flagged for missing netpols.
func TestScanNetworkPolicyGaps_SystemNamespaces_Exempt(t *testing.T) {
	cl := newFakeClusterClient()
	for _, ns := range []string{"kube-system", "kube-public", "kube-node-lease", "default", "prod"} {
		cl.addResource("namespaces", map[string]interface{}{
			"metadata": map[string]interface{}{"name": ns},
		})
	}
	// Only "default" has a netpol.
	cl.addResource("networkpolicies", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "d", "namespace": "default"},
	})

	a := &App{}
	issues := a.scanNetworkPolicyGaps(context.Background(), cl, "")

	var flagged []string
	for _, issue := range issues {
		flagged = append(flagged, issue.Namespace)
	}
	for _, system := range []string{"kube-system", "kube-public", "kube-node-lease"} {
		for _, f := range flagged {
			if f == system {
				t.Errorf("%q must be exempt, but was flagged", system)
			}
		}
	}
	// prod has no policies → should be flagged.
	sawProd := false
	for _, f := range flagged {
		if f == "prod" {
			sawProd = true
		}
	}
	if !sawProd {
		t.Error("prod ns without policies must be flagged")
	}
}

// TestScanNetworkPolicyGaps_NamespaceFilter narrows the scan to one ns.
func TestScanNetworkPolicyGaps_NamespaceFilter(t *testing.T) {
	cl := newFakeClusterClient()
	for _, ns := range []string{"a", "b"} {
		cl.addResource("namespaces", map[string]interface{}{
			"metadata": map[string]interface{}{"name": ns},
		})
	}
	a := &App{}
	issues := a.scanNetworkPolicyGaps(context.Background(), cl, "a")
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue restricted to 'a', got %d: %+v", len(issues), issues)
	}
	if issues[0].Namespace != "a" {
		t.Errorf("expected namespace 'a', got %q", issues[0].Namespace)
	}
}

// ---------------------------------------------------------------------------
// GetSecuritySummary end-to-end through fake client
// ---------------------------------------------------------------------------

// TestGetSecuritySummary_EmptyCluster_NoPanic exercises the summary path end
// to end: it must not panic when every list returns empty.
func TestGetSecuritySummary_EmptyCluster_NoPanic(t *testing.T) {
	cl := newFakeClusterClient()
	a := newAppWithFakes(cl)

	summary, err := a.GetSecuritySummary("")
	if err != nil {
		t.Fatalf("GetSecuritySummary: %v", err)
	}
	if summary == nil {
		t.Fatal("summary is nil")
	}
	if summary.Score.Grade == "" {
		t.Error("grade should be set")
	}
}

// TestGetSecuritySummary_PrivilegedPod_BumpsCritical feeds a privileged pod
// and ensures it contributes to CriticalCount.
func TestGetSecuritySummary_PrivilegedPod_BumpsCritical(t *testing.T) {
	cl := newFakeClusterClient()
	priv := true
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "p", "namespace": "default"},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":            "c",
					"securityContext": map[string]interface{}{"privileged": &priv},
				},
			},
		},
	})
	a := newAppWithFakes(cl)

	summary, err := a.GetSecuritySummary("default")
	if err != nil {
		t.Fatalf("GetSecuritySummary: %v", err)
	}
	if summary.CriticalCount == 0 {
		t.Errorf("expected CriticalCount > 0 for privileged pod, got %d", summary.CriticalCount)
	}
}

// TestCalculateSecurityScore_TableDriven pins the score → grade mapping.
func TestCalculateSecurityScore_TableDriven(t *testing.T) {
	cases := []struct {
		name      string
		critical  int
		high      int
		medium    int
		low       int
		wantScore int
		wantGrade string
	}{
		{"no issues", 0, 0, 0, 0, 100, "A"},
		{"one critical", 1, 0, 0, 0, 85, "B"},
		{"one high", 0, 1, 0, 0, 90, "A"},
		{"one medium", 0, 0, 1, 0, 95, "A"},
		{"one low", 0, 0, 0, 1, 98, "A"},
		{"floor at zero", 100, 0, 0, 0, 0, "F"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &SecuritySummaryInfo{
				CriticalCount: tc.critical,
				HighCount:     tc.high,
				MediumCount:   tc.medium,
				LowCount:      tc.low,
			}
			score := calculateSecurityScore(s)
			if score != tc.wantScore {
				t.Errorf("score = %d, want %d", score, tc.wantScore)
			}
			if calculateSecurityGrade(score) != tc.wantGrade {
				t.Errorf("grade = %q, want %q", calculateSecurityGrade(score), tc.wantGrade)
			}
		})
	}
}

// TestSortSecurityIssuesBySeverity verifies the bubble-sort groups severities
// in Critical → High → Medium → Low → Info order.
func TestSortSecurityIssuesBySeverity(t *testing.T) {
	issues := []SecurityIssueInfo{
		{Severity: "Low", Title: "l"},
		{Severity: "Critical", Title: "c"},
		{Severity: "Medium", Title: "m"},
		{Severity: "Info", Title: "i"},
		{Severity: "High", Title: "h"},
	}
	sorted := sortSecurityIssuesBySeverity(issues)
	want := []string{"Critical", "High", "Medium", "Low", "Info"}
	for i, w := range want {
		if sorted[i].Severity != w {
			t.Errorf("sorted[%d] severity = %q, want %q", i, sorted[i].Severity, w)
		}
	}
}
