# Game Service Architecture Roadmap

## 🎯 Game Service Architecture Vision

### **Core Philosophy: Scalable Stateful Game Backend**

**Architecture Pattern**: Stateful microservice that runs inside containers and scales independently
- **Domain**: Game process management, world state synchronization, user data
- **Deployment**: Multiple game service pods, each running multiple concurrent games
- **Scaling**: Horizontal scaling of game service pods based on load
- **State Sync**: Cross-pod synchronization for shared world state (bones files, levels)
- **Session Routing**: Session service connects to any available game service pod

### **Service Boundaries**
```
┌─────────────────┐    ┌─────────────────────────────────────┐
│  Session Service │────│         Game Service Cluster       │
│  (SSH/Terminal) │    │  ┌───────────┐ ┌───────────┐       │
└─────────────────┘    │  │Game Pod 1 │ │Game Pod 2 │ ...   │
         │              │  │- NetHack  │ │- DCSS     │       │
         │              │  │- DCSS     │ │- NetHack  │       │
         │              │  │- Saves    │ │- Saves    │       │
         │              │  └───────────┘ └───────────┘       │
         │              └─────────────────────────────────────┘
         │                       │
         │              ┌─────────────────┐
         └──────────────│  Auth Service   │
                        │ (Identity/Perms)│
                        └─────────────────┘
                                 │
                        ┌─────────────────┐
                        │ Shared Storage  │
                        │- User Saves     │
                        │- Bones Files    │ 
                        │- World State    │
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

### **✅ Phase 5: Stateful Game Backend (IN PROGRESS)**
16. **✅ Game cleanup after exit, save storage and loading**
    - ✅ Database schema for game sessions, saves, and events
    - ✅ Repository implementations for PostgreSQL persistence
    - ✅ Session cleanup service with orphaned process detection
    - ✅ Complete session manager with automatic save creation/loading
    - ✅ Save backup and archival system
    - ✅ Periodic cleanup of expired sessions and old saves
17. **📋 Game Configuration and Path Management (NEXT)**
    - ✅ NetHack path configuration from `--showpaths` output
    - Game-specific configuration directories and files
    - Save directory management and cleanup
    - Configuration validation and defaults
18. **📋 NetHack Bones File Management and Death Analytics**
    - Death information extraction from game output and save files
    - Bones file creation and population with death data
    - Shared bones directory management across server instances
    - Death analytics dashboard with player statistics
    - Bones file synchronization between pods
    - Death event streaming for real-time notifications
    - **Death Event Broadcasting**: Contextual haunting messages sent to all session services
      - "a haunting scream echoes from beyond the gate... something stirs in the shadows" (with bones)
      - "a distant cry fades into the void... all is quiet" (without bones)
      - Messages displayed in session service menu footer with auto-fade
      - gRPC broadcast system for real-time cross-service notifications
19. **📋 Pod-based deployment architecture**
    - Game service runs inside containers/pods
    - Horizontal scaling based on game load
    - Session service load balancing across pods
20. **Cross-pod world state synchronization**
    - NetHack bones files shared across all pods
    - Shared dungeon levels and world state
    - Event-driven synchronization for real-time updates
21. **User data management per pod**
    - Save files accessible from any pod
    - Session migration between pods
    - Distributed save state consistency
22. **Add RabbitMQ spectating integration**
23. **Add stream encryption** for security
24. **Object pooling optimization**

### **📋 Phase 6: PTY Tunneling Implementation (PLANNED)**
**Focus**: Enable PTY communication between session and game services over gRPC

25. **Implement PTY tunnel creation and management**
    - Create PTY tunnel request handling
    - Establish bidirectional gRPC streams for PTY data
    - Handle terminal resize events over tunnel
26. **Add PTY data batching and optimization**
    - Batch PTY operations to reduce gRPC calls
    - Implement efficient binary protocol for PTY data
    - Add connection pooling for persistent gRPC connections
27. **Implement session death handling**
    - Detect session disconnections and trigger auto-save
    - Handle graceful game shutdown on session loss
    - Ensure save file consistency during abrupt disconnections

### **📋 Phase 7: Service Discovery Integration (PLANNED)**
**Focus**: Integrate with Kubernetes service discovery for dynamic game availability

28. **Implement game availability reporting**
    - Add ListAvailableGames gRPC endpoint
    - Report current capacity per game per pod
    - Real-time availability updates via streaming
29. **Add Kubernetes service discovery integration**
    - Watch K8s service endpoints for pod health
    - Combine K8s health with game-level availability
    - Implement intelligent routing to optimal pods
30. **Implement capacity tracking and monitoring**
    - Track game slots usage per pod
    - Monitor CPU/memory usage for capacity decisions
    - Implement circuit breakers for unhealthy pods

### **📋 Phase 8: Game Availability Reporting (PLANNED)**
**Focus**: Real-time game menu updates and intelligent routing

31. **Implement real-time menu updates**
    - Stream capacity changes to session services
    - Update game menus dynamically as pods change
    - Handle race conditions in game selection
32. **Add intelligent game routing**
    - Route game requests to optimal pods based on load
    - Implement fallback handling for unavailable pods
    - Add connection migration support for pod failures
33. **Implement distributed game state synchronization**
    - Share NetHack bones files across pods
    - Synchronize game world state for consistency
    - Handle cross-pod game interactions

### **📋 Phase 9: Kubernetes Integration (PLANNED)**
**Focus**: Native Kubernetes deployment and scaling

34. **Implement Kubernetes-native deployment**
    - Create Helm charts for all services
    - Add Kubernetes operators for game management
    - Implement auto-scaling based on game demand
35. **Add service mesh integration**
    - Integrate with Istio/Linkerd for secure communication
    - Implement distributed tracing across services
    - Add circuit breakers and retry policies
36. **Implement multi-cluster support**
    - Deploy across multiple Kubernetes clusters
    - Add geographic distribution for game services
    - Implement cross-cluster service discovery

## 🎮 Game Configuration Management

### **NetHack Path Configuration**
Based on `nethack --showpaths` output, the game service needs to manage these configuration paths:

```go
type NetHackPaths struct {
    // Variable playground locations (customizable per user/game)
    HackDir    string `json:"hackdir"`    // User-specific game directory
    LevelDir   string `json:"leveldir"`   // Level save directory
    SaveDir    string `json:"savedir"`    // Save game directory
    BonesDir   string `json:"bonesdir"`   // Bones files directory
    DataDir    string `json:"datadir"`    // Game data directory
    LockDir    string `json:"lockdir"`    // Lock files directory
    ConfigDir  string `json:"configdir"`  // User config directory
    TroubleDir string `json:"troubledir"` // Debug/trouble directory
    
    // Fixed system paths (read-only)
    ScoreDir   string `json:"scoredir"`   // "/opt/homebrew/share/nethack/"
    SysConfDir string `json:"sysconfdir"` // "/opt/homebrew/Cellar/nethack/3.6.7/libexec/sysconf"
    SymbolsFile string `json:"symbols"`   // "/opt/homebrew/Cellar/nethack/3.6.7/libexec/symbols"
    DataFile   string `json:"datafile"`  // "nhdat" (basic data files)
    UserConfig string `json:"userconfig"` // "~/.nethackrc" (personal config)
}
```

### **Game Directory Management**
```go
type GameDirectoryManager struct {
    BaseDir    string // Base directory for all game data
    UserDirs   map[string]*UserGameDirs // Per-user directory structure
    TempDirs   map[string]string // Temporary directories for active games
    CleanupQueue []string // Directories pending cleanup
}

