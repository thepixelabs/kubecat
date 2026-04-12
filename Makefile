# Kubecat Makefile
# Kubernetes Command Center

# Build variables
BINARY_NAME := kubecat
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-dev")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X github.com/kubecat/kubecat/internal/version.Version=$(VERSION) \
	-X github.com/kubecat/kubecat/internal/version.GitCommit=$(GIT_COMMIT) \
	-X github.com/kubecat/kubecat/internal/version.BuildDate=$(BUILD_DATE)"

# Go commands
GO := go
GOFMT := gofmt
GOLINT := golangci-lint

# Directories
BUILD_DIR := ./build

.PHONY: all build run dev clean test lint fmt tidy deps help install-deps setup-hooks build-mac build-linux build-windows

## all: Build the application
all: build

## dev: Run in development mode
dev:
	@echo "Starting in development mode..."
	wails dev

## build: Build for current platform
build:
	@echo "Building $(BINARY_NAME)..."
	@cd frontend && npm ci && npm run build
	wails build -s $(LDFLAGS)
	@echo "Binary built: $(BUILD_DIR)/bin/$(BINARY_NAME)"

## build-mac: Build for macOS (universal binary)
build-mac:
	@echo "Building for macOS..."
	@cd frontend && npm ci && npm run build
	wails build -s -platform darwin/universal $(LDFLAGS)
	@echo "macOS app built"

## build-linux: Build for Linux (amd64)
build-linux:
	@echo "Building for Linux..."
	@cd frontend && npm ci && npm run build
	wails build -s -platform linux/amd64 $(LDFLAGS)
	@echo "Linux app built"

## build-windows: Build for Windows (amd64)
build-windows:
	@echo "Building for Windows..."
	@cd frontend && npm ci && npm run build
	wails build -s -platform windows/amd64 $(LDFLAGS)
	@echo "Windows app built"

## run: Build and run the application
run: build
	$(BUILD_DIR)/bin/$(BINARY_NAME)

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@go clean -cache

## install-deps: Install all dependencies and set up pre-commit hooks
install-deps:
	@echo "Installing frontend dependencies..."
	@cd frontend && npm ci
	@echo "Installing lefthook pre-commit hooks..."
	@command -v lefthook >/dev/null 2>&1 || go install github.com/evilmartians/lefthook@latest
	lefthook install

## setup-hooks: Install git hooks via lefthook (alias for install-deps hook step)
setup-hooks:
	@command -v lefthook >/dev/null 2>&1 || go install github.com/evilmartians/lefthook@latest
	lefthook install
	@echo "Pre-commit hooks installed"

## test: Run tests
test:
	@echo "Running tests..."
	$(GO) test -v -race -cover ./...

## lint: Run linters
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Installing golangci-lint..."; go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8; }
	$(GOLINT) run ./...
	@echo "Running ESLint..."
	@cd frontend && npx eslint . --max-warnings=0

## fmt: Format code
fmt:
	$(GOFMT) -s -w .

## tidy: Tidy go.mod
tidy:
	$(GO) mod tidy

## deps: Download dependencies
deps:
	$(GO) mod download

## version: Print version information
version:
	@echo "Version:    $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

## help: Show this help
help:
	@echo "Kubecat - Kubernetes Command Center"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
