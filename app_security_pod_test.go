// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"strings"
	"testing"

	"github.com/thepixelabs/kubecat/internal/client"
)

// ---------------------------------------------------------------------------
// scanPodSecurityIssues — additional gaps pinned (one helper test per
// currently-ignored field). These are intentionally asserting 0 findings so
// that when coverage improves, the assertions flip and the fix is visible.
// ---------------------------------------------------------------------------

// TestScanPodSecurityIssues_HostIPC_CurrentlyIgnored pins that hostIPC: true
// does NOT currently produce a finding. Flip to expect >=1 High issue when
// the scanner is extended to cover hostIPC.
func TestScanPodSecurityIssues_HostIPC_CurrentlyIgnored(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "ipc", "namespace": "default"},
		"spec": map[string]interface{}{
			"hostIPC":    true,
			"containers": []interface{}{},
		},
	})
	issues := scanPodSecurityIssues(cl.resources["pods"][0])
	if len(issues) != 0 {
		t.Errorf("PIN: hostIPC should currently produce 0 findings; got %d (scanner may have been extended — update assertion to expect High severity)",
			len(issues))
	}
}

// TestScanPodSecurityIssues_ReadOnlyRootFS_CurrentlyIgnored pins that the
// absence of readOnlyRootFilesystem is not flagged today.
func TestScanPodSecurityIssues_ReadOnlyRootFS_CurrentlyIgnored(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "writeable", "namespace": "default"},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name": "c",
					// securityContext explicitly sets readOnlyRootFilesystem=false.
					"securityContext": map[string]interface{}{},
				},
			},
		},
	})
	issues := scanPodSecurityIssues(cl.resources["pods"][0])
	if len(issues) != 0 {
		t.Errorf("PIN: non-readOnlyRootFilesystem currently produces no findings; got %d", len(issues))
	}
}

// TestScanPodSecurityIssues_AllowPrivilegeEscalation_CurrentlyIgnored pins
// that allowPrivilegeEscalation=true is not flagged today.
func TestScanPodSecurityIssues_AllowPrivilegeEscalation_CurrentlyIgnored(t *testing.T) {
	cl := newFakeClusterClient()
	allow := true
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "esc", "namespace": "default"},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name": "c",
					"securityContext": map[string]interface{}{
						"allowPrivilegeEscalation": &allow,
					},
				},
			},
		},
	})
	issues := scanPodSecurityIssues(cl.resources["pods"][0])
	if len(issues) != 0 {
		t.Errorf("PIN: allowPrivilegeEscalation=true currently produces no findings; got %d (scanner extended? flip to expect >=1)",
			len(issues))
	}
}

// TestScanPodSecurityIssues_ContainerWithoutSecurityContext_NoPanic verifies
// a pod with containers but no securityContext field doesn't panic.
func TestScanPodSecurityIssues_ContainerWithoutSecurityContext_NoPanic(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "bare", "namespace": "default"},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{"name": "c", "image": "nginx"},
			},
		},
	})
	issues := scanPodSecurityIssues(cl.resources["pods"][0])
	if len(issues) != 0 {
		t.Errorf("bare container should produce 0 findings, got %d", len(issues))
	}
}

// TestScanPodSecurityIssues_MultipleContainers_EachEvaluated verifies that
// each container is scanned independently.
func TestScanPodSecurityIssues_MultipleContainers_EachEvaluated(t *testing.T) {
	cl := newFakeClusterClient()
	priv := true
	uid := int64(0)
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "multi", "namespace": "default"},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name": "priv-c",
					"securityContext": map[string]interface{}{
						"privileged": &priv,
					},
				},
				map[string]interface{}{
					"name": "root-c",
					"securityContext": map[string]interface{}{
						"runAsUser": &uid,
					},
				},
			},
		},
	})
	issues := scanPodSecurityIssues(cl.resources["pods"][0])
	// Expect exactly one Critical (privileged) and one High (root), but
	// order-invariant.
	var critical, high int
	for _, i := range issues {
		switch i.Severity {
		case "Critical":
			critical++
		case "High":
			high++
		}
	}
	if critical != 1 {
		t.Errorf("Critical = %d, want 1", critical)
	}
	if high != 1 {
		t.Errorf("High = %d, want 1", high)
	}
}

