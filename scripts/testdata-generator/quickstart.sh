#!/bin/bash

set -e

echo "🚀 Kubernetes Test Data Generator - Quick Start"
echo "================================================"
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}❌ kubectl is not installed${NC}"
    echo "Please install kubectl: https://kubernetes.io/docs/tasks/tools/"
    exit 1
fi

# Check if cluster is accessible
if ! kubectl cluster-info &> /dev/null; then
    echo -e "${RED}❌ Cannot connect to Kubernetes cluster${NC}"
    echo ""
    echo "Please ensure:"
    echo "  1. Minikube is running: minikube start"
    echo "  2. Or your kubeconfig is properly configured"
    exit 1
fi

echo -e "${GREEN}✅ Connected to Kubernetes cluster${NC}"
kubectl cluster-info | head -n 1
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ Go is not installed${NC}"
    echo "Please install Go 1.24+: https://golang.org/dl/"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo -e "${GREEN}✅ Go ${GO_VERSION} detected${NC}"
echo ""

# Install dependencies
echo "📦 Installing dependencies..."
if go mod download; then
    echo -e "${GREEN}✅ Dependencies installed${NC}"
else
    echo -e "${RED}❌ Failed to install dependencies${NC}"
    exit 1
fi
echo ""

# Build the binary
echo "🔨 Building testdata-generator..."
if go build -o build/testdata-generator main.go; then
    echo -e "${GREEN}✅ Build successful${NC}"
else
    echo -e "${RED}❌ Build failed${NC}"
    exit 1
fi
echo ""

# Show cluster info
echo "📊 Current Cluster Info:"
echo "  Context: $(kubectl config current-context)"
echo "  Nodes: $(kubectl get nodes --no-headers | wc -l | tr -d ' ')"
echo "  Namespaces: $(kubectl get namespaces --no-headers | wc -l | tr -d ' ')"
echo ""

# Run the generator
echo -e "${YELLOW}🎯 Starting Test Data Generator...${NC}"
echo ""
./build/testdata-generator

echo ""
echo -e "${GREEN}✅ Done!${NC}"



