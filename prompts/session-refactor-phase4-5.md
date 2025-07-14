# Session Service Refactor - Phase 4.5: Pool-Based Menu Navigation Engineering

## Context

**Phase 4 COMPLETED**: Pool-based architecture foundation established and functional
**Phase 5 COMPLETED**: Critical bugs fixed, SSH connections working, basic infrastructure validated

**Current Status**: Pool implementation can accept SSH connections but lacks proper menu navigation functionality. The legacy menu system is well-designed and needs to be properly integrated into the pool-based architecture.

## 🎯 Phase 4.5 Objectives

**MENU NAVIGATION ENGINEERING** - Implement comprehensive menu system for pool-based architecture by leveraging proven legacy patterns:

1. **Pool-Based Menu Handler** - Create worker-compatible menu processing
2. **Terminal Integration** - Implement robust input/output handling in pool context
3. **State Management** - Properly handle user authentication states across workers
4. **Menu Flow Control** - Implement complete menu hierarchy and navigation
5. **Error Recovery** - Graceful handling of service unavailability and user errors

---

## Phase 4.5 Tasks

### Task 1: Pool-Based Menu Handler Design

#### 1.1 Analyze Legacy Menu Patterns
**Reference Implementation:** `internal/session/menu/menu.go`

**Legacy Menu Flow:**
```
Connection → getUserInfo() → ShowMenu() → handleMenuChoice() → Action
```

**Key Legacy Components:**
- `MenuHandler.ShowAnonymousMenu()` - Handles non-authenticated users
- `MenuHandler.ShowUserMenu()` - Handles authenticated users  
- `MenuHandler.ShowGameSelectionMenu()` - Game selection interface
- `BannerManager` - Template-based banner rendering
- Terminal input/output handling with ANSI sequences

#### 1.2 Design Pool-Compatible Menu Architecture
**New Architecture Pattern:**
```go
// Pool-based menu request/response pattern
type MenuRequest struct {
    ConnectionID string
    UserInfo     *UserInfo
    MenuType     MenuType
    InputData    *InputData
}

type MenuResponse struct {
    MenuChoice   *MenuChoice
    DisplayData  []byte
    NextState    ConnectionState
    Error        error
}

// Worker-compatible menu handler
type PoolMenuHandler struct {
    bannerManager *banner.Manager
    gameClient    *client.GameClient
    authClient    *client.AuthClient
    logger        *slog.Logger
}

func (pmh *PoolMenuHandler) ProcessMenuRequest(ctx context.Context, req *MenuRequest) (*MenuResponse, error)
```

**Requirements:**
- **Stateless Design**: Each menu interaction is self-contained
- **Worker Pool Compatible**: Menu processing can be distributed across workers
- **Resource Efficient**: Minimal memory allocation per request
- **Context Aware**: Proper context propagation for cancellation

#### 1.3 Implement Core Menu Handler
**File:** `internal/session/handlers/pool_menu_handler.go`

**Essential Methods:**
```go
// Core menu processing
func (pmh *PoolMenuHandler) ShowAnonymousMenu(ctx context.Context, conn *pools.Connection) (*MenuChoice, error)
func (pmh *PoolMenuHandler) ShowUserMenu(ctx context.Context, conn *pools.Connection, userInfo *UserInfo) (*MenuChoice, error)  
func (pmh *PoolMenuHandler) ShowGameSelectionMenu(ctx context.Context, conn *pools.Connection, games []*GameInfo) (*MenuChoice, error)

// Input processing
func (pmh *PoolMenuHandler) ProcessUserInput(ctx context.Context, conn *pools.Connection, input []byte) (*InputEvent, error)
func (pmh *PoolMenuHandler) RenderMenuDisplay(ctx context.Context, menuType MenuType, data interface{}) ([]byte, error)

// State management
func (pmh *PoolMenuHandler) GetUserInfo(ctx context.Context, conn *pools.Connection) (*UserInfo, error)
func (pmh *PoolMenuHandler) UpdateConnectionState(ctx context.Context, conn *pools.Connection, state ConnectionState) error
```

### Task 2: Terminal Integration for Pool Architecture

#### 2.1 Pool-Compatible Terminal I/O
**Reference:** `internal/session/terminal/input.go`

**Legacy Terminal Flow:**
```
SSH Channel → InputHandler → InputEvent → Menu Processing → Output
```

