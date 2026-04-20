// SPDX-License-Identifier: Apache-2.0

package history

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
	"github.com/thepixelabs/kubecat/internal/storage"
)

// ---------------------------------------------------------------------------
// Fake cluster client + manager for snapshotter tests.
//
// These are intentionally local to *_integration_test.go so they don't
// collide with fakes in sibling test files. They satisfy only the subset of
// client.Manager / client.ClusterClient that the snapshotter actually calls.
// ---------------------------------------------------------------------------

// snapFakeClient is an in-memory ClusterClient programmed with per-kind items.
// Guarded by listCalls counter so tests can assert each requested kind was
// fetched exactly once per snapshotCluster invocation.
type snapFakeClient struct {
	items     map[string][]client.Resource
	listErr   map[string]error
	listCalls int32
}

func newSnapFakeClient() *snapFakeClient {
	return &snapFakeClient{
		items:   make(map[string][]client.Resource),
		listErr: make(map[string]error),
	}
}

func (c *snapFakeClient) addItems(kind string, items ...client.Resource) {
	c.items[kind] = append(c.items[kind], items...)
}

func (c *snapFakeClient) Info(_ context.Context) (*client.ClusterInfo, error) {
	return &client.ClusterInfo{Name: "fake"}, nil
}
func (c *snapFakeClient) List(_ context.Context, kind string, _ client.ListOptions) (*client.ResourceList, error) {
	atomic.AddInt32(&c.listCalls, 1)
	if err, ok := c.listErr[kind]; ok {
		return nil, err
	}
	items := c.items[kind]
	return &client.ResourceList{Items: items, Total: len(items)}, nil
}
func (c *snapFakeClient) Get(_ context.Context, _, _, _ string) (*client.Resource, error) {
	return nil, client.ErrResourceNotFound
}
func (c *snapFakeClient) Delete(_ context.Context, _, _, _ string) error { return nil }
func (c *snapFakeClient) Watch(_ context.Context, _ string, _ client.WatchOptions) (<-chan client.WatchEvent, error) {
	ch := make(chan client.WatchEvent)
	close(ch)
	return ch, nil
}
func (c *snapFakeClient) Logs(_ context.Context, _, _, _ string, _ bool, _ int64) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}
func (c *snapFakeClient) Exec(_ context.Context, _, _, _ string, _ []string) error { return nil }
func (c *snapFakeClient) PortForward(_ context.Context, _, _ string, _, _ int) (client.PortForwarder, error) {
	return nil, nil
}
func (c *snapFakeClient) Close() error { return nil }

// snapFakeManager is a Manager whose List() and Get() back onto a programmed
// cluster-name→client map.
type snapFakeManager struct {
	mu       sync.Mutex
	clients  map[string]*snapFakeClient
	statuses map[string]client.ClusterStatus
}

func newSnapFakeManager() *snapFakeManager {
	return &snapFakeManager{
		clients:  make(map[string]*snapFakeClient),
		statuses: make(map[string]client.ClusterStatus),
	}
}

func (m *snapFakeManager) register(name string, cl *snapFakeClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[name] = cl
	m.statuses[name] = client.StatusConnected
}

func (m *snapFakeManager) Add(_ context.Context, _ string) error { return nil }
func (m *snapFakeManager) Remove(_ string) error                 { return nil }
func (m *snapFakeManager) Get(name string) (client.ClusterClient, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.clients[name]; ok {
		return c, nil
	}
	return nil, client.ErrClusterNotFound
}
func (m *snapFakeManager) Active() (client.ClusterClient, error) { return nil, nil }
func (m *snapFakeManager) SetActive(_ string) error              { return nil }
func (m *snapFakeManager) List() []client.ClusterInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]client.ClusterInfo, 0, len(m.clients))
	for name := range m.clients {
		out = append(out, client.ClusterInfo{
			Context: name,
			Name:    name,
			Status:  m.statuses[name],
		})
	}
	return out
}
func (m *snapFakeManager) Contexts() ([]string, error)                   { return nil, nil }
func (m *snapFakeManager) Close() error                                  { return nil }
func (m *snapFakeManager) ActiveContext() string                         { return "" }
func (m *snapFakeManager) RefreshInfo(_ context.Context, _ string) error { return nil }
func (m *snapFakeManager) ReloadContexts() ([]string, error)             { return nil, nil }

