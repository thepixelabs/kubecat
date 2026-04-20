// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Test doubles
//
// These test-only fakes live alongside the manager in-package so they can
// be injected directly into the unexported `manager` struct's `clients`
// map. This lets us exercise the full lifecycle (Remove / Get / Active /
// SetActive / Close / List / RefreshInfo) without needing a real apiserver
// or httptest transport — the code under test is state management, not
// the underlying kube-client implementation.
// ---------------------------------------------------------------------------

// stubClusterClient is a minimal ClusterClient used to exercise manager
// state transitions. Only Info() and Close() are meaningful; the rest
// return canned zero values.
type stubClusterClient struct {
	name      string
	info      *ClusterInfo
	infoErr   error
	infoCalls int
	mu        sync.Mutex

	closed     bool
	closeErr   error
	closeCalls int
}

func (c *stubClusterClient) Info(_ context.Context) (*ClusterInfo, error) {
	c.mu.Lock()
	c.infoCalls++
	c.mu.Unlock()
	if c.infoErr != nil {
		return nil, c.infoErr
	}
	if c.info != nil {
		return c.info, nil
	}
	return &ClusterInfo{Name: c.name, Context: c.name, Status: StatusConnected}, nil
}

func (c *stubClusterClient) List(_ context.Context, _ string, _ ListOptions) (*ResourceList, error) {
	return &ResourceList{}, nil
}
func (c *stubClusterClient) Get(_ context.Context, _, _, _ string) (*Resource, error) {
	return nil, ErrResourceNotFound
}
func (c *stubClusterClient) Delete(_ context.Context, _, _, _ string) error { return nil }
func (c *stubClusterClient) Watch(_ context.Context, _ string, _ WatchOptions) (<-chan WatchEvent, error) {
	ch := make(chan WatchEvent)
	close(ch)
	return ch, nil
}
func (c *stubClusterClient) Logs(_ context.Context, _, _, _ string, _ bool, _ int64) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}
func (c *stubClusterClient) Exec(_ context.Context, _, _, _ string, _ []string) error { return nil }
func (c *stubClusterClient) PortForward(_ context.Context, _, _ string, _, _ int) (PortForwarder, error) {
	return nil, nil
}
func (c *stubClusterClient) ExecInteractive(_ context.Context, _, _, _ string, _ []string, _ io.Reader, _, _ io.Writer, _ bool) error {
	return nil
}
func (c *stubClusterClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeCalls++
	c.closed = true
	return c.closeErr
}

// newTestManager builds a manager value we can mutate directly. It does NOT
// invoke NewManager (which would require a kubeconfig). For tests that need
// a real loader, use newTestManagerWithLoader.
func newTestManager() *manager {
	return &manager{
		clients: make(map[string]ClusterClient),
		infos:   make(map[string]*ClusterInfo),
	}
}

// seedClient injects a stub client under the given context name and
// optionally primes its cached info.
func seedClient(m *manager, contextName string, info *ClusterInfo) *stubClusterClient {
	c := &stubClusterClient{name: contextName, info: info}
	m.clients[contextName] = c
	if info != nil {
		m.infos[contextName] = info
	}
	return c
}

// ---------------------------------------------------------------------------
// Get / Active / ActiveContext / SetActive
// ---------------------------------------------------------------------------

// TestManager_Get_UnknownContext_ReturnsErrContextNotFound pins the contract:
// Get on a never-added context surfaces the sentinel error.
func TestManager_Get_UnknownContext_ReturnsErrContextNotFound(t *testing.T) {
	m := newTestManager()

	_, err := m.Get("ghost")
	if !errors.Is(err, ErrContextNotFound) {
		t.Errorf("expected ErrContextNotFound, got %v", err)
	}
}

// TestManager_Get_ExistingContext_ReturnsClient confirms a seeded client
// round-trips.
func TestManager_Get_ExistingContext_ReturnsClient(t *testing.T) {
	m := newTestManager()
	stub := seedClient(m, "ctx-a", nil)

	got, err := m.Get("ctx-a")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != stub {
		t.Errorf("Get returned wrong client: %v, want %v", got, stub)
	}
}

