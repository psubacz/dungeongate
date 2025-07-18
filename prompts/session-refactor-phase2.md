# Session Service Refactor - Phase 2: Handler Extraction and Integration

## Context

You are continuing the DungeonGate Session Service refactor. **Phase 1 is COMPLETE** with all pool infrastructure and resource management components implemented:

### ✅ Phase 1 Completed Components
- **Pool Infrastructure**: `internal/session/pools/` with connection_pool.go, worker_pool.go, pty_pool.go, backpressure.go
- **Resource Management**: `internal/session/resources/` with limiter.go, tracker.go, metrics.go  
- **Configuration**: Extended `configs/session-service.yaml` with comprehensive pool settings
- **Documentation**: Complete `docs/session.md` with new architecture

### 🎯 Phase 2 Objectives

Your task is to implement **Phase 2: Handler Refactoring** by extracting the monolithic handler and creating specialized handlers that integrate with the pool infrastructure.

## Phase 2 Tasks

### Task 1: Extract SessionHandler from Monolithic Handler

**Current State**: The legacy handler is in `internal/session-old/connection/handler.go` (1315 lines)

**Goal**: Create a new `SessionHandler` that coordinates all session activities using the pool infrastructure.

#### Create `internal/session/handlers/session_handler.go`

```go
package handlers

import (
    "context"
    "log/slog"
    
    "github.com/dungeongate/internal/session/pools"
    "github.com/dungeongate/internal/session/resources"
    "github.com/dungeongate/internal/session-old/menu"  // Existing menu system
    "golang.org/x/crypto/ssh"
)

type SessionHandler struct {
    connectionPool  *pools.ConnectionPool
    workerPool      *pools.WorkerPool
    ptyPool         *pools.PTYPool
    backpressure    *pools.BackpressureManager
    
    resourceLimiter *resources.ResourceLimiter
    resourceTracker *resources.ResourceTracker
    metricsRegistry *resources.MetricsRegistry
    
    authHandler     *AuthHandler
    gameHandler     *GameHandler
    streamHandler   *StreamHandler
    menuHandler     *menu.MenuHandler  // Enhanced existing
    
    logger          *slog.Logger
}

// Main entry point - replaces the old HandleConnection
func (sh *SessionHandler) HandleNewConnection(ctx context.Context, conn net.Conn, config *ssh.ServerConfig) error {
    // 1. Request connection from pool with backpressure check
    // 2. Submit work to worker pool for processing
    // 3. Coordinate between specialized handlers
    // 4. Track resources and metrics
}
```

**Key Requirements**:
- Replace direct goroutine spawning with worker pool submissions
- Use connection pool for all connection management
- Integrate backpressure management for overload protection
- Comprehensive logging using structured logging
- Resource tracking for all operations

### Task 2: Create Specialized Handlers

#### 2.1 Authentication Handler (`internal/session/handlers/auth_handler.go`)

Extract authentication logic from the monolithic handler:

```go
type AuthHandler struct {
    authClient      *client.AuthClient
    resourceLimiter *resources.ResourceLimiter
    workerPool      *pools.WorkerPool
    logger          *slog.Logger
}

// Handle authentication workflows with resource limits
func (ah *AuthHandler) HandleLogin(ctx context.Context, conn *pools.Connection, channel ssh.Channel) error
func (ah *AuthHandler) HandleRegister(ctx context.Context, conn *pools.Connection, channel ssh.Channel) error
func (ah *AuthHandler) ValidateToken(ctx context.Context, token string) (*authv1.User, error)
```

**Features to implement**:
- Resource-aware authentication (check user quotas)
- Worker pool integration for auth operations
- Comprehensive auth metrics tracking
- Rate limiting for brute force protection

#### 2.2 Game Handler (`internal/session/handlers/game_handler.go`)

Extract game session management:

```go
type GameHandler struct {
    gameClient      *client.GameClient
    ptyPool         *pools.PTYPool
    resourceTracker *resources.ResourceTracker
    workerPool      *pools.WorkerPool
    logger          *slog.Logger
}

// Handle game session lifecycle with PTY pool
func (gh *GameHandler) StartGameSession(ctx context.Context, conn *pools.Connection, userInfo *authv1.User, gameID string) error
func (gh *GameHandler) HandleGameSelection(ctx context.Context, conn *pools.Connection, userInfo *authv1.User) error
func (gh *GameHandler) StopGameSession(ctx context.Context, sessionID string) error
```

**Features to implement**:
- PTY pool integration for efficient resource usage
- Game session tracking and metrics
- Worker pool for game operations
- Resource quota enforcement

#### 2.3 Stream Handler (`internal/session/handlers/stream_handler.go`)

Extract I/O streaming and spectating:

```go
type StreamHandler struct {
    streamingManager *streaming.Manager
    resourceTracker  *resources.ResourceTracker
    workerPool       *pools.WorkerPool
    logger           *slog.Logger
}

// Handle I/O streaming with resource tracking
func (sh *StreamHandler) HandleGameIO(ctx context.Context, conn *pools.Connection, sessionID string) error
func (sh *StreamHandler) HandleSpectating(ctx context.Context, conn *pools.Connection, sessionID string) error
func (sh *StreamHandler) TrackDataTransfer(connectionID string, bytesSent, bytesReceived int64)
```