type UserGameDirs struct {
    UserID     int
    BaseDir    string // /data/users/{userID}
    SaveDir    string // /data/users/{userID}/saves
    ConfigDir  string // /data/users/{userID}/config
    BonesDir   string // /data/users/{userID}/bones (personal bones)
    LevelDir   string // /data/users/{userID}/levels (temp levels)
    LockDir    string // /data/users/{userID}/locks
    TroubleDir string // /data/users/{userID}/trouble
}
```

### **Setup and Clear Options**
```go
type GameSetupOptions struct {
    CreateUserDirs    bool `json:"create_user_dirs"`    // Create user-specific directories
    CopyDefaultConfig bool `json:"copy_default_config"` // Copy default .nethackrc
    InitializeShared  bool `json:"initialize_shared"`   // Setup shared bones/data
    ValidatePaths     bool `json:"validate_paths"`      // Validate all paths exist
    SetPermissions    bool `json:"set_permissions"`     // Set correct file permissions
    DetectSystemPaths bool `json:"detect_system_paths"` // Auto-detect system paths via --showpaths
    CreateSaveLinks   bool `json:"create_save_links"`   // Create save directory symlinks
}

type GameCleanupOptions struct {
    RemoveUserDirs     bool `json:"remove_user_dirs"`     // Delete user directories
    ClearTempFiles     bool `json:"clear_temp_files"`     // Clear temporary game files
    RemoveLockFiles    bool `json:"remove_lock_files"`    // Remove stale lock files
    ClearPersonalBones bool `json:"clear_personal_bones"` // Clear user's personal bones
    PreserveConfig     bool `json:"preserve_config"`      // Keep user config files
    BackupSaves        bool `json:"backup_saves"`         // Backup saves before cleanup
    CleanupSaveLinks   bool `json:"cleanup_save_links"`   // Remove save directory symlinks
    ValidateCleanup    bool `json:"validate_cleanup"`     // Verify cleanup completion
}
```

### **NetHack Path Detection**
```go
type NetHackSystemPaths struct {
    // Detected from `nethack --showpaths` command
    ScoreDir    string `json:"scoredir"`    // "/opt/homebrew/share/nethack/"
    SysConfFile string `json:"sysconf"`     // "/opt/homebrew/Cellar/nethack/3.6.7/libexec/sysconf"
    SymbolsFile string `json:"symbols"`     // "/opt/homebrew/Cellar/nethack/3.6.7/libexec/symbols"
    DataFile    string `json:"datafile"`    // "nhdat"
    UserConfig  string `json:"userconfig"`  // "/Users/caboose/.nethackrc"
    
    // Variable paths (customizable, typically "not set")
    HackDir     string `json:"hackdir"`     // User-specific game directory
    LevelDir    string `json:"leveldir"`    // Level save directory
    SaveDir     string `json:"savedir"`     // Save game directory
    BonesDir    string `json:"bonesdir"`    // Bones files directory
    DataDir     string `json:"datadir"`     // Game data directory
    LockDir     string `json:"lockdir"`     // Lock files directory
    ConfigDir   string `json:"configdir"`   // User config directory
    TroubleDir  string `json:"troubledir"`  // Debug/trouble directory
}

func (gm *GameDirectoryManager) DetectNetHackPaths() (*NetHackSystemPaths, error) {
    cmd := exec.Command("nethack", "--showpaths")
    output, err := cmd.CombinedOutput()
    if err != nil {
        return nil, fmt.Errorf("failed to run nethack --showpaths: %w", err)
    }
    
    paths := &NetHackSystemPaths{}
    lines := strings.Split(string(output), "\n")
    
    for _, line := range lines {
        line = strings.TrimSpace(line)
        
        // Parse variable playground locations
        if strings.Contains(line, "[") && strings.Contains(line, "]") {
            if strings.Contains(line, "hackdir") {
                paths.HackDir = parsePathValue(line)
            } else if strings.Contains(line, "leveldir") {
                paths.LevelDir = parsePathValue(line)
            } else if strings.Contains(line, "savedir") {
                paths.SaveDir = parsePathValue(line)
            } else if strings.Contains(line, "bonesdir") {
                paths.BonesDir = parsePathValue(line)
            } else if strings.Contains(line, "datadir") {
                paths.DataDir = parsePathValue(line)
            } else if strings.Contains(line, "scoredir") {
                paths.ScoreDir = parsePathValue(line)
            } else if strings.Contains(line, "lockdir") {
                paths.LockDir = parsePathValue(line)
            } else if strings.Contains(line, "configdir") {
                paths.ConfigDir = parsePathValue(line)
            } else if strings.Contains(line, "troubledir") {
                paths.TroubleDir = parsePathValue(line)
            }
        }
        
        // Parse fixed system paths
        if strings.Contains(line, "system configuration file") {
            if i := strings.Index(line, `"`); i != -1 {
                if j := strings.LastIndex(line, `"`); j > i {
                    paths.SysConfFile = line[i+1 : j]
                }
            }
        } else if strings.Contains(line, "loadable symbols file") {
            if i := strings.Index(line, `"`); i != -1 {
                if j := strings.LastIndex(line, `"`); j > i {
                    paths.SymbolsFile = line[i+1 : j]
                }
            }
        } else if strings.Contains(line, "Basic data files") {
            if i := strings.Index(line, `"`); i != -1 {
                if j := strings.LastIndex(line, `"`); j > i {
                    paths.DataFile = line[i+1 : j]
                }
            }
        } else if strings.Contains(line, "personal configuration file") {
            if i := strings.Index(line, `"`); i != -1 {
                if j := strings.LastIndex(line, `"`); j > i {
                    paths.UserConfig = line[i+1 : j]
                }
            }
        }
    }
    
    return paths, nil
}

