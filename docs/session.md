# DungeonGate Session Service

A modern, stateless SSH-based gateway for terminal gaming, providing secure access to roguelike games like NetHack and Dungeon Crawl Stone Soup with session persistence and reconnection capabilities.

## 🚀 Quick Start

### Prerequisites

- Go 1.24 or higher
- SSH client (for testing)
- Terminal with UTF-8 support
- Auth Service and Game Service running

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
make build-session # Session service
make build-auth    # Auth service
make build-game    # Game service
```

4. **Start all services:**
```bash
# Start all services with proper sequencing:
make run-all

# Or start individually:
make run-session   # Session service only (limited functionality)
make run-auth      # Auth service only
make run-game      # Game service only
```

5. **Test SSH connection:**
```bash
ssh -p 2222 localhost
# Or use the built-in test:
make ssh-test-connection
```

### Default Test Accounts

- **yellow/password** - Test user account (see database migrations)
- Anonymous access available in development mode

## 🏗️ Architecture

### Session Service Architecture (Stateless Design)

```
┌─────────────────────────────────────────────────────────────┐
│                 Session Service (Stateless)                │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ SSH Server  │  │ Connection  │  │ Menu System         │  │
│  │             │  │ Manager     │  │                     │  │
│  │ • Auth      │  │ • Stateless │  │ • User Interface    │  │
│  │ • Channels  │  │ • Tracking  │  │ • Game Selection    │  │
│  │ • Terminal  │  │ • Cleanup   │  │ • Authentication    │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ Auth Client │  │ Game Client │  │ Streaming Manager   │  │
│  │             │  │             │  │                     │  │
│  │ • JWT Auth  │  │ • Game Mgmt │  │ • I/O Bridging      │  │
│  │ • User Mgmt │  │ • PTY Proxy │  │ • Session Handling  │  │
│  │ • Sessions  │  │ • Lifecycle │  │ • Reconnection      │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Microservices Integration

```
SSH Client → Session Service → Game Service → NetHack Process
     ↑              ↓              ↓              ↓
  Terminal     Auth Service    PTY Manager    Game Output
   I/O         JWT Tokens      Process        Terminal
                              Management      Streams
```

### Key Features

- **Stateless Design**: Horizontal scaling support with no session state in memory
- **Session Persistence**: Games survive stream disconnections and reconnections
- **Full SSH Protocol Support**: SSH-2.0 compliant server with proper terminal emulation
- **Terminal Management**: PTY proxy to Game Service with automatic window resizing
- **Session Recording**: TTY recording for playback (via Game Service)
- **Real-time Spectating**: Watch other players' games with live streaming
- **Microservices Authentication**: JWT-based authentication with Auth Service
- **Connection Resilience**: Heartbeat mechanisms and reconnection support

## 🔧 Configuration

### Session Service Configuration

The service uses a comprehensive YAML configuration file located at `configs/development/session-service.yaml`:

