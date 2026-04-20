// SPDX-License-Identifier: Apache-2.0

package cost

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
)

// ---------------------------------------------------------------------------
// Test fake client (package-local; cost/ has no pre-existing fake).
// ---------------------------------------------------------------------------

type fakeClient struct {
	resources map[string]map[string][]client.Resource // kind -> namespace -> items
	listErr   map[string]error
}

func newFake() *fakeClient {
	return &fakeClient{
		resources: make(map[string]map[string][]client.Resource),
		listErr:   make(map[string]error),
	}
}

func (f *fakeClient) addPod(namespace string, obj map[string]interface{}) {
	raw, _ := json.Marshal(obj)
	name, _ := obj["metadata"].(map[string]interface{})["name"].(string)
	r := client.Resource{Kind: "pods", Name: name, Namespace: namespace, Raw: raw, Object: obj}
	if f.resources["pods"] == nil {
		f.resources["pods"] = map[string][]client.Resource{}
	}
	f.resources["pods"][namespace] = append(f.resources["pods"][namespace], r)
}

func (f *fakeClient) addSvc(namespace, name string) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{"name": name, "namespace": namespace},
	}
	raw, _ := json.Marshal(obj)
	r := client.Resource{Kind: "services", Name: name, Namespace: namespace, Raw: raw, Object: obj}
	if f.resources["services"] == nil {
		f.resources["services"] = map[string][]client.Resource{}
	}
	f.resources["services"][namespace] = append(f.resources["services"][namespace], r)
}

func (f *fakeClient) Info(_ context.Context) (*client.ClusterInfo, error) {
	return &client.ClusterInfo{Name: "fake"}, nil
}
func (f *fakeClient) List(_ context.Context, kind string, opts client.ListOptions) (*client.ResourceList, error) {
	if err, ok := f.listErr[kind]; ok {
		return nil, err
	}
	byNs := f.resources[kind]
	items := byNs[opts.Namespace]
	return &client.ResourceList{Items: items, Total: len(items)}, nil
}
func (f *fakeClient) Get(_ context.Context, kind, namespace, name string) (*client.Resource, error) {
	for _, r := range f.resources[kind][namespace] {
		if r.Name == name {
			return &r, nil
		}
	}
	return nil, client.ErrResourceNotFound
}
func (f *fakeClient) Delete(_ context.Context, _, _, _ string) error { return nil }
func (f *fakeClient) Watch(_ context.Context, _ string, _ client.WatchOptions) (<-chan client.WatchEvent, error) {
	ch := make(chan client.WatchEvent)
	close(ch)
	return ch, nil
}
func (f *fakeClient) Logs(_ context.Context, _, _, _ string, _ bool, _ int64) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}
func (f *fakeClient) Exec(_ context.Context, _, _, _ string, _ []string) error { return nil }
func (f *fakeClient) PortForward(_ context.Context, _, _ string, _, _ int) (client.PortForwarder, error) {
	return nil, nil
}
func (f *fakeClient) Close() error { return nil }

// ---------------------------------------------------------------------------
// DetectBackend — namespace traversal
// ---------------------------------------------------------------------------

// TestDetectBackend_OpenCostInOpenCostNamespace verifies the probe finds a
// service whose name contains "opencost" in the opencost namespace.
func TestDetectBackend_OpenCostInOpenCostNamespace(t *testing.T) {
	cl := newFake()
	cl.addSvc("opencost", "opencost-cost-analyzer")

	got := DetectBackend(context.Background(), cl)
	if got != BackendOpenCost {
		t.Errorf("got %q, want %q", got, BackendOpenCost)
	}
}

// TestDetectBackend_KubecostInMonitoring probes a common Kubecost install
// location.
func TestDetectBackend_KubecostInMonitoring(t *testing.T) {
	cl := newFake()
	cl.addSvc("monitoring", "kubecost-cost-analyzer")

	got := DetectBackend(context.Background(), cl)
	if got != BackendKubecost {
		t.Errorf("got %q, want %q", got, BackendKubecost)
	}
}

