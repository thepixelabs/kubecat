# Data Retention

Kubecat stores Kubernetes event history and cluster snapshots in a local SQLite database. Without retention management, this database grows unboundedly. The `RetentionManager` (part of `internal/storage`) provides automated cleanup.

---

## What Is Stored

| Table | Content | Default retention |
|-------|---------|------------------|
| `events` | Kubernetes Warning and Normal events | 7 days |
| `snapshots` | Full cluster state snapshots (compressed) | 7 days |
| `correlations` | Links between related events | Deleted with parent events (CASCADE) |
| `resources` | Resource version tracking for change detection | 7 days |
| `settings` | Key-value user preferences | Permanent (no TTL) |

---

## Retention Configuration

Set in `~/.config/kubecat/config.yaml`:

```yaml
kubecat:
  retention:
    retentionDays: 7          # Days to keep events and snapshots
    maxDatabaseSizeMB: 500    # Warn and aggressively prune if exceeded
```

---

## Cleanup Schedule

The retention manager runs:

1. **On application startup** — initial cleanup of anything older than `retentionDays`.
2. **Every hour** — scheduled background cleanup.

Each cleanup run:
1. Deletes events older than `retentionDays`.
2. Deletes snapshots older than `retentionDays`.
3. If more than 1,000 rows were deleted, runs `PRAGMA wal_checkpoint(TRUNCATE)` followed by `VACUUM` to reclaim disk space.
4. Checks current database file size. If greater than `maxDatabaseSizeMB`, logs a `WARN` and triggers an immediate aggressive prune (reducing retention to 3 days until size is back under limit).

---

## Manual Cleanup

To manually remove all history data:

```bash
# Stop Kubecat first
rm ~/.local/state/kubecat/history.db
```

Kubecat will recreate the database with a fresh schema on next startup.

To compact without data loss:

```bash
sqlite3 ~/.local/state/kubecat/history.db "VACUUM;"
```

---

## Database Size Estimation

Approximate disk usage per day of data:

| Data type | Size per day |
|-----------|-------------|
| Events (typical cluster) | 5–20 MB |
| Snapshots (5-minute interval) | 50–200 MB |

For a busy cluster with the default 5-minute snapshot interval and 7-day retention, expect 350–1,400 MB of snapshot data. Snapshots are zlib-compressed before storage, which typically achieves 70–80% compression on Kubernetes YAML.

Reduce storage usage by increasing the snapshot interval in the snapshotter config (configurable in future UI release).
