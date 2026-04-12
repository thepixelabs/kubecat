# Project Structure

> Applies to: Kubecat v0.x (2026). Last updated: 2026-04-07.

This document describes the organization of the Kubecat codebase.

## Directory Layout

```
kubecat/
├── main.go                     # Entry point — Wails app init
├── app.go                      # App bridge: Go methods bound to frontend
├── app_agent_executor.go       # Agent tool executor implementation
├── app_alerts.go               # Alert monitor wiring
├── go.mod                      # Go module definition
├── go.sum                      # Dependency checksums
├── Makefile                    # Build automation
├── README.md                   # Project README
├── PRIVACY.md                  # Privacy policy
├── LICENSE                     # Apache 2.0 license
│
├── frontend/                   # React/TypeScript UI
│   ├── src/
│   │   ├── components/         # React components (per feature)
│   │   │   ├── AIQueryView.tsx
│   │   │   ├── Dashboard.tsx
│   │   │   ├── Sidebar.tsx
│   │   │   ├── cluster-diff/   # Cluster diff components
│   │   │   ├── cluster-visualizer/ # Graph visualizer
│   │   │   ├── Terminal/       # Embedded xterm.js terminal
│   │   │   ├── agent/          # Agent panel components
│   │   │   └── onboarding/     # Onboarding wizard
│   │   ├── wailsjs/            # Auto-generated Wails bindings
│   │   └── ...
│   ├── package.json
│   ├── vite.config.ts
│   └── vitest.config.ts
│
├── internal/                   # Private Go packages
│   ├── ai/                     # AI/LLM integration
│   ├── alerts/                 # Proactive alert monitor
│   ├── analyzer/               # Resource health analyzers
│   ├── audit/                  # Structured audit logging
│   ├── client/                 # Kubernetes client layer
│   ├── config/                 # Configuration management
│   ├── core/                   # Core business logic (UI-agnostic)
│   ├── diff/                   # Cluster diff / comparison engine
│   ├── events/                 # Wails event emitter
│   ├── gitops/                 # ArgoCD + Flux integration
│   ├── graph/                  # Resource relationship graph
│   ├── health/                 # Cluster health monitor
│   ├── history/                # Event collection + snapshotting
│   ├── keychain/               # OS keychain wrapper
│   ├── logging/                # Structured logging setup
│   ├── metadata/               # Resource metadata extraction
│   ├── network/                # SSRF validator
│   ├── security/               # Security scanner + RBAC
│   ├── storage/                # SQLite schema + repositories
│   ├── telemetry/              # Anonymous usage telemetry
│   ├── terminal/               # Terminal session manager
│   ├── updater/                # Auto-update checker
│   └── version/                # Build version metadata
│
├── docs/                       # Documentation
│   ├── README.md               # Documentation index
│   ├── architecture/           # Architecture docs (this directory)
│   ├── development/            # Developer setup + contributing
│   ├── features/               # Feature documentation
│   ├── getting-started/        # Installation + quickstart
│   ├── operations/             # Observability, retention, updates
│   ├── rbac/                   # RBAC reference
│   ├── reference/              # Config reference, keybindings, themes
│   ├── security/               # Security model documentation
│   ├── testing/                # Test documentation
│   └── user-guide/             # User-facing feature walkthroughs
│
├── build/                      # Platform build assets
│   └── darwin/
│       └── entitlements.plist  # macOS app entitlements
│
├── scripts/                    # Developer tooling
│   └── testdata-generator/     # K8s test data generation
│
└── .devcontainer/              # Dev container definition
    └── devcontainer.json
```

## Package Descriptions

### Root packages

| File | Purpose |
|------|---------|
| `main.go` | Entry point. Initializes logging, config, all Go services, and starts the Wails desktop window. |
| `app.go` | `App` struct — the Wails bridge. All exported methods become callable from the React frontend. |
| `app_agent_executor.go` | `ToolExecutor` implementation: maps agent tool names to real K8s operations. |
| `app_alerts.go` | Wires the proactive alert monitor and Wails event emission. |

### `internal/ai`

AI/LLM integration layer.

| File | Purpose |
|------|---------|
| `provider.go` | `Provider` interface, `ContextBuilder`, `BuildPrompt`, `SanitizeForCloud`, `SanitizeResourceObject` |
| `factory.go` | `NewProviderFromConfig` — constructs the right provider from config |
| `ollama.go` | Ollama (local) provider implementation |
| `openai.go` | OpenAI provider implementation |
| `anthropic.go` | Anthropic provider implementation |
| `google.go` | Google Gemini provider implementation |
| `agent.go` | `Agent` — observe-think-act loop, capped at 10 iterations |
| `guardrails.go` | `Guardrails` — 7-layer safety checks on agent tool calls |
| `tools.go` | Tool registry: definitions of every tool the agent can call |
| `analysis.go` | Deep resource analysis for AI context building |
| `tls.go` | Custom TLS configuration for AI provider connections |

### `internal/client`

Kubernetes client abstraction.