func parsePathValue(line string) string {
    // Extract path from format: [pathtype]="value" or [pathtype]="not set"
    if i := strings.Index(line, `="`); i != -1 {
        if j := strings.LastIndex(line, `"`); j > i {
            value := line[i+2 : j]
            if value == "not set" {
                return ""
            }
            return value
        }
    }
    return ""
}
```

### **Configuration Validation**
```go
func (gm *GameDirectoryManager) ValidateNetHackPaths(paths *NetHackPaths) error {
    // Check system paths are readable
    if err := gm.validateReadablePath(paths.SysConfDir); err != nil {
        return fmt.Errorf("sysconfdir not readable: %w", err)
    }
    
    if err := gm.validateReadableFile(paths.SymbolsFile); err != nil {
        return fmt.Errorf("symbols file not readable: %w", err)
    }
    
    if err := gm.validateReadableFile(paths.DataFile); err != nil {
        return fmt.Errorf("data file not readable: %w", err)
    }
    
    // Check user paths are writable
    userPaths := []string{paths.SaveDir, paths.ConfigDir, paths.BonesDir, paths.LevelDir}
    for _, path := range userPaths {
        if path != "" {
            if err := gm.validateWritablePath(path); err != nil {
                return fmt.Errorf("user path %s not writable: %w", path, err)
            }
        }
    }
    
    return nil
}
```

### **Dynamic Path Setup**
```go
func (gm *GameDirectoryManager) SetupGamePaths(userID int, gameID string, options *GameSetupOptions) (*NetHackPaths, error) {
    userDirs := gm.GetUserDirs(userID)
    
    // Auto-detect system paths if requested
    var systemPaths *NetHackSystemPaths
    var err error
    if options.DetectSystemPaths {
        systemPaths, err = gm.DetectNetHackPaths()
        if err != nil {
            return nil, fmt.Errorf("failed to detect system paths: %w", err)
        }
    }
    
    // Create game-specific temporary directories
    tempDir := filepath.Join(userDirs.BaseDir, "temp", gameID)
    if err := os.MkdirAll(tempDir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create temp dir: %w", err)
    }
    
    paths := &NetHackPaths{
        // User-specific paths
        HackDir:    userDirs.BaseDir,
        SaveDir:    userDirs.SaveDir,
        ConfigDir:  userDirs.ConfigDir,
        BonesDir:   userDirs.BonesDir,
        LevelDir:   tempDir, // Game-specific temp level dir
        LockDir:    userDirs.LockDir,
        TroubleDir: userDirs.TroubleDir,
        
        // System paths (use detected paths if available, fallback to defaults)
        ScoreDir:    gm.getSystemPath(systemPaths, "scoredir", "/opt/homebrew/share/nethack/"),
        SysConfDir:  gm.getSystemPath(systemPaths, "sysconf", "/opt/homebrew/Cellar/nethack/3.6.7/libexec/sysconf"),
        SymbolsFile: gm.getSystemPath(systemPaths, "symbols", "/opt/homebrew/Cellar/nethack/3.6.7/libexec/symbols"),
        DataFile:    gm.getSystemPath(systemPaths, "datafile", "nhdat"),
        UserConfig:  gm.getSystemPath(systemPaths, "userconfig", filepath.Join(userDirs.ConfigDir, ".nethackrc")),
    }
    
    // Create save directory symlinks if requested
    if options.CreateSaveLinks {
        if err := gm.createSaveSymlinks(paths, userDirs); err != nil {
            return nil, fmt.Errorf("failed to create save symlinks: %w", err)
        }
    }
    
    // Register for cleanup
    gm.TempDirs[gameID] = tempDir
    
    return paths, nil
}

func (gm *GameDirectoryManager) getSystemPath(systemPaths *NetHackSystemPaths, pathType, defaultValue string) string {
    if systemPaths == nil {
        return defaultValue
    }
    
    switch pathType {
    case "scoredir":
        if systemPaths.ScoreDir != "" {
            return systemPaths.ScoreDir
        }
    case "sysconf":
        if systemPaths.SysConfFile != "" {
            return systemPaths.SysConfFile
        }
    case "symbols":
        if systemPaths.SymbolsFile != "" {
            return systemPaths.SymbolsFile
        }
    case "datafile":
        if systemPaths.DataFile != "" {
            return systemPaths.DataFile
        }
    case "userconfig":
        if systemPaths.UserConfig != "" {
            return systemPaths.UserConfig
        }
    }
    
    return defaultValue
}

