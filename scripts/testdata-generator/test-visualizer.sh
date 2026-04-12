#!/bin/bash

# Test script for validating the visualizer with generated data
# This script generates resources and validates they were created correctly

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
TESTS_PASSED=0
TESTS_FAILED=0

# Helper functions
log_info() {
    echo -e "${BLUE}ℹ ${NC}$1"
}

log_success() {
    echo -e "${GREEN}✓${NC} $1"
    ((TESTS_PASSED++))
}

log_error() {
    echo -e "${RED}✗${NC} $1"
    ((TESTS_FAILED++))
}

log_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

test_resource_count() {
    local resource=$1
    local namespace=$2
    local expected_min=$3
    local count=$(kubectl get $resource -n $namespace --no-headers 2>/dev/null | wc -l | tr -d ' ')
    
    if [ "$count" -ge "$expected_min" ]; then
        log_success "$resource in $namespace: $count (expected >= $expected_min)"
        return 0
    else
        log_error "$resource in $namespace: $count (expected >= $expected_min)"
        return 1
    fi
}

test_resource_exists() {
    local resource=$1
    local name=$2
    local namespace=$3
    
    if kubectl get $resource $name -n $namespace &>/dev/null; then
        log_success "$resource/$name exists in $namespace"
        return 0
    else
        log_error "$resource/$name not found in $namespace"
        return 1
    fi
}

test_labels_exist() {
    local resource=$1
    local name=$2
    local namespace=$3
    local label=$4
    
    local value=$(kubectl get $resource $name -n $namespace -o jsonpath="{.metadata.labels.$label}" 2>/dev/null)
    
    if [ -n "$value" ]; then
        log_success "$resource/$name has label $label=$value"
        return 0
    else
        log_error "$resource/$name missing label $label"
        return 1
    fi
}

test_pod_running() {
    local namespace=$1
    local label=$2
    
    local running=$(kubectl get pods -n $namespace -l $label --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l | tr -d ' ')
    
    if [ "$running" -gt 0 ]; then
        log_success "Pods with label $label are running in $namespace ($running pods)"
        return 0
    else
        log_warning "No running pods with label $label in $namespace"
        return 1
    fi
}

# Main test execution
echo "╔════════════════════════════════════════════════════════════╗"
echo "║     Kubernetes Test Data Generator - Visualizer Tests     ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# Check prerequisites
log_info "Checking prerequisites..."

if ! command -v kubectl &> /dev/null; then
    log_error "kubectl is not installed"
    exit 1
fi

if ! kubectl cluster-info &> /dev/null; then
    log_error "Cannot connect to Kubernetes cluster"
    exit 1
fi

log_success "Prerequisites OK"
echo ""

# Test 1: Production Stack
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test Suite 1: Production Stack"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if kubectl get namespace production &>/dev/null; then
    log_info "Testing production namespace..."
    
    test_resource_count "deployments" "production" 1
    test_resource_count "statefulsets" "production" 1
    test_resource_count "daemonsets" "production" 1
    test_resource_count "services" "production" 1
    test_resource_count "configmaps" "production" 1
    test_resource_count "secrets" "production" 1
    
    # Test labels
    deployments=$(kubectl get deployments -n production -o name 2>/dev/null | head -n 1 | cut -d'/' -f2)
    if [ -n "$deployments" ]; then
        test_labels_exist "deployment" "$deployments" "production" "app"
        test_labels_exist "deployment" "$deployments" "production" "team"
    fi
else
    log_warning "Production namespace not found - skipping tests"
fi

echo ""

# Test 2: E-Commerce Stack
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test Suite 2: E-Commerce Application"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if kubectl get namespace ecommerce &>/dev/null; then
    log_info "Testing ecommerce namespace..."
    
    # Test specific services
    test_resource_exists "deployment" "frontend-web" "ecommerce"
    test_resource_exists "deployment" "api-gateway" "ecommerce"
    test_resource_exists "deployment" "cart-service" "ecommerce"
    test_resource_exists "statefulset" "order-service" "ecommerce"
    test_resource_exists "deployment" "payment-service" "ecommerce"
    test_resource_exists "statefulset" "postgres-db" "ecommerce"
    test_resource_exists "deployment" "redis-cache" "ecommerce"
    
    # Test services
    test_resource_exists "service" "frontend-web" "ecommerce"
    test_resource_exists "service" "api-gateway" "ecommerce"
    
    # Test ingress
    test_resource_exists "ingress" "ecommerce-ingress" "ecommerce"
    
    # Test tier labels
    test_labels_exist "deployment" "frontend-web" "ecommerce" "tier"
    test_labels_exist "deployment" "api-gateway" "ecommerce" "tier"
    
    # Test compliance labels
    test_labels_exist "deployment" "payment-service" "ecommerce" "compliance"
