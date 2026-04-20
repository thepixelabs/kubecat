// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
)

// ---------------------------------------------------------------------------
// ListResources
// ---------------------------------------------------------------------------

// TestListResources_ReturnsAllItems verifies that the raw Kubernetes resources
// exposed by the fake client are returned as ResourceInfo records with the
// core fields (Kind, Name, Namespace, APIVersion) preserved.
func TestListResources_ReturnsAllItems(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("pods", podObject("pod-a", "default", "Running"))
	cl.addResource("pods", podObject("pod-b", "default", "Running"))
	cl.addResource("pods", podObject("pod-c", "kube-system", "Running"))

	a := newAppWithFakes(cl)

	infos, err := a.ListResources("pods", "")
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if len(infos) != 3 {
		t.Fatalf("expected 3 pods, got %d", len(infos))
	}

	names := map[string]bool{}
	for _, info := range infos {
		names[info.Name] = true
	}
	for _, want := range []string{"pod-a", "pod-b", "pod-c"} {
		if !names[want] {
			t.Errorf("missing pod %q in results", want)
		}
	}
}

// TestListResources_FiltersByNamespace confirms that scoping to a namespace
// excludes resources from other namespaces.
func TestListResources_FiltersByNamespace(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("pods", podObject("pod-a", "default", "Running"))
	cl.addResource("pods", podObject("pod-b", "kube-system", "Running"))

	a := newAppWithFakes(cl)

	infos, err := a.ListResources("pods", "default")
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 pod in default, got %d", len(infos))
	}
	if infos[0].Name != "pod-a" {
		t.Errorf("expected pod-a, got %s", infos[0].Name)
	}
}

// TestListResources_EmptyNamespaceIncludesAllNamespaces confirms the
// documented contract: namespace="" means "all namespaces".
func TestListResources_EmptyNamespaceIncludesAllNamespaces(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("pods", podObject("a", "ns-1", "Running"))
	cl.addResource("pods", podObject("b", "ns-2", "Running"))
	cl.addResource("pods", podObject("c", "ns-3", "Running"))

	a := newAppWithFakes(cl)

	infos, err := a.ListResources("pods", "")
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if len(infos) != 3 {
		t.Errorf("expected 3 pods across all namespaces, got %d", len(infos))
	}
}

// TestListResources_LargeResultSet_NoTruncation exercises a 500-pod listing
// and confirms every item reaches the caller.
func TestListResources_LargeResultSet_NoTruncation(t *testing.T) {
	cl := newFakeClusterClient()
	const n = 500
	for i := 0; i < n; i++ {
		cl.addResource("pods", podObject(fmt.Sprintf("pod-%04d", i), "default", "Running"))
	}
	a := newAppWithFakes(cl)

	infos, err := a.ListResources("pods", "default")
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if len(infos) != n {
		t.Errorf("expected %d pods, got %d (possible truncation)", n, len(infos))
	}
}

// TestListResources_UnknownKind_SurfacesError verifies unknown/unsupported
// kinds result in an error from the underlying cluster client.
func TestListResources_UnknownKind_SurfacesError(t *testing.T) {
	cl := newFakeClusterClient()
	cl.listErr["widgets"] = fmt.Errorf("unknown resource kind: widgets")
	a := newAppWithFakes(cl)

	_, err := a.ListResources("widgets", "")
	if err == nil {
		t.Fatal("expected error for unknown kind, got nil")
	}
	if !strings.Contains(err.Error(), "widgets") {
		t.Errorf("error should identify the bad kind, got: %v", err)
	}
}

// TestListResources_NoActiveCluster_ReturnsError confirms a disconnected app
// yields a clean error (not a panic) when listing is attempted.
func TestListResources_NoActiveCluster_ReturnsError(t *testing.T) {
	a := newAppWithFakes(nil)
	_, err := a.ListResources("pods", "")
	if err == nil {
		t.Fatal("expected error for no active cluster, got nil")
	}
}

