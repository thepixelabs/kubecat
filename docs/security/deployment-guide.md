# Deployment Guide

> Applies to: Kubecat v0.x (2026). Last updated: 2026-04-07.

This guide describes recommended deployment patterns for regulated and enterprise environments. It addresses the most common enterprise requirements: data isolation, write protection, audit logging, and compliance documentation.

**Audience:** IT administrators, security engineers, and DevOps leads deploying Kubecat in enterprise or regulated environments.

## Deployment Model

Kubecat is a **single-user desktop application**. It installs and runs on a developer or operator's machine. There is no server deployment, no central cluster required, and no infrastructure to manage beyond the binary itself.

Each user installs Kubecat locally and connects to Kubernetes clusters using their own kubeconfig credentials. Access control to clusters is governed entirely by Kubernetes RBAC — Kubecat does not add or modify RBAC.

## Deployment Patterns

### Pattern 1: Read-Only Observer (Recommended for Production Access)

Appropriate for: Operators who need visibility into production clusters without write access.

```yaml
# ~/.config/kubecat/config.yaml
kubecat:
  readOnly: true
  ai:
    provider: ollama            # local AI, no data egress
    endpoint: http://localhost:11434
  telemetry:
    enabled: false
  agentGuardrails:
    blockDestructive: true
    protectedNamespaces:
      - kube-system
      - kube-public
      - kube-node-lease
      - production
      - prod
```

What this configuration does:
- `readOnly: true` — blocks all write operations (apply, delete, scale, port-forward with write access) at the App bridge layer, before any API call.
- `provider: ollama` — all AI queries stay local; no cluster data leaves the host.
- `blockDestructive: true` — agent cannot call delete, restart, or drain tools.
- Extended `protectedNamespaces` — agent cannot write to production namespaces even if `blockDestructive` is relaxed in future.

Pair with a read-only `ClusterRole` for the user's kubeconfig credentials:

```yaml
# docs/rbac/viewer-clusterrole.yaml (reference; apply separately)
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubecat-viewer
rules:
  - apiGroups: ["", "apps", "batch", "networking.k8s.io", "rbac.authorization.k8s.io"]
    resources: ["*"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["argoproj.io", "helm.toolkit.fluxcd.io", "kustomize.toolkit.fluxcd.io"]
    resources: ["*"]
    verbs: ["get", "list", "watch"]
```

---

### Pattern 2: Air-Gapped / No External Network

Appropriate for: Environments where no traffic is permitted to the public internet from the operator's machine.

**Ollama is required.** Install Ollama locally before configuring Kubecat:

```bash
# macOS
brew install ollama
ollama pull llama3.2          # or your preferred model

# Linux
curl -fsSL https://ollama.com/install.sh | sh
ollama pull llama3.2
```

Configuration:

```yaml
kubecat:
  ai:
    provider: ollama
    endpoint: http://localhost:11434
    model: llama3.2
  telemetry:
    enabled: false
  checkForUpdates: false       # disable GitHub release check
```

`checkForUpdates: false` prevents the update checker from reaching `api.github.com`. With both this and telemetry disabled, Kubecat makes no outbound network requests except to the configured Kubernetes API servers.

---

### Pattern 3: Regulated Environment (HIPAA / PCI-DSS / GDPR)

Appropriate for: Clusters containing regulated data (patient records, payment card data, personal data subject to GDPR).

Requirements:
- No regulated data must be sent to external AI providers.
- Audit log must be retained and reviewed.
- Write operations must require explicit human approval.

Configuration:

```yaml
kubecat:
  readOnly: true
  ai:
    provider: ollama
    endpoint: http://localhost:11434
    model: llama3.2
  telemetry:
    enabled: false
  checkForUpdates: false
  agentGuardrails:
    blockDestructive: true
    requireDoubleConfirm: true
    sessionRateLimit: 10        # conservative
    sessionToolCap: 20
    tokenBudget: 50000
  storage:
    retentionDays: 90           # match audit log retention
    cleanupInterval: 24h
  logger:
    logLevel: info
```

Additional steps:
1. Set `readOnly: true` — no writes possible through Kubecat, even if the user's kubeconfig has write permission.
2. Review the audit log regularly: `~/.local/state/kubecat/audit.log`. Forward to your SIEM if required.
3. Confirm the OS keychain is available so API keys are not stored in plaintext config.
4. Apply the least-privilege `ClusterRole` from Pattern 1.

---

### Pattern 4: Enterprise Fleet Deployment

Appropriate for: Organizations deploying Kubecat to many developer machines via MDM (Intune, Jamf, etc.).

Kubecat reads its config from `~/.config/kubecat/config.yaml`. A managed default config can be pre-seeded by MDM:

