# Session Service Refactor - Phase 3: Integration and Testing

## Context

**Phase 2 is COMPLETE** with all handler extraction and pool integration implemented:

### ✅ Phase 2 Completed Components
- **SessionHandler**: Main coordinator using pool infrastructure (`internal/session/handlers/session_handler.go`)
- **AuthHandler**: Authentication workflows with resource limits (`internal/session/handlers/auth_handler.go`)
- **GameHandler**: Game session management with PTY pool (`internal/session/handlers/game_handler.go`)
- **StreamHandler**: I/O streaming and spectating (`internal/session/handlers/stream_handler.go`)
- **PoolAwareMenuHandler**: Enhanced menu system (`internal/session/handlers/menu_handler.go`)
- **Service Integration**: Complete initialization examples (`internal/session/handlers/service_integration.go`)

### 🎯 Phase 3 Objectives

Your task is to implement **Phase 3: Integration and Testing** by resolving dependencies, implementing comprehensive tests, and preparing the new architecture for production deployment.

## Phase 3 Tasks

### Task 1: Resolve Import Dependencies

**Current Issue**: The new handlers have import path conflicts with the existing session-old structure.

#### Fix Import Paths and Dependencies

1. **Analyze Current Import Issues**:
   - Review compilation errors in the handlers
   - Identify circular dependencies and missing imports
   - Map the correct import paths for existing components

2. **Create Adapter Interfaces** (if needed):
   ```go
   // internal/session/adapters/legacy.go
   package adapters
   
   // Adapter interfaces to bridge session-old components with new handlers
   type MenuAdapter interface {
       ShowAnonymousMenu(ctx context.Context, channel ssh.Channel, username string) (*menu.MenuChoice, error)
       ShowUserMenu(ctx context.Context, channel ssh.Channel, username string) (*menu.MenuChoice, error)
   }
   
   type ClientAdapter interface {
       AuthClient() AuthClientInterface
       GameClient() GameClientInterface
   }
   ```

3. **Update Import Paths**:
   - Fix all import statements in handlers to use correct paths
   - Ensure compatibility with existing session-old components
   - Create bridge packages if necessary

4. **Verify Compilation**:
   ```bash
   go build ./internal/session/handlers/...
   go build ./cmd/session-service
   ```

### Task 2: Implement Comprehensive Unit Tests

**Goal**: Create thorough test suites for all handlers with >85% coverage.

#### 2.1 SessionHandler Tests (`internal/session/handlers/session_handler_test.go`)

```go
package handlers_test

import (
    "context"
    "net"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/dungeongate/internal/session/handlers"
    "github.com/dungeongate/internal/session/pools"
    "golang.org/x/crypto/ssh"
)

func TestSessionHandler_HandleNewConnection(t *testing.T) {
    tests := []struct {
        name           string
        setupMocks     func(*testing.T) (*mocks, *handlers.SessionHandler)
        simulateConn   func() net.Conn
        expectedError  error
        expectedMetrics map[string]float64
        backpressure   bool
    }{
        {
            name: "successful_connection_under_limit",
            setupMocks: func(t *testing.T) (*mocks, *handlers.SessionHandler) {
                // Create mock pools, clients, etc.
                return setupSuccessfulMocks(t)
            },
            expectedMetrics: map[string]float64{
                "connections_total": 1,
                "connections_active": 1,
            },
            backpressure: false,
        },
        {
            name: "connection_rejected_by_backpressure",
            setupMocks: func(t *testing.T) (*mocks, *handlers.SessionHandler) {
                return setupBackpressureMocks(t)
            },
            expectedError: handlers.ErrServerOverloaded,
            backpressure: true,
        },
        {
            name: "ssh_handshake_failure",
            setupMocks: func(t *testing.T) (*mocks, *handlers.SessionHandler) {
                return setupHandshakeFailureMocks(t)
            },
            expectedError: handlers.ErrSSHHandshakeFailed,
        },
        {
            name: "connection_pool_at_capacity",
            setupMocks: func(t *testing.T) (*mocks, *handlers.SessionHandler) {
                return setupPoolCapacityMocks(t)
            },
            expectedError: pools.ErrConnectionPoolAtCapacity,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mocks, handler := tt.setupMocks(t)
            defer mocks.AssertExpectations(t)
            
            // Create test connection
            conn := tt.simulateConn()
            if conn == nil {
                conn = createMockConnection(t)
            }
            
            // Execute
            err := handler.HandleNewConnection(context.Background(), conn, createMockSSHConfig(t))
            
            // Verify
            if tt.expectedError != nil {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.expectedError.Error())
            } else {
                assert.NoError(t, err)
            }
            
            // Verify metrics
            verifyHandlerMetrics(t, handler, tt.expectedMetrics)
        })
    }
}

func TestSessionHandler_GracefulShutdown(t *testing.T) {
    // Test graceful shutdown with active connections
    // Verify all pools shutdown correctly
    // Ensure no resource leaks
}

func TestSessionHandler_ConcurrentConnections(t *testing.T) {
    // Test multiple concurrent connections
    // Verify pool management under load
    // Check resource tracking accuracy
}
```

