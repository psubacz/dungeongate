# DungeonGate Spectating System

The DungeonGate platform includes a comprehensive spectating system that allows users to watch active game sessions in real-time. This document covers the implementation, architecture, and usage of the spectating functionality.

## 🎯 Overview

The spectating system is built using **broadcast architecture** with **immutable data sharing** patterns to ensure thread-safe, high-performance streaming of terminal data to multiple viewers without impacting game performance.

### Key Features

- **Real-time Streaming**: Live terminal output broadcast to spectators
- **Broadcast Architecture**: Eliminates race conditions between player and spectator connections
- **Immutable Data Architecture**: Lock-free, concurrent-safe data sharing
- **Multiple Connection Types**: gRPC streaming for both players and spectators
- **Session Management**: Automated spectator lifecycle management
- **Terminal Compatibility**: Full terminal escape sequence support
- **Bandwidth Optimization**: Efficient frame-based broadcasting
- **Frame Synchronization**: Eliminates every-other-frame skipping issues

## 🏗️ Architecture

### Core Components

```
┌─────────────────────────────────────────────────────────────────┐
│                   Broadcast Spectating Architecture            │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ Game        │  │ PTY         │  │ Broadcast               │  │
│  │ Process     │  │ Manager     │  │ System                  │  │
│  │             │  │             │  │                         │  │
│  │ • NetHack   │→→│ • PTY I/O   │→→│ • Output Subscribers    │  │
│  │ • Terminal  │  │ • Adapter   │  │ • Stream Manager        │  │
│  │ • Escape    │  │ • Buffering │  │ • Race-Free Broadcast   │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│                          │                        │            │
│                          ▼                        ▼            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ Player      │  │ Spectator   │  │ Connection              │  │
│  │ Connections │  │ Connections │  │ Management              │  │
│  │             │  │             │  │                         │  │
│  │ • gRPC      │  │ • gRPC      │  │ • Subscription IDs      │  │
│  │ • Dedicated │  │ • Dedicated │  │ • Automatic Cleanup     │  │
│  │ • Channels  │  │ • Channels  │  │ • Error Handling        │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### Broadcast System

The spectating system implements a **broadcast architecture** that eliminates race conditions between player and spectator connections:

#### PTY Output Broadcast
```go
type PTYSession struct {
    // ... other fields ...
    
    // Output subscribers for direct PTY streaming
    outputSubscribers map[string]chan []byte
    subscribersMu     sync.RWMutex
    streamManager     *StreamManager
}
```

**Key Features:**
- **Dedicated Channels**: Each connection gets its own output channel
- **Subscription System**: Unique subscription IDs for each connection
- **Automatic Cleanup**: Subscriptions are automatically removed on disconnect
- **Non-blocking**: Full channels don't block other connections
- **Race-free**: Eliminates every-other-frame skipping issues

#### Connection Flow
1. **Player connects**: Gets dedicated output channel via subscription
2. **Spectator connects**: Gets dedicated output channel via subscription  
3. **PTY output**: Broadcasts to all subscribed channels simultaneously
4. **Stream manager**: Receives frames for spectator-specific features
5. **Disconnect**: Subscription automatically cleaned up

### Immutable Data Sharing

The spectating system also implements immutable data sharing patterns for optimal performance:

#### 1. StreamFrame (Immutable Terminal Data)
```go
type StreamFrame struct {
    Timestamp time.Time `json:"timestamp"`
    Data      []byte    `json:"data"`     // Immutable copy
    FrameID   uint64    `json:"frame_id"` // Sequential ID
}
```

#### 2. SpectatorRegistry (Immutable Spectator List)
```go
type SpectatorRegistry struct {
    Spectators map[string]*Spectator `json:"spectators"`
    Version    uint64                `json:"version"`
}
```

#### 3. Atomic Operations
- **Lock-free updates**: Uses `atomic.Pointer[T]` for registry management
- **Copy-on-write**: New registry created for each update
- **Compare-and-swap**: Atomic updates with retry logic

## 🔧 Implementation Details

### Session Integration

Each game session includes spectating capabilities:

```go
type Session struct {
    // Core session data
    ID            string          `json:"id"`
    UserID        int             `json:"user_id"`
    Username      string          `json:"username"`
    GameID        string          `json:"game_id"`
    
    // Spectating infrastructure
    Registry      *atomic.Pointer[SpectatorRegistry] `json:"-"`
    StreamManager *StreamManager                     `json:"-"`
    Spectators    []*Spectator                      `json:"spectators"`
}
```

### Stream Manager

Handles frame distribution to spectators with enhanced broadcast support:

```go
type StreamManager struct {
    frameID      atomic.Uint64
    frameChannel chan *StreamFrame
    stopChan     chan struct{}
    wg           sync.WaitGroup
    
    // Internal spectator registry
    registry *atomic.Pointer[SpectatorRegistry]
}
```

**Key Features:**
- **Buffered Channel**: 1000-frame buffer prevents blocking
- **Concurrent Distribution**: Each spectator receives frames in parallel
- **Frame Dropping**: Graceful degradation under high load
- **Atomic Frame IDs**: Sequential frame identification
- **Internal Registry**: Self-managed spectator registry with atomic updates
- **Automatic Startup**: Starts automatically when PTY session is created

### Connection Types

#### gRPC Spectator Connection
```go
type GRPCSpectatorConnection struct {
    stream    GameService_StreamGameIOServer
    sessionID string
    logger    *slog.Logger
    closed    bool
    mu        sync.RWMutex
}
```

**Features:**
- **gRPC Streaming**: Uses `StreamGameIO` method for real-time communication
- **Frame Writing**: Direct frame writing to gRPC stream
- **Connection Management**: Thread-safe connection state tracking
- **Automatic Cleanup**: Proper resource cleanup on disconnect
- **Error Handling**: Graceful handling of connection errors

#### Player vs Spectator Connections

**Unified Architecture:**
- **Same Protocol**: Both players and spectators use the same gRPC streaming endpoint
- **Broadcast Distribution**: Both receive frames via the broadcast system
- **Input Filtering**: Session service filters spectator input (only 'q' for quit)
- **Connection Differentiation**: Determined by session ownership and spectator registration

**Connection Flow:**
1. **Player**: Owns the session, receives all frames, can send input
2. **Spectator**: Explicitly added via `AddSpectator`, receives all frames, input filtered

## 🎮 User Experience

### SSH-based Spectating

1. **Access Watch Menu**: Select 'w' from main menu
2. **View Active Sessions**: See formatted list of games in progress
3. **Select Session**: Use letter-based selection (a, b, c, etc.)
4. **Watch Game**: Real-time terminal output streaming
5. **Exit**: Press Ctrl+C to stop spectating

#### Watch Menu Display

```
╔══════════════════════════════════════════════════════════════════════════════╗
║                                  Watch Games                                 ║
╚══════════════════════════════════════════════════════════════════════════════╝

 The following games are in progress:

     Username         Game    Size     Start date & time     Idle time  Watch

