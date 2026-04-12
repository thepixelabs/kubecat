// SPDX-License-Identifier: Apache-2.0

package main

// PortForwardInfo is a JSON-friendly port forward info.
type PortForwardInfo struct {
	ID         string `json:"id"`
	Namespace  string `json:"namespace"`
	Pod        string `json:"pod"`
	LocalPort  int    `json:"localPort"`
	RemotePort int    `json:"remotePort"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

// CreatePortForward creates a new port forward.
func (a *App) CreatePortForward(namespace, pod string, localPort, remotePort int) (*PortForwardInfo, error) {
	fwd, err := a.nexus.PortForwards.CreateForward(a.ctx, namespace, pod, localPort, remotePort)
	if err != nil {
		return nil, err
	}
	return &PortForwardInfo{
		ID:         fwd.ID,
		Namespace:  fwd.Namespace,
		Pod:        fwd.Pod,
		LocalPort:  fwd.LocalPort,
		RemotePort: fwd.RemotePort,
		Status:     fwd.Status,
		Error:      fwd.Error,
	}, nil
}

// StopPortForward stops a port forward.
func (a *App) StopPortForward(id string) error {
	return a.nexus.PortForwards.StopForward(id)
}

// ListPortForwards returns all active port forwards.
func (a *App) ListPortForwards() []PortForwardInfo {
	forwards := a.nexus.PortForwards.ListForwards()
	result := make([]PortForwardInfo, len(forwards))
	for i, f := range forwards {
		result[i] = PortForwardInfo{
			ID:         f.ID,
			Namespace:  f.Namespace,
			Pod:        f.Pod,
			LocalPort:  f.LocalPort,
			RemotePort: f.RemotePort,
			Status:     f.Status,
			Error:      f.Error,
		}
	}
	return result
}
