// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thepixelabs/kubecat/internal/client"
)

func TestKindToPlural_KnownMappings(t *testing.T) {
	tests := []struct {
		kind, want string
	}{
		{"Pod", "pods"},
		{"Deployment", "deployments"},
		{"PersistentVolumeClaim", "persistentvolumeclaims"},
		{"ClusterRoleBinding", "clusterrolebindings"},
	}
	for _, tt := range tests {
		if got := kindToPlural(tt.kind); got != tt.want {
			t.Errorf("kindToPlural(%q) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestKindToPlural_UnknownKind_Pluralizes(t *testing.T) {
	if got := kindToPlural("Widget"); got != "Widgets" {
		t.Errorf("fallback = %q, want Widgets", got)
	}
}

func TestExtractOwnerRefs_Empty(t *testing.T) {
	refs, err := extractOwnerRefs(nil)
	if err != nil {
		t.Errorf("expected no error for empty raw, got %v", err)
	}
	if refs != nil {
		t.Errorf("expected nil refs, got %+v", refs)
	}
}

func TestExtractOwnerRefs_SinglePrimary(t *testing.T) {
	raw, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"ownerReferences": []interface{}{
				map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "ReplicaSet",
					"name":       "rs-abc",
					"uid":        "u-1",
				},
			},
		},
	})
	refs, err := extractOwnerRefs(raw)
	if err != nil {
		t.Fatalf("extractOwnerRefs: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 owner ref, got %d", len(refs))
	}
	if refs[0].Kind != "ReplicaSet" || refs[0].Name != "rs-abc" {
		t.Errorf("unexpected owner: %+v", refs[0])
	}
}

func TestGetOwnerChain_FollowsControllerReference(t *testing.T) {
	cl := newFakeClient()

	// ReplicaSet owned by a Deployment (controller=true).
	ctrl := true
	rs := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "rs-abc",
			"namespace": "default",
			"ownerReferences": []interface{}{
				map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "app",
					"uid":        "dep-1",
					"controller": &ctrl,
				},
			},
		},
	}

	// Deployment has no further owners.
	dep := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "app",
			"namespace": "default",
		},
	}

	cl.addResourceRaw("replicasets", rs)
	cl.addResourceRaw("deployments", dep)

	// Pod -> RS (controller=true)
	podRaw, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "pod-1",
			"namespace": "default",
			"ownerReferences": []interface{}{
				map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "ReplicaSet",
					"name":       "rs-abc",
					"uid":        "rs-uid",
					"controller": &ctrl,
				},
			},
		},
	})
	pod := client.Resource{
		Kind:      "Pod",
		Name:      "pod-1",
		Namespace: "default",
		Raw:       podRaw,
	}

	chain, err := GetOwnerChain(context.Background(), cl, pod)
	if err != nil {
		t.Fatalf("GetOwnerChain: %v", err)
	}
	if len(chain.Owners) != 2 {
		t.Fatalf("expected 2 owners (RS, Deployment), got %d: %+v", len(chain.Owners), chain.Owners)
	}
	if chain.Owners[0].Kind != "ReplicaSet" || chain.Owners[0].Name != "rs-abc" {
		t.Errorf("first owner = %+v, want ReplicaSet/rs-abc", chain.Owners[0])
	}
	if chain.Owners[1].Kind != "Deployment" || chain.Owners[1].Name != "app" {
		t.Errorf("second owner = %+v, want Deployment/app", chain.Owners[1])
	}
}

func TestGetOwnerChain_NoOwnerReferences_ReturnsEmptyChain(t *testing.T) {
	cl := newFakeClient()
	raw, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "orphan", "namespace": "default"},
	})
	resource := client.Resource{Kind: "Pod", Name: "orphan", Namespace: "default", Raw: raw}

	chain, err := GetOwnerChain(context.Background(), cl, resource)
	if err != nil {
		t.Fatalf("GetOwnerChain: %v", err)
	}
	if len(chain.Owners) != 0 {
		t.Errorf("orphan resource should have empty owner chain, got %+v", chain.Owners)
	}
}
