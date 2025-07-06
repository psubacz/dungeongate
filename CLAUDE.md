# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

DungeonGate microservices-based platform for hosting terminal games like NetHack, Dungeon Crawl Stone Soup, and other roguelike adventures.
## Development Commands

### Essential Commands
- `make deps` - Install Go dependencies
- `make build` - Build the session service binary
- `make dev` - Run development server with auto-restart (requires `air`)
- `make test` - Run all tests
- `make test-run` - Run SSH service on port 2222 for testing
- `make fmt` - Format Go code
- `make lint` - Run linter (requires `golangci-lint`)
- `make vuln` - Check for vulnerabilities (requires `govulncheck`)

### Testing the SSH Service
```bash
make test-run          # Starts SSH server on port 2222
ssh -p 2222 localhost  # Connect to test the service
```

### Development Configuration
The development setup uses:
- **SQLite database** at `./data/sqlite/dungeongate-dev.db`
- **SSH server** on port 2222 (non-privileged)
- **HTTP API** on port 8083
- **Configuration**: `./configs/development/local.yaml`

## Architecture

### Core Services (Microservices Design)
- **Session Service** (primary, currently implemented): SSH server, PTY management, terminal sessions
- **User Service** (partial): User registration, authentication, profiles
- **Auth Service** (planned): Centralized authentication and authorization  
- **Game Service** (planned): Game management and configuration

### Key Directories
- `cmd/session-service/` - Main application entry point
- `internal/session/` - SSH server implementation, PTY bridging, session management
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
- 🚧 User authentication (in progress)
- 🚧 Database schema validation (in progress)
- 📋 Auth service (planned)
- 📋 Game service (planned)

## Key Implementation Details

### SSH Server
- Runs on port 2222 in development
- Supports both anonymous and authenticated users
- Uses PTY allocation for terminal sessions
- Implements spectating functionality
- Auto-generates SSH host keys if needed

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

## Dependencies

Key Go modules:
- `golang.org/x/crypto` - SSH server implementation
- `github.com/golang-jwt/jwt/v5` - JWT authentication
- `github.com/lib/pq` - PostgreSQL driver
- `github.com/mattn/go-sqlite3` - SQLite driver
- `google.golang.org/grpc` - gRPC for microservices communication
- `gopkg.in/yaml.v3` - YAML configuration parsing

## Development Notes

- The project is actively developed with focus on completing the user service
- Current work involves SSH-based user registration and authentication
- Database implementation is designed but needs validation
- Uses modern Go practices and microservices architecture
- Security features include rate limiting, brute force protection (disabled in dev)
- TTY recording is implemented for session playback and spectating