# DungeonGate

```bash
 ____                                      ___        _
|  _ \ _   _ _ __   __ _  ___  ___  _ __  / __|  __ _| |_ ___
| | | | | | | '_ \ / _` |/ _ \/ _ \| '_ \| |___ / _` | __/ _ \
| |_| | |_| | | | | (_| |  __/ (_) | | | | |__ | (_| |  ||  _/
|____/ \__,_|_| |_|\__, |\___|\____|_| |_|____/ \__,_|\__\___|
                   |___/
```

**A SSH-based gateway to terminal gaming adventures written in Go**

DungeonGate is a over-engineered microservices-based platform inspired by [dgamelaunch](https://github.com/paxed/dgamelaunch) for hosting terminal games like NetHack.

## 🚀 Quick Start

### 1. Run with Make (Easiest)

- review the config options in `configs/development/local.yaml` and make changes to your liking. You can find more information about the options in `docs/CONFIG.md`. the `local.yaml` defualts to a local sqlite database and requires nethack to be installed locally

```bash
make test-run
```

This will:
- Build the session service
- Copy the development config to `/tmp/dungeongate-session-service.yaml`
- Start the SSH server on port 2222

> Note: If you have ran DungeonGate previous or changed the host key, you will need to remove it from your known hosts file 

### 2. Connect via SSH

```bash
ssh -p 2222 localhost
```

### 3. View Metrics

Open your browser to: http://localhost:8083/metrics

### Manual Run

1. **Build the Service**
```bash
make build
```

2. **Run with Config**
```bash
./build/dungeongate-session-service -config=configs/development/local.yaml
```

### Configuration

The `configs/development/local.yaml` file controls:
- **SSH Port**: 2222 (non-privileged for development)
- **HTTP Port**: 8083 (for metrics)
- **Database**: SQLite at `./data/sqlite/dungeongate-dev.db`
- **SSH Host Key**: `host_key_path` - Path to SSH host key (auto-generated if missing)
- **Games**: NetHack configuration (binary path, environment, etc.)

### First Time Setup

1. **Install NetHack** (if not already installed):
```bash
# macOS
brew install nethack

# Ubuntu/Debian
sudo apt-get install nethack
```

2. **Update Game Path** in `configs/development/local.yaml`:
```yaml
games:
  - id: "nethack"
    binary:
      path: "/usr/games/nethack"  # Update this to your NetHack path
```

3. **Create Required Directories**:
```bash
mkdir -p data/sqlite
mkdir -p /tmp/nethack-saves
```

### What You'll See

1. SSH connection shows a welcome banner
2. Anonymous users see:
   - **[l]** Login
   - **[r]** Register
   - **[w]** Watch games
   - **[g]** List games
   - **[q]** Quit
3. After registering/logging in:
   - **[p]** Play a game
   - **[w]** Watch games
   - **[e]** Edit profile
   - Plus other options

### Troubleshooting

If port 2222 is in use:
```bash
lsof -ti:2222 | xargs kill -9
```

If you get permission errors, make sure the data directories exist and are writable.

## 📦 Dependencies

### Core Go Dependencies

- **[golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto)** - SSH server implementation and bcrypt for password hashing
- **[github.com/creack/pty](https://github.com/creack/pty)** - PTY (pseudo-terminal) management for game sessions
- **[google.golang.org/grpc](https://grpc.io/)** - gRPC for microservices communication
- **[github.com/prometheus/client_golang](https://github.com/prometheus/client_golang)** - Prometheus metrics collection
- **[gopkg.in/yaml.v3](https://gopkg.in/yaml.v3)** - YAML configuration parsing

### Database Drivers

- **[github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3)** - SQLite driver for embedded database mode
- **[github.com/lib/pq](https://github.com/lib/pq)** - PostgreSQL driver for production deployments
- **[github.com/go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)** - MySQL driver (alternative to PostgreSQL)

### Kubernetes Support

- **[k8s.io/client-go](https://github.com/kubernetes/client-go)** - Kubernetes API client for game pod management
- **[k8s.io/api](https://github.com/kubernetes/api)** - Kubernetes API types
- **[k8s.io/apimachinery](https://github.com/kubernetes/apimachinery)** - Kubernetes API machinery

### Development Tools

- **Go 1.24+** - Required for building (uses modern Go features)
- **Make** - Build automation (optional but recommended)
- **golangci-lint** - Code linting (optional, for `make lint`)
- **govulncheck** - Vulnerability scanning (optional, for `make vuln`)
- **air** - Live reload for development (optional, for `make dev`)

### Runtime Dependencies

- **NetHack** - The default configured game
  - macOS: `brew install nethack`
  - Ubuntu/Debian: `sudo apt-get install nethack`
  - Other games can be configured in `configs/development/local.yaml`
- **SQLite3** - Automatically handled by Go driver for development mode
- **PostgreSQL 12+** - Required only for production deployments with external database mode

### Optional Dependencies

- **Docker** - For containerized deployments (coming soon)
- **Kubernetes** - For orchestrated game pod management (planned)
- **Prometheus** - For metrics collection (metrics endpoint at `:8083/metrics`)
- **Grafana** - For metrics visualization (optional)

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
# Install linting tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Install vulnerability scanner
go install golang.org/x/vuln/cmd/govulncheck@latest

# Install live reload tool
go install github.com/air-verse/air@latest
```

