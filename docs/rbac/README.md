# Kubecat RBAC Reference

This directory contains reference Kubernetes RBAC manifests for running Kubecat with least-privilege access. Apply the role that matches the features you need.

---

## Manifests

| File | Kind | Name | Use case |
|------|------|------|---------|
| `viewer-clusterrole.yaml` | `ClusterRole` | `kubecat-viewer` | Read-only browsing, security scanning, AI queries (no secret values) |
| `operator-clusterrole.yaml` | `ClusterRole` | `kubecat-operator` | Full feature access including terminal, port-forward, delete, apply, secret viewer |
| `namespace-scoped-role.yaml` | `Role` | `kubecat-namespace-viewer` | Read access restricted to one namespace; cluster-wide features unavailable |

---

## Which Features Require Which Permissions

| Feature | Viewer | Operator | Notes |
|---------|--------|----------|-------|
| Resource Explorer (browse/search) | Yes | Yes | `get`, `list`, `watch` on workload resources |
| Resource YAML view | Yes | Yes | `get` on the resource kind |
| Cluster Visualizer | Yes | Yes | `list` on pods, services, ingresses, deployments, replicasets, statefulsets, daemonsets |
| Pod logs | Yes | Yes | `get` on `pods/log` |
| Timeline / event history | Yes | Yes | `list`, `watch` on `events` |
| Security Scanner | Yes | Yes | `list` on RBAC resources, network policies, pod specs |
| RBAC Analysis | Yes | Yes | `get`, `list` on `clusterroles`, `clusterrolebindings`, `roles`, `rolebindings` |
| AI queries (resource context) | Yes | Yes | Same as Resource Explorer; no extra permissions |
| Node metrics display | Yes | Yes | `get`, `list` on `metrics.k8s.io/nodes` (optional — degrades gracefully) |
| GitOps status (ArgoCD / Flux) | Yes | Yes | `get`, `list`, `watch` on `argoproj.io` and Flux CRDs |
| Secret metadata listing | Yes | Yes | `list` on `secrets` — names and labels only, no values |
| **Secret Viewer (values)** | **No** | **Yes** | Requires `get` on `secrets`; decodes `.data` fields on explicit user request |
| **Resource delete** | **No** | **Yes** | `delete` on the resource kind |
| **Apply YAML** | **No** | **Yes** | `create`, `update`, `patch` on the resource kind |
| **Port Forward** | **No** | **Yes** | `create` on `pods/portforward` subresource |
| **Pod Terminal (exec)** | **No** | **Yes** | `create` on `pods/exec` subresource |
| **GitOps sync trigger** | **No** | **Yes** | `patch` on `argoproj.io/applications` |

The local terminal (built-in shell in the Kubecat UI) runs as your local user process and does not require any Kubernetes RBAC permissions.

---

## Choosing the Right Role

**Production clusters (recommended):** Apply `viewer-clusterrole.yaml` and set `readOnly: true` on that cluster in your Kubecat config. The UI blocks all mutating actions at the application layer as well.

**Development or staging clusters you manage:** Apply `operator-clusterrole.yaml` when you need to delete resources, exec into pods, use port-forward, or apply YAML from within Kubecat.

**Multi-tenant clusters where you manage one namespace:** Apply `namespace-scoped-role.yaml` to your namespace. Cluster-wide features (node metrics, full RBAC analysis, cross-namespace security scanning) are unavailable with this role.

---

## Applying the Viewer Role

```bash
# Apply the ClusterRole
kubectl apply -f viewer-clusterrole.yaml

# Create a dedicated ServiceAccount for Kubecat (recommended over using your personal kubeconfig)
kubectl create namespace kubecat
kubectl create serviceaccount kubecat -n kubecat

# Bind the ClusterRole to the ServiceAccount
kubectl create clusterrolebinding kubecat-viewer \
  --clusterrole=kubecat-viewer \
  --serviceaccount=kubecat:kubecat
```

