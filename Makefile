# Makefile for DungeonGate
# Unified build and test configuration

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
BUILD_DIR=build
SESSION_MAIN_PATH=./cmd/session-service
AUTH_MAIN_PATH=./cmd/auth-service
GAME_MAIN_PATH=./cmd/game-service
TEST_DATA_DIR=test-data
CONFIG_DIR=configs

# Version and build info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

# Test configuration
SESSION_CONFIG=configs/development/session-service.yaml
AUTH_CONFIG=configs/development/auth-service.yaml
GAME_CONFIG=configs/development/game-service.yaml
# Legacy config (deprecated - use service-specific configs)
TEST_TIMEOUT=30s
TEST_COVERAGE_DIR=coverage

# Colors for output
GREEN=\033[0;32m
YELLOW=\033[1;33m
RED=\033[0;31m
NC=\033[0m # No Color

.DEFAULT_GOAL := help

##@ General

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

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

##@ Build Individual Services

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

# Legacy alias for backward compatibility
.PHONY: build
build: build-session ## Build the session service binary (legacy alias)

.PHONY: build-debug
build-debug: deps ## Build session service with debug symbols
	@echo "$(GREEN)Building $(SESSION_BINARY_NAME) with debug symbols...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -gcflags="all=-N -l" -o $(BUILD_DIR)/$(SESSION_BINARY_NAME)-debug $(SESSION_MAIN_PATH)

.PHONY: build-race
build-race: deps ## Build session service with race detection
	@echo "$(GREEN)Building $(SESSION_BINARY_NAME) with race detection...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -race -o $(BUILD_DIR)/$(SESSION_BINARY_NAME)-race $(SESSION_MAIN_PATH)

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

.PHONY: test-ssh
test-ssh: ## Run SSH-specific tests
	@echo "$(GREEN)Running SSH tests...$(NC)"
	$(GOTEST) -v -timeout=$(TEST_TIMEOUT) -run "SSH" ./internal/session/...

.PHONY: test-spectating
test-spectating: ## Run spectating system tests
	@echo "$(GREEN)Running spectating tests...$(NC)"
	$(GOTEST) -v -timeout=$(TEST_TIMEOUT) -run "Spectating|Spectator|Registry|Stream" ./internal/session/...

.PHONY: test-auth
test-auth: ## Run authentication tests
	@echo "$(GREEN)Running authentication tests...$(NC)"
	$(GOTEST) -v -timeout=$(TEST_TIMEOUT) -run "TestAuth|TestUser.*Conversion|TestError.*Mapping|TestMax.*Logic|TestSession.*Setup|TestFallback" ./internal/session/...

.PHONY: test-auth-simple
test-auth-simple: ## Run core authentication logic tests
	@echo "$(GREEN)Running core authentication logic tests...$(NC)"
	$(GOTEST) -v -timeout=$(TEST_TIMEOUT) -run "TestUser.*Conversion|TestAuth.*PathSelection|TestAuth.*RetryLogic|TestError.*Mapping" ./internal/session/...

.PHONY: test-auth-functional
test-auth-functional: ## Run authentication flow tests
	@echo "$(GREEN)Running authentication flow tests...$(NC)"
	$(GOTEST) -v -timeout=$(TEST_TIMEOUT) -run "TestAuth.*FallbackLogic|TestAuth.*ErrorHandling|TestMax.*Logic|TestSession.*Setup|TestFallback.*Flow" ./internal/session/...

.PHONY: test-cover
test-cover: ## Run tests and show coverage percentages
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	$(GOTEST) -cover ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	@mkdir -p $(TEST_COVERAGE_DIR)
	$(GOTEST) -coverprofile=$(TEST_COVERAGE_DIR)/coverage.out ./...
	$(GOCMD) tool cover -html=$(TEST_COVERAGE_DIR)/coverage.out -o $(TEST_COVERAGE_DIR)/coverage.html
	@echo "$(GREEN)Coverage report: $(TEST_COVERAGE_DIR)/coverage.html$(NC)"

.PHONY: test-integration
test-integration: build-session setup-test-env ## Run integration tests
	@echo "$(GREEN)Running integration tests...$(NC)"
	@./scripts/test-integration.sh

.PHONY: test-comprehensive
test-comprehensive: ## Run all core test suites (auth, SSH, spectating)
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

.PHONY: benchmark-ssh
benchmark-ssh: ## Run SSH-specific benchmarks
	@echo "$(GREEN)Running SSH benchmarks...$(NC)"
	$(GOTEST) -bench="SSH" -benchmem -timeout=5m ./internal/session/...

.PHONY: benchmark-spectating
benchmark-spectating: ## Run spectating system benchmarks
	@echo "$(GREEN)Running spectating benchmarks...$(NC)"
	$(GOTEST) -bench="Spectating|Registry|Stream" -benchmem -timeout=5m ./internal/session/...

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

##@ Environment Setup

.PHONY: setup
setup: ## Setup development environment
	@echo "$(GREEN)Setting up development environment...$(NC)"
	@./scripts/dev-setup.sh

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

##@ Running Individual Services

.PHONY: stop
stop: ## Stop all running DungeonGate services
	@echo "$(GREEN)Stopping all DungeonGate services...$(NC)"
	@pkill -f "dungeongate-session-service" 2>/dev/null || true
	@pkill -f "dungeongate-auth-service" 2>/dev/null || true
	@pkill -f "dungeongate-game-service" 2>/dev/null || true
	@echo "$(GREEN)All DungeonGate services stopped$(NC)"

.PHONY: run-session
run-session: build-session setup-test-env ## Build and run the session service
	@echo "$(GREEN)Starting DungeonGate Session Service...$(NC)"
	./$(BUILD_DIR)/$(SESSION_BINARY_NAME) -config=$(SESSION_CONFIG)

.PHONY: run-auth
run-auth: build-auth ## Build and run the auth service
	@echo "$(GREEN)Starting DungeonGate Auth Service...$(NC)"
	./$(BUILD_DIR)/$(AUTH_BINARY_NAME) -config=$(AUTH_CONFIG)

.PHONY: run-game
run-game: build-game ## Build and run the game service
	@echo "$(GREEN)Starting DungeonGate Game Service...$(NC)"
	./$(BUILD_DIR)/$(GAME_BINARY_NAME) -config=$(GAME_CONFIG)

.PHONY: run-all
run-all: build-all setup-test-env ## Build and run all services
	@echo "$(GREEN)Starting all DungeonGate services...$(NC)"
	@echo "$(YELLOW)Auth service will run on port 8081/8082$(NC)"
	@echo "$(YELLOW)Game service will run on port 8085/50051$(NC)"
	@echo "$(YELLOW)Session service will run on port 8083/2222$(NC)"
	@echo "$(YELLOW)Use Ctrl+C to stop all services$(NC)"
	@(./$(BUILD_DIR)/$(AUTH_BINARY_NAME) -config=$(AUTH_CONFIG) &) && \
	 sleep 2 && \
	 (./$(BUILD_DIR)/$(GAME_BINARY_NAME) -config=$(GAME_CONFIG) &) && \
	 sleep 2 && \
	 ./$(BUILD_DIR)/$(SESSION_BINARY_NAME) -config=$(SESSION_CONFIG)

.PHONY: run-debug
run-debug: build-debug ## Run session service with debug build
	@echo "$(GREEN)Starting DungeonGate Session Service (debug)...$(NC)"
	./$(BUILD_DIR)/$(SESSION_BINARY_NAME)-debug -config=$(TEST_CONFIG)

# Legacy alias for backward compatibility
.PHONY: run
run: run-session ## Run the session service (legacy alias)


##@ Legacy Aliases

# Legacy test aliases (deprecated - use run-* targets instead)
.PHONY: test-run
test-run: run-session ## Run session service (legacy alias)

.PHONY: test-run-auth
test-run-auth: run-auth ## Run auth service (legacy alias)

.PHONY: test-run-game
test-run-game: run-game ## Run game service (legacy alias)

.PHONY: test-run-all
test-run-all: run-all ## Run all services (legacy alias)

##@ Docker

.PHONY: docker-build-session
docker-build-session: ## Build session service Docker image
	@echo "$(GREEN)Building session service Docker image...$(NC)"
	docker build -f Dockerfile.session-service -t dungeongate/session-service:$(VERSION) .
	docker tag dungeongate/session-service:$(VERSION) dungeongate/session-service:latest

.PHONY: docker-build-auth
docker-build-auth: ## Build auth service Docker image
	@echo "$(GREEN)Building auth service Docker image...$(NC)"
	docker build -f Dockerfile.auth-service -t dungeongate/auth-service:$(VERSION) .
	docker tag dungeongate/auth-service:$(VERSION) dungeongate/auth-service:latest

.PHONY: docker-build-all
docker-build-all: docker-build-session docker-build-auth ## Build all Docker images

.PHONY: docker-compose-up
docker-compose-up: ## Start all services with docker-compose
	@echo "$(GREEN)Starting all services with docker-compose...$(NC)"
	docker-compose up --build

.PHONY: docker-compose-dev
docker-compose-dev: ## Start development environment with docker-compose
	@echo "$(GREEN)Starting development environment...$(NC)"
	docker-compose -f docker-compose.dev.yml up --build

.PHONY: docker-compose-down
docker-compose-down: ## Stop and remove all containers
	@echo "$(GREEN)Stopping all services...$(NC)"
	docker-compose down
	docker-compose -f docker-compose.dev.yml down

.PHONY: docker-compose-logs
docker-compose-logs: ## Show logs from all services
	docker-compose logs -f

.PHONY: docker-test
docker-test: docker-build-all ## Test Docker images
	@echo "$(GREEN)Testing Docker images...$(NC)"
	docker run --rm dungeongate/session-service:$(VERSION) --version
	docker run --rm dungeongate/auth-service:$(VERSION) --version

.PHONY: docker-clean
docker-clean: ## Clean up Docker images and containers
	@echo "$(GREEN)Cleaning up Docker resources...$(NC)"
	docker-compose down --volumes --remove-orphans
	docker-compose -f docker-compose.dev.yml down --volumes --remove-orphans
	docker system prune -f

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

##@ Cleanup

.PHONY: clean
clean: ## Clean build artifacts
	@echo "$(GREEN)Cleaning build artifacts...$(NC)"
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR)

.PHONY: clean-all
clean-all: clean clean-test-env ## Clean everything including test data
	@echo "$(GREEN)Cleaning everything...$(NC)"
	@rm -rf vendor/
	@$(GOMOD) tidy

##@ Information

.PHONY: version
version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"

.PHONY: deps-check
deps-check: ## Check dependency status
	@echo "$(GREEN)Checking dependencies...$(NC)"
	$(GOMOD) list -u -m all

.PHONY: info
info: version ## Show project information
	@echo "Project: DungeonGate"
	@echo "Session Binary: $(SESSION_BINARY_NAME)"
	@echo "Auth Binary: $(AUTH_BINARY_NAME)"
	@echo "Game Binary: $(GAME_BINARY_NAME)"
	@echo "Build Directory: $(BUILD_DIR)"
	@echo "Test Config: $(TEST_CONFIG)"
	@echo "Go Version: $$(go version)"

##@ Release

.PHONY: release-check
release-check: verify test-coverage vuln ## Run all checks for release
	@echo "$(GREEN)All release checks passed!$(NC)"

.PHONY: release-build
release-build: ## Build release binaries for all services
	@echo "$(GREEN)Building release binaries...$(NC)"
	@mkdir -p $(BUILD_DIR)/release
	@for service in session auth game user; do \
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
		echo "Run 'make test-run' to start the test server"; \
	fi

##@ Spectating System

.PHONY: test-spectating-full
test-spectating-full: ## Run comprehensive spectating system tests
	@echo "$(GREEN)Running comprehensive spectating tests...$(NC)"
	$(GOTEST) -v -timeout=60s -run "Spectating|Spectator|Registry|Stream|Immutable" ./internal/session/...
	@echo "$(GREEN)Running spectating benchmarks...$(NC)"
	$(GOTEST) -bench="Spectating|Registry|Stream" -benchmem -timeout=5m ./internal/session/...

.PHONY: demo-spectating
demo-spectating: build setup-test-env ## Demo spectating functionality
	@echo "$(GREEN)Starting spectating demonstration...$(NC)"
	@echo "This will start the server and show how to test spectating"
	@echo "1. Server will start on port 2222"
	@echo "2. Connect with: ssh -p 2222 localhost"
	@echo "3. Select 'w' to watch games"
	@echo "4. Test sessions will be available"
	@echo ""
	@echo "Press Enter to continue..."
	@read
	@$(MAKE) test-run

# Protocol Buffers
PROTOC=protoc
PROTO_DIR=api/proto
GO_OUT_DIR=pkg/api

.PHONY: proto-gen
proto-gen: ## Generate Go code from Protocol Buffers
	@echo "$(GREEN)Generating Go code from protobuf definitions...$(NC)"
	@mkdir -p $(GO_OUT_DIR)/auth/v1 $(GO_OUT_DIR)/games/v1 $(GO_OUT_DIR)/games/v2
	$(PROTOC) --go_out=$(GO_OUT_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GO_OUT_DIR) --go-grpc_opt=paths=source_relative \
		-I $(PROTO_DIR) $(PROTO_DIR)/auth/auth_service.proto
	$(PROTOC) --go_out=$(GO_OUT_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GO_OUT_DIR) --go-grpc_opt=paths=source_relative \
		-I $(PROTO_DIR) $(PROTO_DIR)/games/game_service_v1.proto
	$(PROTOC) --go_out=$(GO_OUT_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GO_OUT_DIR) --go-grpc_opt=paths=source_relative \
		-I $(PROTO_DIR) $(PROTO_DIR)/games/game_service_v2.proto
	@mv $(GO_OUT_DIR)/auth/auth_service.pb.go $(GO_OUT_DIR)/auth/v1/
	@mv $(GO_OUT_DIR)/auth/auth_service_grpc.pb.go $(GO_OUT_DIR)/auth/v1/
	@mv $(GO_OUT_DIR)/games/game_service_v1.pb.go $(GO_OUT_DIR)/games/v1/
	@mv $(GO_OUT_DIR)/games/game_service_v1_grpc.pb.go $(GO_OUT_DIR)/games/v1/
	@mv $(GO_OUT_DIR)/games/game_service_v2.pb.go $(GO_OUT_DIR)/games/v2/
	@mv $(GO_OUT_DIR)/games/game_service_v2_grpc.pb.go $(GO_OUT_DIR)/games/v2/

.PHONY: proto-clean
proto-clean: ## Clean generated protobuf files
	@echo "$(GREEN)Cleaning generated protobuf files...$(NC)"
	@rm -rf $(GO_OUT_DIR)/auth $(GO_OUT_DIR)/games $(GO_OUT_DIR)/github.com


.PHONY: test-game-service
test-game-service: ## Test game service
	@echo "$(GREEN)Testing game service...$(NC)"
	$(GOTEST) -v -timeout=30s ./internal/games/...

.PHONY: test-run-game
test-run-game: build-game-service ## Run test game service on port 50051
	@echo "$(GREEN)Starting test game service...$(NC)"
	./$(BUILD_DIR)/dungeongate-game-service -config=$(TEST_CONFIG)