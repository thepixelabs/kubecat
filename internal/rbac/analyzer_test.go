// SPDX-License-Identifier: Apache-2.0

package rbac

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/thepixelabs/kubecat/internal/client"
)

// fakeClusterClient is a minimal in-memory client.ClusterClient used by RBAC
// analyzer tests. It deliberately does NOT bundle a list-filter for namespace
// (namespace filtering happens at the caller), matching how the real in-cluster
// client treats `rolebindings`: the server applies the namespace filter and
// returns only scoped items. The tests mirror that by only stocking the
// resources we expect the analyzer to consume.
type fakeClusterClient struct {
	resources map[string][]client.Resource
	listErrs  map[string]error
	getErrs   map[string]error
}

func newFakeClient() *fakeClusterClient {
	return &fakeClusterClient{
		resources: make(map[string][]client.Resource),
		listErrs:  make(map[string]error),
		getErrs:   make(map[string]error),
	}
}

func (f *fakeClusterClient) addResource(kind string, obj map[string]interface{}) {
	raw, _ := json.Marshal(obj)
	var name, namespace string
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		name, _ = meta["name"].(string)
		namespace, _ = meta["namespace"].(string)
	}
	f.resources[kind] = append(f.resources[kind], client.Resource{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
		Raw:       raw,
		Object:    obj,
	})
}

func (f *fakeClusterClient) Info(_ context.Context) (*client.ClusterInfo, error) {
	return &client.ClusterInfo{Name: "fake"}, nil
}
func (f *fakeClusterClient) List(_ context.Context, kind string, _ client.ListOptions) (*client.ResourceList, error) {
	if err, ok := f.listErrs[kind]; ok {
		return nil, err
	}
	items := f.resources[kind]
	return &client.ResourceList{Items: items, Total: len(items)}, nil
}
func (f *fakeClusterClient) Get(_ context.Context, kind, namespace, name string) (*client.Resource, error) {
	if err, ok := f.getErrs[kind+"/"+namespace+"/"+name]; ok {
		return nil, err
	}
	for _, r := range f.resources[kind] {
		if r.Name == name && r.Namespace == namespace {
			return &r, nil
		}
	}
	return nil, client.ErrResourceNotFound
}
func (f *fakeClusterClient) Delete(_ context.Context, _, _, _ string) error { return nil }
func (f *fakeClusterClient) Watch(_ context.Context, _ string, _ client.WatchOptions) (<-chan client.WatchEvent, error) {
	ch := make(chan client.WatchEvent)
	close(ch)
	return ch, nil
}
func (f *fakeClusterClient) Logs(_ context.Context, _, _, _ string, _ bool, _ int64) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}
func (f *fakeClusterClient) Exec(_ context.Context, _, _, _ string, _ []string) error { return nil }
func (f *fakeClusterClient) PortForward(_ context.Context, _, _ string, _, _ int) (client.PortForwarder, error) {
	return nil, nil
}
func (f *fakeClusterClient) Close() error { return nil }

// rb is a shorthand for a RoleBinding manifest map.
func rb(name, namespace, roleKind, roleName string, subjects []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{"name": name, "namespace": namespace},
		"roleRef":  map[string]interface{}{"kind": roleKind, "name": roleName},
		"subjects": subjects,
	}
}

// crb is a shorthand for a ClusterRoleBinding manifest map.
func crb(name, roleKind, roleName string, subjects []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{"name": name},
		"roleRef":  map[string]interface{}{"kind": roleKind, "name": roleName},
		"subjects": subjects,
	}
}

// role is a shorthand for a Role or ClusterRole manifest map.
func role(name, namespace string, rules []map[string]interface{}) map[string]interface{} {
	md := map[string]interface{}{"name": name}
	if namespace != "" {
		md["namespace"] = namespace
	}
	return map[string]interface{}{"metadata": md, "rules": rules}
}

// ---------------------------------------------------------------------------
// ListNamespaceRBAC
// ---------------------------------------------------------------------------

// TestListNamespaceRBAC_EmptyNamespace returns zero subjects and no warning.
func TestListNamespaceRBAC_EmptyNamespace(t *testing.T) {
	cl := newFakeClient()
	matrix, err := ListNamespaceRBAC(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("ListNamespaceRBAC: %v", err)
	}
	if len(matrix.Subjects) != 0 {
		t.Errorf("empty cluster: expected 0 subjects, got %d", len(matrix.Subjects))
	}
	if matrix.Warning != "" {
		t.Errorf("no error path hit: expected empty warning, got %q", matrix.Warning)
	}
	if matrix.Namespace != "ns" {
		t.Errorf("Namespace = %q, want ns", matrix.Namespace)
	}
}

// TestListNamespaceRBAC_RoleBindingListErr_SetsWarningAndReturnsEarly exercises
// the early-return path when RoleBindings cannot be listed.
func TestListNamespaceRBAC_RoleBindingListErr_SetsWarningAndReturnsEarly(t *testing.T) {
	cl := newFakeClient()
	cl.listErrs["rolebindings"] = errors.New("forbidden")

	matrix, err := ListNamespaceRBAC(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("ListNamespaceRBAC must not error when RBs are denied, got %v", err)
	}
	if matrix.Warning == "" {
		t.Error("expected warning populated on denied RoleBindings list")
	}
	if len(matrix.Subjects) != 0 {
		t.Errorf("on RB denial, expected 0 subjects, got %d", len(matrix.Subjects))
	}
}