**Features to implement**:
- Data transfer tracking and bandwidth limiting
- Spectator session management
- Worker pool for I/O operations
- Stream health monitoring

### Task 3: Enhance Existing MenuHandler

**Current Location**: `internal/session-old/menu/menu.go`

**Goal**: Make the menu system pool-aware while maintaining existing functionality.

#### Create Pool-Aware Menu Actions

```go
// Add to existing MenuHandler
type PoolAwareMenuHandler struct {
    *menu.MenuHandler  // Embed existing functionality
    workerPool         *pools.WorkerPool
    resourceLimiter    *resources.ResourceLimiter
    connectionPool     *pools.ConnectionPool
    logger             *slog.Logger
}

func (pmh *PoolAwareMenuHandler) ExecuteAction(ctx context.Context, conn *pools.Connection, choice *menu.MenuChoice) error {
    // Check resource limits before execution
    if !pmh.resourceLimiter.CanExecute(conn.UserID, choice.Action) {
        return ErrResourceLimitExceeded
    }
    
    // Create work item based on action type
    workType := pmh.getWorkTypeForAction(choice.Action)
    work := &pools.WorkItem{
        Type:       workType,
        Connection: conn,
        Handler:    pmh.getHandlerForAction(choice.Action),
        Context:    ctx,
        Priority:   pmh.getPriorityForAction(choice.Action),
        QueuedAt:   time.Now(),
    }
    
    // Submit to worker pool
    return pmh.workerPool.Submit(work)
}
```

### Task 4: Configuration Integration

#### 4.1 Update Service Configuration Loading

Modify the service startup to load and use pool configurations:

```go
// In cmd/session-service/main.go or service initialization
func initializePoolBasedService(config *Config) (*SessionHandler, error) {
    // Initialize pools
    connectionPool, err := pools.NewConnectionPool(config.ConnectionPool, logger)
    workerPool, err := pools.NewWorkerPool(config.WorkerPool, logger)
    ptyPool, err := pools.NewPTYPool(config.PTYPool, logger)
    backpressure, err := pools.NewBackpressureManager(config.Backpressure, logger)
    
    // Initialize resource management
    resourceLimiter, err := resources.NewResourceLimiter(config.ResourceManagement, logger)
    resourceTracker := resources.NewResourceTracker(logger)
    metricsRegistry := resources.NewMetricsRegistry(config.PoolMetrics, logger)
    
    // Create specialized handlers
    authHandler := handlers.NewAuthHandler(authClient, resourceLimiter, workerPool, logger)
    gameHandler := handlers.NewGameHandler(gameClient, ptyPool, resourceTracker, workerPool, logger)
    streamHandler := handlers.NewStreamHandler(streamingManager, resourceTracker, workerPool, logger)
    
    // Create enhanced menu handler
    menuHandler := handlers.NewPoolAwareMenuHandler(existingMenuHandler, workerPool, resourceLimiter, logger)
    
    // Create session handler
    sessionHandler := handlers.NewSessionHandler(
        connectionPool, workerPool, ptyPool, backpressure,
        resourceLimiter, resourceTracker, metricsRegistry,
        authHandler, gameHandler, streamHandler, menuHandler,
        logger)
    
    return sessionHandler, nil
}
```

#### 4.2 Graceful Migration Path

Implement a feature flag system to gradually migrate from old to new handlers:

```yaml
# In session-service.yaml
migration:
  use_pool_based_handlers: true  # Enable new architecture
  fallback_to_legacy: false      # Disable legacy fallback
  
  # Individual handler migration flags
  handlers:
    session_handler: true
    auth_handler: true
    game_handler: true
    stream_handler: true
    menu_handler: true
```

## Implementation Guidelines

### Error Handling

All handlers must implement comprehensive error handling:

```go
// Example error handling pattern
func (h *Handler) HandleOperation(ctx context.Context, conn *pools.Connection) error {
    // Track operation start
    h.resourceTracker.TrackOperation(conn.ID, "operation_start")
    defer h.resourceTracker.TrackOperation(conn.ID, "operation_end")
    
    // Check resource limits
    if !h.resourceLimiter.CanExecute(conn.UserID, "operation") {
        h.logger.Warn("Operation blocked by resource limiter", 
            "user_id", conn.UserID, 
            "connection_id", conn.ID)
        return ErrResourceLimitExceeded
    }
    
    // Perform operation with timeout
    operationCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    err := h.performOperation(operationCtx, conn)
    if err != nil {
        h.logger.Error("Operation failed", 
            "error", err,
            "user_id", conn.UserID,
            "connection_id", conn.ID)
        return fmt.Errorf("operation failed: %w", err)
    }
    
    h.logger.Info("Operation completed successfully",
        "user_id", conn.UserID,
        "connection_id", conn.ID)
    
    return nil
}
```

### Logging Standards

All handlers must use structured logging with consistent fields:

```go
// Standard logging fields for all handlers
logger.Info("Handler operation",
    "handler", "auth",
    "operation", "login",
    "connection_id", conn.ID,
    "user_id", conn.UserID,
    "remote_addr", conn.RemoteAddr,
    "duration", time.Since(startTime),
    "success", success)
```

### Metrics Integration

Each handler must register and update relevant metrics:

```go
// Example metrics integration
type AuthHandler struct {
    // ... other fields
    
    // Metrics
    loginAttempts   *resources.CounterMetric
    loginSuccesses  *resources.CounterMetric
    loginFailures   *resources.CounterMetric
    loginDuration   *resources.HistogramMetric
}

func (ah *AuthHandler) initializeMetrics(registry *resources.MetricsRegistry) {
    ah.loginAttempts = registry.RegisterCounter(
        "session_auth_login_attempts_total",
        "Total number of login attempts",
        map[string]string{"handler": "auth"})
    
    ah.loginDuration = registry.RegisterHistogram(
        "session_auth_login_duration_seconds",
        "Time spent processing login requests",
        nil,
        map[string]string{"handler": "auth"})
}
```

## Testing Requirements

### Unit Tests

Each handler must have comprehensive unit tests:

```go
// Example test structure
func TestSessionHandler_HandleNewConnection(t *testing.T) {
    tests := []struct {
        name           string
        setupMocks     func(*testing.T) (*mocks, *SessionHandler)
        inputConn      net.Conn
        expectedError  error
        expectedMetrics map[string]float64
    }{
        {
            name: "successful_connection_under_limit",
            setupMocks: func(t *testing.T) (*mocks, *SessionHandler) {
                // Setup mock pools and dependencies
            },
            expectedMetrics: map[string]float64{
                "active_connections": 1,
                "total_connections": 1,
            },
        },
        {
            name: "connection_rejected_at_limit",
            setupMocks: func(t *testing.T) (*mocks, *SessionHandler) {
                // Setup mock pools at capacity
            },
            expectedError: ErrConnectionPoolAtCapacity,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mocks, handler := tt.setupMocks(t)
            defer mocks.AssertExpectations(t)
            
            err := handler.HandleNewConnection(context.Background(), tt.inputConn, nil)
            
            if tt.expectedError != nil {
                assert.Error(t, err)
                assert.Equal(t, tt.expectedError, err)
            } else {
                assert.NoError(t, err)
            }
            
            // Verify metrics
            for metricName, expectedValue := range tt.expectedMetrics {
                // Assert metric values
            }
        })
    }
}
```

### Integration Tests

Create integration tests that verify handler coordination:

```go
func TestHandlerIntegration_CompleteUserFlow(t *testing.T) {
    // Setup: Start all pools and handlers
    // Test: Complete user flow from connection → auth → game → disconnect
    // Verify: All resources cleaned up, metrics updated correctly
}
```

## Success Criteria

### Phase 2 Complete When:

1. **✅ SessionHandler Created**: New main coordinator using pools
2. **✅ Specialized Handlers**: Auth, Game, and Stream handlers implemented
3. **✅ Menu Integration**: Existing menu system made pool-aware
4. **✅ Configuration**: Pool configs loaded and used throughout
5. **✅ Migration Path**: Feature flags for gradual rollout
6. **✅ Testing**: Comprehensive unit and integration tests
7. **✅ Logging**: Structured logging throughout all handlers
8. **✅ Metrics**: All handlers integrated with metrics registry
9. **✅ Documentation**: Updated docs with handler architecture

### Performance Targets:

- **Connection Setup**: < 100ms for new connections
- **Resource Utilization**: Pool utilization tracking working
- **Memory Usage**: No memory leaks in handlers
- **Graceful Shutdown**: All handlers support clean shutdown

### Compatibility Requirements:

- **Existing SSH Clients**: No changes to SSH protocol handling
- **Menu System**: All existing menu functionality preserved
- **Auth Service**: Existing auth service integration maintained
- **Game Service**: Existing game service integration maintained

## Next Steps After Phase 2

Upon successful completion of Phase 2, Phase 3 will focus on:

1. **Pool Integration**: Replace remaining direct goroutine usage
2. **Backpressure Activation**: Enable circuit breaker and load shedding
3. **Advanced Features**: Dynamic scaling, connection warming
4. **Load Testing**: Comprehensive performance testing

## Important Notes

### Code Quality Requirements:

- **No Breaking Changes**: Maintain existing API compatibility
- **Comprehensive Logging**: Every operation must be logged with context
- **Error Handling**: All errors properly wrapped and logged
- **Resource Cleanup**: All resources properly released
- **Thread Safety**: All handlers must be thread-safe

### Performance Considerations:

- **Worker Pool Usage**: Prefer worker pool over direct goroutines
- **Resource Limits**: Always check limits before operations
- **Metrics Overhead**: Minimize metrics collection overhead
- **Memory Management**: Proper cleanup of connections and sessions

This refactor transforms the session service from a monolithic design to a modern, scalable, pool-based architecture while maintaining full backward compatibility and adding comprehensive observability.