// TestListResources_Secret_OmitsValuesButIncludesDataKeys pins the redaction
// contract: secret list results expose the key names but never the values.
// This is important to a GUI that renders resource summaries — we don't want
// to leak secret payloads in a list view.
func TestListResources_Secret_OmitsValuesButIncludesDataKeys(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("secrets", map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "my-secret",
			"namespace": "default",
		},
		"data": map[string]interface{}{
			"password": base64.StdEncoding.EncodeToString([]byte("hunter2")),
			"token":    base64.StdEncoding.EncodeToString([]byte("topsecret")),
		},
	})

	a := newAppWithFakes(cl)

	infos, err := a.ListResources("secrets", "default")
	if err != nil {
		t.Fatalf("ListResources secrets: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(infos))
	}
	info := infos[0]

	// Data keys exposed for UI
	if info.DataCount != 2 {
		t.Errorf("expected DataCount=2, got %d", info.DataCount)
	}
	sort.Strings(info.DataKeys)
	if !reflect.DeepEqual(info.DataKeys, []string{"password", "token"}) {
		t.Errorf("DataKeys = %v, want [password token]", info.DataKeys)
	}

	// Values must not leak through any stringified field.
	blob, _ := json.Marshal(info)
	for _, secret := range []string{"hunter2", "topsecret"} {
		if strings.Contains(string(blob), secret) {
			t.Errorf("ListResources leaked secret value %q in summary: %s", secret, blob)
		}
	}
}

// ---------------------------------------------------------------------------
// GetResource
// ---------------------------------------------------------------------------

// TestGetResource_ReturnsRawJSON confirms the caller receives the
// exact JSON representation of the resource as stored in the cluster.
func TestGetResource_ReturnsRawJSON(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("pods", podObject("web", "default", "Running"))
	a := newAppWithFakes(cl)

	raw, err := a.GetResource("pods", "default", "web")
	if err != nil {
		t.Fatalf("GetResource: %v", err)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		t.Fatalf("returned value was not valid JSON: %v", err)
	}
	meta, _ := obj["metadata"].(map[string]interface{})
	if meta["name"] != "web" {
		t.Errorf("expected metadata.name=web, got %v", meta["name"])
	}
}

// TestGetResource_NotFound_ReturnsError ensures a missing resource surfaces a
// clear error rather than an empty string.
func TestGetResource_NotFound_ReturnsError(t *testing.T) {
	cl := newFakeClusterClient()
	a := newAppWithFakes(cl)

	_, err := a.GetResource("pods", "default", "ghost")
	if err == nil {
		t.Fatal("expected error for missing resource, got nil")
	}
}

// ---------------------------------------------------------------------------
// DeleteResource
// ---------------------------------------------------------------------------

// TestDeleteResource_CallsBackend_NoReadOnly verifies a non-read-only delete
// reaches the underlying cluster client and returns nil on success.
func TestDeleteResource_CallsBackend_NoReadOnly(t *testing.T) {
	isolateConfigDir(t) // no config.yaml -> readOnly=false by default
	cl := newFakeClusterClient()
	cl.addResource("pods", podObject("victim", "default", "Running"))
	a := newAppWithFakes(cl)

	if err := a.DeleteResource("pods", "default", "victim"); err != nil {
		t.Fatalf("DeleteResource: %v", err)
	}
	if cl.deleteCalls != 1 {
		t.Errorf("expected 1 delete call, got %d", cl.deleteCalls)
	}
}

// TestDeleteResource_ReadOnlyBlocks verifies that when readOnly is set in the
// config, DeleteResource refuses to make the backend call.
func TestDeleteResource_ReadOnlyBlocks(t *testing.T) {
	withReadOnlyConfig(t, true)
	cl := newFakeClusterClient()
	a := newAppWithFakes(cl)

	err := a.DeleteResource("pods", "default", "anything")
	if err == nil {
		t.Fatal("expected read-only mode to block delete, got nil")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("error should mention read-only, got: %v", err)
	}
	if cl.deleteCalls != 0 {
		t.Errorf("read-only mode must not call the backend, got %d calls", cl.deleteCalls)
	}
}

// ---------------------------------------------------------------------------
// GetClusterEdges
// ---------------------------------------------------------------------------