// TestListNamespaceRBAC_CRBListErr_ContinuesWithRBsOnly verifies that a
// denied ClusterRoleBinding list degrades gracefully: RoleBindings in the
// namespace still show up, and the warning mentions permissions.
func TestListNamespaceRBAC_CRBListErr_ContinuesWithRBsOnly(t *testing.T) {
	cl := newFakeClient()
	cl.listErrs["clusterrolebindings"] = errors.New("forbidden")
	cl.addResource("rolebindings", rb(
		"rb", "ns", "Role", "viewer",
		[]map[string]interface{}{{"kind": "User", "name": "alice"}},
	))
	cl.addResource("roles", role("viewer", "ns", []map[string]interface{}{
		{"verbs": []string{"get"}, "resources": []string{"pods"}, "apiGroups": []string{""}},
	}))

	matrix, err := ListNamespaceRBAC(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("ListNamespaceRBAC: %v", err)
	}
	if matrix.Warning == "" {
		t.Error("expected warning for denied CRB list")
	}
	if len(matrix.Subjects) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(matrix.Subjects))
	}
}

// TestListNamespaceRBAC_WildcardRuleSetsIsWildcard pins the flag when verbs
// contain "*".
func TestListNamespaceRBAC_WildcardRuleSetsIsWildcard(t *testing.T) {
	cl := newFakeClient()
	cl.addResource("rolebindings", rb(
		"admin-rb", "ns", "Role", "admin",
		[]map[string]interface{}{{"kind": "User", "name": "alice"}},
	))
	cl.addResource("roles", role("admin", "ns", []map[string]interface{}{
		{"verbs": []string{"*"}, "resources": []string{"pods"}, "apiGroups": []string{""}},
	}))

	matrix, err := ListNamespaceRBAC(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("ListNamespaceRBAC: %v", err)
	}
	if len(matrix.Subjects) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(matrix.Subjects))
	}
	if len(matrix.Subjects[0].Rules) == 0 || !matrix.Subjects[0].Rules[0].IsWildcard {
		t.Errorf("expected IsWildcard=true for verbs=[\"*\"], got %+v", matrix.Subjects[0].Rules)
	}
}

// TestListNamespaceRBAC_WildcardResourceSetsIsWildcard pins the flag when
// resources contain "*".
func TestListNamespaceRBAC_WildcardResourceSetsIsWildcard(t *testing.T) {
	cl := newFakeClient()
	cl.addResource("rolebindings", rb(
		"r", "ns", "Role", "any",
		[]map[string]interface{}{{"kind": "User", "name": "alice"}},
	))
	cl.addResource("roles", role("any", "ns", []map[string]interface{}{
		{"verbs": []string{"get"}, "resources": []string{"*"}, "apiGroups": []string{""}},
	}))

	matrix, err := ListNamespaceRBAC(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("ListNamespaceRBAC: %v", err)
	}
	if len(matrix.Subjects[0].Rules) == 0 || !matrix.Subjects[0].Rules[0].IsWildcard {
		t.Errorf("expected IsWildcard=true for resources=[\"*\"], got %+v", matrix.Subjects[0].Rules)
	}
}

// TestListNamespaceRBAC_MultipleSubjectKinds enumerates User, Group, and
// ServiceAccount subjects separately under the same binding.
func TestListNamespaceRBAC_MultipleSubjectKinds(t *testing.T) {
	cl := newFakeClient()
	cl.addResource("rolebindings", rb(
		"mixed", "ns", "Role", "viewer",
		[]map[string]interface{}{
			{"kind": "User", "name": "alice"},
			{"kind": "Group", "name": "platform"},
			{"kind": "ServiceAccount", "name": "runner", "namespace": "ns"},
		},
	))
	cl.addResource("roles", role("viewer", "ns", []map[string]interface{}{
		{"verbs": []string{"get"}, "resources": []string{"pods"}, "apiGroups": []string{""}},
	}))

	matrix, err := ListNamespaceRBAC(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("ListNamespaceRBAC: %v", err)
	}
	if len(matrix.Subjects) != 3 {
		t.Fatalf("expected 3 subjects (User/Group/ServiceAccount), got %d", len(matrix.Subjects))
	}
	seen := map[SubjectKind]bool{}
	for _, s := range matrix.Subjects {
		seen[s.Kind] = true
	}
	for _, k := range []SubjectKind{SubjectUser, SubjectGroup, SubjectServiceAccount} {
		if !seen[k] {
			t.Errorf("expected subject kind %q to be enumerated", k)
		}
	}
}

