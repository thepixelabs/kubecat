# Quick Reference Card

## 🚀 Quick Start (30 seconds)

```bash
cd scripts/testdata-generator
./quickstart.sh
# Select option from menu
```

## 📋 Menu Options

| Option | Stack | Resources Created | Use Case |
|--------|-------|-------------------|----------|
| 1 | Production Stack | ~15 resources | General testing |
| 2 | E-Commerce | ~25 resources | Service relationships |
| 3 | Monitoring | ~20 resources | DaemonSets, metrics |
| 4 | Data Pipeline | ~15 resources | StatefulSets, PVCs |
| 5 | Microservices | ~18 resources | Many services |
| 6 | All Stacks | ~90 resources | Full stress test |
| 7 | Cleanup | Removes all | Reset cluster |

## 🎯 Common Commands

### Generate Test Data
```bash
# Interactive mode
go run main.go

# Or use the built binary
make build
./build/testdata-generator
```

### View Resources
```bash
# All namespaces
kubectl get all --all-namespaces

# Specific namespace
kubectl get all -n ecommerce

# With labels
kubectl get pods -n production --show-labels

# Wide output
kubectl get pods -n ecommerce -o wide
```

### Test Visualizer
```bash
# Generate data
go run main.go  # Select option 6

# Run validation
./test-visualizer.sh

# Start Kubecat
cd ../.. && make run
```

### Cleanup
```bash
# From menu
go run main.go  # Select option 7

# Or manually
kubectl delete ns production ecommerce monitoring data-pipeline microservices
```

## 🔍 Verification Commands

```bash
# Count resources
kubectl get all --all-namespaces | wc -l

# Check pod status
kubectl get pods --all-namespaces

# View events
kubectl get events -n production --sort-by='.lastTimestamp'

# Describe resource
kubectl describe deployment frontend-web -n ecommerce
```

## 📊 Resource Breakdown by Stack

### Production Stack
- 3 Deployments
- 2 StatefulSets  
- 2 DaemonSets
- 7 Services
- 2 ConfigMaps
- 3 Secrets
- 1 Ingress
- 2 Jobs

### E-Commerce Stack
- 5 Deployments (frontend, api-gateway, cart, payment, redis)
- 2 StatefulSets (orders, postgres)
- 7 Services
- 1 Ingress
- 1 ConfigMap
- 1 Secret

### Monitoring Stack
- 3 Deployments (grafana, alertmanager, prometheus)
- 2 StatefulSets (prometheus, loki)
- 2 DaemonSets (node-exporter, promtail)
- 5 Services
- 1 ConfigMap

### Data Pipeline
- 1 Deployment (stream-processor)
- 3 StatefulSets (kafka, zookeeper, elasticsearch)
- 4 Services
- 2 Jobs

### Microservices
- 6 Deployments (user, auth, notification, analytics, search, recommendation)
- 6 Services

## 🏷️ Important Labels

All resources include:
- `app`: Resource name
- `app.kubernetes.io/name`: Resource name
- `app.kubernetes.io/managed-by`: testdata-generator
- `app.kubernetes.io/version`: Version (1.0.0, 1.1.0, etc.)
- `team`: Team name (platform, backend, frontend, data)
- `environment`: test

E-Commerce specific:
- `tier`: frontend, backend, data, cache
- `component`: Specific component name
- `compliance`: pci-dss (for payment service)

## 🔧 Troubleshooting

### Cannot connect to cluster
```bash
minikube status
minikube start
kubectl cluster-info
```

### Insufficient resources
```bash
minikube delete
minikube start --cpus=4 --memory=8192
```

### Namespace already exists
```bash
kubectl delete namespace <name>
# Or use cleanup option (7)
```

### Pods not starting
```bash
# Check events
kubectl get events -n <namespace> --sort-by='.lastTimestamp'

# Check pod logs
kubectl logs <pod-name> -n <namespace>

# Describe pod
kubectl describe pod <pod-name> -n <namespace>
```

## 📈 Performance Tips

### Minikube Resource Allocation

| Scenario | Command |
|----------|---------|
| Light (1-2 stacks) | `minikube start --cpus=2 --memory=4096` |
| Medium (3-4 stacks) | `minikube start --cpus=4 --memory=8192` |
| Heavy (all stacks) | `minikube start --cpus=6 --memory=12288` |

### Speed Up Pod Startup
```bash
# Pre-pull common images
minikube ssh docker pull nginx:1.21
minikube ssh docker pull redis:7.0-alpine
minikube ssh docker pull postgres:14-alpine
```

## 🎨 Visualizer Testing Checklist

- [ ] Generate test data (option 6)
- [ ] Verify resources created (`kubectl get all --all-namespaces`)
- [ ] Run validation script (`./test-visualizer.sh`)
- [ ] Start Kubecat application
- [ ] Test cluster selection
- [ ] Test namespace filtering
- [ ] Verify node relationships (Deployment → Service)
- [ ] Check label-based grouping
- [ ] Test metadata drawer
- [ ] Verify path highlighting
- [ ] Check resource details
- [ ] Test search/filter
- [ ] Verify performance with many resources

## 🔗 Useful kubectl Aliases

Add to `~/.bashrc` or `~/.zshrc`:

```bash
alias k='kubectl'
alias kgp='kubectl get pods'
alias kgs='kubectl get services'
alias kgd='kubectl get deployments'
alias kga='kubectl get all'
alias kgn='kubectl get nodes'
alias kdp='kubectl describe pod'
alias kl='kubectl logs'
alias kx='kubectl exec -it'
```

## 📚 Related Files

- `README.md` - Full documentation
- `examples.md` - Detailed examples and scenarios
- `quickstart.sh` - Automated setup script
- `test-visualizer.sh` - Validation script
- `Makefile` - Build automation

## 🆘 Getting Help

```bash
# Check cluster info
kubectl cluster-info

# Get resource details
kubectl explain <resource>

# View API resources
kubectl api-resources

# Check logs
kubectl logs -n <namespace> <pod-name>

# Interactive shell
kubectl exec -it -n <namespace> <pod-name> -- /bin/sh
```

## 💡 Pro Tips

1. **Use watch mode** for real-time updates:
   ```bash
   watch -n 1 kubectl get pods -n ecommerce
   ```

2. **Export resources** for backup:
   ```bash
   kubectl get all -n ecommerce -o yaml > backup.yaml
   ```

3. **Filter by labels**:
   ```bash
   kubectl get pods -n ecommerce -l tier=backend
   ```

4. **Check resource usage** (requires metrics-server):
   ```bash
   kubectl top nodes
   kubectl top pods -n production
   ```

5. **Port forward** for testing:
   ```bash
   kubectl port-forward -n ecommerce svc/frontend-web 8080:80
   ```

---

**Need more help?** Check `README.md` or `examples.md` for detailed documentation.