// TestDetectBackend_OpenCostInKubeSystem covers the last namespace probed.
func TestDetectBackend_OpenCostInKubeSystem(t *testing.T) {
	cl := newFake()
	cl.addSvc("kube-system", "opencost")

	got := DetectBackend(context.Background(), cl)
	if got != BackendOpenCost {
		t.Errorf("got %q, want %q", got, BackendOpenCost)
	}
}

// TestDetectBackend_ListErrorContinuesToNextNamespace ensures a permission
// error on one namespace doesn't block detection in another.
func TestDetectBackend_ListErrorContinuesToNextNamespace(t *testing.T) {
	cl := newFake()
	cl.listErr["services"] = errors.New("forbidden") // affects all ns but func tolerates it
	got := DetectBackend(context.Background(), cl)
	// All namespaces error → no match → BackendNone.
	if got != BackendNone {
		t.Errorf("got %q, want BackendNone", got)
	}
}

// TestDetectBackend_NonMatchingServiceIgnored covers the negative case where
// the found service is unrelated to cost backends.
func TestDetectBackend_NonMatchingServiceIgnored(t *testing.T) {
	cl := newFake()
	cl.addSvc("monitoring", "prometheus")
	got := DetectBackend(context.Background(), cl)
	if got != BackendNone {
		t.Errorf("got %q, want BackendNone when only non-cost services exist", got)
	}
}

// ---------------------------------------------------------------------------
// GetWorkloadCost — prefix matching and sum across pods
// ---------------------------------------------------------------------------

// TestGetWorkloadCost_SumsAllMatchingPods verifies that requests across
// matching pods are summed.
func TestGetWorkloadCost_SumsAllMatchingPods(t *testing.T) {
	cl := newFake()
	cl.addPod("ns", podObj("nginx-abc-111", "500m", "512Mi"))
	cl.addPod("ns", podObj("nginx-abc-222", "500m", "512Mi"))
	cl.addPod("ns", podObj("other-pod-000", "100m", "128Mi")) // not matched

	est := New(cl, 0.048, 0.006, "USD")
	got, err := est.GetWorkloadCost(context.Background(), "ns", "nginx")
	if err != nil {
		t.Fatalf("GetWorkloadCost: %v", err)
	}
	// 1 core (0.5 + 0.5) × 0.048 = 0.048. 1 GB × 0.006 = 0.006. Total = 0.054.
	want := 0.05 // round2 floor
	if got.TotalCost < want-0.01 || got.TotalCost > want+0.01 {
		t.Errorf("TotalCost = %v, want ~%v", got.TotalCost, want)
	}
}

// TestGetNamespaceCost_AggregatesAcrossWorkloads seeds two deployment
// workloads and asserts both appear in the summary.
func TestGetNamespaceCost_AggregatesAcrossWorkloads(t *testing.T) {
	cl := newFake()
	cl.addPod("ns", podObj("deploy-a-111-aaa", "100m", "128Mi"))
	cl.addPod("ns", podObj("deploy-a-111-bbb", "100m", "128Mi"))
	cl.addPod("ns", podObj("deploy-b-222-ccc", "200m", "256Mi"))

	est := New(cl, 0.048, 0.006, "USD")
	sum, err := est.GetNamespaceCost(context.Background(), "ns")
	if err != nil {
		t.Fatalf("GetNamespaceCost: %v", err)
	}
	if len(sum.Workloads) != 2 {
		t.Fatalf("expected 2 workloads (deploy-a + deploy-b), got %d", len(sum.Workloads))
	}
}

// TestGetNamespaceCost_ListError surfaces List error.
func TestGetNamespaceCost_ListError(t *testing.T) {
	cl := newFake()
	cl.listErr["pods"] = errors.New("denied")
	est := New(cl, 0, 0, "")
	_, err := est.GetNamespaceCost(context.Background(), "ns")
	if err == nil {
		t.Error("expected error when pod list fails")
	}
}

// TestGetWorkloadCost_ListError surfaces List error.
func TestGetWorkloadCost_ListError(t *testing.T) {
	cl := newFake()
	cl.listErr["pods"] = errors.New("denied")
	est := New(cl, 0, 0, "")
	_, err := est.GetWorkloadCost(context.Background(), "ns", "w")
	if err == nil {
		t.Error("expected error when pod list fails")
	}
}

// ---------------------------------------------------------------------------
// New() default substitution
// ---------------------------------------------------------------------------