// TestListNamespaceRBAC_ClusterRoleBindingRecordedWithClusterWide verifies the
// ClusterWide marker is set on the RBACBinding metadata.
func TestListNamespaceRBAC_ClusterRoleBindingRecordedWithClusterWide(t *testing.T) {
	cl := newFakeClient()
	cl.addResource("clusterrolebindings", crb(
		"cluster-admins", "ClusterRole", "cluster-admin",
		[]map[string]interface{}{{"kind": "Group", "name": "admins"}},
	))
	cl.addResource("clusterroles", role("cluster-admin", "", []map[string]interface{}{
		{"verbs": []string{"*"}, "resources": []string{"*"}, "apiGroups": []string{"*"}},
	}))

	matrix, err := ListNamespaceRBAC(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("ListNamespaceRBAC: %v", err)
	}
	if len(matrix.Subjects) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(matrix.Subjects))
	}
	if len(matrix.Subjects[0].Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(matrix.Subjects[0].Bindings))
	}
	b := matrix.Subjects[0].Bindings[0]
	if !b.ClusterWide {
		t.Error("ClusterRoleBinding entry must have ClusterWide=true")
	}
	if b.Kind != "ClusterRoleBinding" {
		t.Errorf("Kind = %q, want ClusterRoleBinding", b.Kind)
	}
}

// TestListNamespaceRBAC_MissingRoleGet_ContinuesWithoutRules verifies the
// analyzer tolerates a role-fetch failure (RBAC misconfiguration / role
// deleted mid-analysis) and still returns the binding without rules.
func TestListNamespaceRBAC_MissingRoleGet_ContinuesWithoutRules(t *testing.T) {
	cl := newFakeClient()
	cl.addResource("rolebindings", rb(
		"rb", "ns", "Role", "deleted-role",
		[]map[string]interface{}{{"kind": "User", "name": "alice"}},
	))
	// NOTE: no matching role resource is seeded — Get returns
	// ErrResourceNotFound, which fetchRoleRules wraps into a non-nil error
	// and the caller swallows (no rules attached).

	matrix, err := ListNamespaceRBAC(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("ListNamespaceRBAC: %v", err)
	}
	if len(matrix.Subjects) != 1 {
		t.Fatalf("expected 1 subject even when role is missing, got %d", len(matrix.Subjects))
	}
	if len(matrix.Subjects[0].Rules) != 0 {
		t.Errorf("expected 0 rules when role is missing, got %d", len(matrix.Subjects[0].Rules))
	}
}

// TestListNamespaceRBAC_InvalidJSON_RoleBinding_Skipped ensures a malformed
// RoleBinding's Raw doesn't break the whole analysis — it's skipped.
func TestListNamespaceRBAC_InvalidJSON_RoleBinding_Skipped(t *testing.T) {
	cl := newFakeClient()
	// Inject a broken resource manually.
	cl.resources["rolebindings"] = []client.Resource{
		{Kind: "rolebindings", Raw: []byte("not-json")},
	}

	matrix, err := ListNamespaceRBAC(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("ListNamespaceRBAC: %v", err)
	}
	if len(matrix.Subjects) != 0 {
		t.Errorf("malformed RB must be skipped, got %d subjects", len(matrix.Subjects))
	}
}

// ---------------------------------------------------------------------------
// fetchRoleRules
// ---------------------------------------------------------------------------

// TestFetchRoleRules_ClusterRoleRoutesToClusterroles pins that ClusterRole
// roleKinds query the `clusterroles` endpoint with an empty namespace.
func TestFetchRoleRules_ClusterRoleRoutesToClusterroles(t *testing.T) {
	cl := newFakeClient()
	cl.addResource("clusterroles", role("view", "", []map[string]interface{}{
		{"verbs": []string{"get", "list"}, "resources": []string{"pods"}, "apiGroups": []string{""}},
	}))

	rules, err := fetchRoleRules(context.Background(), cl, "ClusterRole", "view", "irrelevant-ns")
	if err != nil {
		t.Fatalf("fetchRoleRules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].IsWildcard {
		t.Error("no wildcards in rule, IsWildcard should be false")
	}
}

// TestFetchRoleRules_RoleRoutesToRolesInNamespace pins that Role roleKinds
// query `roles` endpoint scoped to the supplied namespace.
func TestFetchRoleRules_RoleRoutesToRolesInNamespace(t *testing.T) {
	cl := newFakeClient()
	cl.addResource("roles", role("viewer", "my-ns", []map[string]interface{}{
		{"verbs": []string{"get"}, "resources": []string{"pods"}, "apiGroups": []string{""}},
	}))

	rules, err := fetchRoleRules(context.Background(), cl, "Role", "viewer", "my-ns")
	if err != nil {
		t.Fatalf("fetchRoleRules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
}

// TestFetchRoleRules_InvalidJSON_ReturnsError verifies malformed role JSON is
// surfaced to the caller.
func TestFetchRoleRules_InvalidJSON_ReturnsError(t *testing.T) {
	cl := newFakeClient()
	cl.resources["roles"] = []client.Resource{{
		Kind: "roles", Name: "bad", Namespace: "ns",
		Raw: []byte("not-json"),
	}}
	_, err := fetchRoleRules(context.Background(), cl, "Role", "bad", "ns")
	if err == nil {
		t.Error("malformed role should surface error")
	}
}
