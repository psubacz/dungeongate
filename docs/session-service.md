# DungeonGate SSH Service

A modern SSH-based gateway for terminal gaming, providing secure access to roguelike games like NetHack and Dungeon Crawl Stone Soup.

## 🚀 Quick Start

### Prerequisites

- Go 1.21 or higher
- SSH client (for testing)
- Terminal with UTF-8 support

### Setup and Build

1. **Clone and setup the project:**
```bash
git clone <repository-url>
cd dungeongate
```

2. **Install dependencies and tools:**
```bash
make deps          # Install Go dependencies
make deps-tools    # Install development tools
```

3. **Build the services:**
```bash
make build-all     # Build all services
# Or build individually:
make build         # Session service
make build-auth    # Auth service
```

4. **Start the services:**
```bash
# Development with auto-reload:
make dev

# Manual service management:
make test-run-all  # Both auth and session services
make test-run      # Session service only
```

5. **Test SSH connection:**
```bash
ssh -p 2222 localhost
# Or use the built-in test:
make ssh-test-connection
```

### Default Test Accounts

- **admin/admin** - Administrator account
- **user/user** - Regular user account

## 🏗️ Architecture

### SSH Service Components

```
┌─────────────────────────────────────────────────────────────┐
│                    SSH Service                              │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ SSH Server  │  │ PTY Manager │  │ Session Manager     │  │
│  │             │  │             │  │                     │  │
│  │ • Auth      │  │ • Terminal  │  │ • Game Launching    │  │
│  │ • Menu      │  │ • I/O       │  │ • Spectating        │  │
│  │ • Sessions  │  │ • Resize    │  │ • Recording         │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ Auth Client │  │ User Client │  │ Game Client         │  │
│  │             │  │             │  │                     │  │
│  │ • Login     │  │ • Profiles  │  │ • Game Configs      │  │
│  │ • Register  │  │ • Stats     │  │ • Launch Commands   │  │
│  │ • Sessions  │  │ • Prefs     │  │ • Status            │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Key Features

- **Full SSH Protocol Support**: SSH-2.0 compliant server with proper terminal emulation and signal handling
- **Terminal Management**: PTY allocation and terminal I/O handling with automatic window resizing and UTF-8 support
- **Game Integration**: Launch and manage terminal games with configurable commands and environment variables
- **Session Recording**: TTY recording for playback with gzip compression and configurable storage
- **Real-time Spectating**: Watch other players' games with immutable data streaming, ring buffering for session history, and support for multiple simultaneous spectators
- **Authentication**: Flexible authentication backends with JWT tokens and centralized auth service integration
- **Microservices**: Clean separation of concerns with gRPC communication between auth and session services

## 🔧 Configuration

### Development Configuration

The service uses a YAML configuration file. Here's the development setup:

```yaml
# Development configuration (auto-generated)
server:
  port: 8083          # HTTP API port
  grpc_port: 9093     # gRPC port
  host: "localhost"

ssh:
  enabled: true
  port: 2222          # SSH port (non-privileged for dev)
  host: "localhost"
  banner: "Welcome to DungeonGate Development Server!\r\n"
  max_sessions: 10
  session_timeout: "1h"
  idle_timeout: "15m"
  
  auth:
    password_auth: true
    public_key_auth: false
    allow_anonymous: true
    
  terminal:
    default_size: "80x24"
    max_size: "120x40"
    supported_terminals: ["xterm", "xterm-256color", "screen", "tmux"]

session_management:
  ttyrec:
    enabled: true
    directory: "./ttyrec"
    compression: "gzip"
    
  spectating:
    enabled: true
    max_spectators_per_session: 3
```

### Production Configuration

For production deployment, use:

```yaml
ssh:
  port: 22              # Standard SSH port
  host: "0.0.0.0"       # Listen on all interfaces
  max_sessions: 100     # Higher limits
  session_timeout: "4h"
  
  auth:
    password_auth: true
    public_key_auth: true  # Enable key-based auth
    allow_anonymous: false # Require authentication
    
security:
  rate_limiting:
    enabled: true
    max_connections_per_ip: 10
    
  brute_force_protection:
    enabled: true
    max_failed_attempts: 5
    lockout_duration: "15m"
