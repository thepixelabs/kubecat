// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"testing"

	"github.com/thepixelabs/kubecat/internal/client"
	"github.com/thepixelabs/kubecat/internal/core"
)

// ---------------------------------------------------------------------------
// App.GetContexts / GetActiveContext / IsConnected
// ---------------------------------------------------------------------------

// TestGetContexts_DelegatesToClusterService exercises the thin shim by wiring
// the App to a manager that exposes two contexts.
func TestGetContexts_DelegatesToClusterService(t *testing.T) {
	a := newAppWithFakes(newFakeClusterClient())
	ctxs, err := a.GetContexts()
	if err != nil {
		t.Fatalf("GetContexts: %v", err)
	}
	// fakeManager returns []string{"test"}
	if len(ctxs) != 1 || ctxs[0] != "test" {
		t.Errorf("GetContexts = %v, want [test]", ctxs)
	}
}

// TestIsConnected_FalseBeforeConnect pins the initial "not connected" state.
// newAppWithFakes sets activeContext from the manager, so IsConnected will
// read through. We explicitly verify the semantic: no connect call = true only
// if the manager already owns an active context.
func TestIsConnected_ReflectsActiveContext(t *testing.T) {
	// fakeManager starts with active="test", so IsConnected should be true.
	a := newAppWithFakes(newFakeClusterClient())
	if !a.IsConnected() {
		t.Error("IsConnected should be true when manager has active context")
	}
}

// TestGetActiveContext_ReturnsManagerActive confirms the shim returns the
// manager's ActiveContext().
func TestGetActiveContext_ReturnsManagerActive(t *testing.T) {
	a := newAppWithFakes(newFakeClusterClient())
	if got := a.GetActiveContext(); got != "test" {
		t.Errorf("GetActiveContext = %q, want test", got)
	}
}

// ---------------------------------------------------------------------------
// Connect / Disconnect — exercised through ClusterService.Connect
// ---------------------------------------------------------------------------

// fakeManagerWithAddHooks lets us verify Connect's Add+SetActive call
// sequence and error propagation from Add.
type fakeManagerWithAddHooks struct {
	*fakeManager
	addCalled       int
	setActiveCalled int
	removeCalled    int
	addReturnErr    error
	setActiveErr    error
	removeErr       error
	known           map[string]bool
}

func newManagerWithAddHooks(known ...string) *fakeManagerWithAddHooks {
	cl := newFakeClusterClient()
	m := newFakeManager(cl)
	known2 := make(map[string]bool, len(known))
	for _, k := range known {
		known2[k] = true
	}
	return &fakeManagerWithAddHooks{fakeManager: m, known: known2}
}

func (m *fakeManagerWithAddHooks) Add(_ context.Context, ctxName string) error {
	m.addCalled++
	if m.addReturnErr != nil {
		return m.addReturnErr
	}
	if m.known[ctxName] {
		return client.ErrClusterAlreadyExists
	}
	m.known[ctxName] = true
	return nil
}

func (m *fakeManagerWithAddHooks) SetActive(ctxName string) error {
	m.setActiveCalled++
	if m.setActiveErr != nil {
		return m.setActiveErr
	}
	m.active = ctxName
	return nil
}

func (m *fakeManagerWithAddHooks) Remove(ctxName string) error {
	m.removeCalled++
	if m.removeErr != nil {
		return m.removeErr
	}
	delete(m.known, ctxName)
	if m.active == ctxName {
		m.active = ""
	}
	return nil
}

func newAppWithManager(m client.Manager) *App {
	clusters := core.NewClusterServiceWithManager(m)
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

// TestConnect_CallsAddThenSetActive verifies a fresh context is added and
// promoted to active in one step.
func TestConnect_CallsAddThenSetActive(t *testing.T) {
	m := newManagerWithAddHooks()
	a := newAppWithManager(m)

	if err := a.Connect("prod"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if m.addCalled != 1 {
		t.Errorf("Add called %d times, want 1", m.addCalled)
	}
	if m.setActiveCalled != 1 {
		t.Errorf("SetActive called %d times, want 1", m.setActiveCalled)
	}
	if a.GetActiveContext() != "prod" {
		t.Errorf("active = %q, want prod", a.GetActiveContext())
	}
}

// TestConnect_AlreadyExists_IsTolerated pins that an already-known context is
// NOT an error; Connect still promotes it to active.
func TestConnect_AlreadyExists_IsTolerated(t *testing.T) {
	m := newManagerWithAddHooks("prod")
	a := newAppWithManager(m)

	if err := a.Connect("prod"); err != nil {
		t.Fatalf("Connect to existing context should succeed, got %v", err)
	}
	if m.setActiveCalled != 1 {
		t.Errorf("SetActive should still be called once when context existed, got %d", m.setActiveCalled)
	}
}

// TestConnect_AddRealErrorPropagates verifies that other Add errors surface
// up to the caller.
func TestConnect_AddRealErrorPropagates(t *testing.T) {
	m := newManagerWithAddHooks()
	m.addReturnErr = client.ErrNotConnected // a non-ErrClusterAlreadyExists error
	a := newAppWithManager(m)

	err := a.Connect("prod")
	if err == nil {
		t.Fatal("expected error when Add returns non-duplicate error, got nil")
	}
}

// TestDisconnect_CallsRemove verifies disconnect flows through to the manager.
func TestDisconnect_CallsRemove(t *testing.T) {
	m := newManagerWithAddHooks("prod")
	a := newAppWithManager(m)

	if err := a.Disconnect("prod"); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if m.removeCalled != 1 {
		t.Errorf("Remove called %d times, want 1", m.removeCalled)
	}
}

// TestDisconnect_ErrorPropagates ensures failures are surfaced.
func TestDisconnect_ErrorPropagates(t *testing.T) {
	m := newManagerWithAddHooks("prod")
	m.removeErr = client.ErrContextNotFound
	a := newAppWithManager(m)

	err := a.Disconnect("prod")
	if err == nil {
		t.Fatal("expected Disconnect to propagate error, got nil")
	}
}

// ---------------------------------------------------------------------------
// GetAppVersion
// ---------------------------------------------------------------------------

// TestGetAppVersion_ReturnsNonEmpty does not pin the actual value (which
// changes every release) but does assert the shim returns something.
func TestGetAppVersion_ReturnsNonEmpty(t *testing.T) {
	a := newAppWithFakes(nil)
	v := a.GetAppVersion()
	if v == "" {
		t.Error("GetAppVersion returned empty string")
	}
}
