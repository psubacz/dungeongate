# Game Service Architecture Roadmap

## 🎯 Game Service Architecture Vision

### **Core Philosophy: Event-Driven Microservice with Clean Separation**

**Architecture Pattern**: Domain-Driven Design (DDD) with CQRS and Event Sourcing
- **Domain**: Game lifecycle, process management, save states
- **Command Side**: Game operations (start, stop, save, load)
- **Query Side**: Game metadata, status, statistics
- **Events**: Game state changes, session events

### **Service Boundaries**
```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  Session Service │────│  Game Service   │────│  Storage Service│
│  (SSH/Terminal) │    │  (Lifecycle)    │    │  (Saves/Config) │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │              ┌─────────────────┐              │
         └──────────────│  Auth Service   │──────────────┘
                        │ (Identity/Perms)│
                        └─────────────────┘
```

## 🐛 Known Issues

### **SSH Client Window Size Compatibility**
**Issue**: Some SSH clients send `width=0` in PTY requests, causing NetHack and other terminal games to fail with exit status 1.

**Root Cause**: SSH clients may send invalid window dimensions (0x0) in the initial PTY request payload, resulting in `COLUMNS=0` and `LINES=0` environment variables being passed to games.

**Solution**: Implemented fallback using `max()` function in PTY manager:
```go
session.Environment["COLUMNS"] = fmt.Sprintf("%d", max(windowSize.Width, 80))
session.Environment["LINES"] = fmt.Sprintf("%d", max(windowSize.Height, 24))
```

**Files**: 
- `internal/session/pty_manager.go` - PTY environment variable setup
- `internal/session/ssh.go` - SSH PTY request parsing

**Impact**: Ensures minimum viable terminal dimensions (80x24) for all games, preventing startup failures due to invalid window sizes.

## ✅ Implementation Status Update

### **Phase 1-3 Complete: Game Service Foundation Implemented**

**Status**: ✅ **COMPLETED** - Game Service architecture and foundation implemented successfully

#### **✅ Completed Tasks:**

1. **✅ Save Management Extracted** 
   - `internal/session/save_manager.go` → `internal/games/saves/manager.go`
   - Added domain-driven save types and interfaces in `internal/games/saves/types.go`

2. **✅ Game Client Extracted**
   - `internal/session/game_client.go` → `internal/games/client/grpc_client.go`  
   - Added proper types in `internal/games/client/types.go`

3. **✅ Game Types Extracted**
   - Game-related types moved from `internal/session/types.go` to `internal/games/types.go`
   - Eliminated duplicate type definitions

4. **✅ Domain-Driven Design Structure**
   - Created comprehensive DDD package structure in `internal/games/`
   - Implemented domain aggregates: Game, GameSession, GameSave
   - Added repository interfaces with proper abstraction

5. **✅ Application Services**
   - Implemented `GameService` and `SessionService` in application layer
   - Added proper request/response types and validation

6. **✅ Game Service Entry Point**
   - Created `cmd/game-service/main.go` with dual HTTP/gRPC servers
   - Added graceful shutdown and health check endpoints

7. **✅ gRPC Protocol Definition**
   - Comprehensive protobuf definition in `api/proto/games/game_service.proto`
   - Added Makefile targets for code generation: `make proto-gen`

#### **📁 Current Package Structure (Implemented):**
```
✅ internal/games/
├── ✅ domain/              # Domain models and interfaces
│   ├── ✅ game.go         # Game aggregate root
│   ├── ✅ session.go      # GameSession aggregate 
│   ├── ✅ save.go         # GameSave aggregate
│   └── ✅ repository.go   # Repository interfaces
├── ✅ application/         # Application services and use cases
│   ├── ✅ game_service.go # Game management use cases
│   ├── ✅ session_service.go # Session management use cases
│   └── ✅ types.go        # Request/response DTOs
├── ✅ infrastructure/     # External integrations
│   └── ✅ repository/     # Repository implementations
├── ✅ saves/              # Save management (extracted from session)
│   ├── ✅ manager.go      # Save operations
│   └── ✅ types.go        # Save interfaces
├── ✅ client/             # Game service client (extracted)
│   ├── ✅ grpc_client.go  # gRPC client implementation
│   └── ✅ types.go        # Client types
└── ✅ types.go            # Common game types
```

