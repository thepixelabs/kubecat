# Implementation Status (Docs vs Code)

This page tracks what is **actually implemented in the repository today**, compared to what the docs describe.

## Status Legend

- **✅ Implemented (reachable)**: Implemented and currently wired into the shipped GUI.
- **🟡 Partially implemented / not wired**: Code exists, but it's not connected to the main app flow yet (or only a subset exists).
- **🔄 Planned**: Described in docs, but no meaningful implementation exists yet.

## What's implemented and reachable today (GUI)

These features are available in the Wails GUI (`app.go`):

- **✅ Multi-cluster connect/switch/disconnect**
  Implemented in `internal/client/*`. GUI methods: `GetContexts()`, `ConnectToCluster()`, `DisconnectFromCluster()`, `SetActiveContext()`.

- **✅ Resource explorer (list + kind switching + filtering)**
  Implemented in the frontend (`frontend/src/components/views/ExplorerView.tsx`) against `GetResources()`, `GetResourceDetail()`, and `DeleteResource()` backend methods.

- **✅ Event timeline (history-backed)**
  Backend implemented in `internal/history/*` with SQLite storage.
  GUI methods: `GetEvents()`, `GetTimeline()`.

- **✅ AI Query (chat UI + providers)**
  Implemented in `frontend/src/components/AIQueryView.tsx` + `internal/ai/*`.
  Requires `kubecat.ai.enabled: true` in config.

- **✅ Describe**
  GUI method: `GetResourceDetail()`, rendered in the frontend detail panel.

- **✅ Logs**
  GUI method: `GetPodLogs()`, rendered in the frontend log viewer.

- **✅ Exec/Shell**
  GUI method: `StartTerminal()`, rendered in `frontend/src/components/Terminal/`.

- **✅ Port forwarding**
  GUI method: `CreatePortForward()`.

- **✅ Delete (with confirmation)**
  GUI method: `DeleteResource()`. Confirmation handled in the frontend.

- **✅ GitOps integration (Flux/ArgoCD)**
  Backend implemented in `internal/gitops/*` with Flux and ArgoCD providers.
  GUI methods: `GetGitOpsStatus()`, `GetGitOpsApplication()`, `SyncGitOpsApplication()`.
  Auto-detects Flux Kustomizations/HelmReleases and ArgoCD Applications.

- **✅ Security center (RBAC analysis, runtime scanning, network policies)**
  Backend implemented in `internal/security/*` with comprehensive scanning.
  GUI methods: `GetSecuritySummary()`, `GetSecurityIssues()`.
  Scans for: privileged containers, root users, host namespaces, RBAC issues, network policy gaps.
  Provides security score (A-F grade) and categorized issues.

- **✅ Multi-cluster health dashboard**
  GUI methods: `GetMultiClusterHealth()` returns health info for all connected clusters.
  Includes: node count, pod count, issue detection.

- **✅ Cross-cluster search**
  GUI method: `SearchAcrossClusters(kind, query)` searches resources across all connected clusters.

- **✅ Copy resource YAML**
  GUI method: `CopyResourceYAML(kind, namespace, name)` returns YAML for clipboard.

- **✅ Snapshot diff / time-travel**
  GUI methods: `GetSnapshots()`, `GetSnapshotDiff()`, `TakeSnapshot()`.
  Compares cluster state between two points in time.

## Partially implemented / limitations

- **🟡 GitOps sync operations**
  Detection and listing work, but triggering sync requires CLI commands (flux/argocd) due to K8s API patch requirements not yet in client interface.

- **🟡 Explorer "edit resource"**
  Copy YAML is implemented; edit requires kubectl or API patch support.

- **🟡 Custom columns**
  Configuration structure exists but UI for custom column selection not yet implemented.

## Not implemented yet (described in docs)

- **🔄 CVE scanning integration**
  Docs describe Trivy/Grype integration. Security scanner detects runtime issues but doesn't call external CVE scanners yet.

- **🔄 Policy enforcement (Gatekeeper/Kyverno details)**
  Basic policy detection exists but detailed violation reporting needs more work.

## Major documentation mismatches (resolved)

- **Keybinding expectations differ**: docs say `/` opens Query "globally", but `/` is used for filtering/search in multiple views, so Query is intentionally not bound globally.

## Legacy / Archived

An earlier TUI prototype existed at `cmd/nexus/main.go` and `internal/ui/`. This code is **no longer present in the repository**. Kubecat is now exclusively a Wails desktop GUI application. References to the old TUI entrypoint in older documentation or commit history describe that prototype only.
