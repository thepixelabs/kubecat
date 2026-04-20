// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/thepixelabs/kubecat/internal/client"
)

// ---------------------------------------------------------------------------
// Fake PortForwarder + ClusterClient extensions
// ---------------------------------------------------------------------------

// fakePortForwarder implements client.PortForwarder for tests.
type fakePortForwarder struct {
	localPort int
	stopCh    chan struct{}
	doneCh    chan struct{}
	err       error

	mu        sync.Mutex
	stopCalls int
}

func newFakePortForwarder(local int) *fakePortForwarder {
	return &fakePortForwarder{
		localPort: local,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

func (f *fakePortForwarder) LocalPort() int        { return f.localPort }
func (f *fakePortForwarder) Done() <-chan struct{} { return f.doneCh }
func (f *fakePortForwarder) Error() error          { return f.err }

func (f *fakePortForwarder) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopCalls++
	select {
	case <-f.doneCh:
	default:
		close(f.doneCh)
	}
}

// pfClusterClient wraps fakeClusterClient to intercept PortForward calls.
type pfClusterClient struct {
	*fakeClusterClient
	pfFn func(ctx context.Context, namespace, pod string, localPort, remotePort int) (client.PortForwarder, error)
}

func (c *pfClusterClient) PortForward(ctx context.Context, namespace, pod string, localPort, remotePort int) (client.PortForwarder, error) {
	if c.pfFn != nil {
		return c.pfFn(ctx, namespace, pod, localPort, remotePort)
	}
	return nil, fmt.Errorf("pfFn not configured")
}

// newAppWithPortForwardClient wires an App through the core services where
// the underlying cluster client is a pfClusterClient.
func newAppWithPortForwardClient(t *testing.T, pfFn func(context.Context, string, string, int, int) (client.PortForwarder, error)) *App {
	t.Helper()
	base := newFakeClusterClient()
	wrapped := &pfClusterClient{fakeClusterClient: base, pfFn: pfFn}
	// Our fake manager expects *fakeClusterClient; we replace the stored
	// client via a custom manager that returns the wrapper.
	mgr := &pfManager{cl: wrapped}
	a := newAppWithManagerForPF(mgr)
	return a
}

// pfManager is a lightweight client.Manager that returns the pfClusterClient.
type pfManager struct{ cl client.ClusterClient }

func (m *pfManager) Add(_ context.Context, _ string) error         { return nil }
func (m *pfManager) Remove(_ string) error                         { return nil }
func (m *pfManager) Get(_ string) (client.ClusterClient, error)    { return m.cl, nil }
func (m *pfManager) Active() (client.ClusterClient, error)         { return m.cl, nil }
func (m *pfManager) SetActive(_ string) error                      { return nil }
func (m *pfManager) List() []client.ClusterInfo                    { return nil }
func (m *pfManager) Contexts() ([]string, error)                   { return []string{"test"}, nil }
func (m *pfManager) Close() error                                  { return nil }
func (m *pfManager) ActiveContext() string                         { return "test" }
func (m *pfManager) RefreshInfo(_ context.Context, _ string) error { return nil }
func (m *pfManager) ReloadContexts() ([]string, error)             { return []string{"test"}, nil }

func newAppWithManagerForPF(mgr client.Manager) *App {
	return newAppWithManager(mgr)
}

// findFreeLocalPort picks a free port >= 1024 by asking the OS for an
// ephemeral bind, then immediately releasing it. Returns 0 if no port is
// available.
func findFreeLocalPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	defer ln.Close()
	p := ln.Addr().(*net.TCPAddr).Port
	// The service validates localPort >= 1024; an OS-assigned ephemeral port
	// is always >=32768, so this is always safe.
	return p
}

// ---------------------------------------------------------------------------
// CreatePortForward
// ---------------------------------------------------------------------------

// TestCreatePortForward_HappyPath verifies that a valid request reaches the
// underlying client, the returned ActiveForward mirrors the requested
// ports, and ListPortForwards shows the session.
func TestCreatePortForward_HappyPath(t *testing.T) {
	local := findFreeLocalPort(t)

	pfFn := func(_ context.Context, ns, pod string, lp, rp int) (client.PortForwarder, error) {
		if ns != "default" || pod != "web" || lp != local || rp != 8080 {
			t.Errorf("PortForward called with unexpected args: ns=%s pod=%s lp=%d rp=%d",
				ns, pod, lp, rp)
		}
		return newFakePortForwarder(lp), nil
	}
	a := newAppWithPortForwardClient(t, pfFn)

	info, err := a.CreatePortForward("default", "web", local, 8080)
	if err != nil {
		t.Fatalf("CreatePortForward: %v", err)
	}
	if info.LocalPort != local {
		t.Errorf("LocalPort = %d, want %d", info.LocalPort, local)
	}
	if info.RemotePort != 8080 {
		t.Errorf("RemotePort = %d, want 8080", info.RemotePort)
	}
	if info.Status != "Active" {
		t.Errorf("Status = %q, want Active", info.Status)
	}

	list := a.ListPortForwards()
	if len(list) != 1 {
		t.Errorf("ListPortForwards len = %d, want 1", len(list))
	}
}

