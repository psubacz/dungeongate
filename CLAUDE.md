# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

DungeonGate is a microservices-based platform for hosting terminal games like NetHack, Dungeon Crawl Stone Soup, and other roguelike adventures. The platform provides SSH access, user authentication, game session management, and spectating capabilities.

## Architecture

### Microservices Design
- **Session Service** (ports 8083/9093/2222): SSH server, PTY management, terminal sessions
- **Auth Service** (ports 8081/8082): Centralized authentication, authorization, and user management via gRPC
- **Game Service** (ports 8085/50051): Game management, configuration, and session orchestration

### API Structure
Protocol Buffers with versioned APIs:
- **Auth API v1**: `api/proto/auth/auth_service.proto` → `pkg/api/auth/v1/`
- **Games API v1**: `api/proto/games/game_service_v1.proto` → `pkg/api/games/v1/` (legacy)
- **Games API v2**: `api/proto/games/game_service_v2.proto` → `pkg/api/games/v2/` (current)

### Key Directories
- `cmd/` - Service entry points (session-service, auth-service, game-service)
- `internal/` - Business logic organized by domain
- `pkg/` - Shared packages (config, database, encryption, ttyrec, logging, metrics)
- `api/proto/` - Protocol Buffer definitions with versioned APIs
- `configs/` - Service-specific configurations
- `migrations/` - Database migrations

## Development Commands

### Essential Commands
- `make deps` - Install Go dependencies
- `make deps-tools` - Install development tools (air, golangci-lint, govulncheck)

### Building Services
- `make build-session` - Build session service binary
- `make build-auth` - Build auth service binary
- `make build-game` - Build game service binary
- `make build-all` - Build all service binaries

### Running Services
- `make run-session` - Run session service (SSH on port 2222, HTTP on 8083, gRPC on 9093)
- `make run-auth` - Run auth service (gRPC on 8082, HTTP on 8081)
- `make run-game` - Run game service (gRPC on 50051, HTTP on 8085)
- `make run-all` - Run all services with proper startup sequence

### Protocol Buffers
- `make proto-gen` - Generate Go code from all proto files
- `make proto-clean` - Clean generated protobuf files

### Testing Commands
- `make test` - Run all tests
- `make test-short` - Run short tests only
- `make test-race` - Run tests with race detection
- `make test-coverage` - Generate coverage reports
- `make test-comprehensive` - Run all core test suites

#### Specialized Testing
- `make test-ssh` - SSH server functionality tests
- `make test-auth` - Authentication system tests
- `make test-spectating` - Spectating system tests
- `make benchmark` - Run performance benchmarks

### Quality Assurance
- `make fmt` - Format Go code
- `make lint` - Run linter (requires golangci-lint)
- `make vet` - Run go vet
- `make vuln` - Check for security vulnerabilities
- `make verify` - Run all verification checks (format, vet, lint, test)

### SSH Testing
- `make ssh-test-connection` - Test SSH connection to running server
- `make ssh-check-server` - Check if SSH server is running

## Current Implementation Status

### ✅ Completed Features
- **Stateless Session Service**: Complete refactor to stateless architecture for horizontal scaling
- SSH server with terminal session management
- Centralized auth service with JWT tokens
- User registration and authentication flows
- PTY bridging and terminal recording (ttyrec)
- **Broadcast Spectating System**: Race-free spectating with dedicated channels per connection
- Database abstraction with dual-mode support (SQLite/PostgreSQL)
- gRPC communication between services
- Comprehensive configuration management
- Versioned Protocol Buffer APIs (Auth v1, Games v2)
- **Prometheus Metrics**: Comprehensive metrics collection across all services
- **Structured Logging**: Standardized logging framework with context correlation

### 🚧 In Progress
- Game service domain implementation
- Save file management
- Session lifecycle management
- Advanced game configuration and path management

### 📋 Planned Features
- NetHack integration with event broadcasting
- Death event broadcasting system
- Container/Kubernetes deployment
- Game statistics and leaderboards

## Database Architecture

The project supports dual-mode database operation:
- **Development**: SQLite at `./data/sqlite/dungeongate-dev.db`
- **Production**: PostgreSQL/MySQL with read/write endpoint separation

### Configuration
- **Service configs**: Individual YAML files per service inherit from `./configs/common.yaml`
- All services share the same database for consistency
- Database connection pooling and failover support

## Authentication System