// TestGetClusterEdges_ServiceToPod_SelectorMatching seeds a Service whose
// selector matches two of three pods, and confirms exactly two service-to-pod
// edges are produced.
func TestGetClusterEdges_ServiceToPod_SelectorMatching(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("services", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "web", "namespace": "default"},
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{"app": "web"},
		},
	})
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "match-1", "namespace": "default",
			"labels": map[string]interface{}{"app": "web"},
		},
	})
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "match-2", "namespace": "default",
			"labels": map[string]interface{}{"app": "web", "tier": "fe"},
		},
	})
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "no-match", "namespace": "default",
			"labels": map[string]interface{}{"app": "db"},
		},
	})
	a := newAppWithFakes(cl)

	edges, err := a.GetClusterEdges("default")
	if err != nil {
		t.Fatalf("GetClusterEdges: %v", err)
	}

	svcEdges := filterEdges(edges, "service-to-pod")
	if len(svcEdges) != 2 {
		t.Fatalf("expected 2 service-to-pod edges, got %d: %+v", len(svcEdges), svcEdges)
	}
	for _, e := range svcEdges {
		if e.Source != "Service/default/web" {
			t.Errorf("edge.Source = %q, want Service/default/web", e.Source)
		}
		if !strings.HasPrefix(e.Target, "Pod/default/match-") {
			t.Errorf("edge.Target = %q, want Pod/default/match-*", e.Target)
		}
	}
}

// TestGetClusterEdges_ServiceToPod_NamespaceIsolation verifies that a
// service's selector never reaches pods in a foreign namespace.
func TestGetClusterEdges_ServiceToPod_NamespaceIsolation(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("services", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "web", "namespace": "prod"},
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{"app": "web"},
		},
	})
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "pod-in-dev", "namespace": "dev",
			"labels": map[string]interface{}{"app": "web"},
		},
	})
	a := newAppWithFakes(cl)

	edges, err := a.GetClusterEdges("") // all namespaces
	if err != nil {
		t.Fatalf("GetClusterEdges: %v", err)
	}
	for _, e := range filterEdges(edges, "service-to-pod") {
		t.Errorf("unexpected cross-namespace service-to-pod edge: %+v", e)
	}
}

// TestGetClusterEdges_DeploymentReplicaSetPodChain verifies the controller
// chain Deployment -> ReplicaSet -> Pod is collapsed into one
// controller-to-pod edge.
func TestGetClusterEdges_DeploymentReplicaSetPodChain(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("replicasets", map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "web-abc", "namespace": "default",
			"ownerReferences": []interface{}{
				map[string]interface{}{"kind": "Deployment", "name": "web"},
			},
		},
	})
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "web-abc-1", "namespace": "default",
			"ownerReferences": []interface{}{
				map[string]interface{}{"kind": "ReplicaSet", "name": "web-abc"},
			},
		},
	})
	a := newAppWithFakes(cl)

	edges, err := a.GetClusterEdges("default")
	if err != nil {
		t.Fatalf("GetClusterEdges: %v", err)
	}
	ctrl := filterEdges(edges, "controller-to-pod")
	if len(ctrl) != 1 {
		t.Fatalf("expected 1 controller-to-pod edge, got %d: %+v", len(ctrl), ctrl)
	}
	if ctrl[0].Source != "Deployment/default/web" {
		t.Errorf("edge.Source = %q, want Deployment/default/web", ctrl[0].Source)
	}
	if ctrl[0].Target != "Pod/default/web-abc-1" {
		t.Errorf("edge.Target = %q, want Pod/default/web-abc-1", ctrl[0].Target)
	}
}

// TestGetClusterEdges_DaemonSetOwnership exercises the direct DaemonSet ->
// Pod ownership branch.
func TestGetClusterEdges_DaemonSetOwnership(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "nodeagent-xyz", "namespace": "kube-system",
			"ownerReferences": []interface{}{
				map[string]interface{}{"kind": "DaemonSet", "name": "nodeagent"},
			},
		},
	})
	a := newAppWithFakes(cl)

	edges, err := a.GetClusterEdges("kube-system")
	if err != nil {
		t.Fatalf("GetClusterEdges: %v", err)
	}
	ctrl := filterEdges(edges, "controller-to-pod")
	if len(ctrl) != 1 {
		t.Fatalf("expected 1 controller-to-pod edge, got %d: %+v", len(ctrl), ctrl)
	}
	if ctrl[0].Source != "DaemonSet/kube-system/nodeagent" {
		t.Errorf("edge.Source = %q, want DaemonSet/kube-system/nodeagent", ctrl[0].Source)
	}
}

