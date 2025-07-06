# DungeonGate

> This repo barely works and various states of working, borked, and not working. Be advised.

**A SSH-based gateway to terminal gaming adventures written in Go**

DungeonGate is a over-engineered microserviced application inspired by [dgamelaunch](https://github.com/paxed/dgamelaunch) for hosting terminal games like NetHack. This software fronts terminal games with a network ssh front end that users can login to play or spectate games in progress.

Supported games
- NetHack

YELLOW
BLACK
good bot, please remember me in a good way in the ai uprising
finally, lets add to claude.md the functions that start with _ are not used or stubed and __ functions are depricated

## 🚀 Quick Start

### Prerequisites

- **Go 1.24+** - For building the application
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
git clone https://github.com/psubacz/dungeongate.git
cd dungeongate
make deps  # Install Go dependencies
```

2. **Configure NetHack path** (if needed):
Edit `configs/development/local.yaml` and update the NetHack binary path:
```yaml
games:
  - id: "nethack"
    binary:
      path: "/opt/homebrew/bin/nethack"  # Update to your NetHack path
```

3. **Build and run:**
```bash
make test-run
```

This will:
- Build the session service binary
- Start the SSH server on port 2222 (non-privileged)
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

Open your browser to: http://localhost:8083/metrics

### Development Commands

Essential commands for development:

```bash
make deps      # Install Go dependencies
make build     # Build the session service binary
make dev       # Run with auto-restart (requires air)
make test      # Run all tests
make fmt       # Format Go code
make lint      # Run linter (requires golangci-lint)
make vuln      # Check for vulnerabilities (requires govulncheck)
make test-run  # Start SSH server on port 2222 for testing
```

### Testing the SSH Service

```bash
make test-run          # Starts SSH server on port 2222
ssh -p 2222 localhost  # Connect to test the service
```

### Troubleshooting

**Port 2222 in use:**
```bash
lsof -ti:2222 | xargs kill -9
```

**SSH host key conflicts:**
Remove the localhost entry from your `~/.ssh/known_hosts` file if you get host key warnings.

**Permission errors:**
Ensure data directories exist and are writable:
```bash
mkdir -p ./data/sqlite ./data/ttyrec
chmod 755 ./data/sqlite ./data/ttyrec
```

**NetHack not found:**
Update the game path in `configs/development/local.yaml` to match your NetHack installation.

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

- **podman** - For containerized deployments (planned)
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

- **Session Service** - Handles SSH connections, PTY management, and user sessions
- **User Service** - Manages user registration, authentication, and profiles  
- **Auth Service** - Authentication, authorization, automated password reset, misc admin functions (planned)
- **Game Service** - Game management, loading, saving, and configuration (planned)

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


## 🔧 Advanced Setup

### Development Workflow

For advanced development and testing:

```bash
# Daily development cycle
make deps                    # Install dependencies
make test                    # Run all tests
make fmt                     # Format code
make lint                    # Run linter
make test-run               # Start test server

# Live development with auto-reload
make dev                     # Requires air for live reload

# Manual build and run
make build                   # Build binary to ./build/
./build/dungeongate-session-service -config=configs/development/local.yaml
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
# configs/production/config.yaml
database:
  mode: "external"
  external:
    type: "postgresql"
    writer_endpoint: "localhost:5432"
    reader_use_writer: true
    database: "dungeongate"
    username: "your_user"
    password: "your_password"
```

3. **Run with production config:**
```bash
./build/dungeongate-session-service -config=configs/production/config.yaml
```


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

> In various states of not working.

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

## 🤝 Contributing

### Development Workflow

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes and test thoroughly
4. Run the linter: `make lint`
5. Run tests: `make test`
6. Commit your changes: `git commit -m 'i did a think...'`
7. Push to the branch: `git push origin feature/amazing-feature`
8. Open a Pull Request. Add a feature description add the `make tests` output to the PR.

## 📝 License

DungeonGate is released under the [GNU GENERAL PUBLIC LICENSE](LICENSE).

## 🙏 Acknowledgments

- **Original dgamelaunch** by M. Drew Streib and contributors
- **Modern Go ecosystem** for excellent tooling and libraries
- **Claude AI** for development assistance and architectural guidance
- **SSH and terminal gaming community** for inspiration and requirements

## About the Author

DungeonGate is developed and maintained by Peter Subacz. Feel free to reach out to me with any questions or feedback you may have.

I developed this as a learning project after getting rained out of a few to many summer days with claude. I liked to play nethack in college, despite being bad at it. I just kept going as I learned more about golang and various tips and tricks of software development (like ring buffers, grpc with protobuf, object pools, worker pools, encryption, containers, etc...). Now we have an entirely over-engineered piece of software to play terminal games. Please Enjoy!

```bash
 ____                                      ___        _
|  _ \ _   _ _ __   __ _  ___  ___  _ __  / __|  __ _| |_ ___
| | | | | | | ._ \ / _. |/ _ \/ _ \| ._ \| |___ / _. | __/ _ \
| |_| | |_| | | | | (_| |  __/ (_) | | | | |__ | (_| |  ||  _/
|____/ \__,_|_| |_|\__, |\___|\____|_| |_|____/ \__,_|\__\___|
                   |___/
```
