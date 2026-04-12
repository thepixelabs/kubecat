# Test Data Generator - Summary

## 🎉 What Was Created

A comprehensive Kubernetes test data generator for testing your Kubecat visualizer with realistic, production-like resources.

### 📁 Project Structure

```
scripts/testdata-generator/
├── main.go                  # Main generator application (1000+ lines)
├── go.mod                   # Go module dependencies
├── go.sum                   # Dependency checksums (auto-generated)
├── Makefile                 # Build automation
├── README.md                # Complete documentation
├── QUICKREF.md              # Quick reference card
├── examples.md              # Detailed usage examples
├── quickstart.sh            # Automated setup script
├── test-visualizer.sh       # Validation test suite
├── .gitignore               # Git ignore rules
└── build/
    └── testdata-generator   # Compiled binary (49MB)
```

## ✨ Features

### 🎯 5 Pre-built Application Stacks

1. **Production Stack** - General production environment
   - Deployments, StatefulSets, DaemonSets
   - ConfigMaps, Secrets, Services, Ingress, Jobs

2. **E-Commerce Application** - Complete shopping platform
   - Frontend web UI
   - API Gateway
   - Cart Service
   - Order Service (StatefulSet)
   - Payment Service (PCI-compliant)
   - PostgreSQL Database
   - Redis Cache
   - Production-ready ingress configuration

3. **Monitoring Stack** - Full observability platform
   - Prometheus + Grafana
   - Node Exporter (DaemonSet)
   - Loki + Promtail (DaemonSet)
   - AlertManager
   - Pre-configured for metrics collection

4. **Data Processing Pipeline** - Big data infrastructure
   - Kafka cluster (3 replicas)
   - Zookeeper ensemble (3 replicas)
   - Elasticsearch cluster (3 replicas)
   - Batch processing jobs
   - Stream processors

5. **Microservices Architecture** - Service mesh ready
   - 6 independent microservices
   - Service-to-service communication
   - Ready for Istio/Linkerd integration

### 🏷️ Production-Like Features

✅ **Realistic Labels**
- `app`, `app.kubernetes.io/name`, `app.kubernetes.io/version`
- `team`, `environment`, `cost-center`
- `compliance` (PCI, HIPAA, SOX, GDPR)
- `tier` (frontend, backend, data, cache)

✅ **Comprehensive Annotations**
- Prometheus scraping configuration
- Ingress controller settings
- Cert-manager integration
- Deployment revision tracking

✅ **Resource Management**
- CPU and memory requests/limits
- Quality of Service (QoS) classes
- Resource quotas ready

✅ **Health & Reliability**
- Liveness probes (HTTP, TCP, Exec)
- Readiness probes
- Startup probes
- Rolling update strategies

✅ **Security**
- Non-root users
- Read-only root filesystems
- Security contexts
- Pod security standards

✅ **Networking**
- ClusterIP, NodePort, LoadBalancer services
- Ingress with TLS
- Host networking for system pods
- Network policy ready

## 📊 Resource Counts

When you generate all stacks (Option 6):

| Resource Type | Count |
|--------------|-------|
| Namespaces | 5 |
| Deployments | 18 |
| StatefulSets | 9 |
| DaemonSets | 4 |
| Services | 29 |
| Ingresses | 3+ |
| ConfigMaps | 5+ |
| Secrets | 5+ |
| Jobs | 4+ |
| **Total Pods** | **52-80** |

## 🚀 Quick Start

### Prerequisites
- Go 1.24+
- kubectl configured
- Kubernetes cluster (minikube recommended)

### Basic Usage

```bash
# Option 1: Quick start (easiest)
cd scripts/testdata-generator
./quickstart.sh

# Option 2: Manual build and run
cd scripts/testdata-generator
go mod download
go build -o build/testdata-generator
./build/testdata-generator

# Option 3: Direct run
cd scripts/testdata-generator
go run main.go

# Option 4: Using Make
cd scripts/testdata-generator
make build
make run
```

### Generate All Test Data

```bash
go run main.go
# Select option 6 (Generate All)
```

### Validate Generated Resources

```bash
./test-visualizer.sh
```

### Clean Up

```bash
go run main.go
# Select option 7 (Clean Up)
```

## 🎯 Testing Your Visualizer

### Step 1: Generate Test Data
```bash
cd scripts/testdata-generator
go run main.go
# Select option 6 (Generate All)
```

### Step 2: Verify Resources Created
```bash
# Quick check
kubectl get all --all-namespaces

# Detailed validation
./test-visualizer.sh
```

### Step 3: Start Kubecat
```bash
cd ../..
make run
```