---

## Applying the Operator Role

```bash
kubectl apply -f operator-clusterrole.yaml

kubectl create namespace kubecat
kubectl create serviceaccount kubecat -n kubecat

kubectl create clusterrolebinding kubecat-operator \
  --clusterrole=kubecat-operator \
  --serviceaccount=kubecat:kubecat
```

---

## Applying the Namespace-Scoped Role

```bash
# Apply the Role into the namespace you want Kubecat to access
kubectl apply -f namespace-scoped-role.yaml -n my-team

# Create a ServiceAccount in that namespace
kubectl create serviceaccount kubecat -n my-team

# Bind the Role
kubectl create rolebinding kubecat-viewer \
  --role=kubecat-namespace-viewer \
  --serviceaccount=my-team:kubecat \
  --namespace=my-team
```

---

## Creating a Dedicated Kubeconfig for Kubecat

Using a dedicated kubeconfig with a ServiceAccount token scopes Kubecat's access to exactly the permissions above and prevents it from inheriting your personal cluster-admin rights.

### Step 1 — Get a ServiceAccount token

Kubernetes 1.24+ no longer auto-creates long-lived tokens. Create one explicitly:

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: kubecat-token
  namespace: kubecat
  annotations:
    kubernetes.io/service-account.name: kubecat
type: kubernetes.io/service-account-token
EOF

# Wait for the token to be populated (a few seconds)
kubectl -n kubecat get secret kubecat-token -o jsonpath='{.data.token}' | base64 -d
```

### Step 2 — Build the kubeconfig

```bash
# Set variables
CLUSTER_NAME="my-cluster"
SERVER="https://your-api-server:6443"
NAMESPACE="kubecat"
SECRET_NAME="kubecat-token"

# Extract values
CA=$(kubectl -n $NAMESPACE get secret $SECRET_NAME -o jsonpath='{.data.ca\.crt}')
TOKEN=$(kubectl -n $NAMESPACE get secret $SECRET_NAME -o jsonpath='{.data.token}' | base64 -d)

# Write kubeconfig
cat > ~/.kube/kubecat-config.yaml <<EOF
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: ${CA}
    server: ${SERVER}
  name: ${CLUSTER_NAME}
contexts:
- context:
    cluster: ${CLUSTER_NAME}
    namespace: default
    user: kubecat
  name: kubecat@${CLUSTER_NAME}
current-context: kubecat@${CLUSTER_NAME}
users:
- name: kubecat
  user:
    token: ${TOKEN}
EOF

chmod 600 ~/.kube/kubecat-config.yaml
```

### Step 3 — Point Kubecat at the dedicated kubeconfig

Set `KUBECONFIG=~/.kube/kubecat-config.yaml` in your shell before starting Kubecat, or configure it in `~/.config/kubecat/config.yaml`:

```yaml
kubeconfig: ~/.kube/kubecat-config.yaml
```

---

## Principle of Least Privilege Notes

- **Secret values:** The viewer role grants only `list` on secrets (names and labels visible, no `.data`). The operator role adds `get` which is required by the Secret Viewer feature. If you use the operator role but do not want secret values to be accessible, remove `get` from the secrets rule.
- **Nodes:** Node management verbs (`cordon`, `drain`, `taint`, `delete`) are never granted. Both roles grant only `get`, `list`, `watch` on nodes.
- **RBAC resources:** Both roles grant read-only access to RBAC resources. Kubecat does not create or modify roles or bindings.
- **Pod exec:** Only the operator role grants `pods/exec`. Remove this subresource entry from the operator role if you want to prevent exec access while keeping other operator capabilities.
- **Port-forward:** Only the operator role grants `pods/portforward`. Port forwards open a tunnel from localhost to a pod port — treat this as equivalent to network access to the pod.
- **Persistent data:** Neither role grants `delete` on `persistentvolumes` or `storageclasses` to prevent accidental data loss.
