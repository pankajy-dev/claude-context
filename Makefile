# Makefile for Claude Context Manager

# Variables
BINARY_NAME=cctx
CLI_DIR=cli
BUILD_DIR=.
INSTALL_DIR=$(HOME)/bin
GO=go
GOFLAGS=

# Version info
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

.PHONY: all build install uninstall clean test help

# Default target
all: build

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	cd $(CLI_DIR) && $(GO) build $(GOFLAGS) $(LDFLAGS) -o ../$(BINARY_NAME) ./main.go
	@echo "✓ Build complete: ./$(BINARY_NAME)"

## install: Build and install to ~/bin
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	@cp $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@chmod +x $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "✓ Installed to $(INSTALL_DIR)/$(BINARY_NAME)"
	@echo ""
	@echo "⚠️  Clear your shell's command cache to use the updated binary:"
	@echo "  hash -r         # For bash/zsh"
	@echo "  rehash          # For zsh (alternative)"
	@echo ""
	@echo "Make sure $(INSTALL_DIR) is in your PATH:"
	@echo "  export PATH=\"\$$HOME/bin:\$$PATH\""
	@echo ""
	@echo "Initialize the data directory:"
	@echo "  $(BINARY_NAME) init"

## install-global: Install to /usr/local/bin (requires sudo)
install-global: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo cp $(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "✓ Installed to /usr/local/bin/$(BINARY_NAME)"
	@echo ""
	@echo "⚠️  Clear your shell's command cache to use the updated binary:"
	@echo "  hash -r         # For bash/zsh"
	@echo "  rehash          # For zsh (alternative)"

## uninstall: Remove the binary from ~/bin
uninstall:
	@echo "Uninstalling $(BINARY_NAME) from $(INSTALL_DIR)..."
	@rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "✓ Uninstalled"

## uninstall-global: Remove the binary from /usr/local/bin (requires sudo)
uninstall-global:
	@echo "Uninstalling $(BINARY_NAME) from /usr/local/bin..."
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "✓ Uninstalled"

## clean: Remove build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)
	@rm -f fix_files.py
	@cd $(CLI_DIR)/cmd && rm -f *.bak *.bak[0-9] *.sh
	@echo "✓ Clean complete"

## test: Run all tests
test:
	@echo "Running all tests..."
	cd $(CLI_DIR) && $(GO) test -v ./...

## test-ticket: Run ticket-related tests only
test-ticket:
	@echo "Running ticket tests..."
	cd $(CLI_DIR) && $(GO) test -v ./cmd -run Test_Ticket

## test-link: Run link/unlink tests only
test-link:
	@echo "Running link tests..."
	cd $(CLI_DIR) && $(GO) test -v ./cmd -run Test_Link

## test-global: Run global context tests only
test-global:
	@echo "Running global context tests..."
	cd $(CLI_DIR) && $(GO) test -v ./cmd -run Test_Global

## test-integration: Run integration tests only
test-integration:
	@echo "Running integration tests..."
	cd $(CLI_DIR) && $(GO) test -v ./cmd -run Integration

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	cd $(CLI_DIR) && $(GO) test -v -coverprofile=coverage.out ./...
	cd $(CLI_DIR) && $(GO) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report generated: cli/coverage.html"

## fmt: Format Go code
fmt:
	@echo "Formatting Go code..."
	cd $(CLI_DIR) && $(GO) fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	cd $(CLI_DIR) && $(GO) vet ./...

## check: Run fmt, vet, and test
check: fmt vet test

## help: Show this help message
help:
	@echo "Claude Context Manager - Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' Makefile | column -t -s ':' | sed -e 's/^/ /'
