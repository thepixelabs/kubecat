# Resource Explorer

The Resource Explorer provides a powerful interface for browsing and managing Kubernetes resources.

## Overview

Access the explorer by pressing `2` from any view (requires connected cluster).

```
po   deploy   svc   cm   secret   ns   no  ←/→ to change
Namespace: all namespaces  (Tab to cycle)

NAMESPACE          NAME                    STATUS     AGE
─────────────────────────────────────────────────────────
default            nginx-abc123            Running     2d
default            api-server-xyz789       Running     5h
kube-system        coredns-abcdef          Running    30d
kube-system        etcd-master             Running    30d

4 pods
```

## Navigation

### Resource Types

Switch between resource types using `←` and `→`:

| Short | Resource |
|-------|----------|
| po | Pods |
| deploy | Deployments |
| svc | Services |
| cm | ConfigMaps |
| secret | Secrets |
| ns | Namespaces |
| no | Nodes |
| ing | Ingresses |
| sts | StatefulSets |
| ds | DaemonSets |
| job | Jobs |
| cj | CronJobs |
| pvc | PersistentVolumeClaims |
| sa | ServiceAccounts |
| ev | Events |

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `←/→` | Switch resource type |
| `↑/↓` or `j/k` | Navigate rows |
| `Tab` | Cycle namespaces |
| `/` | Open filter |
| `Enter` | View resource details |
| `d` | Describe resource |
| `e` | Edit resource (planned) |
| `l` | View logs (pods only) |
| `s` | Shell into container |
| `f` | Port forward |
| `Ctrl+D` | Delete resource |
| `r` | Refresh |

## Filtering

Press `/` to open the filter input:

```
Filter resources...: nginx

NAMESPACE          NAME                    STATUS     AGE
─────────────────────────────────────────────────────────
default            nginx-abc123            Running     2d
web                nginx-frontend          Running     1d

2 pods (filtered, showing 2)
```

The filter searches across:
- Resource name
- Namespace
- Status

Press `Esc` to clear the filter.

## Namespace Selection

Press `Tab` to cycle through namespaces:

1. All namespaces
2. default
3. kube-system
4. (other namespaces...)

## Status Colors

Resources are color-coded by status:

| Color | Statuses |
|-------|----------|
| Green | Running, Active, Bound, Ready |
| Yellow | Pending, ContainerCreating |
| Red | Failed, Error, CrashLoopBackOff, ImagePullBackOff |
| Gray | Terminating, Completed |

## Table Columns

Default columns shown:

| Column | Description |
|--------|-------------|
| NAMESPACE | Resource namespace |
| NAME | Resource name |
| STATUS | Current status |
| AGE | Time since creation |

Age is displayed in human-readable format:
- `30s` - seconds
- `5m` - minutes
- `2h` - hours
- `3d` - days
- `2w` - weeks
- `1mo` - months
- `1y` - years

## Architecture

### Explorer Component

```go
type Explorer struct {
    theme      *theme.Theme
    table      *components.Table
    manager    clientpkg.Manager
    kind       string           // Current resource kind
    namespace  string           // Filter namespace
    resources  []Resource       // Loaded resources
    filter     textinput.Model  // Search filter
}
```

### Resource Loading

Resources are loaded asynchronously:

```go
func (e *Explorer) loadResources() tea.Cmd {
    return func() tea.Msg {
        client, _ := e.manager.Active()
        list, err := client.List(ctx, e.kind, ListOptions{
            Namespace: e.namespace,
            Limit:     1000,
        })
        return ExplorerMsg{Resources: list.Items, Error: err}
    }
}
```

## Implemented Features

### Resource Details
Press `Enter` or `d` to view full resource description with syntax highlighting.

### Quick Actions

| Key | Action | Status |
|-----|--------|--------|
| `d` | Describe resource | ✅ Implemented |
| `e` | Edit resource | 🔄 Planned |
| `y` | Copy YAML to clipboard | 🔄 Planned |
| `l` | Stream logs | ✅ Implemented |
| `s` | Shell into container | ✅ Implemented |
| `f` | Port forward | ✅ Implemented |
| `Ctrl+D` | Delete resource | ✅ Implemented |

## Planned Features

### Custom Columns
Configure visible columns per resource type:

```yaml
# ~/.config/kubecat/views.yaml
views:
  pods:
    columns:
      - NAMESPACE
      - NAME
      - READY
      - STATUS
      - RESTARTS
      - AGE
      - NODE
```