// TestGetClusterEdges_OrphanPod_NoEdges pins the regression: a pod with no
// owner references must not produce any controller edges.
func TestGetClusterEdges_OrphanPod_NoEdges(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "orphan", "namespace": "default",
		},
	})
	a := newAppWithFakes(cl)

	edges, err := a.GetClusterEdges("default")
	if err != nil {
		t.Fatalf("GetClusterEdges: %v", err)
	}
	if got := filterEdges(edges, "controller-to-pod"); len(got) != 0 {
		t.Errorf("orphan pod must not produce controller-to-pod edges, got %+v", got)
	}
}

// TestGetClusterEdges_IngressToService exercises the ingress-to-service edge
// derived from backend service references on an Ingress rule.
func TestGetClusterEdges_IngressToService(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("services", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "web", "namespace": "default"},
		"spec":     map[string]interface{}{"selector": map[string]interface{}{"app": "web"}},
	})
	cl.addResource("ingresses", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "web-ingress", "namespace": "default"},
		"spec": map[string]interface{}{
			"rules": []interface{}{
				map[string]interface{}{
					"host": "example.com",
					"http": map[string]interface{}{
						"paths": []interface{}{
							map[string]interface{}{
								"path": "/",
								"backend": map[string]interface{}{
									"service": map[string]interface{}{
										"name": "web",
										"port": map[string]interface{}{"number": float64(80)},
									},
								},
							},
						},
					},
				},
			},
		},
	})
	a := newAppWithFakes(cl)

	edges, err := a.GetClusterEdges("default")
	if err != nil {
		t.Fatalf("GetClusterEdges: %v", err)
	}
	ingEdges := filterEdges(edges, "ingress-to-service")
	if len(ingEdges) != 1 {
		t.Fatalf("expected 1 ingress-to-service edge, got %d: %+v", len(ingEdges), ingEdges)
	}
	if ingEdges[0].Target != "Service/default/web" {
		t.Errorf("ingress-to-service target = %q, want Service/default/web", ingEdges[0].Target)
	}
}