func (gm *GameDirectoryManager) createSaveSymlinks(paths *NetHackPaths, userDirs *UserGameDirs) error {
    // Create symlink from system save directory to user save directory
    // This allows NetHack to find saves in the expected location
    systemSaveDir := filepath.Join(filepath.Dir(paths.SysConfDir), "save")
    userSaveLink := filepath.Join(systemSaveDir, fmt.Sprintf("user_%d", userDirs.UserID))
    
    // Remove existing symlink if it exists
    if _, err := os.Lstat(userSaveLink); err == nil {
        if err := os.Remove(userSaveLink); err != nil {
            return fmt.Errorf("failed to remove existing symlink: %w", err)
        }
    }
    
    // Create new symlink
    if err := os.Symlink(userDirs.SaveDir, userSaveLink); err != nil {
        return fmt.Errorf("failed to create save symlink: %w", err)
    }
    
    return nil
}
```

### **Game Cleanup Implementation**
```go
func (gm *GameDirectoryManager) CleanupGame(gameID string, options *GameCleanupOptions) error {
    // Remove temporary directories
    if tempDir, exists := gm.TempDirs[gameID]; exists {
        if options.ClearTempFiles {
            if err := os.RemoveAll(tempDir); err != nil {
                return fmt.Errorf("failed to remove temp dir: %w", err)
            }
        }
        delete(gm.TempDirs, gameID)
    }
    
    // Clear lock files
    if options.RemoveLockFiles {
        if err := gm.clearLockFiles(gameID); err != nil {
            return fmt.Errorf("failed to clear lock files: %w", err)
        }
    }
    
    // Backup saves if requested
    if options.BackupSaves {
        if err := gm.backupSaves(gameID); err != nil {
            return fmt.Errorf("failed to backup saves: %w", err)
        }
    }
    
    // Cleanup save symlinks
    if options.CleanupSaveLinks {
        if err := gm.cleanupSaveSymlinks(gameID); err != nil {
            return fmt.Errorf("failed to cleanup save symlinks: %w", err)
        }
    }
    
    // Validate cleanup completion
    if options.ValidateCleanup {
        if err := gm.validateCleanupCompletion(gameID); err != nil {
            return fmt.Errorf("cleanup validation failed: %w", err)
        }
    }
    
    return nil
}

func (gm *GameDirectoryManager) cleanupSaveSymlinks(gameID string) error {
    // Find and remove symlinks created for this game
    systemPaths, err := gm.DetectNetHackPaths()
    if err != nil {
        return fmt.Errorf("failed to detect system paths: %w", err)
    }
    
    if systemPaths.SysConfFile != "" {
        systemSaveDir := filepath.Join(filepath.Dir(systemPaths.SysConfFile), "save")
        
        // List all symlinks in the system save directory
        entries, err := os.ReadDir(systemSaveDir)
        if err != nil {
            return fmt.Errorf("failed to read system save directory: %w", err)
        }
        
        // Remove symlinks that match the game pattern
        for _, entry := range entries {
            if entry.Type()&os.ModeSymlink != 0 {
                linkPath := filepath.Join(systemSaveDir, entry.Name())
                if strings.Contains(entry.Name(), gameID) {
                    if err := os.Remove(linkPath); err != nil {
                        return fmt.Errorf("failed to remove symlink %s: %w", linkPath, err)
                    }
                }
            }
        }
    }
    
    return nil
}

func (gm *GameDirectoryManager) validateCleanupCompletion(gameID string) error {
    // Verify that all temporary files and directories are removed
    if _, exists := gm.TempDirs[gameID]; exists {
        return fmt.Errorf("temporary directory for game %s still exists", gameID)
    }
    
    // Check for remaining lock files
    if lockFiles, err := gm.findLockFiles(gameID); err != nil {
        return fmt.Errorf("failed to check for lock files: %w", err)
    } else if len(lockFiles) > 0 {
        return fmt.Errorf("found %d remaining lock files for game %s", len(lockFiles), gameID)
    }
    
    // Verify symlinks are removed
    if symlinks, err := gm.findRemainingSymlinks(gameID); err != nil {
        return fmt.Errorf("failed to check for remaining symlinks: %w", err)
    } else if len(symlinks) > 0 {
        return fmt.Errorf("found %d remaining symlinks for game %s", len(symlinks), gameID)
    }
    
    return nil
}

func (gm *GameDirectoryManager) findLockFiles(gameID string) ([]string, error) {
    // Implementation to find lock files associated with the game
    var lockFiles []string
    
    // Check common lock file locations
    lockDirs := []string{
        "/tmp",
        "/var/tmp",
        "/usr/games/lib/nethackdir",
    }
    
    for _, dir := range lockDirs {
        entries, err := os.ReadDir(dir)
        if err != nil {
            continue // Skip directories that can't be read
        }
        
        for _, entry := range entries {
            if strings.Contains(entry.Name(), gameID) && strings.Contains(entry.Name(), "lock") {
                lockFiles = append(lockFiles, filepath.Join(dir, entry.Name()))
            }
        }
    }
    
    return lockFiles, nil
}

