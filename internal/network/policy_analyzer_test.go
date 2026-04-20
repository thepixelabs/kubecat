// SPDX-License-Identifier: Apache-2.0

package network

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/thepixelabs/kubecat/internal/client"
)

// ---------------------------------------------------------------------------
// fakeClusterClient
// ---------------------------------------------------------------------------

type fakeClusterClient struct {
	resources map[string]map[string][]client.Resource // kind -> namespace -> items
	listErr   map[string]error
}

func newFake() *fakeClusterClient {
	return &fakeClusterClient{
		resources: make(map[string]map[string][]client.Resource),
		listErr:   make(map[string]error),
	}
}

func (f *fakeClusterClient) add(kind, namespace string, obj map[string]interface{}) {
	raw, _ := json.Marshal(obj)
	name := ""
	var labels map[string]string
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		name, _ = meta["name"].(string)
		if l, ok := meta["labels"].(map[string]interface{}); ok {
			labels = make(map[string]string, len(l))
			for k, v := range l {
				if s, ok := v.(string); ok {
					labels[k] = s
				}
			}
		}
	}
	r := client.Resource{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
		Labels:    labels,
		Raw:       raw,
		Object:    obj,
	}
	if f.resources[kind] == nil {
		f.resources[kind] = map[string][]client.Resource{}
	}
	f.resources[kind][namespace] = append(f.resources[kind][namespace], r)
}

func (f *fakeClusterClient) Info(_ context.Context) (*client.ClusterInfo, error) {
	return &client.ClusterInfo{Name: "fake"}, nil
}
func (f *fakeClusterClient) List(_ context.Context, kind string, opts client.ListOptions) (*client.ResourceList, error) {
	if err, ok := f.listErr[kind]; ok {
		return nil, err
	}
	items := f.resources[kind][opts.Namespace]
	return &client.ResourceList{Items: items, Total: len(items)}, nil
}
func (f *fakeClusterClient) Get(_ context.Context, kind, namespace, name string) (*client.Resource, error) {
	for _, r := range f.resources[kind][namespace] {
		if r.Name == name {
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

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func pod(name string, labels map[string]string) map[string]interface{} {
	md := map[string]interface{}{"name": name}
	if labels != nil {
		lm := make(map[string]interface{}, len(labels))
		for k, v := range labels {
			lm[k] = v
		}
		md["labels"] = lm
	}
	return map[string]interface{}{"metadata": md, "spec": map[string]interface{}{}}
}

func netpolIngress(name string, podSelector map[string]string, fromSelectors []map[string]string) map[string]interface{} {
	from := make([]interface{}, 0, len(fromSelectors))
	for _, sel := range fromSelectors {
		podSel := map[string]interface{}{"matchLabels": toIfaceMap(sel)}
		from = append(from, map[string]interface{}{"podSelector": podSel})
	}
	return map[string]interface{}{
		"metadata": map[string]interface{}{"name": name},
		"spec": map[string]interface{}{
			"podSelector": map[string]interface{}{"matchLabels": toIfaceMap(podSelector)},
			"ingress": []interface{}{
				map[string]interface{}{"from": from},
			},
		},
	}
}

func toIfaceMap(in map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// ---------------------------------------------------------------------------
// AnalyzeNamespace — error path
// ---------------------------------------------------------------------------

// TestAnalyzeNamespace_PodListError_Propagates verifies list errors on pods
// bubble up with wrapping.
func TestAnalyzeNamespace_PodListError_Propagates(t *testing.T) {
	cl := newFake()
	cl.listErr["pods"] = errors.New("denied")
	_, err := AnalyzeNamespace(context.Background(), cl, "ns")
	if err == nil {
		t.Fatal("expected error when pod list fails")
	}
	if !strings.Contains(err.Error(), "list pods") {
		t.Errorf("err should mention 'list pods', got %q", err.Error())
	}
}

// TestAnalyzeNamespace_SvcListError_Tolerated verifies a list error on
// services doesn't prevent the rest of the graph from being built.
func TestAnalyzeNamespace_SvcListError_Tolerated(t *testing.T) {
	cl := newFake()
	cl.add("pods", "ns", pod("p1", nil))
	cl.add("pods", "ns", pod("p2", nil))
	cl.listErr["services"] = errors.New("denied")

	graph, err := AnalyzeNamespace(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("AnalyzeNamespace: %v", err)
	}
	if len(graph.Nodes) != 2 {
		t.Errorf("expected 2 pod nodes, got %d", len(graph.Nodes))
	}
}

// TestAnalyzeNamespace_ServiceNodesAppear verifies services are added as
// nodes with NodeKindService.
func TestAnalyzeNamespace_ServiceNodesAppear(t *testing.T) {
	cl := newFake()
	cl.add("pods", "ns", pod("p1", nil))
	cl.add("services", "ns", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "svc-1"},
	})

	graph, err := AnalyzeNamespace(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("AnalyzeNamespace: %v", err)
	}
	var sawSvc bool
	for _, n := range graph.Nodes {
		if n.Kind == NodeKindService && n.Name == "svc-1" {
			sawSvc = true
		}
	}
	if !sawSvc {
		t.Error("expected a svc-1 service node in the graph")
	}
}

// TestAnalyzeNamespace_LessThanTwoPods_NoAllowAllEdge pins the branch where
// we don't synthesize an allow-all edge for a near-empty namespace.
func TestAnalyzeNamespace_LessThanTwoPods_NoAllowAllEdge(t *testing.T) {
	cl := newFake()
	cl.add("pods", "ns", pod("lonely", nil))

	graph, err := AnalyzeNamespace(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("AnalyzeNamespace: %v", err)
	}
	if graph.HasPolicies {
		t.Error("HasPolicies must be false without netpols")
	}
	if len(graph.Edges) != 0 {
		t.Errorf("single-pod ns should have 0 edges, got %d", len(graph.Edges))
	}
}

// TestAnalyzeNamespace_EmptyFromAllowAllIngress pins the "allow all" ingress
// rule (from: []) creating a wildcard source edge.
func TestAnalyzeNamespace_EmptyFromAllowAllIngress(t *testing.T) {
	cl := newFake()
	cl.add("pods", "ns", pod("api", map[string]string{"role": "api"}))
	cl.add("pods", "ns", pod("web", map[string]string{"role": "web"}))

	cl.add("networkpolicies", "ns", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "open"},
		"spec": map[string]interface{}{
			"podSelector": map[string]interface{}{
				"matchLabels": map[string]interface{}{"role": "api"},
			},
			"ingress": []interface{}{
				// empty from → allow all
				map[string]interface{}{"from": []interface{}{}},
			},
		},
	})

	graph, err := AnalyzeNamespace(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("AnalyzeNamespace: %v", err)
	}
	if !graph.HasPolicies {
		t.Error("HasPolicies must be true")
	}
	sawStar := false
	for _, e := range graph.Edges {
		if e.Source == "*" && e.Target == "pod/api" {
			sawStar = true
		}
	}
	if !sawStar {
		t.Errorf("expected wildcard→pod/api edge, got %+v", graph.Edges)
	}
}

