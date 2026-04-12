# GitOps Integration

Kubecat automatically detects ArgoCD and Flux installations and provides a unified view of application sync and health status.

---

## Auto-Detection

When you connect to a cluster, Kubecat probes for:

- **ArgoCD** — checks for `argoproj.io/v1alpha1` Application resources
- **Flux** — checks for `kustomize.toolkit.fluxcd.io/v1` Kustomization and `helm.toolkit.fluxcd.io/v2` HelmRelease resources

If neither is found, the GitOps view shows "No GitOps provider detected".

---

## ArgoCD View

For each ArgoCD Application, Kubecat shows:

| Field | Description |
|-------|-------------|
| Name | Application name |
| Sync status | Synced / OutOfSync / Unknown |
| Health status | Healthy / Progressing / Degraded / Suspended / Missing / Unknown |
| Destination | Cluster + namespace the app deploys to |
| Source | Git repository URL and path |
| Last sync | Timestamp of last sync operation |

Color coding: green = synced + healthy, amber = progressing or out-of-sync, red = degraded.

---

## Flux View

For each Flux Kustomization or HelmRelease, Kubecat shows:

| Field | Description |
|-------|-------------|
| Name | Resource name |
| Kind | Kustomization or HelmRelease |
| Ready | True / False |
| Status message | Last reconciliation message |
| Last reconciled | Timestamp |

---

## Triggering a Sync

If the cluster is not in read-only mode, click **Sync Now** next to any ArgoCD application to trigger an immediate sync. This calls the ArgoCD Application resource's sync operation via the Kubernetes API — no ArgoCD CLI or server access required.

Flux reconciliation triggers are not yet supported (planned for a future release).

---

## Limitations

- Kubecat reads GitOps resources via the Kubernetes API. It does not connect to the ArgoCD API server or Flux notification server directly.
- Multi-source ArgoCD applications show the first source only.
- ArgoCD ApplicationSets are listed but individual applications within the set are not expanded.