func (gm *GameDirectoryManager) findRemainingSymlinks(gameID string) ([]string, error) {
    // Implementation to find remaining symlinks associated with the game
    var symlinks []string
    
    systemPaths, err := gm.DetectNetHackPaths()
    if err != nil {
        return symlinks, err
    }
    
    if systemPaths.SysConfFile != "" {
        systemSaveDir := filepath.Join(filepath.Dir(systemPaths.SysConfFile), "save")
        entries, err := os.ReadDir(systemSaveDir)
        if err != nil {
            return symlinks, err
        }
        
        for _, entry := range entries {
            if entry.Type()&os.ModeSymlink != 0 && strings.Contains(entry.Name(), gameID) {
                symlinks = append(symlinks, filepath.Join(systemSaveDir, entry.Name()))
            }
        }
    }
    
    return symlinks, nil
}
```

## 💀 NetHack Bones File Management and Death Analytics

### **Overview**
NetHack bones files contain information about player deaths and are left in the dungeon for other players to discover. The game service needs to extract death information from game output, populate server-side bones files, and provide death analytics.

### **Death Information Extraction**
```go
type DeathInfo struct {
    // Player Information
    PlayerName     string    `json:"player_name"`
    PlayerRole     string    `json:"player_role"`      // Valkyrie, Wizard, etc.
    PlayerRace     string    `json:"player_race"`      // Human, Elf, etc.
    PlayerGender   string    `json:"player_gender"`    // Male, Female
    PlayerAlign    string    `json:"player_align"`     // Lawful, Neutral, Chaotic
    
    // Death Details
    DeathCause     string    `json:"death_cause"`      // "killed by a grid bug"
    DeathLevel     int       `json:"death_level"`      // Dungeon level where death occurred
    DeathBranch    string    `json:"death_branch"`     // "Dungeons of Doom", "Gehennom", etc.
    DeathLocation  string    `json:"death_location"`   // "on level 3 of the Dungeons of Doom"
    DeathTurn      int64     `json:"death_turn"`       // Game turn when death occurred
    
    // Character Stats at Death
    ExperienceLevel int      `json:"experience_level"` // Character level
    HitPoints      int       `json:"hit_points"`       // HP at death
    MaxHitPoints   int       `json:"max_hit_points"`   // Max HP
    Score          int64     `json:"score"`            // Final score
    Gold           int64     `json:"gold"`             // Gold pieces
    
    // Game State
    GameDuration   time.Duration `json:"game_duration"`   // Time played
    ServerTime     time.Time     `json:"server_time"`     // When death occurred on server
    SessionID      string        `json:"session_id"`      // Associated game session
    UserID         int           `json:"user_id"`         // Player user ID
    
    // Inventory and Equipment (simplified)
    Inventory      []string      `json:"inventory"`       // List of items carried
    Equipment      map[string]string `json:"equipment"`   // Worn/wielded items
    
    // Bones File Information
    BonesFileName  string        `json:"bones_file_name"` // Generated bones file name
    BonesChecksum  string        `json:"bones_checksum"`  // Integrity verification
}
```

### **Death Information Sources**

#### **1. Game Output Parsing**
```go
type NetHackOutputParser struct {
    logger *log.Logger
}

func (p *NetHackOutputParser) ParseDeathFromOutput(output []byte) (*DeathInfo, error) {
    lines := strings.Split(string(output), "\n")
    
    var deathInfo DeathInfo
    var inDeathScreen bool
    
    for _, line := range lines {
        line = strings.TrimSpace(line)
        
        // Detect death screen start
        if strings.Contains(line, "You die...") || strings.Contains(line, "Do you want your possessions identified?") {
            inDeathScreen = true
            continue
        }
        
        if inDeathScreen {
            // Parse death cause
            if strings.Contains(line, "killed by") {
                deathInfo.DeathCause = p.extractDeathCause(line)
            }
            
            // Parse final stats
            if strings.Contains(line, "You were level") {
                deathInfo.ExperienceLevel = p.extractLevel(line)
            }
            
            if strings.Contains(line, "You had") && strings.Contains(line, "hit points") {
                deathInfo.HitPoints, deathInfo.MaxHitPoints = p.extractHitPoints(line)
            }
            
            if strings.Contains(line, "You had") && strings.Contains(line, "gold pieces") {
                deathInfo.Gold = p.extractGold(line)
            }
        }
        
        // Parse location information
        if strings.Contains(line, "Dlvl:") {
            deathInfo.DeathLevel = p.extractDungeonLevel(line)
        }
        
        if strings.Contains(line, "$:") {
            deathInfo.Gold = p.extractGoldFromStatus(line)
        }
    }
    
    return &deathInfo, nil
}

func (p *NetHackOutputParser) extractDeathCause(line string) string {
    // Extract death cause from lines like "You were killed by a grid bug."
    if idx := strings.Index(line, "killed by "); idx != -1 {
        cause := line[idx+10:]
        if idx := strings.Index(cause, "."); idx != -1 {
            cause = cause[:idx]
        }
        return strings.TrimSpace(cause)
    }
    return "unknown cause"
}

func (p *NetHackOutputParser) extractLevel(line string) int {
    // Extract from "You were level 3 with a maximum of 31 hit points when you died."
    re := regexp.MustCompile(`level (\d+)`)
    matches := re.FindStringSubmatch(line)
    if len(matches) > 1 {
        if level, err := strconv.Atoi(matches[1]); err == nil {
            return level
        }
    }
    return 0
}
```

#### **2. Save File Analysis**
```go
type NetHackSaveParser struct {
    logger *log.Logger
}

func (p *NetHackSaveParser) ExtractDeathInfoFromSave(saveFilePath string) (*DeathInfo, error) {
    // NetHack save files are binary format
    // This would require parsing the specific NetHack save format
    // For now, we'll focus on what we can extract from game output
    
    saveData, err := os.ReadFile(saveFilePath)
    if err != nil {
        return nil, fmt.Errorf("failed to read save file: %w", err)
    }
    
    // Parse character info from save header
    var deathInfo DeathInfo
    
    // Extract player name (typically at a fixed offset in save file)
    if len(saveData) > 32 {
        playerName := p.extractNullTerminatedString(saveData, 8) // Example offset
        deathInfo.PlayerName = playerName
    }
    
    // Extract other stats from known offsets
    // This would require understanding NetHack's save file format
    
    return &deathInfo, nil
}

func (p *NetHackSaveParser) extractNullTerminatedString(data []byte, offset int) string {
    if offset >= len(data) {
        return ""
    }
    
    end := offset
    for end < len(data) && data[end] != 0 {
        end++
    }
    
    return string(data[offset:end])
}
```

### **Bones File Creation and Management**
```go
type BonesFileManager struct {
    bonesDir        string
    sharedBonesDir  string
    eventPublisher  EventPublisher
    logger          *log.Logger
}

func NewBonesFileManager(bonesDir, sharedBonesDir string, eventPublisher EventPublisher, logger *log.Logger) *BonesFileManager {
    return &BonesFileManager{
        bonesDir:       bonesDir,
        sharedBonesDir: sharedBonesDir,
        eventPublisher: eventPublisher,
        logger:         logger,
    }
}

