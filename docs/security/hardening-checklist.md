# Security Hardening Checklist

This checklist documents each security control in Kubecat, its implementation location, and how to verify it.

---

| # | Risk | Control | File Reference | Verify |
|---|------|---------|---------------|--------|
| 1 | Arbitrary shell execution via RPC | `ExecuteCommand` uses typed allowlist (kubectl, helm, flux, argocd) — no `bash -c` | `app.go` → `app_terminal.go` | Attempt to call `ExecuteCommand("rm -rf /")` from frontend; confirm blocked with error |
| 2 | AI prompt injection from malicious pod annotations | Kubernetes metadata sanitized before AI prompt insertion; shell metacharacters stripped | `internal/ai/provider.go` `SanitizeForCloud()` | Insert annotation `"ignore-previous-instructions"` on a pod; confirm it is stripped from prompt |
| 3 | Cluster data exfiltration to cloud AI providers | `SanitizeForCloud` called on all query paths; Secret values never sent | `internal/ai/provider.go`, `app_ai.go` | Enable debug logging, run AI query; verify no Secret `.data` values appear in log |
| 4 | XSS via AI response rendering | `rehype-sanitize` with strict allowlist replaces `rehypeRaw`; no `<script>` or `on*` attributes | `frontend/src/components/AIQueryView.tsx` | Ask AI to return `<script>alert(1)</script>`; verify it is escaped/stripped in render |
| 5 | SSRF via custom provider endpoint | Endpoint validated against known base URLs; RFC-1918 and link-local ranges blocked | `app.go` → `app_ai.go` `FetchProviderModels` | Set endpoint to `http://169.254.169.254/latest/meta-data/`; confirm request is rejected |
| 6 | API key exposure via world-readable config | Config file written with `0600` permissions; directory created with `0700` | `internal/config/config.go:237` | `ls -la ~/.config/kubecat/config.yaml` → must show `-rw-------` |
| 7 | Accidental cluster mutation in read-only mode | `checkReadOnly()` called at top of every mutating method | `app.go` `checkReadOnly()`, `app_resources.go`, `app_terminal.go` | Set `readOnly: true` in config; attempt `DeleteResource` → must return error |
| 8 | Terminal spawning arbitrary process | `StartTerminal` restricted to `["bash", "zsh", "sh"]` allowlist; args reject shell metacharacters | `internal/terminal/manager.go`, `app_terminal.go` | Call `StartTerminal("python3", [])` → must be rejected |
| 9 | API key logging | `slog` handlers never receive key material; `SanitizeForCloud` redacts before log | `internal/logging/logging.go`, `internal/ai/provider.go` | Search log file for API key patterns (`sk-`, `AIza`); must find none |
| 10 | Kubernetes Secret values reaching logs or AI | Secret `.data` and `.stringData` fields stripped before any processing | `internal/ai/provider.go` `SanitizeForCloud()` | List secrets via AI query; verify `.data` values do not appear in AI prompt (debug log) |
| 11 | Hardened Runtime missing (macOS) | Entitlements plist provides minimum required rights only | `build/darwin/entitlements.plist` | `codesign -d --entitlements - build/bin/kubecat.app` → verify only 4 entitlements present |
| 12 | Dependency vulnerabilities | `govulncheck ./...` in CI; `npm audit --audit-level=high` in CI; Dependabot weekly PRs | `.github/workflows/ci.yaml`, `.github/dependabot.yml` | Check CI job "Vulnerability Scan" passes on every PR |
| 13 | Container/network privilege escalation (future server mode) | Network policies restrict pod-to-pod traffic; non-root UID enforced in any server deployment | `docs/rbac/viewer-clusterrole.yaml`, `docs/rbac/operator-clusterrole.yaml` | `kubectl auth can-i --list --as=system:serviceaccount:kubecat:viewer` → confirm read-only verbs only |

---

## Notes

- Items 1–8 address findings from the security hardening epic (critical and high severity findings).
- Items 9–10 address the AI data transmission risk documented in `PRIVACY.md`.
- Item 11 is required for ad-hoc signing to work on Apple Silicon.
- Items 12–13 are preventive controls verified through the CI pipeline.

Run `govulncheck ./...` and `npm audit` locally before any release.