```yaml
# HTTP/gRPC Server Configuration
server:
  port: 8083          # HTTP API port
  grpc_port: 9093     # gRPC port for service communication
  host: "localhost"
  timeout: "30s"
  max_connections: 1000

# SSH Server Configuration
ssh:
  enabled: true
  port: 2222          # SSH port (2222 for development, 22 for production)
  host: "localhost"
  banner: "Welcome to DungeonGate Development Server!\r\n"
  max_sessions: 10
  session_timeout: "1h"
  idle_timeout: "15m"
  
  # SSH Authentication
  auth:
    password_auth: false      # Disabled - using centralized auth
    public_key_auth: false    # Public keys not implemented yet
    allow_anonymous: true     # Allow anonymous connections in dev
    
  # SSH Keepalive Configuration (NEW)
  keepalive:
    enabled: true
    interval: "30s"           # Prevent timeout during NetHack idle periods
    count_max: 3
    
  # Terminal Configuration
  terminal:
    default_size: "80x24"
    max_size: "120x40"
    supported_terminals: ["xterm", "xterm-256color", "screen", "tmux"]

# Session Management Configuration
session_management:
  terminal:
    default_size: "80x24"
    max_size: "120x40"
    encoding: "utf-8"
    
  # Session Timeout Configuration
  timeouts:
    idle_timeout: "15m"
    max_session_duration: "1h"
    cleanup_interval: "1m"
    
  # Heartbeat Configuration (NEW)
  heartbeat:
    enabled: true
    interval: "60s"                    # General heartbeat interval
    idle_detection_threshold: "2m"     # Detect idle state after 2 minutes
    
    # gRPC Stream Heartbeat
    grpc_stream:
      enabled: true
      ping_interval: "45s"             # gRPC stream ping
      pong_timeout: "10s"              # Timeout for pong response
    
  # TTY Recording
  ttyrec:
    enabled: true
    compression: "gzip"
    directory: "/Users/caboose/Desktop/dungeongate/ttyrec"
    max_file_size: "10MB"
    retention_days: 7
    
  # Spectating System
  spectating:
    enabled: true
    max_spectators_per_session: 3
    spectator_timeout: "30m"

# Service Integration
services:
  user_service: "localhost:8084"      # Future user service
  game_service: "localhost:50051"     # Game Service gRPC
  auth_service: "localhost:8082"      # Auth Service gRPC

# Authentication Service Integration
auth:
  enabled: true
  service_address: "localhost:8081"   # Auth Service HTTP
  grpc_address: "localhost:8082"      # Auth Service gRPC
  jwt_secret: "dev-secret-please-change-in-production"
  jwt_issuer: "dungeongate-dev"
  access_token_expiration: "15m"
  refresh_token_expiration: "168h"    # 7 days
  max_login_attempts: 3
  lockout_duration: "15m"
```

### Production Configuration Differences

For production deployment:

```yaml
ssh:
  port: 22              # Standard SSH port
  host: "0.0.0.0"       # Listen on all interfaces
  max_sessions: 1000    # Higher limits
  session_timeout: "4h"
  
  auth:
    allow_anonymous: false # Require authentication
    
security:
  rate_limiting:
    enabled: true
    max_connections_per_ip: 100
    
  brute_force_protection:
    enabled: true
    max_failed_attempts: 10
    lockout_duration: "1m"
```

## 🎮 Game Integration

### Stateless Game Session Management

The Session Service now operates in a **stateless mode** that enables:

- **Session Persistence**: NetHack games survive SSH disconnections
- **Reconnection Support**: Users can reconnect to ongoing games
- **Horizontal Scaling**: Multiple Session Service instances can handle the same user
- **Process Independence**: Games run in Game Service, not Session Service

### Game Service Integration

```
┌─────────────┐    gRPC     ┌─────────────┐    PTY      ┌─────────────┐
│   Session   │ ◄─────────► │    Game     │ ◄─────────► │   NetHack   │
│   Service   │             │   Service   │             │   Process   │
│             │             │             │             │             │
│ • SSH Menu  │             │ • PTY Mgmt  │             │ • Game Loop │
│ • Auth      │             │ • Process   │             │ • Save/Load │
│ • Streaming │             │ • Adapters  │             │ • Terminal  │
└─────────────┘             └─────────────┘             └─────────────┘
```

### Supported Games

- **NetHack 3.6.7**: Classic roguelike with full feature support
- **Dungeon Crawl Stone Soup**: Modern tactical roguelike (planned)
- **Custom Games**: Easy to add through Game Service adapters

### Game Session Flow

1. **User Connection**: SSH connection to Session Service
2. **Authentication**: JWT-based auth through Auth Service
3. **Game Selection**: Menu-driven game selection
4. **Session Creation**: Game Service creates PTY and starts process
5. **Stream Bridging**: Session Service bridges SSH ↔ Game Service I/O
6. **Session Persistence**: Games survive disconnections
7. **Reconnection**: Users can reconnect to ongoing sessions

## 🔐 Security Features

### Multi-Layer Authentication

1. **SSH Layer**: SSH protocol authentication (currently allows all)
2. **Application Layer**: JWT-based authentication with Auth Service
3. **Service Layer**: gRPC service-to-service authentication

### Security Controls

- **Rate Limiting**: Configurable connection limits per IP
- **Brute Force Protection**: Account lockout after failed attempts
- **Session Security**: JWT tokens with configurable expiration
- **Host Key Verification**: SSH server identity verification
- **Connection Monitoring**: Comprehensive logging and tracking

### Configuration Security

