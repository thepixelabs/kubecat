// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"context"
	"encoding/json"

	"github.com/thepixelabs/kubecat/internal/client"
)

// fakeClusterClient is an in-memory client.ClusterClient used across analyzer
// tests. The shape mirrors the one used by internal/security tests so analyzer
// tests and security tests use the same mental model.
type fakeClusterClient struct {
	resources map[string][]client.Resource // kind -> resources
	listErr   map[string]error             // kind -> error
}

func newFakeClient() *fakeClusterClient {
	return &fakeClusterClient{
		resources: make(map[string][]client.Resource),
		listErr:   make(map[string]error),
	}
}

// addResourceRaw adds a resource of the given kind with raw JSON already
// serialized from the provided interface.  Name/Namespace are copied from the
// raw metadata when present so resource.Name/resource.Namespace are populated.
func (f *fakeClusterClient) addResourceRaw(kind string, v interface{}) client.Resource {
	b, _ := json.Marshal(v)
	// Best-effort extract metadata for the struct fields.
	var meta struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
	}
	_ = json.Unmarshal(b, &meta)
	r := client.Resource{
		Kind:      kind,
		Name:      meta.Metadata.Name,
		Namespace: meta.Metadata.Namespace,
		Raw:       b,
	}
	f.resources[kind] = append(f.resources[kind], r)
	return r
}

func (f *fakeClusterClient) Info(_ context.Context) (*client.ClusterInfo, error) {
	return &client.ClusterInfo{Name: "fake"}, nil
}

func (f *fakeClusterClient) List(_ context.Context, kind string, _ client.ListOptions) (*client.ResourceList, error) {
	if err, ok := f.listErr[kind]; ok {
		return nil, err
	}
	items := f.resources[kind]
	return &client.ResourceList{Items: items, Total: len(items)}, nil
}

func (f *fakeClusterClient) Get(_ context.Context, kind, namespace, name string) (*client.Resource, error) {
	for _, r := range f.resources[kind] {
		if r.Namespace == namespace && r.Name == name {
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

// -----------------------------------------------------------------------------
// Factories — minimal raw-JSON builders for pods, deployments, nodes, etc.
// -----------------------------------------------------------------------------

// podOpt mutates a pod raw-map. Used as variadic options on the pod factory
// so tests remain readable: newPod("n", "d", withCrashLoop("app"), withRestarts(5)).
type podOpt func(map[string]interface{})

func newPod(name, namespace string, opts ...podOpt) map[string]interface{} {
	p := map[string]interface{}{
		"metadata": map[string]interface{}{"name": name, "namespace": namespace},
		"spec":     map[string]interface{}{"containers": []interface{}{}},
		"status": map[string]interface{}{
			"phase":             "Running",
			"containerStatuses": []interface{}{},
		},
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

func withPodPhase(phase string) podOpt {
	return func(p map[string]interface{}) {
		p["status"].(map[string]interface{})["phase"] = phase
	}
}

func withPodReason(reason, message string) podOpt {
	return func(p map[string]interface{}) {
		s := p["status"].(map[string]interface{})
		s["reason"] = reason
		s["message"] = message
	}
}

func addContainerStatus(status map[string]interface{}) podOpt {
	return func(p map[string]interface{}) {
		s := p["status"].(map[string]interface{})
		cs := s["containerStatuses"].([]interface{})
		s["containerStatuses"] = append(cs, status)
	}
}

func withCrashLoop(containerName string, restarts int) podOpt {
	return addContainerStatus(map[string]interface{}{
		"name":         containerName,
		"image":        "app:latest",
		"restartCount": float64(restarts),
		"state": map[string]interface{}{
			"waiting": map[string]interface{}{"reason": "CrashLoopBackOff", "message": "back-off restarting"},
		},
	})
}

func withImagePullBackOff(containerName string) podOpt {
	return addContainerStatus(map[string]interface{}{
		"name":         containerName,
		"image":        "bogus:nope",
		"restartCount": float64(0),
		"state": map[string]interface{}{
			"waiting": map[string]interface{}{"reason": "ImagePullBackOff", "message": "pull denied"},
		},
	})
}

func withOOMKilled(containerName string) podOpt {
	return addContainerStatus(map[string]interface{}{
		"name":         containerName,
		"image":        "app:latest",
		"restartCount": float64(2),
		"state":        map[string]interface{}{"running": map[string]interface{}{}},
		"lastState": map[string]interface{}{
			"terminated": map[string]interface{}{"reason": "OOMKilled", "exitCode": float64(137)},
		},
	})
}

func withHighRestarts(containerName string, restarts int) podOpt {
	return addContainerStatus(map[string]interface{}{
		"name":         containerName,
		"image":        "app:latest",
		"restartCount": float64(restarts),
		"state":        map[string]interface{}{"running": map[string]interface{}{}},
	})
}

// -----------------------------------------------------------------------------
// Node factory
// -----------------------------------------------------------------------------

type nodeOpt func(map[string]interface{})

func newNode(name string, opts ...nodeOpt) map[string]interface{} {
	n := map[string]interface{}{
		"metadata": map[string]interface{}{"name": name, "labels": map[string]interface{}{}},
		"spec":     map[string]interface{}{"taints": []interface{}{}},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
			"allocatable": map[string]interface{}{"cpu": "4", "memory": "8Gi"},
		},
	}
	for _, o := range opts {
		o(n)
	}
	return n
}

func withNodeCondition(condType, status string) nodeOpt {
	return func(n map[string]interface{}) {
		s := n["status"].(map[string]interface{})
		conds := s["conditions"].([]interface{})
		// Replace Ready if we're overriding it.
		found := false
		for i, c := range conds {
			cm := c.(map[string]interface{})
			if cm["type"] == condType {
				cm["status"] = status
				conds[i] = cm
				found = true
				break
			}
		}
		if !found {
			conds = append(conds, map[string]interface{}{"type": condType, "status": status})
		}
		s["conditions"] = conds
	}
}

// -----------------------------------------------------------------------------
// PVC factory
// -----------------------------------------------------------------------------

func newPVC(name, namespace, phase string) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{"name": name, "namespace": namespace},
		"spec":     map[string]interface{}{"storageClassName": "standard", "accessModes": []interface{}{"ReadWriteOnce"}},
		"status":   map[string]interface{}{"phase": phase},
	}
}

// -----------------------------------------------------------------------------
// Deployment / StatefulSet / DaemonSet factories
// -----------------------------------------------------------------------------

func newDeployment(name, namespace string, replicas, ready, unavailable int) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{"name": name, "namespace": namespace},
		"spec":     map[string]interface{}{"replicas": float64(replicas)},
		"status": map[string]interface{}{
			"replicas":            float64(replicas),
			"readyReplicas":       float64(ready),
			"unavailableReplicas": float64(unavailable),
			"conditions":          []interface{}{},
		},
	}
}

func withDeploymentCondition(dep map[string]interface{}, condType, status, reason, message string) {
	s := dep["status"].(map[string]interface{})
	conds := s["conditions"].([]interface{})
	conds = append(conds, map[string]interface{}{
		"type": condType, "status": status, "reason": reason, "message": message,
	})
	s["conditions"] = conds
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

// hasIssueID returns true if any issue has the provided ID.
func hasIssueID(issues []Issue, id string) bool {
	for _, i := range issues {
		if i.ID == id {
			return true
		}
	}
	return false
}

// findIssue returns the first issue matching the given ID, or nil.
func findIssue(issues []Issue, id string) *Issue {
	for i := range issues {
		if issues[i].ID == id {
			return &issues[i]
		}
	}
	return nil
}
