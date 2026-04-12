// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
	"github.com/thepixelabs/kubecat/internal/storage"
)

// ---------------------------------------------------------------------------
// fakeManager — hand-written fake for client.Manager
// ---------------------------------------------------------------------------

type fakeManager struct {
	clusters     map[string]*fakeCluster
	active       string
	contextList  []string
	addErr       error
	setActiveErr error
}

type fakeCluster struct {
	info      *client.ClusterInfo
	resources map[string][]client.Resource
}

func newFakeManager(contextNames ...string) *fakeManager {
	m := &fakeManager{
		clusters:    make(map[string]*fakeCluster),
		contextList: contextNames,
	}
	for _, name := range contextNames {
		m.clusters[name] = &fakeCluster{
			info: &client.ClusterInfo{
				Name:    name,
				Context: name,
				Status:  client.StatusConnected,
				Version: "v1.28.0",
			},
			resources: make(map[string][]client.Resource),
		}
	}
	if len(contextNames) > 0 {
		m.active = contextNames[0]
	}
	return m
}

func (m *fakeManager) Add(_ context.Context, contextName string) error {
	if m.addErr != nil {
		return m.addErr
	}
	if _, exists := m.clusters[contextName]; exists {
		return client.ErrClusterAlreadyExists
	}
	m.clusters[contextName] = &fakeCluster{
		info:      &client.ClusterInfo{Name: contextName, Status: client.StatusConnected},
		resources: make(map[string][]client.Resource),
	}
	return nil
}

func (m *fakeManager) Remove(contextName string) error {
	if _, exists := m.clusters[contextName]; !exists {
		return client.ErrContextNotFound
	}
	delete(m.clusters, contextName)
	if m.active == contextName {
		m.active = ""
	}
	return nil
}

func (m *fakeManager) Get(contextName string) (client.ClusterClient, error) {
	c, ok := m.clusters[contextName]
	if !ok {
		return nil, client.ErrContextNotFound
	}
	return &fakeClusterForManager{cluster: c}, nil
}

func (m *fakeManager) Active() (client.ClusterClient, error) {
	if m.active == "" {
		return nil, client.ErrNoActiveCluster
	}
	c, ok := m.clusters[m.active]
	if !ok {
		return nil, client.ErrNoActiveCluster
	}
	return &fakeClusterForManager{cluster: c}, nil
}

func (m *fakeManager) SetActive(contextName string) error {
	if m.setActiveErr != nil {
		return m.setActiveErr
	}
	if _, ok := m.clusters[contextName]; !ok {
		return client.ErrContextNotFound
	}
	m.active = contextName
	return nil
}

func (m *fakeManager) List() []client.ClusterInfo {
	var infos []client.ClusterInfo
	for _, c := range m.clusters {
		infos = append(infos, *c.info)
	}
	return infos
}

func (m *fakeManager) Contexts() ([]string, error) {
	return m.contextList, nil
}

func (m *fakeManager) Close() error                                  { return nil }
func (m *fakeManager) ActiveContext() string                         { return m.active }
func (m *fakeManager) RefreshInfo(_ context.Context, _ string) error { return nil }
func (m *fakeManager) ReloadContexts() ([]string, error)             { return m.contextList, nil }

// fakeClusterForManager wraps a fakeCluster and implements client.ClusterClient.
type fakeClusterForManager struct {
	cluster *fakeCluster
}

func (f *fakeClusterForManager) Info(_ context.Context) (*client.ClusterInfo, error) {
	return f.cluster.info, nil
}

