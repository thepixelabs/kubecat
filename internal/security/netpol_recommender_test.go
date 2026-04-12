// SPDX-License-Identifier: Apache-2.0

package security

import (
	"context"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// RecommendDefaultDeny
// ---------------------------------------------------------------------------

func TestRecommendDefaultDeny_NonSystemNamespace_ProducesYAML(t *testing.T) {
	cl := newFakeClient()
	r := NewNetpolRecommender(cl)

	rec, err := r.RecommendDefaultDeny(context.Background(), "my-app")
	if err != nil {
		t.Fatalf("RecommendDefaultDeny: %v", err)
	}
	if rec.Suppressed {
		t.Error("recommendation for non-system namespace must not be suppressed")
	}
	if rec.YAML == "" {
		t.Error("expected non-empty YAML for non-system namespace")
	}
}

func TestRecommendDefaultDeny_YAMLIsValidKubernetesResource(t *testing.T) {
	cl := newFakeClient()
	r := NewNetpolRecommender(cl)

	rec, err := r.RecommendDefaultDeny(context.Background(), "production")
	if err != nil {
		t.Fatalf("RecommendDefaultDeny: %v", err)
	}

	var obj map[string]interface{}
	if err := yaml.Unmarshal([]byte(rec.YAML), &obj); err != nil {
		t.Fatalf("invalid YAML: %v\n---\n%s", err, rec.YAML)
	}
	if obj["kind"] != "NetworkPolicy" {
		t.Errorf("kind = %v, want NetworkPolicy", obj["kind"])
	}
	if obj["apiVersion"] != "networking.k8s.io/v1" {
		t.Errorf("apiVersion = %v, want networking.k8s.io/v1", obj["apiVersion"])
	}
}

func TestRecommendDefaultDeny_YAMLDeniesIngressAndEgress(t *testing.T) {
	cl := newFakeClient()
	r := NewNetpolRecommender(cl)

	rec, err := r.RecommendDefaultDeny(context.Background(), "staging")
	if err != nil {
		t.Fatalf("RecommendDefaultDeny: %v", err)
	}

	if !strings.Contains(rec.YAML, "Ingress") {
		t.Error("default-deny policy must declare Ingress policy type")
	}
	if !strings.Contains(rec.YAML, "Egress") {
		t.Error("default-deny policy must declare Egress policy type")
	}
}

func TestRecommendDefaultDeny_SystemNamespace_Suppressed(t *testing.T) {
	cl := newFakeClient()
	r := NewNetpolRecommender(cl)

	systemNamespaces := []string{"kube-system", "kube-public", "kube-node-lease"}
	for _, ns := range systemNamespaces {
		t.Run(ns, func(t *testing.T) {
			rec, err := r.RecommendDefaultDeny(context.Background(), ns)
			if err != nil {
				t.Fatalf("RecommendDefaultDeny(%q): %v", ns, err)
			}
			if !rec.Suppressed {
				t.Errorf("system namespace %q must produce suppressed recommendation", ns)
			}
			if rec.YAML != "" {
				t.Errorf("suppressed recommendation for %q must have empty YAML", ns)
			}
		})
	}
}

func TestRecommendDefaultDeny_NamespaceAppearsInPolicy(t *testing.T) {
	cl := newFakeClient()
	r := NewNetpolRecommender(cl)

	rec, err := r.RecommendDefaultDeny(context.Background(), "my-unique-ns-9999")
	if err != nil {
		t.Fatalf("RecommendDefaultDeny: %v", err)
	}
	if !strings.Contains(rec.YAML, "my-unique-ns-9999") {
		t.Errorf("namespace name not found in generated YAML:\n%s", rec.YAML)
	}
}

// ---------------------------------------------------------------------------
// RecommendForPod
// ---------------------------------------------------------------------------

func TestRecommendForPod_SystemNamespace_ReturnsEmpty(t *testing.T) {
	cl := newFakeClient()
	r := NewNetpolRecommender(cl)

	for _, ns := range []string{"kube-system", "kube-public", "kube-node-lease"} {
		t.Run(ns, func(t *testing.T) {
			recs, err := r.RecommendForPod(context.Background(), ns, "coredns")
			if err != nil {
				t.Fatalf("unexpected error for system ns %q: %v", ns, err)
			}
			if len(recs) != 0 {
				t.Errorf("expected 0 recommendations for %q, got %d", ns, len(recs))
			}
		})
	}
}

func TestRecommendForPod_UnknownPod_ReturnsError(t *testing.T) {
	cl := newFakeClient()
	r := NewNetpolRecommender(cl)

	_, err := r.RecommendForPod(context.Background(), "default", "ghost-pod")
	if err == nil {
		t.Error("RecommendForPod for non-existent pod should return error")
	}
}

// ---------------------------------------------------------------------------
// RecommendForService
// ---------------------------------------------------------------------------

func TestRecommendForService_SystemNamespace_Suppressed(t *testing.T) {
	cl := newFakeClient()
	r := NewNetpolRecommender(cl)

	rec, err := r.RecommendForService(context.Background(), "kube-system", "kube-dns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rec.Suppressed {
		t.Error("recommendation for kube-system service must be suppressed")
	}
}

func TestRecommendForService_UnknownService_ReturnsError(t *testing.T) {
	cl := newFakeClient()
	r := NewNetpolRecommender(cl)

	_, err := r.RecommendForService(context.Background(), "default", "ghost-svc")
	if err == nil {
		t.Error("RecommendForService for non-existent service should return error")
	}
}

// ---------------------------------------------------------------------------
// sortRecommendations
// ---------------------------------------------------------------------------

func TestSortRecommendations_ByNamespaceThenName(t *testing.T) {
	recs := []*NetworkPolicyRecommendation{
		{Namespace: "z-ns", Name: "allow-svc-b"},
		{Namespace: "a-ns", Name: "allow-svc-c"},
		{Namespace: "a-ns", Name: "allow-svc-a"},
	}
	sortRecommendations(recs)

	if recs[0].Namespace != "a-ns" || recs[0].Name != "allow-svc-a" {
		t.Errorf("sort[0] = {%s, %s}, want {a-ns, allow-svc-a}", recs[0].Namespace, recs[0].Name)
	}
	if recs[1].Namespace != "a-ns" || recs[1].Name != "allow-svc-c" {
		t.Errorf("sort[1] = {%s, %s}, want {a-ns, allow-svc-c}", recs[1].Namespace, recs[1].Name)
	}
	if recs[2].Namespace != "z-ns" {
		t.Errorf("sort[2].Namespace = %s, want z-ns", recs[2].Namespace)
	}
}

func TestSortRecommendations_EmptySlice_NoPanic(t *testing.T) {
	sortRecommendations(nil)
	sortRecommendations([]*NetworkPolicyRecommendation{})
}

// ---------------------------------------------------------------------------
// uniqueSortedInts
// ---------------------------------------------------------------------------

func TestUniqueSortedInts(t *testing.T) {
	cases := []struct {
		name string
		in   []int
		want []int
	}{
		{"deduplicates and sorts", []int{3, 1, 2, 1, 3}, []int{1, 2, 3}},
		{"common port deduplicate", []int{80, 443, 80}, []int{80, 443}},
		{"nil input", nil, nil},
		{"single value", []int{8080}, []int{8080}},
		{"already sorted", []int{1, 2, 3}, []int{1, 2, 3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := uniqueSortedInts(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("uniqueSortedInts(%v) len=%d, want len=%d; got %v", tc.in, len(got), len(tc.want), got)
			}
			for i, v := range got {
				if v != tc.want[i] {
					t.Errorf("uniqueSortedInts(%v)[%d] = %d, want %d", tc.in, i, v, tc.want[i])
				}
			}
		})
	}
}