#### 2.2 AuthHandler Tests (`internal/session/handlers/auth_handler_test.go`)

```go
func TestAuthHandler_HandleLogin(t *testing.T) {
    tests := []struct {
        name              string
        setupMocks        func(*testing.T) (*authMocks, *handlers.AuthHandler)
        userInput         []string // Simulated user input
        expectedResult    *authv1.User
        expectedError     error
        expectedMetrics   map[string]float64
        resourceLimited   bool
    }{
        {
            name: "successful_login",
            userInput: []string{"testuser", "password123"},
            expectedResult: &authv1.User{Id: "1", Username: "testuser"},
            expectedMetrics: map[string]float64{
                "login_attempts": 1,
                "login_successes": 1,
                "login_failures": 0,
            },
        },
        {
            name: "invalid_credentials",
            userInput: []string{"testuser", "wrongpassword"},
            expectedError: handlers.ErrAuthenticationFailed,
            expectedMetrics: map[string]float64{
                "login_attempts": 1,
                "login_failures": 1,
            },
        },
        {
            name: "rate_limited",
            resourceLimited: true,
            expectedError: handlers.ErrRateLimitExceeded,
        },
        {
            name: "user_cancellation",
            userInput: []string{"", ""}, // Empty inputs simulate cancellation
            expectedError: nil, // Should return gracefully
        },
    }
    
    // Test implementation...
}

func TestAuthHandler_HandleRegister(t *testing.T) {
    // Test registration workflows
    // Test password confirmation validation
    // Test retry logic
    // Test resource limiting
}

func TestAuthHandler_TokenValidation(t *testing.T) {
    // Test token validation scenarios
    // Test expired tokens
    // Test invalid tokens
    // Test service unavailability
}
```

#### 2.3 GameHandler Tests (`internal/session/handlers/game_handler_test.go`)

```go
func TestGameHandler_StartGameSession(t *testing.T) {
    tests := []struct {
        name            string
        setupMocks      func(*testing.T) (*gameMocks, *handlers.GameHandler)
        userInfo        *authv1.User
        gameID          string
        expectedSession *GameSession
        expectedError   error
        ptyPoolFull     bool
        serviceDown     bool
    }{
        {
            name: "successful_nethack_session",
            userInfo: &authv1.User{Id: "1", Username: "testuser"},
            gameID: "nethack",
            expectedSession: &GameSession{ID: "session-123"},
        },
        {
            name: "invalid_user_id",
            userInfo: &authv1.User{Id: "invalid", Username: "testuser"},
            gameID: "nethack",
            expectedError: handlers.ErrInvalidUserID,
        },
        {
            name: "pty_pool_exhausted",
            ptyPoolFull: true,
            expectedError: pools.ErrPTYPoolAtCapacity,
        },
        {
            name: "game_service_unavailable",
            serviceDown: true,
            expectedError: handlers.ErrGameServiceUnavailable,
        },
    }
    
    // Test implementation...
}

func TestGameHandler_GameIO(t *testing.T) {
    // Test bidirectional I/O
    // Test stream management
    // Test disconnect scenarios
    // Test data transfer tracking
}

func TestGameHandler_TerminalResize(t *testing.T) {
    // Test terminal resize operations
    // Test invalid dimensions
    // Test resize during active session
}
```

#### 2.4 StreamHandler Tests (`internal/session/handlers/stream_handler_test.go`)

