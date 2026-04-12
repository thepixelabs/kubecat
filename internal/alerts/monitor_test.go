// SPDX-License-Identifier: Apache-2.0

package alerts

import (
	"context"
	"testing"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
)

// ── mock emitter ─────────────────────────────────────────────────────────────

type mockEmitter struct {
	events []mockEvent
}

type mockEvent struct {
	name string
	data interface{}
}

func (m *mockEmitter) Emit(event string, data ...interface{}) {
	var d interface{}
	if len(data) > 0 {
		d = data[0]
	}
	m.events = append(m.events, mockEvent{name: event, data: d})
}

func (m *mockEmitter) SetContext(_ context.Context) {}

// ── mock manager ─────────────────────────────────────────────────────────────

type mockManager struct {
	clusters []client.ClusterInfo
	clients  map[string]client.ClusterClient
}

func (m *mockManager) Add(_ context.Context, _ string) error         { return nil }
func (m *mockManager) Remove(_ string) error                         { return nil }
func (m *mockManager) Get(ctx string) (client.ClusterClient, error)  { return m.clients[ctx], nil }
func (m *mockManager) Active() (client.ClusterClient, error)         { return nil, nil }
func (m *mockManager) SetActive(_ string) error                      { return nil }
func (m *mockManager) List() []client.ClusterInfo                    { return m.clusters }
func (m *mockManager) Contexts() ([]string, error)                   { return nil, nil }
func (m *mockManager) Close() error                                  { return nil }
func (m *mockManager) ActiveContext() string                         { return "" }
func (m *mockManager) RefreshInfo(_ context.Context, _ string) error { return nil }
func (m *mockManager) ReloadContexts() ([]string, error)             { return nil, nil }

// ── mock cluster client ───────────────────────────────────────────────────────

type mockCluster struct {
	pods []client.Resource
	pvcs []client.Resource
}

func (c *mockCluster) Info(_ context.Context) (*client.ClusterInfo, error) { return nil, nil }
func (c *mockCluster) List(_ context.Context, kind string, _ client.ListOptions) (*client.ResourceList, error) {
	switch kind {
	case "pods":
		return &client.ResourceList{Items: c.pods}, nil
	case "persistentvolumeclaims":
		return &client.ResourceList{Items: c.pvcs}, nil
	}
	return &client.ResourceList{}, nil
}
func (c *mockCluster) Get(_ context.Context, _, _, _ string) (*client.Resource, error) {
	return nil, nil
}
func (c *mockCluster) Delete(_ context.Context, _, _, _ string) error { return nil }
func (c *mockCluster) Watch(_ context.Context, _ string, _ client.WatchOptions) (<-chan client.WatchEvent, error) {
	return nil, nil
}
func (c *mockCluster) Logs(_ context.Context, _, _, _ string, _ bool, _ int64) (<-chan string, error) {
	return nil, nil
}
func (c *mockCluster) Exec(_ context.Context, _, _, _ string, _ []string) error { return nil }
func (c *mockCluster) PortForward(_ context.Context, _, _ string, _, _ int) (client.PortForwarder, error) {
	return nil, nil
}
func (c *mockCluster) Close() error { return nil }

// ── helpers ───────────────────────────────────────────────────────────────────

func podResource(name, namespace, status string) client.Resource {
	return client.Resource{
		Name:      name,
		Namespace: namespace,
		Kind:      "Pod",
		Status:    status,
	}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestScanCluster_CrashLoopPod_EmitsAlert(t *testing.T) {
	em := &mockEmitter{}
	mgr := &mockManager{}
	monitor := NewAlertMonitor(mgr, em)

	cl := &mockCluster{
		pods: []client.Resource{
			podResource("my-pod", "default", "CrashLoopBackOff"),
		},
	}

	monitor.scanCluster(context.Background(), cl, "test-cluster")

	if len(em.events) == 0 {
		t.Fatal("expected at least one alert event, got none")
	}
	if em.events[0].name != "ai:alert" {
		t.Errorf("expected event name %q, got %q", "ai:alert", em.events[0].name)
	}
	alert, ok := em.events[0].data.(Alert)
	if !ok {
		t.Fatalf("expected Alert payload, got %T", em.events[0].data)
	}
	if alert.Severity != "critical" {
		t.Errorf("expected severity critical, got %q", alert.Severity)
	}
	if alert.Name != "my-pod" {
		t.Errorf("expected pod name my-pod, got %q", alert.Name)
	}
	if alert.SuggestedQuery == "" {
		t.Error("SuggestedQuery must not be empty")
	}
}

func TestScanCluster_HealthyPod_NoAlert(t *testing.T) {
	em := &mockEmitter{}
	mgr := &mockManager{}
	monitor := NewAlertMonitor(mgr, em)
	cl := &mockCluster{
		pods: []client.Resource{
			podResource("ok-pod", "default", "Running"),
		},
	}
	monitor.scanCluster(context.Background(), cl, "test-cluster")
	if len(em.events) != 0 {
		t.Errorf("expected no alerts for healthy pod, got %d", len(em.events))
	}
}

func TestScanCluster_Deduplication_CooldownSuppressesDuplicate(t *testing.T) {
	em := &mockEmitter{}
	mgr := &mockManager{}
	monitor := NewAlertMonitor(mgr, em)
	cl := &mockCluster{
		pods: []client.Resource{
			podResource("crash-pod", "default", "CrashLoopBackOff"),
		},
	}

	monitor.scanCluster(context.Background(), cl, "test-cluster")
	monitor.scanCluster(context.Background(), cl, "test-cluster")

	if len(em.events) != 1 {
		t.Errorf("deduplication should suppress repeated alert within cooldown; got %d events", len(em.events))
	}
}

func TestScanCluster_Deduplication_EmitsAfterCooldownExpiry(t *testing.T) {
	em := &mockEmitter{}
	mgr := &mockManager{}
	monitor := NewAlertMonitor(mgr, em)

	// Manually seed the lastSeen map with an old timestamp.
	key := dedupKey{
		cluster:   "test-cluster",
		namespace: "default",
		kind:      "Pod",
		name:      "crash-pod",
		message:   "Pod default/crash-pod is in CrashLoopBackOff state",
	}
	monitor.mu.Lock()
	monitor.lastSeen[key] = time.Now().Add(-2 * cooldown)
	monitor.mu.Unlock()

	cl := &mockCluster{
		pods: []client.Resource{
			podResource("crash-pod", "default", "CrashLoopBackOff"),
		},
	}
	monitor.scanCluster(context.Background(), cl, "test-cluster")

	if len(em.events) != 1 {
		t.Errorf("expected 1 alert after cooldown expiry, got %d", len(em.events))
	}
}

func TestScanCluster_FailedPod_EmitsWarning(t *testing.T) {
	em := &mockEmitter{}
	mgr := &mockManager{}
	monitor := NewAlertMonitor(mgr, em)
	cl := &mockCluster{
		pods: []client.Resource{
			podResource("failed-pod", "kube-system", "Failed"),
		},
	}
	monitor.scanCluster(context.Background(), cl, "test-cluster")
	if len(em.events) == 0 {
		t.Fatal("expected alert for Failed pod")
	}
	alert := em.events[0].data.(Alert)
	if alert.Severity != "warning" {
		t.Errorf("expected warning severity, got %q", alert.Severity)
	}
}
