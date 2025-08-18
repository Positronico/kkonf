# Makefile for kkonf

# Get version information from git
VERSION ?= $(shell git describe --tags --always --dirty=-dev 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Go build settings
BINARY_NAME = kkonf
PACKAGE = github.com/positronico/kkonf
VERSION_PACKAGE = $(PACKAGE)/internal/version

# Build flags
LDFLAGS = -s -w \
	-X $(VERSION_PACKAGE).Version=$(VERSION) \
	-X $(VERSION_PACKAGE).GitCommit=$(GIT_COMMIT) \
	-X $(VERSION_PACKAGE).BuildDate=$(BUILD_DATE)

# Default target
.PHONY: all
all: build

# Build binary
.PHONY: build
build:
	@echo "Building $(BINARY_NAME) $(VERSION)"
	go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .

# Build for all platforms (like CI)
.PHONY: build-all
build-all:
	@echo "Building for all platforms..."
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-windows-amd64.exe .

# Development build (no optimization)
.PHONY: build-dev
build-dev:
	@echo "Building development version..."
	go build -ldflags="-X $(VERSION_PACKAGE).Version=dev-$(GIT_COMMIT) -X $(VERSION_PACKAGE).GitCommit=$(GIT_COMMIT) -X $(VERSION_PACKAGE).BuildDate=$(BUILD_DATE)" -o $(BINARY_NAME) .

# Clean built binaries
.PHONY: clean
clean:
	@echo "Cleaning binaries..."
	rm -f $(BINARY_NAME) $(BINARY_NAME)-*

# Run tests
.PHONY: test
test:
	go test ./...

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Lint code
.PHONY: lint
lint:
	golangci-lint run

# Install binary to $GOPATH/bin
.PHONY: install
install:
	go install -ldflags="$(LDFLAGS)" .

# Show version information
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build      - Build binary for current platform"
	@echo "  build-all  - Build binaries for all platforms"
	@echo "  build-dev  - Build development version"
	@echo "  clean      - Remove built binaries"
	@echo "  test       - Run tests"
	@echo "  fmt        - Format code"
	@echo "  lint       - Lint code"
	@echo "  install    - Install binary to GOPATH/bin"
	@echo "  version    - Show version information"
	@echo "  help       - Show this help"