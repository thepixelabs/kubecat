# Cluster Management

## Multiple Clusters

Kubecat supports connecting to multiple clusters simultaneously. Each connected cluster maintains its own:

- Event collection stream
- Snapshot history
- Connection health state

Use the cluster selector in the Navbar to switch the active cluster. The sidebar and all views reflect the active cluster. Multi-cluster comparisons are available in the Cluster Diff and AI query views.

---

## Adding Clusters

Kubecat automatically discovers all contexts in your `~/.kube/config`. To add a new cluster:

1. Add the context to your kubeconfig with `kubectl config set-context` or via your cloud provider's CLI (e.g., `aws eks update-kubeconfig`, `gcloud container clusters get-credentials`).
2. Click **Refresh Contexts** in the cluster selector dropdown.
3. Select the new context and click **Connect**.

---

## Disconnecting

Click the cluster name in the Navbar → **Disconnect**. Kubecat stops event collection for that cluster. Historical data (events, snapshots) is retained in the local database.

---

## Read-Only Mode

Kubecat supports a read-only mode that blocks all cluster mutation operations. This is strongly recommended when connecting to production clusters.

### Global read-only

```yaml
kubecat:
  readOnly: true
```

### Per-cluster read-only

```yaml
kubecat:
  clusters:
    - context: prod-us-east-1
      readOnly: true
    - context: dev-local
      readOnly: false
```

When read-only mode is active, the following operations are blocked:
- Delete resource
- Apply resource
- GitOps sync trigger
- Terminal (interactive shell with cluster access)
- Port forwarding (can mutate via shell in forwarded connection)

Read-only operations always work: list resources, view logs, AI queries, security scanning, time travel, cluster diff.

---

## Cluster-Specific Namespace Default

You can set a default namespace per cluster:

```yaml
kubecat:
  clusters:
    - context: my-cluster
      namespace: my-team-namespace
```

When connecting to this cluster, the namespace selector in the resource explorer defaults to `my-team-namespace`.

---

## Connection Health

The Navbar cluster selector shows a color-coded status indicator:

| Color | Meaning |
|-------|---------|
| Green | Connected and healthy |
| Yellow | Degraded (slow response) |
| Red | Disconnected |

Kubecat automatically reconnects with exponential backoff when a cluster becomes unreachable. The reconnect interval starts at 1 second and doubles up to a maximum of 60 seconds.