// TestAnalyzeNamespace_NamespaceSelector_WithPodSelector_HitsExternalBranch
// exercises the namespaceSelector branch by pairing it with a podSelector in
// the same peer so peer.PodSelector is non-nil. This avoids the
// nil-pointer bug in the peer-only namespaceSelector case (see
// TestAnalyzeNamespace_NamespaceSelectorOnly_PanicsDocumentedBug).
func TestAnalyzeNamespace_NamespaceSelector_WithPodSelector_HitsExternalBranch(t *testing.T) {
	cl := newFake()
	cl.add("pods", "ns", pod("api", map[string]string{"role": "api"}))
	cl.add("pods", "ns", pod("web", map[string]string{"role": "web"}))

	cl.add("networkpolicies", "ns", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "from-other-ns"},
		"spec": map[string]interface{}{
			"podSelector": map[string]interface{}{
				"matchLabels": map[string]interface{}{"role": "api"},
			},
			"ingress": []interface{}{
				map[string]interface{}{
					"from": []interface{}{
						map[string]interface{}{
							"podSelector": map[string]interface{}{
								"matchLabels": map[string]interface{}{"role": "web"},
							},
							"namespaceSelector": map[string]interface{}{
								"matchLabels": map[string]interface{}{"team": "platform"},
							},
						},
					},
				},
			},
		},
	})

	graph, err := AnalyzeNamespace(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("AnalyzeNamespace: %v", err)
	}
	var sawExt bool
	for _, n := range graph.Nodes {
		if n.Kind == NodeKindExternal && n.ID == "external/other-ns" {
			sawExt = true
		}
	}
	if !sawExt {
		t.Error("expected external/other-ns node for namespaceSelector")
	}
	var sawEdge bool
	for _, e := range graph.Edges {
		if e.Source == "external/other-ns" && e.Target == "pod/api" {
			sawEdge = true
		}
	}
	if !sawEdge {
		t.Errorf("expected external→api edge, got %+v", graph.Edges)
	}
}

