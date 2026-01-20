# Sample Makefile for syntax highlighting

# Variables
PROJECT := myproject
VERSION := 1.0.0
GO := go
GOFLAGS := -v
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

# Directories
SRC_DIR := ./cmd
BUILD_DIR := ./build
DIST_DIR := ./dist

# Source files
SOURCES := $(shell find . -name '*.go' -type f)

# Phony targets
.PHONY: all build clean test lint install help

# Default target
all: build

# Build the application
build: $(BUILD_DIR)/$(PROJECT)

$(BUILD_DIR)/$(PROJECT): $(SOURCES)
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $@ $(SRC_DIR)/main.go

# Run tests
test:
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	golangci-lint run ./...

# Install dependencies
deps:
	$(GO) mod download
	$(GO) mod verify

# Clean build artifacts
clean:
	@rm -rf $(BUILD_DIR)
	@rm -rf $(DIST_DIR)
	@rm -f coverage.out coverage.html
	@echo "Cleaned build artifacts"

# Install to GOPATH
install: build
	cp $(BUILD_DIR)/$(PROJECT) $(GOPATH)/bin/

# Cross-compilation
.PHONY: release
release: clean
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(PROJECT)-linux-amd64 $(SRC_DIR)/main.go
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(PROJECT)-darwin-amd64 $(SRC_DIR)/main.go
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST_DIR)/$(PROJECT)-windows-amd64.exe $(SRC_DIR)/main.go

# Docker build
.PHONY: docker
docker:
	docker build -t $(PROJECT):$(VERSION) .
	docker tag $(PROJECT):$(VERSION) $(PROJECT):latest

# Help
help:
	@echo "Available targets:"
	@echo "  all      - Build the project (default)"
	@echo "  build    - Compile the binary"
	@echo "  test     - Run tests with coverage"
	@echo "  lint     - Run linter"
	@echo "  deps     - Download dependencies"
	@echo "  clean    - Remove build artifacts"
	@echo "  install  - Install to GOPATH"
	@echo "  release  - Cross-compile for all platforms"
	@echo "  docker   - Build Docker image"
	@echo "  help     - Show this help"

# Conditional
ifdef DEBUG
GOFLAGS += -gcflags="all=-N -l"
endif

# Include local overrides if present
-include Makefile.local
