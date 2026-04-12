.PHONY: build run clean install help

# Binary name
BINARY_NAME=testdata-generator
BUILD_DIR=build

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GORUN=$(GOCMD) run
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the binary
	@echo "🔨 Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v
	@echo "✅ Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

run: ## Run the application directly
	@echo "🚀 Running $(BINARY_NAME)..."
	@$(GORUN) main.go

install: ## Install dependencies
	@echo "📦 Installing dependencies..."
	@$(GOMOD) download
	@$(GOMOD) tidy
	@echo "✅ Dependencies installed"

clean: ## Clean build artifacts
	@echo "🧹 Cleaning..."
	@$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@echo "✅ Clean complete"

deps: ## Download dependencies
	@echo "📥 Downloading dependencies..."
	@$(GOMOD) download

verify: ## Verify dependencies
	@echo "🔍 Verifying dependencies..."
	@$(GOMOD) verify

tidy: ## Tidy go.mod
	@echo "🧹 Tidying go.mod..."
	@$(GOMOD) tidy

fmt: ## Format code
	@echo "📝 Formatting code..."
	@$(GOCMD) fmt ./...

all: clean install build ## Clean, install deps, and build

.DEFAULT_GOAL := help



