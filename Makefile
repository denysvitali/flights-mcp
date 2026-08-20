# flights-mcp Makefile

BINARY_NAME=flights-mcp
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

.PHONY: all build clean test lint run install docker-build help

# Default target
all: lint test build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/flights-mcp

# Build for Linux (amd64)
build-linux:
	@echo "Building $(BINARY_NAME) for Linux amd64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/flights-mcp

# Build for macOS (arm64)
build-darwin:
	@echo "Building $(BINARY_NAME) for macOS arm64..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/flights-mcp

# Build for Windows
build-windows:
	@echo "Building $(BINARY_NAME) for Windows..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/flights-mcp

# Build all platforms
build-all: build-linux build-darwin build-windows

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	go clean

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -cover ./...

# Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run short tests only
test-short:
	@echo "Running short tests..."
	go test -v -short ./...

# Run linter
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest)
	golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

# Run the MCP server
run:
	@echo "Running MCP server..."
	go run ./cmd/flights-mcp run

# Run test search
run-test:
	@echo "Running test search..."
	go run ./cmd/flights-mcp test JFK LAX 2025-12-15

# Install the binary
install:
	@echo "Installing $(BINARY_NAME)..."
	go install $(LDFLAGS) ./cmd/flights-mcp

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) .
	docker tag $(BINARY_NAME):$(VERSION) $(BINARY_NAME):latest

# Run in Docker
docker-run:
	docker run -it --rm $(BINARY_NAME):latest info

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Update dependencies
deps-update:
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

# Generate mocks (if needed)
generate:
	@echo "Generating code..."
	go generate ./...

# Show help
help:
	@echo "flights-mcp Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all           Run lint, test, and build (default)"
	@echo "  build         Build the binary for current platform"
	@echo "  build-linux   Build for Linux amd64"
	@echo "  build-darwin  Build for macOS arm64"
	@echo "  build-windows Build for Windows amd64"
	@echo "  build-all     Build for all platforms"
	@echo "  clean         Remove build artifacts"
	@echo "  test          Run all tests"
	@echo "  test-coverage Run tests with coverage report"
	@echo "  test-short    Run short tests only"
	@echo "  lint          Run golangci-lint"
	@echo "  fmt           Format code"
	@echo "  run           Run the MCP server"
	@echo "  run-test      Run a test flight search"
	@echo "  install       Install binary to GOPATH/bin"
	@echo "  docker-build  Build Docker image"
	@echo "  docker-run    Run in Docker"
	@echo "  deps          Download dependencies"
	@echo "  deps-update   Update dependencies"
	@echo "  help          Show this help"