```yaml
security:
  rate_limiting:
    enabled: true
    max_connections_per_ip: 100
    connection_window: "1m"
    
  brute_force_protection:
    enabled: true
    max_failed_attempts: 10
    lockout_duration: "1m"
    
  session_security:
    require_encryption: false     # Development setting
    session_token_length: 32
    secure_random: true
```

## 🛠️ Development

### Project Structure

```
internal/session/
├── connection/
│   └── handler.go                  # SSH connection and channel handling
├── menu/
│   └── menu.go                     # User interface and menu system
├── terminal/
│   └── input.go                    # Terminal input processing
├── streaming/
│   └── manager.go                  # I/O streaming and session management
├── auth/
│   └── service.go                  # Authentication service integration
└── config.go                       # Session service configuration

cmd/session-service/
└── main.go                         # Service entry point

pkg/config/
├── config.go                       # Base configuration
└── session_config.go               # Session service configuration
```

### Key Components

#### Connection Handler (`connection/handler.go`)
- SSH protocol implementation with stateless session management
- Connection and channel lifecycle management
- **Fixed**: No longer auto-terminates games on disconnection
- Menu system integration and user interaction

#### Streaming Manager (`streaming/manager.go`)
- I/O bridging between SSH and Game Service
- Session state tracking (stateless mode)
- Connection cleanup without game termination
- **New**: Heartbeat support for connection resilience

#### Menu System (`menu/menu.go`)
- Text-based user interface
- Authentication flow integration
- Game selection and management
- Real-time status display

#### Authentication Service (`auth/service.go`)
- JWT token management
- User authentication and authorization
- Integration with centralized Auth Service
- Session state management

### API Endpoints

#### HTTP Endpoints
- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics (future)
- `GET /connections` - List active connections (development)

#### gRPC Services
- Internal gRPC server on port 9093 for service communication

### Recent Fixes and Improvements

#### NetHack Session Persistence (FIXED)
- **Issue**: NetHack processes were terminated immediately after startup
- **Root Cause**: gRPC context binding and aggressive PTY cleanup
- **Solution**: 
  - Context-independent process creation (`exec.Command` vs `exec.CommandContext`)
  - Session cleanup modifications to preserve running games
  - Process lifecycle improvements for interactive games

#### Heartbeat and Reconnection Support (NEW)
- Configurable SSH keepalive to prevent timeouts
- gRPC stream heartbeat for connection health monitoring
- Idle detection and graceful handling
- Support for NetHack session reconnection

#### Stateless Architecture (IMPROVED)
- Connection management without persistent state
- Horizontal scaling support
- Improved cleanup and resource management

## 📊 Monitoring

### Metrics (Future Implementation)

The service will expose Prometheus metrics:

- `ssh_total_connections` - Total SSH connections
- `ssh_active_connections` - Active SSH connections
- `ssh_failed_connections` - Failed SSH connections
- `game_sessions_total` - Total game sessions started
- `game_sessions_active` - Active game sessions
- `session_duration_seconds` - Session duration histogram

### Health Checks

- **HTTP Health**: `GET /health`
- **SSH Health**: Connection test on SSH port
- **Service Dependencies**: Auth Service and Game Service health

### Logging

Structured logging with configurable levels:

```json
{
  "timestamp": "2025-07-13T18:45:00Z",
  "level": "info",
  "message": "SSH connection established",
  "connection_id": "conn_123",
  "username": "yellow",
  "remote_addr": "127.0.0.1:54321"
}
```

## 🚀 Deployment

### Development Deployment

```bash
# Quick start - all services
make run-all

# Individual services (for debugging)
make run-session      # Session service only
make run-auth        # Auth service only  
make run-game        # Game service only

# Connect via SSH
ssh -p 2222 localhost
```

### Production Deployment

1. **Build for production:**
```bash
make build-all
```

2. **Configure for production:**
```yaml
# Update configs/production/session-service.yaml
ssh:
  port: 22
  host: "0.0.0.0"
  allow_anonymous: false
  
security:
  rate_limiting:
    enabled: true
  brute_force_protection:
    enabled: true
```

3. **Deploy services:**
```bash
# Copy binaries
sudo cp build/dungeongate-session-service /usr/local/bin/
sudo cp build/dungeongate-auth-service /usr/local/bin/
sudo cp build/dungeongate-game-service /usr/local/bin/

# Install systemd services (if available)
sudo systemctl enable dungeongate-session
sudo systemctl start dungeongate-session
```