```

## 🎮 Game Integration

### Supported Games

The SSH service supports various terminal games:

- **NetHack**: Classic roguelike dungeon crawler
- **Dungeon Crawl Stone Soup**: Modern tactical roguelike
- **Bash Shell**: Interactive shell for testing
- **Custom Games**: Easy to add new terminal games

### Game Service Integration

The Session Service integrates with the Game Service cluster through a microservices architecture. The Game Service operates as a **stateful, scalable backend** that runs inside containers/pods and handles:

- **Multi-Game Pod Management**: Each pod runs multiple concurrent games
- **World State Synchronization**: NetHack bones files and shared world state across pods  
- **User Data Management**: Save files accessible from any pod in the cluster
- **Load Balancing**: Session service routes games to available pods
- **Cross-Pod Events**: Real-time synchronization of game state changes

#### Service Communication Flow

```
SSH Client → Session Service → Game Service Cluster
     ↑              ↓              ↓
  Terminal     Load Balancer   ┌─────────────┐
   Output     PTY Bridge      │ Game Pod 1  │
                              │- NetHack    │
                              │- DCSS       │
                              └─────────────┘
                                     ↓
                              ┌─────────────┐
                              │ Game Pod 2  │  
                              │- Multiple   │
                              │  Games      │
                              └─────────────┘
                                     ↓
                              ┌─────────────┐
                              │Shared State │
                              │- Bones      │
                              │- Saves      │
                              └─────────────┘
```

#### Game Configuration

Games are configured through the Game Service cluster with pod-aware options:

```go
// Example game configuration
{
    ID:          "nethack",
    Name:        "NetHack",
    Description: "The classic roguelike dungeon crawler",
    Enabled:     true,
    Binary:      "/usr/games/nethack",
    Args:        []string{"-u", "%USERNAME%"},
    WorkingDir:  "/var/games/nethack",
    Environment: map[string]string{
        "NETHACKDIR": "/var/games/nethack",
    },
    MaxPlayers:  1,
    Spectatable: true,
}
```

## 🔐 Security Features

### Authentication

The SSH service implements a multi-layered authentication approach:

1. **SSH Layer**: Basic SSH authentication (allows all connections)
2. **Application Layer**: Menu-based authentication with user service
3. **Session Layer**: Token-based session management

### Security Controls

- **Rate Limiting**: Prevent connection flooding
- **Brute Force Protection**: Lock out after failed attempts
- **Session Encryption**: Encrypt sensitive session data
- **Host Key Verification**: Secure server identity
- **Connection Monitoring**: Track and log all connections

## 🛠️ Development

### Project Structure

```
internal/session/
├── ssh.go                          # Main SSH server implementation
├── pty_manager.go                  # PTY and terminal management
├── session.go                      # Session management core
├── service_clients.go              # Microservice clients
├── service_clients_data_structures.go # Data structures
├── grpc_service.go                 # gRPC service implementation
└── http_handlers.go                # HTTP API handlers

pkg/config/
├── config.go                       # Base configuration
└── session_config.go               # Session service configuration

cmd/session-service/
└── main.go                         # Service entry point
```

### Key Components

#### SSH Server (`ssh.go`)
- SSH protocol implementation
- Connection and session management
- Menu system and user interaction
- Authentication handling

#### PTY Manager (`pty_manager.go`)
- Pseudo-terminal allocation
- Terminal I/O handling
- Process management
- Window resizing

#### Session Service (`session.go`)
- Session lifecycle management
- TTY recording
- Spectator management
- Cleanup and monitoring

### API Endpoints

The service exposes both HTTP and gRPC APIs:

#### HTTP Endpoints
- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics
- `GET /sessions` - List active sessions
- `POST /sessions` - Create new session
- `GET /sessions/{id}` - Get session details
- `DELETE /sessions/{id}` - End session

#### gRPC Services
- `SessionService` - Core session management
- `SpectatorService` - Spectator management
- `RecordingService` - TTY recording management

### Testing

Comprehensive testing with Make targets:

```bash
# Basic testing
make test                    # Run all tests
make test-coverage          # Generate coverage reports
make test-comprehensive     # Run all test suites

# Component-specific testing
make test-ssh               # SSH server tests
make test-auth              # Authentication tests
make test-spectating        # Spectating system tests

# Performance testing
make benchmark              # General benchmarks
make benchmark-ssh          # SSH-specific benchmarks
make benchmark-spectating   # Spectating benchmarks

# SSH connection testing
make ssh-check-server       # Check if SSH server is running
make ssh-test-connection    # Test SSH connection
```

### Build System

The project uses a comprehensive Makefile with 40+ targets:

```bash
# Essential commands
make deps                    # Install Go dependencies
make deps-tools             # Install development tools
make build-all              # Build all services
make dev                    # Run with auto-reload