func (bm *BonesFileManager) CreateBonesFile(deathInfo *DeathInfo, originalBonesData []byte) error {
    // Generate bones file name based on level and branch
    bonesFileName := fmt.Sprintf("bon%c%02d.gz", 
        bm.getBranchChar(deathInfo.DeathBranch), 
        deathInfo.DeathLevel)
    
    // Create enhanced bones data with server information
    enhancedBones := &EnhancedBonesData{
        OriginalBones: originalBonesData,
        DeathInfo:     deathInfo,
        ServerInfo: ServerBonesInfo{
            ServerName:    "DungeonGate",
            CreatedAt:     time.Now(),
            DeathID:       uuid.New().String(),
            PlayerCount:   bm.getServerPlayerCount(),
            Verified:      true,
        },
    }
    
    // Serialize enhanced bones data
    bonesData, err := bm.serializeEnhancedBones(enhancedBones)
    if err != nil {
        return fmt.Errorf("failed to serialize bones data: %w", err)
    }
    
    // Write to local bones directory
    localBonesPath := filepath.Join(bm.bonesDir, bonesFileName)
    err = bm.writeCompressedBonesFile(localBonesPath, bonesData)
    if err != nil {
        return fmt.Errorf("failed to write local bones file: %w", err)
    }
    
    // Copy to shared bones directory for other pods
    sharedBonesPath := filepath.Join(bm.sharedBonesDir, bonesFileName)
    err = bm.writeCompressedBonesFile(sharedBonesPath, bonesData)
    if err != nil {
        bm.logger.Printf("Failed to write shared bones file: %v", err)
        // Don't fail the operation if shared write fails
    }
    
    // Update death info with bones file information
    deathInfo.BonesFileName = bonesFileName
    deathInfo.BonesChecksum = bm.calculateChecksum(bonesData)
    
    // Publish death event for analytics
    event := &DeathEvent{
        Type:      "player.death",
        DeathInfo: deathInfo,
        Timestamp: time.Now(),
    }
    
    err = bm.eventPublisher.PublishDeathEvent(event)
    if err != nil {
        bm.logger.Printf("Failed to publish death event: %v", err)
    }
    
    bm.logger.Printf("Created bones file %s for player %s (level %d)", 
        bonesFileName, deathInfo.PlayerName, deathInfo.ExperienceLevel)
    
    return nil
}

type EnhancedBonesData struct {
    OriginalBones []byte           `json:"original_bones"`
    DeathInfo     *DeathInfo       `json:"death_info"`
    ServerInfo    ServerBonesInfo  `json:"server_info"`
}

type ServerBonesInfo struct {
    ServerName    string    `json:"server_name"`
    CreatedAt     time.Time `json:"created_at"`
    DeathID       string    `json:"death_id"`
    PlayerCount   int       `json:"player_count"`
    Verified      bool      `json:"verified"`
}

func (bm *BonesFileManager) getBranchChar(branchName string) byte {
    switch branchName {
    case "Dungeons of Doom":
        return 'd'
    case "Gehennom":
        return 'g'
    case "The Gnomish Mines":
        return 'm'
    case "Sokoban":
        return 's'
    case "The Quest":
        return 'q'
    case "Ludios":
        return 'l'
    case "Vlad's Tower":
        return 't'
    default:
        return 'd' // Default to Dungeons of Doom
    }
}
```

### **Death Analytics and Dashboard**
```go
type DeathAnalyticsService struct {
    deathRepo  DeathRepository
    cacheRepo  CacheRepository
    logger     *log.Logger
}

type DeathStatistics struct {
    TotalDeaths           int64                    `json:"total_deaths"`
    DeathsByLevel         map[int]int64            `json:"deaths_by_level"`
    DeathsByCause         map[string]int64         `json:"deaths_by_cause"`
    DeathsByRole          map[string]int64         `json:"deaths_by_role"`
    DeathsByRace          map[string]int64         `json:"deaths_by_race"`
    AverageLevel          float64                  `json:"average_level"`
    AverageGameDuration   time.Duration            `json:"average_game_duration"`
    TopKillers            []KillerStats            `json:"top_killers"`
    RecentDeaths          []*DeathInfo             `json:"recent_deaths"`
    DeathsPerDay          map[string]int64         `json:"deaths_per_day"`
    MostDangerousLevels   []LevelDangerStats       `json:"most_dangerous_levels"`
}

type KillerStats struct {
    Killer      string  `json:"killer"`
    DeathCount  int64   `json:"death_count"`
    Percentage  float64 `json:"percentage"`
}

type LevelDangerStats struct {
    Level       int     `json:"level"`
    Branch      string  `json:"branch"`
    DeathCount  int64   `json:"death_count"`
    DangerRatio float64 `json:"danger_ratio"` // deaths per player visits
}

func (das *DeathAnalyticsService) GetDeathStatistics(ctx context.Context, timeRange TimeRange) (*DeathStatistics, error) {
    // Check cache first
    cacheKey := fmt.Sprintf("death_stats_%s_%s", timeRange.Start.Format("2006-01-02"), timeRange.End.Format("2006-01-02"))
    if cached, err := das.cacheRepo.Get(ctx, cacheKey); err == nil {
        var stats DeathStatistics
        if err := json.Unmarshal(cached, &stats); err == nil {
            return &stats, nil
        }
    }
    
    // Query death data
    deaths, err := das.deathRepo.FindDeathsInRange(ctx, timeRange)
    if err != nil {
        return nil, fmt.Errorf("failed to query deaths: %w", err)
    }
    
    stats := &DeathStatistics{
        TotalDeaths:         int64(len(deaths)),
        DeathsByLevel:       make(map[int]int64),
        DeathsByCause:       make(map[string]int64),
        DeathsByRole:        make(map[string]int64),
        DeathsByRace:        make(map[string]int64),
        DeathsPerDay:        make(map[string]int64),
    }
    
    // Calculate statistics
    var totalLevel int64
    var totalDuration time.Duration
    
    for _, death := range deaths {
        // Level statistics
        stats.DeathsByLevel[death.DeathLevel]++
        totalLevel += int64(death.ExperienceLevel)
        
        // Cause statistics
        stats.DeathsByCause[death.DeathCause]++
        
        // Role/Race statistics
        stats.DeathsByRole[death.PlayerRole]++
        stats.DeathsByRace[death.PlayerRace]++
        
        // Duration statistics
        totalDuration += death.GameDuration
        
        // Daily statistics
        day := death.ServerTime.Format("2006-01-02")
        stats.DeathsPerDay[day]++
    }
    
    // Calculate averages
    if stats.TotalDeaths > 0 {
        stats.AverageLevel = float64(totalLevel) / float64(stats.TotalDeaths)
        stats.AverageGameDuration = totalDuration / time.Duration(stats.TotalDeaths)
    }
    
    // Get top killers
    stats.TopKillers = das.calculateTopKillers(stats.DeathsByCause, stats.TotalDeaths)
    
    // Get recent deaths (last 10)
    recentDeaths, err := das.deathRepo.FindRecentDeaths(ctx, 10)
    if err == nil {
        stats.RecentDeaths = recentDeaths
    }
    
    // Cache results for 1 hour
    if statsData, err := json.Marshal(stats); err == nil {
        das.cacheRepo.Set(ctx, cacheKey, statsData, time.Hour)
    }
    
    return stats, nil
}
```

### **Death Event Broadcasting System**
```go
type DeathEventBroadcaster struct {
    sessionServices []SessionServiceClient
    messageBroker   MessageBroker
    bonesDetector   BonesFileDetector
    logger          *log.Logger
}

