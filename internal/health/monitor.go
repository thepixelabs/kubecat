// Package health provides cluster connectivity monitoring with automatic
// reconnection and Wails event emission.
package health

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
	"github.com/thepixelabs/kubecat/internal/events"
)

// State represents the cluster connectivity state machine.
type State int

const (
	StateConnected    State = iota // Cluster is reachable.
	StateDegraded                  // Recent failures but not yet fully disconnected.
	StateDisconnected              // Cannot reach the cluster API.
	StateReconnecting              // Actively retrying connection.
)

func (s State) String() string {
	switch s {
	case StateConnected:
		return "connected"
	case StateDegraded:
		return "degraded"
	case StateDisconnected:
		return "disconnected"
	case StateReconnecting:
		return "reconnecting"
	default:
		return "unknown"
	}
}

const (
	heartbeatInterval = 30 * time.Second
	backoffMin        = 1 * time.Second
	backoffMax        = 60 * time.Second
	degradeAfter      = 2 // consecutive failures before moving to degraded
	disconnectAfter   = 5 // consecutive failures before moving to disconnected
)

// StatusEvent is emitted as the data payload for the "cluster:status" event.
type StatusEvent struct {
	Context string `json:"context"`
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
}

// ClusterHealthMonitor runs a heartbeat loop against the active cluster.
type ClusterHealthMonitor struct {
	manager client.Manager
	emitter events.EmitterInterface

	mu         sync.Mutex
	state      State
	failures   int
	backoff    time.Duration
	cancelFunc context.CancelFunc
	done       chan struct{}
}

// NewClusterHealthMonitor creates a monitor. Call Start to begin heartbeating.
func NewClusterHealthMonitor(mgr client.Manager, em events.EmitterInterface) *ClusterHealthMonitor {
	return &ClusterHealthMonitor{
		manager: mgr,
		emitter: em,
		state:   StateDisconnected,
		backoff: backoffMin,
		done:    make(chan struct{}),
	}
}

// Start begins the heartbeat loop in a background goroutine.
func (m *ClusterHealthMonitor) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	m.cancelFunc = cancel
	go m.loop(ctx)
}

// Stop shuts down the heartbeat loop and waits for it to exit.
func (m *ClusterHealthMonitor) Stop() {
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
	<-m.done
}

// CurrentState returns the current health state.
func (m *ClusterHealthMonitor) CurrentState() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// NotifyConnected signals that the cluster just became reachable (e.g. after
// a successful Connect call), resetting failure counts immediately.
func (m *ClusterHealthMonitor) NotifyConnected() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures = 0
	m.backoff = backoffMin
	m.transitionLocked(StateConnected, "")
}

// NotifyDisconnected signals an explicit disconnect.
func (m *ClusterHealthMonitor) NotifyDisconnected() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transitionLocked(StateDisconnected, "explicit disconnect")
}

// loop is the background heartbeat goroutine.
func (m *ClusterHealthMonitor) loop(ctx context.Context) {
	defer close(m.done)

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	// Run an immediate check.
	m.heartbeat(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.heartbeat(ctx)
		}
	}
}

// heartbeat performs one liveness check against the API server.
func (m *ClusterHealthMonitor) heartbeat(ctx context.Context) {
	cl, err := m.manager.Active()
	if err != nil {
		// No active cluster — stay disconnected silently.
		return
	}

	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, apiErr := cl.Info(checkCtx)

	m.mu.Lock()
	defer m.mu.Unlock()

	if apiErr != nil {
		m.failures++
		slog.Debug("cluster heartbeat failed",
			slog.Int("consecutive_failures", m.failures),
			slog.Any("error", apiErr))

		switch {
		case m.failures >= disconnectAfter && m.state != StateDisconnected:
			m.transitionLocked(StateDisconnected, apiErr.Error())
			m.transitionLocked(StateReconnecting, "")
		case m.failures >= degradeAfter && m.state == StateConnected:
			m.transitionLocked(StateDegraded, apiErr.Error())
		}

		// Exponential backoff (not used for ticker yet, but useful for callers).
		m.backoff *= 2
		if m.backoff > backoffMax {
			m.backoff = backoffMax
		}
		return
	}

	// Success.
	if m.state != StateConnected {
		m.transitionLocked(StateConnected, "")
	}
	m.failures = 0
	m.backoff = backoffMin
}

// transitionLocked changes state and emits a Wails event.
// Caller must hold m.mu.
func (m *ClusterHealthMonitor) transitionLocked(next State, message string) {
	if m.state == next {
		return
	}
	prev := m.state
	m.state = next
	slog.Info("cluster health state change",
		slog.String("from", prev.String()),
		slog.String("to", next.String()))

	ctx := m.manager.ActiveContext()
	m.emitter.Emit("cluster:status", StatusEvent{
		Context: ctx,
		State:   next.String(),
		Message: message,
	})
}
