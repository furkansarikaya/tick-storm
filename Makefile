# Tick-Storm Build Configuration
# Go 1.22+ with static binary compilation

# Variables
BINARY_NAME=tick-storm
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GO_VERSION=$(shell go version | cut -d' ' -f3)

# Build flags
LDFLAGS=-ldflags "-s -w \
	-X 'main.Version=$(VERSION)' \
	-X 'main.BuildTime=$(BUILD_TIME)' \
	-X 'main.GitCommit=$(GIT_COMMIT)' \
	-X 'main.GoVersion=$(GO_VERSION)'"

# Platforms for cross-compilation
PLATFORMS=darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64

# Colors for output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[0;33m
NC=\033[0m # No Color

.PHONY: all build clean test bench lint fmt vet security-scan help

## help: Display this help message
help:
	@echo "Tick-Storm Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^##' Makefile | sed 's/## /  /'

## all: Build for current platform
all: clean fmt vet lint test build

## build: Build static binary for current platform
build:
	@echo "$(GREEN)Building $(BINARY_NAME) v$(VERSION)...$(NC)"
	@CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/server
	@echo "$(GREEN)✓ Binary built: bin/$(BINARY_NAME)$(NC)"
	@ls -lh bin/$(BINARY_NAME)

## build-all: Build for all supported platforms
build-all: clean
	@echo "$(GREEN)Building for all platforms...$(NC)"
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} make build-platform PLATFORM=$$platform; \
	done

build-platform:
	@PLATFORM_NAME=$$(echo $(PLATFORM) | tr '/' '-')
	@echo "$(YELLOW)Building for $(PLATFORM)...$(NC)"
	@mkdir -p bin
	@GOOS=$${PLATFORM%/*} GOARCH=$${PLATFORM#*/} CGO_ENABLED=0 \
		go build $(LDFLAGS) -o bin/$(BINARY_NAME)-$$PLATFORM_NAME ./cmd/server
	@echo "$(GREEN)✓ Built: bin/$(BINARY_NAME)-$$PLATFORM_NAME$(NC)"

## run: Build and run the server
run: build
	@echo "$(GREEN)Starting $(BINARY_NAME)...$(NC)"
	@./bin/$(BINARY_NAME)

## test: Run all tests
test:
	@echo "$(GREEN)Running tests...$(NC)"
	@go test -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)✓ Tests completed$(NC)"

## test-coverage: Run tests with coverage report
test-coverage: test
	@echo "$(GREEN)Generating coverage report...$(NC)"
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ Coverage report: coverage.html$(NC)"

## bench: Run benchmarks
bench:
	@echo "$(GREEN)Running benchmarks...$(NC)"
	@go test -bench=. -benchmem ./...

## lint: Run golangci-lint
lint:
	@echo "$(GREEN)Running linter...$(NC)"
	@if command -v golangci-lint &> /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "$(YELLOW)golangci-lint not installed. Install with: brew install golangci-lint$(NC)"; \
	fi

## fmt: Format code
fmt:
	@echo "$(GREEN)Formatting code...$(NC)"
	@go fmt ./...
	@gofmt -s -w .
	@echo "$(GREEN)✓ Code formatted$(NC)"

## vet: Run go vet
vet:
	@echo "$(GREEN)Running go vet...$(NC)"
	@go vet ./...
	@echo "$(GREEN)✓ Vet completed$(NC)"

## security-scan: Run security vulnerability scan
security-scan:
	@echo "$(GREEN)Running security scan...$(NC)"
	@if command -v gosec &> /dev/null; then \
		gosec -fmt json -out security-report.json ./...; \
		echo "$(GREEN)✓ Security report: security-report.json$(NC)"; \
	else \
		echo "$(YELLOW)gosec not installed. Install with: go install github.com/securego/gosec/v2/cmd/gosec@latest$(NC)"; \
	fi

## deps: Download dependencies
deps:
	@echo "$(GREEN)Downloading dependencies...$(NC)"
	@go mod download
	@go mod tidy
	@echo "$(GREEN)✓ Dependencies updated$(NC)"

## proto: Generate protobuf files
proto:
	@echo "$(GREEN)Generating protobuf files...$(NC)"
	@protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/proto/*.proto
	@echo "$(GREEN)✓ Protobuf files generated$(NC)"

## docker-build: Build Docker image
docker-build:
	@echo "$(GREEN)Building Docker image...$(NC)"
	@docker build -t $(BINARY_NAME):$(VERSION) -t $(BINARY_NAME):latest .
	@echo "$(GREEN)✓ Docker image built: $(BINARY_NAME):$(VERSION)$(NC)"

## docker-push: Push Docker image to registry
docker-push: docker-build
	@echo "$(GREEN)Pushing Docker image...$(NC)"
	@docker push $(BINARY_NAME):$(VERSION)
	@docker push $(BINARY_NAME):latest
	@echo "$(GREEN)✓ Docker image pushed$(NC)"

## clean: Clean build artifacts
clean:
	@echo "$(GREEN)Cleaning build artifacts...$(NC)"
	@rm -rf bin/ coverage.* security-report.json
	@echo "$(GREEN)✓ Clean completed$(NC)"

## install: Install binary to GOPATH/bin
install: build
	@echo "$(GREEN)Installing $(BINARY_NAME)...$(NC)"
	@cp bin/$(BINARY_NAME) $(GOPATH)/bin/
	@echo "$(GREEN)✓ Installed to $(GOPATH)/bin/$(BINARY_NAME)$(NC)"

## version: Display version information
version:
	@echo "Tick-Storm Build Information:"
	@echo "  Version:    $(VERSION)"
	@echo "  Git Commit: $(GIT_COMMIT)"
	@echo "  Build Time: $(BUILD_TIME)"
	@echo "  Go Version: $(GO_VERSION)"

# CI/CD targets
.PHONY: ci-test ci-build ci-lint

## ci-test: Run tests for CI
ci-test:
	@go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

## ci-build: Build for CI
ci-build:
	@CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/server

## ci-lint: Run linting for CI
ci-lint:
	@golangci-lint run --timeout 5m ./...