### Step 4: What to Test in Visualizer

✅ **Service Relationships**
- Deployments → Services
- Services → Ingress
- StatefulSets → Services → PVCs

✅ **Label-Based Grouping**
- Group by `tier` (frontend, backend, data)
- Group by `team`
- Group by `app`

✅ **Node Distribution**
- DaemonSets on all nodes
- Pod placement across nodes
- Resource utilization

✅ **Health Status**
- Running pods (green)
- Pending pods (yellow)
- Failed pods (red)
- CrashLoopBackOff detection

✅ **Resource Details**
- Metadata display
- Labels and annotations
- Resource limits
- Environment variables

✅ **Network Topology**
- Ingress → Service → Deployment flow
- Service discovery
- Internal vs external services

## 📚 Documentation

- **README.md** - Complete documentation with installation, usage, and features
- **QUICKREF.md** - Quick reference card for common commands
- **examples.md** - Detailed examples and testing scenarios
- **SUMMARY.md** (this file) - Project overview

## 🔧 Available Commands

### Build & Run
```bash
make build          # Build binary
make run            # Run directly
make clean          # Clean build artifacts
make install        # Install dependencies
```

### Testing
```bash
./test-visualizer.sh              # Run validation tests
./quickstart.sh                   # Quick setup and run
kubectl get all --all-namespaces  # View all resources
```

### Kubernetes Commands
```bash
# View specific namespace
kubectl get all -n ecommerce
kubectl get all -n monitoring
kubectl get all -n data-pipeline

# Check labels
kubectl get pods --all-namespaces --show-labels

# Check resource usage
kubectl top nodes
kubectl top pods --all-namespaces

# Cleanup
kubectl delete namespace production ecommerce monitoring data-pipeline microservices
```

## 💡 Use Cases

### 1. Visualizer Development
Test graph rendering with realistic service relationships and varying resource counts.

### 2. Performance Testing
Generate 50-80 pods to test visualizer performance under load.

### 3. Feature Testing
- Test label-based filtering
- Verify metadata display
- Test search functionality
- Validate relationship mapping

### 4. Demo & Presentations
Show realistic production-like Kubernetes environments for demos.

### 5. Learning & Training
Understand Kubernetes resource relationships and best practices.

## 🎨 Generated Resource Examples

### E-Commerce Ingress
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ecommerce-ingress
  namespace: ecommerce
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  rules:
  - host: shop.example.com
    http:
      paths:
      - path: /
        backend:
          service:
            name: frontend-web
            port: 80
      - path: /api
        backend:
          service:
            name: api-gateway
            port: 8080
```

### Deployment with Best Practices
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: payment-service
  namespace: ecommerce
  labels:
    app: payment-service
    tier: backend
    compliance: pci-dss
    team: backend
spec:
  replicas: 2
  template:
    metadata:
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
      containers:
      - name: payment-service
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
```

## 🐛 Troubleshooting

### Build Issues
```bash
# If go.sum is missing
go mod download
go mod tidy

# If build fails
go clean
go build -v
```

### Cluster Connection Issues
```bash
# Check cluster status
kubectl cluster-info
minikube status

# Start minikube
minikube start --cpus=4 --memory=8192
```

### Resource Issues
```bash
# If pods don't start
kubectl describe pod <pod-name> -n <namespace>
kubectl logs <pod-name> -n <namespace>

# Check events
kubectl get events -n <namespace> --sort-by='.lastTimestamp'
```

## 📈 Performance Recommendations

### Minikube Configuration

| Test Scenario | Command |
|--------------|---------|
| Single stack | `minikube start --cpus=2 --memory=4096` |
| Multiple stacks | `minikube start --cpus=4 --memory=8192` |
| All stacks | `minikube start --cpus=6 --memory=12288` |

## 🎯 Next Steps

1. ✅ **Generated** - Test data generator is ready
2. 🔄 **Run** - Execute and generate test data
3. 🧪 **Validate** - Run test-visualizer.sh
4. 🎨 **Test** - Use with your Kubecat visualizer
5. 📊 **Iterate** - Add more scenarios as needed

## 🔗 Integration with Kubecat

The generator creates resources that work seamlessly with Kubecat features:

- **Explorer View** - Navigate through generated resources
- **Cluster Visualizer** - See service relationships
- **Time Travel** - View resource history
- **AI Integration** - Query about resources
- **Port Forwarding** - Connect to services
- **Logs Viewer** - View container logs

## 📝 License

Same as Kubecat project.

---

**Ready to test your visualizer?** 🚀

```bash
cd scripts/testdata-generator
./quickstart.sh
```

Then start Kubecat and explore the generated resources!



