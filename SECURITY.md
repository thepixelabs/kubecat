# Security Policy — Kubecat

## Supported Versions

Kubecat is maintained by a single developer. Only the **latest release** receives security fixes.
There are no long-term support branches. If you are running an older version, upgrade before
reporting a vulnerability.

| Version        | Supported |
|----------------|-----------|
| Latest release | Yes       |
| Older releases | No        |

---

## Reporting a Vulnerability

**Please do not file public GitHub issues for security vulnerabilities.**

### Preferred: GitHub Security Advisories (private disclosure)

Open a [private security advisory](https://github.com/thepixelabs/kubecat/security/advisories/new)
on the GitHub repository. This keeps the report confidential until a fix is ready.

### Backup: Email

If you cannot use GitHub's advisory flow, send a report to **team@kubecat.io** with:

- A description of the vulnerability
- Steps to reproduce or a proof-of-concept (PoC)
- The version of Kubecat and macOS you are running
- Your assessment of severity and potential impact

---

## Response Timeline

Kubecat is a solo-maintained project. Here is what you can honestly expect:

| Milestone                           | Target          |
|-------------------------------------|-----------------|
| Acknowledgement of report           | Within 7 days   |
| Initial severity assessment         | Within 14 days  |
| Fix or mitigation for critical/high | Within 30 days  |
| Fix or mitigation for medium/low    | Best effort     |
| Coordinated public disclosure       | After fix lands or 90 days, whichever comes first |

If a critical vulnerability is reported and I have not acknowledged it within 7 days, feel
free to follow up at the backup email address.

---

## Security Scope

The following are **in scope** for vulnerability reports:

- **kubeconfig handling** — improper reading, storage, or exposure of kubeconfig credentials
- **AI provider API key storage** — flaws in OS Keychain integration (`go-keyring`) or any
  code path that could leak API keys to logs, responses, or the filesystem
- **AI agent guardrails** — bypasses that allow destructive or write operations on clusters
  when the guardrail layer should have blocked them, including production-cluster protections
- **MCP server authentication/authorization** — unauthenticated access to the MCP server
  endpoint or authorization flaws that allow callers to exceed their permitted scope
- **RBAC scanning accuracy** — false negatives in the RBAC analyzer that cause genuinely
  dangerous permission grants to go unreported
- **Network policy analysis** — flaws that cause insecure network policies to be reported as safe
- **Terminal feature (PTY)** — any path to remote code execution or privilege escalation via
  the embedded terminal
- **Port forwarding** — unintended network exposure created by the port-forwarding feature
- **SQL injection or path traversal** — in the SQLite history layer or any file-path handling
- **Secrets in logs or audit trail** — API keys, kubeconfig tokens, or Kubernetes Secret
  values appearing in `kubecat.log` or `audit.log`

---

## Out of Scope

The following are **not in scope**:

- Vulnerabilities in **upstream Kubernetes** itself — report those to the
  [Kubernetes Security Disclosure Process](https://kubernetes.io/docs/reference/issues-security/security/)
- Vulnerabilities in **AI provider APIs** (OpenAI, Anthropic, Google, Ollama) — report those
  directly to the respective providers
- **User's own cluster misconfigurations** — Kubecat surfaces what it finds; it cannot fix
  misconfigurations in the cluster being observed
- **Findings from automated scanners without validation** — please verify that a finding
  represents an actual exploitable issue before reporting
- **Denial of service against a user's own local machine** — Kubecat is a desktop app with no
  multi-tenant attack surface

---

## Security Model

**Local-first, no cloud backend.** Kubecat has no backend server. All state (history database,
config, logs) lives on the user's machine. There is no telemetry, no cloud sync, and no
external service that holds user data.

**API keys in the OS Keychain.** AI provider API keys are stored exclusively in the
platform-native secure credential store (macOS Keychain via `go-keyring`). On startup, any
plaintext keys found in the config file are migrated into the keychain and removed from the
file. Keys are never written to logs.

**All cluster operations use the user's existing kubeconfig permissions.** Kubecat does not
hold cluster credentials of its own. It connects to the Kubernetes API using the credentials
already present in the user's `~/.kube/config`, exactly as `kubectl` does. Kubecat cannot
exceed the RBAC permissions the user's own kubeconfig grants.

**AI agent guardrails.** The agentic AI layer enforces a multi-layer guardrail stack before
any tool call reaches the cluster: namespace sandbox, protected-namespace write blocking,
production-cluster destructive-operation blocking, double-confirm prompting, per-session rate
limiting, per-session tool cap, and token budget. Destructive operations on clusters whose
context name matches `prod`, `prd`, `production`, or `live` are blocked by default.

**Audit logging.** Sensitive operations are recorded in an append-only audit log at
`~/.local/state/kubecat/audit.log`. The audit log stores only hashes and metadata — never
raw query text, Secret values, or API keys.

---

## Attribution

Security researchers who responsibly disclose valid vulnerabilities will be credited in the
release notes for the fix, unless they prefer to remain anonymous.

---

*For general questions or non-security bugs, open a standard
[GitHub issue](https://github.com/thepixelabs/kubecat/issues).*
