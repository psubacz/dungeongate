# DungeonGate Game Service

The Game Service is the core game management backend in the DungeonGate microservices architecture. It handles game process lifecycle, PTY management, and provides gRPC APIs for the Session Service to create and manage terminal-based games like NetHack and Dungeon Crawl Stone Soup.

## 🚀 Overview

The Game Service provides:

- **Game Process Management**: Start, stop, and monitor game processes with proper lifecycle management
- **PTY Management**: Pseudo-terminal allocation and I/O handling for interactive games
- **Game Adapters**: Extensible adapter system for different game types (NetHack, DCSS, etc.)
- **Session Persistence**: Games survive client disconnections and support reconnection
- **gRPC API**: High-performance service integration with Session Service
- **Configuration Management**: Per-game configuration with environment setup
- **Process Monitoring**: Health checks and process state tracking

## 🏗️ Architecture

### Game Service Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Game Service                             │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ gRPC Server │  │ Session     │  │ Game Adapters       │  │
│  │             │  │ Manager     │  │                     │  │
│  │ • StartGame │  │ • Lifecycle │  │ • NetHack Adapter   │  │
│  │ • StopGame  │  │ • Tracking  │  │ • Default Adapter   │  │
│  │ • Streaming │  │ • Cleanup   │  │ • Custom Adapters   │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ PTY Manager │  │ Streaming   │  │ Configuration       │  │
│  │             │  │ Handler     │  │ Management          │  │
│  │ • Process   │  │ • I/O       │  │ • Game Configs      │  │
│  │ • Terminal  │  │ • Bridging  │  │ • Environments      │  │
│  │ • Lifecycle │  │ • Events    │  │ • Path Management   │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Process Flow

```
Session Service → Game Service → PTY → NetHack Process
       ↑               ↓           ↓           ↓
   SSH Client      gRPC API    Terminal    Game Output
                               I/O        (stdout/stderr)
```

## 🔧 Core Components

### 1. gRPC Service (`internal/games/infrastructure/grpc/service.go`)

The main gRPC service implementation providing:

```go
type GameService struct {
    ptyManager      *pty.PTYManager
    sessionManager  *session.SessionManager
    logger          *slog.Logger
    adapters        *adapters.GameAdapterRegistry
}

// Key gRPC Methods
func (s *Service) StartGameSession(ctx context.Context, req *StartGameSessionRequest) (*StartGameSessionResponse, error)
func (s *Service) StopGameSession(ctx context.Context, req *StopGameSessionRequest) (*StopGameSessionResponse, error)
func (s *Service) StreamGameIO(stream GameService_StreamGameIOServer) error
```

**Recent Fix**: Uses `context.Background()` for PTY creation to prevent process termination when gRPC contexts are cancelled.

### 2. PTY Manager (`internal/games/infrastructure/pty/manager.go`)

Manages pseudo-terminals and game processes:

```go
type PTYManager struct {
    sessions map[string]*PTYSession
    logger   *slog.Logger
    adapters *adapters.GameAdapterRegistry
}

type PTYSession struct {
    SessionID  string
    PTY        *os.File
    Cmd        *exec.Cmd
    Size       *pty.Winsize
    inputChan  chan []byte
    outputChan chan []byte
    errorChan  chan error
    closeChan  chan struct{}
    adapter    adapters.GameAdapter
    session    *domain.GameSession
}
```

**Key Features**:
- **Process Independence**: Games survive PTY session disconnections
- **Graceful Cleanup**: Proper process termination only on explicit user quit
- **Session Persistence**: Support for reconnection to ongoing games

### 3. Game Adapters (`internal/games/adapters/`)

Extensible adapter system for different game types:

#### NetHack Adapter (`nethack_adapter.go`)
- Environment setup with proper NetHack directories
- Save file management and permissions
- Terminal configuration for optimal NetHack experience
- **Fixed**: Uses `exec.Command()` instead of `exec.CommandContext()` to prevent context-based termination

#### Default Adapter (`default_adapter.go`)
- Generic adapter for simple terminal applications
- Basic environment and command setup
- **Fixed**: Same context independence fix applied

### 4. Session Manager (`internal/games/application/session_manager.go`)

