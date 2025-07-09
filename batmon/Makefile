# Makefile for Go project compilation

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

# Main package path
MAIN_PATH=.

# Output directory
BUILD_DIR=build

# Binary name pattern: app-${GOOS}-${GOARCH}
BINARY_NAME=batmon

# Ensure build directory exists
$(shell mkdir -p $(BUILD_DIR))

.PHONY: all build build-linux-amd64 build-linux-arm64 clean test help

all: clean build

# Build for current platform
build:
		$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

# Build for Linux AMD64
build-linux-amd64:
		GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-amd64 $(MAIN_PATH)

# Build for Linux ARM64
build-linux-arm64:
		GOOS=linux GOARCH=arm64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

# Build for all platforms
build-all: build-linux-amd64 build-linux-arm64

# Clean build artifacts
clean:
		$(GOCLEAN)
		rm -rf $(BUILD_DIR)

# Run tests
test:
		$(GOTEST) -v ./...

# Show help
help:
		@echo "Available commands:"
		@echo " make               - Build for current platform"
		@echo " make build         - Build for current platform"
		@echo " make build-linux-amd64 - Build for Linux AMD64"
		@echo " make build-linux-arm64 - Build for Linux ARM64"
		@echo " make build-all     - Build for all platforms"
		@echo " make clean         - Remove binaries and build artifacts"
		@echo " make test          - Run tests"
		@echo " make help          - Show this help message"