| File | Purpose |
|------|---------|
| `types.go` | `ClusterClient` and `Manager` interfaces, `Resource` type |
| `cluster.go` | Single-cluster implementation using client-go dynamic client |
| `manager.go` | Multi-cluster pool: add, remove, list, get active cluster |
| `kubeconfig.go` | Kubeconfig file parsing and context enumeration |
| `errors.go` | Domain-specific error types (`ErrNotConnected`, etc.) |

### `internal/config`

Configuration management following XDG Base Directory specification.

```yaml
# Default config paths
~/.config/kubecat/config.yaml     # User configuration
~/.local/share/kubecat/           # Application data (history DB)
~/.local/state/kubecat/           # Logs and audit log
~/.cache/kubecat/                 # Cache data
```

Top-level config keys: `kubecat.readOnly`, `kubecat.theme`, `kubecat.ai`, `kubecat.agentGuardrails`, `kubecat.alerts`, `kubecat.storage`, `kubecat.telemetry`.

### `internal/core`

UI-agnostic business logic. The `Kubecat` service container holds:

- `ClusterService` — connection management, context switching
- `ResourceService` — CRUD operations on any K8s resource
- `LogService` — streaming pod log tailing
- `PortForwardService` — port-forward session lifecycle

`safety.go` provides input validation (`ValidateResourceName`, `ValidateNamespace`) and the `guardReadOnly` helper used throughout the package.

### `internal/history`

Time-travel subsystem.

| File | Purpose |
|------|---------|
| `events.go` | `EventCollector` — watches K8s events, persists to SQLite, triggers correlation |
| `snapshotter.go` | `Snapshotter` — 5-minute periodic cluster state capture, `CompareSnapshots` |
| `correlator.go` | `Correlator` + `DefaultCorrelationRules` — links related events with confidence scores |

### `internal/storage`

SQLite database layer.

| File | Purpose |
|------|---------|
| `db.go` | `DB` — opens WAL-mode SQLite, runs migrations |
| `schema.go` | SQL migrations (v1: snapshots/events/correlations/resources, v2: settings) |
| `events.go` | `EventRepository` — save, list, delete-older-than |
| `snapshots.go` | `SnapshotRepository` — save, get-at, get-latest, list-timestamps |
| `correlations.go` | `CorrelationRepository` — save, list-by-event |
| `retention.go` | `RetentionManager` — periodic cleanup goroutine |

### `internal/security`

Security scanning.

| File | Purpose |
|------|---------|
| `scanner.go` | Scans for privileged containers, host namespace exposure, RBAC issues |
| `types.go` | Finding severity levels and result types |
| `netpol_recommender.go` | Recommends NetworkPolicy rules for namespaces lacking them |

### `internal/network`

| File | Purpose |
|------|---------|
| `validator.go` | `Validate(url, providerName)` — 5-layer SSRF protection for AI provider endpoints |

### `internal/audit`

| File | Purpose |
|------|---------|
| `logger.go` | Structured JSON audit logger. Logs to `~/.local/state/kubecat/audit.log`. 50 MiB rotation, 90-day retention. Event types: `ai_query`, `secret_access`, `resource_deletion`, `command_execution`, `provider_config_change`, `terminal_session`. |

### `internal/keychain`

OS keychain wrapper using `go-keyring`. Used to store AI provider API keys. Falls back to config file with a warning when the OS keychain is unavailable (e.g., headless server, CI).

### `frontend/src/components`

React components, organized by feature area:

| Directory / File | Purpose |
|-----------------|---------|
| `Dashboard.tsx` | Cluster overview dashboard |
| `AIQueryView.tsx` | Natural language AI query interface |
| `Sidebar.tsx` | Navigation sidebar |
| `cluster-diff/` | Cluster comparison UI |
| `cluster-visualizer/` | Graph-based resource relationship visualizer |
| `Terminal/` | Embedded xterm.js terminal (uses `terminal.Manager` backend) |
| `agent/AgentPanel.tsx` | Agentic AI panel with tool approval UI |
| `onboarding/` | First-run onboarding wizard |
| `CloudAIConsentDialog.tsx` | Consent prompt before first cloud AI use |
| `TelemetryConsentDialog.tsx` | Consent prompt for anonymous telemetry |
| `SettingsModal.tsx` | Application settings UI |
| `ProactiveAlertBanner.tsx` | In-app alert banner for proactive cluster alerts |

## Import Conventions

```go
// Standard library first
import (
    "context"
    "fmt"
    "time"
)

// External packages
import (
    "github.com/wailsapp/wails/v2"
    "k8s.io/client-go/dynamic"
)

// Internal packages
import (
    "github.com/thepixelabs/kubecat/internal/ai"
    "github.com/thepixelabs/kubecat/internal/client"
    "github.com/thepixelabs/kubecat/internal/config"
)
```

## File Naming Conventions

- Go files: lowercase, underscores for multi-word (`port_forward.go`)
- Test files: `*_test.go` alongside the file under test
- Interface definitions: `types.go`
- Error definitions: `errors.go`
- Main implementation: named after the primary type (`snapshotter.go` contains `Snapshotter`)