**Pool-Based Terminal Handler:**
```go
type PoolTerminalHandler struct {
    inputBuffer  []byte
    outputBuffer []byte
    logger       *slog.Logger
}

func (pth *PoolTerminalHandler) ReadInput(ctx context.Context, channel ssh.Channel) (*InputEvent, error)
func (pth *PoolTerminalHandler) WriteOutput(ctx context.Context, channel ssh.Channel, data []byte) error
func (pth *PoolTerminalHandler) ClearScreen(ctx context.Context, channel ssh.Channel) error
func (pth *PoolTerminalHandler) ProcessControlSequences(input []byte) (*InputEvent, error)
```

**Key Features:**
- **Non-blocking I/O**: Use context-aware operations
- **Buffer Management**: Efficient memory usage for input/output buffers
- **ANSI Support**: Proper handling of terminal control sequences
- **Error Recovery**: Graceful handling of terminal disconnections

#### 2.2 Input Event Processing
**Input Event Types:**
- **Character Input**: Regular keystrokes for menu navigation
- **Control Keys**: Enter, Escape, Ctrl+C handling
- **Line Editing**: Backspace, cursor movement, line completion
- **Terminal Resizing**: Window size change notifications

**Implementation Pattern:**
```go
type InputEvent struct {
    Type     InputEventType
    Data     []byte
    Key      KeyCode
    Sequence string
}

func (pth *PoolTerminalHandler) ParseInputEvent(data []byte) (*InputEvent, error) {
    // Parse raw bytes into structured input events
    // Handle escape sequences and control characters
    // Return structured event for menu processing
}
```

### Task 3: User Authentication State Management

#### 3.1 Pool-Aware User Context
**Legacy Pattern:** JWT tokens stored in SSH connection permissions

**Pool-Based Pattern:**
```go
type UserContext struct {
    UserID       string
    Username     string
    Token        string
    Permissions  []string
    LastActivity time.Time
    State        UserState
}

func (pmh *PoolMenuHandler) ExtractUserContext(conn *pools.Connection) (*UserContext, error)
func (pmh *PoolMenuHandler) ValidateUserState(ctx context.Context, userCtx *UserContext) error
func (pmh *PoolMenuHandler) RefreshUserInfo(ctx context.Context, userCtx *UserContext) (*UserInfo, error)
```

#### 3.2 Authentication Flow Integration
**Anonymous User Flow:**
1. Connection established → Show anonymous menu
2. User selects "login" → Process authentication
3. Success → Update connection state → Show user menu

**Authenticated User Flow:**
1. Extract user context from connection
2. Validate token with auth service
3. Fetch current user info
4. Show appropriate menu based on user state

**Session State Transitions:**
```go
type ConnectionState int

const (
    StateConnected     ConnectionState = iota
    StateAnonymous     // Showing anonymous menu
    StateAuthenticating // Processing login/register
    StateAuthenticated  // Showing user menu
    StateGameSelection // Choosing game
    StateInGame       // Active game session
    StateWatching     // Spectating mode
    StateClosing      // Graceful shutdown
)
```

### Task 4: Menu Flow Implementation

#### 4.1 Complete Menu Hierarchy
**Menu Structure:**
```
Anonymous Menu:
├── Login → Authentication Flow → User Menu
├── Register → Registration Flow → User Menu  
├── Watch → Game Selection (Anonymous) → Spectate
├── List Games → Game Information Display
└── Quit → Connection Close

User Menu:
├── Play → Game Selection → Start Game Session
├── Watch → Game Selection → Spectate Session
├── Edit Profile → Profile Management
├── List Games → Enhanced Game Information
├── View Recordings → Recording Browser
├── Statistics → User Stats Display
└── Quit → Connection Close
```

#### 4.2 Menu Choice Processing
**Legacy Reference:** `internal/session/connection/handler.go:652-765`

**Pool-Based Implementation:**
```go
func (pmh *PoolMenuHandler) ProcessMenuChoice(ctx context.Context, conn *pools.Connection, choice *MenuChoice) (*ActionResult, error) {
    switch choice.Action {
    case "login":
        return pmh.handleLogin(ctx, conn, choice.Value)
    case "register":
        return pmh.handleRegister(ctx, conn, choice.Value)
    case "play":
        return pmh.handleGameStart(ctx, conn, choice.Value)
    case "watch":
        return pmh.handleSpectate(ctx, conn, choice.Value)
    // ... other actions
    }
}

type ActionResult struct {
    NextState    ConnectionState
    RedirectTo   *MenuRequest
    SessionData  interface{}
    Error        error
}
```