Manages game session lifecycle:

```go
type SessionManager struct {
    sessions    map[string]*GameSession
    mutex       sync.RWMutex
    logger      *slog.Logger
}

func (sm *SessionManager) StartGameSession(ctx context.Context, req *StartGameSessionRequest) (*GameSession, error)
func (sm *SessionManager) EndGameSession(ctx context.Context, sessionID string, reason string) error
func (sm *SessionManager) GetActiveSessionsForUser(ctx context.Context, userID int64) ([]*GameSession, error)
```

### 5. Streaming Handler (`internal/games/infrastructure/grpc/streaming.go`)

Handles bidirectional streaming for game I/O:

```go
type StreamHandler struct {
    ptyManager *pty.PTYManager
    sessions   map[string]*StreamSession
    logger     *slog.Logger
}

func (h *StreamHandler) HandleStream(stream GameService_StreamGameIOServer) error
```

**Recent Fix**: Removed duplicate close calls that were interfering with session lifecycle.

## 🎮 Game Configuration

### Game Service Configuration

Located at `configs/development/game-service.yaml`:

```yaml
# HTTP/gRPC Server Configuration
server:
  port: 8085          # HTTP API port
  grpc_port: 50051    # gRPC port for Session Service communication
  host: "localhost"
  timeout: "30s"
  max_connections: 100

# Database Configuration (shared with other services)
database:
  mode: "embedded"
  type: "sqlite"
  embedded:
    path: "./data/sqlite/dungeongate-dev.db"
    migration_path: "./migrations"

# Game Configuration
games:
  - id: "nethack"
    name: "NetHack"
    short_name: "nh"
    version: "3.6.7"
    enabled: true
    
    # Binary Configuration
    binary:
      path: "/opt/homebrew/bin/nethack"
      args: ["-u", "${USERNAME}"]
      working_directory: "/tmp"
      permissions: "0755"
      
    # File and Directory Management
    files:
      log_directory: "/tmp/nethack-logs"
      temp_directory: "/tmp/nethack-temp"
      shared_files: ["nhdat", "license", "recover"]
      user_files: ["${USERNAME}.nh", "${USERNAME}.0", "${USERNAME}.bak"]
      
    # Game-specific Settings
    settings:
      max_players: 50
      max_session_duration: "4h"
      idle_timeout: "30m"
      save_interval: "5m"
      auto_save: true
      
      # Spectating Configuration
      spectating:
        enabled: true
        max_spectators_per_session: 5
        spectator_timeout: "2h"
        
      # Recording Configuration
      recording:
        enabled: true
        format: "ttyrec"
        compression: "gzip"
        max_file_size: "100MB"
        retention_days: 30
        
    # Environment Variables
    environment:
      TERM: "xterm-256color"
      USER: "${USERNAME}"
      LOGNAME: "${USERNAME}"
      
    # Resource Limits
    resources:
      cpu_limit: "500m"
      memory_limit: "256Mi"
      disk_limit: "1Gi"
      pids_limit: 50
```

### NetHack-Specific Configuration

The NetHack adapter uses advanced configuration for proper game environment setup:

```yaml
# NetHack Adapter Configuration (in game config)
nethack:
  paths:
    shared:
      data_dir: "/opt/homebrew/Cellar/nethack/3.6.7/libexec"
      hack_dir: "/opt/homebrew/Cellar/nethack/3.6.7/libexec"
    user:
      playground_dir: "games/nethack"
      save_dir: "games/nethack/saves"
      level_dir: "games/nethack/levels"
      bones_dir: "games/nethack/bones"
      lock_dir: "games/nethack/locks"
      trouble_dir: "games/nethack/trouble"
      config_dir: "games/nethack/config"
      
  environment:
    NETHACKDIR: "${HACKDIR}"
    HACKDIR: "${HACKDIR}"
    NETHACK_PLAYGROUND: "${HOME}/${PLAYGROUND_DIR}"
    NETHACK_SAVEDIR: "${HOME}/${SAVE_DIR}"
    NETHACK_LEVELDIR: "${HOME}/${LEVEL_DIR}"
    NETHACK_BONESDIR: "${HOME}/${BONES_DIR}"
    NETHACK_LOCKDIR: "${HOME}/${LOCK_DIR}"
    NETHACK_TROUBLEDIR: "${HOME}/${TROUBLE_DIR}"
    NETHACK_CONFIGDIR: "${HOME}/${CONFIG_DIR}"
```

