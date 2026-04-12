# History System

> Applies to: Kubecat v0.x (2026). Last updated: 2026-04-07.

This document describes the history subsystem: how Kubecat collects Kubernetes events, periodically snapshots cluster state, and correlates related events to help you understand causation.

**Audience:** Developers extending the history system and operators troubleshooting history collection issues.

## Overview

The history subsystem has three components, all in `internal/history/`:

| Component | File | Responsibility |
|-----------|------|---------------|
| `EventCollector` | `events.go` | Watches K8s events in real time, persists them to SQLite |
| `Snapshotter` | `snapshotter.go` | Periodic cluster state captures every 5 minutes |
| `Correlator` | `correlator.go` | Links related events using configurable rules |

All three run as background goroutines started during `App.startup()`. If the database fails to open at startup, the entire history subsystem is disabled gracefully — the app works without history but logs a warning.

## EventCollector

### Purpose

Maintains a live, queryable record of Kubernetes events so users can see what happened in their clusters over the last N days (default: 7), without requiring `kubectl get events` access at query time.

### Lifecycle

```
NewEventCollector(db, manager, DefaultEventCollectorConfig())
    │
    ▼
Start()
  └── go run()
        ├── refreshWatches()         // start watches for connected clusters
        └── every 1 hour:
              refreshWatches()       // add watches for newly connected clusters
              cleanup(cutoff)        // purge events older than retention period
```

When a new cluster connects (triggered by `App.Connect()`), `Refresh()` is called explicitly so the watch starts immediately without waiting for the hourly tick.

### Per-Cluster Watch

For each connected cluster, a goroutine runs `watchCluster(ctx, clusterName)`:

1. **Initial sync** — `clusterClient.List("events", limit=1000)` backfills recent events into SQLite.
2. **Streaming watch** — `clusterClient.Watch("events")` streams new and modified events.
3. **Reconnect with backoff** — if the watch channel closes (API disconnect, network interruption), the goroutine retries with exponential backoff: 1s → 2s → 4s → ... → 60s cap. Retries continue indefinitely until the context is cancelled.

### Event Processing

Each event from the watch stream goes through `processEvent()`:

1. Parse raw JSON into a `k8sEvent` struct: extracts `involvedObject.kind`, `involvedObject.name`, `type`, `reason`, `message`, timestamps, and `source.component`/`source.host`.
2. Save to the `events` table via `EventRepository.Save()`. The repository uses `INSERT OR REPLACE` to handle duplicate events (same cluster/namespace/kind/name, updated count and last_seen).
3. If a `Correlator` is configured, call `CorrelateEvent()` non-blocking (errors are discarded to avoid slowing event ingestion).

### Configuration

```go
type EventCollectorConfig struct {
    Retention       time.Duration  // default: 7 * 24 * time.Hour
    CleanupInterval time.Duration  // default: time.Hour
}
```

## Snapshotter

### Purpose

Provides time-travel: the ability to see what resources existed at any point in the last N days. The `CompareSnapshots()` function drives the Cluster Diff view.

### Lifecycle

```
NewSnapshotter(db, manager, DefaultSnapshotterConfig())
    │
    ▼
Start()
  └── go run()
        ├── takeSnapshot()           // immediate snapshot on startup
        └── every 5 minutes:
              takeSnapshot()
              cleanup(cutoff)        // purge snapshots older than retention period
```

### Snapshot Content

For each connected cluster, `snapshotCluster()` captures:

1. All namespace names (LIST `/api/v1/namespaces`, limit 1000).
2. For each resource kind in the configured list: name, namespace, resource version, labels, and status (LIST, limit 10000 per kind).

Captured kinds (default): `pods`, `deployments`, `services`, `configmaps`, `statefulsets`, `daemonsets`, `jobs`, `cronjobs`.

The `SnapshotData` struct is JSON-encoded, zlib-compressed, and stored as a BLOB in the `snapshots` table.

### Snapshot Comparison

`CompareSnapshots(before, after *SnapshotData) *SnapshotDiff` compares two snapshots and returns three lists:

- **Added** — resources present in `after` but not in `before`.
- **Removed** — resources present in `before` but not in `after`.
- **Modified** — resources present in both where `Status` or `ResourceVersion` changed.

This is the data source for the Cluster Diff feature.

### Manual Snapshots

The App bridge exposes `TakeManualSnapshot(ctx)` for the frontend to request an immediate snapshot (e.g., before applying a change, to create a known-good baseline).

### Configuration

```go
type SnapshotterConfig struct {
    Interval      time.Duration   // default: 5 * time.Minute
    Retention     time.Duration   // default: 7 * 24 * time.Hour
    ResourceKinds []string        // configurable list of kinds to snapshot
}
```

## Correlator

### Purpose

Links events that are causally related so the Timeline view can show "this Deployment scale caused these Pod events" rather than an undifferentiated stream of events.

### How Correlation Works

`CorrelateEvent(ctx, newEvent)` runs after every event is persisted:

1. Query recent events within the rule's time window that match the source kind/reason.
2. Query recent events that match the target kind.
3. If both source and target events exist within the time window, insert a row into the `correlations` table with the rule's confidence score and relationship label.

### Built-In Rules

Defined in `DefaultCorrelationRules`:

| Rule | When | Confidence | Relationship label |
|------|------|-----------|-------------------|
| `configmap-to-pod` | ConfigMap event ↔ Pod event within 5 min | 0.70 | `config_change_affected` |
| `deployment-scaling` | Deployment `ScalingReplicaSet` ↔ Pod event within 2 min | 0.95 | `scaling_caused` |
| `replicaset-to-pod` | ReplicaSet event ↔ Pod event within 2 min | 0.90 | `managed_by` |
| `job-to-pod` | Job event ↔ Pod event within 2 min | varies | varies |

Confidence scores are surfaced in the Timeline view to help you judge which correlations are reliable.

### Adding Custom Rules

Custom correlation rules can be added programmatically by constructing a `Correlator` with additional `CorrelationRule` entries. This is not yet configurable via `config.yaml`.

## Querying History

History is surfaced to the frontend through App bridge methods:

| Method | Returns |
|--------|---------|
| `GetEvents(filter)` | Filtered event list from SQLite |
| `GetRecentEvents(cluster, namespace, kind, name, limit)` | Events for a specific resource |
| `GetSnapshot(cluster, at)` | Snapshot closest to a given time |
| `GetLatestSnapshot(cluster)` | Most recent snapshot |
| `ListSnapshots(cluster, limit)` | Available snapshot timestamps |
| `CompareSnapshots(before, after)` | Diff between two snapshots |

## Troubleshooting

**History collection is disabled at startup:**
Check the application log (`~/.local/state/kubecat/kubecat.log`) for `"failed to open history database"`. This usually means a permissions issue on the data directory or a corrupted database file. Delete `~/.local/share/kubecat/history.db` and restart to rebuild.

**Events are not appearing in the Timeline:**
Verify the cluster is connected and the EventCollector is running by checking the log for `"watchCluster"` entries. If you see repeated backoff messages, the cluster's API server may be rate-limiting watch requests.

**Snapshots are very large:**
Large clusters (thousands of resources) produce large snapshot blobs. Increase `cleanupInterval` and reduce `retentionDays` to control disk usage. The default 7-day retention is appropriate for most users.

## Related Documents

- [Storage](storage.md) — SQLite schema details
- [Data Flow: History Collection](data-flow.md#4-event-history-collection-flow-background)
- [User Guide: Time Travel](../user-guide/time-travel.md)
- [Operations: Data Retention](../operations/data-retention.md)