```go
func TestStreamHandler_HandleGameIO(t *testing.T) {
    // Test I/O streaming setup
    // Test bandwidth tracking
    // Test stream health monitoring
}

func TestStreamHandler_HandleSpectating(t *testing.T) {
    // Test spectator connections
    // Test spectator limits
    // Test spectator data streaming
}

func TestStreamHandler_DataTransferTracking(t *testing.T) {
    // Test bandwidth calculations
    // Test data transfer metrics
    // Test resource usage tracking
}
```

#### 2.5 PoolAwareMenuHandler Tests (`internal/session/handlers/menu_handler_test.go`)

```go
func TestPoolAwareMenuHandler_ExecuteAction(t *testing.T) {
    // Test resource limit checking
    // Test worker pool integration
    // Test action prioritization
    // Test immediate vs deferred execution
}

func TestPoolAwareMenuHandler_MenuLoop(t *testing.T) {
    // Test complete menu workflows
    // Test user session management
    // Test menu navigation
}
```

### Task 3: Integration Tests

**Goal**: Verify handlers work together correctly in realistic scenarios.

#### 3.1 End-to-End Flow Tests (`test/integration/handlers_integration_test.go`)

```go
func TestCompleteUserFlow(t *testing.T) {
    // Setup: Start all pools and handlers
    // Test: Complete user flow from connection → auth → game → disconnect
    // Verify: All resources cleaned up, metrics updated correctly
}

func TestConcurrentUserSessions(t *testing.T) {
    // Test multiple users simultaneously
    // Verify resource isolation
    // Check pool utilization
}

func TestServiceFailureRecovery(t *testing.T) {
    // Test auth service failures
    // Test game service failures
    // Verify graceful degradation
}

func TestResourceExhaustion(t *testing.T) {
    // Test behavior at resource limits
    // Verify backpressure activation
    // Check resource cleanup
}
```

#### 3.2 Performance Tests (`test/performance/handlers_performance_test.go`)

```go
func BenchmarkSessionHandler_NewConnections(b *testing.B) {
    // Benchmark connection setup time
    // Target: < 100ms per connection
}

func BenchmarkAuthHandler_LoginOperations(b *testing.B) {
    // Benchmark authentication performance
    // Test under various loads
}

func BenchmarkGameHandler_IOThroughput(b *testing.B) {
    // Benchmark game I/O performance
    // Test data transfer rates
}

func TestLoadHandling(t *testing.T) {
    // Test under sustained load
    // Verify memory usage stability
    // Check for resource leaks
}
```

### Task 4: Update Service Configuration

#### 4.1 Update `configs/session-service.yaml`

```yaml
# Add pool-based configuration
pools:
  connection_pool:
    max_connections: 1000
    queue_size: 100
    queue_timeout: 30s
    idle_timeout: 300s
    drain_timeout: 60s
  
  worker_pool:
    min_workers: 10
    max_workers: 100
    queue_size: 1000
    worker_timeout: 60s
    scaling_factor: 0.8
  
  pty_pool:
    max_ptys: 500
    cleanup_interval: 60s
    idle_timeout: 300s
    resource_limits:
      memory_mb: 100
      cpu_percent: 50
  
  backpressure:
    enabled: true
    cpu_threshold: 80.0
    memory_threshold: 85.0
    connection_threshold: 900
    circuit_breaker:
      failure_threshold: 10
      timeout: 30s
      max_requests: 3

resource_management:
  limits:
    connections_per_user: 5
    pty_per_user: 3
    bandwidth_mbps: 10
    memory_mb: 500
  rate_limiting:
    login_attempts: 5
    login_window: 300s
    register_attempts: 3
    register_window: 3600s

metrics:
  collection_interval: 10s
  export_interval: 30s
  retention_period: 24h
  buckets: [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]

# Migration settings
migration:
  use_pool_based_handlers: true
  fallback_to_legacy: false
  handlers:
    session_handler: true
    auth_handler: true
    game_handler: true
    stream_handler: true
    menu_handler: true
```

#### 4.2 Update Service Initialization (`cmd/session-service/main.go`)

