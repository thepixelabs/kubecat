# Privacy Policy — Kubecat

**Effective date:** 2026-04-07
**Application:** Kubecat (desktop application for macOS, Linux, and Windows)

---

## 1. Data Stored Locally

Kubecat is a local-first desktop application. All data it collects is stored exclusively on your machine, in your user home directory under the XDG base directories:

| Data type | Default path | Purpose |
|-----------|-------------|---------|
| Configuration | `~/.config/kubecat/config.yaml` | App settings, AI provider config (API keys migrated to OS keychain on startup — see §6) |
| History database | `~/.local/state/kubecat/history.db` | Kubernetes event history, cluster snapshots for time-travel |
| Application logs | `~/.local/state/thepixelabs/kubecat.log` | Structured JSON logs for debugging |
| Audit log | `~/.local/state/kubecat/audit.log` | Hashed audit trail of sensitive operations (AI queries, Secret access, deletions, commands, terminal sessions) |
| Cache | `~/.cache/kubecat/` | Temporary API response cache |

Kubecat does **not** have a backend server. There is no cloud sync, telemetry, or analytics of any kind.

---

## 2. Data Transmitted to AI Providers

When AI features are enabled, Kubecat sends Kubernetes resource data to the configured AI provider in order to answer your queries. This data may include:

- Resource names, namespaces, labels, and annotations
- Pod specs, container images, resource limits
- Event messages and reasons
- Error logs from pods

**What is never sent:**
- Kubernetes Secrets or Secret values
- kubeconfig tokens, certificates, or credentials
- Any data from resources you have not explicitly queried about

You control which provider is used. API keys are read from your local config file and are transmitted only to the provider's own API endpoint.

---

## 3. AI Provider Data Residency

The data residency of AI queries depends on the provider you configure:

| Provider | Data residency | Notes |
|---------|---------------|-------|
| Ollama | **Local only** | Model runs on your machine; no data leaves |
| OpenAI | US (primarily) | Subject to OpenAI's data processing agreement |
| Anthropic | US | Subject to Anthropic's privacy policy |
| Google Gemini | Google Cloud regions | Subject to Google's privacy policy |

Kubecat does not control how third-party providers store or process data. Refer to each provider's privacy policy for details.

---

## 4. Ollama Air-Gapped Operation

If you configure Kubecat to use Ollama with a local endpoint (default: `http://localhost:11434`), **no AI query data leaves your machine or network**. This is the recommended configuration for:

- Air-gapped or highly sensitive environments
- Clusters containing PII or regulated data (HIPAA, PCI-DSS, GDPR)
- Organizations with strict data sovereignty requirements

---

## 5. Kubernetes Cluster Access

Kubecat reads your local `~/.kube/config` to discover available clusters. It connects directly from your machine to the Kubernetes API server using your existing credentials. No credentials are stored by Kubecat beyond what is already in your kubeconfig.

Network traffic between Kubecat and the Kubernetes API server uses the existing TLS configuration in your kubeconfig (same as `kubectl`).

---

## 6. Secrets and Credential Handling

- **AI provider API keys are stored in the OS-native secure credential store** (macOS Keychain, Windows Credential Manager, Linux libsecret via Secret Service API). On startup, Kubecat automatically migrates any plaintext API keys found in `~/.config/kubecat/config.yaml` into the keychain and removes them from the file.
- On systems where the OS keychain is unavailable (headless servers, CI environments), Kubecat falls back to `~/.config/kubecat/config.yaml` (file permissions `0600`) and logs a warning. Kubecat never logs API key values.
- Kubecat does not store Kubernetes Secret values in its database or logs.
- The Kubecat application log file is automatically rotated and capped at 10 MiB.
- Sensitive operations (AI queries, Secret access, resource deletions, command executions, terminal sessions) are recorded in an audit log at `~/.local/state/kubecat/audit.log`. The audit log contains only hashes and metadata — never raw query text, Secret values, or API keys. The audit log is rotated at 50 MiB and entries are purged after 90 days.

---

## 7. Your Rights

Since all data is local, you have full control:

- **Access:** All data is in the directories listed in §1 above.
- **Deletion:** Remove `~/.config/kubecat/`, `~/.local/state/kubecat/` (includes `history.db` and `audit.log`), and `~/.cache/kubecat/` to remove all Kubecat data. Also remove the OS keychain entries: on macOS, search for "kubecat" in Keychain Access; on Linux, search for service "kubecat" in your secret-service manager.
- **Portability:** The SQLite database (`history.db`) and YAML config are standard formats you can read with any compatible tool.

---

## 8. Disclaimer

Kubecat is provided as-is. The authors accept no liability for data transmitted to third-party AI providers or for data losses resulting from application bugs. Use read-only mode (`readOnly: true` in config) when connecting to production clusters.

**This document is not legal advice.** It describes the technical data-handling behavior of the Kubecat application. Organizations operating in regulated industries (HIPAA, PCI-DSS, GDPR, SOC 2, FedRAMP, etc.) should consult qualified legal counsel and their internal compliance teams before deploying Kubecat against clusters containing regulated data. For such deployments we recommend using Ollama as the AI provider (see §4) so that no cluster data leaves your environment.

---

*Questions? Open an issue at https://github.com/thepixelabs/kubecat*
