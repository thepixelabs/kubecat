# Architecture Overview

> Applies to: Kubecat v0.x (2026). Last updated: 2026-04-07.

Kubecat is a desktop GUI application for managing Kubernetes clusters. It is built on [Wails v2](https://wails.io/), which embeds a React/TypeScript frontend inside a native desktop window using the platform's system WebView (WebKit on macOS, WebView2 on Windows, WebKitGTK on Linux). There is no server process, no separate HTTP port, and no network-accessible surface — the entire application runs as a single OS process.

**Audience:** Engineers evaluating, contributing to, or operating Kubecat.

## Design Principles

- **Desktop-native, not web-based.** The UI runs in a system WebView embedded in the app binary. There is no web server, no port binding, no remote access surface.
- **Go backend, React frontend.** All cluster access, AI calls, storage, and business logic live in Go. The React frontend is a display and interaction layer only.
- **Local by default.** Kubernetes credentials stay on disk (kubeconfig). AI API keys are stored in the OS keychain when available, not in plaintext config. No data leaves the host unless the user explicitly configures a cloud AI provider.
- **Read-only mode.** A `readOnly: true` config flag blocks every write operation at the Go bridge layer, before any API call is made.

## Component Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          OS Process: kubecat                                 │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                  Wails v2 Desktop Window (WebView)                    │   │
│  │                                                                        │   │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │   │
│  │  │                  React / TypeScript Frontend                      │  │   │
│  │  │                                                                    │  │   │
│  │  │  Dashboard  Explorer  Cluster Diff  Time Travel  GitOps  AI Query│  │   │
│  │  │  Security   Terminal  Visualizer   Settings     Onboarding       │  │   │
│  │  └──────────────────────────┬──────────────────────────────────────┘  │   │
│  │                             │ Wails IPC (JS ↔ Go)                      │   │
│  └─────────────────────────────┼────────────────────────────────────────┘   │
│                                │                                              │
│  ┌─────────────────────────────▼────────────────────────────────────────┐   │
│  │                      App Bridge (app.go)                               │   │
│  │  Exported Go methods bound to the Wails runtime.                       │   │
│  │  checkReadOnly() gate on every mutating method.                        │   │
│  └──┬──────────────┬──────────────┬──────────────┬────────────────────┘   │
│     │              │              │              │                           │
│  ┌──▼──────┐ ┌──▼──────┐ ┌──▼────────┐ ┌──▼──────────────┐             │
│  │  core   │ │ history │ │    ai     │ │    storage      │             │
│  │Kubecat  │ │Collector│ │ Provider  │ │     DB          │             │
│  │         │ │Snapshotr│ │ Agent     │ │ RetentionMgr    │             │
│  └──┬──────┘ └──┬──────┘ └──┬────────┘ └─────────────────┘             │
│     │           │            │                                             │
│  ┌──▼──────┐    │    ┌───▼──────────┐                                    │
│  │ client  │    │    │   network    │                                    │
│  │ Manager │    │    │  Validator   │ (SSRF protection)                  │
│  └──┬──────┘    │    └───────────────┘                                    │
│     │           │                                                           │
│  ┌──▼──────┐  ┌─▼──────┐                                                 │
│  │ SQLite  │  │ Events │  history DB + event tables                       │
│  │   WAL   │  │ tables │                                                  │
│  └─────────┘  └────────┘                                                  │
│                                                                              │
│  ┌────────────────────────────────────────────────────────────────────────┐  │
│  │  OS Services                                                             │  │
│  │  keychain (API keys)  audit.log  kubecat.log  kubeconfig (~/.kube)     │  │
│  └────────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
              │                                          │
              ▼                                          ▼
  Kubernetes Cluster API (client-go)          AI Provider API
  (uses kubeconfig credentials)               (Ollama / OpenAI /
                                               Anthropic / Google)
```

## Core Components

### Wails v2 Desktop Window

Wails bundles the compiled React app as an embedded filesystem (`//go:embed all:frontend/dist`) and serves it from a platform WebView. There is no network socket. IPC between JavaScript and Go uses a Wails-provided binding layer: exported methods on the `App` struct become callable as `window.go.App.MethodName(args)` in JavaScript.

Window startup sequence:
1. `main()` initializes logging, config, and all Go services.
2. `wails.Run()` opens the native window and calls `App.startup(ctx)`.
3. The WebView loads the embedded React bundle.
4. React calls Go methods over the Wails IPC bridge to hydrate the UI.

### App Bridge (`app.go`)

`App` is the single struct bound to the Wails runtime. It holds references to every Go service and exposes cluster, resource, AI, history, terminal, and settings operations as exported methods. All mutating methods call `checkReadOnly()` before touching the cluster API.

Key fields:

| Field | Type | Purpose |
|-------|------|---------|
| `nexus` | `*core.Kubecat` | Service container (clusters, resources, logs, port-forwards) |
| `db` | `*storage.DB` | SQLite history database |
| `eventCollector` | `*history.EventCollector` | Watches K8s events, persists them |
| `snapshotter` | `*history.Snapshotter` | Periodic cluster state snapshots |
| `emitter` | `*events.Emitter` | Pushes Wails events to the frontend |
| `healthMonitor` | `*health.ClusterHealthMonitor` | Heartbeat and reconnect state machine |
| `retentionMgr` | `*storage.RetentionManager` | Periodic SQLite cleanup |
| `terminalManager` | `*terminal.Manager` | Manages in-app terminal sessions |

### Core Service Container (`internal/core`)

`core.Kubecat` is a UI-agnostic service container wrapping four services:

- **ClusterService** — kubeconfig parsing, multi-cluster connection pool, context switching
- **ResourceService** — list, get, apply, delete operations on any K8s resource
- **LogService** — streaming pod log tailing
- **PortForwardService** — kubectl-compatible port-forward sessions

### Kubernetes Client (`internal/client`)

Built on `client-go`. `client.Manager` manages a pool of `ClusterClient` instances, one per connected cluster. `ClusterClient` exposes a dynamic interface (List, Get, Apply, Watch, Delete) that works with any Kubernetes resource kind via the discovery API.

### History Subsystem (`internal/history`, `internal/storage`)

Two background goroutines maintain a time-series record of cluster state:

- **EventCollector** — watches `events` resources on every connected cluster. On reconnect it uses exponential backoff (1s → 2s → … → 60s cap). Persists events to SQLite, then triggers the `Correlator`.
- **Snapshotter** — every 5 minutes (default), snapshots pods, deployments, services, configmaps, statefulsets, daemonsets, jobs, and cronjobs for every connected cluster. Stores compressed JSON blobs in SQLite.
- **Correlator** — links related events using configurable rules (e.g., Deployment scaling → Pod events within a 2-minute window). Writes `correlations` table rows with a confidence score.

Default retention for both events and snapshots: 7 days. Configurable via `storage.retentionDays` in config.

### AI Subsystem (`internal/ai`)

See [ai-system.md](ai-system.md) for full details.

Summary:
- **Provider interface** — `Ollama`, `OpenAI`, `Anthropic`, and `Google` implementations, all interchangeable via `ai.Provider`.
- **ContextBuilder** — assembles cluster context (resources, recent events) for a prompt. Excludes secrets from cloud providers.
- **Agent** — observe-think-act loop, capped at 10 iterations, with `Guardrails` enforcing namespace restrictions, rate limits, token budgets, and double-confirm on destructive tools.
- **Sanitization** — `SanitizeForCloud()` and `SanitizeResourceObject()` redact bearer tokens, secret values, and base64 blobs before any cloud API call.
- **SSRF protection** — `internal/network.Validate()` enforces a 5-layer check on every outbound endpoint URL.

### Security Subsystem (`internal/security`)

Runs security scans against the live cluster: RBAC analysis, privileged container detection, host namespace exposure, network policy gaps. Returns scored findings to the frontend.

### GitOps Subsystem (`internal/gitops`)

Detects and reads ArgoCD `Application` and Flux `Kustomization`/`HelmRelease` resources from the cluster. Surfaces sync status, health, and drift indicators in the GitOps view.

### Storage (`internal/storage`)

Single embedded SQLite database at `~/.local/share/kubecat/history.db`. WAL mode enabled for concurrent reads. Schema managed by sequential migrations (see [storage.md](storage.md)).

### Observability

- **Structured logging** — `slog`-based JSON logs at `~/.local/state/kubecat/kubecat.log`.
- **Audit log** — `internal/audit` writes structured JSON entries for AI queries, secret access, resource deletions, command executions, and provider config changes. Path: `~/.local/state/kubecat/audit.log`. 50 MiB rotation, 90-day retention.
- **Wails event push** — `events.Emitter` pushes real-time events (cluster health changes, alert notifications) to the React frontend without polling.

## Technology Stack

| Layer | Technology | Notes |
|-------|-----------|-------|
| Desktop shell | Wails v2 | Native window + WebView bridge |
| Frontend | React + TypeScript | Embedded at build time |
| Backend language | Go 1.23+ | All business logic |
| K8s client | client-go | Dynamic + typed clients |
| Database | SQLite (WAL mode) | Embedded, no server |
| AI providers | Ollama, OpenAI, Anthropic, Google | Provider interface abstraction |
| Credential store | OS keychain (go-keyring) | Fallback: config file |
| Logging | slog (structured JSON) | |
| Audit | Custom JSON rotation logger | |

## File Locations

All paths follow XDG Base Directory conventions on Linux/macOS:

| Purpose | Default Path |
|---------|-------------|
| Config file | `~/.config/kubecat/config.yaml` |
| Application log | `~/.local/state/kubecat/kubecat.log` |
| Audit log | `~/.local/state/kubecat/audit.log` |
| History database | `~/.local/share/kubecat/history.db` |
| Cache | `~/.cache/kubecat/` |

## Related Documents

- [Data Flow](data-flow.md) — how data moves through each subsystem
- [Project Structure](project-structure.md) — codebase layout
- [AI System](ai-system.md) — provider abstraction, prompt pipeline, agent loop
- [Storage](storage.md) — SQLite schema and retention
- [History](history.md) — event collection, snapshotting, correlation
