# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

DungeonGate is a microservices-based platform for hosting terminal games like NetHack, Dungeon Crawl Stone Soup, and other roguelike adventures. The platform provides SSH access, user authentication, game session management, and spectating capabilities.

## Architecture

### Microservices Design
- **Session Service** (port 8083/9093): SSH server, PTY management, terminal sessions
- **Auth Service** (port 8081/8082): Centralized authentication and authorization via gRPC
- **Game Service** (port 8085/50051): Game management, configuration, and session orchestration
- **User Service** (port 8084/9084): User registration and profile management

### API Structure
Protocol Buffers with versioned APIs:
- **Auth API**: `api/proto/auth/auth_service.proto` → `pkg/api/auth/v1/`
- **Games API v1**: `api/proto/games/game_service_v1.proto` → `pkg/api/games/v1/`
- **Games API v2**: `api/proto/games/game_service_v2.proto` → `pkg/api/games/v2/`

### Key Directories
- `cmd/` - Service entry points (session-service, auth-service, game-service, user-service)
- `internal/` - Business logic organized by domain
- `pkg/` - Shared packages (config, database, encryption, ttyrec)
- `api/proto/` - Protocol Buffer definitions with versioned APIs
- `configs/` - Environment-specific configurations
- `migrations/` - Database migrations

## Development Commands

### Essential Commands
- `make deps` - Install Go dependencies
- `make deps-tools` - Install development tools (air, golangci-lint, govulncheck)

### Building Services
- `make build-session` - Build session service binary
- `make build-auth` - Build auth service binary
- `make build-game` - Build game service binary
- `make build-user` - Build user service binary
- `make build-all` - Build all service binaries

### Running Services
- `make run-session` - Run session service (SSH on port 2222, HTTP on 8083)
- `make run-auth` - Run auth service (gRPC on 8082, HTTP on 8081)
- `make run-game` - Run game service (gRPC on 50051, HTTP on 8085)
- `make run-user` - Run user service (gRPC on 9084, HTTP on 8084)
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
- Spectating system for watching games
- Database abstraction with dual-mode support (SQLite/PostgreSQL)
- gRPC communication between services
- Comprehensive configuration management
- Versioned Protocol Buffer APIs

### 🚧 In Progress
- Comprehensive test suite for Session Service
- Game service domain implementation
- Save file management
- Game configuration and path management
- Session lifecycle management

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
- **Development config**: `./configs/development/local.yaml`
- All services share the same database for consistency
- Database connection pooling and failover support

## Authentication System

- **Centralized Auth Service**: All authentication handled via dedicated gRPC service
- **JWT Tokens**: Secure token-based authentication with configurable expiration
- **No Fallback Authentication**: Session service waits for auth service availability
- **Rate Limiting**: Brute force protection (disabled in development)
- **User Management**: Registration, login, and profile management

## Configuration System

- **YAML-based**: Environment-specific configs with validation
- **Service-specific**: Each service has its own configuration file
- **Environment Variables**: Support for templating and overrides
- **Locations**:
  - `configs/development/` - Full development configurations
  - `configs/production/` - Production templates
  - `configs/testing/` - Database testing configurations

## Testing the Platform

### Test Structure and Organization

The project follows Go standard layout with tests organized as follows:

- **Unit Tests**: Co-located with source files (`*_test.go`)
- **Integration Tests**: In `/test` directory for larger test suites
- **Test Data**: Uses `/test/data` or `/test/testdata` for fixtures
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
- Go 1.24+ with modern toolchain
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

### Development Tools
- `air` - Live reload for Go applications
- `golangci-lint` - Go linting tool
- `govulncheck` - Vulnerability scanning

## Docker and Deployment

### Docker Commands
- `make docker-build-session` - Build session service image
- `make docker-build-auth` - Build auth service image
- `make docker-build-all` - Build all Docker images
- `make docker-compose-up` - Start all services with docker-compose
- `make docker-compose-dev` - Start development environment

### Database Management
- `make db-migrate` - Run database migrations
- `make db-migrate-down` - Rollback migrations
- `make db-reset` - Reset database (DESTRUCTIVE)

## Development Notes

- **Service Communication**: All inter-service communication via gRPC
- **Database Sharing**: All services use the same database for data consistency
- **Error Handling**: Services gracefully handle dependencies being unavailable
- **Modern Go Practices**: Context management, proper error handling, structured logging
- **Code Conventions**: 
  - Functions prefixed with `_` are unused/stubbed
  - Functions prefixed with `__` are deprecated
- **Security**: JWT tokens, rate limiting, input validation
- **Terminal Recording**: TTY recording for session playback and spectating

## Version Information

Use `make version` and `make info` to get current build and project information.

For detailed help with all available commands, run `make help`.