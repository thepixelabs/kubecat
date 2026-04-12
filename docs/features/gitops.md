# GitOps Integration

> **Status**: ✅ Available

Kubecat provides native integration with GitOps tools like Flux and ArgoCD.

## Vision

See your GitOps state at a glance:

```
⎈ GitOps Dashboard

Applications                                      Sync Status
────────────────────────────────────────────────────────────────
frontend          main@abc1234    ● Synced        2 mins ago
backend           main@abc1234    ● Synced        2 mins ago
api-gateway       main@def5678    ⚠ OutOfSync     drift detected
monitoring        main@abc1234    ● Synced        2 mins ago
```

## Features

### Application Overview

See all Flux/ArgoCD applications:

| Column | Description |
|--------|-------------|
| Name | Application name |
| Source | Git branch/commit |
| Status | Sync status |
| Health | Application health |
| Last Sync | Time since last sync |

### Sync Status

| Status | Icon | Description |
|--------|------|-------------|
| Synced | ● | Git matches cluster |
| OutOfSync | ⚠ | Drift detected |
| Unknown | ? | Status unavailable |
| Progressing | ↻ | Sync in progress |

### Drift Detection

View what's different between Git and cluster:

```
Drift Analysis: api-gateway

┌─────────────────────────────────────────────────────┐
│ Deployment: api-gateway                             │
├─────────────────────────────────────────────────────┤
│ - replicas: 3                                       │
│ + replicas: 5                                       │
│                                                     │
│ Container: api                                      │
│ - image: api:v1.2.3                                │
│ + image: api:v1.2.4                                │
└─────────────────────────────────────────────────────┘

Press 's' to sync, 'r' to revert
```

### Git History

Link resources to Git commits:

```
Resource: deployment/api-gateway

Git History:
────────────────────────────────────────────────────────
abc1234  2 hours ago   @developer   Bump replicas to 5
def5678  1 day ago     @sre-team    Update image to v1.2.3
ghi9012  3 days ago    @developer   Add readiness probe
```

### Sync Preview

Preview what will change before syncing:

```
Sync Preview: api-gateway

Changes to apply:
  deployment/api-gateway
    - replicas: 5 → 3
    - image: api:v1.2.4 → api:v1.2.3

Resources to create: 0
Resources to update: 1
Resources to delete: 0

Press Enter to confirm sync, Esc to cancel
```

## Supported Tools

### Flux CD

Detect and display:
- Kustomizations
- HelmReleases
- GitRepositories
- Source status

```yaml
# Detection
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
```

### ArgoCD

Detect and display:
- Applications
- ApplicationSets
- AppProjects
- Sync status

```yaml
# Detection
apiVersion: argoproj.io/v1alpha1
kind: Application
```

## Architecture

### GitOps Detection

```go
func DetectGitOpsController(client ClusterClient) GitOpsProvider {
    // Check for Flux
    if hasFluxCRDs(client) {
        return &FluxProvider{client: client}
    }
    
    // Check for ArgoCD
    if hasArgoCDCRDs(client) {
        return &ArgoCDProvider{client: client}
    }
    
    return nil
}
```

### Provider Interface

```go
type GitOpsProvider interface {
    // List all applications
    ListApplications(ctx context.Context) ([]Application, error)
    
    // Get application details
    GetApplication(ctx context.Context, name string) (*Application, error)
    
    // Get drift/diff
    GetDrift(ctx context.Context, name string) (*Drift, error)
    
    // Trigger sync
    Sync(ctx context.Context, name string) error
    
    // Get git history
    GetHistory(ctx context.Context, name string) ([]Commit, error)
}
```

## Configuration

```yaml
# ~/.config/kubecat/config.yaml
kubecat:
  gitops:
    enabled: true
    # Auto-detect or specify
    provider: auto  # auto, flux, argocd
    # Refresh interval
    refreshInterval: 30s
```