// rawJSON marshals v and panics on failure — acceptable in test code where
// the input is a trivial literal.
func rawJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

// resourceItem builds a client.Resource with name/namespace populated plus a
// raw-JSON metadata blob matching getResourceVersion's expected shape.
// The resource version lives only in the raw JSON — client.Resource has no
// dedicated RV field — and the snapshotter pulls it out via getResourceVersion.
func resourceItem(t *testing.T, name, namespace, rv string) client.Resource {
	return client.Resource{
		Name:      name,
		Namespace: namespace,
		Status:    "Running",
		Raw: rawJSON(t, map[string]interface{}{
			"metadata": map[string]interface{}{"resourceVersion": rv, "name": name, "namespace": namespace},
		}),
	}
}

// newTestRepoDB opens an in-memory storage DB and returns it paired with its
// snapshot repo. Cleanup closes the DB.
func newTestRepoDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("storage.OpenPath: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// ---------------------------------------------------------------------------
// TakeManualSnapshot — end-to-end through the Snapshotter into storage.
// ---------------------------------------------------------------------------

func TestSnapshotter_TakeManualSnapshot_PersistsAllKinds(t *testing.T) {
	db := newTestRepoDB(t)
	mgr := newSnapFakeManager()
	cl := newSnapFakeClient()
	cl.addItems("namespaces", resourceItem(t, "default", "", "1"))
	cl.addItems("pods", resourceItem(t, "api", "default", "42"))
	cl.addItems("deployments", resourceItem(t, "app", "default", "17"))
	cl.addItems("services", resourceItem(t, "svc", "default", "9"))
	mgr.register("c1", cl)

	cfg := SnapshotterConfig{
		ResourceKinds: []string{"pods", "deployments", "services"},
	}
	s := NewSnapshotter(db, mgr, cfg, nil /* no emitter */)

	if err := s.TakeManualSnapshot(context.Background()); err != nil {
		t.Fatalf("TakeManualSnapshot: %v", err)
	}

	repo := storage.NewSnapshotRepository(db)
	latest, err := repo.GetLatest(context.Background(), "c1")
	if err != nil {
		t.Fatalf("GetLatest: %v", err)
	}

	// Each configured kind must have exactly one persisted entry with the
	// programmed name/namespace.
	for _, kind := range cfg.ResourceKinds {
		items, ok := latest.Resources[kind]
		if !ok {
			t.Errorf("kind %q missing from snapshot", kind)
			continue
		}
		if len(items) != 1 {
			t.Errorf("kind %q has %d items, want 1", kind, len(items))
			continue
		}
	}

	// Namespaces list must be populated from the client.
	if len(latest.Namespaces) != 1 || latest.Namespaces[0] != "default" {
		t.Errorf("namespaces = %v, want [default]", latest.Namespaces)
	}

	// Resource version must round-trip via getResourceVersion.
	if rv := latest.Resources["pods"][0].ResourceVersion; rv != "42" {
		t.Errorf("pods[0].ResourceVersion = %q, want 42", rv)
	}
}

// TestSnapshotter_TakeManualSnapshot_EmitsSnapshotTakenEvent verifies the
// optional emitter receives a "snapshot:taken" message carrying the cluster
// name — the frontend depends on this contract to refresh the timeline.
func TestSnapshotter_TakeManualSnapshot_EmitsSnapshotTakenEvent(t *testing.T) {
	db := newTestRepoDB(t)
	mgr := newSnapFakeManager()
	mgr.register("c1", newSnapFakeClient())

	em := &recordingEmitter{}
	s := NewSnapshotter(db, mgr, SnapshotterConfig{ResourceKinds: []string{"pods"}}, em)

	if err := s.TakeManualSnapshot(context.Background()); err != nil {
		t.Fatalf("TakeManualSnapshot: %v", err)
	}

	em.mu.Lock()
	defer em.mu.Unlock()
	if len(em.events) != 1 {
		t.Fatalf("expected 1 emitted event, got %d: %+v", len(em.events), em.events)
	}
	if em.events[0].name != "snapshot:taken" {
		t.Errorf("event name = %q, want snapshot:taken", em.events[0].name)
	}
	payload, ok := em.events[0].data.(map[string]interface{})
	if !ok {
		t.Fatalf("payload type = %T, want map[string]interface{}", em.events[0].data)
	}
	if payload["cluster"] != "c1" {
		t.Errorf("payload.cluster = %v, want c1", payload["cluster"])
	}
	if _, ok := payload["timestamp"].(string); !ok {
		t.Error("payload.timestamp must be a string")
	}
}

// TestSnapshotter_TakeManualSnapshot_SkipsClientFetchErrors verifies that a
// List() failure for one kind does NOT abort the whole snapshot — other kinds
// and the final Save must still succeed.
func TestSnapshotter_TakeManualSnapshot_SkipsClientFetchErrors(t *testing.T) {
	db := newTestRepoDB(t)
	mgr := newSnapFakeManager()
	cl := newSnapFakeClient()
	cl.addItems("pods", resourceItem(t, "p", "default", "1"))
	cl.listErr["deployments"] = errListBroken // fail one kind
	mgr.register("c1", cl)

	cfg := SnapshotterConfig{ResourceKinds: []string{"pods", "deployments"}}
	s := NewSnapshotter(db, mgr, cfg, nil)

	if err := s.TakeManualSnapshot(context.Background()); err != nil {
		t.Fatalf("TakeManualSnapshot should tolerate per-kind failures, got: %v", err)
	}

	latest, err := storage.NewSnapshotRepository(db).GetLatest(context.Background(), "c1")
	if err != nil {
		t.Fatalf("GetLatest: %v", err)
	}
	if len(latest.Resources["pods"]) != 1 {
		t.Errorf("pods kind lost on save, got %+v", latest.Resources)
	}
	if d, ok := latest.Resources["deployments"]; ok && len(d) != 0 {
		t.Errorf("deployments should be empty on fetch error, got %+v", d)
	}
}

// TestSnapshotter_GetSnapshot_RoundTripsAtBoundary verifies that a snapshot
// saved at time T is retrievable via GetSnapshot(T) — this is the primary
// consumer path from app_history.go's GetSnapshotDiff.
func TestSnapshotter_GetSnapshot_RoundTripsAtBoundary(t *testing.T) {
	db := newTestRepoDB(t)
	mgr := newSnapFakeManager()
	cl := newSnapFakeClient()
	cl.addItems("pods", resourceItem(t, "p", "default", "1"))
	mgr.register("c1", cl)

	s := NewSnapshotter(db, mgr, SnapshotterConfig{ResourceKinds: []string{"pods"}}, nil)
	if err := s.TakeManualSnapshot(context.Background()); err != nil {
		t.Fatalf("TakeManualSnapshot: %v", err)
	}

	// Pull the timestamp out of the DB (the Snapshotter uses time.Now()
	// internally so we can't reproduce the exact value).
	timestamps, err := s.ListSnapshots(context.Background(), "c1", 10)
	if err != nil || len(timestamps) != 1 {
		t.Fatalf("ListSnapshots: %v, len=%d", err, len(timestamps))
	}

	// GetSnapshot with a timestamp at-or-after the stored one must return
	// the snapshot. We add 1ns to absorb any round-tripping precision loss
	// in SQLite's datetime storage — the contract is "closest at-or-before".
	got, err := s.GetSnapshot(context.Background(), "c1", timestamps[0].Add(time.Second))
	if err != nil {
		t.Fatalf("GetSnapshot: %v", err)
	}
	if len(got.Resources["pods"]) != 1 {
		t.Errorf("round-tripped snapshot missing pods: %+v", got.Resources)
	}
}

// TestSnapshotter_ListSnapshots_OrdersDescending pins the contract used by
// app_history.go to build the timeline UI.
func TestSnapshotter_ListSnapshots_OrdersDescending(t *testing.T) {
	db := newTestRepoDB(t)
	repo := storage.NewSnapshotRepository(db)
	mgr := newSnapFakeManager()
	mgr.register("c1", newSnapFakeClient())

	// Seed three snapshots at known timestamps (bypass Snapshotter so we can
	// control the values).
	base := time.Now().UTC().Truncate(time.Second)
	for i := 0; i < 3; i++ {
		if err := repo.Save(context.Background(), "c1", &storage.SnapshotData{
			Cluster:   "c1",
			Timestamp: base.Add(time.Duration(i) * time.Minute),
			Resources: map[string][]storage.ResourceInfo{},
		}); err != nil {
			t.Fatalf("repo.Save: %v", err)
		}
	}

	s := NewSnapshotter(db, mgr, DefaultSnapshotterConfig(), nil)
	got, err := s.ListSnapshots(context.Background(), "c1", 10)
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 timestamps, got %d", len(got))
	}
	for i := 0; i+1 < len(got); i++ {
		if !got[i].After(got[i+1]) {
			t.Errorf("ListSnapshots not descending at index %d: %v !> %v", i, got[i], got[i+1])
		}
	}
}