type DeathBroadcastMessage struct {
    Type        string    `json:"type"`        // "death.event"
    Message     string    `json:"message"`     // Contextual message based on bones
    PlayerName  string    `json:"player_name"`
    DeathLevel  int       `json:"death_level"`
    DeathCause  string    `json:"death_cause"`
    HasBones    bool      `json:"has_bones"`   // Whether bones file was created
    Timestamp   time.Time `json:"timestamp"`
}

func (deb *DeathEventBroadcaster) BroadcastDeath(deathInfo *DeathInfo) error {
    // Check if bones file was created for this death
    hasBones := deb.bonesDetector.CheckBonesCreated(deathInfo)
    
    // Select appropriate message based on bones status
    message := deb.selectDeathMessage(hasBones)
    
    broadcastMsg := &DeathBroadcastMessage{
        Type:       "death.event",
        Message:    message,
        PlayerName: deathInfo.PlayerName,
        DeathLevel: deathInfo.DeathLevel,
        DeathCause: deathInfo.DeathCause,
        HasBones:   hasBones,
        Timestamp:  time.Now(),
    }
    
    // Broadcast to all session services via gRPC
    return deb.broadcastToSessionServices(broadcastMsg)
}

func (deb *DeathEventBroadcaster) selectDeathMessage(hasBones bool) string {
    if hasBones {
        return "a haunting scream echoes from beyond the gate... something stirs in the shadows"
    }
    return "a distant cry fades into the void... all is quiet"
}
```

### **Session Service Integration**
```go
// Session service receives death broadcasts and displays in menu footer
type MenuFooterManager struct {
    currentMessage string
    fadeTimer      *time.Timer
    messageMutex   sync.RWMutex
}

func (mfm *MenuFooterManager) OnDeathBroadcast(msg *DeathBroadcastMessage) {
    mfm.messageMutex.Lock()
    defer mfm.messageMutex.Unlock()
    
    // Update footer message
    mfm.currentMessage = msg.Message
    
    // Set fade timer for 5 seconds
    if mfm.fadeTimer != nil {
        mfm.fadeTimer.Stop()
    }
    mfm.fadeTimer = time.AfterFunc(5*time.Second, func() {
        mfm.messageMutex.Lock()
        mfm.currentMessage = ""
        mfm.messageMutex.Unlock()
    })
}
```

### **Event Streaming and Notifications**
```go
type DeathEventStreamer struct {
    messageBroker MessageBroker
    subscribers   map[string][]DeathEventSubscriber
    mutex         sync.RWMutex
    logger        *log.Logger
}

type DeathEvent struct {
    Type      string     `json:"type"`
    DeathInfo *DeathInfo `json:"death_info"`
    Timestamp time.Time  `json:"timestamp"`
}

func (des *DeathEventStreamer) PublishDeathEvent(event *DeathEvent) error {
    // Publish to message broker for persistence and cross-pod communication
    eventData, err := json.Marshal(event)
    if err != nil {
        return fmt.Errorf("failed to marshal death event: %w", err)
    }
    
    err = des.messageBroker.Publish("death.events", eventData)
    if err != nil {
        return fmt.Errorf("failed to publish death event: %w", err)
    }
    
    // Notify local subscribers immediately
    des.notifyLocalSubscribers(event)
    
    des.logger.Printf("Published death event for player %s on level %d", 
        event.DeathInfo.PlayerName, event.DeathInfo.DeathLevel)
    
    return nil
}

func (des *DeathEventStreamer) SubscribeToDeaths(subscriberID string, subscriber DeathEventSubscriber) {
    des.mutex.Lock()
    defer des.mutex.Unlock()
    
    if des.subscribers == nil {
        des.subscribers = make(map[string][]DeathEventSubscriber)
    }
    
    des.subscribers[subscriberID] = append(des.subscribers[subscriberID], subscriber)
}

type DeathEventSubscriber interface {
    OnDeath(event *DeathEvent) error
}

// Example subscriber for real-time notifications
type DiscordNotifier struct {
    webhookURL string
}

func (dn *DiscordNotifier) OnDeath(event *DeathEvent) error {
    message := fmt.Sprintf("💀 **%s** the %s %s died on level %d of %s\n**Cause:** %s\n**Level:** %d\n**Score:** %d", 
        event.DeathInfo.PlayerName,
        event.DeathInfo.PlayerRace,
        event.DeathInfo.PlayerRole,
        event.DeathInfo.DeathLevel,
        event.DeathInfo.DeathBranch,
        event.DeathInfo.DeathCause,
        event.DeathInfo.ExperienceLevel,
        event.DeathInfo.Score)
    
    // Send Discord webhook notification
    return dn.sendWebhook(message)
}
```

### **Bones File Synchronization**
```go
type BonesFileSynchronizer struct {
    localBonesDir  string
    sharedBonesDir string
    syncInterval   time.Duration
    eventBus       EventBus
    logger         *log.Logger
}