#### 4.3 Service Integration
**Health Check Integration:**
```go
func (pmh *PoolMenuHandler) ValidateServiceHealth(ctx context.Context) error {
    // Check auth service availability
    if err := pmh.authClient.HealthCheck(ctx); err != nil {
        return fmt.Errorf("auth service unavailable: %w", err)
    }
    
    // Check game service availability  
    if err := pmh.gameClient.HealthCheck(ctx); err != nil {
        return fmt.Errorf("game service unavailable: %w", err)
    }
    
    return nil
}

func (pmh *PoolMenuHandler) HandleServiceDegradation(ctx context.Context, conn *pools.Connection) error {
    // Show maintenance mode banner
    // Provide limited functionality
    // Allow graceful disconnection
}
```

### Task 5: Worker Pool Integration

#### 5.1 Menu Work Items
**Work Type Definition:**
```go
const (
    WorkTypeMenuDisplay WorkType = iota + 100
    WorkTypeMenuInput
    WorkTypeMenuChoice
    WorkTypeUserAuth
    WorkTypeGameSelection
)

type MenuWorkItem struct {
    *pools.WorkItem
    MenuRequest  *MenuRequest
    ResponseChan chan *MenuResponse
}
```

#### 5.2 Menu Handler Integration
**Integration with Pool Session Handler:**
```go
func (sh *SessionHandler) HandleMenuAction(ctx context.Context, conn *pools.Connection) error {
    // Create menu work item
    menuWork := &pools.WorkItem{
        Type: pools.WorkTypeMenuAction,
        Handler: func(ctx context.Context, conn *pools.Connection) error {
            return sh.menuHandler.ProcessMenuRequest(ctx, conn)
        },
        Context:  ctx,
        Priority: pools.PriorityNormal,
        QueuedAt: time.Now(),
    }
    
    // Submit to worker pool
    return sh.workerPool.Submit(menuWork)
}
```

#### 5.3 Resource Management
**Memory Management:**
- Input/output buffer pooling
- Menu template caching
- Connection state cleanup
- Graceful worker shutdown

**Concurrency Safety:**
- Thread-safe menu state access
- Proper context cancellation
- Resource cleanup on worker timeout
- Connection state synchronization

### Task 6: Banner and Template System

#### 6.1 Pool-Compatible Banner Manager
**Legacy Reference:** `internal/session/banner/banner.go`

**Pool Implementation:**
```go
type PoolBannerManager struct {
    templates map[string]*template.Template
    config    *BannerConfig
    cache     *sync.Map
    logger    *slog.Logger
}

func (pbm *PoolBannerManager) RenderBanner(ctx context.Context, bannerType BannerType, vars map[string]string) ([]byte, error)
func (pbm *PoolBannerManager) LoadBannerTemplates(bannerDir string) error
func (pbm *PoolBannerManager) GetBannerVariables(conn *pools.Connection) map[string]string
```

#### 6.2 Template Variable System
**Available Variables:**
- `$USERNAME` - Current user's name (or "Anonymous")
- `$SERVERID` - Server identifier
- `$DATE` - Current date
- `$TIME` - Current time
- `$ACTIVE_USERS` - Current user count
- `$ACTIVE_GAMES` - Active game sessions

**Configuration-Driven Templates:**
```yaml
menu:
  banners:
    main_anon: "assets/banners/main_anonymous.txt"
    main_user: "assets/banners/main_authenticated.txt"
    watch_menu: "assets/banners/watch_selection.txt"
    idle_mode: "assets/banners/idle_mode.txt"
```

### Task 7: Error Handling and Recovery

#### 7.1 Graceful Error Recovery
**Error Scenarios:**
- Service unavailability (auth/game services down)
- Network disconnections during menu display
- Invalid user input or malformed requests
- Authentication token expiration
- Resource exhaustion (connection limits)

**Recovery Patterns:**
```go
func (pmh *PoolMenuHandler) RecoverFromError(ctx context.Context, conn *pools.Connection, err error) error {
    switch {
    case errors.Is(err, context.Canceled):
        return pmh.handleConnectionCanceled(ctx, conn)
    case isServiceUnavailable(err):
        return pmh.showMaintenanceMode(ctx, conn)
    case isAuthenticationError(err):
        return pmh.redirectToLogin(ctx, conn)
    default:
        return pmh.showErrorAndReturnToMenu(ctx, conn, err)
    }
}
```

