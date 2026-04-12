# Quick Start

Get up and running with Kubecat in under 5 minutes.

## Prerequisites

1. kubectl configured and working
2. Access to at least one Kubernetes cluster
3. Terminal with 256 color support

Verify kubectl works:
```bash
kubectl cluster-info
```

## Starting Kubecat

```bash
kubecat
```

You'll see the Dashboard view:

```
⎈ Kubecat  ○ no cluster (press c)                    Dashboard
─────────────────────────────────────────────────────────────

  Multi-Cluster Dashboard
  Overview of all connected Kubernetes clusters


  No clusters connected yet.
  Press 'c' to configure clusters.

─────────────────────────────────────────────────────────────
c Clusters  1 Dashboard  2 Explorer  3 Timeline  q Quit
```

## Connecting to a Cluster

1. Press `c` to open cluster configuration
2. Use `↑/↓` to select your cluster context
3. Press `Enter` to connect

```
⎈ Cluster Configuration

Select a context and press Enter to connect

┌────────────────────────────────────────────────────┐
│ Kubernetes Contexts                                │
├────────────────────────────────────────────────────┤
│ > minikube               ○ Not connected          │
│   docker-desktop         ○ Not connected          │
│   production             ○ Not connected          │
└────────────────────────────────────────────────────┘
```

After connecting, the header shows your cluster:

```
⎈ Kubecat  ● minikube                                Dashboard
```

4. Press `Esc` to return to Dashboard

## Exploring Resources

1. Press `2` to open the Resource Explorer
2. Use `←/→` to switch resource types (pods, deployments, etc.)
3. Use `↑/↓` to navigate resources
4. Press `Tab` to filter by namespace
5. Press `/` to search

```
po   deploy   svc   cm   secret   ns   no  ←/→ to change
Namespace: all namespaces  (Tab to cycle)

NAMESPACE     NAME                         STATUS     AGE
─────────────────────────────────────────────────────────────
default       nginx-deployment-abc123      Running    2d
kube-system   coredns-xyz789               Running   30d
kube-system   etcd-minikube                Running   30d

3 pods
```

## Essential Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `c` | Configure clusters |
| `1` | Dashboard view |
| `2` | Resource Explorer |
| `q` | Quit |
| `?` | Help |
| `Esc` | Go back |

## Navigation (Vim-style)

| Key | Action |
|-----|--------|
| `j` or `↓` | Move down |
| `k` or `↑` | Move up |
| `h` or `←` | Move left |
| `l` or `→` | Move right |
| `g` | Go to top |
| `G` | Go to bottom |

## Next Steps

- [Configuration Guide](./configuration.md) - Customize Kubecat
- [Resource Explorer](../features/explorer.md) - Deep dive into browsing resources
- [Keyboard Reference](../reference/keybindings.md) - All shortcuts

## Troubleshooting

### "no cluster (press c)" after connecting

Make sure your kubeconfig is valid:
```bash
kubectl config get-contexts
kubectl cluster-info
```

### Slow connection

Check network connectivity to the cluster:
```bash
kubectl get nodes --request-timeout=5s
```

### Colors look wrong

Ensure your terminal supports 256 colors:
```bash
echo $TERM
# Should be xterm-256color or similar
```

Set it if needed:
```bash
export TERM=xterm-256color
```

