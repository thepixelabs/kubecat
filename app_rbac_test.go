// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"testing"

	"github.com/thepixelabs/kubecat/internal/client"
	"github.com/thepixelabs/kubecat/internal/rbac"
)

// roleBindingObj builds a minimal RoleBinding object.
func roleBindingObj(name, namespace, roleKind, roleName string, subjects []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{"name": name, "namespace": namespace},
		"roleRef":  map[string]interface{}{"kind": roleKind, "name": roleName},
		"subjects": subjects,
	}
}

// clusterRoleBindingObj builds a minimal ClusterRoleBinding object.
func clusterRoleBindingObj(name, roleKind, roleName string, subjects []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{"name": name},
		"roleRef":  map[string]interface{}{"kind": roleKind, "name": roleName},
		"subjects": subjects,
	}
}

// roleObj builds a Role (or ClusterRole — structure matches) with flat rules.
func roleObj(name, namespace string, rules []map[string]interface{}) map[string]interface{} {
	md := map[string]interface{}{"name": name}
	if namespace != "" {
		md["namespace"] = namespace
	}
	return map[string]interface{}{
		"metadata": md,
		"rules":    rules,
	}
}

// TestListNamespaceRBAC_RoleBindingDereferencesRole wires a RoleBinding to a
// Role and asserts the rule set is flattened onto the subject.
func TestListNamespaceRBAC_RoleBindingDereferencesRole(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("rolebindings", roleBindingObj(
		"viewer-binding", "team-a", "Role", "viewer",
		[]map[string]interface{}{
			{"kind": "User", "name": "alice"},
		},
	))
	cl.addResource("roles", roleObj("viewer", "team-a", []map[string]interface{}{
		{"verbs": []string{"get", "list"}, "resources": []string{"pods"}, "apiGroups": []string{""}},
	}))

	matrix, err := rbac.ListNamespaceRBAC(context.Background(), cl, "team-a")
	if err != nil {
		t.Fatalf("ListNamespaceRBAC: %v", err)
	}
	if len(matrix.Subjects) != 1 {
		t.Fatalf("expected 1 subject, got %d: %+v", len(matrix.Subjects), matrix.Subjects)
	}
	sub := matrix.Subjects[0]
	if sub.Subject != "alice" || sub.Kind != rbac.SubjectUser {
		t.Errorf("subject = %+v, want alice/User", sub)
	}
	if len(sub.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(sub.Rules))
	}
	if sub.Rules[0].IsWildcard {
		t.Error("rule should not be wildcard")
	}
}

// TestListNamespaceRBAC_ClusterRoleBindingEnumerated verifies a
// ClusterRoleBinding's subjects show up alongside RoleBinding subjects.
func TestListNamespaceRBAC_ClusterRoleBindingEnumerated(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("clusterrolebindings", clusterRoleBindingObj(
		"cluster-admins", "ClusterRole", "cluster-admin",
		[]map[string]interface{}{
			{"kind": "Group", "name": "admins"},
			{"kind": "ServiceAccount", "name": "robot", "namespace": "ops"},
		},
	))
	cl.addResource("clusterroles", roleObj("cluster-admin", "", []map[string]interface{}{
		{"verbs": []string{"*"}, "resources": []string{"*"}, "apiGroups": []string{"*"}},
	}))

	matrix, err := rbac.ListNamespaceRBAC(context.Background(), cl, "team-a")
	if err != nil {
		t.Fatalf("ListNamespaceRBAC: %v", err)
	}
	if len(matrix.Subjects) != 2 {
		t.Fatalf("expected 2 subjects (Group, ServiceAccount), got %d", len(matrix.Subjects))
	}
	// Every subject inherits the wildcard rule.
	for _, s := range matrix.Subjects {
		if len(s.Rules) == 0 || !s.Rules[0].IsWildcard {
			t.Errorf("subject %s/%s should inherit wildcard rule; got rules=%+v", s.Kind, s.Subject, s.Rules)
		}
	}
}

