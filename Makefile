# Makefile for continueplz

# Build variables
BINARY_NAME := continueplz
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Go settings
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOVET := $(GOCMD) vet
GOFMT := gofmt
GOMOD := $(GOCMD) mod

# Directories
CMD_DIR := ./cmd/continueplz
BUILD_DIR := ./build
COVERAGE_FILE := coverage.out

# Default target
.PHONY: all
all: build

# Build for current platform
.PHONY: build
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(CMD_DIR)

# Install to GOPATH/bin
.PHONY: install
install:
	$(GOCMD) install $(LDFLAGS) $(CMD_DIR)

# Run all tests
.PHONY: test
test:
	$(GOTEST) -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	$(GOTEST) -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	$(GOCMD) tool cover -html=$(COVERAGE_FILE) -o coverage.html

# Run linting
.PHONY: lint
lint:
	$(GOVET) ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, skipping"; \
	fi

# Format code
.PHONY: fmt
fmt:
	$(GOFMT) -w -s .

# Check formatting
.PHONY: fmt-check
fmt-check:
	@if [ -n "$$($(GOFMT) -l .)" ]; then \
		echo "Go files need formatting:"; \
		$(GOFMT) -l .; \
		exit 1; \
	fi

# Tidy dependencies
.PHONY: tidy
tidy:
	$(GOMOD) tidy

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)
	rm -f $(COVERAGE_FILE) coverage.html

# Cross-compilation targets
.PHONY: build-all
build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64

.PHONY: build-linux-amd64
build-linux-amd64:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)

.PHONY: build-linux-arm64
build-linux-arm64:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)

.PHONY: build-darwin-amd64
build-darwin-amd64:
	mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)

.PHONY: build-darwin-arm64
build-darwin-arm64:
	mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)

# Release build with all platforms
.PHONY: release
release: clean build-all
	@echo "Release builds created in $(BUILD_DIR)/"
	@ls -la $(BUILD_DIR)/

# Development helpers
.PHONY: run
run: build
	./$(BINARY_NAME)

.PHONY: dev
dev:
	$(GOBUILD) -o $(BINARY_NAME) $(CMD_DIR) && ./$(BINARY_NAME)

# Show help
.PHONY: help
help:
	@echo "continueplz Makefile targets:"
	@echo ""
	@echo "  build          Build for current platform"
	@echo "  install        Install to GOPATH/bin"
	@echo "  test           Run tests"
	@echo "  test-coverage  Run tests with coverage report"
	@echo "  lint           Run linting checks"
	@echo "  fmt            Format Go code"
	@echo "  fmt-check      Check code formatting"
	@echo "  tidy           Tidy go.mod dependencies"
	@echo "  clean          Remove build artifacts"
	@echo ""
	@echo "  build-all      Build for all platforms"
	@echo "  build-linux-amd64    Build for Linux amd64"
	@echo "  build-linux-arm64    Build for Linux arm64"
	@echo "  build-darwin-amd64   Build for macOS amd64"
	@echo "  build-darwin-arm64   Build for macOS arm64"
	@echo "  release        Create release builds for all platforms"
	@echo ""
	@echo "  run            Build and run"
	@echo "  dev            Quick build and run"
	@echo "  help           Show this help"