```go
func main() {
    // Load configuration
    config := loadConfig()
    
    // Check if pool-based handlers are enabled
    if config.Migration.UsePoolBasedHandlers {
        // Initialize pool-based service
        sessionHandler, err := handlers.InitializePoolBasedService(
            config.ServiceConfig, authClient, gameClient, menuHandler, logger)
        if err != nil {
            log.Fatal("Failed to initialize pool-based service:", err)
        }
        
        // Start all components
        if err := handlers.StartPoolBasedService(ctx, sessionHandler); err != nil {
            log.Fatal("Failed to start pool-based service:", err)
        }
        
        // Use new handler
        sshServer.SetConnectionHandler(sessionHandler.HandleNewConnection)
        
        // Setup graceful shutdown
        defer func() {
            if err := handlers.ShutdownPoolBasedService(ctx, sessionHandler); err != nil {
                log.Error("Failed to shutdown pool-based service:", err)
            }
        }()
    } else {
        // Use legacy handler
        if config.Migration.FallbackToLegacy {
            sshServer.SetConnectionHandler(legacyHandler.HandleConnection)
        } else {
            log.Fatal("Pool-based handlers disabled and legacy fallback disabled")
        }
    }
}
```

### Task 5: Metrics and Monitoring Integration

#### 5.1 Create Metrics Dashboard Configuration

```yaml
# configs/metrics-dashboard.yaml
dashboards:
  session_service:
    panels:
      - title: "Connection Metrics"
        metrics:
          - session_connections_total
          - session_connections_active
          - session_connection_duration_seconds
      
      - title: "Authentication Metrics"
        metrics:
          - session_auth_login_attempts_total
          - session_auth_login_successes_total
          - session_auth_login_failures_total
      
      - title: "Game Session Metrics"
        metrics:
          - session_game_sessions_started_total
          - session_game_sessions_active
          - session_game_session_duration_seconds
      
      - title: "Resource Utilization"
        metrics:
          - session_pool_connections_utilization
          - session_pool_workers_utilization
          - session_pool_pty_utilization
```

#### 5.2 Create Alerting Rules

```yaml
# configs/alerting-rules.yaml
groups:
  - name: session_service_alerts
    rules:
      - alert: HighConnectionFailureRate
        expr: rate(session_handler_errors_total[5m]) > 0.1
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High connection failure rate"
      
      - alert: ResourcePoolExhaustion
        expr: session_connections_active / session_pool_max_connections > 0.9
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Connection pool near capacity"
```

### Task 6: Documentation and Migration Guide

#### 6.1 Create Migration Guide (`docs/migration-guide.md`)

Document the complete migration process:
- Configuration changes required
- Feature flag usage
- Rollback procedures
- Performance comparisons
- Troubleshooting common issues

#### 6.2 Update API Documentation

Update all relevant documentation to reflect:
- New handler architecture
- Metrics available
- Configuration options
- Performance characteristics

## Success Criteria

### Phase 3 Complete When:

1. **✅ Import Resolution**: All handlers compile without errors
2. **✅ Unit Tests**: >85% test coverage for all handlers
3. **✅ Integration Tests**: End-to-end flows tested and working
4. **✅ Performance Tests**: Load testing shows performance targets met
5. **✅ Configuration**: Updated configs with pool settings
6. **✅ Service Integration**: Service startup uses new architecture
7. **✅ Metrics**: Complete observability stack working
8. **✅ Documentation**: Migration guide and API docs updated

### Performance Targets:

- **Connection Setup**: < 100ms average
- **Memory Usage**: No memory leaks under sustained load
- **Throughput**: Handle 1000+ concurrent connections
- **Resource Utilization**: Pool utilization tracking accurate

### Compatibility Requirements:

- **Backward Compatible**: All existing SSH clients work unchanged
- **Feature Complete**: All menu functionality preserved
- **Service Integration**: Auth and Game services work unchanged
- **Graceful Fallback**: Legacy handler available if needed

## Testing Strategy

### Unit Testing
- Mock all external dependencies
- Test success and failure scenarios
- Verify metrics accuracy
- Test resource cleanup

### Integration Testing
- Test handler coordination
- Verify pool integration
- Test service failure scenarios
- Performance under load

### Acceptance Testing
- Complete user workflows
- SSH client compatibility
- Performance requirements
- Resource management

This phase transforms the proof-of-concept handlers into a production-ready, thoroughly tested, and fully integrated session service architecture.