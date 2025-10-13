# Makefile for GitHub MCP Server with LMM Oracle and MPC

.PHONY: build build-all clean test test-coverage lint fmt vet deps docker-build docker-run help

# Build variables
BINARY_NAME=github-mcp-server
LMM_BINARY_NAME=lmm-server
VERSION?=dev
COMMIT?=$(shell git rev-parse --short HEAD)
DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}"

# Go variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Directories
BUILD_DIR=build
CMD_DIR=cmd
PKG_DIR=pkg

help: ## Display this help message
	@echo "GitHub MCP Server with LMM Oracle and MPC"
	@echo "========================================="
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the main GitHub MCP server
	@echo "Building GitHub MCP Server..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)/github-mcp-server

build-lmm: ## Build the LMM Oracle and MPC server
	@echo "Building LMM Oracle and MPC Server..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(LMM_BINARY_NAME) ./$(CMD_DIR)/lmm-server

build-mcpcurl: ## Build the mcpcurl tool
	@echo "Building mcpcurl..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/mcpcurl ./$(CMD_DIR)/mcpcurl

build-all: build build-lmm build-mcpcurl ## Build all binaries
	@echo "All binaries built successfully!"
	@ls -la $(BUILD_DIR)/

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-lmm: ## Run LMM-specific tests
	@echo "Running LMM tests..."
	$(GOTEST) -v ./$(PKG_DIR)/lmm/...

lint: ## Run linter
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

fmt: ## Format Go code
	@echo "Formatting Go code..."
	$(GOFMT) -s -w .

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	$(GOMOD) get -u ./...
	$(GOMOD) tidy

# Docker targets
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t github-mcp-server:$(VERSION) .

docker-build-lmm: ## Build LMM Docker image
	@echo "Building LMM Docker image..."
	docker build -f Dockerfile.lmm -t lmm-server:$(VERSION) .

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run -it --rm \
		-e GITHUB_PERSONAL_ACCESS_TOKEN=${GITHUB_PERSONAL_ACCESS_TOKEN} \
		github-mcp-server:$(VERSION)

docker-run-lmm: ## Run LMM Docker container
	@echo "Running LMM Docker container..."
	docker run -it --rm lmm-server:$(VERSION) stdio

# Development targets
dev-setup: deps ## Setup development environment
	@echo "Setting up development environment..."
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Development environment ready!"

run-github: build ## Run GitHub MCP server locally
	@echo "Running GitHub MCP server..."
	./$(BUILD_DIR)/$(BINARY_NAME) stdio

run-lmm: build-lmm ## Run LMM server locally
	@echo "Running LMM server..."
	./$(BUILD_DIR)/$(LMM_BINARY_NAME) stdio

run-example: build-lmm ## Run LMM usage example
	@echo "Running LMM usage example..."
	$(GOCMD) run ./examples/lmm_usage.go

# Testing with mcpcurl
test-mcpcurl: build build-mcpcurl ## Test with mcpcurl
	@echo "Testing GitHub MCP server with mcpcurl..."
	@echo "Make sure to set GITHUB_PERSONAL_ACCESS_TOKEN environment variable"
	./$(BUILD_DIR)/mcpcurl --stdio-server-cmd="./$(BUILD_DIR)/$(BINARY_NAME) stdio" tools --help

test-lmm-mcpcurl: build-lmm build-mcpcurl ## Test LMM server with mcpcurl
	@echo "Testing LMM server with mcpcurl..."
	./$(BUILD_DIR)/mcpcurl --stdio-server-cmd="./$(BUILD_DIR)/$(LMM_BINARY_NAME) stdio" tools --help

# Release targets
release-build: ## Build release binaries for multiple platforms
	@echo "Building release binaries..."
	@mkdir -p $(BUILD_DIR)/release
	
	# Linux AMD64
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-linux-amd64 ./$(CMD_DIR)/github-mcp-server
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/release/$(LMM_BINARY_NAME)-linux-amd64 ./$(CMD_DIR)/lmm-server
	
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-darwin-amd64 ./$(CMD_DIR)/github-mcp-server
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/release/$(LMM_BINARY_NAME)-darwin-amd64 ./$(CMD_DIR)/lmm-server
	
	# macOS ARM64
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-darwin-arm64 ./$(CMD_DIR)/github-mcp-server
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/release/$(LMM_BINARY_NAME)-darwin-arm64 ./$(CMD_DIR)/lmm-server
	
	# Windows AMD64
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-windows-amd64.exe ./$(CMD_DIR)/github-mcp-server
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/release/$(LMM_BINARY_NAME)-windows-amd64.exe ./$(CMD_DIR)/lmm-server
	
	@echo "Release binaries built:"
	@ls -la $(BUILD_DIR)/release/

# Quality assurance
qa: fmt vet lint test ## Run all quality assurance checks
	@echo "All QA checks passed!"

# CI/CD targets
ci: deps qa test-coverage ## Run CI pipeline
	@echo "CI pipeline completed successfully!"

# Documentation
docs: ## Generate documentation
	@echo "Generating documentation..."
	$(GOCMD) doc -all ./$(PKG_DIR)/lmm > docs/lmm-api.md
	@echo "Documentation generated in docs/"

# Benchmarks
bench: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

bench-lmm: ## Run LMM benchmarks
	@echo "Running LMM benchmarks..."
	$(GOTEST) -bench=. -benchmem ./$(PKG_DIR)/lmm/...

# Security
security-scan: ## Run security scan
	@echo "Running security scan..."
	@which gosec > /dev/null || go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	gosec ./...

# Performance profiling
profile-cpu: build-lmm ## Profile CPU usage
	@echo "Profiling CPU usage..."
	$(GOCMD) test -cpuprofile=cpu.prof -bench=. ./$(PKG_DIR)/lmm/...
	$(GOCMD) tool pprof cpu.prof

profile-mem: build-lmm ## Profile memory usage
	@echo "Profiling memory usage..."
	$(GOCMD) test -memprofile=mem.prof -bench=. ./$(PKG_DIR)/lmm/...
	$(GOCMD) tool pprof mem.prof

# Installation
install: build build-lmm ## Install binaries to GOPATH/bin
	@echo "Installing binaries..."
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/
	cp $(BUILD_DIR)/$(LMM_BINARY_NAME) $(GOPATH)/bin/
	@echo "Binaries installed to $(GOPATH)/bin/"

uninstall: ## Uninstall binaries from GOPATH/bin
	@echo "Uninstalling binaries..."
	rm -f $(GOPATH)/bin/$(BINARY_NAME)
	rm -f $(GOPATH)/bin/$(LMM_BINARY_NAME)
	@echo "Binaries uninstalled"

# Default target
all: build-all test ## Build all and run tests