- **Centralized Auth Service**: All authentication handled via dedicated gRPC service
- **JWT Tokens**: Secure token-based authentication with configurable expiration
- **No Fallback Authentication**: Session service waits for auth service availability
- **Rate Limiting**: Brute force protection (disabled in development)
- **User Management**: Registration, login, and profile management

## Configuration System

- **YAML-based**: Service-specific configs with inheritance from common.yaml
- **Environment Variables**: Support for templating and overrides
- **Locations**:
  - `configs/session-service.yaml` - Session service configuration
  - `configs/auth-service.yaml` - Auth service configuration
  - `configs/game-service.yaml` - Game service configuration
  - `configs/common.yaml` - Shared configuration base

## Service Communication

### Port Assignment
**Session Service:**
- SSH: 2222
- HTTP API: 8083
- gRPC: 9093
- Metrics: 8085

**Auth Service:**
- HTTP API: 8081
- gRPC: 8082
- Metrics: 9091

**Game Service:**
- HTTP API: 8085
- gRPC: 50051
- Metrics: 9090

### gRPC APIs
- **Auth Service**: Uses Auth API v1 for authentication operations
- **Game Service**: Uses Games API v2 for game management and session orchestration
- **Session Service**: Consumer of both Auth v1 and Games v2 APIs

## Spectating System Architecture

### Broadcast System
The spectating system uses a **broadcast architecture** that eliminates race conditions between player and spectator connections:

- **PTY Output Broadcast**: Each PTY output is broadcast to all connected subscribers
- **Dedicated Channels**: Each connection (player or spectator) gets its own output channel
- **Subscription System**: Unique subscription IDs for automatic cleanup
- **Race-Free Design**: Eliminates every-other-frame skipping issues

### Key Components
- **PTY Manager**: Handles broadcast to multiple subscribers (`internal/games/infrastructure/pty/manager.go`)
- **Stream Manager**: Manages spectator-specific features with internal registry (`internal/games/types.go`)
- **gRPC Streaming**: Unified streaming endpoint for both players and spectators (`internal/games/infrastructure/grpc/streaming.go`)
- **Session Service**: Handles spectator registration and input filtering (`internal/session/connection/handler.go`)

### Connection Flow
1. **Player**: Starts session → connects to StreamGameIO → gets dedicated channel
2. **Spectator**: Added via AddSpectator → connects to StreamGameIO → gets dedicated channel
3. **Broadcast**: PTY output sent to all subscribed channels simultaneously
4. **Input**: Player input forwarded to game, spectator input filtered at session service level

## Testing the Platform

### Test Structure and Organization

The project follows Go standard layout with tests organized as follows:

- **Unit Tests**: Co-located with source files (`*_test.go`)
- **Integration Tests**: In `/test` directory for larger test suites
- **Test Data**: Uses `/test-data` directory for fixtures
- **Mock Objects**: Extensive use of `testify/mock` for dependency isolation

### Core Test Commands

```bash
# Run all tests
make test

# Run with coverage report
make test-coverage

# Run specific test suites
make test-ssh          # SSH server functionality
make test-auth         # Authentication system
make test-spectating   # Spectating system
make test-race         # Race condition detection
make test-short        # Quick tests only
```

### Session Service Testing

The stateless Session Service includes comprehensive test coverage:

- **Connection Management**: Connection lifecycle, rate limiting, cleanup
- **SSH Server**: Authentication, channel handling, PTY management
- **Game Client**: gRPC communication with Game Service
- **Auth Client**: gRPC communication with Auth Service
- **Terminal Manager**: PTY creation, resizing, process management
- **Streaming Manager**: Spectator streaming, stream lifecycle
- **Service Integration**: End-to-end service functionality

### Test Categories

#### Unit Tests
- **Components**: Individual service components and managers
- **Business Logic**: Core functionality and edge cases
- **Error Handling**: Proper error propagation and recovery
- **Concurrency**: Thread safety and race condition prevention

#### Integration Tests
- **Service Communication**: gRPC client-server interactions
- **Database Operations**: Repository patterns and data persistence
- **File System**: PTY management and session cleanup
- **Network**: SSH connections and protocol handling

#### Performance Tests
- **Benchmarks**: Critical path performance measurement
- **Load Testing**: Connection limits and resource management
- **Memory Usage**: Memory leak detection and optimization

### Quick Start Testing
```bash
# Start all services
make run-all

# In another terminal, connect via SSH
ssh -p 2222 localhost
```

### Service Testing
```bash
# Test session service only (limited functionality)
make run-session
ssh -p 2222 localhost

# Test with full authentication
make run-all
ssh -p 2222 localhost
```