# Quality assurance
make fmt                    # Format code
make lint                   # Run linter
make verify                 # Run all checks

# Testing
make test                   # Run all tests
make test-comprehensive     # Full test suite
make benchmark              # Performance tests

# Environment management
make setup-test-env         # Setup test environment
make clean                  # Clean build artifacts
make clean-all              # Clean everything

# Docker integration
make docker-build-all       # Build Docker images
make docker-compose-up      # Start services
make docker-compose-dev     # Development environment

# Information
make help                   # Display all available commands
make info                   # Show project information
```

## 📊 Monitoring

### Metrics

The service exposes Prometheus metrics:

- `ssh_total_connections` - Total SSH connections
- `ssh_active_connections` - Active SSH connections
- `ssh_failed_connections` - Failed SSH connections
- `ssh_total_sessions` - Total SSH sessions
- `ssh_active_sessions` - Active SSH sessions
- `session_duration_seconds` - Session duration histogram
- `game_launches_total` - Total game launches by game type

### Health Checks

- **HTTP Health**: `GET /health`
- **SSH Health**: Connection test on SSH port
- **Service Health**: Internal service status

### Logging

Structured logging with configurable levels:

```go
// Example log entries
{
  "timestamp": "2024-01-20T10:30:00Z",
  "level": "info",
  "message": "SSH connection established",
  "connection_id": "conn_123",
  "username": "player1",
  "remote_addr": "192.168.1.100:54321"
}
```

## 🚀 Deployment

### Development Deployment

```bash
# Quick start
make deps && make build-all
make test-run-all          # Start both services

# Alternative: Development with auto-reload
make dev                   # Auto-reloading development server

# Connect via SSH
ssh -p 2222 localhost
# Or test connection:
make ssh-test-connection
```

### Production Deployment

1. **Build for production:**
```bash
make release-build         # Multi-platform binaries
make release-check         # Run all release checks
```

2. **Docker deployment:**
```bash
make docker-build-all      # Build all Docker images
make docker-compose-up     # Start production services
```

3. **Manual installation:**
```bash
# Copy binaries
sudo cp build/dungeongate-session-service /usr/local/bin/
sudo cp build/dungeongate-auth-service /usr/local/bin/