#### **🚀 Ready for Next Phase:**
- **✅ Build Command**: `make build-game-service`
- **✅ Protocol Generation**: `make proto-gen` 
- **✅ Testing**: `make test-game-service`

## ✅ Integration Complete - Service Communication Established

### **Session Service Integration (COMPLETED)**
The session service has been successfully updated to use the game service:

1. **✅ Updated session service gRPC client** - Real protobuf-generated client replaces all mock implementations
2. **✅ Replaced mock implementations** - All game service calls now use real gRPC communication
3. **✅ Service communication verified** - Both services build and can communicate via gRPC
4. **✅ gRPC method implementations** - StartGame, StopGame, ListGames, GetGameSession, and HealthCheck all implemented

### **Next Steps: Repository Implementation and Advanced Features**
With the core microservices architecture complete, the focus shifts to:

1. **Repository implementations** - Complete the database layer in the game service
2. **Game process management** - Implement actual game launching and process management
3. **Advanced features** - Stream encryption, game isolation, RabbitMQ spectating

## 📋 Implementation Focus: internal/games Package Structure

### **Setting up internal/games Package Structure**
**What it involves:**
- Create the Domain-Driven Design directory structure
- Implement core domain models (Game, Session, Save)
- Set up repository interfaces and basic implementations
- Create application service layer

**Benefits:**
- Clean separation of concerns
- Testable business logic
- Foundation for all future features
- Proper abstraction layers

**Package Structure:**
```go
internal/games/
├── domain/           # Domain models and interfaces
│   ├── game.go      # Game aggregate root
│   ├── session.go   # Game session entity
│   ├── save.go      # Save management (moved from session service)
│   └── repository.go # Repository interfaces
├── application/      # Application services and use cases
│   ├── game_service.go
│   ├── commands/    # Command handlers
│   └── queries/     # Query handlers
├── infrastructure/  # External integrations
│   ├── grpc/       # gRPC server
│   ├── http/       # REST API
│   ├── process/    # Process management
│   ├── container/  # Container runtime
│   └── storage/    # File system operations
├── saves/           # Save management (moved from session service)
│   ├── manager.go   # Save operations
│   └── types.go     # Save data structures
├── client/          # Game service client (moved from session service)
│   └── grpc_client.go
└── adapters/        # External service adapters
    ├── session_client.go
    └── auth_client.go
```


## 📋 Enhanced Roadmap with Additional Requirements

### **Additional Requirements Identified:**

#### **Stream Encryption Implementation**
```go
type StreamEncryption interface {
    EncryptFrame(data []byte, sessionKey []byte) ([]byte, error)
    DecryptFrame(encrypted []byte, sessionKey []byte) ([]byte, error)
    GenerateSessionKey() ([]byte, error)
    RotateKey(oldKey []byte) ([]byte, error)
}

// Implementation options:
// 1. AES-256-GCM for authenticated encryption
// 2. ChaCha20-Poly1305 for better performance
// 3. Key derivation from user session tokens
// 4. Automatic key rotation every N hours
```

#### **Game Isolation Architecture**
```go
type GameIsolation struct {
    ProcessIsolation  *ProcessIsolationConfig  // cgroups, namespaces
    FilesystemIsolation *FilesystemConfig      // separate user directories
    NetworkIsolation  *NetworkConfig           // separate network namespaces
    ResourceLimits    *ResourceConfig          // CPU/memory per game
}

// Isolation strategies:
// 1. Process-level: User namespaces + cgroups
// 2. Container-level: Docker user namespaces
// 3. Filesystem: Separate game directories per user
// 4. Network: Isolated network namespaces (no internet access)
```

#### **Shared Game State (NetHack Bones)**
```go
type SharedGameState interface {
    StoreBones(gameType string, level int, bones *BonesData) error
    LoadBones(gameType string, level int) (*BonesData, error)
    SyncBones(sourceServer, targetServer string) error
    CleanupOldBones(olderThan time.Duration) error
}

// Implementation:
// 1. Distributed storage (PostgreSQL/File system)
// 2. Event-driven synchronization via RabbitMQ
// 3. Conflict resolution for concurrent bones updates
// 4. Cleanup policies for old bones data
```

