# Time-Travel Debugging

> **Status**: ✅ Implemented

Kubecat provides historical state analysis and event correlation for debugging.

## Vision

Understand what changed and when:

```
⎈ Event Timeline

[14:35] ─────────────────────────────────────────────────
        ConfigMap 'app-config' updated by user/admin

[14:36] ─────────────────────────────────────────────────
        Deployment 'api-server' rollout triggered
        Pods: api-server-abc → api-server-xyz

[14:37] ─────────────────────────────────────────────────
        ⚠ Pod 'api-server-xyz' CrashLoopBackOff
        Reason: Container exited with code 1

[14:38] ─────────────────────────────────────────────────
        🔗 CORRELATED: Config change → Pod crash
```

## Features

### Event Timeline (✅ Implemented)

Chronological view of cluster events:

```
Filter: namespace=default, severity=warning+

14:37:22  ⚠ Pod/api-server-xyz     CrashLoopBackOff
14:37:15  ⚠ Pod/api-server-xyz     BackOff
14:36:45  ℹ Pod/api-server-xyz     Created
14:36:44  ℹ Pod/api-server-abc     Deleted
14:36:44  ℹ Deployment/api-server  ScalingReplicaSet
14:35:30  ℹ ConfigMap/app-config   Updated

← Previous Hour    Now →
```

### State Snapshots (✅ Implemented)

View cluster state at any point in time:

```
Snapshot: 2024-01-15 14:30:00

Namespace: default
────────────────────────────────────────────────────────
Pods:           12 (vs 15 now)
Deployments:     5 (vs 5 now)
Services:        8 (vs 8 now)

Changed since snapshot:
  + Pod/api-server-xyz (created)
  + Pod/api-server-def (created)
  - Pod/api-server-abc (deleted)
  ~ ConfigMap/app-config (modified)
```

### What Changed Analysis (✅ Implemented)

Quickly find changes in a time window:

```
> What changed in the last hour?

Changes from 13:40 to 14:40
────────────────────────────────────────────────────────
ConfigMaps:
  ~ app-config        Modified by user/admin

Deployments:
  ~ api-server        Image updated, replicas changed

Pods:
  + api-server-xyz    Created (Running)
  + api-server-def    Created (Running)
  - api-server-abc    Deleted
  - api-server-123    Deleted

Total: 1 configmap, 1 deployment, 4 pods changed
```

### Event Correlation (✅ Implemented)

Automatic linking of related events:

```
Incident Analysis: Pod CrashLoopBackOff

Timeline:
────────────────────────────────────────────────────────
14:35:30  ConfigMap/app-config updated
          Changed keys: DATABASE_URL, REDIS_URL

14:36:44  Deployment/api-server triggered rollout
          Reason: ConfigMap change detected

14:37:22  Pod/api-server-xyz failed to start
          Exit code: 1
          Last log: "Error: Invalid REDIS_URL format"

Root Cause: Invalid REDIS_URL in ConfigMap update
Confidence: High (95%)

Suggested Fix:
  Revert ConfigMap to previous version:
  kubectl rollout undo configmap/app-config
```

### Diff View

Compare resources between time points:

```
Diff: ConfigMap/app-config

Before (14:35:00)              After (14:35:30)
────────────────────────────────────────────────────────
DATABASE_URL: postgres://...   DATABASE_URL: postgres://...
REDIS_URL: redis://prod:6379   REDIS_URL: redis://prod:6379:  ← syntax error
LOG_LEVEL: info                LOG_LEVEL: debug
```

## Storage Architecture (✅ Implemented)

### Embedded SQLite

State is stored in pure-Go SQLite:

```
~/.local/state/kubecat/history.db

Tables:
  - snapshots      (cluster state at intervals)
  - events         (Kubernetes events)
  - correlations   (linked events)
  - resources      (resource versions)
```

### Retention Policy

```yaml
# ~/.config/kubecat/config.yaml
kubecat:
  history:
    enabled: true
    # How often to take snapshots
    snapshotInterval: 5m
    # How long to keep data
    retention: 7d
    # Cleanup interval (runs hourly)
    cleanupInterval: 1h
```

### Scalability & Stability

Kubecat is designed to handle years of continuous operation without database bloat or crashes:

1. **Rolling Window Retention**: The application enforces a strict 7-day rolling window (default). An automatic background process runs every hour to prune events older than the retention period. This ensures the database size remains stable over time and never grows indefinitely, regardless of how long the app runs.
2. **WAL Mode**: The underlying SQLite database uses **Write-Ahead Logging (WAL)** mode. This provides:
   - significantly better concurrency (readers don't block writers)
   - robustness against crashes
   - improved write performance
3. **Optimized Indexing**: The `history.db` schema includes dedicated indices on `last_seen`, `cluster`, and `timestamp` columns, ensuring that queries efficiently filter the 7-day window without full table scans, maintaining UI responsiveness even with high event volume.

### Schema

```sql
CREATE TABLE snapshots (
    id INTEGER PRIMARY KEY,
    cluster TEXT NOT NULL,
    timestamp DATETIME NOT NULL,
    data BLOB NOT NULL  -- compressed JSON
);

CREATE TABLE events (
    id INTEGER PRIMARY KEY,
    cluster TEXT NOT NULL,
    namespace TEXT,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    reason TEXT,
    message TEXT,
    timestamp DATETIME NOT NULL,
    first_seen DATETIME,
    last_seen DATETIME,
    count INTEGER DEFAULT 1
);

CREATE TABLE correlations (
    id INTEGER PRIMARY KEY,
    source_event INTEGER REFERENCES events(id),
    target_event INTEGER REFERENCES events(id),
    confidence REAL,
    relationship TEXT
);
```

## Correlation Engine

### Event Linking Rules

```go
type CorrelationRule struct {
    Name       string
    Source     EventMatcher
    Target     EventMatcher
    TimeWindow time.Duration
    Confidence float64
}

var DefaultRules = []CorrelationRule{
    {
        Name:       "configmap-to-pod",
        Source:     Match{Kind: "ConfigMap", Reason: "Updated"},
        Target:     Match{Kind: "Pod", Reason: "Started,Failed"},
        TimeWindow: 5 * time.Minute,
        Confidence: 0.8,
    },
    {
        Name:       "deployment-to-pod",
        Source:     Match{Kind: "Deployment", Reason: "ScalingReplicaSet"},
        Target:     Match{Kind: "Pod", Reason: "*"},
        TimeWindow: 2 * time.Minute,
        Confidence: 0.95,
    },
}
```

### Confidence Scoring

Correlations are scored based on:

- Time proximity
- Namespace match
- Label overlap
- Ownership references
- Historical patterns

```
Confidence Levels:
  95%+ : Definite (ownership chain)
  80%+ : Likely (same namespace, close timing)
  60%+ : Possible (related resources)
  <60% : Suggested (pattern match)
```