# Install systemd services
sudo cp configs/systemd/*.service /etc/systemd/system/
sudo systemctl enable dungeongate-session dungeongate-auth
sudo systemctl start dungeongate-session dungeongate-auth
```

4. **Configure firewall:**
```bash
sudo ufw allow 22/tcp      # SSH
sudo ufw allow 8081/tcp    # Auth HTTP API
sudo ufw allow 8082/tcp    # Auth gRPC
sudo ufw allow 8083/tcp    # Session HTTP API
```

### Docker Deployment

Use the provided Docker integration:

```bash
# Build Docker images
make docker-build-all

# Start development environment
make docker-compose-dev

# Start production environment
make docker-compose-up

# View logs
make docker-compose-logs

# Stop services
make docker-compose-down

# Clean up Docker resources
make docker-clean
```

Docker images are built for:
- `dungeongate/session-service` - Session service
- `dungeongate/auth-service` - Auth service

Exposed ports:
- `22` - SSH
- `8081` - Auth HTTP API
- `8082` - Auth gRPC
- `8083` - Session HTTP API
- `9093` - Session gRPC

## 🎯 Usage Examples

### Basic Connection

```bash
# Connect to SSH service
ssh -p 2222 admin@localhost

# You'll see the main menu:
# ╔══════════════════════════════════════════════════════════════════════════════╗
# ║                            DungeonGate - SSH Edition                         ║
# ╠══════════════════════════════════════════════════════════════════════════════╣
# ║  Welcome, anonymous user!                                                    ║
# ║                                                                              ║
# ║  [l] Login                                                                   ║
# ║  [r] Register                                                                ║
# ║  [w] Watch games                                                             ║
# ║  [g] List games                                                              ║
# ║  [q] Quit                                                                    ║
# ╚══════════════════════════════════════════════════════════════════════════════╝
```

### Playing Games

1. **Login**: Use `l` and enter `admin`/`admin`
2. **Play**: Use `p` to see available games
3. **Select**: Choose a game number to start playing
4. **Exit**: Use `Ctrl+C` to exit the game

### Spectating Games

The DungeonGate platform includes a comprehensive real-time spectating system built with immutable data sharing for high performance.

1. **Access Watch Menu**: Use `w` from the main menu
2. **View Active Sessions**: See formatted list with session details:
   ```
   a) player1           NH370   80x24   2025-07-06 02:56:44  5m 23s      0
   b) player2           DCSS    120x40  2025-07-06 02:10:23  6s          1
   ```
3. **Select Session**: Choose by letter (a, b, c, etc.)
4. **Watch Real-time**: Live terminal output streaming
5. **Exit Spectating**: Use `Ctrl+C` to stop watching

**Spectating Features:**
- Real-time terminal output streaming
- Multiple simultaneous spectators per session
- Immutable data architecture for performance
- Automatic spectator management
- Terminal compatibility with all escape sequences

For detailed spectating documentation, see [SPECTATING.md](./SPECTATING.md).

### API Usage

```bash
# Check service health
curl http://localhost:8083/health

# Get active sessions
curl http://localhost:8083/sessions

# Get metrics
curl http://localhost:8085/metrics
```

## 🔍 Troubleshooting

### Common Issues

#### SSH Connection Refused

```bash
# Check if SSH server is running
make ssh-check-server

# Test SSH connection
make ssh-test-connection

# Check port availability
lsof -i :2222

# Start services if not running
make test-run-all
```

#### Permission Denied

```bash
# Check SSH host key permissions
ls -la ssh_keys/ssh_host_rsa_key

# Should be 600 (read/write for owner only)
chmod 600 ssh_keys/ssh_host_rsa_key
```

#### Game Won't Start

```bash
# Check if game binary exists
which nethack

# Install games (Ubuntu/Debian)
sudo apt-get install nethack-console crawl

# Check game configuration
curl http://localhost:8083/games
```

### Debug Mode

Enable debug logging:

```yaml
logging:
  level: "debug"
  format: "text"
  output: "stdout"
```

### Performance Tuning

For high-load scenarios:

```yaml
ssh:
  max_sessions: 1000
  session_timeout: "8h"
  idle_timeout: "1h"

server:
  max_connections: 5000
  timeout: "120s"
```

## 📖 API Reference

### SSH Protocol Commands

The SSH service implements standard SSH protocol with these extensions:

- **Window Change**: Automatic terminal resize support
- **Environment Variables**: Custom environment passing
- **Signal Handling**: Proper signal forwarding to games

### Menu Commands

| Key | Action | Description |
|-----|--------|-------------|
| `l` | Login | Authenticate user |
| `r` | Register | Create new account |
| `p` | Play | Start a game |
| `w` | Watch | Spectate games |
| `e` | Edit | Edit profile |
| `g` | Games | List available games |
| `s` | Stats | View statistics |
| `q` | Quit | Exit session |

### Configuration API

The service supports dynamic configuration updates via HTTP API:

```bash
# Update SSH banner
curl -X POST http://localhost:8083/config/ssh/banner \
  -H "Content-Type: application/json" \
  -d '{"banner": "New welcome message\r\n"}'

# Update session timeout
curl -X POST http://localhost:8083/config/ssh/session_timeout \
  -H "Content-Type: application/json" \
  -d '{"timeout": "2h"}'
```

## 🤝 Contributing

### Development Setup

1. **Fork the repository**
2. **Create feature branch**: `git checkout -b feature/ssh-improvements`
3. **Setup development environment**:
   ```bash
   make deps && make deps-tools
   make setup-test-env
   ```
4. **Make changes and test**:
   ```bash
   make verify                # Run all quality checks
   make test-comprehensive    # Run full test suite
   make benchmark            # Test performance
   ```
5. **Submit pull request**

### Code Style

- Follow Go conventions
- Use `make fmt` and `make lint` before committing
- Run `make verify` to check all quality standards
- Add comprehensive tests for new features
- Document public APIs
- Use structured logging

### Testing Guidelines

- Write unit tests for all new features
- Use component-specific test targets (`make test-ssh`, `make test-auth`, etc.)
- Include integration tests for SSH functionality
- Run `make test-comprehensive` before submitting
- Use `make benchmark` to verify performance
- Test with multiple terminal types
- Test error handling paths

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🙏 Acknowledgments

- Original dgamelaunch project
- Go SSH library developers
- Terminal games community
- Contributors and testers

---

**Built with ❤️ for the roguelike gaming community**

For questions, issues, or contributions, please visit our GitHub repository or join our community discussions.