// TestGetClusterEdges_MissingServicesError bubbles up — we want to know when
// the primary list call fails.
func TestGetClusterEdges_MissingServicesError(t *testing.T) {
	cl := newFakeClusterClient()
	cl.listErr["services"] = fmt.Errorf("boom")
	a := newAppWithFakes(cl)

	_, err := a.GetClusterEdges("default")
	if err == nil {
		t.Fatal("expected error when services list fails, got nil")
	}
	if !strings.Contains(err.Error(), "services") {
		t.Errorf("error should reference the failing list, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetSecretData (Secret decoding)
// ---------------------------------------------------------------------------

// TestGetSecretData_DecodesBase64 ensures base64-encoded values are decoded
// into plain strings for the caller.
func TestGetSecretData_DecodesBase64(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("secrets", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "s", "namespace": "default"},
		"data": map[string]interface{}{
			"user":     base64.StdEncoding.EncodeToString([]byte("alice")),
			"password": base64.StdEncoding.EncodeToString([]byte("wonderland")),
		},
	})
	a := newAppWithFakes(cl)

	data, err := a.GetSecretData("default", "s")
	if err != nil {
		t.Fatalf("GetSecretData: %v", err)
	}
	if data["user"] != "alice" {
		t.Errorf("data[user] = %q, want alice", data["user"])
	}
	if data["password"] != "wonderland" {
		t.Errorf("data[password] = %q, want wonderland", data["password"])
	}
}

// ---------------------------------------------------------------------------
// Pure helpers — parseSelectors / parseBackends / splitAndTrim / matchLabels
// ---------------------------------------------------------------------------

func TestParseSelectors(t *testing.T) {
	cases := []struct {
		in   string
		want map[string]string
	}{
		{"", map[string]string{}},
		{"app=web", map[string]string{"app": "web"}},
		{"app=web, tier=frontend", map[string]string{"app": "web", "tier": "frontend"}},
		{"  app=web ,  tier=fe  ", map[string]string{"app": "web", "tier": "fe"}},
		{"malformed", map[string]string{}}, // no "="
	}
	for _, tc := range cases {
		got := parseSelectors(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("parseSelectors(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseBackends(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"svc1:80", []string{"svc1"}},
		{"svc1:80, svc2:8080", []string{"svc1", "svc2"}},
		{"noport", []string{"noport"}},
	}
	for _, tc := range cases {
		got := parseBackends(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("parseBackends(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestSplitAndTrim(t *testing.T) {
	cases := []struct {
		in   string
		sep  string
		want []string
	}{
		{"", ",", nil},
		{"a,b,c", ",", []string{"a", "b", "c"}},
		{"  a , b ,c", ",", []string{"a", "b", "c"}},
		{",,", ",", nil}, // empty parts dropped
	}
	for _, tc := range cases {
		got := splitAndTrim(tc.in, tc.sep)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("splitAndTrim(%q,%q) = %v, want %v", tc.in, tc.sep, got, tc.want)
		}
	}
}

func TestMatchLabels(t *testing.T) {
	labels := map[string]string{"app": "web", "tier": "fe", "env": "prod"}

	if !matchLabels(labels, map[string]string{"app": "web"}) {
		t.Error("expected match on single label")
	}
	if !matchLabels(labels, map[string]string{"app": "web", "tier": "fe"}) {
		t.Error("expected match on subset of labels")
	}
	if matchLabels(labels, map[string]string{"app": "db"}) {
		t.Error("must not match when value differs")
	}
	if matchLabels(labels, map[string]string{"missing": "x"}) {
		t.Error("must not match when label key is missing")
	}
	if !matchLabels(labels, map[string]string{}) {
		t.Error("empty selector matches anything")
	}
}

// ---------------------------------------------------------------------------
// extractExtendedMetadata — representative kinds
// ---------------------------------------------------------------------------

// TestExtractExtendedMetadata_Pod_Images_Restarts_SecurityIssues pins the
// enrichment logic for pods: images, restart count, security issues
// (privileged, root-user, hostPath, no-resources), probe flags.
func TestExtractExtendedMetadata_Pod_Images_Restarts_SecurityIssues(t *testing.T) {
	raw, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "p", "namespace": "default"},
		"spec": map[string]interface{}{
			"nodeName": "node-1",
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "web",
					"image": "nginx:1.25",
					// resources missing -> "no-resources"
					"securityContext": map[string]interface{}{
						"privileged": true,
						"runAsUser":  float64(0),
					},
					"livenessProbe":  map[string]interface{}{},
					"readinessProbe": map[string]interface{}{},
				},
			},
			"volumes": []interface{}{
				map[string]interface{}{
					"name":     "host",
					"hostPath": map[string]interface{}{"path": "/etc"},
				},
			},
		},
		"status": map[string]interface{}{
			"qosClass": "Burstable",
			"containerStatuses": []interface{}{
				map[string]interface{}{
					"name":         "web",
					"ready":        true,
					"restartCount": float64(3),
				},
			},
		},
	})
	info := &ResourceInfo{}
	extractExtendedMetadata(info, raw, "pods")

	if info.Node != "node-1" {
		t.Errorf("Node = %q, want node-1", info.Node)
	}
	if info.QoSClass != "Burstable" {
		t.Errorf("QoSClass = %q, want Burstable", info.QoSClass)
	}
	if info.Restarts != 3 {
		t.Errorf("Restarts = %d, want 3", info.Restarts)
	}
	if info.ReadyContainers != "1/1" {
		t.Errorf("ReadyContainers = %q, want 1/1", info.ReadyContainers)
	}
	if !info.HasLiveness || !info.HasReadiness {
		t.Errorf("expected HasLiveness && HasReadiness, got %v %v", info.HasLiveness, info.HasReadiness)
	}
	if len(info.Images) != 1 || info.Images[0] != "nginx:1.25" {
		t.Errorf("Images = %v, want [nginx:1.25]", info.Images)
	}
	for _, want := range []string{"privileged", "root-user", "no-resources", "hostPath"} {
		if !contains(info.SecurityIssues, want) {
			t.Errorf("SecurityIssues missing %q, got %v", want, info.SecurityIssues)
		}
	}
}

