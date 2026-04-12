# Cluster Diff

The Cluster Diff view lets you compare:

- **Two clusters** — side-by-side comparison of what's in cluster A vs. cluster B
- **Two snapshots** — what changed in a single cluster between two points in time

---

## Comparing Two Clusters

1. Click **Diff** in the sidebar.
2. Select **Cluster vs. Cluster** mode.
3. Choose the left cluster and right cluster from the dropdowns.
4. Optionally filter to a specific namespace.
5. Click **Run Diff**.

Kubecat fetches the current state of both clusters and computes the diff. This operation runs live API calls — both clusters must be connected.

---

## Comparing Snapshots

1. Click **Diff** in the sidebar.
2. Select **Snapshot vs. Snapshot** mode.
3. Choose the cluster and two snapshot timestamps.
4. Click **Run Diff**.

This uses locally stored snapshots — no live API calls required. Works even if the cluster is currently offline.

---

## Reading the Diff Results

The diff result shows:

| Category | Color | Meaning |
|----------|-------|---------|
| Added | Green | Resource exists in right side but not left |
| Removed | Red | Resource exists in left side but not right |
| Changed | Yellow | Resource exists in both but has differences |
| Unchanged | Gray | Resource is identical on both sides |

Summary counts at the top: `+12 added / -3 removed / ~7 changed`.

---

## Viewing Changed Resources

Click any **Changed** resource to open the detail view:

- Left panel: YAML from the left side (older snapshot or left cluster)
- Right panel: YAML from the right side (newer snapshot or right cluster)
- Changed fields are highlighted inline

Use the **Copy diff** button to copy a unified diff to your clipboard.

---

## Filtering the Diff

- Filter by resource kind (show only Pod changes, etc.)
- Filter by namespace
- Filter to show only changes of a specific type (e.g., only Added)
- Text search across resource names
