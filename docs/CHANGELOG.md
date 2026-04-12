# Changelog

All notable changes to Kubecat are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
This project adheres to [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

### Architecture

- Split `app.go` (4,500+ lines) into domain-specific bridge files (`app_cluster.go`, `app_resources.go`, `app_ai.go`, etc.)
- Move business logic from bridge layer into testable internal packages (`internal/security/rbac.go`, `internal/diff/`, `internal/graph/`, `internal/metadata/`)
- Extract AI provider factory into `internal/ai/factory.go` — eliminates 4x duplicated provider construction switch
- Decompose `App.tsx` frontend monolith into route-based page components
- Fix JSON double-parse in resource listing (unstructured → JSON → unmarshal twice per resource)
- Wire up `history.Correlator` — event correlation was fully implemented but never called

### Security

- Lock down `ExecuteCommand`: replace arbitrary `bash -c` with typed allowlist (kubectl, helm, flux, argocd)
- Fix AI autopilot prompt injection: anchor regex patterns, reject shell metacharacters, disable autopilot by default
- Activate `SanitizeForCloud` in all AI query paths — previously defined but never called; full resource YAML was going to cloud providers unsanitized
- Fix XSS via `rehypeRaw` — replace with `rehype-sanitize` with strict allowlist
- Fix SSRF in `FetchProviderModels` — `localhost.attacker.com` bypass
- Fix config file permissions from `0644` to `0600`
- Enforce `readOnly` mode on all mutating methods
- Restrict `StartTerminal` to allowlisted shells only

### CI/CD

- Pin Wails CLI version in CI (`v2.11.0`, not `@latest`) for reproducible builds
- Create GitHub releases as draft first, upload artifacts, then publish — prevents empty public releases on upload failure
- Add `.github/dependabot.yml` for Go, npm, and GitHub Actions ecosystems
- Add `.golangci.yml` — lint gate on PRs
- Add `lefthook.yml` — pre-commit (gofmt, golangci-lint --fast) and pre-push (go test -race, vitest)
- Add `build/darwin/entitlements.plist` for Hardened Runtime
- Add `.devcontainer/` for one-command dev environment setup

### Observability

- Implement structured logging via `log/slog` — replaces 13+ `fmt.Printf("[DEBUG]")` calls
- Rotating log file at `~/.local/state/kubecat/kubecat.log` (10 MiB cap)
- Configurable log level via config (`logLevel: "info"`)
- Add React error boundaries on ClusterVisualizer, AIQueryView, ClusterDiff, TerminalDrawer
- Implement cluster connection health monitoring with exponential backoff reconnection
- Emit Wails events for reactive UI updates (`cluster:status`, `cluster:event`, `log:line`, `resource:changed`, `snapshot:taken`)
- Add data retention automation for SQLite — bounded database growth with configurable TTL

### Testing

- Add unit tests for AI provider factory, sanitization, RBAC analysis, diff computation
- Expand frontend test coverage with coverage thresholds (70% lines/functions, 60% branches)
- Add integration tests with fake Kubernetes API (`k8s.io/client-go/testing`)
- Add SQLite migration system tests
- Rename `safety_test.go` → `cluster_nil_test.go`; add real safety tests for ReadOnly enforcement

### Product / UX

- Build first-launch onboarding flow with cluster connection wizard and AI setup
- Build real dashboard view: cluster health, recent events, unhealthy resources, security score, GitOps sync status
- Fix ELK layout re-render performance — memoize highlight callbacks; layout only runs on data changes
- Add 300ms debounce + AbortController cancellation to `useClusterGraph`
- Make AI model lists dynamic (fetched from provider, hardcoded as fallback)
- Implement opt-in anonymized usage analytics with explicit consent dialog

---

## Phases Completed (Development History)

### Foundation & Core Services

- Initialized Wails v2 application with React + TypeScript frontend
- Implemented `internal/client` package for Kubernetes API interaction via `client-go`
- Implemented `core.ClusterService` — connect, disconnect, multi-cluster management
- Implemented `core.ResourceService` — list, get, apply, delete resources
- Implemented `core.LogService` — real-time log streaming with buffering
- Implemented `core.PortForwardService` — TCP port forwarding to pods
- Added kubeconfig auto-discovery and context switching
- Implemented graceful shutdown sequence (stop collectors, close DB, close clusters)

### AI Integration

- Added AI provider abstraction (`internal/ai/Provider` interface)
- Implemented OpenAI provider with SSE streaming
- Implemented Anthropic provider with SSE streaming
- Implemented Google Gemini provider with streaming
- Implemented Ollama provider for local/air-gapped use
- Added AI query context building from cluster state
- Added resource-specific AI analysis (`AIAnalyzeResource`)
- Added AI typing animation for response streaming
- Added AI settings UI with provider/model selection
- Updated AI settings structure for per-provider configuration map

### History & Time Travel

- Implemented `history.Snapshotter` — periodic cluster state snapshots to SQLite
- Implemented `history.EventCollector` — continuous Kubernetes event ingestion
- Implemented `history.Correlator` — event correlation by ownership chain and timing
- Added `GetTimelineEvents`, `GetSnapshots`, `CompareSnapshots` to Wails bridge
- Added time-travel view showing resource state at any captured snapshot

### Security Scanning

- Implemented `internal/security.Scanner` with RBAC analysis, runtime scanning, network policy analysis
- RBAC analysis: dangerous permissions, wildcard access, secrets access detection
- Runtime scanning: privileged containers, root users, host network/PID/IPC usage
- Network policy analysis: missing policy detection per namespace
- Security scoring system (A–F grade, 0–100 score)
- Policy engine detection: Gatekeeper and Kyverno support
- Added security scan view with filterable issue table

### GitOps Integration

- Implemented `internal/gitops` package with ArgoCD and Flux detection
- ArgoCD application parsing: sync status, health status, destination
- Flux kustomization and HelmRelease parsing
- Added GitOps view showing application sync status

### Cluster Visualization

- Added `@xyflow/react` + `elkjs` cluster visualizer
- Node types: Deployment, StatefulSet, DaemonSet, Job, CronJob, Pod, Service, ConfigMap, Secret, Ingress, PVC
- Edge rendering: owner references, service selectors, ingress → service → pod chains
- Namespace filtering and node highlighting
- Added `useClusterGraph` hook for data fetching

### Cluster Diff

- Implemented snapshot comparison engine
- Added `ClusterDiff` frontend component
- Side-by-side YAML diff with added/removed/changed resource counts
- Changed resource detail modal with before/after YAML

### Terminal

- Implemented `internal/terminal.Manager` using `creack/pty`
- PTY session management: create, resize, write, close
- `xterm.js` frontend terminal component with fit-addon
- Expandable terminal drawer in UI

### Storage Layer

- Implemented `internal/storage.DB` wrapping SQLite via `modernc.org/sqlite`
- Schema migrations: events, snapshots, correlations, resources, settings tables
- `EventRepository`, `SnapshotRepository`, `CorrelationRepository`
- WAL mode enabled for concurrent access

### UI / UX

- Implemented Navbar with cluster selector and connection status
- Implemented Sidebar with navigation (Dashboard, Resources, Visualizer, AI, Security, Timeline, Diff, GitOps, Port Forwards, Terminal, Settings)
- Dark theme with Tailwind CSS — "cockpit" aesthetic with glass panels and accent glows
- Navbar + Sidebar redesign with improved information hierarchy
- Responsive layout with collapsible sidebar

### Configuration

- Implemented `internal/config` package with XDG-compliant directory layout
- YAML config at `~/.config/kubecat/config.yaml`
- `refreshRate`, `theme`, `readOnly`, `ai`, `logger`, `clusters` settings
- Per-cluster ReadOnly override
- `KUBECAT_CONFIG_DIR` environment variable override

### Build & Release

- Wails build pipeline for macOS universal binary
- `create-dmg` DMG creation with custom window layout
- ZIP archive creation alongside DMG
- Semantic release via `go-semantic-release/action`
- Homebrew cask auto-update on release
- GitHub Release draft-first artifact upload pattern
- `internal/version` package with build-time version injection via ldflags