#### **RabbitMQ Spectating Model**
```go
type RabbitMQSpectatingBus struct {
    conn    *amqp.Connection
    channel *amqp.Channel
}

func (bus *RabbitMQSpectatingBus) PublishFrame(gameID string, frame *GameFrame) error {
    return bus.channel.Publish("game.frames", gameID, false, false, amqp.Publishing{
        Body: frame.Data,
    })
}

func (bus *RabbitMQSpectatingBus) SubscribeToGame(gameID string, spectatorID string) error {
    q, _ := bus.channel.QueueDeclare("spectator."+spectatorID, false, false, false, false, nil)
    return bus.channel.QueueBind(q.Name, gameID, "game.frames", false, nil)
}

// Events to handle:
// - GameStarted/GameEnded
// - PlayerDisconnected -> ReturnSpectatorsToLobby
// - GamePaused/GameResumed
// - PlayerJoined/PlayerLeft
```

#### **Object Pooling for Performance**
```go
// Reduce allocation churn with object pools
type FramePool struct {
    pool sync.Pool
}

func (p *FramePool) Get() *StreamFrame {
    if f := p.pool.Get(); f != nil {
        return f.(*StreamFrame)
    }
    return &StreamFrame{Data: make([]byte, 0, 4096)}
}

func (p *FramePool) Put(f *StreamFrame) {
    f.Data = f.Data[:0] // Reset slice
    p.pool.Put(f)
}
```BLACK

#### **Bounded SSH Connection Pool**
```go
type SSHConnectionPool struct {
    connections chan *SSHConnection
    workers     int
    semaphore   chan struct{}
}

// Prevent resource exhaustion with worker goroutines
func (p *SSHConnectionPool) HandleConnection(conn net.Conn) {
    select {
    case p.semaphore <- struct{}{}: // Acquire worker
        go func() {
            defer func() { <-p.semaphore }() // Release worker
            p.processConnection(conn)
        }()
    default:
        conn.Close() // Pool exhausted, reject connection
    }
}
```

## 🚀 Implementation Order

### **✅ Phase 1: Session Service Refactoring (COMPLETED)**
**Focus: Extract game logic from session service**

1. **✅ Extract save management** - Moved `internal/session/save_manager.go` to `internal/games/saves/`
2. **✅ Extract game client** - Moved `internal/session/game_client.go` to `internal/games/client/`
3. **✅ Extract game types** - Moved game-related types from `internal/session/types.go` to `internal/games/types.go`
4. **🔄 Remove mock implementations** - Replace fake game service client with real gRPC calls *(Next Priority)*

### **✅ Phase 2: Games Service Implementation (COMPLETED)**
**Focus: internal/games package structure with DDD**

5. **✅ Create internal/games package structure** with Domain-Driven Design
6. **✅ Implement core domain models** (Game, Session, Save)
7. **✅ Set up repository interfaces** and basic implementations
8. **✅ Create application service layer** for use cases

### **✅ Phase 3: Service Infrastructure (COMPLETED)**
**Focus: Service foundation and gRPC definitions**

9. **✅ Create service entry point** - `cmd/game-service/main.go` with HTTP/gRPC servers
10. **✅ Define gRPC protocol** - Comprehensive protobuf definitions
11. **✅ Add build system integration** - Makefile targets for building and code generation

### **✅ Phase 4: Service Integration (COMPLETED)**
**Focus: Connect session service to games service**

12. **✅ Implement gRPC communication** - Session service calls games service via real protobuf-generated clients
13. **✅ Refactor game launching** - Session service now uses game service gRPC calls instead of mock implementations
14. **✅ Update game client integration** - Replaced all mock gRPC calls with real protobuf service calls
15. **✅ Service interoperability** - Both services build and can communicate via gRPC

### **📋 Phase 5: Advanced Features (PLANNED)**
16. **Add RabbitMQ spectating integration**
17. **Add stream encryption** for security
18. **Implement game isolation** (critical for multi-user)
19. **Shared game state (bones) implementation**
20. **Object pooling optimization**

### **🎯 Current Status: Ready for Phase 5 - Advanced Features**
The microservices architecture is now complete! Both session and game services are fully integrated:
- **Session Service**: Handles SSH/terminal management and user authentication
- **Game Service**: Handles game lifecycle, process management, and save operations
- **gRPC Communication**: Real protobuf-based service-to-service communication established
- **Service Integration**: Session service successfully calls game service for all game-related operations