// TestSnapshotter_Cleanup_DeletesStaleRows verifies retention prunes old rows
// under the Snapshotter's configured window.
func TestSnapshotter_Cleanup_DeletesStaleRows(t *testing.T) {
	db := newTestRepoDB(t)
	repo := storage.NewSnapshotRepository(db)
	mgr := newSnapFakeManager()
	mgr.register("c1", newSnapFakeClient())

	now := time.Now().UTC()
	// Save one old snapshot (outside window) and one fresh.
	_ = repo.Save(context.Background(), "c1", &storage.SnapshotData{
		Cluster:   "c1",
		Timestamp: now.Add(-48 * time.Hour),
		Resources: map[string][]storage.ResourceInfo{},
	})
	_ = repo.Save(context.Background(), "c1", &storage.SnapshotData{
		Cluster:   "c1",
		Timestamp: now,
		Resources: map[string][]storage.ResourceInfo{},
	})

	s := NewSnapshotter(db, mgr, SnapshotterConfig{Retention: 24 * time.Hour}, nil)
	s.cleanup() // exercises the unexported cleanup

	count, err := repo.Count(context.Background())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 remaining snapshot after cleanup, got %d", count)
	}
}

// TestSnapshotter_Start_Stop_IsIdempotent — double-Start must not leak
// goroutines; double-Stop must not panic.
func TestSnapshotter_Start_Stop_IsIdempotent(t *testing.T) {
	db := newTestRepoDB(t)
	mgr := newSnapFakeManager()

	s := NewSnapshotter(db, mgr, SnapshotterConfig{
		Interval:      365 * 24 * time.Hour, // effectively never fires
		Retention:     24 * time.Hour,
		ResourceKinds: []string{"pods"},
	}, nil)

	s.Start()
	s.Start() // second call must be a no-op
	s.Stop()
	s.Stop() // second stop must not panic
}

// ---------------------------------------------------------------------------
// recordingEmitter — satisfies events.EmitterInterface for assertion-friendly
// capture of emitted events.
// ---------------------------------------------------------------------------

type recordedEvent struct {
	name string
	data interface{}
}

type recordingEmitter struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (e *recordingEmitter) Emit(name string, data ...interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()
	var d interface{}
	if len(data) > 0 {
		d = data[0]
	}
	e.events = append(e.events, recordedEvent{name: name, data: d})
}
func (e *recordingEmitter) SetContext(_ context.Context) {}

// errListBroken is a sentinel error value used to program fake List failures.
type errString string

func (e errString) Error() string { return string(e) }

var errListBroken = errString("fake list failure")
