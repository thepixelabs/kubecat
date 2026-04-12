# Time Travel

Kubecat captures periodic snapshots of your cluster state and stores them locally. The Time Travel feature lets you browse historical state and understand what changed over time.

---

## How Snapshots Work

When you connect to a cluster, Kubecat automatically starts taking snapshots every **5 minutes**. Each snapshot captures the current state of:

- Pods, Deployments, StatefulSets, DaemonSets, Jobs, CronJobs
- Services, ConfigMaps

Snapshots are compressed with zlib and stored in the local SQLite database at `~/.local/state/kubecat/history.db`.

Default retention: **7 days** (configurable — see `docs/operations/data-retention.md`).

---

## Event Timeline

The **Timeline** view shows a chronological feed of Kubernetes events ingested since you connected:

- Warning events (highlighted in amber)
- Normal events (shown in muted style)
- Correlated events grouped by the resource they affect

Use the timeline to answer "what happened to this pod between 2pm and 3pm?"

Filter the timeline by:
- Namespace
- Resource kind
- Severity (Warning / Normal)
- Time range

---

## Viewing Historical State

1. Click **Timeline** in the sidebar.
2. Click any snapshot timestamp in the snapshot list on the right.
3. The resource list switches to show the cluster state captured at that snapshot.
4. Resources that existed then but not now are shown with a "deleted" badge. Resources that didn't exist then are shown with a "new" badge when you return to current state.

---

## Taking a Manual Snapshot

Click **Take Snapshot** in the Timeline toolbar to capture the current state immediately, outside the automatic schedule.

---

## Comparing Snapshots

Use the **Cluster Diff** view to compare two snapshots. See [Cluster Diff](cluster-diff.md) for details.

---

## Limitations

- Snapshots only include resource kinds listed above. CRDs, RBAC resources, and Secrets are not snapshotted.
- Very large clusters (10,000+ resources) may cause snapshot operations to take longer than the 5-minute interval, resulting in overlapping snapshots. Future improvement: adaptive snapshot scheduling.
- Events are only collected while Kubecat is running. Events that occurred while the app was closed are not captured retroactively (Kubernetes only retains recent events in the API server).
