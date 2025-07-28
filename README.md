# DungeonGate
```bash
 ____
|  _ \ _   _ _ __   __ _  ___  ___  _ __
| | | | | | | ._ \ / _. |/ _ \/ _ \| ._ \
| |_| | |_| | | | | (_| |  __/ (_) | | | |
|____/ \__,_|_| |_|\__, |\___|\____|_| |_|
        ___        |___/
       / __|  __ _| |_ ___
      | |___ / _. | __/ _ \
      | |__ | (_| |  ||  _/
      |____/ \__,_|\__\___|
```

**A SSH-based gateway to terminal gaming adventures written in Go**

DungeonGate is a microservices-based platform inspired by [dgamelaunch](https://github.com/paxed/dgamelaunch) for hosting terminal games like NetHack. This software provides an SSH frontend where users can login to play or spectate games in progress.

**Supported games:**
- NetHack

## 🚀 Quick Start

### Prerequisites

- **Go 1.24.0+** - For building the application
- **NetHack** - The terminal game we'll be hosting
- **Make** - Build automation (optional but recommended)

### Install NetHack

```bash
# macOS
brew install nethack

# Ubuntu/Debian
sudo apt-get install nethack

# Arch Linux
sudo pacman -S nethack
```

### Setup and Run

1. **Clone and setup the project:**
```bash
git clone https://github.com/your-username/dungeongate.git
cd dungeongate
make deps  # Install Go dependencies
```

2. **Configure NetHack path** (if needed):
Edit `configs/session-service.yaml` and update the NetHack binary path:
```yaml
games:
  - id: "nethack"
    binary:
      path: "/opt/homebrew/bin/nethack"  # Update to your NetHack path
```

3. **Build and run:**
```bash
# Run only the session service (basic functionality)
make run-session

# OR run all services together (full functionality)
make run-all
```

This will:
- Build the required service binaries
- Start the SSH server on port 2222 (non-privileged)
- Start auth service on port 8081/8082 (HTTP/gRPC)
- Start game service on port 8085/50051 (HTTP/gRPC)
- Use SQLite database in `./data/sqlite/`
- Enable anonymous access for testing

### Connect and Play

```bash
ssh -p 2222 localhost
```

You'll see a welcome banner with menu options:
- **[l]** Login (if you have an account)
- **[r]** Register a new account
- **[w]** Watch active games
- **[g]** List available games
- **[q]** Quit

After registering/logging in, you get additional options:
- **[p]** Play a game
- **[e]** Edit profile
- **[s]** View statistics

### View Metrics

The services provide Prometheus metrics endpoints:
- Session Service: http://localhost:8085/metrics
- Auth Service: http://localhost:9091/metrics
- Game Service: http://localhost:9090/metrics

## 🛠️ Development Commands

Essential commands for development:

```bash
# Dependencies and setup
make deps           # Install Go dependencies
make deps-tools     # Install development tools (air, golangci-lint, govulncheck)

# Building services
make build-session  # Build session service binary
make build-auth     # Build auth service binary
make build-game     # Build game service binary
make build-all      # Build all service binaries

# Running services
make run-session    # Run session service (SSH on port 2222)
make run-auth       # Run auth service (HTTP 8081, gRPC 8082)
make run-game       # Run game service (HTTP 8085, gRPC 50051)
make run-all        # Run all services with proper startup sequence

# Testing and quality
make test           # Run all tests
make test-coverage  # Run tests with coverage report
make test-short     # Run short tests only
make test-race      # Run tests with race detection
make benchmark      # Run performance benchmarks
make fmt            # Format Go code
make lint           # Run linter (requires golangci-lint)
make vuln           # Check for vulnerabilities (requires govulncheck)
```

### Testing the Services

```bash
# Test complete system
make run-all           # Start all services with full functionality

# Connect to test
ssh -p 2222 localhost  # Connect to test the SSH service

# Test individual services (limited functionality)
make run-session       # Start session service only
make run-auth          # Start auth service only
make run-game          # Start game service only
```

### Troubleshooting

**Services already running:**
Kill all previous runs before starting new ones:
```bash
make stop
```

**Port 2222 in use:**
```bash
lsof -ti:2222 | xargs kill -9
```

**SSH host key conflicts:**
Remove the localhost entry from your `~/.ssh/known_hosts` file if you get host key warnings.

**Permission errors:**
Ensure data directories exist and are writable:
```bash
mkdir -p ./data/sqlite ./logs
chmod 755 ./data/sqlite ./logs
```

**NetHack not found:**
Update the game path in `configs/session-service.yaml` to match your NetHack installation.

## 🏗️ Architecture

DungeonGate uses a microservices architecture with three core services:

### Service Architecture

- **Session Service** (ports 2222/8083/9093) - Handles SSH connections, PTY management, and user sessions
- **Auth Service** (ports 8081/8082) - Centralized authentication, authorization, and user management via gRPC
- **Game Service** (ports 8085/50051) - Game management, configuration, and session orchestration

Each service can be built and run independently, or all services can be run together using `make run-all`.

### Service Communication

Services communicate via gRPC:
- **Auth API v1**: Authentication and user management
- **Games API v2**: Game management and session orchestration
- Session service consumes both APIs

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

## 📁 Project Structure

```
dungeongate/
├── cmd/
│   ├── session-service/      # Session service entry point
│   ├── auth-service/         # Auth service entry point
│   └── game-service/         # Game service entry point
├── internal/
│   ├── session/             # SSH server, PTY bridging, session management
│   ├── auth/               # Authentication service implementation
│   ├── games/              # Game management implementation
│   └── user/               # User management (integrated into auth)
├── pkg/
│   ├── config/             # Configuration management
│   ├── database/           # Database abstraction layer (SQLite/PostgreSQL)
│   ├── encryption/         # Encryption utilities
│   ├── logging/            # Structured logging framework
│   ├── metrics/            # Prometheus metrics
│   └── ttyrec/            # Terminal recording functionality
├── assets/               # Static assets following Go community conventions
│   └── banners/          # Banner templates with variable substitution
├── configs/               # Service-specific configurations
│   ├── session-service.yaml # Session service configuration
│   ├── auth-service.yaml    # Auth service configuration
│   ├── game-service.yaml    # Game service configuration
│   └── common.yaml          # Shared configuration base
├── api/proto/             # Protocol Buffer definitions
├── migrations/            # Database migration files
└── scripts/              # Build and deployment scripts
```

## 🔧 Configuration

### Development Configuration

The development setup uses SQLite for simplicity and individual service configurations:

```yaml
# configs/session-service.yaml
version: "0.4.0"
inherit_from: "common.yaml"

server:
  port: 8083
  grpc_port: 9093
  host: "localhost"

ssh:
  enabled: true
  port: 2222
  host: "localhost"
  auth:
    allow_anonymous: true

games:
  - id: "nethack"
    name: "NetHack"
    enabled: true
    binary:
      path: "/opt/homebrew/bin/nethack"
```

### Service Configuration Files

Each service has its own configuration file that inherits common settings:

- `configs/session-service.yaml` - SSH server, terminal management, game integration
- `configs/auth-service.yaml` - Authentication, JWT tokens, user management
- `configs/game-service.yaml` - Game execution, path management, resource limits
- `configs/common.yaml` - Shared database, logging, and base configuration

### Production Configuration

Production configuration supports external databases with full read/write separation:

```yaml
# configs/common.yaml (production values)
database:
  type: "postgresql"
  connection:
    host: "${DB_HOST}"
    port: "${DB_PORT}"
    database: "dungeongate"
    username: "${DB_USER}"
    password: "${DB_PASSWORD}"
    ssl_mode: "require"

logging:
  level: "info"
  format: "json"
  output: "journald"
```

## 🎨 Banner System

DungeonGate features a dynamic banner system with template variables:

### Banner Templates

**Anonymous User Banner** (`assets/banners/main_anon.txt`):
```
Welcome to DungeonGate!

Connected as: Anonymous
Date: $DATE | Time: $TIME

Menu Options:
  [l] Login
  [r] Register
  [w] Watch games
  [g] List games
  [q] Quit
```

**Authenticated User Banner** (`assets/banners/main_user.txt`):
```
Welcome back to DungeonGate, $USERNAME!

Authenticated User: $USERNAME
Date: $DATE | Time: $TIME

Menu Options:
  [p] Play a game
  [w] Watch games
  [e] Edit profile
  [l] List games
  [r] View recordings
  [s] Statistics
  [q] Quit
```

### Template Variables
- `$USERNAME` - Current username or "Anonymous"
- `$DATE` - Current date (YYYY-MM-DD)
- `$TIME` - Current time (HH:MM:SS)

### Features
- **📏 Automatic resizing** - Banners adapt to terminal width
- **🔄 Real-time replacement** - Variables updated on each display
- **⚙️ Configurable version footer** - Shows version from config file
- **🎯 Left-aligned layout** - Clean, readable presentation

## 🗄️ Database Support

### Development Mode (SQLite)
- **SQLite** with WAL mode for better concurrency
- Automatic database creation and schema migrations
- File-based storage at `./data/sqlite/dungeongate-dev.db`
- Perfect for development and small single-server deployments

### Production Mode (PostgreSQL/MySQL)
- **PostgreSQL** (recommended for production)
- **MySQL** support (alternative option)
- **Connection pooling** with configurable limits
- **Health monitoring** and automatic failover
- Shared across all services for data consistency

### Cloud Database Examples

**PostgreSQL:**
```yaml
database:
  type: "postgresql"
  connection:
    host: "db.example.com"
    port: 5432
    database: "dungeongate"
    username: "dungeongate_user"
    password: "secure_password"
    ssl_mode: "require"
```

## 📊 Observability

### Metrics Collection
All services implement comprehensive Prometheus metrics:

**Session Service:**
- SSH connections, session duration, terminal operations
- PTY management and spectating metrics

**Auth Service:**
- Authentication attempts, token operations, security events
- Brute force protection and rate limiting metrics

**Game Service:**
- Game instances, resource usage, session duration
- Save file operations and cleanup metrics

### Structured Logging
Standardized logging using `pkg/logging` with:
- Context correlation (session_id, user_id, etc.)
- Structured fields for searchability
- Service-specific log files in `logs/` directory
- Support for file rotation and journald output

### Health Endpoints
Each service provides health check endpoints:
- Session Service: http://localhost:8083/health
- Auth Service: http://localhost:8081/health
- Game Service: http://localhost:8085/health

## 📦 Dependencies

### Core Go Dependencies

- **[golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto)** - SSH server implementation and bcrypt for password hashing
- **[github.com/creack/pty](https://github.com/creack/pty)** - PTY (pseudo-terminal) management for game sessions
- **[google.golang.org/grpc](https://grpc.io/)** - gRPC for microservices communication
- **[github.com/prometheus/client_golang](https://github.com/prometheus/client_golang)** - Prometheus metrics collection
- **[gopkg.in/yaml.v3](https://gopkg.in/yaml.v3)** - YAML configuration parsing
- **[github.com/golang-jwt/jwt/v5](https://github.com/golang-jwt/jwt)** - JWT authentication tokens
- **[github.com/stretchr/testify](https://github.com/stretchr/testify)** - Testing framework

### Database Drivers

- **[github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3)** - SQLite driver for embedded database mode
- **[github.com/lib/pq](https://github.com/lib/pq)** - PostgreSQL driver for production deployments
- **[github.com/go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)** - MySQL driver (alternative to PostgreSQL)

### Kubernetes Support

- **[k8s.io/client-go](https://github.com/kubernetes/client-go)** - Kubernetes API client for game pod management
- **[k8s.io/api](https://github.com/kubernetes/api)** - Kubernetes API types
- **[k8s.io/apimachinery](https://github.com/kubernetes/apimachinery)** - Kubernetes API machinery

### Development Tools

- **Go 1.24.0+** - Required for building (uses modern Go features)
- **Make** - Build automation (optional but recommended)
- **golangci-lint** - Code linting (optional, for `make lint`)
- **govulncheck** - Vulnerability scanning (optional, for `make vuln`)
- **air** - Live reload for development (optional)

### Runtime Dependencies

- **NetHack** - The default configured game
  - macOS: `brew install nethack`
  - Ubuntu/Debian: `sudo apt-get install nethack`
  - Other games can be configured in `configs/session-service.yaml`
- **SQLite3** - Automatically handled by Go driver for development mode
- **PostgreSQL 12+** - Required only for production deployments

### Optional Dependencies

- **Docker/Podman** - For containerized deployments (planned)
- **Kubernetes** - For orchestrated game pod management (in progress)
- **Prometheus** - For metrics collection and monitoring
- **Grafana** - For metrics visualization

### Installing Dependencies

Most Go dependencies are automatically managed via `go.mod`:

```bash
# Install all Go dependencies
make deps
# or
go mod download
```

For development tools:

```bash
# Install all development tools
make deps-tools

# Or install individually:
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
go install github.com/air-verse/air@latest
```

## 🧪 Testing

### Test Suite

Run the comprehensive test suite:
```bash
make test                    # Run all tests
make test-coverage          # Run tests with coverage report
make test-short             # Run short tests only
make test-race              # Run tests with race detection
make benchmark              # Run performance benchmarks
```

### Specialized Testing

```bash
make test-ssh               # SSH server functionality tests
make test-auth              # Authentication system tests
make test-spectating        # Spectating system tests
```

### Interactive Testing

Test the SSH service interactively:
```bash
make run-all                # Start all services
ssh -p 2222 localhost      # Connect and test functionality
```

### Test Coverage

The project maintains comprehensive test coverage including:
- **Unit Tests**: Individual component testing
- **Integration Tests**: Service-to-service communication
- **Performance Tests**: Load testing and benchmarks
- **Security Tests**: Authentication and authorization

## 🔧 Advanced Setup

### Development Workflow

For advanced development and testing:

```bash
# Daily development cycle
make deps                    # Install dependencies
make test                    # Run all tests
make fmt                     # Format code
make lint                    # Run linter
make run-all                 # Start all services

# Manual build and run individual services
make build-session           # Build session service binary
make build-auth             # Build auth service binary
make build-game             # Build game service binary

# Run individual services manually
./build/dungeongate-session-service -config=configs/session-service.yaml
./build/dungeongate-auth-service -config=configs/auth-service.yaml
./build/dungeongate-game-service -config=configs/game-service.yaml
```

### External Database Setup

For production deployments with PostgreSQL:

1. **Setup PostgreSQL database:**
```bash
createdb dungeongate
psql dungeongate < migrations/001_initial_schema.sql
```

2. **Update configuration:**
```yaml
# configs/common.yaml
database:
  type: "postgresql"
  connection:
    host: "localhost"
    port: 5432
    database: "dungeongate"
    username: "your_user"
    password: "your_password"
```

3. **Run with production config:**
```bash
make run-all
```

## 🤝 Contributing

### Development Workflow

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes and test thoroughly
4. Run the linter: `make lint`
5. Run tests: `make test`
6. Commit your changes: `git commit -m 'Add amazing feature'`
7. Push to the branch: `git push origin feature/amazing-feature`
8. Open a Pull Request with feature description and test output

### Code Standards

- All new features MUST include comprehensive tests
- All new features MUST include structured logging using `pkg/logging`
- All new features MUST include Prometheus metrics
- Follow existing code conventions and patterns
- Ensure all services remain stateless

## 📝 License

DungeonGate is released under the [GNU GENERAL PUBLIC LICENSE](LICENSE).

## 🤖 Development with Claude AI

This repository was **coded and maintained with significant assistance from Claude AI**. The majority of the codebase, architecture design, and documentation was collaboratively developed through extensive pair programming sessions with Claude.

### Claude's Contributions Include:
- **Architecture Design**: Microservices design, gRPC communication patterns, and stateless session management
- **Code Implementation**: Core business logic, SSH server implementation, authentication system, and game integration
- **Testing Strategy**: Comprehensive test suites, integration testing, and performance benchmarks
- **Documentation**: README, roadmaps, API documentation, and inline code comments
- **Observability**: Structured logging framework and comprehensive metrics implementation

### Human-AI Collaboration
While Peter Subacz initiated the project and provided domain knowledge, requirements, and direction, Claude AI contributed the detailed implementation, best practices, and extensive documentation that makes this codebase production-ready.

This project demonstrates the potential of human-AI collaboration in software development, combining human creativity and domain expertise with AI's ability to write comprehensive, well-structured, and documented code.

## 🙏 Acknowledgments

- **Original dgamelaunch** by M. Drew Streib and contributors
- **Modern Go ecosystem** for excellent tooling and libraries
- **SSH and terminal gaming community** for inspiration and requirements

## About the Author

DungeonGate is developed and maintained by Peter Subacz in collaboration with Claude AI. Feel free to reach out with any questions or feedback.

This project started as a learning exercise after getting rained out of too many summer days. What began as a simple attempt to recreate dgamelaunch in Go evolved into a production-ready microservices platform through extensive collaboration with Claude AI. The result demonstrates modern Go development practices, microservices architecture, and comprehensive observability - far beyond what was originally envisioned!