## 🏗️ Architecture

DungeonGate uses a microservices architecture with the following services:

- **Session Service** ✅ - Handles SSH connections, PTY management, and user sessions
- **User Service** ✅ - Manages user registration, authentication, and profiles  
- **Auth Service** 🔄 - Authentication, authorization, automated password reset, misc admin functions (planned)
- **Game Service** 📋 - Game management, loading, saving, and configuration (planned)
- **Log Service** 📋 - Genernal Logging for tracing and debugging

## 📁 Project Structure

```
dungeongate/
├── cmd/
│   └── session-service/      # Main application entry point
├── internal/
│   ├── session/             # SSH server, PTY bridging, session management
│   ├── user/               # User registration and management
│   ├── auth/               # Authentication service (planned)
│   └── games/              # Game management (planned)
├── pkg/
│   ├── config/             # Configuration management with dual-mode database
│   ├── database/           # Database abstraction layer (SQLite/PostgreSQL)
│   ├── encryption/         # Encryption utilities
│   └── ttyrec/            # Terminal recording functionality
├── banners/               # Dynamic banner templates
├── configs/               # Environment-specific configurations
├── examples/              # Example configurations and banners
├── migrations/            # Database migration files
└── scripts/              # Build and deployment scripts
```

## ✅ Currently Implemented

### Session Service (SSH Server)
- **Password-free SSH access** - Anonymous users can connect directly without password prompts
- **Dynamic banner system** - Customizable banners with template variables and terminal width adaptation
- **Real-time Spectating** - Watch active game sessions with immutable data streaming architecture
- **PTY Management** - Full pseudo-terminal support with session multiplexing
- **Terminal Recording** - TTY recording functionality for session playback

### Database Layer
- **Dual-mode database support**:
  - **Embedded mode**: SQLite for development and small deployments
  - **External mode**: PostgreSQL for production with read/write separation
- **Flexible connection management** - Same endpoint for both reader/writer or separate endpoints
- **Health monitoring** with automatic failover support
- **Connection pooling** with configurable limits and lifecycle management
- **Database metrics** and query logging for performance monitoring

### Configuration System
- **Environment-specific configs** - Separate development, testing, and production configurations
- **Environment variable support** - Secure handling of secrets and credentials
- **Versioning support** - Configurable application version display
- **Menu and banner configuration** - Customizable banner paths and menu options
- **Validation with defaults** - Robust configuration parsing with sensible fallbacks

## 🔧 Configuration

### Development Configuration

The development setup uses SQLite for simplicity and includes menu banner configuration:

```yaml
# configs/development/local.yaml
version: "0.0.3"

database:
  mode: "embedded"
  embedded:
    type: "sqlite"
    path: "./data/sqlite/dungeongate-dev.db"
    wal_mode: true

ssh:
  enabled: true
  port: 2222  # Non-privileged port for development
  host: "localhost"
  host_key_path: "./test-data/ssh_keys/test_host_key"
  auth:
    allow_anonymous: true

menu:
  banners:
    main_anon: "./banners/main_anon.txt"
    main_user: "./banners/main_user.txt"
```