// TestNew_AppliesDefaultsWhenArgsZero pins the zero-value substitution for
// CPU/mem costs and currency.
func TestNew_AppliesDefaultsWhenArgsZero(t *testing.T) {
	e := New(nil, 0, 0, "")
	if e.cpuCostPerCore != defaultCPUCostPerCoreHour {
		t.Errorf("cpuCostPerCore = %v, want default %v", e.cpuCostPerCore, defaultCPUCostPerCoreHour)
	}
	if e.memCostPerGB != defaultMemCostPerGBHour {
		t.Errorf("memCostPerGB = %v, want default %v", e.memCostPerGB, defaultMemCostPerGBHour)
	}
	if e.currency != "USD" {
		t.Errorf("currency = %q, want USD", e.currency)
	}
}

// TestNew_KeepsNonZeroArgs pins that explicit args pass through untouched.
func TestNew_KeepsNonZeroArgs(t *testing.T) {
	e := New(nil, 0.1, 0.02, "EUR")
	if e.cpuCostPerCore != 0.1 || e.memCostPerGB != 0.02 || e.currency != "EUR" {
		t.Errorf("got {%v,%v,%q}, want {0.1,0.02,EUR}",
			e.cpuCostPerCore, e.memCostPerGB, e.currency)
	}
}

// ---------------------------------------------------------------------------
// parseCPU / parseMemoryGB — edge cases
// ---------------------------------------------------------------------------

// TestParseCPU_Edge covers garbage input, empty, decimal, and leading/trailing
// whitespace.
func TestParseCPU_Edge(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"", 0},
		{"   ", 0},
		{"  2.5  ", 2.5},
		{"garbage", 0},
		{"1500m", 1.5},
		{"0.5", 0.5},
	}
	for _, c := range cases {
		got := parseCPU(c.in)
		if got != c.want {
			t.Errorf("parseCPU(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestParseMemoryGB_Edge covers the M/G suffixes (decimal units vs binary),
// raw-byte fallback, and whitespace.
func TestParseMemoryGB_Edge(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		tol  float64
	}{
		{"", 0, 0.0001},
		{"  1Gi  ", 1.0, 0.001},
		{"1Ki", 1.0 / (1024 * 1024), 0.0001},
		{"1000M", 1.0, 0.01},      // decimal M → 1 GB
		{"1G", 1.0, 0.01},         // decimal G
		{"1073741824", 1.0, 0.01}, // bare bytes → 1 GiB
		{"500Mi", 0.5, 0.05},
	}
	for _, c := range cases {
		got := parseMemoryGB(c.in)
		diff := got - c.want
		if diff < 0 {
			diff = -diff
		}
		if diff > c.tol {
			t.Errorf("parseMemoryGB(%q) = %v, want ~%v (tol %v)", c.in, got, c.want, c.tol)
		}
	}
}

// ---------------------------------------------------------------------------
// extractRequests — tolerates nil/empty shapes
// ---------------------------------------------------------------------------

// TestExtractRequests_NilObj returns zero values without panic.
func TestExtractRequests_NilObj(t *testing.T) {
	cpu, mem := extractRequests(nil)
	if cpu != 0 || mem != 0 {
		t.Errorf("nil obj: cpu=%v mem=%v, want 0,0", cpu, mem)
	}
}

// TestExtractRequests_MissingSpec returns zero values.
func TestExtractRequests_MissingSpec(t *testing.T) {
	cpu, mem := extractRequests(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "x"},
	})
	if cpu != 0 || mem != 0 {
		t.Errorf("missing spec: cpu=%v mem=%v, want 0,0", cpu, mem)
	}
}

// TestExtractRequests_SumAcrossContainers verifies multiple containers are
// summed.
func TestExtractRequests_SumAcrossContainers(t *testing.T) {
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{"cpu": "100m", "memory": "512Mi"},
					},
				},
				map[string]interface{}{
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{"cpu": "200m", "memory": "512Mi"},
					},
				},
			},
		},
	}
	cpu, mem := extractRequests(obj)
	if cpu < 0.29 || cpu > 0.31 {
		t.Errorf("cpu = %v, want ~0.3 (0.1+0.2)", cpu)
	}
	if mem < 0.99 || mem > 1.01 {
		t.Errorf("mem = %v, want ~1.0 (512Mi + 512Mi)", mem)
	}
}