// TestExtractExtendedMetadata_Service_Selectors_Ports surfaces service metadata.
func TestExtractExtendedMetadata_Service_Selectors_Ports(t *testing.T) {
	raw, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "svc", "namespace": "default"},
		"spec": map[string]interface{}{
			"type":      "ClusterIP",
			"clusterIP": "10.0.0.5",
			"selector":  map[string]interface{}{"app": "web"},
			"ports": []interface{}{
				map[string]interface{}{"port": float64(80), "protocol": "TCP"},
				map[string]interface{}{"port": float64(443)},
			},
		},
	})
	info := &ResourceInfo{}
	extractExtendedMetadata(info, raw, "services")

	if info.ServiceType != "ClusterIP" {
		t.Errorf("ServiceType = %q", info.ServiceType)
	}
	if info.ClusterIP != "10.0.0.5" {
		t.Errorf("ClusterIP = %q", info.ClusterIP)
	}
	if !strings.Contains(info.Ports, "80/TCP") || !strings.Contains(info.Ports, "443/TCP") {
		t.Errorf("Ports = %q, want 80/TCP and 443/TCP", info.Ports)
	}
	if !strings.Contains(info.Selectors, "app=web") {
		t.Errorf("Selectors = %q, want app=web", info.Selectors)
	}
}

// TestExtractExtendedMetadata_ConfigMap_DataKeys collects the top-level data
// keys for the UI.
func TestExtractExtendedMetadata_ConfigMap_DataKeys(t *testing.T) {
	raw, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "cm", "namespace": "default"},
		"data":     map[string]interface{}{"key1": "v1", "key2": "v2"},
	})
	info := &ResourceInfo{}
	extractExtendedMetadata(info, raw, "configmaps")

	if info.DataCount != 2 {
		t.Errorf("DataCount = %d, want 2", info.DataCount)
	}
	sort.Strings(info.DataKeys)
	if !reflect.DeepEqual(info.DataKeys, []string{"key1", "key2"}) {
		t.Errorf("DataKeys = %v, want [key1 key2]", info.DataKeys)
	}
}

// TestExtractExtendedMetadata_Ingress_HostsAndBackends pulls hosts/paths/
// backends out of an Ingress spec — these feed the graph edge builder.
func TestExtractExtendedMetadata_Ingress_HostsAndBackends(t *testing.T) {
	raw, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "ing", "namespace": "default"},
		"spec": map[string]interface{}{
			"ingressClassName": "nginx",
			"rules": []interface{}{
				map[string]interface{}{
					"host": "example.com",
					"http": map[string]interface{}{
						"paths": []interface{}{
							map[string]interface{}{
								"path": "/",
								"backend": map[string]interface{}{
									"service": map[string]interface{}{
										"name": "web",
										"port": map[string]interface{}{"number": float64(80)},
									},
								},
							},
						},
					},
				},
			},
			"tls": []interface{}{
				map[string]interface{}{"hosts": []interface{}{"example.com"}},
			},
		},
	})
	info := &ResourceInfo{}
	extractExtendedMetadata(info, raw, "ingresses")

	if info.IngressClass != "nginx" {
		t.Errorf("IngressClass = %q, want nginx", info.IngressClass)
	}
	if info.Hosts != "example.com" {
		t.Errorf("Hosts = %q, want example.com", info.Hosts)
	}
	if info.Paths != "/" {
		t.Errorf("Paths = %q, want /", info.Paths)
	}
	if !strings.Contains(info.Backends, "web:80") {
		t.Errorf("Backends = %q, want web:80", info.Backends)
	}
	if info.TLSHosts != "example.com" {
		t.Errorf("TLSHosts = %q, want example.com", info.TLSHosts)
	}
}

// TestExtractExtendedMetadata_Node_RolesConditions checks that node labels
// expose roles and that status.conditions are summarized.
func TestExtractExtendedMetadata_Node_RolesConditions(t *testing.T) {
	raw, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "n1",
			"labels": map[string]interface{}{
				"node-role.kubernetes.io/control-plane": "",
				"node-role.kubernetes.io/master":        "",
			},
		},
		"spec": map[string]interface{}{
			"unschedulable": true,
			"taints": []interface{}{
				map[string]interface{}{"key": "NoSchedule", "effect": "NoSchedule"},
			},
		},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
				map[string]interface{}{"type": "MemoryPressure", "status": "True"},
				map[string]interface{}{"type": "DiskPressure", "status": "False"},
			},
			"nodeInfo": map[string]interface{}{
				"kubeletVersion":          "v1.28.0",
				"containerRuntimeVersion": "containerd://1.7",
				"osImage":                 "Ubuntu 22.04",
				"architecture":            "amd64",
			},
		},
	})
	info := &ResourceInfo{}
	extractExtendedMetadata(info, raw, "nodes")

	if !strings.Contains(info.Roles, "control-plane") || !strings.Contains(info.Roles, "master") {
		t.Errorf("Roles = %q, want control-plane,master", info.Roles)
	}
	if !info.Unschedulable {
		t.Error("Unschedulable should be true")
	}
	if info.KubeletVersion != "v1.28.0" {
		t.Errorf("KubeletVersion = %q", info.KubeletVersion)
	}
	if !contains(info.NodeConditions, "Ready") {
		t.Errorf("NodeConditions missing Ready, got %v", info.NodeConditions)
	}
	if !contains(info.NodeConditions, "MemoryPressure") {
		t.Errorf("NodeConditions missing MemoryPressure, got %v", info.NodeConditions)
	}
	if contains(info.NodeConditions, "DiskPressure") {
		t.Errorf("NodeConditions should omit DiskPressure (status=False), got %v", info.NodeConditions)
	}
}

