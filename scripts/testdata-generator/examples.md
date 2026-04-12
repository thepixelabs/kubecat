# Test Data Generator Examples

This document provides practical examples and use cases for the Kubernetes Test Data Generator.

## Table of Contents

1. [Quick Start Examples](#quick-start-examples)
2. [Testing Visualizer Features](#testing-visualizer-features)
3. [Stress Testing](#stress-testing)
4. [Production Scenarios](#production-scenarios)
5. [Troubleshooting Scenarios](#troubleshooting-scenarios)

---

## Quick Start Examples

### Example 1: Basic Setup with Minikube

```bash
# Start minikube with adequate resources
minikube start --cpus=4 --memory=8192 --disk-size=20g

# Navigate to the generator
cd scripts/testdata-generator

# Run the quick start script
./quickstart.sh

# Select option 1 (Production Stack)
# This creates a basic production-like environment
```

### Example 2: Generate All Stacks

```bash
# Run the generator
go run main.go

# Select option 6 (Generate All)
# This creates all available stacks across multiple namespaces

# Verify resources
kubectl get namespaces
kubectl get all --all-namespaces
```

---

## Testing Visualizer Features

### Test 1: Service Dependencies

Generate the e-commerce stack to test service relationship visualization:

```bash
# Generate e-commerce stack
go run main.go
# Select option 2

# View the relationships
kubectl get all -n ecommerce

# Expected resources:
# - frontend-web (Deployment) → frontend-web (Service)
# - api-gateway (Deployment) → api-gateway (Service)
# - cart-service (Deployment) → cart-service (Service)
# - order-service (StatefulSet) → order-service (Service)
# - payment-service (Deployment) → payment-service (Service)
# - postgres-db (StatefulSet) → postgres-db (Service)
# - redis-cache (Deployment) → redis-cache (Service)

# Test ingress routing
kubectl get ingress -n ecommerce -o yaml
```

**What to verify in visualizer:**

- All services are connected to their deployments/statefulsets
- Ingress routes to frontend-web and api-gateway
- ConfigMap and Secret connections to pods
- Label-based grouping (tier: frontend, backend, data, cache)

### Test 2: Multi-Tier Architecture

```bash
# Generate e-commerce stack
go run main.go
# Select option 2

# Check tier labels
kubectl get pods -n ecommerce -L tier

# Expected tiers:
# - frontend: frontend-web
# - backend: api-gateway, cart-service, order-service, payment-service
# - data: postgres-db
# - cache: redis-cache
```

**What to verify in visualizer:**

- Resources grouped by tier
- Visual separation between frontend, backend, data layers
- Traffic flow from frontend → backend → data

### Test 3: StatefulSet Visualization

```bash
# Generate data pipeline
go run main.go
# Select option 4

# View StatefulSets
kubectl get statefulsets -n data-pipeline
kubectl get pvc -n data-pipeline

# Expected:
# - kafka (3 replicas with PVCs)
# - zookeeper (3 replicas with PVCs)
# - elasticsearch (3 replicas with PVCs)
```

**What to verify in visualizer:**

- StatefulSet pods with ordinal indices (kafka-0, kafka-1, kafka-2)
- PVC attachments to each pod
- Headless service for StatefulSets

### Test 4: DaemonSet on All Nodes

```bash
# Generate monitoring stack
go run main.go
# Select option 3

# View DaemonSets
kubectl get daemonsets -n monitoring
kubectl get pods -n monitoring -o wide

# Expected:
# - node-exporter running on every node
# - promtail running on every node
```

**What to verify in visualizer:**

- DaemonSet pods distributed across all nodes
- Host network mode indicators
- Privileged container badges

### Test 5: Ingress and External Access

```bash
# Generate e-commerce stack
go run main.go
# Select option 2

# View ingress configuration
kubectl describe ingress ecommerce-ingress -n ecommerce

# Expected rules:
# - shop.example.com/ → frontend-web:80
# - shop.example.com/api → api-gateway:8080
```

**What to verify in visualizer:**

- Ingress as entry point
- Multiple backend services
- TLS certificate references
- Annotations (nginx, cert-manager)

---

## Stress Testing

### Test 1: High Resource Count

Create a custom scenario with many resources:

```bash
# Generate all stacks
go run main.go
# Select option 6

# Count total resources
kubectl get all --all-namespaces | wc -l

# Expected: 100+ resources across 5 namespaces
```

**What to verify in visualizer:**

- Performance with 100+ resources
- Smooth rendering and interactions
- Search/filter functionality
- Zoom and pan performance

### Test 2: Rapid Updates

```bash
# Generate production stack
go run main.go
# Select option 1

# Scale deployments up and down
kubectl scale deployment app-deployment-1 --replicas=10 -n production
kubectl scale deployment app-deployment-2 --replicas=15 -n production
kubectl scale deployment app-deployment-3 --replicas=5 -n production

# Watch the changes
kubectl get pods -n production -w
```

**What to verify in visualizer:**

- Real-time updates as pods scale
- Smooth animations
- No memory leaks during updates

---

## Production Scenarios

### Scenario 1: E-Commerce Black Friday

Simulate high-traffic e-commerce scenario:

```bash
# Generate e-commerce stack
go run main.go
# Select option 2

# Scale for high traffic
kubectl scale deployment frontend-web --replicas=10 -n ecommerce
kubectl scale deployment api-gateway --replicas=8 -n ecommerce
kubectl scale deployment cart-service --replicas=12 -n ecommerce
kubectl scale deployment payment-service --replicas=6 -n ecommerce

# Add resource pressure
kubectl set resources deployment frontend-web \
  --requests=cpu=500m,memory=512Mi \
  --limits=cpu=2000m,memory=2Gi \
  -n ecommerce
```

**What to verify in visualizer:**

- Resource utilization indicators
- Scaling status
- Health check status (green/yellow/red)

### Scenario 2: Database Migration

Simulate a database migration scenario:

```bash
# Generate e-commerce stack
go run main.go
# Select option 2

# Create a migration job
kubectl create job db-migration \
  --image=postgres:14-alpine \
  --namespace=ecommerce \
  -- psql -h postgres-db -U admin -c "SELECT version();"

# Watch job completion
kubectl get jobs -n ecommerce -w
```

**What to verify in visualizer:**

- Job lifecycle (Pending → Running → Completed)
- Job-to-database relationship
- Completion status

### Scenario 3: Rolling Update

Simulate a deployment update:

```bash
# Generate production stack
go run main.go
# Select option 1

# Update deployment image
kubectl set image deployment/app-deployment-1 \
  app-deployment-1=nginx:1.22 \
  -n production

# Watch rollout
kubectl rollout status deployment/app-deployment-1 -n production
```

**What to verify in visualizer:**

- Rolling update progress
- Old vs new ReplicaSets
- Pod replacement animation

---

## Troubleshooting Scenarios

### Scenario 1: Pod Crash Loop

```bash
# Generate production stack
go run main.go
# Select option 1

# Create a failing pod
kubectl run failing-pod \
  --image=busybox \
  --namespace=production \
  --restart=Always \
  -- /bin/sh -c "exit 1"

# Watch it crash
kubectl get pods -n production -w
```

**What to verify in visualizer:**

- CrashLoopBackOff status indicator
- Restart count
- Error highlighting

### Scenario 2: ImagePullBackOff

```bash
# Generate production stack
go run main.go
# Select option 1

# Create pod with invalid image
kubectl run invalid-image \
  --image=nonexistent/image:latest \
  --namespace=production

# Check status
kubectl get pods -n production
```

**What to verify in visualizer:**

- ImagePullBackOff status
- Error message display
- Visual differentiation from healthy pods

### Scenario 3: Resource Limits Exceeded

```bash
# Generate production stack
go run main.go
# Select option 1

# Create pod with impossible resource requests
kubectl run resource-hog \
  --image=nginx \
  --namespace=production \
  --requests=cpu=1000,memory=1000Gi

# Check status
kubectl get pods -n production
```

**What to verify in visualizer:**

- Pending status with reason
- Resource constraint indicator
- Node capacity visualization

---

## Cleanup Examples

### Clean Specific Namespace

```bash
# Delete only e-commerce namespace
kubectl delete namespace ecommerce

# Verify deletion
kubectl get namespaces
```

### Clean All Test Data

```bash
# Use built-in cleanup
go run main.go
# Select option 7

# Or manually
kubectl delete namespace production ecommerce monitoring data-pipeline microservices
```

### Selective Cleanup

```bash
# Delete only deployments in production
kubectl delete deployments --all -n production

# Delete only jobs
kubectl delete jobs --all -n production

# Keep namespace but remove all resources
kubectl delete all --all -n production
```

---

## Advanced Examples

### Custom Resource Combinations

```bash
# Generate base stack
go run main.go
# Select option 1

# Add custom resources
kubectl apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: external-api
  namespace: production
spec:
  type: ExternalName
  externalName: api.external-service.com
EOF

# Add network policy
kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-all
  namespace: production
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
EOF
```

### Monitoring Integration Test

```bash
# Generate monitoring stack
go run main.go
# Select option 3

# Port-forward to Grafana
kubectl port-forward -n monitoring svc/grafana 3000:3000

# Access Grafana at http://localhost:3000
# Default credentials: admin/admin

# Port-forward to Prometheus
kubectl port-forward -n monitoring svc/prometheus 9090:9090

# Access Prometheus at http://localhost:9090
```

### Load Testing

```bash
# Generate e-commerce stack
go run main.go
# Select option 2

# Install hey (HTTP load generator)
# macOS: brew install hey
# Linux: go install github.com/rakyll/hey@latest

# Port-forward to frontend
kubectl port-forward -n ecommerce svc/frontend-web 8080:80

# Generate load
hey -z 60s -c 50 http://localhost:8080/
```

---

## Tips and Tricks

### Quick Namespace Switch

```bash
# Set default namespace
kubectl config set-context --current --namespace=ecommerce

# Now all commands default to ecommerce namespace
kubectl get pods
kubectl get services
```

### Watch All Resources

```bash
# Watch all resources in namespace
watch -n 1 kubectl get all -n production

# Or use kubectl built-in watch
kubectl get all -n production -w
```

### Export Resource Definitions

```bash
# Export all resources for backup
kubectl get all -n ecommerce -o yaml > ecommerce-backup.yaml

# Restore later
kubectl apply -f ecommerce-backup.yaml
```

### Resource Metrics

```bash
# Enable metrics-server in minikube
minikube addons enable metrics-server

# View resource usage
kubectl top nodes
kubectl top pods -n production
```

---

## Integration with Kubecat

### Test Complete Workflow

```bash
# 1. Generate test data
cd scripts/testdata-generator
go run main.go
# Select option 6 (Generate All)

# 2. Start Kubecat
cd ../..
make run

# 3. In Kubecat TUI:
#    - Navigate to Explorer view
#    - Select different namespaces
#    - View resource relationships
#    - Check metadata drawer
#    - Test path highlighting
#    - Verify labels and annotations

# 4. Test specific features:
#    - Time travel (view resource history)
#    - AI queries (ask about resources)
#    - Port forwarding
#    - Log viewing
#    - Resource describe
```

---

## Performance Benchmarks

### Expected Resource Counts

| Stack          | Namespaces | Deployments | StatefulSets | DaemonSets | Services | Pods (approx) |
| -------------- | ---------- | ----------- | ------------ | ---------- | -------- | ------------- |
| Production     | 1          | 3           | 2            | 2          | 7        | 10-15         |
| E-Commerce     | 1          | 5           | 2            | 0          | 7        | 12-18         |
| Monitoring     | 1          | 3           | 2            | 2          | 5        | 8-12          |
| Data Pipeline  | 1          | 1           | 3            | 0          | 4        | 10-15         |
| Microservices  | 1          | 6           | 0            | 0          | 6        | 12-20         |
| **All Stacks** | **5**      | **18**      | **9**        | **4**      | **29**   | **52-80**     |

### Minikube Resource Requirements

| Scenario        | CPUs | Memory | Disk |
| --------------- | ---- | ------ | ---- |
| Single Stack    | 2    | 4GB    | 10GB |
| All Stacks      | 4    | 8GB    | 20GB |
| With Monitoring | 4    | 10GB   | 25GB |
| Stress Test     | 6    | 12GB   | 30GB |

---

## Troubleshooting

### Common Issues

**Issue: "Cannot connect to cluster"**

```bash
# Solution: Start minikube
minikube start

# Or check kubeconfig
kubectl config view
kubectl config use-context minikube
```

**Issue: "Insufficient resources"**

```bash
# Solution: Increase minikube resources
minikube delete
minikube start --cpus=4 --memory=8192
```

**Issue: "Namespace already exists"**

```bash
# Solution: Clean up first
kubectl delete namespace production
# Or use cleanup option in menu
```

**Issue: "Image pull errors"**

```bash
# Solution: Use minikube's Docker daemon
eval $(minikube docker-env)

# Or pull images manually
minikube ssh docker pull nginx:1.21
```

---

## Next Steps

1. **Extend the generator** - Add more resource types (HPA, VPA, NetworkPolicies)
2. **Custom scenarios** - Create your own application stacks
3. **Chaos testing** - Add failure injection scenarios
4. **GitOps integration** - Generate ArgoCD/Flux resources
5. **Service mesh** - Add Istio/Linkerd configurations

Happy testing! 🚀