The next phase focuses on advanced features like RabbitMQ spectating, stream encryption, and game isolation.

## 🛠️ Developer Quick Start

### **Building and Running the Game Service**

```bash
# Generate gRPC code from protobuf definitions
make proto-gen

# Build the game service
make build-game-service

# Run tests
make test-game-service

# Start the game service (requires configuration)
./build/dungeongate-game-service -config configs/development/game-service.yaml
```

### **Service Architecture Overview**

```
┌─────────────────────────────────────────────────────────────┐
│                     🎮 Game Service                         │
├─────────────────────────────────────────────────────────────┤
│  🌐 gRPC API (port 50051)     🌐 HTTP API (port 8084)     │
├─────────────────────────────────────────────────────────────┤
│              📋 Application Layer                           │
│  ├── GameService (game management)                          │
│  ├── SessionService (session lifecycle)                     │
│  └── SaveService (save management)                          │
├─────────────────────────────────────────────────────────────┤
│              🏗️ Domain Layer                               │
│  ├── Game (aggregate)                                       │
│  ├── GameSession (aggregate)                                │
│  └── GameSave (aggregate)                                   │
├─────────────────────────────────────────────────────────────┤
│              🔧 Infrastructure Layer                        │
│  ├── Repository implementations                             │
│  ├── gRPC server                                           │
│  └── HTTP handlers                                         │
└─────────────────────────────────────────────────────────────┘
```

### **Key Design Principles Implemented**

1. **Domain-Driven Design**: Business logic encapsulated in domain aggregates
2. **Clean Architecture**: Dependencies point inward toward domain
3. **CQRS Ready**: Separate command and query interfaces
4. **Event-Driven**: Game events for system integration
5. **Microservice Patterns**: gRPC for service-to-service communication

### **Integration with Session Service**

The session service should now use:
```go
// Instead of internal implementations
import "github.com/dungeongate/internal/games/client"

// Use the extracted game client
gameClient := client.NewGameServiceGRPCClient("localhost:50051")
```

## 🛠 Technical Architecture

### **Recommended Technology Stack**

#### **Core Technologies:**
- **Language**: Go 1.21+
- **Communication**: gRPC for service-to-service, HTTP/REST for external
- **Database**: PostgreSQL (primary) + SQLite (development)
- **Event Bus**: RabbitMQ for event streaming and spectating
- **Container Runtime**: Docker + Kubernetes
- **Monitoring**: Prometheus + Grafana

#### **Game Process Management:**
```go
type GameService interface {
    // Lifecycle Management
    StartGame(ctx context.Context, req *StartGameRequest) (*GameInstance, error)
    StopGame(ctx context.Context, gameID string) error
    PauseGame(ctx context.Context, gameID string) error
    ResumeGame(ctx context.Context, gameID string) error
    
    // State Management
    SaveGame(ctx context.Context, gameID string) (*SaveMetadata, error)
    LoadGame(ctx context.Context, userID int, saveID string) (*GameInstance, error)
    
    // Monitoring
    GetGameStatus(ctx context.Context, gameID string) (*GameStatus, error)
    ListActiveGames(ctx context.Context) ([]*GameInstance, error)
    
    // Resource Management
    GetGameMetrics(ctx context.Context, gameID string) (*GameMetrics, error)
    ScaleGame(ctx context.Context, gameID string, resources *ResourceSpec) error
}
```

### **Event-Driven Architecture**
```go
type GameEvent struct {
    ID        string    `json:"id"`
    Type      EventType `json:"type"`
    GameID    string    `json:"game_id"`
    UserID    int       `json:"user_id"`
    Data      any       `json:"data"`
    Timestamp time.Time `json:"timestamp"`
}

// Event Types
const (
    GameStarted    EventType = "game.started"
    GameStopped    EventType = "game.stopped"
    GameSaved      EventType = "game.saved"
    GameLoaded     EventType = "game.loaded"
    PlayerJoined   EventType = "player.joined"
    PlayerLeft     EventType = "player.left"
    ResourceAlert  EventType = "resource.alert"
)
```

### **Process Management Strategy**

#### **Option 1: Process-based (Recommended for Start)**
- Direct process management with PTY
- Fast startup, low overhead
- Easier debugging and development
- Resource isolation via cgroups