```yaml
# Default config deployed via MDM (can be overridden by users)
kubecat:
  telemetry:
    enabled: false
  ai:
    provider: ollama           # or set to your enterprise Ollama endpoint
  agentGuardrails:
    protectedNamespaces:
      - kube-system
      - kube-public
      - kube-node-lease
  checkForUpdates: false
  logger:
    logLevel: warn
```

For environments where you want to enforce read-only on all users and prevent override, this requires a custom build with the `readOnly` default changed — the config file approach alone does not prevent a user from editing `~/.config/kubecat/config.yaml`.

---

## Kubernetes RBAC for Kubecat

Kubecat uses the user's kubeconfig credentials. Grant the minimum permissions needed for the features you want to enable.

### Read-only viewer (all features except writes)

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubecat-viewer
rules:
  - apiGroups: ["", "apps", "batch", "networking.k8s.io", "storage.k8s.io",
                "autoscaling", "policy", "rbac.authorization.k8s.io"]
    resources: ["*"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["argoproj.io"]
    resources: ["applications", "appprojects"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["helm.toolkit.fluxcd.io", "kustomize.toolkit.fluxcd.io",
                "source.toolkit.fluxcd.io"]
    resources: ["*"]
    verbs: ["get", "list", "watch"]
```

### Operator (read + apply/restart, no delete)

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubecat-operator
rules:
  - apiGroups: ["", "apps", "batch"]
    resources: ["pods", "deployments", "statefulsets", "daemonsets",
                "replicasets", "services", "configmaps", "jobs", "cronjobs"]
    verbs: ["get", "list", "watch", "update", "patch"]
  - apiGroups: [""]
    resources: ["pods/log", "pods/exec"]
    verbs: ["get", "create"]
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["get", "list", "watch"]
```

### Restricting write access to specific namespaces

Use `RoleBinding` scoped to individual namespaces rather than `ClusterRoleBinding` when operators should only write in their own namespaces.

---

## Audit Log Management

Kubecat's audit log is at `~/.local/state/kubecat/audit.log`. Each entry is a JSON object on one line.

To forward to a SIEM, use a log shipper agent (Filebeat, Fluent Bit, etc.) configured to tail this file. Example Filebeat input:

```yaml
filebeat.inputs:
  - type: log
    paths:
      - /home/*/kubecat/audit.log   # adjust to actual XDG path
    json.keys_under_root: true
    json.add_error_key: true
    fields:
      app: kubecat
      log_type: audit
```

The audit log retains 90 days of entries by default. If your compliance framework requires longer retention, forward logs to durable external storage.

---

## macOS App Entitlements

On macOS, the release build is signed with the hardened runtime and a minimal entitlements set:

| Entitlement | Reason |
|------------|--------|
| `com.apple.security.app-sandbox` disabled | Required for kubeconfig and filesystem access |
| `com.apple.security.network.client` | Kubernetes API and AI provider connections |
| `com.apple.security.cs.allow-jit` | WebKit WebView JIT compilation |
| `com.apple.security.cs.disable-library-validation` | Dynamic library loading for WebView |

Verify the entitlements on a signed build:

```bash
codesign -d --entitlements - /Applications/Kubecat.app
```

---

## Verifying Security Controls

Run these checks after deployment:

```bash
# 1. Verify config file permissions
ls -la ~/.config/kubecat/config.yaml
# Expected: -rw------- (0600)

# 2. Verify read-only mode is active (attempt a delete, expect error)
# Use the Kubecat UI: Resources → select any non-critical pod → Delete
# Expected: "operation blocked: Kubecat is running in read-only mode"

# 3. Verify no API keys in logs
grep -i 'sk-\|AIza\|Bearer' ~/.local/state/kubecat/kubecat.log
# Expected: no matches

# 4. Verify Ollama is the configured provider (if required)
grep 'provider' ~/.config/kubecat/config.yaml
# Expected: provider: ollama

# 5. Verify telemetry is disabled
grep 'enabled' ~/.config/kubecat/config.yaml | grep telemetry
# Expected: enabled: false (or line absent; default is false)
```

---

## Incident Response

If you suspect Kubecat has made unauthorized API calls:

1. Check the audit log: `cat ~/.local/state/kubecat/audit.log | jq 'select(.eventType == "resource_deletion")'`
2. Check the application log for cluster API errors: `grep -i 'error\|delete\|apply' ~/.local/state/kubecat/kubecat.log`
3. Review Kubernetes audit logs on the API server for calls originating from the user's IP with the user's credentials during the suspected time window.
4. If the Kubecat process was compromised, the OS keychain may contain the AI provider API key — rotate it with the provider immediately.

---

## Related Documents

- [Threat Model](threat-model.md)
- [Data Handling](data-handling.md)
- [AI Data Privacy](ai-data-privacy.md)
- [Security Hardening Checklist](hardening-checklist.md)
- [RBAC Reference](../rbac/README.md)
