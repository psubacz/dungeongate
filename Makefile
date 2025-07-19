# Makefile for DungeonGate
# Unified build and test configuration with standardized logging

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt

# Build parameters
SESSION_BINARY_NAME=dungeongate-session-service
AUTH_BINARY_NAME=dungeongate-auth-service
GAME_BINARY_NAME=dungeongate-game-service
BUILD_DIR=bin
SESSION_MAIN_PATH=./cmd/session-service
AUTH_MAIN_PATH=./cmd/auth-service
GAME_MAIN_PATH=./cmd/game-service

# Configuration files
SESSION_CONFIG=configs/session-service.yaml
AUTH_CONFIG=configs/auth-service.yaml
GAME_CONFIG=configs/game-service.yaml

# Test configuration
TEST_TIMEOUT=30s
TEST_COVERAGE_DIR=coverage
TEST_DATA_DIR=test-data

# Logging configuration
LOG_DIR=logs
LOG_LEVEL?=debug
LOG_FORMAT?=text
LOG_OUTPUT?=stdout

# Version and build info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

# Colors for output
GREEN=\033[0;32m
YELLOW=\033[1;33m
RED=\033[0;31m
BLUE=\033[0;34m
NC=\033[0m # No Color

.DEFAULT_GOAL := help

##@ General

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Dependencies

.PHONY: deps
deps: ## Install and update Go dependencies
	@echo "$(GREEN)Installing dependencies...$(NC)"
	$(GOMOD) download
	$(GOMOD) tidy
	$(GOMOD) verify

.PHONY: deps-tools
deps-tools: ## Install development tools
	@echo "$(GREEN)Installing development tools...$(NC)"
	@which air > /dev/null || go install github.com/air-verse/air@latest
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@which govulncheck > /dev/null || go install golang.org/x/vuln/cmd/govulncheck@latest

##@ Build Services

.PHONY: build-session
build-session: deps ## Build the session service binary
	@echo "$(GREEN)Building $(SESSION_BINARY_NAME)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(SESSION_BINARY_NAME) $(SESSION_MAIN_PATH)
	@echo "$(GREEN)Build completed: $(BUILD_DIR)/$(SESSION_BINARY_NAME)$(NC)"

.PHONY: build-auth
build-auth: deps ## Build the auth service binary
	@echo "$(GREEN)Building $(AUTH_BINARY_NAME)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(AUTH_BINARY_NAME) $(AUTH_MAIN_PATH)
	@echo "$(GREEN)Build completed: $(BUILD_DIR)/$(AUTH_BINARY_NAME)$(NC)"

.PHONY: build-game
build-game: deps ## Build the game service binary
	@echo "$(GREEN)Building $(GAME_BINARY_NAME)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(GAME_BINARY_NAME) $(GAME_MAIN_PATH)
	@echo "$(GREEN)Build completed: $(BUILD_DIR)/$(GAME_BINARY_NAME)$(NC)"

.PHONY: build-all
build-all: build-session build-auth build-game ## Build all service binaries

.PHONY: build-debug
build-debug: deps ## Build session service with debug symbols
	@echo "$(GREEN)Building $(SESSION_BINARY_NAME) with debug symbols...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -gcflags="all=-N -l" -o $(BUILD_DIR)/$(SESSION_BINARY_NAME)-debug $(SESSION_MAIN_PATH)

.PHONY: build-race
build-race: deps ## Build all services with race detection
	@echo "$(GREEN)Building services with race detection...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -race -o $(BUILD_DIR)/$(SESSION_BINARY_NAME)-race $(SESSION_MAIN_PATH)
	$(GOBUILD) $(LDFLAGS) -race -o $(BUILD_DIR)/$(AUTH_BINARY_NAME)-race $(AUTH_MAIN_PATH)
	$(GOBUILD) $(LDFLAGS) -race -o $(BUILD_DIR)/$(GAME_BINARY_NAME)-race $(GAME_MAIN_PATH)

##@ Testing

.PHONY: test
test: ## Run all tests
	@echo "$(GREEN)Running all tests...$(NC)"
	$(GOTEST) -v -timeout=$(TEST_TIMEOUT) ./...

.PHONY: test-short
test-short: ## Run short tests only
	@echo "$(GREEN)Running short tests...$(NC)"
	$(GOTEST) -short -timeout=$(TEST_TIMEOUT) ./...

.PHONY: test-race
test-race: ## Run tests with race detection
	@echo "$(GREEN)Running tests with race detection...$(NC)"
	$(GOTEST) -race -timeout=$(TEST_TIMEOUT) ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	@mkdir -p $(TEST_COVERAGE_DIR)
	$(GOTEST) -coverprofile=$(TEST_COVERAGE_DIR)/coverage.out ./...
	$(GOCMD) tool cover -html=$(TEST_COVERAGE_DIR)/coverage.out -o $(TEST_COVERAGE_DIR)/coverage.html
	@echo "$(GREEN)Coverage report: $(TEST_COVERAGE_DIR)/coverage.html$(NC)"

.PHONY: test-ssh
test-ssh: ## Run SSH-specific tests
	@echo "$(GREEN)Running SSH tests...$(NC)"
	$(GOTEST) -v -timeout=$(TEST_TIMEOUT) -run "SSH" ./internal/session/...

.PHONY: test-auth
test-auth: ## Run authentication tests
	@echo "$(GREEN)Running authentication tests...$(NC)"
	$(GOTEST) -v -timeout=$(TEST_TIMEOUT) -run "TestAuth" ./internal/...

.PHONY: test-spectating
test-spectating: ## Run spectating system tests
	@echo "$(GREEN)Running spectating tests...$(NC)"
	$(GOTEST) -v -timeout=$(TEST_TIMEOUT) -run "Spectating|Spectator|Registry|Stream" ./internal/session/...

.PHONY: test-comprehensive
test-comprehensive: ## Run all core test suites
	@echo "$(GREEN)Running comprehensive test suite...$(NC)"
	@echo "$(YELLOW)1. Authentication tests...$(NC)"
	@$(MAKE) test-auth
	@echo "$(YELLOW)2. SSH tests...$(NC)"
	@$(MAKE) test-ssh  
	@echo "$(YELLOW)3. Spectating tests...$(NC)"
	@$(MAKE) test-spectating
	@echo "$(GREEN)All core test suites completed!$(NC)"

.PHONY: benchmark
benchmark: ## Run performance benchmarks
	@echo "$(GREEN)Running benchmarks...$(NC)"
	$(GOTEST) -bench=. -benchmem -timeout=5m ./...

##@ Quality Assurance

.PHONY: fmt
fmt: ## Format Go code
	@echo "$(GREEN)Formatting code...$(NC)"
	$(GOFMT) -s -w .

.PHONY: fmt-check
fmt-check: ## Check if code is formatted
	@echo "$(GREEN)Checking code format...$(NC)"
	@if [ -n "$$($(GOFMT) -s -l .)" ]; then \
		echo "$(RED)Code is not formatted. Run 'make fmt' to fix.$(NC)"; \
		$(GOFMT) -s -l .; \
		exit 1; \
	fi

.PHONY: lint
lint: ## Run linter
	@echo "$(GREEN)Running linter...$(NC)"
	@which golangci-lint > /dev/null || (echo "$(RED)golangci-lint not installed. Run 'make deps-tools'$(NC)" && exit 1)
	golangci-lint run

.PHONY: lint-fix
lint-fix: ## Run linter with auto-fix
	@echo "$(GREEN)Running linter with auto-fix...$(NC)"
	@which golangci-lint > /dev/null || (echo "$(RED)golangci-lint not installed. Run 'make deps-tools'$(NC)" && exit 1)
	golangci-lint run --fix

.PHONY: vet
vet: ## Run go vet
	@echo "$(GREEN)Running go vet...$(NC)"
	$(GOCMD) vet ./...

.PHONY: vuln
vuln: ## Check for security vulnerabilities
	@echo "$(GREEN)Checking for vulnerabilities...$(NC)"
	@which govulncheck > /dev/null || (echo "$(RED)govulncheck not installed. Run 'make deps-tools'$(NC)" && exit 1)
	govulncheck ./...

.PHONY: verify
verify: fmt-check vet lint test ## Run all verification checks

##@ Protocol Buffers

.PHONY: proto-gen
proto-gen: ## Generate Go code from proto files
	@echo "$(GREEN)Generating protobuf code...$(NC)"
	@find api/proto -name "*.proto" -exec protoc --go_out=. --go-grpc_out=. {} \;

.PHONY: proto-clean
proto-clean: ## Clean generated protobuf files
	@echo "$(GREEN)Cleaning generated protobuf files...$(NC)"
	@find pkg/api -name "*.pb.go" -delete

##@ Environment Setup

.PHONY: setup
setup: deps deps-tools ## Setup development environment
	@echo "$(GREEN)Setting up development environment...$(NC)"

.PHONY: setup-test-env
setup-test-env: ## Setup test environment
	@echo "$(GREEN)Setting up test environment...$(NC)"
	@mkdir -p $(TEST_DATA_DIR)/ssh_keys $(TEST_DATA_DIR)/sqlite
	@if [ ! -f "$(TEST_DATA_DIR)/ssh_keys/test_host_key" ]; then \
		ssh-keygen -t rsa -b 2048 -f $(TEST_DATA_DIR)/ssh_keys/test_host_key -N "" -C "dungeongate-test" -q; \
		chmod 600 $(TEST_DATA_DIR)/ssh_keys/test_host_key; \
	fi

.PHONY: clean-test-env
clean-test-env: ## Clean test environment
	@echo "$(GREEN)Cleaning test environment...$(NC)"
	@rm -rf $(TEST_DATA_DIR)
	@rm -rf $(TEST_COVERAGE_DIR)

##@ Logging Management

.PHONY: logs-setup
logs-setup: ## Create logs directory
	@echo "$(GREEN)Creating logs directory...$(NC)"
	@mkdir -p $(LOG_DIR)
	@echo "$(GREEN)Logs directory created at $(LOG_DIR)$(NC)"

.PHONY: logs-clean
logs-clean: ## Clean log files
	@echo "$(GREEN)Cleaning log files...$(NC)"
	@rm -rf $(LOG_DIR)/*.log
	@rm -f *.log
	@echo "$(GREEN)Log files cleaned$(NC)"

.PHONY: logs-view
logs-view: ## View recent log entries
	@echo "$(GREEN)Recent log entries:$(NC)"
	@if [ -f "$(LOG_DIR)/auth.log" ]; then echo "$(YELLOW)=== Auth Service Logs ===$(NC)"; tail -20 $(LOG_DIR)/auth.log; fi
	@if [ -f "$(LOG_DIR)/game.log" ]; then echo "$(YELLOW)=== Game Service Logs ===$(NC)"; tail -20 $(LOG_DIR)/game.log; fi
	@if [ -f "$(LOG_DIR)/session.log" ]; then echo "$(YELLOW)=== Session Service Logs ===$(NC)"; tail -20 $(LOG_DIR)/session.log; fi
	@if [ -d "$(LOG_DIR)" ]; then \
		for log in $(LOG_DIR)/*.log; do \
			if [ -f "$$log" ]; then \
				echo "$(YELLOW)=== $$(basename $$log) ===$(NC)"; \
				tail -10 "$$log"; \
			fi; \
		done; \
	fi

.PHONY: logs-follow
logs-follow: ## Follow all log files in real-time
	@echo "$(GREEN)Following all log files... (Ctrl+C to stop)$(NC)"
	@tail -f *.log $(LOG_DIR)/*.log 2>/dev/null || echo "$(YELLOW)No log files found yet$(NC)"

##@ Running Services

.PHONY: stop
stop: ## Stop all running DungeonGate services
	@echo "$(GREEN)Stopping all DungeonGate services...$(NC)"
	@pkill -f "dungeongate-session-service" 2>/dev/null || true
	@pkill -f "dungeongate-auth-service" 2>/dev/null || true
	@pkill -f "dungeongate-game-service" 2>/dev/null || true
	@echo "$(GREEN)All DungeonGate services stopped$(NC)"

.PHONY: run-session
run-session: build-session setup-test-env ## Run the session service
	@echo "$(GREEN)Starting DungeonGate Session Service...$(NC)"
	./$(BUILD_DIR)/$(SESSION_BINARY_NAME) -config=$(SESSION_CONFIG)

.PHONY: run-auth
run-auth: build-auth ## Run the auth service
	@echo "$(GREEN)Starting DungeonGate Auth Service...$(NC)"
	./$(BUILD_DIR)/$(AUTH_BINARY_NAME) -config=$(AUTH_CONFIG)

.PHONY: run-game
run-game: build-game ## Run the game service
	@echo "$(GREEN)Starting DungeonGate Game Service...$(NC)"
	./$(BUILD_DIR)/$(GAME_BINARY_NAME) -config=$(GAME_CONFIG)

.PHONY: run-all
run-all: build-all setup-test-env logs-setup ## Run all services with proper startup sequence
	@echo "$(GREEN)Starting all DungeonGate services...$(NC)"
	@echo "$(YELLOW)Auth service will run on port 8081/8082$(NC)"
	@echo "$(YELLOW)Game service will run on port 8085/50051$(NC)"
	@echo "$(YELLOW)Session service will run on port 8083/2222$(NC)"
	@echo "$(YELLOW)Use Ctrl+C to stop all services$(NC)"
	@./$(BUILD_DIR)/$(AUTH_BINARY_NAME) -config=$(AUTH_CONFIG) > $(LOG_DIR)/auth.log 2>&1 & \
	 AUTH_PID=$$!; \
	 echo "Auth service started (PID: $$AUTH_PID) -> $(LOG_DIR)/auth.log"; \
	 sleep 3 && \
	 ./$(BUILD_DIR)/$(GAME_BINARY_NAME) -config=$(GAME_CONFIG) > $(LOG_DIR)/game.log 2>&1 & \
	 GAME_PID=$$!; \
	 echo "Game service started (PID: $$GAME_PID) -> $(LOG_DIR)/game.log"; \
	 sleep 3 && \
	 echo "Starting session service -> $(LOG_DIR)/session.log" && \
	 trap 'echo "Stopping services..."; kill $$AUTH_PID $$GAME_PID $$SESSION_PID 2>/dev/null; exit' INT TERM && \
	 ./$(BUILD_DIR)/$(SESSION_BINARY_NAME) -config=$(SESSION_CONFIG) > $(LOG_DIR)/session.log 2>&1 & \
	 SESSION_PID=$$!; \
	 echo "Session service started (PID: $$SESSION_PID) -> $(LOG_DIR)/session.log"; \
	 wait

.PHONY: run-file-logs
run-file-logs: build-all logs-setup ## Run all services with file logging
	@echo "$(GREEN)Starting all services with file logging...$(NC)"
	@echo "$(BLUE)Logs will be written to $(LOG_DIR)/$(NC)"
	@./$(BUILD_DIR)/$(AUTH_BINARY_NAME) -config=$(AUTH_CONFIG) > $(LOG_DIR)/auth-service.log 2>&1 & \
	 AUTH_PID=$$!; \
	 echo "Auth service started (PID: $$AUTH_PID) -> $(LOG_DIR)/auth-service.log"; \
	 sleep 3 && \
	 ./$(BUILD_DIR)/$(GAME_BINARY_NAME) -config=$(GAME_CONFIG) > $(LOG_DIR)/game-service.log 2>&1 & \
	 GAME_PID=$$!; \
	 echo "Game service started (PID: $$GAME_PID) -> $(LOG_DIR)/game-service.log"; \
	 sleep 3 && \
	 echo "Starting session service -> $(LOG_DIR)/session-service.log" && \
	 trap 'echo "Stopping services..."; kill $$AUTH_PID $$GAME_PID 2>/dev/null; exit' INT TERM && \
	 ./$(BUILD_DIR)/$(SESSION_BINARY_NAME) -config=$(SESSION_CONFIG) > $(LOG_DIR)/session-service.log 2>&1

##@ SSH Testing

.PHONY: ssh-test-connection
ssh-test-connection: ## Test SSH connection to running server
	@echo "$(GREEN)Testing SSH connection...$(NC)"
	@echo "Connecting to localhost:2222 (use Ctrl+C to exit)"
	@ssh -p 2222 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null localhost || true

.PHONY: ssh-check-server
ssh-check-server: ## Check if SSH server is running
	@echo "$(GREEN)Checking SSH server status...$(NC)"
	@if lsof -Pi :2222 -sTCP:LISTEN -t >/dev/null 2>&1; then \
		echo "$(GREEN)SSH server is running on port 2222$(NC)"; \
		lsof -Pi :2222 -sTCP:LISTEN; \
	else \
		echo "$(YELLOW)SSH server is not running on port 2222$(NC)"; \
		echo "Run 'make run-session' to start the session service"; \
	fi

##@ Database

.PHONY: db-migrate
db-migrate: ## Run database migrations
	@echo "$(GREEN)Running database migrations...$(NC)"
	@./scripts/migrate.sh up

.PHONY: db-migrate-down
db-migrate-down: ## Rollback database migrations
	@echo "$(YELLOW)Rolling back database migrations...$(NC)"
	@./scripts/migrate.sh down

.PHONY: db-reset
db-reset: ## Reset database (DESTRUCTIVE)
	@echo "$(RED)Resetting database (this will delete all data)...$(NC)"
	@read -p "Are you sure? [y/N] " -n 1 -r; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		./scripts/migrate.sh reset; \
	else \
		echo "$(YELLOW)Database reset cancelled.$(NC)"; \
	fi

##@ Container Builds

.PHONY: container-binaries
container-binaries: ## Extract compiled binaries to ./build using containers
	@./scripts/build-container.sh binaries

.PHONY: container-development
container-development: ## Build development image with full toolchain
	@./scripts/build-container.sh development

.PHONY: container-production
container-production: ## Build all production images
	@./scripts/build-container.sh production

.PHONY: container-session
container-session: ## Build session service production image
	@./scripts/build-container.sh session

.PHONY: container-auth
container-auth: ## Build auth service production image
	@./scripts/build-container.sh auth

.PHONY: container-game
container-game: ## Build game service production image
	@./scripts/build-container.sh game

.PHONY: container-all
container-all: ## Build everything (binaries + all images)
	@./scripts/build-container.sh all

##@ Container Run (Podman/Docker)

.PHONY: container-run-dev
container-run-dev: ## Run development container with volume mount
	@echo "$(GREEN)Running development container...$(NC)"
	@CONTAINER_CMD=$$(command -v podman || command -v docker); \
	$$CONTAINER_CMD run -it --rm \
		--name dungeongate-dev \
		-v $(PWD):/app \
		-p 2222:2222 \
		-p 8081:8081 \
		-p 8082:8082 \
		-p 8083:8083 \
		-p 8085:8085 \
		-p 9090:9090 \
		-p 9091:9091 \
		-p 9093:9093 \
		-p 50051:50051 \
		localhost/dungeongate:dev-$(VERSION)

.PHONY: container-run-session
container-run-session: ## Run session service container
	@echo "$(GREEN)Running session service container...$(NC)"
	@CONTAINER_CMD=$$(command -v podman || command -v docker); \
	$$CONTAINER_CMD run -d \
		--name dungeongate-session \
		-p 2222:2222 \
		-p 8083:8083 \
		-p 9093:9093 \
		-v $(PWD)/configs:/app/configs:ro \
		-v $(PWD)/assets:/app/assets:ro \
		-v $(PWD)/data:/app/data \
		-v $(PWD)/logs:/app/logs \
		localhost/dungeongate-session:$(VERSION)

.PHONY: container-run-auth
container-run-auth: ## Run auth service container
	@echo "$(GREEN)Running auth service container...$(NC)"
	@CONTAINER_CMD=$$(command -v podman || command -v docker); \
	$$CONTAINER_CMD run -d \
		--name dungeongate-auth \
		-p 8081:8081 \
		-p 8082:8082 \
		-p 9091:9091 \
		-v $(PWD)/configs:/app/configs:ro \
		-v $(PWD)/data:/app/data \
		-v $(PWD)/logs:/app/logs \
		localhost/dungeongate-auth:$(VERSION)

.PHONY: container-run-game
container-run-game: ## Run game service container
	@echo "$(GREEN)Running game service container...$(NC)"
	@CONTAINER_CMD=$$(command -v podman || command -v docker); \
	$$CONTAINER_CMD run -d \
		--name dungeongate-game \
		-p 8085:8085 \
		-p 50051:50051 \
		-p 9090:9090 \
		-v $(PWD)/configs:/app/configs:ro \
		-v $(PWD)/data:/app/data \
		-v $(PWD)/logs:/app/logs \
		localhost/dungeongate-game:$(VERSION)

.PHONY: container-run-all
container-run-all: ## Run all-in-one container
	@echo "$(GREEN)Running all-in-one container...$(NC)"
	@CONTAINER_CMD=$$(command -v podman || command -v docker); \
	$$CONTAINER_CMD run -d \
		--name dungeongate-all \
		-p 2222:2222 \
		-p 8081:8081 \
		-p 8082:8082 \
		-p 8083:8083 \
		-p 8085:8085 \
		-p 9090:9090 \
		-p 9091:9091 \
		-p 9093:9093 \
		-p 50051:50051 \
		-v $(PWD)/configs:/app/configs:ro \
		-v $(PWD)/assets:/app/assets:ro \
		-v $(PWD)/data:/app/data \
		-v $(PWD)/logs:/app/logs \
		localhost/dungeongate:$(VERSION)

.PHONY: container-stop
container-stop: ## Stop and remove all DungeonGate containers
	@echo "$(GREEN)Stopping all DungeonGate containers...$(NC)"
	@CONTAINER_CMD=$$(command -v podman || command -v docker); \
	$$CONTAINER_CMD stop dungeongate-dev dungeongate-session dungeongate-auth dungeongate-game dungeongate-all 2>/dev/null || true; \
	$$CONTAINER_CMD rm dungeongate-dev dungeongate-session dungeongate-auth dungeongate-game dungeongate-all 2>/dev/null || true

##@ Container Compose

.PHONY: compose-up
compose-up: ## Start all services using docker-compose
	@echo "$(GREEN)Starting services with docker-compose...$(NC)"
	@cd build/container && docker-compose up -d

.PHONY: compose-up-dev
compose-up-dev: ## Start development environment
	@echo "$(GREEN)Starting development environment...$(NC)"
	@cd build/container && docker-compose --profile dev up -d

.PHONY: compose-up-simple
compose-up-simple: ## Start all-in-one production service
	@echo "$(GREEN)Starting all-in-one service...$(NC)"
	@cd build/container && docker-compose --profile simple up -d

.PHONY: compose-down
compose-down: ## Stop all services and remove containers
	@echo "$(GREEN)Stopping services and removing containers...$(NC)"
	@cd build/container && docker-compose down

.PHONY: compose-logs
compose-logs: ## Show logs from all services
	@cd build/container && docker-compose logs -f

##@ Skaffold (Kubernetes Development)

.PHONY: skaffold-dev
skaffold-dev: ## Start Skaffold development mode
	@echo "$(GREEN)Starting Skaffold development mode...$(NC)"
	@cd build/container && skaffold dev --port-forward

.PHONY: skaffold-dev-local
skaffold-dev-local: ## Start Skaffold with docker-compose profile
	@echo "$(GREEN)Starting Skaffold local development...$(NC)"
	@cd build/container && skaffold dev --profile local --port-forward

.PHONY: skaffold-run
skaffold-run: ## Deploy to Kubernetes using Skaffold
	@echo "$(GREEN)Deploying to Kubernetes with Skaffold...$(NC)"
	@cd build/container && skaffold run

.PHONY: skaffold-run-prod
skaffold-run-prod: ## Deploy to production using Skaffold
	@echo "$(GREEN)Deploying to production with Skaffold...$(NC)"
	@cd build/container && skaffold run --profile prod

.PHONY: skaffold-delete
skaffold-delete: ## Delete Skaffold deployments
	@echo "$(GREEN)Deleting Skaffold deployments...$(NC)"
	@cd build/container && skaffold delete

.PHONY: skaffold-build
skaffold-build: ## Build images using Skaffold
	@echo "$(GREEN)Building images with Skaffold...$(NC)"
	@cd build/container && skaffold build

.PHONY: skaffold-debug
skaffold-debug: ## Start Skaffold in debug mode
	@echo "$(GREEN)Starting Skaffold debug mode...$(NC)"
	@cd build/container && skaffold debug --port-forward

##@ Legacy Docker (deprecated - use container-* targets)

.PHONY: docker-build-session
docker-build-session: container-session ## Build session service Docker image (deprecated)

.PHONY: docker-build-auth
docker-build-auth: container-auth ## Build auth service Docker image (deprecated)

.PHONY: docker-build-game
docker-build-game: container-game ## Build game service Docker image (deprecated)

.PHONY: docker-build-all
docker-build-all: container-production ## Build all Docker images (deprecated)

.PHONY: docker-compose-up
docker-compose-up: ## Start all services with docker-compose
	@echo "$(GREEN)Starting all services with docker-compose...$(NC)"
	docker-compose up --build

.PHONY: docker-compose-down
docker-compose-down: ## Stop and remove all containers
	@echo "$(GREEN)Stopping all services...$(NC)"
	docker-compose down

##@ Cleanup

.PHONY: clean
clean: ## Clean build artifacts
	@echo "$(GREEN)Cleaning build artifacts...$(NC)"
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR)

.PHONY: clean-all
clean-all: clean clean-test-env logs-clean ## Clean everything including test data and logs
	@echo "$(GREEN)Cleaning everything...$(NC)"
	@rm -rf vendor/
	@$(GOMOD) tidy

##@ Information

.PHONY: version
version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"

.PHONY: info
info: version ## Show project information
	@echo "Project: DungeonGate"
	@echo "Session Binary: $(SESSION_BINARY_NAME)"
	@echo "Auth Binary: $(AUTH_BINARY_NAME)"
	@echo "Game Binary: $(GAME_BINARY_NAME)"
	@echo "Build Directory: $(BUILD_DIR)"
	@echo "Log Directory: $(LOG_DIR)"
	@echo "Go Version: $$(go version)"

.PHONY: deps-check
deps-check: ## Check dependency status
	@echo "$(GREEN)Checking dependencies...$(NC)"
	$(GOMOD) list -u -m all

##@ Release

.PHONY: release-check
release-check: verify test-coverage vuln ## Run all checks for release
	@echo "$(GREEN)All release checks passed!$(NC)"

.PHONY: release-build
release-build: ## Build release binaries for all services
	@echo "$(GREEN)Building release binaries...$(NC)"
	@mkdir -p $(BUILD_DIR)/release
	@for service in session auth game; do \
		for os in linux darwin windows; do \
			for arch in amd64 arm64; do \
				if [ "$$os" = "windows" ]; then \
					ext=".exe"; \
				else \
					ext=""; \
				fi; \
				echo "Building $$service service for $$os/$$arch..."; \
				GOOS=$$os GOARCH=$$arch $(GOBUILD) $(LDFLAGS) \
					-o $(BUILD_DIR)/release/dungeongate-$$service-service-$$os-$$arch$$ext ./cmd/$$service-service; \
			done; \
		done; \
	done
	@echo "$(GREEN)Release binaries built in $(BUILD_DIR)/release/$(NC)"

##@ Legacy Aliases (Deprecated)

.PHONY: build test-run
build: build-session ## Legacy alias for build-session
test-run: run-session ## Legacy alias for run-session