// TestScanPodSecurityIssues_RunAsUserNonZero_NotFlagged verifies only UID 0
// triggers the root finding (UID 1000 is explicitly NOT flagged).
func TestScanPodSecurityIssues_RunAsUserNonZero_NotFlagged(t *testing.T) {
	cl := newFakeClusterClient()
	uid := int64(1000)
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "nonroot", "namespace": "default"},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name": "c",
					"securityContext": map[string]interface{}{
						"runAsUser": &uid,
					},
				},
			},
		},
	})
	issues := scanPodSecurityIssues(cl.resources["pods"][0])
	for _, i := range issues {
		if strings.Contains(i.Title, "root") {
			t.Errorf("non-zero UID (%d) must not trigger root finding; got %+v", uid, i)
		}
	}
}

// ---------------------------------------------------------------------------
// scanRBACSecurityIssues — service account exemptions
// ---------------------------------------------------------------------------

// TestScanRBACSecurityIssues_ExemptKubeSystemSAs verifies kube-system SAs
// bound to cluster-admin are exempt.
func TestScanRBACSecurityIssues_ExemptKubeSystemSAs(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("clusterrolebindings", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "sys-ca"},
		"roleRef":  map[string]interface{}{"kind": "ClusterRole", "name": "cluster-admin"},
		"subjects": []interface{}{
			map[string]interface{}{
				"kind": "ServiceAccount", "namespace": "kube-system", "name": "controller",
			},
		},
	})
	a := &App{}
	issues := a.scanRBACSecurityIssues(context.Background(), cl)
	if len(issues) != 0 {
		t.Errorf("kube-system SA must be exempt, got %d issues", len(issues))
	}
}

// TestScanRBACSecurityIssues_ExemptDefaultSA verifies the "default" SA in any
// ns is currently exempt.
func TestScanRBACSecurityIssues_ExemptDefaultSA(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("clusterrolebindings", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "default-ca"},
		"roleRef":  map[string]interface{}{"kind": "ClusterRole", "name": "cluster-admin"},
		"subjects": []interface{}{
			map[string]interface{}{
				"kind": "ServiceAccount", "namespace": "team-a", "name": "default",
			},
		},
	})
	a := &App{}
	issues := a.scanRBACSecurityIssues(context.Background(), cl)
	if len(issues) != 0 {
		t.Errorf("'default' SA must be exempt in any ns; got %d issues", len(issues))
	}
}

// TestScanRBACSecurityIssues_UserBoundToClusterAdmin_Flagged flags a real user.
func TestScanRBACSecurityIssues_UserBoundToClusterAdmin_Flagged(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("clusterrolebindings", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "alice-admin"},
		"roleRef":  map[string]interface{}{"kind": "ClusterRole", "name": "cluster-admin"},
		"subjects": []interface{}{
			map[string]interface{}{"kind": "User", "name": "alice"},
		},
	})
	a := &App{}
	issues := a.scanRBACSecurityIssues(context.Background(), cl)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for alice cluster-admin, got %d", len(issues))
	}
	if issues[0].Severity != "High" {
		t.Errorf("Severity = %q, want High", issues[0].Severity)
	}
	if issues[0].Category != "RBAC" {
		t.Errorf("Category = %q, want RBAC", issues[0].Category)
	}
}

// TestScanRBACSecurityIssues_InvalidJSON_Skipped ensures malformed raw bytes
// don't break the scan — the row is skipped.
func TestScanRBACSecurityIssues_InvalidJSON_Skipped(t *testing.T) {
	cl := newFakeClusterClient()
	cl.resources["clusterrolebindings"] = []client.Resource{
		{Kind: "clusterrolebindings", Raw: []byte("not-json")},
	}
	a := &App{}
	issues := a.scanRBACSecurityIssues(context.Background(), cl)
	if len(issues) != 0 {
		t.Errorf("malformed CRB must yield 0 issues, got %d", len(issues))
	}
}
