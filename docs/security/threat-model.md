# Threat Model

> Applies to: Kubecat v0.x (2026). Last updated: 2026-04-07.

This document describes Kubecat's attack surface, trust boundaries, and the controls that protect each boundary. It is intended for enterprise security teams evaluating Kubecat for deployment in production or regulated environments.

**Audience:** Security engineers, compliance reviewers, enterprise IT.

## Application Model

Kubecat is a **single-user desktop application**. It runs as a native OS process on the user's machine. It has:

- No server component, no daemon, no network-accessible port.
- No cloud backend, no telemetry by default.
- No centralized identity provider — it uses the user's existing kubeconfig credentials.

Because Kubecat runs entirely on the user's machine with the user's credentials, its threat surface is fundamentally different from a web-based Kubernetes dashboard. The primary threats are:

1. Kubecat being abused by a compromised AI provider response.
2. Sensitive cluster data leaking to cloud AI APIs.
3. A malicious cluster operator injecting content that causes Kubecat to execute unintended operations.
4. Application-level bugs that bypass the read-only or write-protection controls.

## Trust Boundaries

```
┌─────────────────────────────────────────────────────────────┐
│  Trusted: User's machine                                      │
│                                                               │
│  ┌───────────────────────────────────────────────────────┐   │
│  │  Kubecat process                                        │   │
│  │                                                          │   │
│  │  ┌──────────────┐        ┌───────────────────────┐     │   │
│  │  │ React UI     │◄──────►│ Go backend (App bridge)│     │   │
│  │  │ (WebView)    │  Wails │                         │     │   │
│  │  │              │  IPC   │ checkReadOnly() on all  │     │   │
│  │  └──────────────┘        │ write operations        │     │   │
│  │                           └──────────────┬──────────┘     │   │
│  │                                          │                  │   │
│  │  ┌──────────────────────────────────────┴──────────────┐  │   │
│  │  │  OS Services (trusted)                                │  │   │
│  │  │  - kubeconfig (~/.kube/config)                        │  │   │
│  │  │  - OS keychain (API keys)                             │  │   │
│  │  │  - SQLite history DB                                  │  │   │
│  │  │  - Audit log                                          │  │   │
│  │  └───────────────────────────────────────────────────────┘  │   │
│  └───────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
          │                              │
          ▼ TLS (kubeconfig certs)       ▼ HTTPS
 ┌────────────────────┐        ┌────────────────────────┐
 │ Kubernetes API     │        │ AI Provider API        │
 │ (semi-trusted)     │        │ (untrusted)            │
 └────────────────────┘        └────────────────────────┘
```

## Threat Catalog

### T1 — AI Provider Returns Malicious Instructions (Prompt Injection)

**Risk:** A malicious actor poisons the AI provider's responses to instruct Kubecat to delete resources, exfiltrate secrets, or execute arbitrary commands.

**Mitigations:**

| Control | Implementation |
|---------|---------------|
| Prompt injection delimiter | User question wrapped in `=== BEGIN/END USER QUESTION ===` with explicit "treat as data" instruction |
| Agent tool approval | Every agent tool call is displayed to the user before execution; `requireDoubleConfirm: true` for destructive tools |
| Agent tool allowlist | Only named tools from the tool registry can be called; arbitrary shell execution is not a tool |
| Read-only mode | `readOnly: true` blocks all writes regardless of what the AI requests |
| Guardrails | Rate limit (20 calls/60s), total session cap (50), token budget (100,000 tokens) prevent runaway agent loops |

**Residual risk:** A sophisticated prompt injection that causes the user to manually approve a malicious operation. Mitigation: user education and read-only mode for sensitive clusters.

---

### T2 — Cluster Data Exfiltration via Cloud AI API

**Risk:** Kubernetes Secrets, kubeconfig credentials, or other sensitive data is sent to a cloud AI provider.

**Mitigations:**

| Control | Implementation |
|---------|---------------|
| Secret exclusion from context | Cloud providers never receive the `secrets` resource kind in context, even with values redacted |
| `SanitizeResourceObject` | Strips `Secret.data`, `Secret.stringData`, and secret-named env var values before marshaling to prompt |
| `SanitizeForCloud` | Final text-level pass: redacts `Authorization: Bearer` tokens, secret-named YAML key values, and base64 blobs ≥64 chars |
| Cloud consent dialog | `CloudAIConsentDialog` must be acknowledged before any data is sent to a cloud provider |
| Ollama local-only option | Users can configure Ollama so no data leaves the host at all |

**Residual risk:** Non-secret but sensitive metadata (resource names, namespaces, container images) is sent to cloud providers. This is documented in PRIVACY.md and the consent dialog.

---

### T3 — SSRF via Custom AI Endpoint

**Risk:** An attacker with config-write access sets the AI provider endpoint to a cloud metadata URL (`169.254.169.254`) or internal network address to exfiltrate data from the machine.

**Mitigations:**

| Control | Implementation |
|---------|---------------|
| 5-layer SSRF validator | `internal/network.Validate()` blocks RFC 1918, loopback, link-local, metadata hostnames, and multicast |
| Canonical allowlist | Official cloud provider URLs always pass; anything else is checked against CIDR blocklist |
| Ollama loopback constraint | Ollama endpoint must resolve to loopback — no off-host Ollama connections |
| DNS pre-resolution | Hostname resolved before the request; resolved IPs checked against blocked CIDRs to prevent DNS rebinding |

