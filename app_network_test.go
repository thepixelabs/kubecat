// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"testing"

	"github.com/thepixelabs/kubecat/internal/network"
	"github.com/thepixelabs/kubecat/internal/security"
)

// podObj builds a minimal Pod object map.
func podObj(name, namespace string, labels map[string]string, containers []map[string]interface{}) map[string]interface{} {
	md := map[string]interface{}{"name": name, "namespace": namespace}
	if labels != nil {
		md["labels"] = asAnyMap(labels)
	}
	cs := make([]interface{}, 0, len(containers))
	for _, c := range containers {
		cs = append(cs, c)
	}
	return map[string]interface{}{
		"metadata": md,
		"spec":     map[string]interface{}{"containers": cs},
	}
}

// asAnyMap converts map[string]string → map[string]interface{} for JSON shape.
func asAnyMap(m map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// TestAnalyzeNamespace_NoPolicies_SynthesizesAllowAllEdge verifies the
// "fully-open" case emits a single allow-all edge when at least two pods
// exist in the namespace.
func TestAnalyzeNamespace_NoPolicies_SynthesizesAllowAllEdge(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("pods", podObj("a", "ns", map[string]string{"app": "a"}, nil))
	cl.addResource("pods", podObj("b", "ns", map[string]string{"app": "b"}, nil))

	graph, err := network.AnalyzeNamespace(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("AnalyzeNamespace: %v", err)
	}
	if graph.HasPolicies {
		t.Error("HasPolicies should be false when no netpols exist")
	}
	if len(graph.Edges) != 1 || graph.Edges[0].ID != "allow-all" {
		t.Errorf("expected single allow-all edge, got %+v", graph.Edges)
	}
}

// TestAnalyzeNamespace_PolicyRestrictsTraffic asserts that with a policy
// selecting one pod, only pods matching the from selector may ingress.
func TestAnalyzeNamespace_PolicyRestrictsTraffic(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("pods", podObj("api", "ns", map[string]string{"role": "api"}, nil))
	cl.addResource("pods", podObj("web", "ns", map[string]string{"role": "web"}, nil))
	cl.addResource("pods", podObj("db", "ns", map[string]string{"role": "db"}, nil))

	// Allow web → api only.
	cl.addResource("networkpolicies", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "api-ingress", "namespace": "ns"},
		"spec": map[string]interface{}{
			"podSelector": map[string]interface{}{"matchLabels": map[string]interface{}{"role": "api"}},
			"ingress": []interface{}{
				map[string]interface{}{
					"from": []interface{}{
						map[string]interface{}{
							"podSelector": map[string]interface{}{"matchLabels": map[string]interface{}{"role": "web"}},
						},
					},
				},
			},
		},
	})

	graph, err := network.AnalyzeNamespace(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("AnalyzeNamespace: %v", err)
	}
	if !graph.HasPolicies {
		t.Error("HasPolicies should be true with a netpol")
	}
	// Expect exactly one edge: web → api.
	if len(graph.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d: %+v", len(graph.Edges), graph.Edges)
	}
	edge := graph.Edges[0]
	if edge.Source != "pod/web" || edge.Target != "pod/api" {
		t.Errorf("edge = %s → %s, want pod/web → pod/api", edge.Source, edge.Target)
	}
	if edge.PolicyName != "api-ingress" {
		t.Errorf("edge.PolicyName = %q, want api-ingress", edge.PolicyName)
	}
}

// TestAnalyzeNamespace_EmptyNamespace_NoPanic confirms an empty namespace
// returns an empty graph without panicking.
func TestAnalyzeNamespace_EmptyNamespace_NoPanic(t *testing.T) {
	cl := newFakeClusterClient()
	graph, err := network.AnalyzeNamespace(context.Background(), cl, "empty")
	if err != nil {
		t.Fatalf("AnalyzeNamespace: %v", err)
	}
	if len(graph.Edges) != 0 {
		t.Errorf("empty namespace should have no edges, got %d", len(graph.Edges))
	}
}

// TestAnalyzeNetworkPolicies_RoutesThroughApp exercises the App bridge.
func TestAnalyzeNetworkPolicies_RoutesThroughApp(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("pods", podObj("a", "ns", nil, nil))
	a := newAppWithFakes(cl)

	graph, err := a.AnalyzeNetworkPolicies("some-ctx", "ns")
	if err != nil {
		t.Fatalf("AnalyzeNetworkPolicies: %v", err)
	}
	if len(graph.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(graph.Nodes))
	}
}

// TestGetNetworkPolicyYAML_ReturnsRaw verifies the app handler returns raw
// bytes from the cluster client.
func TestGetNetworkPolicyYAML_ReturnsRaw(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("networkpolicies", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "np-1", "namespace": "ns"},
		"spec": map[string]interface{}{
			"podSelector": map[string]interface{}{},
		},
	})
	a := newAppWithFakes(cl)

	yaml, err := a.GetNetworkPolicyYAML("ctx", "ns", "np-1")
	if err != nil {
		t.Fatalf("GetNetworkPolicyYAML: %v", err)
	}
	if len(yaml) == 0 {
		t.Error("expected non-empty YAML/JSON bytes")
	}
}

// TestGetNetworkPolicyYAML_NotFound returns an error for missing policies.
func TestGetNetworkPolicyYAML_NotFound(t *testing.T) {
	cl := newFakeClusterClient()
	a := newAppWithFakes(cl)
	_, err := a.GetNetworkPolicyYAML("ctx", "ns", "ghost")
	if err == nil {
		t.Error("expected error for non-existent NetworkPolicy")
	}
}

// ---------------------------------------------------------------------------
// netpol recommender — recommendation idempotency on re-run
// ---------------------------------------------------------------------------

// TestNetpolRecommender_RecommendDefaultDeny_Deterministic re-runs the
// default-deny generator twice and compares output — the generator must
// produce identical YAML on each invocation (no randomness, no duplicate
// labels, no diff).
func TestNetpolRecommender_RecommendDefaultDeny_Deterministic(t *testing.T) {
	cl := newFakeClusterClient()
	r := security.NewNetpolRecommender(cl)

	a, err := r.RecommendDefaultDeny(context.Background(), "prod")
	if err != nil {
		t.Fatalf("first RecommendDefaultDeny: %v", err)
	}
	b, err := r.RecommendDefaultDeny(context.Background(), "prod")
	if err != nil {
		t.Fatalf("second RecommendDefaultDeny: %v", err)
	}
	if a.YAML != b.YAML {
		t.Errorf("default-deny YAML is non-deterministic across re-runs:\nfirst:\n%s\nsecond:\n%s",
			a.YAML, b.YAML)
	}
}
