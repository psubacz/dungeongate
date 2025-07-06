# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

DungeonGate microservices-based platform for hosting terminal games like NetHack, Dungeon Crawl Stone Soup, and other roguelike adventures.
## Development Commands

### Essential Commands
- `make deps` - Install Go dependencies
- `make deps-tools` - Install development tools (air, golangci-lint, govulncheck)
- `make build` - Build session service binary
- `make build-auth` - Build auth service binary
- `make build-all` - Build all service binaries
- `make test` - Run all tests
- `make test-run` - Run SSH service on port 2222 for testing
- `make test-run-all` - Run both auth and session services for full testing
- `make fmt` - Format Go code
- `make lint` - Run linter (requires `golangci-lint`)
- `make vuln` - Check for vulnerabilities (requires `govulncheck`)
- `make verify` - Run all verification checks (format, vet, lint, test)

### Testing the Services
```bash
# Test session service only (limited functionality without auth)
make test-run          # Starts SSH server on port 2222
ssh -p 2222 localhost  # Connect to test the service

# Test complete system with authentication
make test-run-all      # Starts both auth service (port 8082) and session service (port 2222)
ssh -p 2222 localhost  # Connect with full authentication support
```

### Development Configuration
The development setup uses:
- **SQLite database** at `./data/sqlite/dungeongate-dev.db`
- **Auth Service** on port 8082 (gRPC) and 8081 (HTTP)
- **SSH server** on port 2222 (non-privileged)
- **HTTP API** on port 8083
- **Configuration**: `./configs/development/local.yaml`

## Architecture

### Core Services (Microservices Design)
- **Session Service** (primary, currently implemented): SSH server, PTY management, terminal sessions
- **Auth Service** (implemented): Centralized authentication and authorization via gRPC
- **User Service** (partial): User registration and profile management
- **Game Service** (planned): Game management and configuration

### Key Directories
- `cmd/session-service/` - Session service entry point
- `cmd/auth-service/` - Auth service entry point
- `internal/session/` - SSH server implementation, PTY bridging, session management
- `internal/auth/` - Authentication service implementation, gRPC server and client
- `internal/user/` - User registration and management
- `pkg/config/` - Configuration management
- `pkg/database/` - Database abstraction layer with dual-mode support (SQLite/PostgreSQL)
- `pkg/encryption/` - Encryption utilities
- `pkg/ttyrec/` - Terminal recording functionality
- `configs/` - Environment-specific configurations
- `migrations/` - Database migrations

### Database Architecture
The project supports dual-mode database operation:
- **Embedded mode**: SQLite for development (configured in `configs/development/local.yaml`)
- **External mode**: PostgreSQL/MySQL for production with read/write endpoint separation

### Current Implementation Status
- ✅ SSH server with terminal session management
- ✅ User registration flow via SSH terminal
- ✅ Menu system and navigation
- ✅ Game launching with PTY bridging
- ✅ Database layer with SQLite/PostgreSQL support
- ✅ Auth service with gRPC authentication
- ✅ User authentication via centralized auth service
- 🚧 Database schema validation (in progress)
- 📋 Game service (planned)

## Key Implementation Details

### SSH Server
- Runs on port 2222 in development
- Supports both anonymous and authenticated users
- Uses PTY allocation for terminal sessions
- Implements spectating functionality
- Auto-generates SSH host keys if needed

### Authentication System
- **Centralized Auth Service**: All authentication handled via dedicated gRPC service
- **No Fallback**: Session service waits for auth service to be available (no local auth fallback)
- **Resilient Design**: Service spins with user feedback when auth service is unavailable
- **JWT Tokens**: Secure token-based authentication with configurable expiration
- **User Management**: Registration, login, and profile management through auth service
- **Service Communication**: gRPC for inter-service communication with proper error handling

### User Registration
- Step-by-step flow within SSH terminal
- Progress indicators and navigation
- Real-time input validation
- Username, password, and email collection
- Terms of service acceptance

### Configuration System
- YAML-based configuration with environment variable support
- Environment-specific configs in `configs/` directory
- Validation with sensible defaults
- Supports both development and production deployments

## Testing

The project includes comprehensive test configurations:
- Unit tests: `make test`
- SSH service testing: `make test-run` then `ssh -p 2222 localhost`
- Database testing configs in `configs/testing/`

