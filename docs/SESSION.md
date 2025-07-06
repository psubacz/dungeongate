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
cd /Users/caboose/Desktop/dungeongate
chmod +x build-ssh.sh
./build-ssh.sh setup
```

2. **Build the service:**
```bash
./build-ssh.sh build
```

3. **Start the service:**
```bash
./build-ssh.sh start
```

4. **Test SSH connection (in another terminal):**
```bash
./build-ssh.sh ssh
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

- **Full SSH Protocol Support**: SSH-2.0 compliant server
- **Terminal Management**: PTY allocation and terminal I/O handling
- **Game Integration**: Launch and manage terminal games
- **Session Recording**: TTY recording for playback
- **Real-time Spectating**: Watch other players' games with immutable data streaming
- **Authentication**: Flexible authentication backends
- **Microservices**: Clean separation of concerns

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

### Game Configuration

Games are configured through the Game Service:

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

Run the test suite:

```bash
# Run all tests
./build-ssh.sh test

# Run specific test
go test -v ./internal/session -run TestSSHServer

# Run benchmarks
go test -v ./internal/session -bench=.

# Run with coverage
go test -v ./internal/session -cover
```

### Build Scripts

The project includes a comprehensive build script:

```bash
./build-ssh.sh [command]

Commands:
  setup     - Initial setup (directories, keys, config)
  build     - Build the service binary
  test      - Run unit tests
  start     - Start service in development mode
  stop      - Stop the running service
  restart   - Restart the service
  ssh       - Test SSH connection
  health    - Check service health
  logs      - Show service logs
  clean     - Clean build artifacts
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
./build-ssh.sh setup
./build-ssh.sh start

# Connect via SSH
ssh -p 2222 admin@localhost
```

### Production Deployment

1. **Build for production:**
```bash
go build -ldflags="-s -w" -o dungeongate-session-service ./cmd/session-service
```

2. **Install systemd service:**
```bash
sudo cp dungeongate-session-service /usr/local/bin/
sudo cp configs/systemd/dungeongate-session.service /etc/systemd/system/
sudo systemctl enable dungeongate-session
sudo systemctl start dungeongate-session
```

3. **Configure firewall:**
```bash
sudo ufw allow 22/tcp    # SSH
sudo ufw allow 8083/tcp  # HTTP API (if needed)
```

### Docker Deployment

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o dungeongate-session-service ./cmd/session-service

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/dungeongate-session-service .
COPY configs/docker/session-service.yaml .
EXPOSE 22 8083 9093
CMD ["./dungeongate-session-service", "-config", "session-service.yaml"]
```

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
# Check if service is running
./build-ssh.sh health

# Check port availability
lsof -i :2222

# Check logs
./build-ssh.sh logs
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
3. **Setup development environment**: `./build-ssh.sh setup`
4. **Make changes and test**: `./build-ssh.sh test`
5. **Submit pull request**

### Code Style

- Follow Go conventions
- Use `gofmt` for formatting
- Add comprehensive tests
- Document public APIs
- Use structured logging

### Testing Guidelines

- Write unit tests for all new features
- Include integration tests for SSH functionality
- Test with multiple terminal types
- Verify memory usage and performance
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