// TestManager_Active_NoActive_ReturnsErrNoActiveCluster pins the contract when
// no context has been promoted.
func TestManager_Active_NoActive_ReturnsErrNoActiveCluster(t *testing.T) {
	m := newTestManager()

	_, err := m.Active()
	if !errors.Is(err, ErrNoActiveCluster) {
		t.Errorf("expected ErrNoActiveCluster, got %v", err)
	}
}

// TestManager_Active_DanglingActiveNotInClients_ReturnsErrNoActiveCluster
// pins the defensive check: if m.active names a context that is no longer
// in m.clients, Active() must not return nil,nil.
func TestManager_Active_DanglingActiveNotInClients_ReturnsErrNoActiveCluster(t *testing.T) {
	m := newTestManager()
	m.active = "gone" // set but never added

	_, err := m.Active()
	if !errors.Is(err, ErrNoActiveCluster) {
		t.Errorf("expected ErrNoActiveCluster for dangling active, got %v", err)
	}
}

// TestManager_SetActive_MissingContext_Errors verifies SetActive refuses
// to promote a context that has never been added.
func TestManager_SetActive_MissingContext_Errors(t *testing.T) {
	m := newTestManager()

	err := m.SetActive("ghost")
	if !errors.Is(err, ErrContextNotFound) {
		t.Errorf("expected ErrContextNotFound, got %v", err)
	}
	if m.active != "" {
		t.Errorf("m.active mutated on failed SetActive: %q", m.active)
	}
}

// TestManager_SetActive_PromotesKnownContext validates the happy path.
func TestManager_SetActive_PromotesKnownContext(t *testing.T) {
	m := newTestManager()
	stub := seedClient(m, "ctx-a", nil)

	if err := m.SetActive("ctx-a"); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if m.ActiveContext() != "ctx-a" {
		t.Errorf("ActiveContext = %q, want ctx-a", m.ActiveContext())
	}

	active, err := m.Active()
	if err != nil {
		t.Fatalf("Active after SetActive: %v", err)
	}
	if active != stub {
		t.Errorf("Active() returned wrong client")
	}
}

// ---------------------------------------------------------------------------
// Remove
// ---------------------------------------------------------------------------

// TestManager_Remove_UnknownContext_Errors pins the ErrContextNotFound
// contract for Remove.
func TestManager_Remove_UnknownContext_Errors(t *testing.T) {
	m := newTestManager()

	err := m.Remove("ghost")
	if !errors.Is(err, ErrContextNotFound) {
		t.Errorf("expected ErrContextNotFound, got %v", err)
	}
}