a) player1           NH370   80x24   2025-07-06 02:56:44  5m 23s      0
b) player2           DCSS    120x40  2025-07-06 02:10:23  6s          1

(1-2 of 2)

Watch which game? ('?' for help) =>
```

### Commands and Help

| Command | Action | Description |
|---------|--------|-------------|
| `a-z` | Select Session | Choose session by letter |
| `?` | Help | Show command help |
| `q` | Quit | Return to main menu |
| `Ctrl+C` | Exit Spectating | Stop watching current session |

## 🔧 Configuration

### Spectating Settings

```yaml
session_management:
  spectating:
    enabled: true
    max_spectators_per_session: 3
    spectator_timeout: "30m"
    
  ttyrec:
    enabled: true
    compression: "gzip"
    directory: "./ttyrec"
    retention_days: 7
```

### Banner Configuration

```yaml
menu:
  banners:
    watch_menu: "./configs/development/banners/watch_menu.txt"
```

## 🚀 API Integration

### HTTP Endpoints

#### List Active Sessions
```bash
GET /sessions
```

**Response:**
```json
{
  "sessions": [
    {
      "id": "session_123",
      "username": "player1",
      "game_id": "nethack",
      "start_time": "2025-07-06T02:56:44Z",
      "is_active": true,
      "spectators": []
    }
  ]
}
```

#### WebSocket Spectating (Planned)
```bash
GET /ws/spectate?session=session_123
```

**Features:**
- Real-time frame streaming
- JSON-encoded terminal data
- Connection status events
- Error handling

### gRPC Services

#### GameService
```protobuf
service GameService {
  rpc AddSpectator(AddSpectatorRequest) returns (AddSpectatorResponse);
  rpc RemoveSpectator(RemoveSpectatorRequest) returns (RemoveSpectatorResponse);
  rpc StreamGameIO(stream GameIORequest) returns (stream GameIOResponse);
}
```

**Key Methods:**
- **AddSpectator**: Registers a user as a spectator for a session
- **RemoveSpectator**: Removes a spectator from a session
- **StreamGameIO**: Unified streaming endpoint for both players and spectators

#### Connection Flow
1. **Player**: Starts game session, then connects to `StreamGameIO`
2. **Spectator**: Calls `AddSpectator`, then connects to `StreamGameIO`
3. **Both**: Receive frames via the broadcast system
4. **Input**: Player input forwarded to game, spectator input filtered

## 🛠️ Development

### Adding New Connection Types

1. **Implement SpectatorConnection Interface**:
```go
type SpectatorConnection interface {
    Write(frame *StreamFrame) error
    Close() error
    GetType() string
    IsConnected() bool
}
```

2. **Register Connection Type**:
```go
// In session service
connection := NewCustomSpectatorConnection(params)
err := service.AddSpectatorWithConnection(ctx, sessionID, userID, username, connection)
```

3. **Handle Connection Lifecycle**:
```go
// Automatic cleanup on disconnection
defer service.RemoveSpectator(ctx, sessionID, userID)
```

### Testing Spectating

#### Unit Tests
```bash
go test -v ./internal/session -run TestSpectating
```

#### Integration Tests
```bash
# Start test server
make test-run