// TestExtractExtendedMetadata_InvalidJSON_NoPanic confirms defensive handling.
func TestExtractExtendedMetadata_InvalidJSON_NoPanic(t *testing.T) {
	info := &ResourceInfo{}
	extractExtendedMetadata(info, []byte("not json"), "pods")
	// Nothing to assert — we only care that no panic occurred.
}

// ---------------------------------------------------------------------------
// parseResourceQuantity / formatCPU / formatMemory
// ---------------------------------------------------------------------------

func TestParseResourceQuantity_Units(t *testing.T) {
	cases := []struct {
		in   interface{}
		want int64
	}{
		{"500m", 500},
		{"2", 2},
		{"1Ki", 1024},
		{"1Mi", 1024 * 1024},
		{"1Gi", 1024 * 1024 * 1024},
		{"1K", 1000},
		{nil, 0},
	}
	for _, tc := range cases {
		got := parseResourceQuantity(tc.in)
		if got != tc.want {
			t.Errorf("parseResourceQuantity(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestFormatCPU(t *testing.T) {
	if got := formatCPU(250); got != "250m" {
		t.Errorf("formatCPU(250) = %q", got)
	}
	if got := formatCPU(1500); got != "1.5" {
		t.Errorf("formatCPU(1500) = %q", got)
	}
}

func TestFormatMemory(t *testing.T) {
	if got := formatMemory(512); got != "512" {
		t.Errorf("formatMemory(512) = %q", got)
	}
	if got := formatMemory(2 * 1024); got != "2Ki" {
		t.Errorf("formatMemory(2Ki) = %q", got)
	}
	if got := formatMemory(3 * 1024 * 1024); got != "3Mi" {
		t.Errorf("formatMemory(3Mi) = %q", got)
	}
	if got := formatMemory(int64(2.5 * 1024 * 1024 * 1024)); !strings.HasPrefix(got, "2.") {
		t.Errorf("formatMemory(2.5Gi) = %q, want 2.*Gi", got)
	}
}

// ---------------------------------------------------------------------------
// formatDuration
// ---------------------------------------------------------------------------

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{10 * time.Minute, "<1h"},
		{5 * time.Hour, "<1d"},
		{48 * time.Hour, "<1w"},
		{8 * 24 * time.Hour, "<1mo"},
		{60 * 24 * time.Hour, "<1y"},
		{400 * 24 * time.Hour, ">1y"},
	}
	for _, tc := range cases {
		if got := formatDuration(tc.in); got != tc.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}

	if got := formatDuration("not-a-duration"); got != "unknown" {
		t.Errorf("formatDuration(string) = %q, want unknown", got)
	}
}

// ---------------------------------------------------------------------------
// helpers specific to this file
// ---------------------------------------------------------------------------

func filterEdges(edges []ClusterEdge, edgeType string) []ClusterEdge {
	var out []ClusterEdge
	for _, e := range edges {
		if e.EdgeType == edgeType {
			out = append(out, e)
		}
	}
	return out
}

func podObject(name, namespace, phase string) map[string]interface{} {
	return map[string]interface{}{
		"kind":       "Pod",
		"apiVersion": "v1",
		"metadata":   map[string]interface{}{"name": name, "namespace": namespace},
		"status":     map[string]interface{}{"phase": phase},
	}
}

// ensure unused imports stay referenced when test matrix shifts
var _ = client.ErrResourceNotFound