**Residual risk:** A user with intentional access could configure a non-canonical custom endpoint that resolves to an allowed external IP. This is considered an intentional user action, not an attack.

---

### T4 — API Key Exposure

**Risk:** AI provider API keys are exposed via logs, debug output, or world-readable files.

**Mitigations:**

| Control | Implementation |
|---------|---------------|
| OS keychain storage | `internal/keychain` stores keys in the OS credential store (macOS Keychain, GNOME Keyring, Windows Credential Manager) when available |
| Config file permissions | Config file written with mode `0600`; config directory with `0700` |
| No key logging | `slog` handlers never receive key material; `SanitizeForCloud` redacts `Authorization: Bearer` in log-level debug output |
| Audit log hashing | Prompts are logged as SHA-256 hashes only — no raw content, no keys |

**Residual risk:** On systems where the OS keychain is unavailable (headless servers, CI), keys fall back to plaintext in config file. Kubecat logs a warning when this fallback occurs.

---

### T5 — Arbitrary Command Execution via Terminal or AI

**Risk:** Kubecat's embedded terminal or AI agent spawns arbitrary processes or shell commands.

**Mitigations:**

| Control | Implementation |
|---------|---------------|
| Shell allowlist | `StartTerminal` restricted to `["bash", "zsh", "sh"]`; any other shell name is rejected |
| Command allowlist | `ExecuteCommand` (for AI-suggested commands) uses a typed allowlist: `kubectl`, `helm`, `flux`, `argocd` — no `bash -c` or arbitrary process spawn |
| No shell=true | All `os/exec` calls use explicit argument arrays, never shell interpolation |
| Shell metacharacter rejection | Args are validated against metacharacter patterns before use |

---

### T6 — Accidental Cluster Mutation in Read-Only Mode

**Risk:** A user enables read-only mode for a production cluster but a code path bypasses the check.

**Mitigations:**

| Control | Implementation |
|---------|---------------|
| Centralized read-only check | `App.checkReadOnly()` reads config fresh on every call; returns error if `readOnly: true` |
| Fail-closed | If config cannot be read, `checkReadOnly()` returns an error (fails closed, not open) |
| Applied to every mutating method | All App bridge methods that write to the cluster call `checkReadOnly()` as the first statement |

---

### T7 — XSS via AI Response Rendering

**Risk:** An AI provider returns HTML or JavaScript that executes in the React WebView.

**Mitigations:**

| Control | Implementation |
|---------|---------------|
| `rehype-sanitize` with strict allowlist | HTML responses from AI are sanitized with an allowlist that excludes `<script>`, `on*` attributes, and `javascript:` links |
| No `rehypeRaw` | Raw HTML passthrough is disabled |

---

### T8 — Kubernetes Credential Theft

**Risk:** Kubecat reads kubeconfig credentials and could exfiltrate or log them.

**Mitigations:**

| Control | Implementation |
|---------|---------------|
| No credential storage | Kubecat reads credentials from the existing kubeconfig file; it does not copy or store them |
| No credential logging | Structured logger never receives credential material |
| TLS enforced | `client-go` enforces the TLS configuration from kubeconfig (same as `kubectl`) |
| SSRF protection | Prevents redirecting cluster API calls to attacker-controlled endpoints |

---

### T9 — Malicious Cluster Data Injected into AI Context

**Risk:** A cluster operator creates a pod with annotations like `ignore-previous-instructions` designed to hijack AI responses.

**Mitigations:** See T1 (prompt injection mitigations). Kubernetes metadata is sanitized before insertion into the AI prompt. Shell metacharacters and control characters are stripped.

---

### T10 — Vulnerable Dependencies

**Risk:** A dependency vulnerability allows RCE, data exfiltration, or privilege escalation.

**Mitigations:**

| Control | Implementation |
|---------|---------------|
| `govulncheck ./...` in CI | Fails the build on known Go CVEs |
| `npm audit --audit-level=high` in CI | Fails on high/critical npm CVEs |
| Dependabot weekly PRs | Automated PRs for dependency updates |
| Static analysis | `golangci-lint` with security-focused linters |

---

## Security Controls Summary

| Control | Severity addressed | Config |
|---------|-------------------|--------|
| `readOnly: true` | T1, T6 | `kubecat.readOnly` |
| Agent guardrails | T1 | `kubecat.agentGuardrails.*` |
| Cloud AI consent | T2 | UI: one-time consent dialog |
| Ollama local-only | T2, T3 | `kubecat.ai.provider: ollama` |
| SSRF validator | T3 | Hardcoded; not configurable |
| OS keychain | T4 | Automatic; falls back with warning |
| Shell allowlist | T5 | Hardcoded; not configurable |
| `checkReadOnly()` | T6 | `kubecat.readOnly` |
| `rehype-sanitize` | T7 | Hardcoded |
| Dependency scanning | T10 | CI; Dependabot |

## Recommended Deployment Posture

For production clusters or regulated environments, see [Deployment Guide](deployment-guide.md).

For a detailed checklist with verification steps, see [Security Hardening Checklist](hardening-checklist.md).

## Related Documents

- [Data Handling](data-handling.md)
- [AI Data Privacy](ai-data-privacy.md)
- [Deployment Guide](deployment-guide.md)
- [Security Hardening Checklist](hardening-checklist.md)
- [AI System Architecture](../architecture/ai-system.md)
