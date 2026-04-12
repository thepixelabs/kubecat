# Data Handling

> Applies to: Kubecat v0.x (2026). Last updated: 2026-04-07.

This document describes every category of data that Kubecat collects, stores, or transmits: what it is, where it lives, who can access it, and how to delete it.

**Audience:** Security engineers, compliance reviewers, data protection officers, enterprise IT.

## Data Residency Principle

Kubecat is **local-first**. All data is stored on the user's machine. There is no cloud backend, no central database, and no network-accessible server process. Data leaves the host only in two explicitly opt-in scenarios:

1. The user configures a cloud AI provider (OpenAI, Anthropic, Google).
2. The user enables anonymous telemetry (disabled by default).

## Data Inventory

### 1. Configuration

| Attribute | Value |
|-----------|-------|
| Location | `~/.config/kubecat/config.yaml` |
| File permissions | `0600` (owner read/write only) |
| Directory permissions | `0700` |
| Sensitive fields | `kubecat.ai.apiKey` (if not using OS keychain) |
| Transmitted | Never |

Contains: AI provider name, UI preferences, theme, alert configuration, guardrail settings, telemetry opt-in status. May contain an AI provider API key in plaintext if the OS keychain is unavailable (see §5 below).

### 2. History Database

| Attribute | Value |
|-----------|-------|
| Location | `~/.local/share/kubecat/history.db` |
| Format | SQLite 3, WAL mode |
| File permissions | OS default (typically `0600`) |
| Default retention | 7 days |
| Transmitted | Never |

Contains:
- **Events table** — Kubernetes event records: cluster, namespace, involvedObject kind/name, type (`Normal`/`Warning`), reason, message text, timestamps, source component and host.
- **Snapshots table** — Periodic cluster state: namespaces, resource names, statuses, labels. Stored as compressed JSON blobs.
- **Correlations table** — Links between related events with confidence scores.
- **Resources table** — Resource version tracking for change detection.
- **Settings table** — User preferences key-value pairs.

**What is NOT stored in the history database:**
- Kubernetes Secret values
- Pod log contents
- kubeconfig credentials
- AI provider API keys
- Container environment variable values

### 3. Application Log

| Attribute | Value |
|-----------|-------|
| Location | `~/.local/state/kubecat/kubecat.log` |
| Format | Structured JSON (one object per line) |
| Rotation | At 10 MiB |
| Transmitted | Never |

Contains: startup/shutdown events, cluster connection events, error traces. Sensitive values (API keys, secret data) are never written to this log. Log level is configurable via `kubecat.logger.logLevel`.

### 4. Audit Log

| Attribute | Value |
|-----------|-------|
| Location | `~/.local/state/kubecat/audit.log` |
| Format | Structured JSON (one object per line) |
| Rotation | At 50 MiB |
| Retention | 90 days (automatic purge at startup) |
| Transmitted | Never |

Contains structured entries for security-sensitive operations:

| Event type | Logged when |
|-----------|-------------|
| `ai_query` | User sends an AI query |
| `secret_access` | AI context includes a secrets resource list |
| `resource_deletion` | User deletes a Kubernetes resource |
| `command_execution` | Agent executes a tool or user runs a command |
| `provider_config_change` | AI provider settings are modified |
| `terminal_session` | User opens a terminal session |

Each entry contains: timestamp, event type, cluster name, namespace, resource name, provider name, and a SHA-256 hash of the prompt/command. **The prompt/command itself is never stored** — only its hash.

### 5. AI Provider API Keys

| Attribute | Value |
|-----------|-------|
| Primary storage | OS credential store (macOS Keychain, GNOME Keyring, Windows Credential Manager) |
| Fallback storage | `~/.config/kubecat/config.yaml` (plaintext, file permissions `0600`) |
| Fallback warning | Kubecat logs a warning when the OS keychain is unavailable |
| Transmitted | Only to the configured provider's API endpoint |

### 6. kubeconfig and Cluster Credentials

| Attribute | Value |
|-----------|-------|
| Location | `~/.kube/config` (standard kubeconfig; Kubecat reads but does not copy) |
| Stored by Kubecat | No |
| Transmitted | TLS-encrypted to the configured Kubernetes API server only |

Kubecat reads the existing kubeconfig using the same mechanisms as `kubectl`. It does not copy, cache, or store kubeconfig credentials. Cluster connections use the TLS certificates and tokens already configured in the kubeconfig.

### 7. Anonymous Telemetry (opt-in)

| Attribute | Value |
|-----------|-------|
| Default | Disabled |
| Enabled by | User explicit opt-in during onboarding or in Settings |
| Transmitted | Usage event names + anonymous installation ID |
| Cluster data transmitted | None |
| Secret data transmitted | None |

When enabled, Kubecat transmits anonymized usage events (feature usage counts, not content) identified by a random UUID generated at installation time. No cluster names, resource names, namespaces, or query content are included. Events are buffered in memory (max 100) and flushed every 5 minutes.

To disable: set `kubecat.telemetry.enabled: false` in config or use the Settings UI.

## Data That Never Leaves The Host

The following data categories are explicitly excluded from any transmission path:

- Kubernetes Secret values (`.data`, `.stringData`, container env vars with secret-named keys)
- kubeconfig credentials (tokens, certificates, passwords)
- Pod log contents
- AI provider API keys
- Raw cluster resource YAML
- Prompt text (only SHA-256 hash is logged in audit log)
- Base64-encoded blobs ≥ 64 characters (redacted by `SanitizeForCloud`)

## Sanitization Controls

See [AI Data Privacy](ai-data-privacy.md) for the complete sanitization pipeline for cloud AI queries.

The two sanitization functions applied before any cloud AI request:

- `ai.SanitizeResourceObject()` — strips secret values at the Go struct level before marshaling
- `ai.SanitizeForCloud()` — text-level final pass: redacts bearer tokens, secret-named key values, and base64 blobs

## Data Deletion

To remove all Kubecat data from a machine:

```bash
# Configuration
rm -rf ~/.config/kubecat/

# History database + logs + audit log
rm -rf ~/.local/state/kubecat/
rm -rf ~/.local/share/kubecat/

# Cache
rm -rf ~/.cache/kubecat/

# API keys from OS keychain (macOS)
security delete-generic-password -s kubecat

# API keys from OS keychain (Linux, using secret-tool)
secret-tool clear service kubecat
```

On macOS, paths may be under `~/Library/` depending on whether the XDG override is set. Check the application log location in Settings to confirm the actual paths on your machine.

## Regulatory Considerations

Kubecat is not certified for any regulatory framework. Organizations operating in regulated environments should review:

- Whether the data in the history database (event messages, resource names) constitutes regulated data under HIPAA, PCI-DSS, GDPR, or similar.
- Whether the 90-day audit log retention meets or exceeds their compliance requirements.
- Whether using Ollama (local-only AI) is required to meet data sovereignty or data residency obligations.

See [Deployment Guide](deployment-guide.md) for recommended configurations for regulated environments.

## Related Documents

- [Threat Model](threat-model.md)
- [AI Data Privacy](ai-data-privacy.md)
- [Deployment Guide](deployment-guide.md)
- [Privacy Policy](../../PRIVACY.md)
