#!/usr/bin/env bash
set -euo pipefail

echo "==> Installing Wails CLI v2.11.0..."
go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0

echo "==> Installing golangci-lint..."
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
  | sh -s -- -b "$(go env GOPATH)/bin" v1.62.2

echo "==> Installing lefthook..."
go install github.com/evilmartians/lefthook@latest

echo "==> Installing Go module dependencies..."
go mod download

echo "==> Installing frontend dependencies..."
cd frontend && npm ci && cd ..

echo "==> Installing lefthook git hooks..."
lefthook install

echo "==> Verifying tools..."
wails version
golangci-lint version
lefthook version
node --version
npm --version
kubectl version --client

echo ""
echo "Dev container ready. Run 'wails dev' to start in development mode."