else
    log_warning "E-commerce namespace not found - skipping tests"
fi

echo ""

# Test 3: Monitoring Stack
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test Suite 3: Monitoring Stack"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if kubectl get namespace monitoring &>/dev/null; then
    log_info "Testing monitoring namespace..."
    
    test_resource_exists "statefulset" "prometheus" "monitoring"
    test_resource_exists "deployment" "grafana" "monitoring"
    test_resource_exists "daemonset" "node-exporter" "monitoring"
    test_resource_exists "statefulset" "loki" "monitoring"
    test_resource_exists "daemonset" "promtail" "monitoring"
    test_resource_exists "deployment" "alertmanager" "monitoring"
    
    # Test DaemonSet distribution
    local node_count=$(kubectl get nodes --no-headers | wc -l | tr -d ' ')
    local exporter_count=$(kubectl get pods -n monitoring -l app=node-exporter --no-headers | wc -l | tr -d ' ')
    
    if [ "$exporter_count" -eq "$node_count" ]; then
        log_success "node-exporter running on all $node_count nodes"
    else
        log_warning "node-exporter: $exporter_count pods, expected $node_count (one per node)"
    fi
else
    log_warning "Monitoring namespace not found - skipping tests"
fi

echo ""

# Test 4: Data Pipeline
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test Suite 4: Data Processing Pipeline"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if kubectl get namespace data-pipeline &>/dev/null; then
    log_info "Testing data-pipeline namespace..."
    
    test_resource_exists "statefulset" "kafka" "data-pipeline"
    test_resource_exists "statefulset" "zookeeper" "data-pipeline"
    test_resource_exists "statefulset" "elasticsearch" "data-pipeline"
    
    # Test StatefulSet replicas
    local kafka_replicas=$(kubectl get statefulset kafka -n data-pipeline -o jsonpath='{.spec.replicas}' 2>/dev/null)
    if [ "$kafka_replicas" -ge 3 ]; then
        log_success "Kafka has $kafka_replicas replicas (expected >= 3)"
    else
        log_warning "Kafka has $kafka_replicas replicas (expected >= 3)"
    fi
    
    # Test PVCs
    test_resource_count "pvc" "data-pipeline" 3
else
    log_warning "Data-pipeline namespace not found - skipping tests"
fi

echo ""

# Test 5: Microservices
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test Suite 5: Microservices Architecture"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if kubectl get namespace microservices &>/dev/null; then
    log_info "Testing microservices namespace..."
    
    test_resource_exists "deployment" "user-service" "microservices"
    test_resource_exists "deployment" "auth-service" "microservices"
    test_resource_exists "deployment" "notification-service" "microservices"
    
    test_resource_count "deployments" "microservices" 3
    test_resource_count "services" "microservices" 3
    
    # Test version labels
    test_labels_exist "deployment" "user-service" "microservices" "version"
else
    log_warning "Microservices namespace not found - skipping tests"
fi

echo ""

# Test 6: Resource Relationships
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test Suite 6: Resource Relationships"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if kubectl get namespace ecommerce &>/dev/null; then
    log_info "Testing service-to-deployment relationships..."
    
    # Check if services have matching deployments
    services=$(kubectl get services -n ecommerce -o jsonpath='{.items[*].metadata.name}' 2>/dev/null)
    for svc in $services; do
        if kubectl get deployment $svc -n ecommerce &>/dev/null || kubectl get statefulset $svc -n ecommerce &>/dev/null; then
            log_success "Service $svc has matching workload"
        else
            log_warning "Service $svc has no matching workload"
        fi
    done
fi

echo ""

