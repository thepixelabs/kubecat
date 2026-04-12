// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/thepixelabs/kubecat/internal/client"
)

// reservedLocalPorts lists ports that must not be used as the local side of a
// port-forward because they are occupied by the application's own services.
// 5173  — Vite dev server (frontend:dev:watcher)
// 34115 — Wails internal IPC bridge
// 34116 — Wails internal IPC bridge (secondary)
var reservedLocalPorts = map[int]string{
	5173:  "Vite dev server",
	34115: "Wails IPC bridge",
	34116: "Wails IPC bridge (secondary)",
}

// validatePortForwardPorts checks that localPort and remotePort are within
// acceptable ranges and that localPort does not conflict with known local
// services or system-privileged port numbers.
//
// localPort must be in [1024, 65535]:
//   - 0        is reserved / ephemeral-only
//   - 1–1023   are privileged; unprivileged processes must not bind them
//
// remotePort must be in [1, 65535]:
//   - 0 is reserved and invalid as a named target port
func validatePortForwardPorts(localPort, remotePort int) error {
	if localPort < 1024 || localPort > 65535 {
		return fmt.Errorf("localPort %d is out of range: must be 1024–65535 (ports 1–1023 are privileged)", localPort)
	}
	if remotePort < 1 || remotePort > 65535 {
		return fmt.Errorf("remotePort %d is out of range: must be 1–65535", remotePort)
	}
	if svc, reserved := reservedLocalPorts[localPort]; reserved {
		return fmt.Errorf("localPort %d conflicts with %s — choose a different local port", localPort, svc)
	}
	return nil
}

// checkLocalPortAvailable probes whether localPort is already bound on the
// loopback interface. It opens and immediately closes a TCP listener; if that
// fails the port is in use and a descriptive error is returned.
func checkLocalPortAvailable(localPort int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", localPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("localPort %d is already in use: %w", localPort, err)
	}
	// Port is available — release the listener before the real forwarder binds.
	_ = ln.Close()
	return nil
}

// PortForwardService manages port forwarding sessions.
type PortForwardService struct {
	clusterService *ClusterService
	mu             sync.RWMutex
	forwards       map[string]*ActiveForward
}

// ActiveForward represents an active port forward session.
type ActiveForward struct {
	ID         string
	Namespace  string
	Pod        string
	LocalPort  int
	RemotePort int
	Status     string
	Error      string
	forwarder  client.PortForwarder
}

// NewPortForwardService creates a new port forward service.
func NewPortForwardService(cs *ClusterService) *PortForwardService {
	return &PortForwardService{
		clusterService: cs,
		forwards:       make(map[string]*ActiveForward),
	}
}

// CreateForward creates a new port forward.
//
// Validation is performed before any network operation:
//  1. localPort and remotePort are checked for valid, non-privileged ranges.
//  2. localPort is probed to ensure it is not already bound locally.
//
// On success the forward is logged at INFO level with namespace, pod, and port
// details so that all port-forward activity is auditable.
func (s *PortForwardService) CreateForward(ctx context.Context, namespace, pod string, localPort, remotePort int) (*ActiveForward, error) {
	if err := validatePortForwardPorts(localPort, remotePort); err != nil {
		return nil, fmt.Errorf("port-forward parameter validation failed: %w", err)
	}

	if err := checkLocalPortAvailable(localPort); err != nil {
		return nil, fmt.Errorf("port-forward pre-bind check failed: %w", err)
	}

	c, err := s.clusterService.Manager().Active()
	if err != nil {
		return nil, err
	}

	log.Printf("[portforward] creating forward: namespace=%s pod=%s localPort=%d remotePort=%d",
		namespace, pod, localPort, remotePort)

	fwd, err := c.PortForward(ctx, namespace, pod, localPort, remotePort)
	if err != nil {
		return nil, err
	}

	id := fmt.Sprintf("%s/%s:%d->%d", namespace, pod, localPort, remotePort)
	active := &ActiveForward{
		ID:         id,
		Namespace:  namespace,
		Pod:        pod,
		LocalPort:  fwd.LocalPort(),
		RemotePort: remotePort,
		Status:     "Active",
		forwarder:  fwd,
	}

	s.mu.Lock()
	s.forwards[id] = active
	s.mu.Unlock()

	log.Printf("[portforward] active: id=%s namespace=%s pod=%s localPort=%d remotePort=%d",
		id, namespace, pod, active.LocalPort, remotePort)

	// Monitor for errors
	go func() {
		<-fwd.Done()
		s.mu.Lock()
		if f, ok := s.forwards[id]; ok {
			if err := fwd.Error(); err != nil {
				f.Status = "Error"
				f.Error = err.Error()
			} else {
				f.Status = "Stopped"
			}
		}
		s.mu.Unlock()
	}()

	return active, nil
}

// StopForward stops a port forward.
func (s *PortForwardService) StopForward(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fwd, ok := s.forwards[id]
	if !ok {
		return fmt.Errorf("port forward not found: %s", id)
	}

	if fwd.forwarder != nil {
		fwd.forwarder.Stop()
	}
	delete(s.forwards, id)
	return nil
}

// ListForwards returns all active port forwards.
func (s *PortForwardService) ListForwards() []*ActiveForward {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ActiveForward, 0, len(s.forwards))
	for _, fwd := range s.forwards {
		result = append(result, fwd)
	}
	return result
}

// StopAll stops all port forwards.
func (s *PortForwardService) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, fwd := range s.forwards {
		if fwd.forwarder != nil {
			fwd.forwarder.Stop()
		}
	}
	s.forwards = make(map[string]*ActiveForward)
}
