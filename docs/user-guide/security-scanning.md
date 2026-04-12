# Security Scanning

Kubecat's built-in security scanner analyzes your cluster for common misconfigurations and policy violations without requiring external tools.

---

## Running a Scan

Click **Security** in the sidebar, then click **Scan Now**. The scanner runs three passes:

1. **RBAC Analysis** — inspects ClusterRoleBindings and RoleBindings
2. **Runtime Scanning** — inspects all Pods across the selected namespace (or all namespaces)
3. **Network Policy Analysis** — checks each namespace for network policy coverage

---

## Security Score

After scanning, Kubecat displays a **security score** (0–100) and letter grade (A–F):

| Grade | Score | Meaning |
|-------|-------|---------|
| A | 90–100 | Very few or no issues |
| B | 80–89 | Minor issues |
| C | 70–79 | Moderate issues that should be addressed |
| D | 60–69 | Significant issues |
| F | 0–59 | Critical issues requiring immediate attention |

Scoring deductions: Critical = -15 pts, High = -10 pts, Medium = -5 pts, Low = -2 pts.

---

## RBAC Analysis

The scanner flags:

- **Dangerous permissions** — subjects with `*` verbs on `*` resources (cluster-admin equivalent)
- **Secrets access** — subjects that can `get` or `list` Secrets
- **Wildcard permissions** — any `*` in verbs or resources

Each finding shows the subject name and kind (User, Group, ServiceAccount), the binding name, and recommended remediation.

---

## Runtime Security

The scanner checks every Pod for:

| Check | Severity |
|-------|---------|
| Container running as root (UID 0) | High |
| Privileged container (`privileged: true`) | Critical |
| `hostNetwork: true` | High |
| `hostPID: true` | High |
| `hostIPC: true` | High |
| No read-only root filesystem | Low |
| `allowPrivilegeEscalation: true` | Medium |

---

## Network Policy Analysis

The scanner reports any namespace that has no NetworkPolicy resources. Namespaces without network policies allow all pod-to-pod traffic — a lateral movement risk.

System namespaces (`kube-system`, `kube-public`, `kube-node-lease`) are excluded from this check.

---

## Policy Engine Detection

If Gatekeeper or Kyverno is installed, Kubecat also shows:

- Total policies installed
- Policies in `deny`/`enforce` vs. `warn`/`audit` mode
- Current violation counts (Gatekeeper)

---

## Filtering Results

Use the filter bar above the issue list to:

- Filter by severity (Critical, High, Medium, Low)
- Filter by category (RBAC, Runtime, Network)
- Search by resource name or namespace

---

## RBAC Manifests

For reference RBAC manifests to deploy Kubecat service accounts with appropriate permissions, see `docs/rbac/`.