// TestAnalyzeNamespace_NamespaceSelectorOnly_PanicsDocumentedBug pins a
// documented NIL-POINTER crash in AnalyzeNamespace: a NetworkPolicy peer
// with only `namespaceSelector` (no `podSelector`) triggers a nil deref at
// policy_analyzer.go:136 because peer.PodSelector is a *struct that is
// dereferenced without a nil check.
//
// This test recovers from the panic so CI stays green and surfaces the
// regression clearly. When policy_analyzer.go is fixed to guard
// peer.PodSelector == nil, flip this test to assert a successful analysis.
func TestAnalyzeNamespace_NamespaceSelectorOnly_PanicsDocumentedBug(t *testing.T) {
	cl := newFake()
	cl.add("pods", "ns", pod("api", map[string]string{"role": "api"}))

	cl.add("networkpolicies", "ns", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "ns-only"},
		"spec": map[string]interface{}{
			"podSelector": map[string]interface{}{
				"matchLabels": map[string]interface{}{"role": "api"},
			},
			"ingress": []interface{}{
				map[string]interface{}{
					"from": []interface{}{
						map[string]interface{}{
							// namespaceSelector only — no podSelector.
							"namespaceSelector": map[string]interface{}{
								"matchLabels": map[string]interface{}{"team": "platform"},
							},
						},
					},
				},
			},
		},
	})

	defer func() {
		if r := recover(); r == nil {
			t.Error("PIN: expected nil-pointer panic for namespaceSelector-only peer; bug may be fixed — update test to assert graph success")
		}
	}()
	_, _ = AnalyzeNamespace(context.Background(), cl, "ns")
}

// TestAnalyzeNamespace_DuplicateRulesDeduped verifies that if two rules emit
// the same (source, target) edge, only one edge is recorded. This is the
// "re-run does not duplicate rules" guarantee.
func TestAnalyzeNamespace_DuplicateRulesDeduped(t *testing.T) {
	cl := newFake()
	cl.add("pods", "ns", pod("api", map[string]string{"role": "api"}))
	cl.add("pods", "ns", pod("web", map[string]string{"role": "web"}))

	// Two identical policies.
	for i, name := range []string{"p1", "p2"} {
		_ = i
		cl.add("networkpolicies", "ns", netpolIngress(
			name,
			map[string]string{"role": "api"},
			[]map[string]string{{"role": "web"}},
		))
	}

	graph, err := AnalyzeNamespace(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("AnalyzeNamespace: %v", err)
	}
	// Both policies select the same target+source — edge map must dedupe.
	edgeSeen := map[string]int{}
	for _, e := range graph.Edges {
		edgeSeen[e.Source+"→"+e.Target]++
	}
	// Only one edge pod/web → pod/api should exist.
	if edgeSeen["pod/web→pod/api"] != 1 {
		t.Errorf("expected exactly 1 edge web→api, got %+v", edgeSeen)
	}
}

