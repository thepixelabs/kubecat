# Storage

> Applies to: Kubecat v0.x (2026). Last updated: 2026-04-07.

This document describes Kubecat's embedded SQLite database: its location, schema, WAL configuration, migration system, and data retention.

**Audience:** Developers and operators who need to understand or query the history database.

## Database Location

```
~/.local/share/kubecat/history.db
```

The path is computed by `storage.Open()` using `config.StateDir()` which follows XDG Base Directory conventions. On macOS, this is typically `~/Library/Application Support/kubecat/history.db` when the XDG override is not set.

## Configuration

WAL (Write-Ahead Logging) mode is enabled at database open time. WAL allows concurrent reads during writes, which is important because the `EventCollector`, `Snapshotter`, and the frontend (via App bridge reads) all access the database simultaneously.

```sql
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;
```

## Schema

The schema is managed by sequential migrations in `internal/storage/schema.go`. Migrations are applied in version order at startup; each migration runs in a transaction.

### Migration 1

#### `snapshots` — Periodic cluster state captures

```sql
CREATE TABLE snapshots (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    cluster    TEXT    NOT NULL,
    timestamp  DATETIME NOT NULL,
    data       BLOB    NOT NULL,    -- zlib-compressed JSON SnapshotData
    compressed INTEGER DEFAULT 1,
    UNIQUE(cluster, timestamp)
);
```

Indexes: `idx_snapshots_cluster`, `idx_snapshots_timestamp`

The `data` column stores a zlib-compressed JSON encoding of `storage.SnapshotData`:

```go
type SnapshotData struct {
    Cluster    string                       `json:"cluster"`
    Timestamp  time.Time                    `json:"timestamp"`
    Namespaces []string                     `json:"namespaces"`
    Resources  map[string][]ResourceInfo    `json:"resources"`
    // key: resource kind (e.g. "pods"), value: list of resources
}

type ResourceInfo struct {
    Name            string            `json:"name"`
    Namespace       string            `json:"namespace"`
    ResourceVersion string            `json:"resourceVersion"`
    Labels          map[string]string `json:"labels"`
    Status          string            `json:"status"`
}
```

Resource kinds captured per snapshot (configurable, default):
`pods`, `deployments`, `services`, `configmaps`, `statefulsets`, `daemonsets`, `jobs`, `cronjobs`

#### `events` — Kubernetes events

```sql
CREATE TABLE events (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    cluster          TEXT    NOT NULL,
    namespace        TEXT,
    kind             TEXT    NOT NULL,   -- involvedObject.kind
    name             TEXT    NOT NULL,   -- involvedObject.name
    type             TEXT    NOT NULL,   -- "Normal" or "Warning"
    reason           TEXT,
    message          TEXT,
    first_seen       DATETIME,
    last_seen        DATETIME NOT NULL,
    count            INTEGER DEFAULT 1,
    source_component TEXT,
    source_host      TEXT
);
```

Indexes: `idx_events_cluster`, `idx_events_namespace`, `idx_events_kind_name`, `idx_events_last_seen`, `idx_events_reason`

#### `correlations` — Links between related events

```sql
CREATE TABLE correlations (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    source_event_id INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    target_event_id INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    confidence      REAL    NOT NULL,   -- 0.0 to 1.0
    relationship    TEXT    NOT NULL,   -- e.g. "scaling_caused", "managed_by"
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

Indexes: `idx_correlations_source`, `idx_correlations_target`

Cascade deletes ensure orphaned correlation rows are cleaned up automatically when events are purged during retention cleanup.

Built-in correlation rules (defined in `internal/history/correlator.go`):

| Rule name | Source → Target | Time window | Confidence |
|-----------|----------------|-------------|-----------|
| `configmap-to-pod` | ConfigMap → Pod | 5 min | 0.70 |
| `deployment-scaling` | Deployment (ScalingReplicaSet) → Pod | 2 min | 0.95 |
| `replicaset-to-pod` | ReplicaSet → Pod | 2 min | 0.90 |
| `job-to-pod` | Job → Pod | 2 min | varies |

#### `resources` — Resource version tracking for change detection

```sql
CREATE TABLE resources (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    cluster          TEXT    NOT NULL,
    namespace        TEXT,
    kind             TEXT    NOT NULL,
    name             TEXT    NOT NULL,
    resource_version TEXT    NOT NULL,
    data             BLOB,
    first_seen       DATETIME NOT NULL,
    last_seen        DATETIME NOT NULL,
    UNIQUE(cluster, namespace, kind, name)
);
```

Indexes: `idx_resources_cluster`, `idx_resources_kind`

### Migration 2

#### `settings` — User preferences

```sql
CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

Simple key-value store for persistent UI preferences (e.g., last selected namespace, theme preference).

## Repository Layer

Each table has a corresponding repository in `internal/storage/`:

| Repository | Table | Key operations |
|-----------|-------|---------------|
| `EventRepository` | `events` | `Save`, `List(EventFilter)`, `DeleteOlderThan` |
| `SnapshotRepository` | `snapshots` | `Save`, `GetAt(cluster, time)`, `GetLatest`, `ListTimestamps`, `DeleteOlderThan` |
| `CorrelationRepository` | `correlations` | `Save`, `ListByEvent` |

`EventFilter` supports filtering by cluster, namespace, kind, name, time range (`Since`/`Until`), and row limit.

## Data Retention

The `RetentionManager` (`internal/storage/retention.go`) runs as a background goroutine started during `App.startup()`. It periodically deletes rows older than the configured retention period.

Default retention: **7 days** for both events and snapshots.

Configure in `~/.config/kubecat/config.yaml`:

```yaml
kubecat:
  storage:
    retentionDays: 30    # keep 30 days of history
    cleanupInterval: 6h  # run cleanup every 6 hours
```

Cascade deletes on the `correlations` table mean that purging old events also removes their correlation links automatically.

## Manual Inspection

You can inspect the database directly with the `sqlite3` CLI:

```bash
sqlite3 ~/.local/share/kubecat/history.db

# Count events per cluster
SELECT cluster, count(*) FROM events GROUP BY cluster;

# Recent Warning events
SELECT last_seen, cluster, namespace, kind, name, reason, message
FROM events
WHERE type = 'Warning'
ORDER BY last_seen DESC
LIMIT 20;

# List available snapshot timestamps for a cluster
SELECT timestamp FROM snapshots
WHERE cluster = 'production'
ORDER BY timestamp DESC
LIMIT 10;
```

## Database Size

Typical growth rates at default settings:
- **Events**: ~100–500 events/day per active cluster, ~200 bytes each uncompressed. Expect 50–250 KiB/day per cluster, bounded to ~7 days.
- **Snapshots**: compressed JSON per 5-minute snapshot. Size depends on cluster resource count. A 100-resource cluster typically produces 5–20 KiB per snapshot, or 2–8 MiB/day, bounded to 7 days.

If disk space is a concern, increase `cleanupInterval` or reduce `retentionDays`.

## Related Documents

- [History](history.md) — event collection, snapshotting, and correlation
- [Data Flow](data-flow.md#4-event-history-collection-flow-background)
- [Config Reference](../reference/config-reference.md)
- [Operations: Data Retention](../operations/data-retention.md)
