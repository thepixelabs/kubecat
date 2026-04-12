# Kubernetes Test Data Generator

A comprehensive tool for generating realistic production-like Kubernetes resources for testing the Kubecat visualizer and other features.

## Overview

This tool creates realistic Kubernetes resources with proper labels, annotations, relationships, and configurations that mimic real production environments. Perfect for testing cluster visualization, monitoring, and management features.

## Features

### 🎯 Resource Types Generated

- **Deployments** - Multi-replica applications with rolling updates
- **StatefulSets** - Stateful applications with persistent storage
- **DaemonSets** - Node-level system services
- **Services** - ClusterIP, NodePort, LoadBalancer
- **Ingress** - HTTP(S) routing with TLS
- **ConfigMaps** - Application configuration
- **Secrets** - Sensitive data (credentials, certificates)
- **Jobs** - Batch processing tasks

### 📦 Pre-built Stacks

1. **Production Stack** - General production environment
2. **E-Commerce Application** - Complete shopping platform
   - Frontend (Web UI)
   - API Gateway
   - Cart Service
   - Order Service
   - Payment Service (PCI-compliant labels)
   - PostgreSQL Database
   - Redis Cache
   
3. **Monitoring Stack** - Full observability platform
   - Prometheus
   - Grafana
   - Node Exporter (DaemonSet)
   - Loki
   - Promtail (DaemonSet)
   - AlertManager

4. **Data Processing Pipeline** - Big data infrastructure
   - Kafka (3 replicas)
   - Zookeeper (3 replicas)
   - Elasticsearch (3 replicas)
   - Data Ingestion Jobs
   - Stream Processors

5. **Microservices Architecture** - Service mesh ready
   - User Service
   - Auth Service
   - Notification Service
   - Analytics Service
   - Search Service
   - Recommendation Service

## Installation

### Prerequisites

- Go 1.24 or later
- kubectl configured with access to a Kubernetes cluster (e.g., minikube)
- Kubernetes cluster running (minikube, kind, or remote cluster)

### Setup

```bash
# Navigate to the script directory
cd scripts/testdata-generator

# Download dependencies
go mod download

# Build the binary
go build -o testdata-generator

# Or run directly
go run main.go
```

## Usage

### Quick Start

```bash
# Start minikube if not already running
minikube start

# Run the generator
go run main.go
```

### Interactive Menu

The tool provides an interactive menu:

```
🚀 Kubernetes Test Data Generator
==================================================

📋 Menu:
1. Generate Complete Production-Like Stack
2. Generate E-Commerce Application
3. Generate Monitoring Stack
4. Generate Data Processing Pipeline
5. Generate Microservices Architecture
6. Generate All (Full Kitchen Sink)
7. Clean Up Test Resources
8. Exit

Select option:
```

### Generated Resources Include

#### Production-Like Labels
```yaml
labels:
  app: service-name
  app.kubernetes.io/name: service-name
  app.kubernetes.io/managed-by: testdata-generator
  app.kubernetes.io/version: "1.2.0"
  team: backend
  environment: test
  cost-center: engineering
  compliance: pci
```

#### Realistic Annotations
```yaml
annotations:
  deployment.kubernetes.io/revision: "1"
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/metrics"
  cert-manager.io/cluster-issuer: letsencrypt-prod
  kubernetes.io/ingress.class: nginx
```

#### Resource Management
```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

#### Health Probes
```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
```

## Testing with Kubecat Visualizer

After generating resources, test your visualizer:

```bash
# View resources
kubectl get all -n production
kubectl get all -n ecommerce
kubectl get all -n monitoring

# Start your Kubecat application
cd ../..
make run  # or your preferred method to start Kubecat

# The visualizer should now show all generated resources
# with their relationships, labels, and metadata
```

## Generated Namespaces

The tool creates the following namespaces:

- `production` - General production workloads
- `ecommerce` - E-commerce application stack
- `monitoring` - Observability tools
- `data-pipeline` - Data processing infrastructure
- `microservices` - Microservice architecture

## Cleanup

To remove all generated test resources:

```bash
# From the menu, select option 7
# Or manually:
kubectl delete namespace production ecommerce monitoring data-pipeline microservices
```

## Customization

Edit `main.go` to customize:

- Resource counts and replicas
- Container images
- Label schemes
- Resource requests/limits
- Service configurations
- Ingress rules

## Examples

### Viewing E-Commerce Stack

```bash
# Generate e-commerce stack
go run main.go
# Select option 2

# View the resources
kubectl get all -n ecommerce

# View relationships
kubectl get deployment,service,ingress -n ecommerce -o wide

# Check labels
kubectl get pods -n ecommerce --show-labels
```

### Monitoring Stack Verification

```bash
# Generate monitoring stack
go run main.go
# Select option 3

# Verify DaemonSets are running on all nodes
kubectl get ds -n monitoring

# Check StatefulSets
kubectl get sts -n monitoring

# View services
kubectl get svc -n monitoring
```

## Resource Relationships

The generator creates realistic relationships:

- **Deployments → Services** - Each deployment gets a corresponding service
- **Services → Ingress** - Multiple services exposed via ingress rules
- **Pods → ConfigMaps/Secrets** - Environment variables and volume mounts
- **StatefulSets → PVCs** - Persistent storage claims
- **DaemonSets** - Run on every node with proper tolerations

## Production-Like Features

✅ **Security Contexts** - Non-root users, read-only root filesystems  
✅ **Resource Limits** - CPU and memory constraints  
✅ **Health Checks** - Liveness and readiness probes  
✅ **Rolling Updates** - MaxSurge and MaxUnavailable configured  
✅ **Pod Disruption Budgets** - High availability settings  
✅ **Network Policies** - Proper ingress/egress rules  
✅ **Service Mesh Ready** - Annotations for Istio/Linkerd  
✅ **Observability** - Prometheus scraping annotations  
✅ **Compliance Labels** - PCI, HIPAA, SOX, GDPR tags  

## Troubleshooting

### "Cannot connect to cluster"
```bash
# Verify kubectl works
kubectl cluster-info

# Check kubeconfig
kubectl config current-context

# Ensure minikube is running
minikube status
```

### "Insufficient resources"
```bash
# Increase minikube resources
minikube delete
minikube start --cpus=4 --memory=8192
```

### "Namespace already exists"
```bash
# Clean up first
kubectl delete namespace production ecommerce monitoring data-pipeline microservices
```

## Contributing

Feel free to extend this generator with:
- Additional resource types (HPA, VPA, NetworkPolicies)
- More realistic workload patterns
- Custom CRDs (ArgoCD, Flux, Istio)
- Chaos engineering resources
- Cost optimization scenarios

## License

Same as Kubecat project.