### Specialized Testing Commands

**Test Categories:**
- `make test-short` - Run quick tests only
- `make test-race` - Run tests with race detection
- `make test-coverage` - Generate coverage reports
- `make test-comprehensive` - Run all core test suites

**Component-Specific Tests:**
- `make test-ssh` - SSH server functionality tests
- `make test-auth` - Authentication system tests
- `make test-auth-simple` - Core authentication logic tests
- `make test-auth-functional` - Authentication flow tests
- `make test-spectating` - Spectating system tests
- `make test-spectating-full` - Comprehensive spectating tests

**Performance Testing:**
- `make benchmark` - Run performance benchmarks
- `make benchmark-ssh` - SSH-specific benchmarks
- `make benchmark-spectating` - Spectating system benchmarks

**SSH Connection Testing:**
- `make ssh-test-connection` - Test SSH connection to running server
- `make ssh-check-server` - Check if SSH server is running

## Docker and Deployment

### Docker Commands
- `make docker-build-session` - Build session service Docker image
- `make docker-build-auth` - Build auth service Docker image
- `make docker-build-all` - Build all Docker images
- `make docker-compose-up` - Start all services with docker-compose
- `make docker-compose-dev` - Start development environment
- `make docker-compose-down` - Stop and remove all containers
- `make docker-compose-logs` - Show logs from all services
- `make docker-test` - Test Docker images
- `make docker-clean` - Clean up Docker resources

### Database Management
- `make db-migrate` - Run database migrations
- `make db-migrate-down` - Rollback database migrations
- `make db-reset` - Reset database (DESTRUCTIVE)

### Release and Build Variants
- `make build-debug` - Build with debug symbols
- `make build-race` - Build with race detection
- `make release-build` - Build release binaries for multiple platforms
- `make release-check` - Run all checks for release

## Dependencies

Key Go modules:
- `golang.org/x/crypto` - SSH server implementation
- `github.com/golang-jwt/jwt/v5` - JWT authentication
- `github.com/lib/pq` - PostgreSQL driver
- `github.com/mattn/go-sqlite3` - SQLite driver
- `google.golang.org/grpc` - gRPC for microservices communication
- `gopkg.in/yaml.v3` - YAML configuration parsing

### Development Tools
- `air` - Live reload for Go applications
- `golangci-lint` - Go linting tool
- `govulncheck` - Vulnerability scanning for Go

## Quality Assurance

### Code Quality Commands
- `make fmt` - Format Go code
- `make fmt-check` - Check if code is formatted
- `make lint` - Run linter
- `make lint-fix` - Run linter with auto-fix
- `make vet` - Run go vet
- `make vuln` - Check for security vulnerabilities
- `make verify` - Run all verification checks (format, vet, lint, test)

### Environment Management
- `make setup` - Setup development environment
- `make setup-test-env` - Setup test environment
- `make clean` - Clean build artifacts
- `make clean-all` - Clean everything including test data
- `make clean-test-env` - Clean test environment

### Information Commands
- `make help` - Display help with all available commands
- `make version` - Show version information
- `make info` - Show project information
- `make deps-check` - Check dependency status

## Development Notes

- **Microservices Architecture**: Session and Auth services communicate via gRPC
- **Authentication**: Centralized through dedicated auth service (no fallback authentication)
- **Resilience**: Session service waits for auth service availability rather than failing
- **Database**: Dual-mode support (SQLite for dev, PostgreSQL for production)
- **Security**: JWT tokens, rate limiting, brute force protection (disabled in dev)
- **TTY Recording**: Implemented for session playback and spectating
- **Modern Go**: Uses modern Go practices with proper error handling and context management
- **Comprehensive Testing**: Specialized test suites for SSH, authentication, and spectating systems
- **Docker Support**: Full containerization with development and production configurations
- **Release Management**: Multi-platform build support and automated release checks
- **Code Conventions**: Functions prefixed with `_` are unused or stubbed (not fully implemented); functions prefixed with `__` are deprecated

## Service Dependencies

- **Session Service** requires **Auth Service** for user authentication
- Session service will wait (with user feedback) if auth service is unavailable
- Both services share the same database for user data consistency
- gRPC communication ensures type safety and performance between services