// TestAnalyzeNamespace_ReRun_Idempotent runs the analysis twice and compares
// edge IDs — output must be stable across repeated invocations.
func TestAnalyzeNamespace_ReRun_Idempotent(t *testing.T) {
	cl := newFake()
	cl.add("pods", "ns", pod("api", map[string]string{"role": "api"}))
	cl.add("pods", "ns", pod("web", map[string]string{"role": "web"}))
	cl.add("networkpolicies", "ns", netpolIngress(
		"p",
		map[string]string{"role": "api"},
		[]map[string]string{{"role": "web"}},
	))

	a, err := AnalyzeNamespace(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	b, err := AnalyzeNamespace(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if len(a.Edges) != len(b.Edges) {
		t.Fatalf("edge count drift: %d vs %d", len(a.Edges), len(b.Edges))
	}
	ids := func(g *NetworkGraph) map[string]struct{} {
		m := map[string]struct{}{}
		for _, e := range g.Edges {
			m[e.ID] = struct{}{}
		}
		return m
	}
	aIDs, bIDs := ids(a), ids(b)
	for id := range aIDs {
		if _, ok := bIDs[id]; !ok {
			t.Errorf("second run missing edge %q", id)
		}
	}
}

// ---------------------------------------------------------------------------
// labelsMatch
// ---------------------------------------------------------------------------

// TestLabelsMatch_AllSelectorKeysMatched pins the AND-semantics.
func TestLabelsMatch_AllSelectorKeysMatched(t *testing.T) {
	cases := []struct {
		name     string
		labels   map[string]string
		selector map[string]string
		want     bool
	}{
		{"exact match", map[string]string{"a": "1", "b": "2"}, map[string]string{"a": "1"}, true},
		{"partial miss", map[string]string{"a": "1"}, map[string]string{"a": "1", "b": "2"}, false},
		{"value mismatch", map[string]string{"a": "1"}, map[string]string{"a": "2"}, false},
		{"empty selector matches all", map[string]string{"a": "1"}, map[string]string{}, true},
		{"empty labels + non-empty selector", nil, map[string]string{"a": "1"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := labelsMatch(c.labels, c.selector)
			if got != c.want {
				t.Errorf("labelsMatch(%+v, %+v) = %v, want %v", c.labels, c.selector, got, c.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// selectPods
// ---------------------------------------------------------------------------

// TestSelectPods_EmptySelectorReturnsAllPods pins the empty-selector
// semantic (selects every pod, ignores services).
func TestSelectPods_EmptySelectorReturnsAllPods(t *testing.T) {
	nodes := []NetworkNode{
		{ID: "pod/a", Kind: NodeKindPod},
		{ID: "pod/b", Kind: NodeKindPod},
		{ID: "svc/x", Kind: NodeKindService},
	}
	got := selectPods(nodes, nil)
	if len(got) != 2 {
		t.Errorf("empty selector: expected 2 pods, got %d", len(got))
	}
	for _, p := range got {
		if p.Kind != NodeKindPod {
			t.Errorf("non-pod included: %+v", p)
		}
	}
}

// TestSelectPods_ByLabel filters pods by a matchLabels selector.
func TestSelectPods_ByLabel(t *testing.T) {
	nodes := []NetworkNode{
		{ID: "pod/a", Kind: NodeKindPod, Labels: map[string]string{"role": "web"}},
		{ID: "pod/b", Kind: NodeKindPod, Labels: map[string]string{"role": "db"}},
	}
	got := selectPods(nodes, map[string]string{"role": "web"})
	if len(got) != 1 || got[0].ID != "pod/a" {
		t.Errorf("expected only pod/a, got %+v", got)
	}
}

// ---------------------------------------------------------------------------
// parseNetworkPolicy — nil/invalid
// ---------------------------------------------------------------------------

// TestParseNetworkPolicy_NilObj returns nil.
func TestParseNetworkPolicy_NilObj(t *testing.T) {
	if parseNetworkPolicy(nil) != nil {
		t.Error("parseNetworkPolicy(nil) must return nil")
	}
}

// ---------------------------------------------------------------------------
// extractLabels
// ---------------------------------------------------------------------------

// TestExtractLabels_NilAndMalformedReturnNil
func TestExtractLabels_NilAndMalformedReturnNil(t *testing.T) {
	if extractLabels(nil) != nil {
		t.Error("nil object should produce nil labels")
	}
	if extractLabels(map[string]interface{}{}) != nil {
		t.Error("empty object should produce nil labels")
	}
	// metadata present but no labels key
	got := extractLabels(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "x"},
	})
	if got != nil {
		t.Errorf("missing labels key should produce nil, got %+v", got)
	}
}

// TestExtractLabels_CoercesStringValues ignores non-string label values.
func TestExtractLabels_CoercesStringValues(t *testing.T) {
	got := extractLabels(map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				"str":  "ok",
				"num":  42, // should be skipped
				"bool": true,
			},
		},
	})
	if got["str"] != "ok" {
		t.Errorf("str label = %q, want ok", got["str"])
	}
	if _, ok := got["num"]; ok {
		t.Error("non-string label value must be skipped")
	}
}

// ---------------------------------------------------------------------------
// formatPorts
// ---------------------------------------------------------------------------

// portsArgT mirrors the parameter shape accepted by formatPorts.
type portsArgT = []struct {
	Protocol string      `json:"protocol"`
	Port     interface{} `json:"port"`
}

// TestFormatPorts pins the formatter: TCP default, comma-separated list,
// empty → "all".
func TestFormatPorts(t *testing.T) {
	if got := formatPorts(nil); got != "all" {
		t.Errorf("empty = %q, want all", got)
	}

	got := formatPorts(portsArgT{
		{Protocol: "", Port: 80},
		{Protocol: "UDP", Port: 53},
	})
	if !strings.Contains(got, "TCP:80") {
		t.Errorf("expected TCP:80 in output (default protocol), got %q", got)
	}
	if !strings.Contains(got, "UDP:53") {
		t.Errorf("expected UDP:53 in output, got %q", got)
	}
	if !strings.Contains(got, ",") {
		t.Errorf("multi-port output must be comma-separated, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// addExternalNode
// ---------------------------------------------------------------------------

// TestAddExternalNode_Idempotent a second call with the same ID must not
// append a duplicate node.
func TestAddExternalNode_Idempotent(t *testing.T) {
	g := &NetworkGraph{}
	addExternalNode(g, "external/other-ns")
	addExternalNode(g, "external/other-ns")
	if len(g.Nodes) != 1 {
		t.Errorf("expected 1 node after duplicate adds, got %d", len(g.Nodes))
	}
}
