// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"

	"github.com/thepixelabs/kubecat/internal/client"
	"github.com/thepixelabs/kubecat/internal/core"
)

// fakeClusterClient is an in-memory client.ClusterClient for tests.
// It supports List, Get and synthesizes Resource records with Raw/Object set
// from arbitrary map inputs.
type fakeClusterClient struct {
	resources map[string][]client.Resource // kind -> items
	listErr   map[string]error             // kind -> error override
	getErr    map[string]error             // "kind/ns/name" -> error

	// deleteCalls counts how many times Delete has been invoked.
	deleteCalls int
	// deleteErr, when non-nil, is returned by every Delete invocation.
	deleteErr error

	// logsFn overrides Logs behavior when non-nil.
	logsFn func(ctx context.Context, namespace, pod, container string, follow bool, tailLines int64) (<-chan string, error)

	// watchFn overrides Watch behavior when non-nil.
	watchFn func(ctx context.Context, kind string, opts client.WatchOptions) (<-chan client.WatchEvent, error)
}

func newFakeClusterClient() *fakeClusterClient {
	return &fakeClusterClient{
		resources: make(map[string][]client.Resource),
		listErr:   make(map[string]error),
		getErr:    make(map[string]error),
	}
}

// addResource inserts a Kubernetes-like object under kind, populating Name,
// Namespace, Labels, Raw, and Object from its metadata.
func (f *fakeClusterClient) addResource(kind string, obj map[string]interface{}) {
	raw, _ := json.Marshal(obj)

	var name, namespace string
	var labels map[string]string
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		name, _ = meta["name"].(string)
		namespace, _ = meta["namespace"].(string)
		if lbls, ok := meta["labels"].(map[string]interface{}); ok {
			labels = make(map[string]string, len(lbls))
			for k, v := range lbls {
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
	f.resources[kind] = append(f.resources[kind], r)
}

func (f *fakeClusterClient) Info(_ context.Context) (*client.ClusterInfo, error) {
	return &client.ClusterInfo{Name: "fake"}, nil
}

func (f *fakeClusterClient) List(_ context.Context, kind string, opts client.ListOptions) (*client.ResourceList, error) {
	if err, ok := f.listErr[kind]; ok {
		return nil, err
	}
	items := f.resources[kind]
	if opts.Namespace != "" {
		var filtered []client.Resource
		for _, r := range items {
			if r.Namespace == opts.Namespace {
				filtered = append(filtered, r)
			}
		}
		items = filtered
	}
	return &client.ResourceList{Items: items, Total: len(items)}, nil
}

func (f *fakeClusterClient) Get(_ context.Context, kind, namespace, name string) (*client.Resource, error) {
	key := kind + "/" + namespace + "/" + name
	if err, ok := f.getErr[key]; ok {
		return nil, err
	}
	for _, r := range f.resources[kind] {
		if r.Name == name && r.Namespace == namespace {
			return &r, nil
		}
	}
	return nil, client.ErrResourceNotFound
}

func (f *fakeClusterClient) Delete(_ context.Context, _, _, _ string) error {
	f.deleteCalls++
	return f.deleteErr
}
func (f *fakeClusterClient) Watch(ctx context.Context, kind string, opts client.WatchOptions) (<-chan client.WatchEvent, error) {
	if f.watchFn != nil {
		return f.watchFn(ctx, kind, opts)
	}
	ch := make(chan client.WatchEvent)
	close(ch)
	return ch, nil
}
func (f *fakeClusterClient) Logs(ctx context.Context, namespace, pod, container string, follow bool, tailLines int64) (<-chan string, error) {
	if f.logsFn != nil {
		return f.logsFn(ctx, namespace, pod, container, follow, tailLines)
	}
	ch := make(chan string)
	close(ch)
	return ch, nil
}
func (f *fakeClusterClient) Exec(_ context.Context, _, _, _ string, _ []string) error { return nil }
func (f *fakeClusterClient) PortForward(_ context.Context, _, _ string, _, _ int) (client.PortForwarder, error) {
	return nil, nil
}
func (f *fakeClusterClient) Close() error { return nil }

// fakeManager is an in-memory client.Manager for tests. It wraps a single
// fakeClusterClient addressed as "test" and always returns it for Active()/Get().
type fakeManager struct {
	cl     *fakeClusterClient
	active string
}

func newFakeManager(cl *fakeClusterClient) *fakeManager {
	return &fakeManager{cl: cl, active: "test"}
}

func (m *fakeManager) Add(_ context.Context, _ string) error { return nil }
func (m *fakeManager) Remove(_ string) error                 { return nil }
func (m *fakeManager) Get(_ string) (client.ClusterClient, error) {
	if m.cl == nil {
		return nil, client.ErrContextNotFound
	}
	return m.cl, nil
}
func (m *fakeManager) Active() (client.ClusterClient, error) {
	if m.cl == nil {
		return nil, client.ErrNoActiveCluster
	}
	return m.cl, nil
}
func (m *fakeManager) SetActive(s string) error                      { m.active = s; return nil }
func (m *fakeManager) List() []client.ClusterInfo                    { return nil }
func (m *fakeManager) Contexts() ([]string, error)                   { return []string{"test"}, nil }
func (m *fakeManager) Close() error                                  { return nil }
func (m *fakeManager) ActiveContext() string                         { return m.active }
func (m *fakeManager) RefreshInfo(_ context.Context, _ string) error { return nil }
func (m *fakeManager) ReloadContexts() ([]string, error)             { return []string{"test"}, nil }

// newAppWithFakes constructs a minimal *App wired to the supplied
// fakeClusterClient. The nexus.Clusters is set so a.nexus.Clusters.Manager()
// returns our fakeManager, and a.ctx is set to a fresh background context.
func newAppWithFakes(cl *fakeClusterClient) *App {
	mgr := newFakeManager(cl)
	clusters := core.NewClusterServiceWithManager(mgr)
	nx := &core.Kubecat{
		Clusters:     clusters,
		Resources:    core.NewResourceService(clusters),
		Logs:         core.NewLogService(clusters),
		PortForwards: core.NewPortForwardService(clusters),
	}
	return &App{
		ctx:      context.Background(),
		nexus:    nx,
		watchers: make(map[string]context.CancelFunc),
	}
}
