// SPDX-License-Identifier: Apache-2.0

package history

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
	"github.com/thepixelabs/kubecat/internal/events"
	"github.com/thepixelabs/kubecat/internal/storage"
)

// EventCollectorConfig configures the event collector.
type EventCollectorConfig struct {
	// Retention is how long to keep events.
	Retention time.Duration
	// CleanupInterval is how often to run cleanup.
	CleanupInterval time.Duration
}

// DefaultEventCollectorConfig returns sensible defaults.
func DefaultEventCollectorConfig() EventCollectorConfig {
	return EventCollectorConfig{
		Retention:       7 * 24 * time.Hour, // 7 days
		CleanupInterval: time.Hour,
	}
}

// EventCollector watches and persists Kubernetes events.
type EventCollector struct {
	config  EventCollectorConfig
	db      *storage.DB
	repo    *storage.EventRepository
	manager client.Manager
	// emitter (optional) pushes reactive events to the frontend via Wails.
	emitter events.EmitterInterface
	// correlator (optional) is used to create correlation links as events are ingested.
	correlator *Correlator
	mu         sync.Mutex
	running    bool
	watches    map[string]context.CancelFunc
	stopCh     chan struct{}
}

// NewEventCollector creates a new event collector.
// emitter may be nil; when nil, no reactive push events are sent.
func NewEventCollector(db *storage.DB, manager client.Manager, config EventCollectorConfig, emitter events.EmitterInterface) *EventCollector {
	return &EventCollector{
		config:  config,
		db:      db,
		repo:    storage.NewEventRepository(db),
		manager: manager,
		emitter: emitter,
		watches: make(map[string]context.CancelFunc),
	}
}

// SetCorrelator sets an optional correlator to run on ingested events.
func (c *EventCollector) SetCorrelator(corr *Correlator) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.correlator = corr
}

// Start starts collecting events from all clusters.
func (c *EventCollector) Start() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.stopCh = make(chan struct{})
	c.mu.Unlock()

	go c.run()
}

// Stop stops collecting events.
func (c *EventCollector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	close(c.stopCh)
	c.running = false

	// Cancel all watches
	for _, cancel := range c.watches {
		cancel()
	}
	c.watches = make(map[string]context.CancelFunc)
}

// run is the main event collection loop.
func (c *EventCollector) run() {
	// Start watches for all connected clusters
	c.refreshWatches()

	// Periodically refresh watches and cleanup
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.refreshWatches()
			c.cleanup()
		}
	}
}

// Refresh forces a refresh of cluster watches.
func (c *EventCollector) Refresh() {
	c.refreshWatches()
}

// refreshWatches ensures watches are running for all connected clusters.
func (c *EventCollector) refreshWatches() {
	c.mu.Lock()
	defer c.mu.Unlock()

	clusters := c.manager.List()
	activeContexts := make(map[string]bool)

	for _, cluster := range clusters {
		activeContexts[cluster.Context] = true

		// Start watch if not already running
		if _, exists := c.watches[cluster.Context]; !exists {
			ctx, cancel := context.WithCancel(context.Background())
			c.watches[cluster.Context] = cancel
			go c.watchCluster(ctx, cluster.Context)
		}
	}

	// Stop watches for disconnected clusters
	for contextName, cancel := range c.watches {
		if !activeContexts[contextName] {
			cancel()
			delete(c.watches, contextName)
		}
	}
}

// watchCluster watches events for a single cluster.
func (c *EventCollector) watchCluster(ctx context.Context, clusterName string) {
	clusterClient, err := c.manager.Get(clusterName)
	if err != nil {
		return
	}

	// First, list existing events
	c.syncExistingEvents(ctx, clusterClient, clusterName)

	// Then watch for new events
	eventCh, err := clusterClient.Watch(ctx, "events", client.WatchOptions{})
	if err != nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventCh:
			if !ok {
				// Channel closed, reconnect with exponential backoff
				// (1s → 2s → 4s … capped at 60s) and infinite retry until
				// the parent context is canceled. The previous behavior
				// was a single 5s sleep + give-up on first failure, which
				// silently dropped the watcher on any transient API hiccup.
				backoff := 1 * time.Second
				const backoffMax = 60 * time.Second
				for {
					select {
					case <-ctx.Done():
						return
					case <-time.After(backoff):
					}
					eventCh, err = clusterClient.Watch(ctx, "events", client.WatchOptions{})
					if err == nil {
						break
					}
					backoff *= 2
					if backoff > backoffMax {
						backoff = backoffMax
					}
				}
				continue
			}

			c.processEvent(ctx, clusterName, &event.Resource)
		}
	}
}

// syncExistingEvents loads existing events from the cluster.
func (c *EventCollector) syncExistingEvents(ctx context.Context, clusterClient client.ClusterClient, clusterName string) {
	list, err := clusterClient.List(ctx, "events", client.ListOptions{Limit: 1000})
	if err != nil {
		return
	}

	for _, resource := range list.Items {
		c.processEvent(ctx, clusterName, &resource)
	}
}