## 🔄 Process Management

### Game Process Lifecycle

1. **Session Creation**: Session Service requests game start via gRPC
2. **Environment Setup**: Game adapter prepares directories and environment
3. **Process Start**: PTY manager creates pseudo-terminal and starts game
4. **I/O Streaming**: Bidirectional streaming between Session Service and game
5. **Session Persistence**: Game continues running even if streams disconnect
6. **Clean Termination**: Process terminated only on explicit user quit

### Process Independence (Key Fix)

**Previous Issue**: Games were terminated when gRPC contexts were cancelled or streams disconnected.

**Solution Implemented**:
```go
// Before (caused premature termination)
cmd := exec.CommandContext(ctx, gamePath, args...)

// After (process independence)
cmd := exec.Command(gamePath, args...)
```

This ensures NetHack processes run independently of:
- gRPC request contexts
- Stream connection states
- Network interruptions
- Service restarts (if process survives)

### Session Persistence Features

- **Reconnection Support**: Users can reconnect to ongoing NetHack games
- **Save State Preservation**: Game saves are maintained across disconnections
- **Process Monitoring**: Health checks without process interference
- **Graceful Cleanup**: Proper cleanup only when games actually end

## 📡 gRPC API

### Service Definition

```protobuf
service GameService {
    // Start a new game session
    rpc StartGameSession(StartGameSessionRequest) returns (StartGameSessionResponse);
    
    // Stop an existing game session
    rpc StopGameSession(StopGameSessionRequest) returns (StopGameSessionResponse);
    
    // Bidirectional streaming for game I/O
    rpc StreamGameIO(stream GameIORequest) returns (stream GameIOResponse);
    
    // Health check
    rpc HealthGRPC(HealthRequestGRPC) returns (HealthResponseGRPC);
}
```

### Message Types

```protobuf
message StartGameSessionRequest {
    string user_id = 1;
    string game_id = 2;
    TerminalSize terminal_size = 3;
    map<string, string> environment = 4;
}

message StartGameSessionResponse {
    string session_id = 1;
    bool success = 2;
    string error = 3;
}

message GameIORequest {
    oneof request {
        ConnectPTYRequest connect = 1;
        PTYInput input = 2;
        DisconnectPTYRequest disconnect = 3;
    }
}

message GameIOResponse {
    oneof response {
        ConnectPTYResponse connected = 1;
        PTYOutput output = 2;
        PTYEvent event = 3;
        DisconnectPTYResponse disconnected = 4;
    }
}
```

### Error Handling

The service provides comprehensive error handling:

```go
// Common error responses
codes.InvalidArgument  // Invalid request parameters
codes.NotFound        // Session or game not found
codes.Internal        // Internal service errors
codes.Unavailable     // Service temporarily unavailable
codes.Cancelled       // Request cancelled (handled gracefully)
```

## 🔧 Game Adapters

### Adapter Interface

```go
type GameAdapter interface {
    SetupGameEnvironment(session *domain.GameSession) error
    CleanupGameEnvironment(session *domain.GameSession) error
    PrepareCommand(ctx context.Context, session *domain.GameSession, gamePath string, args []string, env []string) (*exec.Cmd, error)
    ProcessOutput(data []byte) []byte
    GetInitialInput() []byte
}
```

### NetHack Adapter Implementation

Key features of the NetHack adapter:

- **Directory Management**: Creates proper NetHack directory structure
- **Environment Setup**: Sets all required NetHack environment variables
- **Save File Handling**: Manages user-specific save files and permissions
- **Terminal Configuration**: Optimizes terminal settings for NetHack
- **Process Independence**: Uses context-free process creation

### Adding Custom Game Adapters

To add a new game type:

1. **Implement GameAdapter Interface**:
```go
type MyGameAdapter struct {
    config *MyGameConfig
}

func (a *MyGameAdapter) SetupGameEnvironment(session *domain.GameSession) error {
    // Setup game-specific directories and files
}

func (a *MyGameAdapter) PrepareCommand(ctx context.Context, session *domain.GameSession, gamePath string, args []string, env []string) (*exec.Cmd, error) {
    // IMPORTANT: Use exec.Command() not exec.CommandContext()
    cmd := exec.Command(gamePath, args...)
    cmd.Env = env
    // Additional setup...
    return cmd, nil
}
```

2. **Register Adapter**:
```go
registry := adapters.NewGameAdapterRegistry()
registry.RegisterAdapter("mygame", &MyGameAdapter{})
```

3. **Add Configuration**:
```yaml
games:
  - id: "mygame"
    name: "My Game"
    enabled: true
    # ... configuration
```

## 🛠️ Development

### Building and Running

```bash
# Build the game service
make build-game

# Run all services (recommended)
make run-all

# Run game service only (for debugging)
make run-game

# The game service runs on:
# - HTTP: localhost:8085
# - gRPC: localhost:50051
```

### Testing

```bash
# Run tests
make test

# Test specific components
make test-games

# Health check
curl http://localhost:8085/health
```

### Development Flow

1. **Start Services**: `make run-all`
2. **Connect via SSH**: `ssh -p 2222 localhost`
3. **Test Game Flow**: Login → Play → Select NetHack
4. **Monitor Logs**: Check Game Service logs for process management
5. **Verify Persistence**: Disconnect and reconnect to verify session survival

## 📊 Monitoring and Observability

### Health Checks

```go
func (s *Service) HealthGRPC(ctx context.Context, req *HealthRequestGRPC) (*HealthResponseGRPC, error) {
    activeSessions := s.sessionManager.GetActiveSessionCount()
    
    return &HealthResponseGRPC{
        Status: "healthy",
        Details: map[string]string{
            "active_sessions": fmt.Sprintf("%d", activeSessions),
            "timestamp":       time.Now().Format(time.RFC3339),
            "version":         version,
        },
    }, nil
}
```

### Metrics (Future Implementation)

Planned Prometheus metrics:

- `game_sessions_total` - Total game sessions started
- `game_sessions_active` - Currently active game sessions
- `game_sessions_duration_seconds` - Session duration histogram
- `game_process_starts_total` - Total game process starts
- `game_process_failures_total` - Failed process starts
- `pty_sessions_active` - Active PTY sessions

### Logging

Structured logging throughout the service:

```json
{
  "timestamp": "2025-07-13T18:45:00Z",
  "level": "info",
  "message": "Game session started",
  "session_id": "session_1752432358790097000",
  "user_id": "user_1",
  "game_id": "nethack",
  "process_id": 90486
}
```

## 🔒 Security Considerations

### Process Security

- **User Isolation**: Games run as specific users with limited permissions
- **Resource Limits**: CPU, memory, and process limits enforced
- **File System**: Controlled access to game directories and files
- **Environment**: Sanitized environment variables

### Service Security

- **gRPC Security**: Service-to-service authentication (future)
- **Input Validation**: All user inputs validated and sanitized
- **Process Monitoring**: Detection of unusual process behavior
- **Error Handling**: Secure error responses without information leakage

### Configuration Security

```yaml
# Security settings in game config
security:
  process_limits:
    max_processes: 50
    max_memory: "256Mi"
    max_cpu: "500m"
    
  file_permissions:
    user_directories: "0755"
    save_files: "0644"
    executable_files: "0755"
    
  environment:
    allowed_variables: ["TERM", "USER", "HOME", "NETHACK*"]
    blocked_variables: ["PATH", "LD_*", "SHELL"]
```

## 🚀 Deployment

### Development Deployment

The Game Service is designed to run alongside Session and Auth services:

```bash
# Start all services
make run-all

# Services will be available at:
# - Game Service gRPC: localhost:50051
# - Game Service HTTP: localhost:8085
# - Session Service SSH: localhost:2222
# - Auth Service: localhost:8081/8082
```

### Production Deployment

For production environments:

1. **Configuration Updates**:
```yaml
server:
  host: "0.0.0.0"
  max_connections: 1000

database:
  mode: "external"
  type: "postgresql"
  # ... production database config

games:
  - id: "nethack"
    settings:
      max_players: 500
      max_session_duration: "8h"
```