// TestCreatePortForward_RejectsPrivilegedLocalPort pins the validation gate:
// ports 1-1023 must be refused before any network call.
func TestCreatePortForward_RejectsPrivilegedLocalPort(t *testing.T) {
	called := false
	pfFn := func(_ context.Context, _, _ string, _, _ int) (client.PortForwarder, error) {
		called = true
		return nil, nil
	}
	a := newAppWithPortForwardClient(t, pfFn)

	_, err := a.CreatePortForward("default", "web", 80, 8080)
	if err == nil {
		t.Fatal("expected error for privileged local port, got nil")
	}
	if !strings.Contains(err.Error(), "1024") {
		t.Errorf("error should reference 1024-65535 range, got: %v", err)
	}
	if called {
		t.Error("backend must not be called when validation fails")
	}
}

// TestCreatePortForward_RejectsReservedLocalPort prevents collisions with
// the app's own Vite / Wails bridge ports.
func TestCreatePortForward_RejectsReservedLocalPort(t *testing.T) {
	a := newAppWithPortForwardClient(t, nil)
	_, err := a.CreatePortForward("default", "web", 5173, 80)
	if err == nil {
		t.Fatal("expected error for Vite-reserved port 5173, got nil")
	}
	if !strings.Contains(err.Error(), "5173") {
		t.Errorf("error should reference the reserved port, got: %v", err)
	}
}

// TestCreatePortForward_RejectsInvalidRemotePort catches the remotePort=0
// corner case that the validator rejects.
func TestCreatePortForward_RejectsInvalidRemotePort(t *testing.T) {
	a := newAppWithPortForwardClient(t, nil)
	local := findFreeLocalPort(t)
	_, err := a.CreatePortForward("default", "web", local, 0)
	if err == nil {
		t.Fatal("expected error for remotePort=0, got nil")
	}
}

// TestCreatePortForward_LocalPortInUse_ReturnsCleanError exercises the
// pre-bind availability check. We hold a listener on a port and verify the
// service refuses to attempt a forward to that port.
func TestCreatePortForward_LocalPortInUse_ReturnsCleanError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	busy := ln.Addr().(*net.TCPAddr).Port

	a := newAppWithPortForwardClient(t, nil)
	_, err = a.CreatePortForward("default", "web", busy, 8080)
	if err == nil {
		t.Fatal("expected error when localPort is already bound, got nil")
	}
	if !strings.Contains(err.Error(), "already in use") {
		t.Errorf("error should mention already-in-use, got: %v", err)
	}
}

// TestCreatePortForward_BackendError_Propagates ensures API-layer failures
// reach the caller untransformed (aside from the service's own wrapping).
func TestCreatePortForward_BackendError_Propagates(t *testing.T) {
	pfFn := func(_ context.Context, _, _ string, _, _ int) (client.PortForwarder, error) {
		return nil, fmt.Errorf("pod not found: web")
	}
	a := newAppWithPortForwardClient(t, pfFn)

	local := findFreeLocalPort(t)
	_, err := a.CreatePortForward("default", "web", local, 8080)
	if err == nil {
		t.Fatal("expected backend error to propagate, got nil")
	}
	if !strings.Contains(err.Error(), "pod not found") {
		t.Errorf("error should mention pod not found, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// StopPortForward
// ---------------------------------------------------------------------------

// TestStopPortForward_StopsActiveSession verifies that Stop reaches the
// underlying forwarder and removes the entry from ListPortForwards.
func TestStopPortForward_StopsActiveSession(t *testing.T) {
	local := findFreeLocalPort(t)

	var forwarder *fakePortForwarder
	pfFn := func(_ context.Context, _, _ string, lp, _ int) (client.PortForwarder, error) {
		forwarder = newFakePortForwarder(lp)
		return forwarder, nil
	}
	a := newAppWithPortForwardClient(t, pfFn)

	info, err := a.CreatePortForward("default", "web", local, 8080)
	if err != nil {
		t.Fatalf("CreatePortForward: %v", err)
	}

	if err := a.StopPortForward(info.ID); err != nil {
		t.Fatalf("StopPortForward: %v", err)
	}
	if forwarder.stopCalls != 1 {
		t.Errorf("forwarder.Stop called %d times, want 1", forwarder.stopCalls)
	}
	if len(a.ListPortForwards()) != 0 {
		t.Errorf("ListPortForwards should be empty after stop, got %d",
			len(a.ListPortForwards()))
	}
}

// TestStopPortForward_UnknownID_ReturnsError pins that stopping a
// never-started forward returns a clear error.
func TestStopPortForward_UnknownID_ReturnsError(t *testing.T) {
	a := newAppWithPortForwardClient(t, nil)
	err := a.StopPortForward("default/web:9999->8080")
	if err == nil {
		t.Fatal("expected error for unknown id, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention not found, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ListPortForwards
// ---------------------------------------------------------------------------

// TestListPortForwards_ReturnsEmptyWhenIdle confirms the zero-state.
func TestListPortForwards_ReturnsEmptyWhenIdle(t *testing.T) {
	a := newAppWithPortForwardClient(t, nil)
	if got := a.ListPortForwards(); len(got) != 0 {
		t.Errorf("idle ListPortForwards = %v, want empty", got)
	}
}