func (f *fakeClusterForManager) List(_ context.Context, kind string, opts client.ListOptions) (*client.ResourceList, error) {
	items := f.cluster.resources[kind]
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

func (f *fakeClusterForManager) Get(_ context.Context, kind, namespace, name string) (*client.Resource, error) {
	for _, r := range f.cluster.resources[kind] {
		if r.Name == name && r.Namespace == namespace {
			return &r, nil
		}
	}
	return nil, client.ErrResourceNotFound
}

func (f *fakeClusterForManager) Delete(_ context.Context, _, _, _ string) error { return nil }
func (f *fakeClusterForManager) Watch(_ context.Context, _ string, _ client.WatchOptions) (<-chan client.WatchEvent, error) {
	ch := make(chan client.WatchEvent)
	close(ch)
	return ch, nil
}
func (f *fakeClusterForManager) Logs(_ context.Context, _, _, _ string, _ bool, _ int64) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}
func (f *fakeClusterForManager) Exec(_ context.Context, _, _, _ string, _ []string) error { return nil }
func (f *fakeClusterForManager) PortForward(_ context.Context, _, _ string, _, _ int) (client.PortForwarder, error) {
	return nil, nil
}
func (f *fakeClusterForManager) Close() error { return nil }

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func makePodResource(name, namespace, status string) client.Resource {
	raw, _ := json.Marshal(map[string]interface{}{
		"kind": "Pod",
		"metadata": map[string]interface{}{
			"name": name, "namespace": namespace,
			"creationTimestamp": time.Now().Format(time.RFC3339),
		},
		"status": map[string]interface{}{"phase": status},
	})
	return client.Resource{
		Kind:      "Pod",
		Name:      name,
		Namespace: namespace,
		Status:    status,
		CreatedAt: time.Now(),
		Raw:       raw,
	}
}

func openTestStorageDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("openTestStorageDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// ---------------------------------------------------------------------------
// Cluster lifecycle
// ---------------------------------------------------------------------------

func TestClusterService_Connect_SetsActiveContext(t *testing.T) {
	mgr := newFakeManager("dev", "prod")
	svc := &ClusterService{manager: mgr, cacheExpiry: 5 * time.Minute}

	if err := svc.Connect(context.Background(), "prod"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if got := svc.ActiveContext(); got != "prod" {
		t.Errorf("ActiveContext = %q, want prod", got)
	}
}

func TestClusterService_Connect_DuplicateIgnored(t *testing.T) {
	mgr := newFakeManager("dev")
	svc := &ClusterService{manager: mgr, cacheExpiry: 5 * time.Minute}

	// First connect
	if err := svc.Connect(context.Background(), "dev"); err != nil {
		t.Fatalf("first Connect: %v", err)
	}
	// Second connect to same context must succeed (duplicate is tolerated)
	if err := svc.Connect(context.Background(), "dev"); err != nil {
		t.Fatalf("second Connect to same context: %v", err)
	}
}

func TestClusterService_Disconnect_RemovesCluster(t *testing.T) {
	mgr := newFakeManager("dev", "prod")
	svc := &ClusterService{manager: mgr, cacheExpiry: 5 * time.Minute}

	if err := svc.Disconnect("prod"); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if _, err := mgr.Get("prod"); !errors.Is(err, client.ErrContextNotFound) {
		t.Errorf("after Disconnect, Get should return ErrContextNotFound, got %v", err)
	}
}

func TestClusterService_IsConnected_FalseBeforeConnect(t *testing.T) {
	mgr := newFakeManager("dev")
	svc := &ClusterService{manager: mgr, cacheExpiry: 5 * time.Minute}
	if svc.IsConnected() {
		t.Error("IsConnected should be false before any Connect call")
	}
}

func TestClusterService_IsConnected_TrueAfterConnect(t *testing.T) {
	mgr := newFakeManager("dev")
	svc := &ClusterService{manager: mgr, cacheExpiry: 5 * time.Minute}
	if err := svc.Connect(context.Background(), "dev"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if !svc.IsConnected() {
		t.Error("IsConnected should be true after Connect")
	}
}

func TestClusterService_GetContexts_ReturnsContextList(t *testing.T) {
	mgr := newFakeManager("alpha", "beta", "gamma")
	svc := &ClusterService{manager: mgr, cacheExpiry: 5 * time.Minute}

	ctxs, err := svc.GetContexts(context.Background())
	if err != nil {
		t.Fatalf("GetContexts: %v", err)
	}
	if len(ctxs) != 3 {
		t.Errorf("GetContexts returned %d contexts, want 3", len(ctxs))
	}
}

func TestClusterService_GetContexts_CacheHit(t *testing.T) {
	mgr := newFakeManager("dev")
	svc := &ClusterService{manager: mgr, cacheExpiry: 5 * time.Minute}

	// Populate cache
	ctxs1, _ := svc.GetContexts(context.Background())

	// Modify the manager's context list — cache should return old list
	mgr.contextList = append(mgr.contextList, "new-ctx")

	ctxs2, _ := svc.GetContexts(context.Background())
	if len(ctxs2) != len(ctxs1) {
		t.Errorf("cache should return %d contexts, got %d", len(ctxs1), len(ctxs2))
	}
}

// ---------------------------------------------------------------------------
// Resource listing
// ---------------------------------------------------------------------------

func TestResourceService_ListResources_ReturnsAllItems(t *testing.T) {
	mgr := newFakeManager("dev")
	mgr.clusters["dev"].resources["pods"] = []client.Resource{
		makePodResource("pod-a", "default", "Running"),
		makePodResource("pod-b", "default", "Running"),
		makePodResource("pod-c", "kube-system", "Running"),
	}
	_ = mgr.SetActive("dev")

	svc := &ClusterService{manager: mgr, cacheExpiry: 5 * time.Minute, activeContext: "dev"}
	rs := NewResourceService(svc)

	pods, err := rs.ListResources(context.Background(), "pods", "")
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if len(pods) != 3 {
		t.Errorf("expected 3 pods, got %d", len(pods))
	}
}

func TestResourceService_ListResources_FiltersByNamespace(t *testing.T) {
	mgr := newFakeManager("dev")
	mgr.clusters["dev"].resources["pods"] = []client.Resource{
		makePodResource("pod-a", "default", "Running"),
		makePodResource("pod-b", "kube-system", "Running"),
	}
	_ = mgr.SetActive("dev")

	svc := &ClusterService{manager: mgr, cacheExpiry: 5 * time.Minute, activeContext: "dev"}
	rs := NewResourceService(svc)

	pods, err := rs.ListResources(context.Background(), "pods", "default")
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if len(pods) != 1 {
		t.Errorf("expected 1 pod in default ns, got %d", len(pods))
	}
	if pods[0].Name != "pod-a" {
		t.Errorf("expected pod-a, got %s", pods[0].Name)
	}
}

func TestResourceService_GetResource_Found(t *testing.T) {
	mgr := newFakeManager("dev")
	mgr.clusters["dev"].resources["pods"] = []client.Resource{
		makePodResource("target-pod", "default", "Running"),
	}
	_ = mgr.SetActive("dev")

	svc := &ClusterService{manager: mgr, cacheExpiry: 5 * time.Minute, activeContext: "dev"}
	rs := NewResourceService(svc)

	pod, err := rs.GetResource(context.Background(), "pods", "default", "target-pod")
	if err != nil {
		t.Fatalf("GetResource: %v", err)
	}
	if pod.Name != "target-pod" {
		t.Errorf("expected target-pod, got %s", pod.Name)
	}
}

func TestResourceService_GetResource_NotFound(t *testing.T) {
	mgr := newFakeManager("dev")
	_ = mgr.SetActive("dev")

	svc := &ClusterService{manager: mgr, cacheExpiry: 5 * time.Minute, activeContext: "dev"}
	rs := NewResourceService(svc)

	_, err := rs.GetResource(context.Background(), "pods", "default", "ghost")
	if err == nil {
		t.Error("GetResource for non-existent resource should return error")
	}
}

func TestResourceService_ListResources_NoActiveCluster(t *testing.T) {
	mgr := newFakeManager()
	svc := &ClusterService{manager: mgr, cacheExpiry: 5 * time.Minute}
	rs := NewResourceService(svc)

	_, err := rs.ListResources(context.Background(), "pods", "")
	if err == nil {
		t.Error("ListResources with no active cluster should return error")
	}
}

func TestResourceService_GetResourceInfo_ReturnsCorrectFields(t *testing.T) {
	svc := &ClusterService{}
	rs := NewResourceService(svc)

	r := &client.Resource{
		Kind:      "Pod",
		Name:      "my-pod",
		Namespace: "default",
		Status:    "Running",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		Labels:    map[string]string{"app": "web"},
	}

	info := rs.GetResourceInfo(r)
	if info.Kind != "Pod" {
		t.Errorf("info.Kind = %q, want Pod", info.Kind)
	}
	if info.Name != "my-pod" {
		t.Errorf("info.Name = %q, want my-pod", info.Name)
	}
	if info.Age < time.Hour {
		t.Errorf("info.Age = %v, expected >= 1h", info.Age)
	}
	if info.Labels["app"] != "web" {
		t.Errorf("info.Labels[app] = %q, want web", info.Labels["app"])
	}
}

// ---------------------------------------------------------------------------
// Event collection → SQLite
// ---------------------------------------------------------------------------

func TestEventRepository_SaveAndList_RoundTrip(t *testing.T) {
	db := openTestStorageDB(t)
	repo := storage.NewEventRepository(db)
	ctx := context.Background()

	event := &storage.StoredEvent{
		Cluster:   "c1",
		Namespace: "default",
		Kind:      "Pod",
		Name:      "my-pod",
		Type:      "Warning",
		Reason:    "BackOff",
		Message:   "container failed",
		LastSeen:  time.Now(),
		Count:     1,
	}

	if err := repo.Save(ctx, event); err != nil {
		t.Fatalf("Save: %v", err)
	}

	events, err := repo.List(ctx, storage.EventFilter{Cluster: "c1"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Reason != "BackOff" {
		t.Errorf("event Reason = %q, want BackOff", events[0].Reason)
	}
}

func TestEventRepository_Save_UpdatesExistingEvent(t *testing.T) {
	db := openTestStorageDB(t)
	repo := storage.NewEventRepository(db)
	ctx := context.Background()

	event := &storage.StoredEvent{
		Cluster:  "c1",
		Kind:     "Pod",
		Name:     "my-pod",
		Type:     "Warning",
		Reason:   "BackOff",
		LastSeen: time.Now(),
		Count:    1,
	}
	if err := repo.Save(ctx, event); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	// Save again with same cluster+kind+name+reason — should update
	event.Count = 5
	event.Message = "updated message"
	if err := repo.Save(ctx, event); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row after upsert, got %d", count)
	}
}

func TestEventRepository_List_FiltersByNamespace(t *testing.T) {
	db := openTestStorageDB(t)
	repo := storage.NewEventRepository(db)
	ctx := context.Background()

	save := func(ns, name string) {
		_ = repo.Save(ctx, &storage.StoredEvent{
			Cluster: "c1", Namespace: ns, Kind: "Pod", Name: name,
			Type: "Normal", Reason: "Started", LastSeen: time.Now(), Count: 1,
		})
	}
	save("default", "pod-a")
	save("kube-system", "pod-b")

	events, err := repo.List(ctx, storage.EventFilter{Namespace: "default"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, e := range events {
		if e.Namespace != "default" {
			t.Errorf("event namespace = %q, want default", e.Namespace)
		}
	}
}

func TestEventRepository_DeleteOlderThan_RemovesStale(t *testing.T) {
	db := openTestStorageDB(t)
	repo := storage.NewEventRepository(db)
	ctx := context.Background()

	old := &storage.StoredEvent{
		Cluster:  "c1",
		Kind:     "Pod",
		Name:     "old-pod",
		Type:     "Warning",
		Reason:   "Error",
		LastSeen: time.Now().Add(-48 * time.Hour),
		Count:    1,
	}
	fresh := &storage.StoredEvent{
		Cluster:  "c1",
		Kind:     "Pod",
		Name:     "fresh-pod",
		Type:     "Warning",
		Reason:   "Error",
		LastSeen: time.Now(),
		Count:    1,
	}

	_ = repo.Save(ctx, old)
	_ = repo.Save(ctx, fresh)

	cutoff := time.Now().Add(-time.Hour)
	deleted, err := repo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("DeleteOlderThan: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	remaining, _ := repo.Count(ctx)
	if remaining != 1 {
		t.Errorf("expected 1 remaining event, got %d", remaining)
	}
}

func TestEventRepository_List_LimitRespected(t *testing.T) {
	db := openTestStorageDB(t)
	repo := storage.NewEventRepository(db)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		_ = repo.Save(ctx, &storage.StoredEvent{
			Cluster: "c1", Kind: "Pod", Type: "Normal",
			Name: fmt.Sprintf("pod-%d", i), Namespace: "default",
			Reason: "Started", LastSeen: time.Now(), Count: 1,
		})
	}

	events, err := repo.List(ctx, storage.EventFilter{Limit: 3})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("List with Limit=3 returned %d events, want 3", len(events))
	}
}

// ---------------------------------------------------------------------------
// Multi-cluster
// ---------------------------------------------------------------------------

func TestFakeManager_MultiCluster_IndependentResources(t *testing.T) {
	mgr := newFakeManager("cluster-a", "cluster-b")
	mgr.clusters["cluster-a"].resources["pods"] = []client.Resource{
		makePodResource("pod-in-a", "default", "Running"),
	}
	mgr.clusters["cluster-b"].resources["pods"] = []client.Resource{
		makePodResource("pod-in-b", "default", "Running"),
		makePodResource("pod-in-b2", "default", "Running"),
	}

	clA, _ := mgr.Get("cluster-a")
	listA, _ := clA.List(context.Background(), "pods", client.ListOptions{})

	clB, _ := mgr.Get("cluster-b")
	listB, _ := clB.List(context.Background(), "pods", client.ListOptions{})

	if len(listA.Items) != 1 {
		t.Errorf("cluster-a has %d pods, want 1", len(listA.Items))
	}
	if len(listB.Items) != 2 {
		t.Errorf("cluster-b has %d pods, want 2", len(listB.Items))
	}
}

func TestFakeManager_SwitchActiveCluster(t *testing.T) {
	mgr := newFakeManager("dev", "staging")
	svc := &ClusterService{manager: mgr, cacheExpiry: 5 * time.Minute}

	if err := svc.Connect(context.Background(), "dev"); err != nil {
		t.Fatalf("Connect dev: %v", err)
	}
	if svc.ActiveContext() != "dev" {
		t.Fatalf("active context should be dev, got %s", svc.ActiveContext())
	}

	if err := svc.Connect(context.Background(), "staging"); err != nil {
		t.Fatalf("Connect staging: %v", err)
	}
	if svc.ActiveContext() != "staging" {
		t.Errorf("active context should be staging, got %s", svc.ActiveContext())
	}
}

func TestFakeManager_RemoveActiveCluster_ClearsActive(t *testing.T) {
	mgr := newFakeManager("dev")
	_ = mgr.SetActive("dev")

	if err := mgr.Remove("dev"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if mgr.ActiveContext() != "" {
		t.Errorf("after Remove, ActiveContext should be empty, got %q", mgr.ActiveContext())
	}
}

func TestFakeManager_AddDuplicate_ReturnsErrAlreadyExists(t *testing.T) {
	mgr := newFakeManager("dev")
	err := mgr.Add(context.Background(), "dev")
	if !errors.Is(err, client.ErrClusterAlreadyExists) {
		t.Errorf("Add duplicate should return ErrClusterAlreadyExists, got %v", err)
	}
}
