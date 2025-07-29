# Atmosbyte - Weather Data Processing System
# Makefile for cross-platform builds

PROJECT_NAME := atmosbyte
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
COMMIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

BUILD_DIR := bin
MAIN_FILE := .

LDFLAGS := -w -s

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

BINARY_NAME := $(PROJECT_NAME)
ifeq ($(GOOS),windows)
	BINARY_NAME := $(PROJECT_NAME).exe
endif

BINARY_PATH := $(BUILD_DIR)/$(GOOS)-$(GOARCH)/$(BINARY_NAME)

GREEN := \033[32m
YELLOW := \033[33m
CYAN := \033[36m
RESET := \033[0m

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo "$(CYAN)Atmosbyte - Weather Data Processing System$(RESET)"
	@echo "$(YELLOW)Available targets:$(RESET)"
	@echo "  help            Show this help message"
	@echo "  clean           Remove build artifacts"
	@echo "  fmt             Format Go code"
	@echo "  test            Run tests"
	@echo "  build           Build binary for target platform"
	@echo "  build-dev       Build with debug information"
	@echo "  build-all       Build for all supported platforms"
	@echo "  run-sim         Run with simulated sensor"
	@echo "  version         Show version information"
	@echo "  info            Show build information"
	@echo ""
	@echo "$(YELLOW)Build examples:$(RESET)"
	@echo "  make build                    # Build for current platform"
	@echo "  make build GOOS=linux GOARCH=amd64  # Build for Linux AMD64"
	@echo "  make build GOOS=windows GOARCH=amd64 # Build for Windows AMD64"
	@echo "  make build GOOS=linux GOARCH=arm64  # Build for Linux ARM64 (Raspberry Pi 4)"
	@echo "  make build GOOS=linux GOARCH=arm    # Build for Linux ARM (Raspberry Pi 3)"

$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

.PHONY: clean
clean: ## Remove build artifacts
	@echo "$(YELLOW)Cleaning build artifacts...$(RESET)"
	@rm -rf $(BUILD_DIR)
	@echo "$(GREEN)✓ Clean completed$(RESET)"

.PHONY: fmt
fmt: ## Format Go code
	@echo "$(YELLOW)Formatting Go code...$(RESET)"
	@go fmt ./...
	@echo "$(GREEN)✓ Code formatted$(RESET)"

.PHONY: test
test: ## Run tests
	@echo "$(YELLOW)Running tests...$(RESET)"
	@go test -v ./...
	@echo "$(GREEN)✓ Tests completed$(RESET)"

.PHONY: build
build: $(BUILD_DIR) fmt ## Build binary for target platform
	@echo "$(YELLOW)Building $(PROJECT_NAME) for $(GOOS)/$(GOARCH)...$(RESET)"
	@mkdir -p $(dir $(BINARY_PATH))
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags "$(LDFLAGS)" \
		-o $(BINARY_PATH) \
		$(MAIN_FILE)
	@echo "$(GREEN)✓ Build completed: $(BINARY_PATH)$(RESET)"
	@ls -lh $(BINARY_PATH)

.PHONY: build-dev
build-dev: $(BUILD_DIR) ## Build with debug information
	@echo "$(YELLOW)Building $(PROJECT_NAME) for development...$(RESET)"
	@mkdir -p $(BUILD_DIR)/dev
	@go build -o $(BUILD_DIR)/dev/$(BINARY_NAME) $(MAIN_FILE)
	@echo "$(GREEN)✓ Development build completed: $(BUILD_DIR)/dev/$(BINARY_NAME)$(RESET)"

.PHONY: build-linux-amd64
build-linux-amd64: ## Build for Linux AMD64
	@$(MAKE) build GOOS=linux GOARCH=amd64

.PHONY: build-linux-arm64
build-linux-arm64: ## Build for Linux ARM64 (Raspberry Pi 4)
	@$(MAKE) build GOOS=linux GOARCH=arm64

.PHONY: build-linux-arm
build-linux-arm: ## Build for Linux ARM (Raspberry Pi 3)
	@$(MAKE) build GOOS=linux GOARCH=arm GOARM=7

.PHONY: build-windows-amd64
build-windows-amd64: ## Build for Windows AMD64
	@$(MAKE) build GOOS=windows GOARCH=amd64

.PHONY: build-darwin-amd64
build-darwin-amd64: ## Build for macOS AMD64
	@$(MAKE) build GOOS=darwin GOARCH=amd64

.PHONY: build-darwin-arm64
build-darwin-arm64: ## Build for macOS ARM64 (Apple Silicon)
	@$(MAKE) build GOOS=darwin GOARCH=arm64

.PHONY: build-all
build-all: clean ## Build for all supported platforms
	@echo "$(CYAN)Building for all supported platforms...$(RESET)"
	@$(MAKE) build-linux-amd64
	@$(MAKE) build-linux-arm64
	@$(MAKE) build-linux-arm
	@$(MAKE) build-windows-amd64
	@$(MAKE) build-darwin-amd64
	@$(MAKE) build-darwin-arm64
	@echo "$(GREEN)✓ All builds completed$(RESET)"
	@echo "$(YELLOW)Build artifacts:$(RESET)"
	@find $(BUILD_DIR) -name "$(PROJECT_NAME)*" -type f -exec ls -lh {} \;

.PHONY: run-sim
run-sim: build-dev ## Run with simulated sensor
	@echo "$(YELLOW)Starting $(PROJECT_NAME) with simulated sensor...$(RESET)"
	@cd $(BUILD_DIR)/dev && \
		USE_SIMULATED_SENSOR=true \
		OPENWEATHER_API_KEY=test \
		STATION_ID=test \
		./$(BINARY_NAME)

.PHONY: version
version: build-dev ## Show version information
	@echo "$(YELLOW)Version Information:$(RESET)"
	@cd $(BUILD_DIR)/dev && ./$(BINARY_NAME) --version

.PHONY: info
info: ## Show build information
	@echo "$(CYAN)Build Information:$(RESET)"
	@echo "  Project:     $(PROJECT_NAME)"
	@echo "  Build Time:  $(BUILD_TIME)"
	@echo "  Commit:      $(COMMIT_HASH)"
	@echo "  Go Version:  $(shell go version)"
	@echo "  Target OS:   $(GOOS)"
	@echo "  Target Arch: $(GOARCH)"
	@echo "  Build Dir:   $(BUILD_DIR)"