### Production Configuration

Production configuration supports external databases with full read/write separation:

```yaml
# configs/production/config.yaml
version: "1.0.0"

database:
  mode: "external"
  external:
    type: "postgresql"
    writer_endpoint: "${DB_WRITER_ENDPOINT}"
    reader_use_writer: false
    reader_endpoint: "${DB_READER_ENDPOINT}"
    database: "dungeongate"
    username: "${DB_USER}"
    password: "${DB_PASSWORD}"
    ssl_mode: "require"
    failover:
      enabled: true
      reader_to_writer_fallback: true

ssh:
  enabled: true
  port: 22
  host: "0.0.0.0"
  max_sessions: 200
  auth:
    allow_anonymous: false
```

## 🎨 Banner System

DungeonGate features a dynamic banner system with template variables:

### Banner Templates

**Anonymous User Banner** (`configs/development/banners/main_anon.txt`):
```
Welcome to $SERVERID!

Connected as: Anonymous
Date: $DATE | Time: $TIME

Menu Options:
  [l] Login
  [r] Register
  [w] Watch games
  [g] List games
  [q] Quit
```

**Authenticated User Banner** (`configs/development/banners/main_user.txt`):
```
Welcome back to $SERVERID, $USERNAME!

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
- `$SERVERID` - Server name (default: "DungeonGate")
- `$USERNAME` - Current username or "Anonymous"
- `$DATE` - Current date (YYYY-MM-DD)
- `$TIME` - Current time (HH:MM:SS)

### Features
- **📏 Automatic resizing** - Banners adapt to terminal width
- **🔄 Real-time replacement** - Variables updated on each display
- **⚙️ Configurable version footer** - Shows version from config file
- **🎯 Left-aligned layout** - Clean, readable presentation

## 🗄️ Database Support

### Embedded Mode (Development/Small Deployments)
- **SQLite** with WAL mode for better concurrency
- Automatic database creation and schema migrations
- File-based storage with configurable paths
- Perfect for development and small single-server deployments

### External Mode (Production/Cloud)
- **PostgreSQL** (recommended for production)
- **MySQL** support (alternative option)
- **Read/Write endpoint separation** for cloud databases like AWS Aurora
- **Connection pooling** with separate reader/writer pools
- **Health monitoring** and automatic failover

### Cloud Database Examples

**AWS Aurora PostgreSQL:**
```yaml
external:
  writer_endpoint: "aurora-cluster.cluster-xyz.us-west-2.rds.amazonaws.com:5432"
  reader_endpoint: "aurora-cluster.cluster-ro-xyz.us-west-2.rds.amazonaws.com:5432"
  reader_use_writer: false
```

**Single PostgreSQL Instance:**
```yaml
external:
  writer_endpoint: "db.example.com:5432"
  reader_use_writer: true  # Use same endpoint for reads and writes
```


## 🚀 Getting Started

### Prerequisites
- Go 1.21 or higher
- Git for version control
- SSH client for testing
- Make (optional, but recommended)
- For external databases: PostgreSQL 12+ or MySQL 8.0+

### Development Setup

1. **Clone the repository:**
```bash
git clone https://github.com/psubacz/dungeongate.git
cd dungeongate
```

2. **Setup development environment:**
```bash
make setup                    # Setup directories, keys, and configs
# or: ./scripts/build-and-test.sh setup
```

3. **Build and verify:**
```bash
make verify                   # Build, test, and verify code quality
# or: ./scripts/build-and-test.sh verify
```

4. **Start the server:**
```bash
make test-run                # Start SSH server on port 2222
# or: ./scripts/build-and-test.sh start
```

5. **Test SSH connection (in another terminal):**
```bash
ssh -p 2222 localhost        # Connect to the test server
# Use 'w' to watch games, 'r' to register, etc.
```

### Development Workflow

```bash
# Daily development cycle
make verify                   # Format, lint, and test
make test-run                # Start test server
make ssh-test-connection     # Test SSH functionality