#### **Option 2: Container-based (Future)**
- Docker/Podman containers
- Better isolation and security
- Kubernetes orchestration
- Resource limits and monitoring

#### **Option 3: Hybrid Approach (Long-term)**
- Process-based for development
- Container-based for production
- Automatic scaling based on load

### **Data Architecture**

#### **Game Instance Storage**
```sql
-- PostgreSQL Schema
CREATE TABLE game_instances (
    id UUID PRIMARY KEY,
    user_id INTEGER NOT NULL,
    game_type VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL,
    process_id INTEGER,
    container_id VARCHAR(255),
    started_at TIMESTAMP NOT NULL,
    last_activity TIMESTAMP,
    resource_usage JSONB,
    metadata JSONB
);

CREATE TABLE game_saves (
    id UUID PRIMARY KEY,
    user_id INTEGER NOT NULL,
    game_type VARCHAR(50) NOT NULL,
    save_data BYTEA,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL,
    version INTEGER NOT NULL
);
```

#### **RabbitMQ for Event Streaming**
```go
// Topic exchanges for spectating
exchange: "game.frames"
routing_key: "nethack.game123"  // Route frames by game ID
routing_key: "dcss.game456"

// Queues for service coordination
queue: "game.lifecycle"    // Game start/stop events
queue: "game.saves"        // Save/load notifications
queue: "spectator.lobby"   // Spectator management
```

## 🚀 Migration Strategy

### **Phase 1: Extract and Refactor**
1. **Extract existing game code** from session service to games service
2. **Maintain backwards compatibility** during transition
3. **Add feature flags** for gradual rollout between old and new implementations

### **Phase 2: Service Integration**
1. **Replace mock clients** with real gRPC communication
2. **Migrate game operations** one by one (saves first, then launching, then lifecycle)
3. **Update PTY management** to work with remote game processes

### **Phase 3: Cleanup and Optimization**
1. **Remove extracted game code** from session service
2. **Clean up unused dependencies** and imports
3. **Optimize performance** with proper caching and connection pooling
4. **Add comprehensive testing** for the new architecture

## 📈 Success Metrics

### **Performance Goals**
- Game startup time: < 2 seconds
- Save operation: < 1 second
- API response time: < 100ms (p95)
- Concurrent games: 1000+ per node

### **Reliability Goals**
- Service uptime: 99.9%
- Data consistency: 99.99%
- Zero data loss on saves
- Graceful degradation on failures

### **Security Goals**
- All game streams encrypted
- Complete game isolation between users
- No privilege escalation possible
- Audit trail for all game operations

## 💡 Specific Architectural Recommendations

### **For Stream Encryption:**
```go
// High-performance encryption with minimal overhead
type ChaCha20StreamCipher struct {
    cipher cipher.AEAD
    nonce  []byte
}

// Encrypt frames in-place to reduce allocations
func (c *ChaCha20StreamCipher) EncryptFrame(frame *StreamFrame) error {
    // Encrypt frame.Data in-place
    // Add authentication tag
    // Update nonce counter
}
```

### **For Game Isolation:**
```go
// Linux namespaces + cgroups approach
type ProcessIsolation struct {
    UserNamespace bool // Isolate UIDs
    PIDNamespace  bool // Isolate process tree
    MountNamespace bool // Isolate filesystem
    NetworkNamespace bool // Isolate network
    CgroupLimits *CgroupConfig // Resource limits
}
```

### **For RabbitMQ Spectating:**
```go
// RabbitMQ-based spectating with topic exchanges
type RabbitMQGameEventBus struct {
    conn    *amqp.Connection
    channel *amqp.Channel
    logger  *log.Logger
}

func (bus *RabbitMQGameEventBus) PublishGameFrame(gameID string, frame []byte) error {
    return bus.channel.Publish("game.frames", gameID, false, false, amqp.Publishing{
        Body: frame,
    })
}

func (bus *RabbitMQGameEventBus) SubscribeToGame(gameID, spectatorID string) error {
    q, _ := bus.channel.QueueDeclare("spectator."+spectatorID, false, false, false, false, nil)
    return bus.channel.QueueBind(q.Name, gameID, "game.frames", false, nil)
}
```

This architecture provides a solid foundation for scaling DungeonGate while maintaining clean separation of concerns and supporting future growth.