2. **Security Hardening**:
```yaml
security:
  process_limits:
    max_processes: 100
    max_memory: "512Mi"
    
  file_permissions:
    strict_mode: true
    readonly_system: true
```

3. **Resource Management**:
```yaml
resources:
  global_limits:
    max_concurrent_games: 1000
    memory_per_game: "128Mi"
    cpu_per_game: "200m"
```

## 🔮 Future Enhancements

### Planned Features

1. **Advanced Session Management**:
   - Session migration between service instances
   - Load balancing for game processes
   - Automatic session cleanup and archival

2. **Enhanced Game Support**:
   - Dungeon Crawl Stone Soup adapter
   - Custom game plugin system
   - Game-specific optimization profiles

3. **Monitoring and Analytics**:
   - Real-time performance metrics
   - Game session analytics
   - Player behavior tracking

4. **Scalability Features**:
   - Horizontal scaling support
   - Container orchestration integration
   - Distributed session storage

### Technical Improvements

1. **Performance Optimization**:
   - Process pooling for faster game starts
   - Optimized I/O streaming
   - Memory usage optimization

2. **Reliability Features**:
   - Automatic process recovery
   - Health-based failover
   - Graceful degradation

3. **Developer Experience**:
   - Hot-reload for game configurations
   - Debugging tools and instrumentation
   - Enhanced logging and tracing

## 🔍 Troubleshooting

### Common Issues

#### Game Process Won't Start

```bash
# Check game binary exists and is executable
ls -la /opt/homebrew/bin/nethack

# Check directory permissions
ls -la /tmp/nethack-users/

# Verify configuration
curl http://localhost:8085/health

# Check logs for specific errors
tail -f game-service.log
```

#### Process Terminated Immediately (FIXED)

**Previous Issue**: NetHack processes were killed after startup
**Status**: RESOLVED
**Verification**: 
```bash
# Logs should show:
# "Process exited successfully" (only when user quits)
# NOT "Process was killed by signal"
```

#### Session Won't Connect

```bash
# Check if Game Service is running
curl http://localhost:8085/health

# Verify gRPC connectivity
grpcurl -plaintext localhost:50051 list

# Check Session Service connectivity
ssh -p 2222 localhost
```

### Debug Commands

```bash
# Check service status
curl http://localhost:8085/health

# List active sessions (if endpoint exists)
curl http://localhost:8085/sessions

# Check process list
ps aux | grep nethack

# Monitor game service logs
tail -f /path/to/game-service.log

# Test gRPC connectivity
grpcurl -plaintext localhost:50051 describe dungeongate.GameService
```

### Performance Tuning

For high-load scenarios:

```yaml
server:
  max_connections: 5000
  timeout: "300s"

games:
  - settings:
      max_players: 1000
      max_session_duration: "12h"
      
resources:
  global_limits:
    max_concurrent_games: 2000
    memory_per_game: "64Mi"
```

## 📖 API Documentation

### Complete gRPC Service Methods

```go
// Start a new game session
StartGameSession(context.Context, *StartGameSessionRequest) (*StartGameSessionResponse, error)

// Stop an existing game session  
StopGameSession(context.Context, *StopGameSessionRequest) (*StopGameSessionResponse, error)

// Bidirectional streaming for real-time game I/O
StreamGameIO(GameService_StreamGameIOServer) error

// Health check for service monitoring
HealthGRPC(context.Context, *HealthRequestGRPC) (*HealthResponseGRPC, error)
```

### HTTP Endpoints (Current and Planned)

```
GET  /health              # Service health check
GET  /games               # List available games (planned)
GET  /sessions            # List active sessions (planned)
POST /sessions            # Create session (planned)
DELETE /sessions/{id}     # Stop session (planned)
GET  /metrics             # Prometheus metrics (planned)
```

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🙏 Acknowledgments

- NetHack development team for the amazing game
- Go community for excellent libraries and tools
- PTY and terminal emulation researchers
- Open source game server projects for inspiration

---

**Built with ❤️ for terminal gaming excellence**

For technical questions or contributions, please visit our GitHub repository or open an issue.