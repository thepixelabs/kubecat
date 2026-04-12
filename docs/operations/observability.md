# Observability

This document describes how Kubecat surfaces runtime information for debugging, auditing, and monitoring.

---

## Structured Logging

Kubecat uses Go's standard `log/slog` library with a JSON handler. All log output is newline-delimited JSON for easy parsing with `jq`, `grep`, or any log aggregator.

### Log file location

```
~/.local/state/kubecat/kubecat.log
~/.local/state/kubecat/kubecat.log.1   (previous rotation)
```

### Log rotation

The log file rotates when it reaches **10 MiB**. A single `.1` backup is kept. The rotation is handled by `internal/logging/logging.go`.

### Log level configuration

Set in `~/.config/kubecat/config.yaml`:

```yaml
kubecat:
  logger:
    logLevel: "info"   # debug | info | warn | error
```

| Level | Use case |
|-------|---------|
| `debug` | Full request/response details, source file + line added to every entry |
| `info` | Default — cluster connect/disconnect, AI queries, major operations |
| `warn` | Recoverable errors, fallback paths |
| `error` | Failures that affect user-facing functionality |

### Log structure

Every log entry includes at minimum:

```json
{
  "time": "2026-04-05T09:00:00Z",
  "level": "INFO",
  "msg": "cluster connected",
  "cluster": "prod-us-east-1"
}
```

Operation-scoped entries add `namespace` and `operation`:

```json
{
  "time": "2026-04-05T09:00:05Z",
  "level": "INFO",
  "msg": "resource deleted",
  "cluster": "prod-us-east-1",
  "namespace": "default",
  "operation": "DeleteResource",
  "kind": "Pod",
  "name": "nginx-abc123"
}
```

### Security rules enforced in logging

- API keys, tokens, and passwords are **never logged** at any level.
- Full resource YAML is **never logged** at any level.
- Kubernetes Secret `.data` values are **never logged**.
- Sensitive fields are redacted before reaching any `slog` call — see `internal/ai/provider.go` `SanitizeForCloud()`.

### Useful log queries

```bash
# All errors from today
jq 'select(.level == "ERROR")' ~/.local/state/kubecat/kubecat.log

# All operations on a specific cluster
jq 'select(.cluster == "prod-us-east-1")' ~/.local/state/kubecat/kubecat.log

# AI query latency (if debug level)
jq 'select(.operation == "AIQuery") | .duration_ms' ~/.local/state/kubecat/kubecat.log
```

---

## Audit Log

Kubecat writes a dedicated audit log at:

```
~/.local/state/kubecat/audit.log
```

All mutating operations are written here at `INFO` level with the operation name, cluster, namespace, kind, and resource name. This provides a searchable, separate record of:

- `DeleteResource` — who deleted what, when
- `ApplyResourceToCluster` — manifest application
- `SyncGitOpsApplication` — GitOps sync triggers
- `StartTerminal` — terminal sessions opened
- `CreatePortForward` — port forwarding sessions

ReadOnly mode violations are logged at `WARN`:

```json
{
  "level": "WARN",
  "msg": "operation blocked: read-only mode",
  "operation": "DeleteResource",
  "cluster": "prod-us-east-1"
}
```

---

## Health Monitoring

### Cluster connection health

Kubecat performs a lightweight heartbeat check every 30 seconds per connected cluster (Kubernetes server version API call). Connection states:

| State | Description |
|-------|-------------|
| `connected` | API server responding normally |
| `degraded` | API server responding slowly (>5s) |
| `disconnected` | API server unreachable or auth failed |

On disconnect, Kubecat attempts automatic reconnection with exponential backoff:
- 1s → 2s → 4s → 8s → 16s → 32s → 60s (cap)

### Wails events emitted

These events are emitted from Go to the frontend via `wailsruntime.EventsEmit`:

| Event name | Payload | Description |
|-----------|---------|-------------|
| `cluster:status` | `{cluster, status}` | Connection state change |
| `cluster:event` | `{cluster, event}` | Kubernetes Warning event received |
| `log:line` | `{cluster, namespace, pod, line}` | New log line from streaming |
| `resource:changed` | `{cluster, kind, namespace, name, action}` | K8s Watch event |
| `snapshot:taken` | `{cluster, timestamp}` | Periodic snapshot completed |
| `app:update-available` | `{version, url}` | New Kubecat release detected |

Frontend components subscribe with:

```typescript
import { EventsOn } from "../wailsjs/runtime/runtime";

EventsOn("cluster:status", (payload) => {
  // update connection indicator
});
```

---

## Database Size Monitoring

The SQLite history database is at `~/.local/state/kubecat/history.db`. Kubecat logs a warning if the database exceeds **500 MB**:

```json
{
  "level": "WARN",
  "msg": "database size warning",
  "path": "/home/user/.local/state/kubecat/history.db",
  "size_mb": 523
}
```

See `docs/operations/data-retention.md` for retention configuration.