// TestManager_Remove_ClosesClient_DeletesEntry_ClearsActive exercises the
// full removal sequence when removing the currently active context.
func TestManager_Remove_ClosesClient_DeletesEntry_ClearsActive(t *testing.T) {
	m := newTestManager()
	stub := seedClient(m, "ctx-a", &ClusterInfo{Name: "ctx-a"})
	m.active = "ctx-a"

	if err := m.Remove("ctx-a"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if stub.closeCalls != 1 {
		t.Errorf("Close called %d times, want 1", stub.closeCalls)
	}
	if _, ok := m.clients["ctx-a"]; ok {
		t.Error("client entry was not deleted")
	}
	if _, ok := m.infos["ctx-a"]; ok {
		t.Error("info entry was not deleted")
	}
	if m.active != "" {
		t.Errorf("m.active = %q, want cleared", m.active)
	}
}

// TestManager_Remove_NonActive_KeepsOtherActive confirms that removing a
// non-active context does NOT clobber the m.active field.
func TestManager_Remove_NonActive_KeepsOtherActive(t *testing.T) {
	m := newTestManager()
	seedClient(m, "ctx-a", nil)
	seedClient(m, "ctx-b", nil)
	m.active = "ctx-a"

	if err := m.Remove("ctx-b"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if m.active != "ctx-a" {
		t.Errorf("m.active = %q, want ctx-a", m.active)
	}
}

// TestManager_Remove_CloseError_Propagates pins that a failure during the
// client's Close() returns the error without partially cleaning up — the
// entry must stay to allow retry or forensics.
func TestManager_Remove_CloseError_Propagates(t *testing.T) {
	m := newTestManager()
	stub := seedClient(m, "ctx-a", nil)
	stub.closeErr = errors.New("close boom")

	err := m.Remove("ctx-a")
	if err == nil || err.Error() != "close boom" {
		t.Errorf("Remove should propagate Close error, got %v", err)
	}
	// Per the current implementation, the entry is NOT deleted when Close
	// returns an error (return happens before delete()).
	if _, ok := m.clients["ctx-a"]; !ok {
		t.Error("failed Close must not delete entry (would hide the problem)")
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

// TestManager_List_EmptyWhenNoClients confirms zero-state.
func TestManager_List_EmptyWhenNoClients(t *testing.T) {
	m := newTestManager()
	if got := m.List(); len(got) != 0 {
		t.Errorf("List = %v, want empty", got)
	}
}

// TestManager_List_UsesCachedInfoWhenAvailable pins that when the manager has
// a cached ClusterInfo for a context, List returns it verbatim.
func TestManager_List_UsesCachedInfoWhenAvailable(t *testing.T) {
	m := newTestManager()
	seedClient(m, "ctx-a", &ClusterInfo{
		Name: "ctx-a", Context: "ctx-a", Version: "v1.28.0", Status: StatusConnected, NodeCount: 5,
	})

	infos := m.List()
	if len(infos) != 1 {
		t.Fatalf("List len = %d, want 1", len(infos))
	}
	if infos[0].Version != "v1.28.0" || infos[0].NodeCount != 5 {
		t.Errorf("List returned wrong info: %+v", infos[0])
	}
	if infos[0].Status != StatusConnected {
		t.Errorf("Status = %v, want Connected", infos[0].Status)
	}
}

// TestManager_List_FallsBackToUnknownWhenNoCachedInfo verifies the defensive
// path where a client is registered but Info() has not yet populated the
// cache — we still want the context visible in the UI, flagged as Unknown.
func TestManager_List_FallsBackToUnknownWhenNoCachedInfo(t *testing.T) {
	m := newTestManager()
	seedClient(m, "ctx-a", nil) // no cached info

	infos := m.List()
	if len(infos) != 1 {
		t.Fatalf("List len = %d, want 1", len(infos))
	}
	if infos[0].Name != "ctx-a" || infos[0].Context != "ctx-a" {
		t.Errorf("List[0] = %+v", infos[0])
	}
	if infos[0].Status != StatusUnknown {
		t.Errorf("Status = %v, want Unknown", infos[0].Status)
	}
}

// TestManager_List_AllClients confirms every seeded client appears.
func TestManager_List_AllClients(t *testing.T) {
	m := newTestManager()
	for _, name := range []string{"a", "b", "c"} {
		seedClient(m, name, &ClusterInfo{Name: name, Context: name})
	}

	infos := m.List()
	names := make([]string, 0, len(infos))
	for _, i := range infos {
		names = append(names, i.Name)
	}
	sort.Strings(names)
	if fmt.Sprintf("%v", names) != "[a b c]" {
		t.Errorf("List names = %v, want [a b c]", names)
	}
}

// ---------------------------------------------------------------------------
// RefreshInfo
// ---------------------------------------------------------------------------

// TestManager_RefreshInfo_UnknownContext_Errors pins the missing-context
// contract.
func TestManager_RefreshInfo_UnknownContext_Errors(t *testing.T) {
	m := newTestManager()
	err := m.RefreshInfo(context.Background(), "ghost")
	if !errors.Is(err, ErrContextNotFound) {
		t.Errorf("expected ErrContextNotFound, got %v", err)
	}
}

// TestManager_RefreshInfo_UpdatesCache verifies the happy path: Info() is
// called and the returned value replaces any stale cache entry.
func TestManager_RefreshInfo_UpdatesCache(t *testing.T) {
	m := newTestManager()
	stub := seedClient(m, "ctx-a", &ClusterInfo{Name: "stale", Version: "old"})
	stub.info = &ClusterInfo{Name: "ctx-a", Version: "v2.0"}

	if err := m.RefreshInfo(context.Background(), "ctx-a"); err != nil {
		t.Fatalf("RefreshInfo: %v", err)
	}
	if stub.infoCalls != 1 {
		t.Errorf("Info calls = %d, want 1", stub.infoCalls)
	}
	if got := m.infos["ctx-a"]; got == nil || got.Version != "v2.0" {
		t.Errorf("cached info not updated: %+v", got)
	}
}

// TestManager_RefreshInfo_PropagatesClientError ensures a failing Info()
// leaves the cache untouched and surfaces the error.
func TestManager_RefreshInfo_PropagatesClientError(t *testing.T) {
	m := newTestManager()
	stub := seedClient(m, "ctx-a", &ClusterInfo{Name: "ctx-a", Version: "v1"})
	stub.infoErr = errors.New("apiserver down")

	err := m.RefreshInfo(context.Background(), "ctx-a")
	if err == nil || err.Error() != "apiserver down" {
		t.Errorf("expected 'apiserver down', got %v", err)
	}
	// Previously cached info must still be there.
	if got := m.infos["ctx-a"]; got == nil || got.Version != "v1" {
		t.Errorf("cached info was clobbered on error: %+v", got)
	}
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

// TestManager_Close_ClosesEveryClient_ClearsState verifies that Close closes
// every client, clears the maps, and resets active.
func TestManager_Close_ClosesEveryClient_ClearsState(t *testing.T) {
	m := newTestManager()
	stubs := []*stubClusterClient{
		seedClient(m, "a", &ClusterInfo{Name: "a"}),
		seedClient(m, "b", &ClusterInfo{Name: "b"}),
	}
	m.active = "a"

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	for _, s := range stubs {
		if s.closeCalls != 1 {
			t.Errorf("stub %q closeCalls = %d, want 1", s.name, s.closeCalls)
		}
	}
	if len(m.clients) != 0 {
		t.Errorf("clients map len = %d, want 0 after Close", len(m.clients))
	}
	if len(m.infos) != 0 {
		t.Errorf("infos map len = %d, want 0 after Close", len(m.infos))
	}
	if m.active != "" {
		t.Errorf("active = %q, want cleared after Close", m.active)
	}
}

// TestManager_Close_ReturnsLastCloseError pins that Close keeps closing every
// client even if one fails, then returns the last non-nil error.
func TestManager_Close_ReturnsLastCloseError(t *testing.T) {
	m := newTestManager()
	seedClient(m, "ok", nil)
	bad := seedClient(m, "bad", nil)
	bad.closeErr = errors.New("bad close")

	err := m.Close()
	if err == nil {
		t.Fatal("expected Close to return an error, got nil")
	}
	// Both stubs should have been asked to close.
	if len(m.clients) != 0 {
		t.Errorf("clients not cleared after Close: %v", m.clients)
	}
}

// ---------------------------------------------------------------------------
// Add (error-path only — Add's happy path requires a reachable apiserver and
// is covered end-to-end by NewManager + a real kubeconfig in integration
// tests; we pin the state-invariant error paths here).
// ---------------------------------------------------------------------------

// TestManager_Add_DuplicateContext_ReturnsErrClusterAlreadyExists pins that
// Add on an already-registered context short-circuits before any kube-client
// work.
func TestManager_Add_DuplicateContext_ReturnsErrClusterAlreadyExists(t *testing.T) {
	m := newTestManager()
	seedClient(m, "ctx-a", nil)

	err := m.Add(context.Background(), "ctx-a")
	if !errors.Is(err, ErrClusterAlreadyExists) {
		t.Errorf("expected ErrClusterAlreadyExists, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Concurrency — exercise the RWMutex on the hot read path
// ---------------------------------------------------------------------------

// TestManager_ConcurrentReads verifies that Get/Active/ActiveContext/List do
// not serialize and do not race under concurrent access. This is a race-mode
// guard, not a performance assertion.
func TestManager_ConcurrentReads(t *testing.T) {
	m := newTestManager()
	for i := 0; i < 4; i++ {
		seedClient(m, fmt.Sprintf("ctx-%d", i), &ClusterInfo{Name: fmt.Sprintf("ctx-%d", i)})
	}
	m.active = "ctx-0"

	var wg sync.WaitGroup
	const workers = 8
	const iters = 200
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				_, _ = m.Get("ctx-1")
				_, _ = m.Active()
				_ = m.ActiveContext()
				_ = m.List()
			}
		}()
	}
	wg.Wait()
}

// TestManager_ConcurrentWritesDoNotRace mixes SetActive / Remove / Add-duplicate
// across goroutines. We seed a couple of known contexts and rotate the active
// pointer. The assertion is implicit: -race must not report anything.
func TestManager_ConcurrentWritesDoNotRace(t *testing.T) {
	m := newTestManager()
	for _, n := range []string{"a", "b"} {
		seedClient(m, n, nil)
	}

	var wg sync.WaitGroup
	const workers = 4
	const iters = 100
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(idx int) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				target := "a"
				if (i+idx)%2 == 0 {
					target = "b"
				}
				_ = m.SetActive(target)
				_, _ = m.Get(target)
				_ = m.Add(context.Background(), target) // duplicate -> err, no mutation
			}
		}(w)
	}
	wg.Wait()

	// At least one of {a,b} must still be the active context.
	if m.ActiveContext() != "a" && m.ActiveContext() != "b" {
		t.Errorf("unexpected final active %q", m.ActiveContext())
	}
}

// ---------------------------------------------------------------------------
// NewManager — integrate with a real kubeconfig loader (no apiserver)
// ---------------------------------------------------------------------------

// TestNewManager_WithMultiContextKubeconfig verifies NewManager surfaces every
// context from the kubeconfig and sets the current-context as active-by-name.
// No Add() is invoked, so no apiserver is needed.
func TestNewManager_WithMultiContextKubeconfig(t *testing.T) {
	writeKubeconfig(t, multiContextKubeconfig)

	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })

	ctxs, err := mgr.Contexts()
	if err != nil {
		t.Fatalf("Contexts: %v", err)
	}
	sort.Strings(ctxs)
	if len(ctxs) != 2 || ctxs[0] != "ctx-a" || ctxs[1] != "ctx-b" {
		t.Errorf("Contexts = %v, want [ctx-a ctx-b]", ctxs)
	}

	// current-context is ctx-a but no Add() has been called yet; Active()
	// should still return ErrNoActiveCluster because clients[] is empty.
	if _, err := mgr.Active(); !errors.Is(err, ErrNoActiveCluster) {
		t.Errorf("Active before Add should be ErrNoActiveCluster, got %v", err)
	}
	if ac := mgr.ActiveContext(); ac != "ctx-a" {
		t.Errorf("ActiveContext = %q, want ctx-a (from current-context)", ac)
	}
}