func (bfs *BonesFileSynchronizer) StartSynchronization(ctx context.Context) {
    ticker := time.NewTicker(bfs.syncInterval)
    defer ticker.Stop()
    
    bfs.logger.Printf("Starting bones file synchronization every %v", bfs.syncInterval)
    
    for {
        select {
        case <-ctx.Done():
            bfs.logger.Println("Stopping bones file synchronization")
            return
        case <-ticker.C:
            err := bfs.synchronizeBones(ctx)
            if err != nil {
                bfs.logger.Printf("Error during bones synchronization: %v", err)
            }
        }
    }
}

func (bfs *BonesFileSynchronizer) synchronizeBones(ctx context.Context) error {
    // Find new bones files in shared directory
    sharedFiles, err := bfs.listBonesFiles(bfs.sharedBonesDir)
    if err != nil {
        return fmt.Errorf("failed to list shared bones files: %w", err)
    }
    
    localFiles, err := bfs.listBonesFiles(bfs.localBonesDir)
    if err != nil {
        return fmt.Errorf("failed to list local bones files: %w", err)
    }
    
    localFileSet := make(map[string]os.FileInfo)
    for _, file := range localFiles {
        localFileSet[file.Name()] = file
    }
    
    syncedCount := 0
    for _, sharedFile := range sharedFiles {
        localFile, exists := localFileSet[sharedFile.Name()]
        
        // Copy if file doesn't exist locally or shared file is newer
        if !exists || sharedFile.ModTime().After(localFile.ModTime()) {
            err := bfs.copyBonesFile(
                filepath.Join(bfs.sharedBonesDir, sharedFile.Name()),
                filepath.Join(bfs.localBonesDir, sharedFile.Name()))
            if err != nil {
                bfs.logger.Printf("Failed to copy bones file %s: %v", sharedFile.Name(), err)
                continue
            }
            syncedCount++
        }
    }
    
    if syncedCount > 0 {
        bfs.logger.Printf("Synchronized %d bones files from shared storage", syncedCount)
    }
    
    return nil
}
```

This implementation provides comprehensive NetHack bones file management with death information extraction, server-side bones file population, analytics, and real-time event streaming.

### **🎯 Current Status: Ready for Phase 5 - Advanced Features**
The microservices architecture is now complete! Both session and game services are fully integrated:
- **Session Service**: Handles SSH/terminal management and user authentication
- **Game Service**: Handles game lifecycle, process management, and save operations
- **gRPC Communication**: Real protobuf-based service-to-service communication established
- **Service Integration**: Session service successfully calls game service for all game-related operations

The next phase focuses on game configuration management, path setup/cleanup, and advanced features like RabbitMQ spectating.

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
    
    // Service Discovery
    ListAvailableGames(ctx context.Context) (*GameAvailability, error)
    GetPodCapacity(ctx context.Context) (*PodCapacity, error)
    SubscribeToCapacityUpdates(ctx context.Context) (CapacityUpdateStream, error)
    
    // PTY Communication
    CreatePTYTunnel(ctx context.Context, req *PTYTunnelRequest) (PTYTunnelStream, error)
    HandlePTYData(ctx context.Context, stream PTYDataStream) error
    
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
    GameStarted         EventType = "game.started"
    GameStopped         EventType = "game.stopped"
    GameSaved           EventType = "game.saved"
    GameLoaded          EventType = "game.loaded"
    PlayerJoined        EventType = "player.joined"
    PlayerLeft          EventType = "player.left"
    ResourceAlert       EventType = "resource.alert"
    PodCapacityUpdated  EventType = "pod.capacity.updated"
    GameAvailabilityChanged EventType = "game.availability.changed"
    PTYTunnelCreated    EventType = "pty.tunnel.created"
    PTYTunnelClosed     EventType = "pty.tunnel.closed"
)

// Service Discovery Data Structures
type GameAvailability struct {
    PodID    string                 `json:"pod_id"`
    Games    map[string]*GameSlots  `json:"games"`
    Updated  time.Time              `json:"updated"`
}

type GameSlots struct {
    GameName        string `json:"game_name"`
    CurrentPlayers  int    `json:"current_players"`
    MaxPlayers      int    `json:"max_players"`
    AcceptingConns  bool   `json:"accepting_connections"`
}

type PodCapacity struct {
    PodID           string `json:"pod_id"`
    TotalSlots      int    `json:"total_slots"`
    UsedSlots       int    `json:"used_slots"`
    AvailableSlots  int    `json:"available_slots"`
    CPUUsage        float64 `json:"cpu_usage"`
    MemoryUsage     float64 `json:"memory_usage"`
}

// PTY Tunneling Data Structures
type PTYTunnelRequest struct {
    SessionID   string `json:"session_id"`
    GameName    string `json:"game_name"`
    UserID      int    `json:"user_id"`
    TerminalCols int   `json:"terminal_cols"`
    TerminalRows int   `json:"terminal_rows"`
}

type PTYData struct {
    SessionID   string    `json:"session_id"`
    Data        []byte    `json:"data"`
    Direction   string    `json:"direction"` // "input" or "output"
    Timestamp   time.Time `json:"timestamp"`
}
```

### **Deployment Strategy**

#### **Game Service Pod Architecture**
- **Game Service runs inside containers/pods** (not managing containers)
- **Multiple games per pod**: Each pod can run multiple concurrent game processes
- **Horizontal scaling**: Scale pods based on game load and resource usage
- **Load balancing**: Session service distributes game requests across available pods

#### **World State Synchronization**
- **Shared storage backend**: NetHack bones files, save data, shared world state
- **Cross-pod consistency**: Real-time synchronization of world changes
- **Event-driven updates**: Game events broadcast to all pods for state consistency
- **Conflict resolution**: Handle concurrent updates to shared world state

#### **Session Routing**
- **Pod discovery**: Session service maintains registry of available game pods
- **Health checks**: Monitor pod health and game capacity
- **Game placement**: Route new games to least-loaded available pods
- **Session migration**: Support moving active sessions between pods (future)

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