# Multi-Cluster Management

> **Status**: ✅ Implemented

Kubecat is designed from the ground up to manage multiple Kubernetes clusters simultaneously.

## Overview

Unlike tools that treat multi-cluster as an afterthought, Kubecat provides:

- **Unified Context Management**: Connect to multiple clusters from your kubeconfig ✅
- **Quick Switching**: Switch active cluster without reconnecting ✅
- **Dashboard Display**: Connected clusters shown in dashboard with active indicator ✅
- **Parallel Operations**: Query resources across clusters (planned)
- **Health Dashboard**: Overview of all cluster health (planned)

## Connecting to Clusters

### Opening Cluster Configuration

Press `c` from any view to open the cluster configuration screen.

```
⎈ Cluster Configuration

Select a context and press Enter to connect

┌────────────────────────────────────────────────────┐
│ Kubernetes Contexts                                │
├────────────────────────────────────────────────────┤
│ > production-cluster     ● Connected (active)     │
│   staging-cluster        ○ Not connected          │
│   dev-local              ○ Not connected          │
│   minikube               ○ Not connected          │
└────────────────────────────────────────────────────┘

Enter connect  a set active  d disconnect  Esc back
```

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑/↓` | Navigate contexts |
| `Enter` | Connect to selected context |
| `a` | Set as active cluster |
| `d` | Disconnect from cluster |
| `/` | Filter contexts |
| `Esc` | Return to previous view |

## Kubeconfig Support

Kubecat reads your standard kubeconfig file:

```bash
# Default location
~/.kube/config

# Or from environment variable
export KUBECONFIG=/path/to/config
```

### Multiple Kubeconfig Files

If `KUBECONFIG` contains multiple paths, Kubecat uses the first one:

```bash
export KUBECONFIG=~/.kube/config:~/.kube/other-config
```

## Connection States

| State | Icon | Description |
|-------|------|-------------|
| Not connected | ○ | Context available but not connected |
| Connected | ● | Active connection established |
| Active | ● (active) | Currently selected for operations |
| Error | ✗ | Connection failed |

## Header Display

The header shows the current cluster status:

```
⎈ Kubecat  ○ no cluster (press c)                    Dashboard
─────────────────────────────────────────────────────────────
```

When connected:

```
⎈ Kubecat  ● production-cluster                      Dashboard
─────────────────────────────────────────────────────────────
```

## Architecture

### Cluster Client Interface

```go
type ClusterClient interface {
    Info(ctx context.Context) (*ClusterInfo, error)
    List(ctx context.Context, kind string, opts ListOptions) (*ResourceList, error)
    Get(ctx context.Context, kind, namespace, name string) (*Resource, error)
    Delete(ctx context.Context, kind, namespace, name string) error
    Watch(ctx context.Context, kind string, opts WatchOptions) (<-chan WatchEvent, error)
    Logs(ctx context.Context, namespace, pod, container string, follow bool, tailLines int64) (<-chan string, error)
    Close() error
}
```

### Manager Interface

```go
type Manager interface {
    Add(ctx context.Context, contextName string) error
    Remove(contextName string) error
    Get(contextName string) (ClusterClient, error)
    Active() (ClusterClient, error)
    SetActive(contextName string) error
    List() []ClusterInfo
    Contexts() ([]string, error)
    Close() error
}
```

## Planned Features

### Cross-Cluster Search
Query resources across all connected clusters:

```
> pods nginx

Cluster: production-cluster
  default/nginx-abc123    Running
  web/nginx-frontend      Running

Cluster: staging-cluster
  default/nginx-test      Running
```

### Cluster Comparison
Compare configurations between clusters:

```
Comparing: production-cluster ↔ staging-cluster

Deployments with differences:
  web/api-server
    - Replicas: 5 vs 2
    - Image: v1.2.3 vs v1.2.1
```

### Health Dashboard
Unified health view:

```
┌─────────────────┬─────────────────┬─────────────────┐
│   production    │    staging      │      dev        │
│   ● Healthy     │   ⚠ Warning     │   ● Healthy     │
│                 │                 │                 │
│   Nodes: 12     │   Nodes: 4      │   Nodes: 1      │
│   Pods: 234     │   Pods: 56      │   Pods: 23      │
│   CPU: 45%      │   CPU: 78%      │   CPU: 12%      │
└─────────────────┴─────────────────┴─────────────────┘
```