# Test 7: Annotations
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test Suite 7: Annotations and Metadata"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if kubectl get namespace production &>/dev/null; then
    log_info "Testing annotations..."
    
    # Check for prometheus annotations on pods
    local pods_with_prometheus=$(kubectl get pods -n production -o json 2>/dev/null | \
        jq -r '.items[] | select(.metadata.annotations."prometheus.io/scrape" == "true") | .metadata.name' | wc -l | tr -d ' ')
    
    if [ "$pods_with_prometheus" -gt 0 ]; then
        log_success "Found $pods_with_prometheus pods with Prometheus annotations"
    else
        log_warning "No pods with Prometheus annotations found"
    fi
fi

if kubectl get namespace ecommerce &>/dev/null; then
    # Check ingress annotations
    local ingress_class=$(kubectl get ingress ecommerce-ingress -n ecommerce -o jsonpath='{.metadata.annotations.kubernetes\.io/ingress\.class}' 2>/dev/null)
    if [ "$ingress_class" = "nginx" ]; then
        log_success "Ingress has correct ingress class annotation"
    else
        log_warning "Ingress missing or has incorrect ingress class"
    fi
fi

echo ""

# Test 8: Resource Limits
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test Suite 8: Resource Limits and Requests"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if kubectl get namespace production &>/dev/null; then
    log_info "Testing resource limits..."
    
    deployments=$(kubectl get deployments -n production -o name 2>/dev/null)
    for deploy in $deployments; do
        deploy_name=$(echo $deploy | cut -d'/' -f2)
        has_limits=$(kubectl get deployment $deploy_name -n production -o json 2>/dev/null | \
            jq -r '.spec.template.spec.containers[0].resources.limits' | grep -v null | wc -l | tr -d ' ')
        
        if [ "$has_limits" -gt 0 ]; then
            log_success "Deployment $deploy_name has resource limits"
        else
            log_warning "Deployment $deploy_name missing resource limits"
        fi
    done
fi

echo ""

# Test 9: Health Probes
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test Suite 9: Health Probes"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if kubectl get namespace production &>/dev/null; then
    log_info "Testing health probes..."
    
    deployments=$(kubectl get deployments -n production -o name 2>/dev/null)
    for deploy in $deployments; do
        deploy_name=$(echo $deploy | cut -d'/' -f2)
        
        has_liveness=$(kubectl get deployment $deploy_name -n production -o json 2>/dev/null | \
            jq -r '.spec.template.spec.containers[0].livenessProbe' | grep -v null | wc -l | tr -d ' ')
        has_readiness=$(kubectl get deployment $deploy_name -n production -o json 2>/dev/null | \
            jq -r '.spec.template.spec.containers[0].readinessProbe' | grep -v null | wc -l | tr -d ' ')
        
        if [ "$has_liveness" -gt 0 ] && [ "$has_readiness" -gt 0 ]; then
            log_success "Deployment $deploy_name has liveness and readiness probes"
        else
            log_warning "Deployment $deploy_name missing health probes"
        fi
    done
fi

echo ""

# Summary
echo "╔════════════════════════════════════════════════════════════╗"
echo "║                       Test Summary                         ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""
echo -e "Tests Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Tests Failed: ${RED}$TESTS_FAILED${NC}"
echo ""

# Overall cluster stats
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Overall Cluster Statistics"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

echo "Namespaces:"
kubectl get namespaces | grep -E "(production|ecommerce|monitoring|data-pipeline|microservices)" | awk '{printf "  - %s\n", $1}'

echo ""
echo "Resource Counts (All Test Namespaces):"
for ns in production ecommerce monitoring data-pipeline microservices; do
    if kubectl get namespace $ns &>/dev/null; then
        echo "  $ns:"
        echo "    Deployments:  $(kubectl get deployments -n $ns --no-headers 2>/dev/null | wc -l | tr -d ' ')"
        echo "    StatefulSets: $(kubectl get statefulsets -n $ns --no-headers 2>/dev/null | wc -l | tr -d ' ')"
        echo "    DaemonSets:   $(kubectl get daemonsets -n $ns --no-headers 2>/dev/null | wc -l | tr -d ' ')"
        echo "    Services:     $(kubectl get services -n $ns --no-headers 2>/dev/null | wc -l | tr -d ' ')"
        echo "    Pods:         $(kubectl get pods -n $ns --no-headers 2>/dev/null | wc -l | tr -d ' ')"
    fi
done

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed! Your visualizer test data is ready.${NC}"
    exit 0
else
    echo -e "${YELLOW}⚠ Some tests failed. Review the output above.${NC}"
    exit 1
fi