// processEvent processes a single Kubernetes event.
func (c *EventCollector) processEvent(ctx context.Context, clusterName string, resource *client.Resource) {
	if resource == nil {
		return
	}

	// Parse the event data
	event, err := parseK8sEvent(resource)
	if err != nil {
		return
	}

	storedEvent := &storage.StoredEvent{
		Cluster:         clusterName,
		Namespace:       event.Namespace,
		Kind:            event.InvolvedObjectKind,
		Name:            event.InvolvedObjectName,
		Type:            event.Type,
		Reason:          event.Reason,
		Message:         event.Message,
		FirstSeen:       event.FirstTimestamp,
		LastSeen:        event.LastTimestamp,
		Count:           event.Count,
		SourceComponent: event.SourceComponent,
		SourceHost:      event.SourceHost,
	}

	if err := c.repo.Save(ctx, storedEvent); err != nil {
		return
	}

	// Emit a reactive push event for Warning-severity events so the frontend
	// can update the timeline without polling.  The emitter is nil-safe: when
	// no emitter is wired (e.g. in unit tests) this block is a no-op.
	if storedEvent.Type == "Warning" && c.emitter != nil {
		c.emitter.Emit("cluster:event", map[string]interface{}{
			"cluster":   storedEvent.Cluster,
			"namespace": storedEvent.Namespace,
			"kind":      storedEvent.Kind,
			"name":      storedEvent.Name,
			"reason":    storedEvent.Reason,
			"message":   storedEvent.Message,
		})
	}

	// Best-effort correlation (non-blocking for main ingest flow).
	c.mu.Lock()
	corr := c.correlator
	c.mu.Unlock()
	if corr != nil && storedEvent.ID != 0 {
		_, _ = corr.CorrelateEvent(ctx, *storedEvent)
	}
}

// cleanup removes old events.
func (c *EventCollector) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cutoff := time.Now().UTC().Add(-c.config.Retention)
	if _, err := c.repo.DeleteOlderThan(ctx, cutoff); err != nil {
		slog.Warn("event collector cleanup failed", slog.Any("error", err))
	}
}

// GetEvents retrieves events matching the filter.
func (c *EventCollector) GetEvents(ctx context.Context, filter storage.EventFilter) ([]storage.StoredEvent, error) {
	return c.repo.List(ctx, filter)
}

// GetRecentEvents retrieves recent events for a resource.
func (c *EventCollector) GetRecentEvents(ctx context.Context, cluster, namespace, kind, name string, limit int) ([]storage.StoredEvent, error) {
	return c.repo.List(ctx, storage.EventFilter{
		Cluster:   cluster,
		Namespace: namespace,
		Kind:      kind,
		Name:      name,
		Limit:     limit,
	})
}

// k8sEvent represents a parsed Kubernetes event.
type k8sEvent struct {
	Namespace          string
	InvolvedObjectKind string
	InvolvedObjectName string
	Type               string
	Reason             string
	Message            string
	FirstTimestamp     time.Time
	LastTimestamp      time.Time
	Count              int
	SourceComponent    string
	SourceHost         string
}

// parseK8sEvent parses a Kubernetes event from a resource.
func parseK8sEvent(resource *client.Resource) (*k8sEvent, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(resource.Raw, &raw); err != nil {
		return nil, err
	}

	event := &k8sEvent{
		Namespace: resource.Namespace,
		Count:     1,
	}

	// Extract involved object
	if involvedObject, ok := raw["involvedObject"].(map[string]interface{}); ok {
		if kind, ok := involvedObject["kind"].(string); ok {
			event.InvolvedObjectKind = kind
		}
		if name, ok := involvedObject["name"].(string); ok {
			event.InvolvedObjectName = name
		}
	}

	// Extract event type and reason
	if t, ok := raw["type"].(string); ok {
		event.Type = t
	}
	if reason, ok := raw["reason"].(string); ok {
		event.Reason = reason
	}
	if message, ok := raw["message"].(string); ok {
		event.Message = message
	}

	// Extract timestamps
	if firstTs, ok := raw["firstTimestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, firstTs); err == nil {
			event.FirstTimestamp = t
		}
	}
	if lastTs, ok := raw["lastTimestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, lastTs); err == nil {
			event.LastTimestamp = t
		}
	} else {
		event.LastTimestamp = time.Now().UTC()
	}

	// Extract count
	if count, ok := raw["count"].(float64); ok {
		event.Count = int(count)
	}

	// Extract source
	if source, ok := raw["source"].(map[string]interface{}); ok {
		if component, ok := source["component"].(string); ok {
			event.SourceComponent = component
		}
		if host, ok := source["host"].(string); ok {
			event.SourceHost = host
		}
	}

	return event, nil
}
