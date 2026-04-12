// Package history provides time-travel debugging capabilities.
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

// SnapshotterConfig configures the snapshotter.
type SnapshotterConfig struct {
	// Interval is how often to take snapshots.
	Interval time.Duration
	// Retention is how long to keep snapshots.
	Retention time.Duration
	// ResourceKinds are the kinds to snapshot (empty for all common kinds).
	ResourceKinds []string
}

// DefaultSnapshotterConfig returns sensible defaults.
func DefaultSnapshotterConfig() SnapshotterConfig {
	return SnapshotterConfig{
		Interval:  5 * time.Minute,
		Retention: 7 * 24 * time.Hour, // 7 days
		ResourceKinds: []string{
			"pods", "deployments", "services", "configmaps",
			"statefulsets", "daemonsets", "jobs", "cronjobs",
		},
	}
}

// Snapshotter periodically captures cluster state.
type Snapshotter struct {
	config  SnapshotterConfig
	db      *storage.DB
	repo    *storage.SnapshotRepository
	manager client.Manager
	// emitter (optional) pushes reactive events to the frontend via Wails.
	emitter  events.EmitterInterface
	mu       sync.Mutex
	running  bool
	stopChan chan struct{}
}

// NewSnapshotter creates a new snapshotter.
// emitter may be nil; when nil, no reactive push events are sent.
func NewSnapshotter(db *storage.DB, manager client.Manager, config SnapshotterConfig, emitter events.EmitterInterface) *Snapshotter {
	return &Snapshotter{
		config:  config,
		db:      db,
		repo:    storage.NewSnapshotRepository(db),
		manager: manager,
		emitter: emitter,
	}
}

// Start starts the periodic snapshotting.
func (s *Snapshotter) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopChan = make(chan struct{})
	s.mu.Unlock()

	go s.run()
}

// Stop stops the periodic snapshotting.
func (s *Snapshotter) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	close(s.stopChan)
	s.running = false
}

// run is the main snapshot loop.
func (s *Snapshotter) run() {
	ticker := time.NewTicker(s.config.Interval)
	defer ticker.Stop()

	// Take an initial snapshot
	s.takeSnapshot()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.takeSnapshot()
			s.cleanup()
		}
	}
}

// takeSnapshot captures the current state of all clusters.
func (s *Snapshotter) takeSnapshot() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	clusters := s.manager.List()
	for _, cluster := range clusters {
		if err := s.snapshotCluster(ctx, cluster.Context); err != nil {
			// Log error but continue with other clusters
			continue
		}
	}
}

// snapshotCluster captures the state of a single cluster.
func (s *Snapshotter) snapshotCluster(ctx context.Context, clusterName string) error {
	clusterClient, err := s.manager.Get(clusterName)
	if err != nil {
		return err
	}

	snapshot := &storage.SnapshotData{
		Cluster:   clusterName,
		Timestamp: time.Now(),
		Resources: make(map[string][]storage.ResourceInfo),
	}

	// Get namespaces
	nsList, err := clusterClient.List(ctx, "namespaces", client.ListOptions{Limit: 1000})
	if err == nil {
		for _, ns := range nsList.Items {
			snapshot.Namespaces = append(snapshot.Namespaces, ns.Name)
		}
	}

	// Snapshot each resource kind
	for _, kind := range s.config.ResourceKinds {
		list, err := clusterClient.List(ctx, kind, client.ListOptions{Limit: 10000})
		if err != nil {
			continue
		}

		resources := make([]storage.ResourceInfo, 0, len(list.Items))
		for _, item := range list.Items {
			resources = append(resources, storage.ResourceInfo{
				Name:            item.Name,
				Namespace:       item.Namespace,
				ResourceVersion: getResourceVersion(item.Raw),
				Labels:          item.Labels,
				Status:          item.Status,
			})
		}
		snapshot.Resources[kind] = resources
	}

	if err := s.repo.Save(ctx, clusterName, snapshot); err != nil {
		return err
	}

	// Emit a reactive push event so the frontend can refresh the timeline
	// without polling.  The emitter is nil-safe.
	if s.emitter != nil {
		s.emitter.Emit("snapshot:taken", map[string]interface{}{
			"cluster":   clusterName,
			"timestamp": snapshot.Timestamp.UTC().Format(time.RFC3339),
		})
	}

	return nil
}