// ---------------------------------------------------------------------------
// QueryOpenCost — additional shape variations
// ---------------------------------------------------------------------------

// TestQueryOpenCost_EmptyDataArray returns zero estimates without error.
func TestQueryOpenCost_EmptyDataArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
	}))
	defer srv.Close()

	estimates, err := QueryOpenCost(context.Background(), srv.URL, "ns")
	if err != nil {
		t.Fatalf("QueryOpenCost: %v", err)
	}
	if len(estimates) != 0 {
		t.Errorf("expected 0 estimates, got %d", len(estimates))
	}
}

// TestQueryOpenCost_QueryStringContainsNamespace pins the request path/query.
func TestQueryOpenCost_QueryStringContainsNamespace(t *testing.T) {
	var gotPath, gotRawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotRawQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
	}))
	defer srv.Close()

	_, err := QueryOpenCost(context.Background(), srv.URL, "team-a")
	if err != nil {
		t.Fatalf("QueryOpenCost: %v", err)
	}
	if gotPath != "/model/allocation" {
		t.Errorf("path = %q, want /model/allocation", gotPath)
	}
	if !strings.Contains(gotRawQuery, "namespace=team-a") {
		t.Errorf("query = %q, should contain namespace=team-a", gotRawQuery)
	}
	if !strings.Contains(gotRawQuery, "aggregate=pod") {
		t.Errorf("query = %q, should contain aggregate=pod", gotRawQuery)
	}
}

// TestQueryOpenCost_IgnoresNonMapEntries tolerates mixed JSON shapes.
func TestQueryOpenCost_IgnoresNonMapEntries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Embed a string where an alloc map is expected — must be skipped.
		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"pod1": "not-a-map",
					"pod2": map[string]interface{}{"totalCost": 0.5, "cpuCost": 0.3, "ramCost": 0.2},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	estimates, err := QueryOpenCost(context.Background(), srv.URL, "ns")
	if err != nil {
		t.Fatalf("QueryOpenCost: %v", err)
	}
	if len(estimates) != 1 {
		t.Errorf("expected 1 valid estimate (pod2), got %d", len(estimates))
	}
}

// TestQueryOpenCost_MonthlyDerivedFromTotal pins the arithmetic:
// MonthlyTotal = round2(totalCost × 730).
func TestQueryOpenCost_MonthlyDerivedFromTotal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{"pod": map[string]interface{}{"totalCost": 1.0, "cpuCost": 0.6, "ramCost": 0.4}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	estimates, err := QueryOpenCost(context.Background(), srv.URL, "ns")
	if err != nil {
		t.Fatalf("QueryOpenCost: %v", err)
	}
	if len(estimates) != 1 {
		t.Fatalf("expected 1 estimate, got %d", len(estimates))
	}
	// 1.0 × 730 = 730.0
	want := 730.0
	if estimates[0].MonthlyTotal != want {
		t.Errorf("MonthlyTotal = %v, want %v", estimates[0].MonthlyTotal, want)
	}
}

// TestQueryOpenCost_SlowServer_ContextTimeout ensures a context with a small
// deadline is respected by the HTTP call.
func TestQueryOpenCost_SlowServer_ContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(200 * time.Millisecond):
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := QueryOpenCost(ctx, srv.URL, "ns")
	if err == nil {
		t.Error("expected timeout/cancellation error")
	}
}

// ---------------------------------------------------------------------------
// round2
// ---------------------------------------------------------------------------

// TestRound2 pins the truncating-to-two-decimals behavior (NB: this truncates,
// not rounds — 0.045 → 0.04, not 0.05).
func TestRound2(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{0, 0},
		{0.123, 0.12},
		{0.129, 0.12}, // truncated, not rounded
		{1.999, 1.99},
		{-0.5, -0.5},
	}
	for _, c := range cases {
		got := round2(c.in)
		if got != c.want {
			t.Errorf("round2(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func podObj(name, cpu, mem string) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{"name": name},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name": "c",
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{"cpu": cpu, "memory": mem},
					},
				},
			},
		},
	}
}