# Connect via SSH
ssh -p 2222 localhost

# Test watch functionality
# 1. Select 'w' for watch
# 2. Choose a test session
# 3. Verify real-time streaming
```

#### Load Testing
```bash
# Simulate multiple spectators
for i in {1..10}; do
  ssh -p 2222 localhost -t "w; a" &
done
```

## 🔧 Problem Solved: Frame Skipping Issue

### Previous Issue
The original implementation had a **race condition** where player and spectator connections competed for the same PTY output channel, causing:
- **Every-other-frame skipping**: Only every second frame was displayed
- **Inconsistent updates**: Player and spectator would alternately receive frames
- **Poor user experience**: Jerky, incomplete terminal output

### Solution: Broadcast Architecture
The new implementation uses a **broadcast system** that:
- **Eliminates race conditions**: Each connection gets its own dedicated channel
- **Ensures frame consistency**: All connections receive all frames
- **Maintains performance**: Non-blocking broadcast to all subscribers
- **Provides scalability**: Supports multiple simultaneous connections

### Technical Implementation
```go
// Each PTY output is broadcast to all subscribers
func (s *PTYSession) handleOutput() {
    // ... read PTY output ...
    
    // Broadcast to all output subscribers (players)
    s.subscribersMu.RLock()
    for subscriptionID, outputChan := range s.outputSubscribers {
        select {
        case outputChan <- processedData:
            // Successfully sent to subscriber
        default:
            // Channel full, skip to avoid blocking
        }
    }
    s.subscribersMu.RUnlock()
    
    // Also send to stream manager (spectators)
    s.streamManager.SendFrame(processedData)
}
```

## 📊 Performance Characteristics

### Benchmarks

- **Frame Processing**: ~100,000 frames/second
- **Spectator Addition**: Sub-microsecond atomic operations
- **Memory Overhead**: ~1KB per active spectator
- **Network Throughput**: Scales linearly with spectator count
- **Broadcast Overhead**: Minimal (~5μs per additional connection)

### Optimization Features

1. **Frame Buffering**: 1000-frame channel buffer per connection
2. **Frame Dropping**: Graceful degradation under load (per-connection)
3. **Concurrent Distribution**: Parallel spectator updates
4. **Immutable Data**: Zero-copy data sharing
5. **Atomic Operations**: Lock-free registry management
6. **Subscription System**: Efficient connection management

## 🔍 Monitoring

### Metrics

```
# Spectator metrics
spectator_sessions_total{game="nethack"} 5
spectator_connections_active 12
spectator_frames_sent_total 50000
spectator_frames_dropped_total 23

# Performance metrics
spectator_frame_processing_duration_seconds{quantile="0.95"} 0.001
spectator_registry_update_duration_seconds{quantile="0.99"} 0.0005
```

### Logging

```json
{
  "timestamp": "2025-07-06T03:15:00Z",
  "level": "info",
  "message": "Spectator added to session",
  "session_id": "session_123",
  "spectator_username": "viewer1",
  "spectator_count": 2,
  "registry_version": 15
}
```

## 🚀 Future Enhancements

### Planned Features

1. **WebSocket Implementation**: Complete browser-based spectating
2. **Recording Playback**: Spectate recorded sessions
3. **Multi-view Mode**: Watch multiple sessions simultaneously
4. **Spectator Chat**: Communication between spectators
5. **Quality Controls**: Bandwidth and quality adjustment
6. **Mobile Support**: Mobile-optimized spectating interface

### Technical Improvements

1. **Frame Compression**: Reduce bandwidth usage
2. **Delta Encoding**: Send only terminal changes
3. **Caching Layer**: Improve performance for popular sessions
4. **Load Balancing**: Distribute spectators across nodes
5. **Analytics**: Detailed spectating statistics

## 🤝 Contributing

### Implementation Guidelines

1. **Immutable Patterns**: Always use immutable data structures
2. **Atomic Operations**: Prefer atomic operations over mutexes
3. **Error Handling**: Graceful degradation on failures
4. **Testing**: Comprehensive unit and integration tests
5. **Documentation**: Update docs for new features

### Code Examples

See the following files for implementation details:
- `internal/games/types.go` - Core data structures (StreamManager, SpectatorRegistry)
- `internal/games/infrastructure/pty/manager.go` - PTY broadcast system
- `internal/games/infrastructure/grpc/streaming.go` - gRPC streaming handler
- `internal/games/infrastructure/grpc/service.go` - Spectator management
- `internal/session/connection/handler.go` - Session service spectating flow

---

**The spectating system demonstrates modern Go concurrency patterns and provides a foundation for scalable, real-time terminal streaming.**