// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

// TestCheckDangerousAccess_WildcardOnAll verifies that a non-system subject
// with verb="*" on resource="*" is flagged via the cluster-admin-equivalent
// path and includes a meaningful reason.
func TestCheckDangerousAccess_WildcardOnAll(t *testing.T) {
	a := &App{}
	summary := &RBACSummary{}
	binding := &RBACBinding{
		Name:     "alice-cluster-admin",
		RoleName: "cluster-admin",
		Subjects: []RBACSubject{{Kind: "User", Name: "alice"}},
	}
	a.checkDangerousAccess(summary, binding)

	if len(summary.DangerousAccess) == 0 {
		t.Fatalf("expected cluster-admin subject to be flagged")
	}
	if !strings.Contains(summary.DangerousAccess[0].Reason, "cluster-admin") {
		t.Errorf("reason should mention cluster-admin, got %q", summary.DangerousAccess[0].Reason)
	}
}

// TestCheckDangerousAccess_SecretsReadAccess verifies that read verbs on the
// secrets resource are flagged even though get/list themselves are not in the
// dangerous-verbs table. This is the previously-dead code path we wired up.
func TestCheckDangerousAccess_SecretsReadAccess(t *testing.T) {
	a := &App{}
	summary := &RBACSummary{}
	binding := &RBACBinding{
		Name:     "bob-secret-reader",
		RoleName: "secret-reader",
		Subjects: []RBACSubject{{Kind: "User", Name: "bob"}},
		Permissions: []RBACPermission{{
			Verbs:     []string{"get", "list"},
			Resources: []string{"secrets"},
		}},
	}
	a.checkDangerousAccess(summary, binding)

	if len(summary.DangerousAccess) < 2 {
		t.Fatalf("expected 2 findings (get, list), got %d", len(summary.DangerousAccess))
	}
	for _, da := range summary.DangerousAccess {
		if !strings.Contains(da.Reason, "secrets") {
			t.Errorf("reason should mention secrets, got %q", da.Reason)
		}
	}
}

// TestCheckDangerousAccess_DangerousVerbReasonIncluded verifies that when a
// subject has `delete` on a non-wildcard resource that is also dangerous
// (e.g. clusterrolebindings), the reason string from the dangerousVerbs map
// is surfaced — this is the code path that previously did `_ = reason`.
func TestCheckDangerousAccess_DangerousVerbReasonIncluded(t *testing.T) {
	a := &App{}
	summary := &RBACSummary{}
	binding := &RBACBinding{
		Name:     "carol-crb-admin",
		RoleName: "crb-admin",
		Subjects: []RBACSubject{{Kind: "User", Name: "carol"}},
		Permissions: []RBACPermission{{
			Verbs:     []string{"delete"},
			Resources: []string{"clusterrolebindings"},
		}},
	}
	a.checkDangerousAccess(summary, binding)

	if len(summary.DangerousAccess) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(summary.DangerousAccess))
	}
	reason := summary.DangerousAccess[0].Reason
	// Both the verb description ("can delete resources") and the resource
	// description ("can manage cluster role bindings") should appear.
	if !strings.Contains(reason, "delete") || !strings.Contains(reason, "cluster role bindings") {
		t.Errorf("reason should surface both verb and resource descriptions, got %q", reason)
	}
}

// TestCheckDangerousAccess_SkipsSystemMasters confirms the system:masters
// group is exempt from cluster-admin findings (it's expected to have it).
func TestCheckDangerousAccess_SkipsSystemMasters(t *testing.T) {
	a := &App{}
	summary := &RBACSummary{}
	binding := &RBACBinding{
		Name:     "cluster-admin",
		RoleName: "cluster-admin",
		Subjects: []RBACSubject{{Kind: "Group", Name: "system:masters"}},
	}
	a.checkDangerousAccess(summary, binding)

	if len(summary.DangerousAccess) != 0 {
		t.Errorf("system:masters should not be flagged, got %d findings", len(summary.DangerousAccess))
	}
}