// cleanup removes old snapshots.
func (s *Snapshotter) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cutoff := time.Now().Add(-s.config.Retention)
	if _, err := s.repo.DeleteOlderThan(ctx, cutoff); err != nil {
		slog.Warn("snapshotter cleanup failed", slog.Any("error", err))
	}
}

// TakeManualSnapshot takes an immediate snapshot.
func (s *Snapshotter) TakeManualSnapshot(ctx context.Context) error {
	clusters := s.manager.List()
	for _, cluster := range clusters {
		if err := s.snapshotCluster(ctx, cluster.Context); err != nil {
			return err
		}
	}
	return nil
}

// GetSnapshot retrieves a snapshot at a specific time.
func (s *Snapshotter) GetSnapshot(ctx context.Context, cluster string, at time.Time) (*storage.SnapshotData, error) {
	return s.repo.GetAt(ctx, cluster, at)
}

// GetLatestSnapshot retrieves the most recent snapshot.
func (s *Snapshotter) GetLatestSnapshot(ctx context.Context, cluster string) (*storage.SnapshotData, error) {
	return s.repo.GetLatest(ctx, cluster)
}

// ListSnapshots lists available snapshot timestamps.
func (s *Snapshotter) ListSnapshots(ctx context.Context, cluster string, limit int) ([]time.Time, error) {
	return s.repo.ListTimestamps(ctx, cluster, limit)
}

// getResourceVersion extracts resource version from raw JSON.
func getResourceVersion(raw []byte) string {
	var meta struct {
		Metadata struct {
			ResourceVersion string `json:"resourceVersion"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return ""
	}
	return meta.Metadata.ResourceVersion
}

// CompareSnapshots compares two snapshots and returns the differences.
func CompareSnapshots(before, after *storage.SnapshotData) *SnapshotDiff {
	diff := &SnapshotDiff{
		Before: before.Timestamp,
		After:  after.Timestamp,
	}

	// Compare each resource kind
	allKinds := make(map[string]bool)
	for kind := range before.Resources {
		allKinds[kind] = true
	}
	for kind := range after.Resources {
		allKinds[kind] = true
	}

	for kind := range allKinds {
		beforeResources := before.Resources[kind]
		afterResources := after.Resources[kind]

		beforeMap := make(map[string]storage.ResourceInfo)
		for _, r := range beforeResources {
			key := r.Namespace + "/" + r.Name
			beforeMap[key] = r
		}

		afterMap := make(map[string]storage.ResourceInfo)
		for _, r := range afterResources {
			key := r.Namespace + "/" + r.Name
			afterMap[key] = r
		}

		// Find added resources
		for key, r := range afterMap {
			if _, exists := beforeMap[key]; !exists {
				diff.Added = append(diff.Added, ResourceChange{
					Kind:      kind,
					Name:      r.Name,
					Namespace: r.Namespace,
				})
			}
		}

		// Find removed resources
		for key, r := range beforeMap {
			if _, exists := afterMap[key]; !exists {
				diff.Removed = append(diff.Removed, ResourceChange{
					Kind:      kind,
					Name:      r.Name,
					Namespace: r.Namespace,
				})
			}
		}

		// Find modified resources
		for key, beforeR := range beforeMap {
			if afterR, exists := afterMap[key]; exists {
				if beforeR.Status != afterR.Status || beforeR.ResourceVersion != afterR.ResourceVersion {
					diff.Modified = append(diff.Modified, ResourceChange{
						Kind:      kind,
						Name:      beforeR.Name,
						Namespace: beforeR.Namespace,
						OldStatus: beforeR.Status,
						NewStatus: afterR.Status,
					})
				}
			}
		}
	}

	return diff
}

// SnapshotDiff represents differences between two snapshots.
type SnapshotDiff struct {
	Before   time.Time
	After    time.Time
	Added    []ResourceChange
	Removed  []ResourceChange
	Modified []ResourceChange
}

// ResourceChange represents a change to a resource.
type ResourceChange struct {
	Kind      string
	Name      string
	Namespace string
	OldStatus string
	NewStatus string
}