#### 7.2 Connection State Recovery
**State Restoration:**
- Reconnection handling with state preservation
- Menu position restoration after service interruption
- Graceful degradation when services are partially available
- User session recovery from authentication tokens

### Task 8: Testing and Validation

#### 8.1 Menu Flow Testing
**Test Coverage Required:**
- Complete menu navigation paths
- Authentication state transitions
- Service degradation scenarios
- Concurrent user handling
- Resource cleanup verification

**Test Structure:**
```go
func TestPoolMenuHandler_AnonymousMenuFlow(t *testing.T)
func TestPoolMenuHandler_AuthenticatedMenuFlow(t *testing.T)  
func TestPoolMenuHandler_ServiceDegradation(t *testing.T)
func TestPoolMenuHandler_ConcurrentMenuAccess(t *testing.T)
func BenchmarkPoolMenuHandler_MenuDisplay(b *testing.B)
```

#### 8.2 Integration Testing
**End-to-End Scenarios:**
- SSH connection → menu navigation → game selection → session start
- User registration → login → profile management → logout
- Service failure → maintenance mode → service recovery → normal operation
- High load → connection pooling → worker distribution → response times

#### 8.3 Performance Validation
**Performance Targets:**
- Menu display latency: < 100ms
- Input processing: < 50ms
- Authentication validation: < 200ms  
- Concurrent menu users: 100+
- Memory usage per menu session: < 10MB

---

## Phase 4.5 Success Criteria

### 🎯 Functional Requirements
- **Complete Menu System**: All legacy menu functionality implemented in pool architecture
- **User Authentication**: Seamless login/logout/registration flows
- **Game Selection**: Full game selection and session initiation
- **Service Integration**: Proper auth/game service integration with health checks
- **Error Recovery**: Graceful handling of all error scenarios

### 🚀 Performance Requirements  
- **Responsive Menus**: Sub-100ms menu display times
- **Concurrent Users**: Support 100+ simultaneous menu users
- **Resource Efficiency**: Minimal memory footprint per connection
- **Worker Pool Utilization**: Efficient distribution of menu work across workers

### 🔧 Technical Requirements
- **Pool Integration**: Seamless integration with existing pool infrastructure
- **State Management**: Robust user state tracking and transitions
- **Terminal Compatibility**: Full ANSI terminal support and input handling
- **Configuration Driven**: Menu options and banners configurable via YAML

### 📊 Quality Requirements
- **Test Coverage**: 90%+ test coverage for menu functionality
- **Documentation**: Complete API documentation and usage examples
- **Maintainability**: Clean, readable code following established patterns
- **Monitoring**: Comprehensive metrics and logging for menu operations

---

## Phase 4.5 Deliverables

### Code Components
1. **`pool_menu_handler.go`** - Core pool-based menu handler
2. **`pool_terminal_handler.go`** - Terminal I/O integration for pools
3. **`pool_banner_manager.go`** - Template and banner rendering
4. **Menu work item integration** - Worker pool menu processing
5. **Authentication state management** - User context handling
6. **Service integration** - Health checks and degradation handling

### Test Suite
1. **Unit tests** for all menu components
2. **Integration tests** for complete menu flows  
3. **Performance benchmarks** for menu operations
4. **Load tests** for concurrent menu usage

### Documentation
1. **Menu architecture documentation**
2. **Configuration guide** for menu customization
3. **Troubleshooting guide** for common menu issues
4. **Performance tuning guide** for menu optimization

---

## Implementation Notes

### Priority Order
1. **Core menu handler structure** (foundation)
2. **Terminal I/O integration** (basic functionality)  
3. **Authentication flows** (security)
4. **Menu hierarchy implementation** (feature completeness)
5. **Worker pool integration** (performance)
6. **Error handling and recovery** (reliability)

### Legacy Integration Points
- **Reuse `internal/session/menu/menu.go` patterns** where possible
- **Adapt `internal/session/banner/banner.go` for pool architecture** 
- **Leverage `internal/session/terminal/input.go` input processing**
- **Maintain compatibility with existing configuration** in `configs/session-service.yaml`

### Risk Mitigation
- **Incremental implementation** with feature flags
- **Fallback to legacy** during development/testing
- **Comprehensive testing** before full migration
- **Performance monitoring** throughout development

This phase bridges the gap between the functional pool infrastructure and a complete user experience, implementing the critical menu navigation that users interact with directly.