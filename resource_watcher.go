// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"log/slog"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/thepixelabs/kubecat/internal/client"
)

// resourceChangedEvent is the payload emitted on the "resource:changed" event.
type resourceChangedEvent struct {
	EventType string `json:"eventType"` // "ADDED", "MODIFIED", "DELETED"
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
}

// StartResourceWatch begins watching the given resource kind and emits
// "resource:changed" Wails events for every change received from the cluster.
func (a *App) StartResourceWatch(kind, namespace string) error {
	if a.nexus == nil {
		return nil
	}

	cl, err := a.nexus.Clusters.Manager().Active()
	if err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Stop any existing watcher for this kind.
	if cancel, ok := a.watchers[kind]; ok {
		cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.watchers[kind] = cancel

	ch, err := cl.Watch(ctx, kind, client.WatchOptions{Namespace: namespace})
	if err != nil {
		cancel()
		delete(a.watchers, kind)
		return err
	}

	go func() {
		defer func() {
			a.mu.Lock()
			delete(a.watchers, kind)
			a.mu.Unlock()
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-ch:
				if !ok {
					return
				}
				slog.Debug("resource watch event",
					slog.String("kind", kind),
					slog.String("type", event.Type),
					slog.String("name", event.Resource.Name))

				// Guard against nil context (tests, or shutdown racing ahead of
				// startup): runtime.EventsEmit calls log.Fatal when ctx is nil.
				if a.ctx == nil {
					continue
				}
				runtime.EventsEmit(a.ctx, "resource:changed", resourceChangedEvent{
					EventType: event.Type,
					Kind:      event.Resource.Kind,
					Name:      event.Resource.Name,
					Namespace: event.Resource.Namespace,
					Status:    event.Resource.Status,
				})
			}
		}
	}()

	return nil
}

// StopResourceWatch stops the watcher for the given kind.
func (a *App) StopResourceWatch(kind string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if cancel, ok := a.watchers[kind]; ok {
		cancel()
		delete(a.watchers, kind)
	}
}

// StopAllResourceWatchers stops all active resource watchers.
func (a *App) StopAllResourceWatchers() {
	a.mu.Lock()
	defer a.mu.Unlock()

	for kind, cancel := range a.watchers {
		cancel()
		delete(a.watchers, kind)
	}
}