// TestListNamespaceRBAC_DeduplicatesIdenticalRules verifies that if a
// subject has two bindings to roles with identical rule sets, the rule
// only appears once in the flattened output.
func TestListNamespaceRBAC_DeduplicatesIdenticalRules(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("rolebindings", roleBindingObj(
		"rb-a", "t", "Role", "role-a",
		[]map[string]interface{}{{"kind": "User", "name": "alice"}},
	))
	cl.addResource("rolebindings", roleBindingObj(
		"rb-b", "t", "Role", "role-b",
		[]map[string]interface{}{{"kind": "User", "name": "alice"}},
	))
	rule := []map[string]interface{}{
		{"verbs": []string{"get", "list"}, "resources": []string{"pods"}, "apiGroups": []string{""}},
	}
	cl.addResource("roles", roleObj("role-a", "t", rule))
	cl.addResource("roles", roleObj("role-b", "t", rule))

	matrix, err := rbac.ListNamespaceRBAC(context.Background(), cl, "t")
	if err != nil {
		t.Fatalf("ListNamespaceRBAC: %v", err)
	}
	if len(matrix.Subjects) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(matrix.Subjects))
	}
	if len(matrix.Subjects[0].Rules) != 1 {
		t.Errorf("expected 1 deduped rule, got %d: %+v",
			len(matrix.Subjects[0].Rules), matrix.Subjects[0].Rules)
	}
	// And two bindings recorded.
	if len(matrix.Subjects[0].Bindings) != 2 {
		t.Errorf("expected 2 bindings recorded, got %d", len(matrix.Subjects[0].Bindings))
	}
}

// errorFakeClient wraps fakeClusterClient and returns an error for
// clusterrolebindings listing. Used to exercise the warning-on-denied path.
type errorFakeClient struct {
	*fakeClusterClient
}

func (e *errorFakeClient) List(ctx context.Context, kind string, opts client.ListOptions) (*client.ResourceList, error) {
	if kind == "clusterrolebindings" {
		return nil, errors.New("forbidden")
	}
	return e.fakeClusterClient.List(ctx, kind, opts)
}

// TestListNamespaceRBAC_ClusterRoleBindingsDenied_SetsWarning pins that a
// denied ClusterRoleBindings list produces a warning but does not fail.
func TestListNamespaceRBAC_ClusterRoleBindingsDenied_SetsWarning(t *testing.T) {
	cl := &errorFakeClient{fakeClusterClient: newFakeClusterClient()}
	cl.addResource("rolebindings", roleBindingObj(
		"rb", "ns", "Role", "r",
		[]map[string]interface{}{{"kind": "User", "name": "alice"}},
	))

	matrix, err := rbac.ListNamespaceRBAC(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("ListNamespaceRBAC must not return error on denied CRB list, got %v", err)
	}
	if matrix.Warning == "" {
		t.Error("expected a warning when ClusterRoleBinding list denied")
	}
}

// TestGetNamespaceRBAC_UsesActiveWhenContextUnknown exercises the
// App.GetNamespaceRBAC fallback path (Get → Active).
func TestGetNamespaceRBAC_UsesActiveWhenContextUnknown(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("rolebindings", roleBindingObj(
		"rb", "ns", "Role", "r",
		[]map[string]interface{}{{"kind": "User", "name": "alice"}},
	))
	cl.addResource("roles", roleObj("r", "ns", []map[string]interface{}{
		{"verbs": []string{"get"}, "resources": []string{"pods"}, "apiGroups": []string{""}},
	}))
	a := newAppWithFakes(cl)

	// Passing any context name falls through to Active() since our fake
	// returns the same client for either.
	matrix, err := a.GetNamespaceRBAC("unknown-ctx", "ns")
	if err != nil {
		t.Fatalf("GetNamespaceRBAC: %v", err)
	}
	if len(matrix.Subjects) != 1 {
		t.Errorf("expected 1 subject, got %d", len(matrix.Subjects))
	}
}