# Specific testing
make test-ssh                # Test SSH functionality
make test-spectating         # Test spectating system
make test-integration        # Run integration tests

# Live development
make dev                     # Start with auto-reload
```

### Build Commands

**Legacy build method:**
```bash
make build
```

4. **Run the SSH service:**
```bash
make test-run
```

5. **Connect via SSH:**
```bash
ssh -p 2222 localhost
```

### Development Commands

Essential commands for development:

```bash
# Install dependencies
make deps

# Build the session service
make build

# Run development server with auto-restart
make dev

# Run tests
make test

# Start SSH service on port 2222
make test-run

# Format Go code
make fmt

# Run linter
make lint

# Check for vulnerabilities
make vuln
```

### Testing the Service

Connect to the development SSH server:
```bash
ssh -p 2222 localhost
```

You'll see the dynamic banner and can:
- Register a new account (option `r`)
- Login with existing credentials (option `l`)
- Browse available games (option `g`)
- Watch active games (option `w`)

## 🔮 Planned Features

### User Service Enhancements
- [ ] **Email verification** workflow
- [ ] **Password reset** functionality via email
- [ ] **User profile management** API endpoints
- [ ] **Role-based access control** (admin, moderator, user)

### Auth Service
- [ ] **Centralized JWT authentication**
- [ ] **OAuth integration** (GitHub, Google, Discord)
- [ ] **Two-factor authentication** support
- [ ] **Session management** across services

### Game Service  
- [ ] **Game configuration management** via API
- [ ] **Game binary management** and updates
- [ ] **Score tracking** and global leaderboards
- [ ] **Game statistics** and player analytics

### Session Service Enhancements
- [x] **Basic spectating** with real-time terminal streaming ✅
- [ ] **TTY recording** with full playback support
- [ ] **Advanced spectating** with chat and multiple viewers
- [ ] **Session persistence** across network disconnections
- [ ] **Load balancing** for distributed session instances

### Infrastructure & Operations
- [ ] **Docker containers** and Kubernetes manifests
- [ ] **Comprehensive monitoring** with Prometheus/Grafana
- [ ] **Centralized logging** with structured logs
- [ ] **Performance profiling** and optimization
- [ ] **Automated health checks** and alerting

## 🧪 Testing

Run the test suite:
```bash
make test
```

Test the SSH service interactively:
```bash
make test-run
ssh -p 2222 localhost
```

Test database configurations:
```bash
./scripts/test-database-configs.sh
```

## 📊 Metrics and Monitoring

The session service provides comprehensive metrics:

### SSH Metrics
- Total and active SSH connections
- Session counts and duration
- Failed connection attempts
- Bytes transferred per session

### Database Metrics  
- Connection pool status (active/idle connections)
- Query performance and execution times
- Database health and failover events
- Connection errors and retry attempts

### System Metrics
- Memory usage and garbage collection
- Goroutine counts and scheduling
- File descriptor usage
- Network I/O statistics

## 🔒 Security Features

Todo

## 🤝 Contributing

This project is actively under development. Current focus areas:

1. **✅ Enhanced user registration system** - Completed
2. **✅ Dynamic banner and menu system** - Completed
3. **✅ Database dual-mode configuration** - Completed
4. **🔄 User authentication completion** - In progress
5. **📋 Game service development** - Planned
6. **📋 Authentication service architecture** - Planned

### Development Workflow

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes and test thoroughly
4. Run the linter: `make lint`
5. Run tests: `make test`
6. Commit your changes: `git commit -m 'Add amazing feature'`
7. Push to the branch: `git push origin feature/amazing-feature`
8. Open a Pull Request

## 📝 License


## 🙏 Acknowledgments

- **Original dgamelaunch** by M. Drew Streib and contributors
- **Modern Go ecosystem** for excellent tooling and libraries
- **Claude AI** for development assistance and architectural guidance
- **SSH and terminal gaming community** for inspiration and requirements

---

**Ready to dive into terminal gaming? Connect via SSH and start your adventure!**

```bash
ssh -p 2222 your-dungeongate-server.com
```