### Docker Deployment

```bash
# Build Docker images
make docker-build-all

# Start development environment
make docker-compose-dev

# Start production environment
make docker-compose-up
```

## 🎯 Usage Examples

### Basic Connection and Game Play

```bash
# Connect to SSH service
ssh -p 2222 localhost

# Main menu appears:
# Welcome to DungeonGate Development Server!
# 
# Menu Options:
#   [l] Login
#   [r] Register  
#   [w] Watch games
#   [g] List games
#   [q] Quit

# Login with test account
# Enter: l
# Username: yellow
# Password: password

# After authentication:
# Welcome back to DungeonGate, yellow!
# 
# Menu Options:
#   [p] Play a game
#   [w] Watch games
#   [e] Edit profile
#   [l] List games
#   [s] Game Statistics
#   [q] Quit

# Start NetHack
# Enter: p
# Choose game: 1 (NetHack)
# NetHack starts and you can play normally!
```

### Session Persistence

One of the key features is that NetHack sessions now persist:

1. **Start NetHack**: Connect and start a game
2. **Network Issue**: If connection drops, NetHack keeps running
3. **Reconnect**: SSH back in and the game session continues
4. **Resume**: Pick up exactly where you left off

### Spectating Games

```bash
# From main menu, select Watch
# Enter: w

# View active sessions:
# Active Game Sessions:
# [1] yellow - NetHack - Started: 14:45:36
# [q] Return to main menu

# Select session to watch
# Enter: 1
# (Live NetHack gameplay streams to your terminal)
```

## 🔍 Troubleshooting

### Common Issues

#### SSH Connection Refused
```bash
# Check if Session Service is running
make ssh-check-server

# Check port availability
lsof -i :2222

# Start services if not running
make run-all
```

#### Authentication Issues
```bash
# Check Auth Service status
curl http://localhost:8081/health

# Verify Auth Service is running
make run-auth

# Check JWT configuration matches between services
```

#### Game Won't Start
```bash
# Check Game Service status
curl http://localhost:8085/health

# Verify Game Service is running
make run-game

# Check NetHack installation
which nethack
```

#### NetHack Process Termination (FIXED)
- **Previous Issue**: NetHack was killed immediately after startup
- **Status**: RESOLVED - NetHack now runs continuously and survives disconnections
- **Verification**: Game logs should show "Process exited successfully" only when user quits

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
server:
  max_connections: 5000
  timeout: "120s"

ssh:
  max_sessions: 1000
  session_timeout: "8h"
  idle_timeout: "2h"
```

## 📖 API Reference

### Menu Commands

| Key | Action | Description |
|-----|--------|-------------|
| `l` | Login | Authenticate with Auth Service |
| `r` | Register | Create new account |
| `p` | Play | Start a game session |
| `w` | Watch | Spectate active games |
| `e` | Edit | Edit user profile |
| `g` | Games | List available games |
| `s` | Stats | View game statistics |
| `q` | Quit | Exit session |

### Configuration Options

Key configuration sections:

- `server` - HTTP/gRPC server settings
- `ssh` - SSH server and authentication
- `ssh.keepalive` - SSH connection keepalive (NEW)
- `session_management.heartbeat` - Heartbeat configuration (NEW)
- `session_management.timeouts` - Session timeout settings
- `services` - Microservice endpoints
- `auth` - Authentication service integration
- `security` - Security policies and limits

## 🤝 Contributing

### Development Setup

1. **Fork and clone the repository**
2. **Setup development environment**:
   ```bash
   make deps && make deps-tools
   ```
3. **Run all services**:
   ```bash
   make run-all
   ```
4. **Test changes**:
   ```bash
   make test
   make verify
   ```

### Code Style

- Follow Go conventions
- Use structured logging
- Add comprehensive tests
- Document configuration options
- Test with multiple terminal types

### Testing Guidelines

- Test SSH protocol compatibility
- Verify authentication flows
- Test game session management
- Validate reconnection scenarios
- Test spectating functionality

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🙏 Acknowledgments

- Original dgamelaunch project inspiration
- Go SSH library developers
- Terminal games community
- NetHack development team

---

**Built with ❤️ for the roguelike gaming community**

For questions, issues, or contributions, please visit our GitHub repository.