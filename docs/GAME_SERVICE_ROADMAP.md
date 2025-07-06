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

## 🚨 Critical Issues to Address

### **Current Session Service Problems**
The session service contains **~1,000+ lines** of game-related code that violates microservices separation:

1. **Session service bypasses games service entirely** - implements mock game client instead of real gRPC
2. **Save management in wrong service** - 248 lines of game save logic in session service
3. **Game launching mixed with session logic** - game lifecycle management in SSH handlers
4. **Duplicate type definitions** - game types defined in both session and games services
5. **Mock implementations** - fake game service responses instead of actual service integration

### **Files Requiring Refactoring**
```
internal/session/
├── save_manager.go      # 248 lines → move to internal/games/saves/
├── game_client.go       # 379 lines → move to internal/games/client/
├── ssh.go              # ~200 lines of game logic → move to internal/games/
├── service_clients.go   # Game client logic → move to internal/games/
└── types.go            # Game types → move to internal/games/types.go
```

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

### **Phase 1: Session Service Refactoring (Critical)**
**Focus: Extract game logic from session service**

1. **Extract save management** - Move `internal/session/save_manager.go` to `internal/games/saves/`
2. **Extract game client** - Move `internal/session/game_client.go` to `internal/games/client/`
3. **Extract game types** - Move game-related types from `internal/session/types.go` to `internal/games/types.go`
4. **Remove mock implementations** - Replace fake game service client with real gRPC calls

### **Phase 2: Games Service Implementation**
**Focus: internal/games package structure with DDD**

5. **Create internal/games package structure** with Domain-Driven Design
6. **Implement core domain models** (Game, Session, Save)
7. **Set up repository interfaces** and basic implementations
8. **Create application service layer** for use cases

### **Phase 3: Service Integration**
**Focus: Connect session service to games service**

9. **Implement gRPC communication** - Session service calls games service
10. **Refactor game launching** - Move game process management to games service
11. **Update PTY management** - Keep PTY allocation in session, move process management to games
12. **Add RabbitMQ spectating integration**

### **Phase 4: Advanced Features**
13. **Add stream encryption** for security
14. **Implement game isolation** (critical for multi-user)
15. **Shared game state (bones) implementation**
16. **Object pooling optimization**

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