## Game Integration

### NetHack Integration Points
- **Log Monitoring**: Parse `xlogfile` for real-time game events
- **Event Broadcasting**: Death events with bones file detection
- **Statistics Tracking**: Player progress and achievements
- **Save Management**: Game save file handling and backup

### Planned Death Event Broadcasting
When a player dies in NetHack:
1. Detect death events from logfile/xlogfile
2. Check for bones file generation
3. Broadcast contextual message via gRPC
4. Display message in session service menu footer

## Dependencies

### Core Technologies
- Go 1.24.0+ with modern toolchain
- gRPC for inter-service communication
- Protocol Buffers for API definitions
- SQLite (dev) / PostgreSQL (prod) databases
- JWT authentication

### Key Go Modules
- `golang.org/x/crypto` - SSH server implementation
- `github.com/golang-jwt/jwt/v5` - JWT authentication
- `google.golang.org/grpc` - gRPC framework
- `github.com/lib/pq` - PostgreSQL driver
- `github.com/mattn/go-sqlite3` - SQLite driver
- `gopkg.in/yaml.v3` - YAML configuration parsing
- `github.com/prometheus/client_golang` - Metrics collection
- `github.com/stretchr/testify` - Testing framework

### Development Tools
- `air` - Live reload for Go applications
- `golangci-lint` - Go linting tool
- `govulncheck` - Vulnerability scanning

## Docker and Deployment

### Docker Commands
- `make docker-build-session` - Build session service image
- `make docker-build-auth` - Build auth service image
- `make docker-build-game` - Build game service image
- `make docker-build-all` - Build all Docker images
- `make docker-compose-up` - Start all services with docker-compose
- `make docker-compose-down` - Stop and remove all containers

### Database Management
- `make db-migrate` - Run database migrations
- `make db-migrate-down` - Rollback migrations
- `make db-reset` - Reset database (DESTRUCTIVE)

## Observability

### Metrics Collection
All services implement comprehensive Prometheus metrics:
- **Session Service**: SSH connections, terminal operations, session lifecycle
- **Auth Service**: Authentication attempts, token operations, security events
- **Game Service**: Game instances, resource usage, session duration

**Metrics Endpoints:**
- Session Service: `:8085/metrics`
- Auth Service: `:9091/metrics`
- Game Service: `:9090/metrics`

### Structured Logging
Standardized logging using `pkg/logging` with:
- Context correlation (session_id, user_id, etc.)
- Structured fields for searchability
- Service-specific log files in `logs/` directory
- Support for file rotation and journald output

## Development Notes

- **Service Communication**: All inter-service communication via gRPC
- **Database Sharing**: All services use the same database for data consistency
- **Error Handling**: Services gracefully handle dependencies being unavailable
- **Modern Go Practices**: Context management, proper error handling, structured logging
- **Code Conventions**: 
  - Functions prefixed with `_` are unused/stubbed
  - Functions prefixed with `__` are deprecated
  - **NEVER hardcode paths in the code** - all paths must come from configuration files or environment variables
  - Game adapters should read configuration from `configs/game-service.yaml` 
  - Use configuration injection rather than hardcoded values
- **Security**: JWT tokens, rate limiting, input validation
- **Terminal Recording**: TTY recording for session playback and spectating
- **Observability**: All new features MUST include metrics and structured logging

### Logging Standards

All new features and redesigned components MUST implement comprehensive structured logging using the standardized logging framework from `pkg/logging`.

#### Mandatory Logging Requirements

**✅ Required for ALL new features:**
- **Structured Logging**: Use `*slog.Logger` from `pkg/logging` package
- **Context Correlation**: Include request/session/user IDs for traceability
- **Error Logging**: Log all errors with context and stack traces where appropriate
- **Performance Logging**: Log operation durations for critical paths
- **Security Events**: Log authentication, authorization, and security-related events

### Metrics Standards

All new features and redesigned components MUST implement Prometheus metrics for monitoring and alerting.

#### Mandatory Metrics Requirements

**✅ Required for ALL new features:**
- **Operation Counters**: Track total operations with success/failure labels
- **Duration Histograms**: Measure operation performance with appropriate buckets
- **Active Gauge Metrics**: Track concurrent operations and resource usage
- **Error Rate Tracking**: Count and categorize errors for alerting
- **Business Metrics**: Track domain-specific KPIs and user behavior

## Version Information

Use `make version` and `make info` to get current build and project information.

For detailed help with all available commands, run `make help`.