// TestNewManager_BrokenKubeconfig_ReturnsCleanError confirms a malformed
// kubeconfig does not panic NewManager.
func TestNewManager_BrokenKubeconfig_ReturnsCleanError(t *testing.T) {
	writeKubeconfig(t, "not: valid: yaml: [[[[")

	if _, err := NewManager(); err == nil {
		t.Fatal("expected NewManager to error on bad kubeconfig, got nil")
	}
}

// TestManager_ReloadContexts_SurfacesNewKubeconfigContexts end-to-ends the
// kubeconfig-reload path through the Manager interface.
func TestManager_ReloadContexts_SurfacesNewKubeconfigContexts(t *testing.T) {
	path := writeKubeconfig(t, multiContextKubeconfig)
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })

	// Rewriting the kubeconfig requires replacing the whole file: the
	// 'contexts:' list is an inline block, so appending at EOF would not
	// extend it.
	const threeContext = `
apiVersion: v1
kind: Config
clusters:
- name: cluster-a
  cluster:
    server: https://api.a.example.com:6443
- name: cluster-b
  cluster:
    server: https://api.b.example.com:6443
contexts:
- name: ctx-a
  context:
    cluster: cluster-a
    user: user-a
    namespace: team-a
- name: ctx-b
  context:
    cluster: cluster-b
    user: user-b
- name: ctx-c
  context:
    cluster: cluster-a
    user: user-a
users:
- name: user-a
  user:
    token: abc
- name: user-b
  user:
    token: def
current-context: ctx-a
`
	if err := os.WriteFile(path, []byte(threeContext), 0600); err != nil {
		t.Fatalf("rewrite kubeconfig: %v", err)
	}

	ctxs, err := mgr.ReloadContexts()
	if err != nil {
		t.Fatalf("ReloadContexts: %v", err)
	}
	sort.Strings(ctxs)
	if len(ctxs) != 3 {
		t.Errorf("ReloadContexts = %v, want 3 contexts", ctxs)
	}
}
