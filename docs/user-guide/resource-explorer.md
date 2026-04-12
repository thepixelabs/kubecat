# Resource Explorer

The Resource Explorer is the primary view for browsing and managing Kubernetes resources.

---

## Navigation

Use the **Sidebar** to select a resource kind:

- Workloads: Pods, Deployments, StatefulSets, DaemonSets, Jobs, CronJobs
- Networking: Services, Ingresses, NetworkPolicies
- Config: ConfigMaps, Secrets (names only — values hidden by default)
- Storage: PersistentVolumeClaims, PersistentVolumes, StorageClasses
- RBAC: ClusterRoles, Roles, ClusterRoleBindings, RoleBindings
- Custom: any CRD installed on the cluster

Use the **namespace selector** in the top bar to filter by namespace or select "All Namespaces".

---

## Resource List

The resource list shows:

- Resource name, namespace, age
- Status (color-coded): Running, Pending, Failed, Unknown
- Key metadata: restart count for pods, replica count for deployments, etc.

Click any resource row to open the **detail panel** on the right.

---

## Resource Detail Panel

The detail panel shows:

- **Summary** — key fields (status, images, resource requests/limits)
- **Labels & Annotations** — all metadata key-value pairs
- **Events** — recent Kubernetes events for this resource
- **YAML** — full resource YAML (read-only by default)
- **Logs** — for Pod resources, a live log stream tab

---

## Editing Resources

If the cluster is not in read-only mode, you can:

- **Edit YAML** — modify the resource YAML inline and click **Apply**
- **Delete** — remove the resource (with confirmation dialog)
- **Scale** — for Deployments/StatefulSets, adjust replica count

All mutations are logged to the application log with the resource kind, name, namespace, and cluster.

---

## Searching and Filtering

- **Name filter** — type in the search box above the resource list to filter by name (substring match)
- **Label filter** — click the filter icon to add label selector expressions (e.g., `app=nginx`)
- **Status filter** — filter to show only unhealthy resources (Pending, Failed, CrashLoopBackOff)

---

## Secrets

Secret resource **names** are listed normally. Secret **values** (`.data` fields) are hidden by default and shown only when you explicitly click "Reveal". Secret values are never logged or sent to AI providers.
